// Package v1 defines the v1 config schema types for .agentsmithy.yaml.
//
// This is currently the latest (and only) config version. When v2 is
// introduced, create internal/config/v2/ with its own types and update
// Schema.Parse here to convert v1 → v2 via the new version's converter.
package v1

import (
	"fmt"
	"text/template"
)

// Version is the schema version this package handles.
const Version = "1"

// Config is the root of .agentsmithy.yaml. It declares the project
// identity, the model and tool catalogs the pipeline draws from, and
// the pipeline itself: the agent (or composition of agents) that runs when the service is invoked.
type Config struct {
	// Config schema version. Must be "1".
	Version string `yaml:"version" agentsmithy:"required"`
	// Project identity and the model catalog the pipeline draws from.
	Project Project `yaml:"project" agentsmithy:"required"`
	// Tool catalog. Names declared here are referenced by pipeline and sub-agent `tools:` lists.
	Tools Tools `yaml:"tools,omitempty"`
	// The pipeline that runs when this service is invoked.
	Pipeline Pipeline `yaml:"pipeline" agentsmithy:"required"`
}

// Project declares top-level identity for the service and the model
// catalog the pipeline draws from. Identity (name, instruction) and
// models live together because they describe what this config is —
// not the wiring of how agents call each other.
type Project struct {
	// Service name. Used in logs and as the A2A service identifier
	// other agents address this pipeline by.
	Name string `yaml:"name" agentsmithy:"required"`
	// System prompt for the root pipeline agent. Sub-agents declare
	// their own instruction; this one is not inherited.
	Instruction string `yaml:"instruction" agentsmithy:"required"`
	// Model catalog grouped by provider. At least one model must be
	// declared so the pipeline (and any descendants) has something to run on.
	Models Models `yaml:"models" agentsmithy:"required"`
}

// Models is the model catalog, keyed by provider. Each provider holds
// a map of author-chosen aliases (e.g. "default", "fast", "long-ctx")
// to model entries. These aliases are what `model:` refs point to.
type Models struct {
	// Ollama-served models (local or remote ollama daemon).
	Ollama map[string]ModelEntry `yaml:"ollama,omitempty"`
	// OpenAI native and OpenAI-compatible endpoints. Use baseUrl on
	// the entry to target a compatible server (LM Studio, vLLM, etc.).
	OpenAI map[string]ModelEntry `yaml:"openai,omitempty"`
}

// ModelEntry holds the connection and runtime parameters for one
// model alias. Provider-specific fields are passed through to the provider SDK at call time.
type ModelEntry struct {
	// Provider's model identifier (e.g. "qwen2.5:7b-instruct", "gpt-4o-mini").
	Model string `yaml:"model" agentsmithy:"required"`
	// Override the provider endpoint. Required for OpenAI-compatible
	// servers (LM Studio, vLLM); optional for the native provider.
	BaseURL string `yaml:"baseUrl,omitempty"`
	// Sampling temperature passed through to the provider.
	Temperature *float64 `yaml:"temperature,omitempty"`
	// Maximum response tokens. Provider-defined when unset.
	MaxTokens *int `yaml:"maxTokens,omitempty" agentsmithy:"min=1"`
}

// ModelRef points at a model entry in the catalog. Both fields
// together identify exactly one entry: `models.<provider>.<name>`.
type ModelRef struct {
	// Provider key under `models:` (e.g. ollama, openai).
	Provider Provider `yaml:"provider" agentsmithy:"required"`
	// Alias under that provider (e.g. "default").
	Name string `yaml:"name" agentsmithy:"required,ref=project.models.ollama|project.models.openai"`
}

// Provider names a supported model provider.
type Provider string

const (
	// Ollama-served models.
	ProviderOllama Provider = "ollama"
	// OpenAI native and OpenAI-compatible endpoints.
	ProviderOpenAI Provider = "openai"
)

// Values returns the set of valid Provider values.
func (Provider) Values() []string {
	return []string{string(ProviderOllama), string(ProviderOpenAI)}
}

// Tools is the tool catalog. Tools listed here can be referenced by
// pipeline and sub-agent `tools:` lists by name. Categories (mcp,
// a2a) reflect how the tool is reached at runtime. Each entry maps
// the name agents see to the SSE/HTTP endpoint that serves it.
type Tools struct {
	// MCP tools. Each value is the SSE/HTTP endpoint of one MCP
	// server (e.g. "http://localhost:8080/sse").
	MCP map[string]string `yaml:"mcp,omitempty"`
	// Agent-to-agent endpoints. Each value is the base URL of
	// another agentsmithy or A2A-compatible service the pipeline can call as a tool.
	A2A map[string]string `yaml:"a2a,omitempty"`
}

