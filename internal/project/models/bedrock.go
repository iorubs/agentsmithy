package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	brdoc "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/iorubs/agentsmithy/internal/config"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

const defaultBedrockRegionEnv = "AWS_REGION"

// newBedrock builds an LLM that speaks the AWS Bedrock Converse API.
func newBedrock(entry config.ModelEntry) (LLM, error) {
	if entry.Model == "" {
		return nil, errors.New("bedrock: model is required")
	}
	return &bedrockLLM{entry: entry}, nil
}

type bedrockLLM struct {
	entry  config.ModelEntry
	client *bedrockruntime.Client
}

func (m *bedrockLLM) Name() string { return m.entry.Model }

func (m *bedrockLLM) getClient(ctx context.Context) (*bedrockruntime.Client, error) {
	if m.client != nil {
		return m.client, nil
	}

	var opts []func(*awsconfig.LoadOptions) error
	if region := os.Getenv(defaultBedrockRegionEnv); region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load AWS config: %w", err)
	}

	var clientOpts []func(*bedrockruntime.Options)
	if m.entry.BaseURL != "" {
		clientOpts = append(clientOpts, func(o *bedrockruntime.Options) {
			o.BaseEndpoint = &m.entry.BaseURL
		})
	}

	m.client = bedrockruntime.NewFromConfig(cfg, clientOpts...)
	return m.client, nil
}

func (m *bedrockLLM) GenerateContent(
	ctx context.Context,
	req *adkmodel.LLMRequest,
	_ bool,
) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		client, err := m.getClient(ctx)
		if err != nil {
			yield(nil, err)
			return
		}

		msgs := contentsToBedrockMessages(req.Contents)
		if len(msgs) == 0 {
			msgs = []brtypes.Message{{
				Role:    brtypes.ConversationRoleUser,
				Content: []brtypes.ContentBlock{&brtypes.ContentBlockMemberText{Value: "Go ahead with your task."}},
			}}
		}

		input := &bedrockruntime.ConverseInput{
			ModelId:  &m.entry.Model,
			Messages: msgs,
		}

		if req.Config != nil && req.Config.SystemInstruction != nil {
			var sysText string
			for _, p := range req.Config.SystemInstruction.Parts {
				if p != nil && p.Text != "" {
					sysText += p.Text
				}
			}
			if sysText != "" {
				input.System = []brtypes.SystemContentBlock{
					&brtypes.SystemContentBlockMemberText{Value: sysText},
				}
			}
		}

		if tools := bedrockToolsFromConfig(req.Config); len(tools) > 0 {
			input.ToolConfig = &brtypes.ToolConfiguration{
				Tools: tools,
			}
		}

		if m.entry.Temperature != nil || m.entry.MaxTokens != nil {
			inf := &brtypes.InferenceConfiguration{}
			if m.entry.Temperature != nil {
				t := float32(*m.entry.Temperature)
				inf.Temperature = &t
			}
			if m.entry.MaxTokens != nil {
				mt := int32(*m.entry.MaxTokens)
				inf.MaxTokens = &mt
			}
			input.InferenceConfig = inf
		}

		slog.DebugContext(ctx, "bedrock request",
			"model", m.entry.Model,
			"messages", len(msgs))

		out, err := client.Converse(ctx, input)
		if err != nil {
			yield(nil, fmt.Errorf("bedrock: converse: %w", err))
			return
		}

		yield(bedrockOutputToLLMResponse(out), nil)
	}
}

