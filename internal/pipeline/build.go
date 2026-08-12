package pipeline

import (
	"context"
	"fmt"

	"github.com/iorubs/agentsmithy/internal/config"
	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/guards"
	"github.com/iorubs/agentsmithy/internal/pipeline/kinds"
	"github.com/iorubs/agentsmithy/internal/pipeline/skills"
	"github.com/iorubs/agentsmithy/internal/project/models"
	"github.com/iorubs/agentsmithy/internal/tools"
	adktool "google.golang.org/adk/tool"
)

// buildPipeline lowers Config.Pipeline (the root). The root pipeline
// is identified by Project.Name and uses Project.Instruction as its
// system prompt; the kind block carries the rest of the wiring.
func buildPipeline(cfg *config.Config) (kinds.Agent, error) {
	node := kinds.PipelineNode(&cfg.Pipeline)
	return buildNode(cfg, node, cfg.Project.Name, cfg.Project.Instruction, nil)
}

// buildSubAgent lowers one SubAgent. The agent's name comes from
// the map key (already populated on sa.Name); the kind block carries
// the instruction and inherits set. ancestors is the chain from
// root to this sub-agent's parent (root first), used to resolve
// `inherits:` field references.
func buildSubAgent(cfg *config.Config, sa *v1.SubAgent, ancestors []kinds.Node) (kinds.Agent, error) {
	node := kinds.SubAgentNode(sa)
	if err := validateInherits(node, ancestors, sa.Name); err != nil {
		return nil, err
	}
	return buildNode(cfg, node, sa.Name, nodeInstruction(node), ancestors)
}

// buildNode is the common spine: resolve model + sub-agents, refuse
// the not-yet-implemented surfaces (tools, skills), then dispatch
// through kinds.New. ancestors is the chain from root to this
// node's parent; nil for the root pipeline.
func buildNode(cfg *config.Config, node kinds.Node, name, instruction string, ancestors []kinds.Node) (kinds.Agent, error) {
	model, err := resolveModel(cfg, node, ancestors, name)
	if err != nil {
		return nil, err
	}

	if err := refuseUnsupported(node, name); err != nil {
		return nil, err
	}

	nodeTools, nodeToolsets, err := resolveTools(cfg, node, ancestors, name)
	if err != nil {
		return nil, err
	}

	skillTools, skillHelpers, err := resolveSkills(node, ancestors, name)
	if err != nil {
		return nil, err
	}
	nodeTools = append(nodeTools, skillTools...)

	children, err := buildSubAgents(cfg, node, ancestors)
	if err != nil {
		return nil, err
	}

	guardList, err := guards.Build(nodeSkills(node).Guards)
	if err != nil {
		return nil, fmt.Errorf("agent %q: skills.guards: %w", name, err)
	}

	return kinds.New(node, kinds.Deps{
		Name:          name,
		Instruction:   instruction,
		LLM:           model,
		Tools:         nodeTools,
		Toolsets:      nodeToolsets,
		Subagents:     children,
		Skills:        skillHelpers,
		Guards:        guardList,
		MaxIterations: nodeMaxIterations(node),
	})
}

// resolveModel reads the kind block's Model ref and looks the entry
// up in the project catalog. The autonomous kind requires a model;
// composition kinds may declare one for `output:` template prompt
// helpers (deferred), so a missing ref is allowed for those and
// surfaced as an error inside the kind constructor when needed.
// When the local node lists `model` in `inherits:`, the nearest
// ancestor's model is used as a fallback.
func resolveModel(cfg *config.Config, node kinds.Node, ancestors []kinds.Node, name string) (models.LLM, error) {
	ref := effectiveModelRef(node, ancestors)
	if ref == nil {
		return nil, nil
	}
	entry, ok := lookupModel(&cfg.Project.Models, *ref)
	if !ok {
		return nil, fmt.Errorf("agent %q: model %s.%s not in catalog", name, ref.Provider, ref.Name)
	}

	ctx := context.Background()

	llm, err := models.New(ctx, *ref, entry)
	if err != nil {
		return nil, fmt.Errorf("agent %q: %w", name, err)
	}
	return llm, nil
}