// Pipeline is the root agent. Exactly one kind block must be set —
// the chosen kind's struct dictates which fields are valid and which
// are required. Pipeline carries no `name:` or `instruction:` of its
// own (those live at the file root) and cannot inherit (it has no ancestor).
type Pipeline struct {
	// Single LLM agent that decides its own tool calls. The 90% case.
	Autonomous *Autonomous `yaml:"autonomous,omitempty" agentsmithy:"oneof=kind"`
	// Sub-agents run in declaration order, sharing transcript history.
	Sequential *Sequential `yaml:"sequential,omitempty" agentsmithy:"oneof=kind"`
	// Sub-agents fan out concurrently against the same input; outputs collected as `map[name]string`.
	Parallel *Parallel `yaml:"parallel,omitempty" agentsmithy:"oneof=kind"`
	// Sub-agents run repeatedly until an exit signal fires
	// (`exit_loop`, `until:`, or `maxIterations`).
	Loop *Loop `yaml:"loop,omitempty" agentsmithy:"oneof=kind"`
	// Explicit step graph wired with Go templates. Steps reference
	// prior step outputs and call tools, sub-agents, or skills via helpers.
	Orchestrator *Orchestrator `yaml:"orchestrator,omitempty" agentsmithy:"oneof=kind"`
}

// SubAgent is a child agent. Exactly one kind block must be set, the
// same as Pipeline. Common identity fields (name, instruction,
// inherits) sit alongside the kind block.
type SubAgent struct {
	// Agent name. Used in logs, A2A identification, and orchestrator
	// template refs (`{{ agent "name" }}`).
	Name string `yaml:"name" agentsmithy:"required,notreserved"`
	// System prompt for this agent. Not inheritable; every sub-agent declares its own.
	Instruction string `yaml:"instruction" agentsmithy:"required"`
	// Fields to pull from the nearest ancestor that declares them.
	// Local declarations always win; `inherits:` only fills gaps.
	Inherits []InheritField `yaml:"inherits,omitempty"`
	// Single LLM agent that decides its own tool calls.
	Autonomous *Autonomous `yaml:"autonomous,omitempty" agentsmithy:"oneof=kind"`
	// Sub-agents run in declaration order, sharing transcript history.
	Sequential *Sequential `yaml:"sequential,omitempty" agentsmithy:"oneof=kind"`
	// Sub-agents fan out concurrently against the same input.
	Parallel *Parallel `yaml:"parallel,omitempty" agentsmithy:"oneof=kind"`
	// Sub-agents run repeatedly until an exit signal fires.
	Loop *Loop `yaml:"loop,omitempty" agentsmithy:"oneof=kind"`
	// Explicit step graph wired with Go templates.
	Orchestrator *Orchestrator `yaml:"orchestrator,omitempty" agentsmithy:"oneof=kind"`
}

// Autonomous is a single LLM agent that decides its own tool calls.
// The 90% case. Sub-agents listed here are delegation targets the
// LLM may hand control to via `transfer_to_agent`.
type Autonomous struct {
	// Reference into the model catalog.
	Model *ModelRef `yaml:"model,omitempty"`
	// Tool names from the root `tools:` catalog this agent may use.
	Tools []string `yaml:"tools,omitempty" agentsmithy:"ref=tools.mcp|tools.a2a"`
	// Built-in skills bound to this agent.
	Skills Skills `yaml:"skills,omitempty"`
	// Memory overrides (kind-aware defaults apply when unset).
	Memory Memory `yaml:"memory,omitempty"`
	// Delegation targets the LLM may hand control to.
	Subagents []SubAgent `yaml:"subagents,omitempty"`
	// Cap on guard-driven retries.
	MaxIterations int `yaml:"maxIterations,omitempty" agentsmithy:"min=1"`
	// Return-value template. Defaults to the LLM's final reply.
	Output TemplateString `yaml:"output,omitempty"`
}

// Sequential runs sub-agents in declaration order, sharing transcript
// history. Each sub-agent sees the prior sub-agents' outputs as inbound context.
type Sequential struct {
	// Reference into the model catalog. Backs the `{{ prompt }}`
	// helper in `output:` and is the inheritance source for descendants.
	Model *ModelRef `yaml:"model,omitempty"`
	// Tool names this agent's `output:` template may reference.
	Tools []string `yaml:"tools,omitempty" agentsmithy:"ref=tools.mcp|tools.a2a"`
	// Built-in skills bound to this agent.
	Skills Skills `yaml:"skills,omitempty"`
	// Memory overrides (kind-aware defaults apply when unset).
	Memory Memory `yaml:"memory,omitempty"`
	// Composition children, run in declaration order.
	Subagents []SubAgent `yaml:"subagents" agentsmithy:"required"`
	// Return-value template. Defaults to the last child's output.
	Output TemplateString `yaml:"output,omitempty"`
}

