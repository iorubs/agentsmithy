package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/iorubs/agentsmithy/internal/pipeline"
	"github.com/iorubs/agentsmithy/internal/pipeline/obs"
	"github.com/iorubs/agentsmithy/internal/project/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// MCPStdio exposes the pipeline's root agent as an MCP tool over
// stdin/stdout. Each tool invocation drives one agent turn and returns
// the final text. When the pipeline declares any borrowed model, the
// connecting client must advertise the `sampling` capability;
// completions for those models round-trip back via
// sampling/createMessage.
func MCPStdio(ctx context.Context, p *pipeline.Pipeline, requireSampling bool) error {
	srv, toolName, err := buildMCPServer(p, requireSampling)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "MCP stdio server starting",
		"name", p.Name,
		"tool", toolName,
		"requireSampling", requireSampling,
	)
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// invokeInput is the JSON schema the exposed tool advertises.
type invokeInput struct {
	Prompt string `json:"prompt" jsonschema:"the user prompt to send to the agent"`
}

// invokeOutput is the structured result returned to MCP clients that
// consume structured output. The same text is also surfaced via
// CallToolResult.Content.
type invokeOutput struct {
	Reply string `json:"reply"`
}

// buildMCPServer wires an *mcp.Server with one agent-invocation tool.
// Shared between the stdio and HTTP transports.
func buildMCPServer(p *pipeline.Pipeline, requireSampling bool) (*mcp.Server, string, error) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    p.Name,
		Version: "1.0.0",
	}, nil)

	r, err := runner.New(runner.Config{
		AppName:           p.Name,
		Agent:             p.Root,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("runner: %w", err)
	}

	toolName := sanitizeToolName(p.Name)
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        toolName,
			Description: fmt.Sprintf("Invoke the %s pipeline.", p.Name),
		},
		func(ctx context.Context, req *mcp.CallToolRequest, in invokeInput) (*mcp.CallToolResult, invokeOutput, error) {
			if strings.TrimSpace(in.Prompt) == "" {
				return nil, invokeOutput{}, fmt.Errorf("prompt is required")
			}
			ss := req.Session

			if requireSampling {
				init := ss.InitializeParams()
				if init == nil || init.Capabilities == nil || init.Capabilities.Sampling == nil {
					return nil, invokeOutput{}, fmt.Errorf(
						"this agent uses provider: borrowed but the connected MCP client did not advertise 'sampling' capability")
				}
			}

			ctx = models.WithSession(ctx, ss)

			sessionID := fmt.Sprintf("mcp-%d", time.Now().UnixNano())
			userID := "mcp-" + sessionID
			msg := &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: in.Prompt}},
			}
			var final strings.Builder
			for ev, err := range r.Run(ctx, userID, sessionID, msg, agent.RunConfig{}) {
				if err != nil {
					return nil, invokeOutput{}, err
				}
				if !obs.EventIsFinal(ev) {
					continue
				}
				for _, part := range ev.LLMResponse.Content.Parts {
					if part != nil && part.Text != "" {
						final.WriteString(part.Text)
					}
				}
			}
			reply := strings.TrimSpace(final.String())
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: reply}},
			}, invokeOutput{Reply: reply}, nil
		})

	return srv, toolName, nil
}

// sanitizeToolName coerces an arbitrary project name into a valid MCP
// tool identifier (alphanumeric plus `_`/`-`). Empty input falls back
// to "agent" so the server always advertises something callable.
func sanitizeToolName(s string) string {
	if s == "" {
		return "agent"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "agent"
	}
	return b.String()
}
