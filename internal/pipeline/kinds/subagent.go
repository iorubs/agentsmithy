package kinds

import (
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// agentHelper backs the {{ agent "name" input }} template helper. It
// looks up a sub-agent by name and runs it through a throwaway
// in-memory runner (mirroring agenttool.New's pattern), then returns
// the sub-agent's last text response. With no input arg, the parent
// orchestrator's user input is forwarded verbatim.
func agentHelper(ictx agent.InvocationContext, selfName string, subs map[string]agent.Agent) func(string, ...any) (string, error) {
	return func(name string, args ...any) (string, error) {
		sub, ok := subs[name]
		if !ok {
			return "", fmt.Errorf("agent %q: agent %q: not declared as subagent", selfName, name)
		}
		input := joinArgs(args)
		if input == "" {
			input = userInputText(ictx.UserContent())
		}
		return runSubAgent(ictx, sub, input)
	}
}

func runSubAgent(ictx agent.InvocationContext, sub agent.Agent, input string) (string, error) {
	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:         sub.Name(),
		Agent:           sub,
		SessionService:  svc,
		ArtifactService: artifact.InMemoryService(),
		MemoryService:   memory.InMemoryService(),
	})
	if err != nil {
		return "", fmt.Errorf("agent %q: runner: %w", sub.Name(), err)
	}
	sess, err := svc.Create(ictx, &session.CreateRequest{
		AppName: sub.Name(),
		UserID:  ictx.Session().UserID(),
	})
	if err != nil {
		return "", fmt.Errorf("agent %q: create session: %w", sub.Name(), err)
	}
	msg := genai.NewContentFromText(input, genai.RoleUser)
	var last *session.Event
	for ev, err := range r.Run(ictx, sess.Session.UserID(), sess.Session.ID(), msg, agent.RunConfig{}) {
		if err != nil {
			return "", fmt.Errorf("agent %q: %w", sub.Name(), err)
		}
		if ev != nil && ev.Content != nil {
			last = ev
		}
	}
	if last == nil || last.Content == nil {
		return "", nil
	}
	var sb strings.Builder
	for _, p := range last.Content.Parts {
		if p == nil || p.Thought {
			continue
		}
		sb.WriteString(p.Text)
	}
	return sb.String(), nil
}

// joinArgs folds a {{ agent "name" ... }} call's tail args into a
// single string. A single value is used verbatim; multiple values
// are space-joined.
func joinArgs(args []any) string {
	switch len(args) {
	case 0:
		return ""
	case 1:
		return fmt.Sprint(args[0])
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprint(a)
	}
	return strings.Join(parts, " ")
}
