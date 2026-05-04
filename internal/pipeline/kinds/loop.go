package kinds

import (
	"fmt"
	"iter"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/tmpl"
	"github.com/iorubs/agentsmithy/internal/project/models"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/session"
)

func newLoop(l *v1.Loop, deps Deps) (Agent, error) {
	if len(deps.Subagents) == 0 {
		return nil, fmt.Errorf("agent %q: loop requires at least one subagent", deps.Name)
	}
	names := childNames(deps.Subagents)
	subs := deps.Subagents
	if l.Until != "" {
		checker, err := newUntilChecker(deps.Name, string(l.Until), names, deps.LLM, deps.Skills)
		if err != nil {
			return nil, err
		}
		subs = append(append([]Agent{}, deps.Subagents...), checker)
	}
	cfg := loopagent.Config{
		MaxIterations: uint(l.MaxIterations),
		AgentConfig: agent.Config{
			Name:        deps.Name,
			Description: deps.Instruction,
			SubAgents:   subs,
		},
	}
	if cb := outputCallback(deps.Name, l.Output, names, deps.LLM, deps.Skills); cb != nil {
		cfg.AgentConfig.AfterAgentCallbacks = []agent.AfterAgentCallback{cb}
	}
	return loopagent.New(cfg)
}

// newUntilChecker builds a synthetic sub-agent that runs after each
// loop pass. It renders the until: predicate against the current
// child outputs in session state; if truthy it yields an event with
// Actions.Escalate set, which loopagent observes to break the loop.
// The checker's name is namespaced under the loop's name to avoid
// colliding with any user-declared sub-agent (notreserved guards
// the user's names already).
func newUntilChecker(loopName, body string, names []string, llm models.LLM, sk map[string]SkillHelper) (Agent, error) {
	checkerName := loopName + ".until"
	return agent.New(agent.Config{
		Name: checkerName,
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				scope := childOutputScope(ctx.Session().State(), names)
				scope["input"] = userInputText(ctx.UserContent())
				rendered, err := tmpl.Render(body, scope, untilRuntime(ctx, loopName, llm, sk))
				if err != nil {
					yield(nil, fmt.Errorf("agent %q until: %w", loopName, err))
					return
				}
				if !tmpl.IsTruthy(rendered) {
					return
				}
				ev := session.NewEvent(ctx.InvocationID())
				ev.Author = checkerName
				ev.Actions.Escalate = true
				yield(ev, nil)
			}
		},
	})
}
