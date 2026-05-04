package kinds

import (
	"fmt"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
)

func newSequential(s *v1.Sequential, deps Deps) (Agent, error) {
	if len(deps.Subagents) == 0 {
		return nil, fmt.Errorf("agent %q: sequential requires at least one subagent", deps.Name)
	}
	cfg := sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        deps.Name,
			Description: deps.Instruction,
			SubAgents:   deps.Subagents,
		},
	}
	if cb := outputCallback(deps.Name, s.Output, childNames(deps.Subagents), deps.LLM, deps.Skills); cb != nil {
		cfg.AgentConfig.AfterAgentCallbacks = []agent.AfterAgentCallback{cb}
	}
	return sequentialagent.New(cfg)
}