func contentsToBedrockMessages(contents []*genai.Content) []brtypes.Message {
	var msgs []brtypes.Message
	for _, c := range contents {
		if c == nil {
			continue
		}
		msg := contentToBedrockMessage(c)
		if len(msg.Content) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func contentToBedrockMessage(c *genai.Content) brtypes.Message {
	role := brtypes.ConversationRoleUser
	if c.Role == "model" || c.Role == "assistant" {
		role = brtypes.ConversationRoleAssistant
	}

	var blocks []brtypes.ContentBlock
	for _, p := range c.Parts {
		if p == nil {
			continue
		}
		switch {
		case p.Text != "":
			blocks = append(blocks, &brtypes.ContentBlockMemberText{Value: p.Text})
		case p.FunctionCall != nil:
			fc := p.FunctionCall
			args := fc.Args
			if args == nil {
				args = map[string]any{}
			}
			id := fc.ID
			if id == "" {
				id = fc.Name
			}
			blocks = append(blocks, &brtypes.ContentBlockMemberToolUse{
				Value: brtypes.ToolUseBlock{
					ToolUseId: &id,
					Name:      &fc.Name,
					Input:     brdoc.NewLazyDocument(args),
				},
			})
		case p.FunctionResponse != nil:
			fr := p.FunctionResponse
			respJSON, err := json.Marshal(fr.Response)
			if err != nil {
				slog.Warn("bedrock: marshal tool-response", "tool", fr.Name, "error", err)
				respJSON = []byte("{}")
			}
			id := fr.ID
			if id == "" {
				id = fr.Name
			}
			blocks = append(blocks, &brtypes.ContentBlockMemberToolResult{
				Value: brtypes.ToolResultBlock{
					ToolUseId: &id,
					Content: []brtypes.ToolResultContentBlock{
						&brtypes.ToolResultContentBlockMemberText{Value: string(respJSON)},
					},
				},
			})
		}
	}
	return brtypes.Message{Role: role, Content: blocks}
}

func bedrockToolsFromConfig(cfg *genai.GenerateContentConfig) []brtypes.Tool {
	if cfg == nil {
		return nil
	}
	var out []brtypes.Tool
	for _, t := range cfg.Tools {
		for _, fd := range t.FunctionDeclarations {
			schema := bedrockInputSchema(fd)
			name := fd.Name
			desc := fd.Description
			if desc == "" {
				desc = name
			}
			spec := brtypes.ToolSpecification{
				Name:        &name,
				Description: &desc,
			}
			if schema != nil {
				spec.InputSchema = &brtypes.ToolInputSchemaMemberJson{Value: brdoc.NewLazyDocument(schema)}
			}
			out = append(out, &brtypes.ToolMemberToolSpec{Value: spec})
		}
	}
	return out
}

// bedrockInputSchema extracts the tool's parameter schema and ensures
// it is a top-level object type (Bedrock rejects anything else).
func bedrockInputSchema(fd *genai.FunctionDeclaration) map[string]any {
	var raw map[string]any
	if fd.ParametersJsonSchema != nil {
		raw = toMapStringAny(fd.ParametersJsonSchema)
	} else if fd.Parameters != nil {
		raw = toMapStringAny(fd.Parameters)
	}
	if raw == nil {
		return nil
	}
	normalizeSchemaTypes(raw)
	if raw["type"] == nil || raw["type"] == "" {
		raw["type"] = "object"
	}
	return raw
}

// normalizeSchemaTypes lowercases the "type" field throughout a schema
// tree. genai.Schema uses uppercase enums ("OBJECT", "STRING", etc.)
// but JSON Schema (and Bedrock) requires lowercase.
func normalizeSchemaTypes(schema map[string]any) {
	if t, ok := schema["type"].(string); ok {
		schema["type"] = strings.ToLower(t)
	}
	// Recurse into properties
	if props, ok := schema["properties"].(map[string]any); ok {
		for _, v := range props {
			if sub, ok := v.(map[string]any); ok {
				normalizeSchemaTypes(sub)
			}
		}
	}
	// Recurse into items (array schemas)
	if items, ok := schema["items"].(map[string]any); ok {
		normalizeSchemaTypes(items)
	}
}

func toMapStringAny(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	}
}

func bedrockOutputToLLMResponse(out *bedrockruntime.ConverseOutput) *adkmodel.LLMResponse {
	content := &genai.Content{Role: "model"}
	if out.Output == nil {
		return &adkmodel.LLMResponse{Content: content}
	}
	msg, ok := out.Output.(*brtypes.ConverseOutputMemberMessage)
	if !ok {
		return &adkmodel.LLMResponse{Content: content}
	}
	for _, block := range msg.Value.Content {
		switch b := block.(type) {
		case *brtypes.ContentBlockMemberText:
			content.Parts = append(content.Parts, &genai.Part{Text: b.Value})
		case *brtypes.ContentBlockMemberToolUse:
			var args map[string]any
			if b.Value.Input != nil {
				if err := b.Value.Input.UnmarshalSmithyDocument(&args); err != nil {
					slog.Warn("bedrock: decode tool-call args", "tool", *b.Value.Name, "error", err)
				}
			}
			id := ""
			if b.Value.ToolUseId != nil {
				id = *b.Value.ToolUseId
			}
			name := ""
			if b.Value.Name != nil {
				name = *b.Value.Name
			}
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   id,
					Name: name,
					Args: args,
				},
			})
		}
	}
	return &adkmodel.LLMResponse{Content: content}
}
