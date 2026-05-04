package models

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"sync"
	"time"

	"github.com/iorubs/agentsmithy/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

// Safety-rail defaults. Tuned for interactive MCP clients (VS Code
// Copilot, Claude Desktop) where a single turn typically fires a
// small handful of sampling requests.
const (
	// borrowedMaxDepth caps reentrant sampling: if a borrowed response
	// drives a tool call that re-enters GenerateContent, the chain is
	// bounded to this many nested hops before erroring.
	borrowedMaxDepth = 3
	// borrowedMaxInflight caps concurrent sampling requests per MCP
	// session. The MCP client itself usually serialises user approvals,
	// so 1 is a safe default.
	borrowedMaxInflight = 1
	// borrowedRPMPerSession caps requests per rolling 60s window per
	// session, as a cheap guard against runaway loops.
	borrowedRPMPerSession = 60
)

// borrowedCtxKey carries the connecting MCP server session. The
// mcp-stdio transport sets it before invoking the agent so the
// borrowed LLM can route GenerateContent back to the client via
// sampling/createMessage.
type borrowedCtxKey struct{}

// borrowedDepthKey carries the current reentrancy depth for sampling.
type borrowedDepthKey struct{}

// WithSession attaches an MCP server session to ctx so the borrowed
// provider can route GenerateContent back to the connected client.
// Set by the mcp-stdio transport.
func WithSession(ctx context.Context, ss *mcp.ServerSession) context.Context {
	return context.WithValue(ctx, borrowedCtxKey{}, ss)
}

func sessionFromContext(ctx context.Context) *mcp.ServerSession {
	ss, _ := ctx.Value(borrowedCtxKey{}).(*mcp.ServerSession)
	return ss
}

// newBorrowed builds an LLM that delegates completion to the
// connecting MCP client via sampling/createMessage. entry.Model is
// optional; when set it becomes a modelPreferences hint, when empty
// the client picks freely. entry.MaxTokens is required (>=1) so the
// host-side LLM cannot run unbounded.
func newBorrowed(entry config.ModelEntry) (LLM, error) {
	if entry.MaxTokens == nil || *entry.MaxTokens < 1 {
		return nil, errors.New(
			"borrowed: maxTokens is required (>=1) to cap per-call token usage")
	}
	return &borrowedLLM{entry: entry}, nil
}

type borrowedLLM struct {
	entry config.ModelEntry
}

func (m *borrowedLLM) Name() string {
	if m.entry.Model == "" {
		return "borrowed"
	}
	return "borrowed:" + m.entry.Model
}

func (m *borrowedLLM) GenerateContent(
	ctx context.Context,
	req *adkmodel.LLMRequest,
	_ bool,
) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		ss := sessionFromContext(ctx)
		if ss == nil {
			yield(nil, fmt.Errorf(
				"borrowed (%s): no MCP session in context; requires serve.transport mcp-stdio",
				m.Name()))
			return
		}

		// Depth cap: bound reentrancy if a borrowed response triggers
		// a local tool that re-enters sampling.
		depth, _ := ctx.Value(borrowedDepthKey{}).(int)
		if depth >= borrowedMaxDepth {
			yield(nil, fmt.Errorf(
				"borrowed: sampling reentrancy depth cap (%d) exceeded",
				borrowedMaxDepth))
			return
		}
		ctx = context.WithValue(ctx, borrowedDepthKey{}, depth+1)

		// Capability assertion: fails loudly if a stray
		// GenerateContent comes from a non-mcp path.
		init := ss.InitializeParams()
		if init == nil || init.Capabilities == nil || init.Capabilities.Sampling == nil {
			yield(nil, errors.New(
				"borrowed: connected MCP client did not advertise 'sampling' capability"))
			return
		}

		gate := gateFor(ss)
		if err := gate.acquire(ctx); err != nil {
			yield(nil, err)
			return
		}
		defer gate.release()

		var sysPrompt string
		if req.Config != nil && req.Config.SystemInstruction != nil {
			for _, p := range req.Config.SystemInstruction.Parts {
				if p != nil && p.Text != "" {
					sysPrompt += p.Text
				}
			}
		}
		var temp float64
		if m.entry.Temperature != nil {
			temp = *m.entry.Temperature
		}

		params := &mcp.CreateMessageParams{
			MaxTokens:    int64(*m.entry.MaxTokens),
			Messages:     contentsToSamplingMessages(req.Contents),
			SystemPrompt: sysPrompt,
			Temperature:  temp,
		}
		if m.entry.Model != "" {
			params.ModelPreferences = &mcp.ModelPreferences{
				Hints: []*mcp.ModelHint{{Name: m.entry.Model}},
			}
		}

		slog.DebugContext(ctx, "borrowed request",
			"model", m.entry.Model,
			"messages", len(params.Messages),
			"maxTokens", params.MaxTokens,
		)

		res, err := ss.CreateMessage(ctx, params)
		if err != nil {
			yield(nil, fmt.Errorf("borrowed: sampling/createMessage: %w", err))
			return
		}

		yield(&adkmodel.LLMResponse{
			Content: &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: contentToText(res.Content)}},
			},
			ModelVersion: res.Model,
		}, nil)
	}
}

// contentsToSamplingMessages flattens ADK Contents into MCP sampling
// messages, preserving role. Tool-call / tool-response parts are
// dropped; borrowed forwards no local tool state to the remote
// model in v0.1 (CreateMessageWithTools is a follow-up).
func contentsToSamplingMessages(contents []*genai.Content) []*mcp.SamplingMessage {
	var out []*mcp.SamplingMessage
	for _, c := range contents {
		if c == nil {
			continue
		}
		var role mcp.Role
		switch c.Role {
		case "user":
			role = "user"
		case "model", "assistant":
			role = "assistant"
		default:
			role = "user"
		}
		var text string
		for _, p := range c.Parts {
			if p != nil && p.Text != "" {
				text += p.Text
			}
		}
		if text == "" {
			continue
		}
		out = append(out, &mcp.SamplingMessage{
			Role:    role,
			Content: &mcp.TextContent{Text: text},
		})
	}
	return out
}

func contentToText(c mcp.Content) string {
	if tc, ok := c.(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// sessionGate enforces per-session in-flight and rolling-window rate
// limits. Both caps are applied before each CreateMessage call.
type sessionGate struct {
	inflight chan struct{}
	mu       sync.Mutex
	recent   []time.Time
}

var (
	gatesMu sync.Mutex
	gates   = map[*mcp.ServerSession]*sessionGate{}
)

func gateFor(ss *mcp.ServerSession) *sessionGate {
	gatesMu.Lock()
	defer gatesMu.Unlock()
	if g, ok := gates[ss]; ok {
		return g
	}
	g := &sessionGate{inflight: make(chan struct{}, borrowedMaxInflight)}
	gates[ss] = g
	return g
}

func (g *sessionGate) acquire(ctx context.Context) error {
	g.mu.Lock()
	cutoff := time.Now().Add(-time.Minute)
	kept := g.recent[:0]
	for _, t := range g.recent {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= borrowedRPMPerSession {
		g.recent = kept
		g.mu.Unlock()
		return fmt.Errorf(
			"borrowed: session rate limit exceeded (>%d requests/min)",
			borrowedRPMPerSession)
	}
	g.recent = append(kept, time.Now())
	g.mu.Unlock()

	select {
	case g.inflight <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *sessionGate) release() { <-g.inflight }
