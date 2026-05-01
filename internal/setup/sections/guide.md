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