// Parallel fans sub-agents out concurrently against the same input.
// Outputs are collected as `map[name]string` and exposed to `output:` as `.<name>`.
type Parallel struct {
	// Reference into the model catalog.
	Model *ModelRef `yaml:"model,omitempty"`
	// Tool names this agent's `output:` template may reference.
	Tools []string `yaml:"tools,omitempty" agentsmithy:"ref=tools.mcp|tools.a2a"`
	// Built-in skills bound to this agent.
	Skills Skills `yaml:"skills,omitempty"`
	// Memory overrides (kind-aware defaults apply when unset).
	Memory Memory `yaml:"memory,omitempty"`
	// Composition children, run concurrently.
	Subagents []SubAgent `yaml:"subagents" agentsmithy:"required"`
	// Return-value template. Defaults to a `name→output` map.
	Output TemplateString `yaml:"output,omitempty"`
}

// Loop runs sub-agents repeatedly until an exit signal fires
// (`exit_loop`, `until:`, or `maxIterations`).
type Loop struct {
	// Reference into the model catalog.
	Model *ModelRef `yaml:"model,omitempty"`
	// Tool names this agent's `output:` template may reference.
	Tools []string `yaml:"tools,omitempty" agentsmithy:"ref=tools.mcp|tools.a2a"`
	// Built-in skills bound to this agent.
	Skills Skills `yaml:"skills,omitempty"`
	// Memory overrides (kind-aware defaults apply when unset).
	Memory Memory `yaml:"memory,omitempty"`
	// Loop body: composition children run on each pass.
	Subagents []SubAgent `yaml:"subagents" agentsmithy:"required"`
	// Iteration cap (runaway guardrail).
	MaxIterations int `yaml:"maxIterations" agentsmithy:"required,min=1"`
	// Loop exit predicate. A Go template evaluated after each pass;
	// when it renders to truthy the loop exits. The `agent` helper
	// is forbidden here; use a reviewer sub-agent with `exit_loop` for LLM-judgment exits.
	Until TemplateString `yaml:"until,omitempty"`
	// Return-value template. Defaults to the last pass's output.
	Output TemplateString `yaml:"output,omitempty"`
}

// Orchestrator is an explicit step graph wired with Go templates.
// Steps reference prior step outputs and call tools, sub-agents, or skills via helpers.
type Orchestrator struct {
	// Reference into the model catalog. Backs the `{{ prompt }}`
	// helper inside `steps[].run` and `output:`.
	Model *ModelRef `yaml:"model,omitempty"`
	// Tool names this orchestrator may call from `steps[].run` and
	// `output:`.
	Tools []string `yaml:"tools,omitempty" agentsmithy:"ref=tools.mcp|tools.a2a"`
	// Built-in skills bound to this agent.
	Skills Skills `yaml:"skills,omitempty"`
	// Memory overrides (kind-aware defaults apply when unset).
	Memory Memory `yaml:"memory,omitempty"`
	// Callable building blocks for `steps:` (referenced via
	// `{{ agent "name" }}`).
	Subagents []SubAgent `yaml:"subagents,omitempty"`
	// Step graph.
	Steps []OrchestratorStep `yaml:"steps" agentsmithy:"required"`
	// Return-value template. Same template engine as `steps[].run`.
	Output TemplateString `yaml:"output" agentsmithy:"required"`
}

// Skills binds built-in skills to an agent. v0.1 supports guard skills only.
type Skills struct {
	// Guard names from the built-in registry. Guards run alongside
	// the agent's LLM loop and can force retries or short-circuit on rule violations.
	Guards []Guard `yaml:"guards,omitempty"`
}

// Guard names a built-in guard skill. Guards run alongside the
// agent's LLM loop and can force retries or short-circuit on rule violations.
type Guard string

const (
	// Forces the LLM to issue at least one tool call per turn.
	// Pairs with `maxIterations:` to cap retries.
	GuardRequireToolCall Guard = "requireToolCall"
)

// Values returns the set of valid Guard values.
func (Guard) Values() []string {
	return []string{string(GuardRequireToolCall)}
}

