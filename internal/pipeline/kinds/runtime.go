package kinds

import (
	"context"
	"fmt"

	"github.com/iorubs/agentsmithy/internal/pipeline/tmpl"
	"github.com/iorubs/agentsmithy/internal/project/models"
	"google.golang.org/adk/agent"
	adktool "google.golang.org/adk/tool"
)

// callbackRuntime is the helper set bound to an AfterAgentCallback
// (sequential, parallel, loop output:). prompt and skill are live;
// tool and agent error explicitly because the callback context lacks
// the InvocationContext needed to invoke sub-agents or build a
// tool.Context. Authors who need helper composition use kind:
// orchestrator.
func callbackRuntime(ctx context.Context, selfName string, llm models.LLM, sk map[string]SkillHelper) tmpl.RuntimeFuncs {
	return tmpl.RuntimeFuncs{
		"tool":       unsupportedHelper("tool", selfName, "output: of sequential/parallel/loop"),
		"agent":      unsupportedHelper("agent", selfName, "output: of sequential/parallel/loop"),
		"skill":      skillDispatch(ctx, selfName, sk),
		"prompt":     promptHelper(ctx, selfName, llm),
		"exit_error": tmpl.ExitErrorFunc,
	}
}

// untilRuntime is the helper set bound to a loop until: predicate.
// agent and tool are forbidden here; skill and prompt are live so
// authors can ask the model to judge a stop condition or call
// deterministic skills like file or shell to gate progress.
func untilRuntime(ctx context.Context, selfName string, llm models.LLM, sk map[string]SkillHelper) tmpl.RuntimeFuncs {
	return tmpl.RuntimeFuncs{
		"tool":       unsupportedHelper("tool", selfName, "until: predicate"),
		"agent":      unsupportedHelper("agent", selfName, "until: predicate"),
		"skill":      skillDispatch(ctx, selfName, sk),
		"prompt":     promptHelper(ctx, selfName, llm),
		"exit_error": tmpl.ExitErrorFunc,
	}
}

// orchestratorRuntime is the full helper set bound to an
// orchestrator's custom Run. tool, agent, prompt, and skill all
// resolve through the orchestrator's invocation context.
func orchestratorRuntime(
	ictx agent.InvocationContext,
	selfName string,
	llm models.LLM,
	subs map[string]agent.Agent,
	tools map[string]adktool.Tool,
	sets []adktool.Toolset,
	sk map[string]SkillHelper,
) tmpl.RuntimeFuncs {
	tr := &toolResolver{ctx: ictx, tools: tools, sets: sets, cache: map[string]adktool.Tool{}}
	return tmpl.RuntimeFuncs{
		"prompt":     promptHelper(ictx, selfName, llm),
		"tool":       toolHelper(ictx, selfName, tr),
		"agent":      agentHelper(ictx, selfName, subs),
		"skill":      skillDispatch(ictx, selfName, sk),
		"exit_error": tmpl.ExitErrorFunc,
	}
}

func unsupportedHelper(helper, selfName, where string) func(string, ...any) (string, error) {
	return func(name string, _ ...any) (string, error) {
		return "", fmt.Errorf("agent %q: %s %q: not callable from %s (use kind: orchestrator)", selfName, helper, name, where)
	}
}

// skillDispatch returns a `skill` template helper that resolves names
// against the per-node skills map. Unknown names error explicitly.
// An empty/nil map means no skills are declared for this node.
func skillDispatch(ctx context.Context, selfName string, sk map[string]SkillHelper) func(string, ...any) (any, error) {
	return func(name string, args ...any) (any, error) {
		if len(sk) == 0 {
			return nil, fmt.Errorf("agent %q: skill %q: no skills declared", selfName, name)
		}
		h, ok := sk[name]
		if !ok {
			return nil, fmt.Errorf("agent %q: skill %q: not declared", selfName, name)
		}
		return h(ctx, args...)
	}
}
