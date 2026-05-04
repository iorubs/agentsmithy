// Package kinds is the config-time catalog of pipeline-agent kinds.
//
// Each kind (`autonomous`, `sequential`, `parallel`, `loop`,
// `orchestrator`) lives in its own file. New picks the right
// constructor for the chosen kind block and returns an ADK agent
// the pipeline runtime hands to its runner.
//
// Kinds own the full ADK wiring for their shape: autonomous calls
// llmagent.New, sequential will call sequentialagent.New, etc. The
// pipeline package stays out of kind details; it just resolves the
// shared inputs (LLM, tools, sub-agents) and calls New.
package kinds

import (
	"context"
	"fmt"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/guards"
	"github.com/iorubs/agentsmithy/internal/project/models"
	adkagent "google.golang.org/adk/agent"
	adktool "google.golang.org/adk/tool"
)

// SkillHelper is a context-bound skill function callable from
// `{{ skill "name" args... }}` templates.
type SkillHelper func(ctx context.Context, args ...any) (any, error)

// Agent is the type kinds return. It is ADK's agent.Agent verbatim;
// aliasing keeps callers off the ADK import while preserving the
// contract the pipeline runner consumes.
type Agent = adkagent.Agent

// Deps carries the resolved, kind-agnostic inputs every kind needs.
// Kind-specific fields (steps, until, maxIterations, output template)
// stay on the v1 node passed alongside.
type Deps struct {
	// Agent name. Used in logs and as ADK's agent.Name.
	Name string
	// System prompt for this agent. Required for autonomous; used
	// as Description by composition kinds.
	Instruction string
	// Resolved LLM. May be nil for composition kinds that don't
	// drive their own model call.
	LLM models.LLM
	// Resolved tools, in declaration order.
	Tools []adktool.Tool
	// Resolved toolsets accompanying Tools.
	Toolsets []adktool.Toolset
	// Already-built sub-agents in declaration order. Autonomous
	// uses these as transfer_to_agent targets; compositions use
	// them as their body.
	Subagents []Agent
	// Skills lowered for this node's templates. Keyed by skill name
	// (or built-in op name for file/web). Autonomous gets skills as
	// ADK tools through Tools above; this map drives the `{{ skill }}`
	// helper used by composition kinds' output: and orchestrator Run.
	Skills map[string]SkillHelper
	// Response guards for autonomous agents. Each guard inspects the
	// model's reply after every turn and may inject a corrective
	// message, prompting a retry within MaxIterations.
	Guards []guards.ResponseGuard
	// Cap on per-guard corrective-message retries. 0 leaves the
	// guard package's default (3) in place.
	MaxIterations int
}

// New builds an agent for one Pipeline or SubAgent node. Exactly
// one kind block on the node must be set; New picks the matching
// constructor. The pipeline walker is responsible for ensuring deps
// are populated and sub-agents are pre-built.
func New(node Node, deps Deps) (Agent, error) {
	switch {
	case node.Autonomous != nil:
		return newAutonomous(node.Autonomous, deps)
	case node.Sequential != nil:
		return newSequential(node.Sequential, deps)
	case node.Parallel != nil:
		return newParallel(node.Parallel, deps)
	case node.Loop != nil:
		return newLoop(node.Loop, deps)
	case node.Orchestrator != nil:
		return newOrchestrator(node.Orchestrator, deps)
	default:
		return nil, fmt.Errorf("agent %q: no kind block set", deps.Name)
	}
}

// Node is the kind-bearing subset of v1.Pipeline / v1.SubAgent the
// dispatcher reads. Both config types satisfy it via PipelineNode /
// SubAgentNode, keeping kinds.New decoupled from the carrier shape.
type Node struct {
	Autonomous   *v1.Autonomous
	Sequential   *v1.Sequential
	Parallel     *v1.Parallel
	Loop         *v1.Loop
	Orchestrator *v1.Orchestrator
}

// PipelineNode lifts a v1.Pipeline into a Node.
func PipelineNode(p *v1.Pipeline) Node {
	return Node{
		Autonomous:   p.Autonomous,
		Sequential:   p.Sequential,
		Parallel:     p.Parallel,
		Loop:         p.Loop,
		Orchestrator: p.Orchestrator,
	}
}

// SubAgentNode lifts a v1.SubAgent into a Node.
func SubAgentNode(s *v1.SubAgent) Node {
	return Node{
		Autonomous:   s.Autonomous,
		Sequential:   s.Sequential,
		Parallel:     s.Parallel,
		Loop:         s.Loop,
		Orchestrator: s.Orchestrator,
	}
}