// nodeModelRef returns the kind block's Model field, or nil if the
// node doesn't declare one.
func nodeModelRef(node kinds.Node) *config.ModelRef {
	switch {
	case node.Autonomous != nil:
		return node.Autonomous.Model
	case node.Sequential != nil:
		return node.Sequential.Model
	case node.Parallel != nil:
		return node.Parallel.Model
	case node.Loop != nil:
		return node.Loop.Model
	case node.Orchestrator != nil:
		return node.Orchestrator.Model
	default:
		return nil
	}
}

// lookupModel finds entry models.<ref.Provider>.<ref.Name>.
func lookupModel(cat *v1.Models, ref config.ModelRef) (config.ModelEntry, bool) {
	var bucket map[string]v1.ModelEntry
	switch ref.Provider {
	case config.ProviderOpenAI:
		bucket = cat.OpenAI
	case config.ProviderAnthropic:
		bucket = cat.Anthropic
	case config.ProviderGoogle:
		bucket = cat.Google
	case config.ProviderBedrock:
		bucket = cat.Bedrock
	case config.ProviderVertex:
		bucket = cat.Vertex
	case config.ProviderBorrowed:
		bucket = cat.Borrowed
	default:
		return config.ModelEntry{}, false
	}
	entry, ok := bucket[ref.Name]
	return entry, ok
}

// refuseUnsupported errors when a node declares a surface the
// pipeline package can't yet honour. These return explicit
// "not implemented yet" errors rather than silently dropping config;
// silent drops would hide misconfiguration.
func refuseUnsupported(node kinds.Node, name string) error {
	if a := node.Autonomous; a != nil {
		if a.Memory.Retain != nil || a.Memory.Inherit != nil {
			return fmt.Errorf("agent %q: memory: not implemented yet", name)
		}
	}
	return nil
}

// buildSubAgents lowers the kind block's Subagents slice (if any).
// ancestors holds the chain leading up to the parent of these
// children; this node is appended before recursing so its declared
// fields are visible to descendants' `inherits:` resolution.
func buildSubAgents(cfg *config.Config, node kinds.Node, ancestors []kinds.Node) ([]kinds.Agent, error) {
	subs := nodeSubAgents(node)
	if len(subs) == 0 {
		return nil, nil
	}
	childAncestors := append(append([]kinds.Node{}, ancestors...), node)
	out := make([]kinds.Agent, 0, len(subs))
	for i := range subs {
		child, err := buildSubAgent(cfg, &subs[i], childAncestors)
		if err != nil {
			return nil, err
		}
		out = append(out, child)
	}
	return out, nil
}

// nodeSubAgents returns the kind block's Subagents list.
func nodeSubAgents(node kinds.Node) []v1.SubAgent {
	switch {
	case node.Autonomous != nil:
		return node.Autonomous.Subagents
	case node.Sequential != nil:
		return node.Sequential.Subagents
	case node.Parallel != nil:
		return node.Parallel.Subagents
	case node.Loop != nil:
		return node.Loop.Subagents
	case node.Orchestrator != nil:
		return node.Orchestrator.Subagents
	default:
		return nil
	}
}

// nodeInstruction returns the kind block's Instruction.
func nodeInstruction(node kinds.Node) string {
	switch {
	case node.Autonomous != nil:
		return node.Autonomous.Instruction
	case node.Sequential != nil:
		return node.Sequential.Instruction
	case node.Parallel != nil:
		return node.Parallel.Instruction
	case node.Loop != nil:
		return node.Loop.Instruction
	case node.Orchestrator != nil:
		return node.Orchestrator.Instruction
	default:
		return ""
	}
}

