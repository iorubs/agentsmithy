package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/iorubs/agentsmithy/internal/config"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

// defaultAnthropicKeyEnv is the conventional env var the anthropic
// provider reads when entry.APIKeyEnv is unset.
const defaultAnthropicKeyEnv = "ANTHROPIC_API_KEY"

// defaultAnthropicBaseURL is the upstream Anthropic endpoint used when entry.BaseURL is empty.
const defaultAnthropicBaseURL = "https://api.anthropic.com"

// anthropicVersion is the API version header the Messages API requires.
const anthropicVersion = "2023-06-01"

// defaultAnthropicMaxTokens applies when the entry declares none; the Messages API rejects requests without one.
const defaultAnthropicMaxTokens = 4096

// newAnthropic builds an LLM that speaks the Anthropic Messages API.
func newAnthropic(entry config.ModelEntry) (LLM, error) {
	if entry.Model == "" {
		return nil, errors.New("anthropic: model is required")
	}
	keyEnv := entry.APIKeyEnv
	if keyEnv == "" {
		keyEnv = defaultAnthropicKeyEnv
	}
	apiKey := os.Getenv(keyEnv)
	if apiKey == "" {
		return nil, errors.New("anthropic: API key not found in " + keyEnv)
	}
	base := entry.BaseURL
	if base == "" {
		base = defaultAnthropicBaseURL
	}
	return &anthropicLLM{
		entry:  entry,
		apiKey: apiKey,
		url:    strings.TrimRight(base, "/") + "/v1/messages",
		client: &http.Client{},
	}, nil
}

type anthropicLLM struct {
	entry  config.ModelEntry
	apiKey string
	url    string
	client *http.Client
}

func (m *anthropicLLM) Name() string { return m.entry.Model }

func (m *anthropicLLM) GenerateContent(
	ctx context.Context,
	req *adkmodel.LLMRequest,
	_ bool,
) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		msgs := contentsToAnthropic(req.Contents)
		if len(msgs) == 0 {
			msgs = []antMessage{{
				Role:    "user",
				Content: []antBlock{{Type: "text", Text: "Go ahead with your task."}},
			}}
		}

		maxTokens := defaultAnthropicMaxTokens
		if m.entry.MaxTokens != nil {
			maxTokens = *m.entry.MaxTokens
		}

		body, err := json.Marshal(antRequest{
			Model:       m.entry.Model,
			MaxTokens:   maxTokens,
			Temperature: m.entry.Temperature,
			System:      systemText(req.Config),
			Messages:    msgs,
			Tools:       antToolsFromConfig(req.Config),
		})
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: marshal request: %w", err))
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.url, bytes.NewReader(body))
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: build request: %w", err))
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-Api-Key", m.apiKey)
		httpReq.Header.Set("Anthropic-Version", anthropicVersion)

		slog.DebugContext(ctx, "anthropic request",
			"url", m.url, "model", m.entry.Model, "messages", len(msgs))

		resp, err := m.client.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: request: %w", err))
			return
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: read response: %w", err))
			return
		}
		if resp.StatusCode != http.StatusOK {
			yield(nil, fmt.Errorf("anthropic: %s %d: %s", m.url, resp.StatusCode, data))
			return
		}

		var antResp antResponse
		if err := json.Unmarshal(data, &antResp); err != nil {
			yield(nil, fmt.Errorf("anthropic: unmarshal response: %w", err))
			return
		}
		if len(antResp.Content) == 0 {
			yield(nil, errors.New("anthropic: empty response from model"))
			return
		}
		yield(antResponseToLLMResponse(antResp), nil)
	}
}

type antRequest struct {
	Model       string       `json:"model"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature *float64     `json:"temperature,omitempty"`
	System      string       `json:"system,omitempty"`
	Messages    []antMessage `json:"messages"`
	Tools       []antTool    `json:"tools,omitempty"`
}

type antMessage struct {
	Role    string     `json:"role"`
	Content []antBlock `json:"content"`
}

type antBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type antTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type antResponse struct {
	Content []antBlock `json:"content"`
	Model   string     `json:"model"`
}

func contentsToAnthropic(contents []*genai.Content) []antMessage {
	var msgs []antMessage
	for _, c := range contents {
		msgs = append(msgs, contentToAnthropic(c)...)
	}
	return msgs
}

func contentToAnthropic(c *genai.Content) []antMessage {
	if c == nil {
		return nil
	}
	var blocks, toolResults []antBlock

	for _, p := range c.Parts {
		switch {
		case p == nil:
			continue
		case p.Text != "":
			blocks = append(blocks, antBlock{Type: "text", Text: p.Text})
		case p.FunctionCall != nil:
			fc := p.FunctionCall
			id := fc.ID
			if id == "" {
				id = fc.Name
			}
			args := fc.Args
			if args == nil {
				args = map[string]any{}
			}
			blocks = append(blocks, antBlock{Type: "tool_use", ID: id, Name: fc.Name, Input: args})
		case p.FunctionResponse != nil:
			fr := p.FunctionResponse
			id := fr.ID
			if id == "" {
				id = fr.Name
			}
			respJSON, err := json.Marshal(fr.Response)
			if err != nil {
				slog.Warn("anthropic: marshal tool-response", "tool", fr.Name, "error", err)
				respJSON = []byte("{}")
			}
			toolResults = append(toolResults, antBlock{
				Type:      "tool_result",
				ToolUseID: id,
				Content:   string(respJSON),
			})
		}
	}

	if len(toolResults) > 0 {
		return []antMessage{{Role: "user", Content: toolResults}}
	}
	if len(blocks) == 0 {
		return nil
	}

	role := "user"
	if c.Role == "model" || c.Role == "assistant" {
		role = "assistant"
	}
	return []antMessage{{Role: role, Content: blocks}}
}

func antToolsFromConfig(cfg *genai.GenerateContentConfig) []antTool {
	if cfg == nil {
		return nil
	}
	var out []antTool
	for _, t := range cfg.Tools {
		for _, fd := range t.FunctionDeclarations {
			schema := jsonSchemaFor(fd)
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			out = append(out, antTool{
				Name:        fd.Name,
				Description: fd.Description,
				InputSchema: schema,
			})
		}
	}
	return out
}

func antResponseToLLMResponse(resp antResponse) *adkmodel.LLMResponse {
	content := &genai.Content{Role: "model"}
	for _, b := range resp.Content {
		switch b.Type {
		case "text":
			content.Parts = append(content.Parts, &genai.Part{Text: b.Text})
		case "tool_use":
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{ID: b.ID, Name: b.Name, Args: b.Input},
			})
		}
	}
	return &adkmodel.LLMResponse{Content: content, ModelVersion: resp.Model}
}
