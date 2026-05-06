package kinds

import (
	"fmt"
	"strings"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/tmpl"
	"github.com/iorubs/agentsmithy/internal/project/models"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// childOutputScope builds the template scope for a parent's
// output: render. Each child contributes a `.<name>.{input,output}`
// map read from session state. Children without recorded output
// (e.g. nested compositions that do not declare output:) are absent
// from the scope so `{{ if .child.output }}` checks fall through.
// `.input` is reserved per D24; today it's always empty because
// kinds don't yet track per-child input rewrites.
func childOutputScope(state session.ReadonlyState, siblings []string) map[string]any {
	scope := make(map[string]any, len(siblings))
	for _, n := range siblings {
		v, err := state.Get(n)
		if err != nil {
			continue
		}
		out, _ := v.(string)
		scope[n] = map[string]any{"input": "", "output": out}
	}
	return scope
}

// userInputText flattens the invocation's user content into a
// single string. Non-text parts and thought parts are dropped.
func userInputText(uc *genai.Content) string {
	if uc == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range uc.Parts {
		if p == nil || p.Thought {
			continue
		}
		sb.WriteString(p.Text)
	}
	return sb.String()
}

// childNames returns each sub-agent's Name(). Used by parents to
// resolve `.<childName>` paths in their output: templates.
func childNames(subs []Agent) []string {
	names := make([]string, 0, len(subs))
	for _, s := range subs {
		names = append(names, s.Name())
	}
	return names
}

// outputCallback returns an AfterAgentCallback that renders the
// output: template. The rendered string is saved to session state
// under selfName so an enclosing parent can read it the same way.
// llm backs the `{{ prompt }}` helper; pass nil if the kind has no
// model in scope (the helper will then error at template-execute time).
// Returns nil when output is empty; callers should drop the slot.
//
// Scope: `.input` is the user input; `.output` is whatever was last
// stored at state[selfName] before the render (autonomous's
// OutputKey writes the LLM reply there, so `.output` lets the
// template post-process it). Each sibling contributes
// `.<name>.{input,output}` for parents reading their children.
func outputCallback(selfName string, output v1.TemplateString, siblings []string, llm models.LLM, sk map[string]SkillHelper) agent.AfterAgentCallback {
	if output == "" {
		return nil
	}
	body := string(output)
	return func(ctx agent.CallbackContext) (*genai.Content, error) {
		scope := childOutputScope(ctx.ReadonlyState(), siblings)
		scope["input"] = userInputText(ctx.UserContent())
		if v, err := ctx.ReadonlyState().Get(selfName); err == nil {
			out, _ := v.(string)
			scope["output"] = out
		} else {
			scope["output"] = ""
		}
		runtime := callbackRuntime(ctx, selfName, llm, sk)
		rendered, err := tmpl.Render(body, scope, runtime)
		if err != nil {
			return nil, fmt.Errorf("output: %w", err)
		}
		if err := ctx.State().Set(selfName, rendered); err != nil {
			return nil, fmt.Errorf("state set: %w", err)
		}
		return genai.NewContentFromText(rendered, genai.RoleModel), nil
	}
}