// nodeInherits returns the kind block's Inherits list.
func nodeInherits(node kinds.Node) []v1.InheritField {
	switch {
	case node.Autonomous != nil:
		return node.Autonomous.Inherits
	case node.Sequential != nil:
		return node.Sequential.Inherits
	case node.Parallel != nil:
		return node.Parallel.Inherits
	case node.Loop != nil:
		return node.Loop.Inherits
	case node.Orchestrator != nil:
		return node.Orchestrator.Inherits
	default:
		return nil
	}
}

// inheritsField reports whether the node opts in to inheriting field.
func inheritsField(node kinds.Node, field v1.InheritField) bool {
	for _, f := range nodeInherits(node) {
		if f == field {
			return true
		}
	}
	return false
}

// effectiveModelRef returns the model ref this node runs with: its
// own declaration if present, otherwise the nearest ancestor's when
// `model` is listed in `inherits:`. Returns nil when no model is
// resolvable; the caller decides whether that is an error for the kind.
func effectiveModelRef(node kinds.Node, ancestors []kinds.Node) *config.ModelRef {
	if r := nodeModelRef(node); r != nil {
		return r
	}
	if !inheritsField(node, v1.InheritModel) {
		return nil
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		if r := nodeModelRef(ancestors[i]); r != nil {
			return r
		}
	}
	return nil
}

// effectiveToolRefs returns the tool name list this node uses:
// its own list if non-empty, otherwise the nearest ancestor's when
// `tools` is listed in `inherits:`.
func effectiveToolRefs(node kinds.Node, ancestors []kinds.Node) []string {
	if local := nodeToolRefs(node); len(local) > 0 {
		return local
	}
	if !inheritsField(node, v1.InheritTools) {
		return nil
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		if t := nodeToolRefs(ancestors[i]); len(t) > 0 {
			return t
		}
	}
	return nil
}

// effectiveSkills returns the skills block this node uses: its own
// if non-zero, otherwise the nearest ancestor's when `skills` is
// listed in `inherits:`. The whole block is replaced — no merging.
func effectiveSkills(node kinds.Node, ancestors []kinds.Node) v1.Skills {
	if s := nodeSkills(node); !skillsEmpty(s) {
		return s
	}
	if !inheritsField(node, v1.InheritSkills) {
		return v1.Skills{}
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		if s := nodeSkills(ancestors[i]); !skillsEmpty(s) {
			return s
		}
	}
	return v1.Skills{}
}

// skillsEmpty mirrors resolveSkills' "nothing to build" predicate.
func skillsEmpty(s v1.Skills) bool {
	return len(s.Shell) == 0 && s.File == nil && s.Web == nil && len(s.Guards) == 0
}

// validateInherits errors when the node lists a field in `inherits:`
// that no ancestor declares — `inherits:` must point at something
// resolvable, otherwise the agent has no way to satisfy the field.
// The local declaration wins regardless, so a locally-declared field
// in the inherits list is allowed (validate-time concern, not built).
func validateInherits(node kinds.Node, ancestors []kinds.Node, name string) error {
	for _, field := range nodeInherits(node) {
		if hasLocalField(node, field) {
			continue
		}
		if !ancestorHasField(ancestors, field) {
			return fmt.Errorf("agent %q: inherits %q but no ancestor declares it", name, field)
		}
	}
	return nil
}

// hasLocalField reports whether the node declares field locally.
func hasLocalField(node kinds.Node, field v1.InheritField) bool {
	switch field {
	case v1.InheritModel:
		return nodeModelRef(node) != nil
	case v1.InheritTools:
		return len(nodeToolRefs(node)) > 0
	case v1.InheritSkills:
		return !skillsEmpty(nodeSkills(node))
	}
	return false
}

// ancestorHasField reports whether any ancestor declares field.
func ancestorHasField(ancestors []kinds.Node, field v1.InheritField) bool {
	for _, a := range ancestors {
		if hasLocalField(a, field) {
			return true
		}
	}
	return false
}

