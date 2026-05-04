package kinds

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/toolconfirmation"
)

// toolHelper backs the {{ tool "name" args }} template helper. It
// resolves the tool by name (from the orchestrator's tools: list,
// expanding MCP toolsets lazily on first reference), invokes it via
// the unexported Run shape every ADK tool kind implements, and
// returns the result as a string.
//
// Argument shaping follows agenttool's convention: a single map
// argument is passed through; a single non-map argument lands under
// {"request": arg}; multiple args fail with a hint to use dict.
func toolHelper(ictx agent.InvocationContext, selfName string, tr *toolResolver) func(string, ...any) (string, error) {
	return func(name string, args ...any) (string, error) {
		tl, err := tr.resolve(name)
		if err != nil {
			return "", fmt.Errorf("agent %q: %w", selfName, err)
		}
		runner, ok := tl.(runnableTool)
		if !ok {
			return "", fmt.Errorf("agent %q: tool %q: not directly runnable (kind not supported in v0.1)", selfName, name)
		}
		argMap, err := argsToMap(args)
		if err != nil {
			return "", fmt.Errorf("agent %q: tool %q: %w", selfName, name, err)
		}
		out, err := runner.Run(newToolCtx(ictx), argMap)
		if err != nil {
			return "", fmt.Errorf("agent %q: tool %q: %w", selfName, name, err)
		}
		return coerceToolOutput(out), nil
	}
}

// runnableTool mirrors ADK's unexported runnable shape. functiontool,
// agenttool, and mcptoolset entries all satisfy it. Type-asserting
// against this is the only way to call a tool from outside the LLM
// flow in ADK v1.1.0; Run is not on the public tool.Tool interface.
type runnableTool interface {
	Run(adktool.Context, any) (map[string]any, error)
}

// toolResolver caches lookups across a single template render. The
// tools map is the directly-provided slice indexed by name; sets
// are MCP toolsets expanded on first miss.
type toolResolver struct {
	ctx      agent.InvocationContext
	tools    map[string]adktool.Tool
	sets     []adktool.Toolset
	cache    map[string]adktool.Tool
	expanded bool
}

func (r *toolResolver) resolve(name string) (adktool.Tool, error) {
	if t, ok := r.tools[name]; ok {
		return t, nil
	}
	if t, ok := r.cache[name]; ok {
		return t, nil
	}
	if !r.expanded {
		r.expanded = true
		rctx := newToolCtx(r.ctx)
		for _, ts := range r.sets {
			tools, err := ts.Tools(rctx)
			if err != nil {
				return nil, fmt.Errorf("toolset %q: %w", ts.Name(), err)
			}
			for _, t := range tools {
				r.cache[t.Name()] = t
			}
		}
	}
	if t, ok := r.cache[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("tool %q: not declared in this agent's tools:", name)
}

// argsToMap shapes a {{ tool "name" ... }} call's tail args into the
// map[string]any that ADK tools expect. A single map argument is
// passed through. A single non-map argument lands under
// {"request": arg}, matching agenttool's convention. Empty args
// yield an empty map. Multiple args fail with a hint to use dict.
func argsToMap(args []any) (map[string]any, error) {
	switch len(args) {
	case 0:
		return map[string]any{}, nil
	case 1:
		if m, ok := args[0].(map[string]any); ok {
			return m, nil
		}
		return map[string]any{"request": args[0]}, nil
	default:
		return nil, fmt.Errorf("expected 0 or 1 argument, got %d (use dict to pass multiple fields)", len(args))
	}
}

// coerceToolOutput reduces ADK's map[string]any return to a string.
// The common single-"result"-string shape unwraps; anything else
// JSON-marshals so the template body sees deterministic output.
func coerceToolOutput(out map[string]any) string {
	if len(out) == 0 {
		return ""
	}
	if len(out) == 1 {
		if v, ok := out["result"]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Sprint(out)
	}
	return string(b)
}

// toolCtx is a minimal tool.Context shim. ADK's NewToolContext lives
// under internal/toolinternal, so we wrap an InvocationContext and
// supply the tool.Context-only methods with sensible no-ops.
// Confirmations are not supported; tools that require HITL approval
// fail closed via ErrConfirmationRequired.
type toolCtx struct {
	agent.InvocationContext
	actions *session.EventActions
}

func newToolCtx(ictx agent.InvocationContext) *toolCtx {
	return &toolCtx{
		InvocationContext: ictx,
		actions: &session.EventActions{
			StateDelta:    map[string]any{},
			ArtifactDelta: map[string]int64{},
		},
	}
}

func (t *toolCtx) AgentName() string                    { return t.Agent().Name() }
func (t *toolCtx) ReadonlyState() session.ReadonlyState { return t.Session().State() }
func (t *toolCtx) UserID() string                       { return t.Session().UserID() }
func (t *toolCtx) AppName() string                      { return t.Session().AppName() }
func (t *toolCtx) SessionID() string                    { return t.Session().ID() }
func (t *toolCtx) State() session.State                 { return t.Session().State() }
func (t *toolCtx) FunctionCallID() string               { return "" }
func (t *toolCtx) Actions() *session.EventActions       { return t.actions }
func (t *toolCtx) SearchMemory(ctx context.Context, q string) (*memory.SearchResponse, error) {
	if m := t.InvocationContext.Memory(); m != nil {
		return m.SearchMemory(ctx, q)
	}
	return &memory.SearchResponse{}, nil
}
func (t *toolCtx) ToolConfirmation() *toolconfirmation.ToolConfirmation { return nil }
func (t *toolCtx) RequestConfirmation(string, any) error                { return nil }
