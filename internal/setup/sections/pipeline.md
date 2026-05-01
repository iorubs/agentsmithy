## Pipeline Section Guide

The `pipeline` key declares the agent that runs when the service is
invoked. Exactly one **kind** must be set. Sub-agents nest inside
the kind block and recurse with the same shape. The field reference
below covers every field; this guide covers how to choose between
kinds and how to wire them well.

### Choosing a Kind

- **`autonomous`**: one LLM that decides its own tool calls. Its
  `subagents:` are delegation targets the LLM may hand control to
  via `transfer_to_agent`. Default choice for any single-purpose
  agent.
- **`sequential`**: sub-agents run in declaration order, each
  seeing the prior one's output. Use when steps are linear.
- **`parallel`**: sub-agents fan out concurrently against the same
  input. Each child's output is exposed to the `output:` template
  as `.<childName>.output`.
- **`loop`**: repeats its sub-agents until the body emits
  `exit_loop`, the `until:` template renders non-empty, or
  `maxIterations` is hit.
- **`orchestrator`**: explicit `steps[]` graph wired with Go
  templates. Use only when the other kinds don't compose what you
  need.

Start with autonomous. Promote to a composition kind when the LLM
provably cannot decide the structure on its own.

### Inheritance, Not Repetition

Every sub-agent can declare `inherits: [model, tools, skills]` to
pull those fields from the nearest ancestor that declared them.
Local declarations always win; `inherits:` only fills gaps.
`instruction:` is not inheritable; every sub-agent declares its own.

### Output Templates and Their Defaults

Every kind has an `output:` field. Without one, defaults apply:
the LLM's final reply for `autonomous`, the last child's output for
`sequential`, a `name→output` map for `parallel`, the last
iteration's body output for `loop`. `orchestrator` requires an
explicit `output:`.

The `model:` on a composition kind backs the `{{ prompt }}` helper
its `output:` template can call, and is the inheritance source for
descendants.

### Memory Defaults Are Position-Aware

`memory.retain` defaults to `true` for the root autonomous agent
and for loop-body children (where the loop *is* the conversation).
It defaults to `false` everywhere else.

`memory.inherit` defaults to `false`. Set it to `true` only when a
child needs the parent's transcript in addition to its hand-off
input. Most agents leave memory unset.

### Templates Are Parsed at Validate Time

`run:`, `output:`, and `until:` are Go `text/template` bodies.
Syntax errors and references to unknown helpers fail at config
load. Variable references (`.input`, `.<stepName>.output`,
`.<childName>.output`) are validated at runtime.

### Helpers

Side-effecting: `tool`, `agent`, `skill`, `prompt`. Pure:
`coalesce`, `dict`, `list`. The full catalog is in the `BuiltinFunc`
enum in the field reference. `until:` cannot call `agent`; exit
predicates must not recurse into the loop body.

### `maxIterations` Has Two Meanings

- On `loop`: hard cap on body iterations.
- On `autonomous`: cap on guard-driven retries within a single
  turn (paired with `skills.guards:`).

Both are `>= 1`.

### Orchestrator Steps Are a Last Resort

`orchestrator` exists for cases the other four kinds can't compose,
typically when you need to build a structured input from multiple
prior outputs, or invoke the same sub-agent twice with different
framing. Step graphs are harder to read and change than
instruction-driven LLM loops. If a sequential pipeline with clear
instructions does the job, use it.

A step's `run:` that renders to whitespace-only is treated as
**skipped**: its `.<stepName>.input` and `.output` are absent from
the template scope, so downstream `coalesce` / `if` checks fall
through naturally.

### Guards

`skills.guards:` lists built-in guards that run alongside the
agent's LLM loop. v0.1 ships `requireToolCall` only; it forces the
LLM to issue at least one tool call per turn and pairs with
`maxIterations:` to cap retries. Add guards only when the agent's
behavior actually needs constraining.