// nodeToolRefs returns the names this node references from the
// project tool catalog.
func nodeToolRefs(node kinds.Node) []string {
	switch {
	case node.Autonomous != nil:
		return node.Autonomous.Tools
	case node.Sequential != nil:
		return node.Sequential.Tools
	case node.Parallel != nil:
		return node.Parallel.Tools
	case node.Loop != nil:
		return node.Loop.Tools
	case node.Orchestrator != nil:
		return node.Orchestrator.Tools
	default:
		return nil
	}
}

// resolveTools turns the node's tool name list into ADK tools and
// toolsets by looking each name up in the project tool catalog. mcp
// entries become toolsets, a2a entries become single tools. When
// the local node lists `tools` in `inherits:`, the nearest ancestor's
// tools list is used as a fallback.
func resolveTools(cfg *config.Config, node kinds.Node, ancestors []kinds.Node, name string) ([]adktool.Tool, []adktool.Toolset, error) {
	refs := effectiveToolRefs(node, ancestors)
	if len(refs) == 0 {
		return nil, nil, nil
	}
	var (
		out      []adktool.Tool
		toolsets []adktool.Toolset
	)
	for _, ref := range refs {
		r, err := lookupTool(&cfg.Tools, ref)
		if err != nil {
			return nil, nil, fmt.Errorf("agent %q: %w", name, err)
		}
		if r.Toolset != nil {
			toolsets = append(toolsets, r.Toolset)
		}
		out = append(out, r.Tools...)
	}
	return out, toolsets, nil
}

// lookupTool resolves a tool name against the project catalog,
// preferring mcp over a2a if the same name is declared in both
// (schema currently doesn't enforce uniqueness across categories).
func lookupTool(cat *v1.Tools, ref string) (tools.Resolved, error) {
	if url, ok := cat.MCP[ref]; ok {
		return tools.MCP(ref, url)
	}
	if url, ok := cat.A2A[ref]; ok {
		return tools.A2A(ref, url)
	}
	return tools.Resolved{}, fmt.Errorf("tool %q not in catalog", ref)
}

// nodeMaxIterations returns the autonomous kind's MaxIterations,
// or 0 for kinds where it isn't meaningful (composition kinds carry
// their own loop semantics).
func nodeMaxIterations(node kinds.Node) int {
	if node.Autonomous != nil {
		return node.Autonomous.MaxIterations
	}
	return 0
}

// nodeSkills returns the kind block's Skills field, or zero if the
// node doesn't declare skills.
func nodeSkills(node kinds.Node) v1.Skills {
	switch {
	case node.Autonomous != nil:
		return node.Autonomous.Skills
	case node.Sequential != nil:
		return node.Sequential.Skills
	case node.Parallel != nil:
		return node.Parallel.Skills
	case node.Loop != nil:
		return node.Loop.Skills
	case node.Orchestrator != nil:
		return node.Orchestrator.Skills
	default:
		return v1.Skills{}
	}
}

// resolveSkills lowers the node's Skills block into ADK tools and
// template helpers via the skills package. Returned tools append to
// the node's Tools list (autonomous exposes skills as ADK tools);
// helpers feed kinds.Deps.Skills (composition kinds invoke skills
// from `output:` / `until:` / orchestrator templates). When the
// local node lists `skills` in `inherits:`, the nearest ancestor's
// Skills block is used as a fallback.
func resolveSkills(node kinds.Node, ancestors []kinds.Node, name string) ([]adktool.Tool, map[string]kinds.SkillHelper, error) {
	s := effectiveSkills(node, ancestors)
	if len(s.Shell) == 0 && s.File == nil && s.Web == nil {
		return nil, nil, nil
	}
	built, err := skills.Build(name, "", s)
	if err != nil {
		return nil, nil, err
	}
	helpers := make(map[string]kinds.SkillHelper, len(built.Helpers))
	for k, v := range built.Helpers {
		helpers[k] = kinds.SkillHelper(v)
	}
	return built.Tools, helpers, nil
}
