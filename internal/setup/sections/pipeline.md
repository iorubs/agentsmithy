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

### Borrowed Provider Limitations

Borrowed delegates completion to the MCP client via
`sampling/createMessage`. The MCP sampling spec carries messages
and a system prompt but has no field for tool or function
definitions. The remote LLM never sees what tools exist.

This means a borrowed **autonomous** agent cannot make tool calls
or transfer control to sub-agents. If you configure `tools:` or
`subagents:` on a borrowed autonomous agent the LLM will
hallucinate fake calls in plain text instead of issuing real
structured tool calls. The tools never execute.

Structural kinds are not affected. Sequential, parallel, and loop
route mechanically without presenting tools to an LLM.
Orchestrator calls tools directly from its `run:` templates via
`{{ tool }}`, bypassing the LLM entirely. All of these work fine
with borrowed.

Two ways to get real tool calls from an autonomous agent:

1. Use a direct provider (`openai`, etc.) on autonomous agents
   that need tools or sub-agents. You can mix providers in the
   same config: borrowed for text-only agents, openai for
   tool-using agents.
2. Wrap tool calls in an orchestrator. The orchestrator's
   templates call tools directly; borrowed is only used for
   `{{ prompt }}` text generation inside steps.

### Inheritance, Not Repetition

Every sub-agent can declare `inherits: [model, tools, skills]`
inside its kind block to pull those fields from the nearest
ancestor that declared them. Local declarations always win;
`inherits:` only fills gaps. `instruction:` is not inheritable;
the root pipeline's instruction lives at `project.instruction`,
and each sub-agent declares its own inside its kind block.

### Output Templates and Their Defaults

Every kind has an `output:` field. Without one, defaults apply:
the LLM's final reply for `autonomous`, the last child's output for
`sequential`, a `name→output` map for `parallel`, the last
iteration's body output for `loop`. `orchestrator` requires an
explicit `output:`.

The `model:` on a composition kind backs the `{{ prompt }}` helper
its `output:` template can call, and is the inheritance source for
descendants.

### Accessing Sub-Agent Outputs

Each sub-agent's rendered `output:` is written to session state
under its `name:`. A parent reads it back through
`.<childName>.output` in its own `output:` template. A child whose
`output:` is empty is **absent** from the parent scope; use
`{{ if .child.output }}` or `{{ coalesce .child.output "fallback" }}`
rather than asserting presence.

`.<childName>.input` is reserved in the scope but currently
renders empty; only `.output` carries data today.

#### Sequential

Only the last child's output flows automatically. To combine
multiple children, declare an `output:` on the sequential parent
and reference each by name:

```yaml
sequential:
  instruction: ...
  subagents:
    extract:
      autonomous:
        instruction: ...
    summarize:
      autonomous:
        instruction: ...
  output: |
    Extract: {{ .extract.output }}
    Summary: {{ .summarize.output }}
```

#### Parallel

All children run concurrently against the same input. Each child's
output is independently addressable, and the default output is a
`name→output` map. Declare an explicit `output:` when the consumer
needs a specific shape:

```yaml
parallel:
  instruction: ...
  subagents:
    web:
      autonomous:
        instruction: ...
    docs:
      autonomous:
        instruction: ...
  output: |
    web: {{ .web.output }}
    docs: {{ .docs.output }}
```

A parent of the parallel agent (e.g. an enclosing sequential)
reads the parallel block's *single rendered* `output:` as
`.<parallelName>.output`, not a per-child map. If a downstream
consumer needs per-child fields, emit structured content (JSON,
labelled lines) from the parallel's `output:` and parse there.

#### Loop

Only the last iteration's body output is exposed by default. The
loop wrapper does not aggregate per-iteration outputs; if each
iteration needs to feed the next, the body itself must carry that
state forward.

#### Autonomous

An autonomous agent's reply is its LLM's final message, surfaced to
the parent under its name automatically. Note: `output:` on
autonomous is not yet honoured (rejected at config load); shape
the reply via the agent's `instruction:` instead.

#### Orchestrator

Step records are exposed as `.<stepName>.{input, output}` to
subsequent steps and to the orchestrator's `output:`, mirroring
the sequential/parallel scope shape. Inside step `run:` and the
orchestrator's `output:`, three side-effecting helpers are live:

- `{{ tool "name" arg }}`: invoke a tool from this orchestrator's
  `tools:` list.
- `{{ agent "name" arg }}`: invoke a named sub-agent and return
  its rendered output.
- `{{ prompt "text" }}`: one-shot LLM call against the
  orchestrator's `model:`.

A step whose `run:` renders to whitespace-only is **skipped**: its
slot is absent from scope, so downstream `{{ if .step.output }}`
and `{{ coalesce .step.output ... }}` checks fall through.

The `agent` helper is unavailable inside a loop's `until:`
predicate (exit predicates must not recurse into the body) and
inside sequential/parallel/loop `output:` templates: those
callbacks lack the invocation context required to run sub-agents
or tools. Use orchestrator when helper composition is needed.

```yaml
orchestrator:
  instruction: ...
  model: { provider: openai, name: default }
  tools: [search]
  subagents:
    web:
      autonomous:
        instruction: ...
    docs:
      autonomous:
        instruction: ...
  steps:
    web_run:
      run: '{{ agent "web" .input }}'
    docs_run:
      run: '{{ agent "docs" .input }}'
    combine:
      run: |
        web: {{ .web_run.output }}
        docs: {{ .docs_run.output }}
  output: '{{ .combine.output }}'
```

### Memory Defaults Are Position-Aware

`memory.retain` defaults to `true` for the root autonomous agent
and for loop-body children (where the loop *is* the conversation).
It defaults to `false` everywhere else.

`memory.inherit` defaults to `false`. Set it to `true` only when a
child needs the parent's transcript in addition to its hand-off
input. Most agents leave memory unset.

### Helper Scope Across Kinds

The full helper catalog is in the `BuiltinFunc` enum in the field
reference. The scoping rules below describe where each side-effecting
helper is callable:

- `prompt`: live in every `output:`, `until:`, and `run:` template
  whose enclosing kind declares a `model:`.
- `tool` and `agent`: live only inside `orchestrator` (`run:` and
  `output:`). Other kinds' callbacks lack the invocation context
  these helpers need.
- `agent`: additionally forbidden inside a loop's `until:` so exit
  predicates cannot recurse into the loop body.
- Variable references (`.input`, `.<name>.output`) are validated at
  runtime; helper-name typos and template syntax errors fail at
  config load.

### `maxIterations` Has Two Meanings

- On `loop`: hard cap on body iterations.
- On `autonomous`: cap on guard-driven retries within a single
  turn (paired with `skills.guards:`).

Both are `>= 1`.

### When to Use Orchestrator

`orchestrator` is the right kind when you need explicit, deterministic
control over the flow: a fixed step graph that builds structured
inputs from prior outputs, invokes the same sub-agent multiple
times with different framing, or interleaves tool calls and
sub-agent calls in a specific order. Pick it when the *shape* of
the work is known up front and you want a reproducible execution
trace rather than an LLM deciding the structure.

Prefer the other kinds when their semantics already match: a
linear hand-off is `sequential`; concurrent fan-out is `parallel`;
a retry-until-condition is `loop`; an LLM choosing among tools is
`autonomous`. Reach for `orchestrator` when those don't compose
the flow you want.

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
