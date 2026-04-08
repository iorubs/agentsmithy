# Config

Auto-generated schema and authoring reference for `.agentsmithy.yaml`.

## Schema Versions

- [Version 1](v1.md)

---

## General Config Guide

You are helping the user write or improve a `.agentsmithy.yaml` file.
This guide gives you the structure; call `config_section` for the
details of each section you need to write.

### What the File Does

`.agentsmithy.yaml` defines one **pipeline**: a single agent or a
composition of agents that runs as one service. `agentsmithy serve`
reads this file and exposes the pipeline over a transport (a2a, MCP
stdio, or MCP HTTP). External callers (humans via `chat`, other
agents via A2A, or MCP clients) talk to the root pipeline; sub-agents
and tools are wired internally.

### Top-Level Shape

The file has three sections:

- **`project`**: identity and the **model catalog**. Declares the
  service name, the root system prompt, and the models the pipeline
  is allowed to use. Models are grouped by provider.
- **`tools`**: the **tool catalog**. Names declared here (under
  `mcp:` or `a2a:`) are what agents reference in their `tools:` lists.
  No tools means the pipeline is pure-LLM.
- **`pipeline`**: the agent. Exactly one of five **kinds**:
  `autonomous`, `sequential`, `parallel`, `loop`, or `orchestrator`.
  Sub-agents nest inside the kind block and recurse with the same
  shape. In `autonomous`, sub-agents are delegation targets the LLM
  may hand control to (`transfer_to_agent`); in the composition
  kinds (`sequential`, `parallel`, `loop`), they are the children
  the kind runs.

### Pick the Right Kind

The kind drives every other decision. From simplest to most flexible:

- **`autonomous`**: one LLM, decides its own tool calls. Use this
  unless you have a concrete reason not to.
- **`sequential`**: sub-agents run in order, each seeing the prior's
  output. Use when steps are linear and the order matters.
- **`parallel`**: sub-agents run concurrently against the same
  input. Outputs are exposed as `.<name>` for the `output:`
  template to combine. Use for fan-out (panel of reviewers,
  multi-source gather).
- **`loop`**: repeats until the body emits `exit_loop`, `until:`
  renders non-empty, or `maxIterations` is hit. Use for iterative
  refinement.
- **`orchestrator`**: explicit `steps[]` graph wired with Go
  templates. Use only when the other kinds don't compose what you
  need; step graphs cost readability for flexibility.

### Inheritance Beats Repetition

Sub-agents can `inherits: [model, tools, skills]` from the nearest
ancestor that declared each field. Declare common configuration once
on the parent kind and let children inherit. Local declarations on a
sub-agent always win; `inherits:` only fills gaps.

### Templates Are Parsed at Validate Time

`run:`, `output:`, and `until:` are Go `text/template` bodies.
Syntax errors and references to unknown helpers are caught when the
config is loaded. Variable references (`.input`, `.<stepName>.output`)
are validated at runtime, not load time.

### Decision Rules

- **One model or many?** Declare aliases per task: e.g. `default`
  for the main model, `fast` for cheap classification, `vision` for
  multimodal. Sub-agents pick which alias they need via `model:` or
  inherit the parent's choice.

- **MCP tool or A2A tool?** Use `tools.mcp` for endpoints that
  speak MCP (skill servers exposing one or more tools). Use
  `tools.a2a` for endpoints that speak A2A (other agentsmithy
  services or A2A-compatible agents).

- **Sub-agent or tool?** A sub-agent gets the same kind tree (LLM
  loop, sub-children, memory). A tool is a single call returning a
  string. If you need reasoning between calls, it's a sub-agent.

- **`autonomous` with `tools:` or an `orchestrator` step graph?**
  Autonomous if the LLM should decide *when* to call each tool.
  Orchestrator if the *order and shape* of tool calls is fixed.

### Next Steps

Call `config_section` for each section you need to write:

- `config_section section=project`: service identity and model
  catalog
- `config_section section=tools`: tool catalog (MCP and A2A entries)
- `config_section section=pipeline`: agent kinds, sub-agents,
  inheritance, memory, templates, guards


---

## Project Section Guide

The `project` key declares **what this config is**: the service name,
the root system prompt, and the model catalog the pipeline draws
from. The field reference below covers every field; this guide
covers strategy.

### The Service Name Is Public

`name` shows up in logs, in the A2A service descriptor other agents
use to address this pipeline, and in CLI output. Pick a name that
makes sense to a stranger reading your stack: usually the role of
the pipeline (`docs-assistant`, `release-coordinator`), not the
implementation detail.

### The Root Instruction Sets Tone, Not Wiring

`instruction` is the system prompt for the **root** pipeline agent
only. Sub-agents declare their own `instruction:`; the root prompt
is not inherited. Use it to establish identity and behavior at the
top of the pipeline. Don't use it to encode workflow steps that
belong in sub-agent prompts or orchestrator templates.

When the root pipeline is composition-only (sequential, parallel,
loop, orchestrator), the instruction still applies; it's the
context the orchestrating layer carries when invoking children.
Keep it short.

### Model Catalog: Aliases, Not Models

`models:` is a map of provider key (`ollama`, `openai`) to a map
of **author-chosen aliases** (`default`, `fast`, `vision`,
`long-ctx`) to model entries. Aliases are what `model:` refs point
at, never raw provider model IDs.

Aliases let you change the underlying model without touching every
sub-agent. Switching from `gpt-4o-mini` to `gpt-4.1-mini` is one
edit in the catalog if every reference uses the `default` alias.

### Declare Aliases by Task, Not by Model

Pick aliases that describe **what the model is for**, not what it is.
`default`, `fast`, `vision`, `summarize-long-doc` are durable.
`gpt4`, `claude-haiku`, `local-llama` rot the moment you change
providers.

### Same Provider Twice Is Fine

Two `openai` entries with different `model:` and the same `baseUrl`
is the normal case for a multi-tier pipeline. Two entries with
different `baseUrl` (e.g. one targeting OpenAI native, one targeting
LM Studio) is also fine; they're independent aliases.

### Provider Defaults and Overrides

`baseUrl` is required for OpenAI-compatible servers (LM Studio,
vLLM, Together, Groq) and optional for the native OpenAI provider.
For Ollama, `baseUrl` defaults to the local daemon; set it only if
you target a remote ollama instance.

`temperature` and `maxTokens` on a model entry are catalog defaults.
Per-call overrides happen at the agent level; v0.1 keeps them on the
catalog only.

### Don't Pre-Declare Models You Won't Use

Every alias in `models:` is part of the validated config. Unused
aliases pass validation but waste reviewer attention. Add aliases
when an agent needs them, not in advance.

