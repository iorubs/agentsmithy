package kinds

import (
	"fmt"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/guards"
	"github.com/iorubs/agentsmithy/internal/pipeline/obs"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
)

// newAutonomous builds a single LLM agent that decides its own tool
// calls. Sub-agents on the node become transfer_to_agent targets.
//
// It wires model, instruction, tools, sub-agents, response guards
// and the optional output: template into llmagent.Config.
// Memory.Retain and Memory.Inherit are not yet supported.
func newAutonomous(a *v1.Autonomous, deps Deps) (Agent, error) {
	if a == nil {
		return nil, fmt.Errorf("agent %q: autonomous block missing", deps.Name)
	}
	if deps.LLM == nil {
		return nil, fmt.Errorf("agent %q: autonomous requires a resolved model", deps.Name)
	}

	beforeModel, afterModel, beforeTool, afterTool := obs.Callbacks(deps.Name)
	afterModelCBs := []llmagent.AfterModelCallback{afterModel}
	if gcb := guards.Callback(deps.Guards, deps.MaxIterations); gcb != nil {
		afterModelCBs = append(afterModelCBs, gcb)
	}
	cfg := llmagent.Config{
		Name:                 deps.Name,
		Description:          deps.Instruction,
		Instruction:          deps.Instruction,
		Model:                deps.LLM,
		Tools:                deps.Tools,
		Toolsets:             deps.Toolsets,
		SubAgents:            deps.Subagents,
		OutputKey:            deps.Name,
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{beforeModel},
		AfterModelCallbacks:  afterModelCBs,
		BeforeToolCallbacks:  []llmagent.BeforeToolCallback{beforeTool},
		AfterToolCallbacks:   []llmagent.AfterToolCallback{afterTool},
	}
	if cb := outputCallback(deps.Name, a.Output, childNames(deps.Subagents), deps.LLM, deps.Skills); cb != nil {
		cfg.AfterAgentCallbacks = []agent.AfterAgentCallback{cb}
	}
	return llmagent.New(cfg)
}
