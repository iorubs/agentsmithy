package kinds

import (
	"fmt"
	"iter"
	"strings"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/tmpl"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// newOrchestrator compiles a step graph into a custom ADK agent.
// The agent walks Steps in declaration order, rendering each step's
// `run:` template against the accumulated scope. Whitespace-only
// renders are treated as skipped per D28: the step's slot stays
// absent from scope so downstream `{{ if .step.output }}` checks
// fall through. After all steps, `output:` renders against the full
// scope and the result becomes the orchestrator's session.State entry.
func newOrchestrator(o *v1.Orchestrator, deps Deps) (Agent, error) {
	if len(o.Steps) == 0 {
		return nil, fmt.Errorf("agent %q: orchestrator requires at least one step", deps.Name)
	}
	if o.Output == "" {
		return nil, fmt.Errorf("agent %q: orchestrator requires output:", deps.Name)
	}
	subs := indexSubagents(deps.Subagents)
	tools := indexTools(deps.Tools)
	steps := append([]v1.OrchestratorStep(nil), o.Steps...)
	output := string(o.Output)
	selfName := deps.Name
	llm := deps.LLM
	sets := append([]adktool.Toolset(nil), deps.Toolsets...)
	sk := deps.Skills

	return agent.New(agent.Config{
		Name:        selfName,
		Description: deps.Instruction,
		SubAgents:   deps.Subagents,
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				rt := orchestratorRuntime(ctx, selfName, llm, subs, tools, sets, sk)
				scope := map[string]any{"input": userInputText(ctx.UserContent())}
				for _, step := range steps {
					rendered, err := tmpl.Render(string(step.Run), scope, rt)
					if err != nil {
						yield(errorEvent(ctx, selfName, fmt.Errorf("step %q: %w", step.Name, err)), err)
						return
					}
					if strings.TrimSpace(rendered) == "" {
						continue
					}
					scope[step.Name] = map[string]any{"input": "", "output": rendered}
				}
				final, err := tmpl.Render(output, scope, rt)
				if err != nil {
					yield(errorEvent(ctx, selfName, fmt.Errorf("output: %w", err)), err)
					return
				}
				if err := ctx.Session().State().Set(selfName, final); err != nil {
					yield(errorEvent(ctx, selfName, fmt.Errorf("state set: %w", err)), err)
					return
				}
				ev := session.NewEvent(ctx.InvocationID())
				ev.Author = selfName
				ev.Content = genai.NewContentFromText(final, genai.RoleModel)
				yield(ev, nil)
			}
		},
	})
}

func indexSubagents(subs []Agent) map[string]agent.Agent {
	m := make(map[string]agent.Agent, len(subs))
	for _, s := range subs {
		m[s.Name()] = s
	}
	return m
}

func indexTools(tools []adktool.Tool) map[string]adktool.Tool {
	m := make(map[string]adktool.Tool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return m
}

func errorEvent(ctx agent.InvocationContext, author string, err error) *session.Event {
	ev := session.NewEvent(ctx.InvocationID())
	ev.Author = author
	ev.ErrorMessage = err.Error()
	return ev
}
