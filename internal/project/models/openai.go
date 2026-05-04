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

// defaultOpenAIKeyEnv is the conventional env var the openai provider
// reads when entry.APIKeyEnv is unset. Local OpenAI-compatible servers
// (Ollama, LM Studio) typically need no key, so an empty value is fine.
const defaultOpenAIKeyEnv = "OPENAI_API_KEY"

// defaultOpenAIBaseURL is the upstream OpenAI endpoint used when
// entry.BaseURL is empty. Authors targeting Ollama / LM Studio /
// vLLM / Together / Groq override BaseURL on the entry.
const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// newOpenAI builds an LLM that speaks the OpenAI Chat Completions wire via entry.BaseURL
// (covers Ollama, LM Studio, vLLM, Together, Groq, OpenRouter, DeepSeek, xAI).
func newOpenAI(entry config.ModelEntry) (LLM, error) {
	if entry.Model == "" {
		return nil, errors.New("openai: model is required")
	}
	keyEnv := entry.APIKeyEnv
	if keyEnv == "" {
		keyEnv = defaultOpenAIKeyEnv
	}
	base := entry.BaseURL
	if base == "" {
		base = defaultOpenAIBaseURL
	}
	return &openaiLLM{
		entry:  entry,
		apiKey: os.Getenv(keyEnv),
		url:    strings.TrimRight(base, "/") + "/chat/completions",
		client: &http.Client{},
	}, nil
}

type openaiLLM struct {
	entry  config.ModelEntry
	apiKey string
	url    string
	client *http.Client
}

func (m *openaiLLM) Name() string { return m.entry.Model }

func (m *openaiLLM) GenerateContent(
	ctx context.Context,
	req *adkmodel.LLMRequest,
	_ bool,
) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		msgs := contentsToOAI(req.Contents)
		tools := toolsFromConfig(req.Config)

		if req.Config != nil && req.Config.SystemInstruction != nil {
			var sysText string
			for _, p := range req.Config.SystemInstruction.Parts {
				if p != nil && p.Text != "" {
					sysText += p.Text
				}
			}
			if sysText != "" {
				msgs = append([]oaiMessage{{Role: "system", Content: sysText}}, msgs...)
			}
		}

		// Some servers reject conversations that contain no user turn.
		hasUser := false
		for _, msg := range msgs {
			if msg.Role == "user" {
				hasUser = true
				break
			}
		}
		if !hasUser {
			msgs = append(msgs, oaiMessage{Role: "user", Content: "Go ahead with your task."})
		}

		body, err := json.Marshal(oaiRequest{
			Model:    m.entry.Model,
			Messages: msgs,
			Tools:    tools,
			Stream:   false,
		})
		if err != nil {
			yield(nil, fmt.Errorf("openai: marshal request: %w", err))
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.url, bytes.NewReader(body))
		if err != nil {
			yield(nil, fmt.Errorf("openai: build request: %w", err))
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if m.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
		}

		slog.DebugContext(ctx, "openai request",
			"url", m.url, "model", m.entry.Model,
			"messages", len(msgs), "tools", len(tools))

		resp, err := m.client.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("openai: request: %w", err))
			return
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			yield(nil, fmt.Errorf("openai: read response: %w", err))
			return
		}
		if resp.StatusCode != http.StatusOK {
			yield(nil, fmt.Errorf("openai: %s %d: %s", m.url, resp.StatusCode, data))
			return
		}

		var oaiResp oaiResponse
		if err := json.Unmarshal(data, &oaiResp); err != nil {
			yield(nil, fmt.Errorf("openai: unmarshal response: %w", err))
			return
		}
		if len(oaiResp.Choices) == 0 {
			yield(nil, errors.New("openai: empty response from model"))
			return
		}
		yield(oaiChoiceToLLMResponse(oaiResp.Choices[0]), nil)
	}
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaiTool struct {
	Type     string         `json:"type"`
	Function oaiFunctionDef `json:"function"`
}

type oaiFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []oaiTool    `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
}

type oaiChoice struct {
	Message      oaiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

func contentsToOAI(contents []*genai.Content) []oaiMessage {
	var msgs []oaiMessage
	for _, c := range contents {
		msgs = append(msgs, contentToOAI(c)...)
	}
	return msgs
}

func contentToOAI(c *genai.Content) []oaiMessage {
	if c == nil {
		return nil
	}
	var textParts []string
	var toolCalls []oaiToolCall
	var toolResults []oaiMessage

	for _, p := range c.Parts {
		switch {
		case p == nil:
			continue
		case p.Text != "":
			textParts = append(textParts, p.Text)
		case p.FunctionCall != nil:
			fc := p.FunctionCall
			id := fc.ID
			if id == "" {
				id = fc.Name
			}
			argsJSON, err := json.Marshal(fc.Args)
			if err != nil {
				slog.Warn("openai: marshal tool-call args", "tool", fc.Name, "error", err)
				argsJSON = []byte("{}")
			}
			toolCalls = append(toolCalls, oaiToolCall{
				ID:       id,
				Type:     "function",
				Function: oaiFunction{Name: fc.Name, Arguments: string(argsJSON)},
			})
		case p.FunctionResponse != nil:
			fr := p.FunctionResponse
			id := fr.ID
			if id == "" {
				id = fr.Name
			}
			respJSON, err := json.Marshal(fr.Response)
			if err != nil {
				slog.Warn("openai: marshal tool-response", "tool", fr.Name, "error", err)
				respJSON = []byte("{}")
			}
			toolResults = append(toolResults, oaiMessage{
				Role:       "tool",
				Content:    string(respJSON),
				ToolCallID: id,
			})
		}
	}

	if len(toolResults) > 0 {
		return toolResults
	}

	role := c.Role
	if role == "model" {
		role = "assistant"
	}
	return []oaiMessage{{
		Role:      role,
		Content:   strings.Join(textParts, ""),
		ToolCalls: toolCalls,
	}}
}

func toolsFromConfig(cfg *genai.GenerateContentConfig) []oaiTool {
	if cfg == nil {
		return nil
	}
	var out []oaiTool
	for _, t := range cfg.Tools {
		for _, fd := range t.FunctionDeclarations {
			var params map[string]any
			// Newer ADK/MCP toolsets set ParametersJsonSchema (already
			// JSON Schema-shaped); older genai.Schema lives in Parameters.
			if fd.ParametersJsonSchema != nil {
				if data, err := json.Marshal(fd.ParametersJsonSchema); err == nil {
					if err := json.Unmarshal(data, &params); err != nil {
						slog.Warn("openai: decode tool params (json schema)", "tool", fd.Name, "error", err)
						params = nil
					}
				}
			}
			if params == nil && fd.Parameters != nil {
				if data, err := json.Marshal(fd.Parameters); err == nil {
					if err := json.Unmarshal(data, &params); err != nil {
						slog.Warn("openai: decode tool params (genai schema)", "tool", fd.Name, "error", err)
						params = nil
					}
				}
			}
			out = append(out, oaiTool{
				Type: "function",
				Function: oaiFunctionDef{
					Name:        fd.Name,
					Description: fd.Description,
					Parameters:  params,
				},
			})
		}
	}
	return out
}

func oaiChoiceToLLMResponse(choice oaiChoice) *adkmodel.LLMResponse {
	content := &genai.Content{Role: "model"}
	msg := choice.Message
	if msg.Content != "" {
		content.Parts = append(content.Parts, &genai.Part{Text: msg.Content})
	}
	for _, tc := range msg.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			slog.Warn("openai: decode tool-call args", "tool", tc.Function.Name, "error", err)
		}
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			},
		})
	}
	return &adkmodel.LLMResponse{Content: content}
}