// Memory controls what conversation context an agent sees and keeps.
// Defaults are kind- and position-aware (root autonomous retains;
// composition children do not), so most agents leave this unset.
type Memory struct {
	// When true, this agent remembers its own turns across calls
	// (durable conversation). Defaults: root autonomous true; loop
	// body children true; everything else false.
	Retain *bool `yaml:"retain,omitempty"`
	// When true, this agent receives the parent's transcript as
	// inbound context in addition to its hand-off input. Defaults
	// to false everywhere; opt in only when a child needs the
	// parent's earlier turns (e.g. a panel specialist).
	Inherit *bool `yaml:"inherit,omitempty"`
}

// InheritField names a field a sub-agent can pull from an ancestor instead of declaring locally.
type InheritField string

const (
	// Pull the nearest ancestor's `model:`.
	InheritModel InheritField = "model"
	// Pull the nearest ancestor's `tools:`.
	InheritTools InheritField = "tools"
	// Pull the nearest ancestor's `skills:`.
	InheritSkills InheritField = "skills"
)

// Values returns the set of valid InheritField values.
func (InheritField) Values() []string {
	return []string{string(InheritModel), string(InheritTools), string(InheritSkills)}
}

// OrchestratorStep is one entry in an orchestrator's step graph.
// Each step's record is exposed to subsequent steps and the
// orchestrator's `output:` as `.<stepName>.{input, output}`.
type OrchestratorStep struct {
	// Step name. Becomes the path `.<stepName>` in templates.
	Name string `yaml:"name" agentsmithy:"required,notreserved"`
	// Step body. A Go text/template that calls one or more helpers.
	// A run that renders to whitespace-only is treated as skipped:
	// `.<stepName>.input` and `.<stepName>.output` are absent from
	// the template scope so downstream `{{ coalesce ... }}` /
	// `{{if ... }}` checks fall through naturally.
	Run TemplateString `yaml:"run" agentsmithy:"required"`
}

// TemplateString is a Go text/template body used by `run:`,
// `output:`, and `until:` fields. Inside the template, the agent's
// inputs and prior step records are exposed as dotted paths
// (`.input`, `.<stepName>.output`, `.<childName>.output`); the
// available helpers are listed under [BuiltinFunc]. Templates are
// parsed at config validate time, so syntax errors and references
// to unknown helpers are caught before the agent runs.
type TemplateString string

// Validate parses the template against the helper registry. It
// returns an error if the template is malformed or references a helper that is not in the registry.
func (t TemplateString) Validate() error {
	if t == "" {
		return nil
	}
	if _, err := template.New("validate").Funcs(builtinFuncStubs()).Parse(string(t)); err != nil {
		return fmt.Errorf("template: %w", err)
	}
	return nil
}

// BuiltinFunc names a template helper available inside agent
// templates (`run:`, `output:`, `until:`). Helper availability
// varies by position (`until:` forbids `agent`), but every helper
// listed here is a known name; references to anything else fail at parse time.
type BuiltinFunc string

const (
	// `tool <name> <args...>`: invoke a tool from the catalog.
	BuiltinFuncTool BuiltinFunc = "tool"
	// `agent <name> <input>`: invoke a sub-agent. Forbidden in `until:`.
	BuiltinFuncAgent BuiltinFunc = "agent"
	// `skill <name> <args...>`: invoke a built-in skill.
	BuiltinFuncSkill BuiltinFunc = "skill"
	// `prompt <template> <args...>`: run a one-shot LLM call against the surrounding agent's model.
	BuiltinFuncPrompt BuiltinFunc = "prompt"
	// `coalesce <args...>`: first non-empty argument.
	BuiltinFuncCoalesce BuiltinFunc = "coalesce"
	// `dict <key value...>`: build a map literal.
	BuiltinFuncDict BuiltinFunc = "dict"
	// `list <args...>`: build a slice literal.
	BuiltinFuncList BuiltinFunc = "list"
)

// builtinFuncStubs returns a parse-time helper registry. The bodies
// are no-op stubs; only names and arities matter for parse-only validation.
func builtinFuncStubs() template.FuncMap {
	return template.FuncMap{
		string(BuiltinFuncTool):     func(string, ...any) string { return "" },
		string(BuiltinFuncAgent):    func(string, ...any) string { return "" },
		string(BuiltinFuncSkill):    func(string, ...any) any { return nil },
		string(BuiltinFuncPrompt):   func(string, ...any) string { return "" },
		string(BuiltinFuncCoalesce): func(args ...any) any { return nil },
		string(BuiltinFuncDict):     func(args ...any) map[string]any { return nil },
		string(BuiltinFuncList):     func(args ...any) []any { return nil },
	}
}
