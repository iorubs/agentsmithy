// Package obs holds reusable observability hooks shared by all agent
// kinds. Exposes ADK callback constructors that emit debug logs at
// model and tool boundaries; later kinds (sequential, parallel,
// loop, orchestrator) reuse these directly.
package obs

import (
	"log/slog"
	"sort"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	adktool "google.golang.org/adk/tool"
)

// Callbacks returns ADK callback functions that log model and tool
// boundary events at debug level. agentName is included on every
// record so dashboards can group by agent.
func Callbacks(agentName string) (
	llmagent.BeforeModelCallback,
	llmagent.AfterModelCallback,
	llmagent.BeforeToolCallback,
	llmagent.AfterToolCallback,
) {
	beforeModel := func(ctx adkagent.CallbackContext, _ *adkmodel.LLMRequest) (*adkmodel.LLMResponse, error) {
		slog.DebugContext(ctx, "model call", "agent", agentName)
		return nil, nil
	}
	afterModel := func(ctx adkagent.CallbackContext, resp *adkmodel.LLMResponse, err error) (*adkmodel.LLMResponse, error) {
		attrs := []any{"agent", agentName}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}
		if resp != nil {
			attrs = append(attrs, "tool_calls", countToolCalls(resp))
		}
		slog.DebugContext(ctx, "model done", attrs...)
		return nil, nil
	}
	beforeTool := func(ctx adktool.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
		slog.DebugContext(ctx, "tool call",
			"agent", agentName,
			"tool", t.Name(),
			"args", argKeys(args),
		)
		return nil, nil
	}
	afterTool := func(ctx adktool.Context, t adktool.Tool, _ map[string]any, _ map[string]any, err error) (map[string]any, error) {
		attrs := []any{"agent", agentName, "tool", t.Name()}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}
		slog.DebugContext(ctx, "tool done", attrs...)
		return nil, nil
	}
	return beforeModel, afterModel, beforeTool, afterTool
}

func argKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func countToolCalls(resp *adkmodel.LLMResponse) int {
	if resp == nil || resp.Content == nil {
		return 0
	}
	n := 0
	for _, p := range resp.Content.Parts {
		if p != nil && p.FunctionCall != nil {
			n++
		}
	}
	return n
}
