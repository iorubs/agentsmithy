// Package pipeline lowers a v1.Config into a runnable agent tree.
//
// Build walks Project.Pipeline + the SubAgent tree, resolves each
// node's ModelRef into the concrete LLM via internal/project/models,
// and hands the per-node deps to internal/pipeline/kinds, which
// returns the ADK agent for that node.
//
// The result of Build is a Pipeline holding the root agent ready to
// run. Higher-level concerns (sessions, transports, the public chat
// API) live in dedicated packages and consume Pipeline.Agent().
package pipeline

import (
	"github.com/iorubs/agentsmithy/internal/config"
	"github.com/iorubs/agentsmithy/internal/pipeline/kinds"
)

// Pipeline is the built agent tree for one Config. Root is the agent
// the runner invokes.
type Pipeline struct {
	// Project name (Config.Project.Name), surfaced for logs and A2A identification.
	Name string
	// Root is the top-level agent built from Config.Pipeline.
	Root kinds.Agent
}

// Build lowers cfg into a runnable Pipeline. Model refs are resolved
// against Project.Models; sub-agents are built post-order so each
// kind constructor receives pre-built children.
func Build(cfg *config.Config) (*Pipeline, error) {
	root, err := buildPipeline(cfg)
	if err != nil {
		return nil, err
	}
	return &Pipeline{Name: cfg.Project.Name, Root: root}, nil
}
