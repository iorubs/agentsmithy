// Package v1 defines the v1 config schema types for .agentsmithy.yaml.
//
// This is currently the latest (and only) config version. When v2 is
// introduced, create internal/config/v2/ with its own types and update
// Schema.Parse here to convert v1 → v2 via the new version's converter.
package v1

import (
	"fmt"
	"reflect"
	"strings"
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

// Project declares top-level identity for the service: the service
// name, the root system prompt, and the model catalog the pipeline
// draws from.
type Project struct {
	// Service name. Used in logs and as the A2A service identifier
	// other agents address this pipeline by.
	Name string `yaml:"name" agentsmithy:"required"`
	// System prompt for the root pipeline agent. Sub-agents declare
	// their own instruction inside their kind block; this one is not
	// inherited.
	Instruction string `yaml:"instruction" agentsmithy:"required"`
	// Model catalog grouped by provider. At least one model must be
	// declared so the pipeline (and any descendants) has something to run on.
	Models Models `yaml:"models" agentsmithy:"required"`
}

// Models is the model catalog, keyed by provider. Each provider holds
// a map of author-chosen aliases (e.g. "default", "fast", "long-ctx")
// to model entries. These aliases are what `model:` refs point to.
type Models struct {
	// OpenAI native and OpenAI-compatible endpoints. Use baseUrl on
	// the entry to target a compatible server (Ollama, LM Studio,
	// vLLM, Together, Groq, OpenRouter, DeepSeek, xAI).
	OpenAI map[string]ModelEntry `yaml:"openai,omitempty"`
	// Anthropic Claude (native Messages API).
	Anthropic map[string]ModelEntry `yaml:"anthropic,omitempty"`
	// Google Gemini (developer API key).
	Google map[string]ModelEntry `yaml:"google,omitempty"`
	// AWS Bedrock-hosted models.
	Bedrock map[string]ModelEntry `yaml:"bedrock,omitempty"`
	// Google Vertex AI-hosted models.
	Vertex map[string]ModelEntry `yaml:"vertex,omitempty"`
	// Borrowed: delegate completion to the connecting MCP client via
	// sampling/createMessage. Requires an MCP transport.
	Borrowed map[string]ModelEntry `yaml:"borrowed,omitempty"`
}

// ModelEntry holds the connection and runtime parameters for one
// model alias. Provider-specific fields are passed through to the provider SDK at call time.
type ModelEntry struct {
	// Provider's model identifier (e.g. "qwen2.5:7b-instruct", "gpt-4o-mini").
	Model string `yaml:"model" agentsmithy:"required"`
	// Override the provider endpoint. Required for OpenAI-compatible
	// servers (LM Studio, vLLM); optional for the native provider.
	BaseURL string `yaml:"baseUrl,omitempty"`
	// Name of the environment variable holding the API key for this
	// model. Defaults to the provider's conventional variable when
	// unset (OPENAI_API_KEY, ANTHROPIC_API_KEY, GOOGLE_API_KEY,
	// AWS_ACCESS_KEY_ID, GOOGLE_APPLICATION_CREDENTIALS). Ignored by
	// the borrowed provider (the connecting MCP client owns auth).
	APIKeyEnv string `yaml:"apiKeyEnv,omitempty"`
	// Sampling temperature passed through to the provider.
	Temperature *float64 `yaml:"temperature,omitempty"`
	// Maximum response tokens. Provider-defined when unset.
	MaxTokens *int `yaml:"maxTokens,omitempty" agentsmithy:"min=1"`
}

// ModelRef points at a model entry in the catalog. Both fields
// together identify exactly one entry: `models.<provider>.<name>`.
type ModelRef struct {
	// Provider key under `models:` (e.g. openai, anthropic).
	Provider Provider `yaml:"provider" agentsmithy:"required"`
	// Alias under that provider (e.g. "default").
	Name string `yaml:"name" agentsmithy:"required,ref=project.models.openai|project.models.anthropic|project.models.google|project.models.bedrock|project.models.vertex|project.models.borrowed"`
}

// Provider names a supported model provider.
type Provider string

const (
	// OpenAI native and OpenAI-compatible endpoints (Ollama, LM Studio,
	// vLLM, Together, Groq, OpenRouter, DeepSeek, xAI). Override
	// baseUrl on the entry to target a compatible server.
	ProviderOpenAI Provider = "openai"
	// Anthropic Claude (native Messages API).
	ProviderAnthropic Provider = "anthropic"
	// Google Gemini (developer API key).
	ProviderGoogle Provider = "google"
	// AWS Bedrock-hosted models.
	ProviderBedrock Provider = "bedrock"
	// Google Vertex AI-hosted models.
	ProviderVertex Provider = "vertex"
	// Borrowed: delegate completion to the connecting MCP client via
	// sampling/createMessage. Requires an MCP transport.
	ProviderBorrowed Provider = "borrowed"
)

// Values returns the set of valid Provider values.
func (Provider) Values() []string {
	return []string{
		string(ProviderOpenAI),
		string(ProviderAnthropic),
		string(ProviderGoogle),
		string(ProviderBedrock),
		string(ProviderVertex),
		string(ProviderBorrowed),
	}
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

// Pipeline is the root agent. Exactly one kind block must be set;
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

// SubAgent is a child agent. Identified by its `name:` field.
// Exactly one kind block must be set. Identity fields (instruction,
// inherits) live inside the kind block alongside everything else,
// so a sub-agent body is just `<kind>: { ... }` with no wrapper noise.
type SubAgent struct {
	// Agent name. Used by templates (`{{ agent "name" }}`), by
	// composition kinds, and as the ADK agent name.
	Name string `yaml:"name" agentsmithy:"required,notreserved"`
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

// Validate ensures the active kind block carries an instruction.
// The root pipeline reads its instruction from project.instruction
// (Project.Validate enforces that side); only SubAgent enforces it
// here so root pipelines aren't double-required.
func (s SubAgent) Validate() error {
	switch {
	case s.Autonomous != nil:
		if s.Autonomous.Instruction == "" {
			return fmt.Errorf("autonomous.instruction is required")
		}
	case s.Sequential != nil:
		if s.Sequential.Instruction == "" {
			return fmt.Errorf("sequential.instruction is required")
		}
	case s.Parallel != nil:
		if s.Parallel.Instruction == "" {
			return fmt.Errorf("parallel.instruction is required")
		}
	case s.Loop != nil:
		if s.Loop.Instruction == "" {
			return fmt.Errorf("loop.instruction is required")
		}
	case s.Orchestrator != nil:
		if s.Orchestrator.Instruction == "" {
			return fmt.Errorf("orchestrator.instruction is required")
		}
	}
	return nil
}

// Autonomous is a single LLM agent that decides its own tool calls.
// The 90% case. Sub-agents listed here are delegation targets the
// LLM may hand control to via `transfer_to_agent`.
type Autonomous struct {
	// System prompt for this agent. Required on sub-agents (enforced
	// by SubAgent.Validate); the root pipeline reads its instruction
	// from project.instruction instead.
	Instruction string `yaml:"instruction,omitempty"`
	// Fields to pull from the nearest ancestor that declares them.
	// Local declarations always win; `inherits:` only fills gaps.
	// Not valid on the root pipeline (it has no ancestor).
	Inherits []InheritField `yaml:"inherits,omitempty"`
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
	// System prompt for this agent. Required on sub-agents (enforced
	// by SubAgent.Validate); the root pipeline reads its instruction
	// from project.instruction instead.
	Instruction string `yaml:"instruction,omitempty"`
	// Fields to pull from the nearest ancestor that declares them.
	Inherits []InheritField `yaml:"inherits,omitempty"`
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
	// System prompt for this agent. Required on sub-agents (enforced
	// by SubAgent.Validate); the root pipeline reads its instruction
	// from project.instruction instead.
	Instruction string `yaml:"instruction,omitempty"`
	// Fields to pull from the nearest ancestor that declares them.
	Inherits []InheritField `yaml:"inherits,omitempty"`
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
	// System prompt for this agent. Required on sub-agents (enforced
	// by SubAgent.Validate); the root pipeline reads its instruction
	// from project.instruction instead.
	Instruction string `yaml:"instruction,omitempty"`
	// Fields to pull from the nearest ancestor that declares them.
	Inherits []InheritField `yaml:"inherits,omitempty"`
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
	// System prompt for this agent. Required on sub-agents (enforced
	// by SubAgent.Validate); the root pipeline reads its instruction
	// from project.instruction instead.
	Instruction string `yaml:"instruction,omitempty"`
	// Fields to pull from the nearest ancestor that declares them.
	Inherits []InheritField `yaml:"inherits,omitempty"`
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
	// Step graph. Steps run in declaration order; each step's record
	// is exposed to subsequent steps and `output:` as `.<name>`.
	Steps []OrchestratorStep `yaml:"steps" agentsmithy:"required"`
	// Return-value template. Same template engine as `steps[].run`.
	Output TemplateString `yaml:"output" agentsmithy:"required"`
}

// Skills binds built-in skills to an agent.
type Skills struct {
	// Guard names from the built-in registry. Guards run alongside
	// the agent's LLM loop and can force retries or short-circuit on rule violations.
	Guards []Guard `yaml:"guards,omitempty"`
	// Shell command execution skills, keyed by skill name. Each entry
	// declares a command, working directory, and typed args; invocable
	// by the agent as a tool (autonomous) or via `{{ skill "name" ... }}`
	// in templates (sequential/parallel/loop/orchestrator).
	Shell map[string]ShellSkill `yaml:"shell,omitempty" agentsmithy:"notreserved"`
	// File read/write skills (read/list/glob/write within workingDir).
	File *FileSkill `yaml:"file,omitempty"`
	// Single-page web scrape skill (URL allowlisted).
	Web *WebSkill `yaml:"web,omitempty"`
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

// ShellSkill is one user-declared shell command exposed to an agent.
// The map key under `skills.shell:` becomes the entry name; the
// command runs in WorkingDir with an empty environment. Args are
// typed inputs rendered into Command via Go text/template.
type ShellSkill struct {
	// Program and arguments. Element 0 must be a program name (not a
	// shell interpreter). Templates may appear inside elements but
	// must occupy the entire element — splicing template output into
	// a literal substring (e.g. `"-l {{ .path }}"`) is rejected.
	Command []string `yaml:"command" agentsmithy:"required"`
	// Working directory for command execution. Project-rooted; no
	// `..` escape allowed at runtime.
	WorkingDir string `yaml:"workingDir" agentsmithy:"required"`
	// Typed input parameters provided by the caller (LLM as tool, or
	// template helper as positional args).
	Args []Param `yaml:"args,omitempty"`
}

// Validate enforces the shell-skill invariants that the schema-tag
// engine can't express: no shell-interpreter Command[0], no spliced
// templates, no duplicate Args names, valid Args types.
func (s ShellSkill) Validate(name string) error {
	if len(s.Command) == 0 {
		return fmt.Errorf("shell skill %q: command is required and non-empty", name)
	}
	restricted := map[string]bool{
		"sh": true, "bash": true, "zsh": true, "ash": true, "dash": true,
		"cmd": true, "cmd.exe": true, "powershell": true, "pwsh": true,
	}
	if restricted[s.Command[0]] {
		return fmt.Errorf("shell skill %q: command[0] cannot be a shell interpreter (%s); pass the program directly so each arg is its own element", name, s.Command[0])
	}
	for i, elem := range s.Command {
		if !strings.Contains(elem, "{{") {
			continue
		}
		// Pure template: trimmed string must start with "{{" and end with "}}",
		// and no other "{{" or "}}" inside (single template expression).
		t := strings.TrimSpace(elem)
		if !strings.HasPrefix(t, "{{") || !strings.HasSuffix(t, "}}") {
			return fmt.Errorf("shell skill %q: command[%d] %q has spliced template; templates must occupy the whole element", name, i, elem)
		}
		// Reject multiple templates in one element (rough heuristic).
		if strings.Count(t, "{{") != 1 || strings.Count(t, "}}") != 1 {
			return fmt.Errorf("shell skill %q: command[%d] %q has multiple templates in one element; split into separate args", name, i, elem)
		}
	}
	seenArgs := make(map[string]struct{}, len(s.Args))
	validTypes := map[ParamType]struct{}{
		ParamTypeString:          {},
		ParamTypeNumber:          {},
		ParamTypeBool:            {},
		ParamTypeArray:           {},
		ParamTypeProjectFilePath: {},
	}
	for _, a := range s.Args {
		if a.Name == "" {
			return fmt.Errorf("shell skill %q: arg has empty name", name)
		}
		if _, dup := seenArgs[a.Name]; dup {
			return fmt.Errorf("shell skill %q: duplicate arg name %q", name, a.Name)
		}
		seenArgs[a.Name] = struct{}{}
		if _, ok := validTypes[a.Type]; !ok {
			return fmt.Errorf("shell skill %q: arg %q has invalid type %q", name, a.Name, a.Type)
		}
	}
	return nil
}

// FileSkill enables sandboxed file ops for an agent rooted at WorkingDir.
// Each operation is gated by an explicit Enabled flag; omitted blocks
// are denied. Empty Paths with Enabled=true means "any path under
// WorkingDir" for read; for write it means "any path under WorkingDir"
// (use a Paths allowlist to narrow further).
type FileSkill struct {
	// WorkingDir is the filesystem root for all file ops. Absolute or
	// relative to project root.
	WorkingDir string `yaml:"workingDir" agentsmithy:"required"`
	// Read configuration. Omitted means reads denied.
	Read *FileOp `yaml:"read,omitempty"`
	// Write configuration. Omitted means writes denied.
	Write *FileOp `yaml:"write,omitempty"`
}

// FileOp gates one file operation (read or write).
type FileOp struct {
	// Enabled is true when this operation is allowed. False (or omitted) denies it.
	Enabled bool `yaml:"enabled"`
	// Paths is a glob allowlist within FileSkill.WorkingDir. Empty with
	// Enabled=true means the full WorkingDir subtree.
	Paths []string `yaml:"paths,omitempty"`
}

// WebSkill enables sandboxed HTTP scraping for an agent. Exposes a
// single `web_scrape` tool that GETs an allowlisted URL and returns
// its content as Markdown (HTML pages are converted; plain text and
// markdown pass through). For richer HTTP (POST, auth, custom
// headers) run mcpsmithy as an MCP server and reference it via the
// project `tools.mcp` catalog instead — credentials stay out of the
// agent's reach.
type WebSkill struct {
	// Enabled toggles the skill. False (or omitted) denies it.
	Enabled bool `yaml:"enabled"`
	// URLs lists the full URLs (scheme+host) to allow, e.g.
	// "https://example.com". Required when Enabled=true.
	URLs []string `yaml:"urls,omitempty"`
}

// Validate rejects the combination that would silently allow any
// URL (Enabled=true with empty URLs).
func (w WebSkill) Validate() error {
	if w.Enabled && len(w.URLs) == 0 {
		return fmt.Errorf("web skill: enabled is true but urls is empty; list allowed URLs or set enabled: false")
	}
	return nil
}

// Param declares one input the caller (LLM as tool, or template
// helper as positional argument) provides when invoking a skill.
type Param struct {
	// Name is used as the tool input name and the template variable
	// inside ShellSkill.Command elements.
	Name string `yaml:"name" agentsmithy:"required,notreserved"`
	// Type drives input validation and template zero-value rendering.
	Type ParamType `yaml:"type" agentsmithy:"required"`
	// Required marks the parameter mandatory.
	Required bool `yaml:"required" agentsmithy:"default=false"`
	// Description is shown to the LLM. Use it to explain the expected format.
	Description string `yaml:"description"`
	// Default value when not supplied. The YAML type must match Type.
	Default any `yaml:"default" agentsmithy:"typed-as=type"`
	// Constraints on the parameter value (enum or numeric range).
	Constraints *ParamConstraints `yaml:"constraints,omitempty" agentsmithy:"typed-as=type"`
}

// Validate checks val against the param's declared constraints.
// Used at invocation time by skill runtimes.
func (p Param) Validate(val any) error {
	c := p.Constraints
	if c == nil {
		return nil
	}
	if len(c.Enum) > 0 {
		for _, allowed := range c.Enum {
			if fmt.Sprint(val) == fmt.Sprint(allowed) {
				return nil
			}
		}
		return fmt.Errorf("value %v not in allowed values %v", val, c.Enum)
	}
	if c.Min != nil || c.Max != nil {
		n, ok := paramToFloat64(val)
		if !ok {
			return fmt.Errorf("expected numeric value for range check, got %T(%v)", val, val)
		}
		if c.Min != nil && n < *c.Min {
			return fmt.Errorf("value %g is below minimum %g", n, *c.Min)
		}
		if c.Max != nil && n > *c.Max {
			return fmt.Errorf("value %g exceeds maximum %g", n, *c.Max)
		}
	}
	return nil
}

// Zero returns a type-appropriate zero value for this param's Type,
// used when rendering templates that reference an unsupplied optional arg.
func (p Param) Zero() any {
	switch p.Type {
	case ParamTypeNumber:
		return float64(0)
	case ParamTypeBool:
		return false
	case ParamTypeArray:
		return []any{}
	default:
		return ""
	}
}

// ParamType names the valid parameter types for skill args.
type ParamType string

const (
	// ParamTypeString accepts JSON Schema string values.
	ParamTypeString ParamType = "string"
	// ParamTypeNumber accepts JSON Schema number values (float64 at runtime).
	ParamTypeNumber ParamType = "number"
	// ParamTypeBool accepts JSON Schema boolean values.
	ParamTypeBool ParamType = "bool"
	// ParamTypeArray accepts JSON Schema array values.
	ParamTypeArray ParamType = "array"
	// ParamTypeProjectFilePath is a string validated as a project-sandbox path at invocation time.
	ParamTypeProjectFilePath ParamType = "project_file_path"
)

// Values returns the set of valid ParamType values. Implements the
// schema.Valuer contract so the schema engine validates `type:`
// fields automatically.
func (ParamType) Values() []string {
	return []string{
		string(ParamTypeString),
		string(ParamTypeNumber),
		string(ParamTypeBool),
		string(ParamTypeArray),
		string(ParamTypeProjectFilePath),
	}
}

// IsNumeric reports whether this type accepts numeric constraints
// (min/max). Implements schema.TypeClassifier.
func (p ParamType) IsNumeric() bool { return p == ParamTypeNumber }

// IsStringLike reports whether this type expects string Go values.
// Implements schema.TypeClassifier.
func (p ParamType) IsStringLike() bool {
	return p == ParamTypeString || p == ParamTypeProjectFilePath
}

// IsBoolean reports whether this type expects bool Go values.
// Implements schema.TypeClassifier.
func (p ParamType) IsBoolean() bool { return p == ParamTypeBool }

// Compatible reports whether v is compatible with this param type.
// YAML unmarshals integers as int, floats as float64, strings as
// string, booleans as bool. Implements schema.TypeClassifier.
func (p ParamType) Compatible(v any) error {
	if v == nil {
		return nil
	}
	switch {
	case p.IsStringLike():
		if _, ok := v.(string); !ok {
			return fmt.Errorf("expected string, got %T(%v)", v, v)
		}
	case p == ParamTypeNumber:
		switch v.(type) {
		case int, float64:
		default:
			return fmt.Errorf("expected number, got %T(%v)", v, v)
		}
	case p.IsBoolean():
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T(%v)", v, v)
		}
	case p == ParamTypeArray:
		if reflect.TypeOf(v).Kind() != reflect.Slice {
			return fmt.Errorf("expected array, got %T(%v)", v, v)
		}
	}
	return nil
}

// ParamConstraints limits what a caller can supply for a parameter.
type ParamConstraints struct {
	// Enum lists the fixed set of valid values. Compatible with string, number, bool.
	Enum []any `yaml:"enum,omitempty" agentsmithy:"oneof?=no_enum_with_min|no_enum_with_max"`
	// Min is the inclusive minimum. Applies to number only.
	Min *float64 `yaml:"min,omitempty" agentsmithy:"oneof?=no_enum_with_min"`
	// Max is the inclusive maximum. Applies to number only.
	Max *float64 `yaml:"max,omitempty" agentsmithy:"oneof?=no_enum_with_max"`
}

func paramToFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
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
// orchestrator's `output:` as `.<name>.{input, output}`. Steps run
// in the order declared in the `steps:` sequence.
type OrchestratorStep struct {
	// Step name. Used as the `.<name>` path in templates.
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
	// `contains <s> <substr>`: reports whether substr is within s.
	BuiltinFuncContains BuiltinFunc = "contains"
	// `trim <s>`: strip leading and trailing whitespace.
	BuiltinFuncTrim BuiltinFunc = "trim"
	// `lower <s>`: lower-case the string.
	BuiltinFuncLower BuiltinFunc = "lower"
	// `upper <s>`: upper-case the string.
	BuiltinFuncUpper BuiltinFunc = "upper"
	// `replace <old> <new> <s>`: replace all occurrences of old with new in s.
	BuiltinFuncReplace BuiltinFunc = "replace"
	// `split <sep> <s>`: split s on sep, returning a slice.
	BuiltinFuncSplit BuiltinFunc = "split"
	// `join <sep> <parts>`: join a slice (string or any) with sep.
	BuiltinFuncJoin BuiltinFunc = "join"
	// `fromJSON <s>`: parse JSON into a Go value (map / slice / scalar).
	BuiltinFuncFromJSON BuiltinFunc = "fromJSON"
)

// builtinFuncStubs returns a parse-time helper registry. The bodies
// are no-op stubs; only names and arities matter for parse-only validation.
func builtinFuncStubs() template.FuncMap {
	return template.FuncMap{
		string(BuiltinFuncTool):     func(string, ...any) (string, error) { return "", nil },
		string(BuiltinFuncAgent):    func(string, ...any) (string, error) { return "", nil },
		string(BuiltinFuncSkill):    func(string, ...any) (any, error) { return nil, nil },
		string(BuiltinFuncPrompt):   func(string, ...any) (string, error) { return "", nil },
		string(BuiltinFuncCoalesce): coalesceStub,
		string(BuiltinFuncDict):     dictStub,
		string(BuiltinFuncList):     listStub,
		string(BuiltinFuncContains): containsStub,
		string(BuiltinFuncTrim):     stringStub,
		string(BuiltinFuncLower):    stringStub,
		string(BuiltinFuncUpper):    stringStub,
		string(BuiltinFuncReplace):  replaceStub,
		string(BuiltinFuncSplit):    splitStub,
		string(BuiltinFuncJoin):     joinStub,
		string(BuiltinFuncFromJSON): fromJSONStub,
	}
}

func coalesceStub(args ...any) any          { return nil }
func dictStub(args ...any) map[string]any   { return nil }
func listStub(args ...any) []any            { return nil }
func containsStub(s, substr string) bool    { return false }
func stringStub(s string) string            { return s }
func replaceStub(old, new, s string) string { return s }
func splitStub(sep, s string) []string      { return nil }
func joinStub(sep string, parts any) string { return "" }
func fromJSONStub(s string) (any, error)    { return nil, nil }
