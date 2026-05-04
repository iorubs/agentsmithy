// Package guards implements response guards for autonomous agents.
//
// A guard inspects each model response and either accepts it or
// returns a corrective message that is fed back to the model as the
// next response, prompting a retry. Retries are capped per-guard by
// MaxRetries (defaults to 3).
//
// Guards are wired through an AfterModel callback built by Callback;
// the callback short-circuits the agent loop by returning the
// corrective LLMResponse, which ADK then treats as the model's reply
// for the current turn and runs the next iteration.
package guards

import (
	"fmt"
	"log/slog"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// ResponseGuard checks a model response. Check returns nil if the
// response passes; a non-nil pointer carries the corrective message
// the model should see on retry.
type ResponseGuard struct {
	Name  string
	Check func(resp *model.LLMResponse) *string
}

// Build lowers a config-level guard list into runnable ResponseGuard
// values. Unknown names error rather than silently dropping.
func Build(list []v1.Guard) ([]ResponseGuard, error) {
	if len(list) == 0 {
		return nil, nil
	}
	out := make([]ResponseGuard, 0, len(list))
	for _, g := range list {
		switch g {
		case v1.GuardRequireToolCall:
			out = append(out, RequireToolCall())
		default:
			return nil, fmt.Errorf("unknown guard %q", g)
		}
	}
	return out, nil
}

// RequireToolCall fails if the model responded without calling at
// least one function. The corrective message instructs the model to
// call a tool first.
func RequireToolCall() ResponseGuard {
	return ResponseGuard{
		Name: "requireToolCall",
		Check: func(resp *model.LLMResponse) *string {
			if resp == nil || resp.Content == nil {
				return nil
			}
			for _, p := range resp.Content.Parts {
				if p != nil && p.FunctionCall != nil {
					return nil
				}
			}
			msg := "You must call a tool before answering. Use one of the available tools first, then base your answer on its output."
			return &msg
		},
	}
}

// Callback returns an AfterModelCallback that enforces the given
// guards. maxRetries caps the per-guard corrective-message budget;
// 0 means use the default of 3.
//
// The requireToolCall guard is special-cased: once any tool has been
// called in this session, the guard stops firing so the follow-up
// text response (the model summarising tool results) is allowed
// through.
func Callback(list []ResponseGuard, maxRetries int) llmagent.AfterModelCallback {
	if len(list) == 0 {
		return nil
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retries := make([]int, len(list))
	const toolCalledKey = "_guard_tool_called"

	return func(ctx agent.CallbackContext, resp *model.LLMResponse, respErr error) (*model.LLMResponse, error) {
		if respErr != nil {
			return nil, nil
		}
		if resp != nil && resp.Content != nil {
			for _, p := range resp.Content.Parts {
				if p != nil && p.FunctionCall != nil {
					_ = ctx.State().Set(toolCalledKey, "1")
					break
				}
			}
		}
		for i, g := range list {
			if g.Name == "requireToolCall" {
				if v, _ := ctx.State().Get(toolCalledKey); v == "1" {
					continue
				}
			}
			if retries[i] >= maxRetries {
				continue
			}
			correction := g.Check(resp)
			if correction == nil {
				retries[i] = 0
				continue
			}
			retries[i]++
			slog.Warn("guard retry",
				"guard", g.Name,
				"attempt", retries[i],
				"max", maxRetries,
				"reason", *correction,
			)
			return &model.LLMResponse{
				Content: &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: *correction}},
				},
			}, nil
		}
		for i := range retries {
			retries[i] = 0
		}
		return nil, nil
	}
}
