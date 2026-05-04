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
  service name, the root system prompt, and the models the
  pipeline is allowed to use. Models are grouped by provider.
- **`tools`**: the **tool catalog**. Names declared here (under
  `mcp:` or `a2a:`) are what agents reference in their `tools:` lists.
  No tools means the pipeline is pure-LLM.
- **`pipeline`**: the agent. Exactly one of five **kinds**:
  `autonomous`, `sequential`, `parallel`, `loop`, or `orchestrator`.
  The chosen kind block carries the agent's model, tools, and
  sub-agents. Sub-agents are a `name → kind block` map and recurse
  with the same shape; each sub-agent's `instruction:` lives inside
  its own kind block.

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
only. Sub-agents declare their own `instruction:` *inside their
kind block*; the root prompt is not inherited. Use it to establish
identity and behavior at the top of the pipeline. Don't use it to
encode workflow steps that belong in sub-agent prompts or
orchestrator templates.

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

### Borrowed Provider

`provider: borrowed` delegates completion to the connecting MCP
client via `sampling/createMessage`. The client's own LLM answers
on your behalf. This is the differentiator: the agent runs locally
but the intelligence comes from whatever model the host is already
using (VS Code Copilot, Claude Desktop, etc.).

`maxTokens` is required on every borrowed model entry. `model` is
optional; when set it becomes a preference hint to the client, not
a binding.

Borrowed requires an MCP transport (`mcp-stdio` or `mcp-http`).
It will not work with `a2a` or `stdio` because those transports
have no MCP session to route sampling through.

Borrowed cannot be used on autonomous agents that have tools or
sub-agents. The MCP sampling spec has no field for tool definitions,
so the remote LLM never sees what is callable. If you configure
`tools:` on a borrowed autonomous agent the LLM will hallucinate
fake calls in plain text but no tools will actually execute.
Structural kinds (sequential, parallel, loop) and orchestrator are
not affected; they route mechanically or call tools directly from
templates.

For autonomous agents that need tools, use a direct provider
(`openai`, etc.) on those specific agents. You can mix providers in
the same config: borrowed for text-only agents, a direct provider
for tool-using autonomous agents.


#### Local OpenAI-compatible (Ollama / LM Studio)

Point `baseUrl` at any local OpenAI-compatible server. No API key
needed — the runtime sends an empty `Authorization` header when the
provider is `openai` and `baseUrl` is set.

```yaml
project:
  name: my-assistant
  instruction: |
    A helpful local assistant.
  models:
    openai:
      default:
        model: qwen2.5:7b-instruct
        baseUrl: http://localhost:11434/v1
        temperature: 0.2
        maxTokens: 2048
```

Need a one-shot way to bring Ollama up with a model preloaded? Drop
the snippet below into `docker-compose.yml` next to your agent
config and run `docker compose up -d ollama model`. The `model`
service exits after the pull; `ollama` keeps serving on
`http://localhost:11434`, which is what `baseUrl` above points at.

```yaml
services:
  ollama:
    image: ollama/ollama
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama
    healthcheck:
      test: ["CMD", "ollama", "list"]
      interval: 30s
      timeout: 5s
      retries: 10
      start_period: 15s
  model:
    image: ollama/ollama
    depends_on:
      ollama:
        condition: service_healthy
    environment:
      - OLLAMA_HOST=ollama:11434
      - MODEL=${MODEL:-qwen2.5:7b-instruct}
    entrypoint: ["/bin/sh", "-c", "ollama pull $$MODEL"]
    restart: "no"

volumes:
  ollama_data:
```

#### Multi-tier model catalog

Declare aliases by **task**, not by model. `default` is the workhorse,
`fast` is for cheap classification or routing. Sub-agents pick
the alias they need — switching models later is one edit per alias.

```yaml
project:
  name: research-assistant
  instruction: |
    Research assistant that answers questions from the docs corpus.
  models:
    openai:
      default:
        model: gpt-4o-mini
        temperature: 0.2
      fast:
        model: gpt-4o-nano
        temperature: 0.0
        maxTokens: 256
```

#### Borrowed provider (host-LLM via MCP sampling)

`provider: borrowed` ships an agent into any MCP-aware host without
distributing API keys. The host's LLM (VS Code Copilot, Claude
Desktop, etc.) does the completion via `sampling/createMessage`.
Requires an MCP transport (`mcp-stdio` / `mcp-http`); cannot be paired
with `tools:` or `subagents:` on autonomous agents because MCP
sampling has no field for tool definitions.

```yaml
project:
  name: review-helper
  instruction: |
    Review the supplied diff and summarise risks.
  models:
    borrowed:
      default:
        maxTokens: 1024
```

---

## Tools Section Guide

The `tools` key is the **catalog** of every tool the pipeline can
reach. It is split into two maps by transport:

- `mcp:` for MCP servers reached over Streamable HTTP. Each entry
  is a named endpoint that exposes one or more tools.
- `a2a:` for agentsmithy-or-A2A-compatible services reached over
  HTTP. Each entry is another agent the pipeline can invoke as a
  tool.

The field reference below covers every field; this guide covers
how to use them well.

### The Catalog Is the Allow-List

A name only counts as a tool if it appears under `tools.mcp` or
`tools.a2a`. Agents reference tools by name in their `tools:` list;
references to names not in the catalog fail validation. Treat the
catalog as the security boundary; every tool an agent can call
must be declared here first.

### Names Are What Agents See

The map key (the part to the left of the colon) is the name agents
use in `tools:` lists and in templates (`{{ tool "<name>" ... }}`).
Choose names that describe the **capability**, not the
implementation:

- `docs`, not `localhost-8080`
- `code-search`, not `mcp-server-1`
- `reviewer`, not `review-agent-v2`

Names appear in agent prompts (the LLM sees the tool list), so they
should read as actions to a model. Short and concrete beats long
and clever.

### MCP vs. A2A: Pick by Protocol

- **MCP** when the endpoint speaks MCP, a skill server exposing
  one or more tools over Streamable HTTP. The agent treats each
  MCP tool as an individually-named callable.
- **A2A** when the endpoint speaks A2A, another agentsmithy
  service or A2A-compatible agent. The agent treats it as a single
  callable that takes input and returns output.

Protocol picks the map; what's behind the protocol picks the
endpoint.

### Catalog Once, Reference Many Times

A name in the catalog can be referenced from any number of agents
in the pipeline tree. Don't declare the same MCP server twice with
different names just to give two agents "their own" copy. They're
sharing the same endpoint either way.

### Don't List Tools You Don't Need

Every catalog entry shows up in the agent's tool list and competes
for attention from the LLM. Drop endpoints you're not actively
using.


#### MCP toolset (paired mcpsmithy server)

The canonical companion: an `agentsmithy` agent + an `mcpsmithy`
server, wired together. The agent calls the server over MCP for
docs search, convention lookup, and any tool the server exposes.

```yaml
tools:
  mcp:
    docs: "http://localhost:8080/sse"
```

Then reference it from any agent's `tools:` list:

```yaml
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    tools: [docs]
```

#### A2A: another agentsmithy agent as a tool

`tools.a2a` lets one agent call another agentsmithy service as a
single tool. Use it to compose pipelines across processes — the
caller doesn't see the callee's internal sub-agents, only its
top-level reply.

```yaml
tools:
  a2a:
    reviewer: "http://localhost:9090/"
```

---

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


#### Autonomous (single-agent chat)

The smallest useful config: one LLM, no tools. Run with
`agentsmithy serve --transport=stdio` and talk to it.

```yaml
version: "1"
project:
  name: chat
  instruction: |
    You are a helpful assistant.
  models:
    openai:
      default: { model: gpt-4o-mini }
pipeline:
  autonomous:
    model: { provider: openai, name: default }
```

#### Autonomous with sub-agent delegation

Sub-agents on an autonomous parent are **delegation targets**. The
parent's LLM may hand control off via `transfer_to_agent`. Children
inherit the parent's `model:` so the catalog stays minimal.

```yaml
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    instruction: |
      Triage the user's question and hand off to a specialist.
    subagents:
      - name: code-helper
        autonomous:
          instruction: Answer code questions.
          inherits: [model]
      - name: docs-helper
        autonomous:
          instruction: Answer documentation questions.
          inherits: [model]
```

#### Sequential pipeline (researcher → reviewer → refiner)

Steps run in declaration order; each child sees the prior child's
output. The last child's reply is the parent's default output.

```yaml
pipeline:
  sequential:
    model: { provider: openai, name: default }
    subagents:
      - name: researcher
        autonomous:
          instruction: Gather facts on the user's topic.
          inherits: [model, tools]
      - name: reviewer
        autonomous:
          instruction: Critique the research for gaps.
          inherits: [model]
      - name: refiner
        autonomous:
          instruction: Rewrite the research addressing the critique.
          inherits: [model]
    tools: [docs]
```

#### Parallel fan-out with synthesis

Children run concurrently against the same input. The default output
is a `name → output` map; declare an explicit `output:` to shape it
for downstream consumers.

```yaml
pipeline:
  parallel:
    model: { provider: openai, name: default }
    subagents:
      - name: web
        autonomous:
          instruction: Summarise relevant web search results.
          inherits: [model]
      - name: docs
        autonomous:
          instruction: Summarise relevant internal docs.
          inherits: [model]
    output: |
      Web: {{ .web.output }}
      Docs: {{ .docs.output }}
```

#### Loop with `requireToolCall` guard

A loop repeats until `until:` renders non-empty, the body emits
`exit_loop`, or `maxIterations` is hit. Pair it with the
`requireToolCall` guard on an autonomous child to force structured
tool use until the exit predicate is satisfied.

```yaml
pipeline:
  loop:
    model: { provider: openai, name: default }
    maxIterations: 5
    until: '{{ skill "tests-pass" .codegen.output }}'
    subagents:
      - name: codegen
        autonomous:
          instruction: |
            Edit the code until tests pass. Use the file and shell
            tools — don't answer in plain text.
          inherits: [model, tools, skills]
          skills:
            guards: [requireToolCall]
          maxIterations: 3
```

#### Orchestrator (explicit step graph)

`orchestrator` is for fixed flows where the *shape* of the work is
known up front. Steps invoke tools, sub-agents, or one-shot prompts
via Go-template helpers; each step's record is exposed to subsequent
steps as `.<stepName>.output`.

```yaml
pipeline:
  orchestrator:
    model: { provider: openai, name: default }
    tools: [docs]
    subagents:
      - name: web
        autonomous:
          instruction: Search the web.
          inherits: [model]
    steps:
      - name: research
        run: '{{ tool "docs" .input }}'
      - name: web_run
        run: '{{ agent "web" .input }}'
      - name: combine
        run: |
          Docs: {{ .research.output }}
          Web:  {{ .web_run.output }}
    output: '{{ .combine.output }}'
```

#### Skills: shell + file + web sandboxes

Skills bind built-in capabilities to an agent. `shell` declares
allow-listed commands, `file` gates sandboxed read/write rooted at
`workingDir`, and `web` allow-lists URLs for single-page scraping. Each block
can be exposed as ADK tools (autonomous) or as `{{ skill "name" }}`
helpers (composition kinds).

```yaml
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    instruction: |
      Help maintain this project. Use the available skills before
      answering.
    skills:
      shell:
        run-tests:
          command: ["go", "test", "./..."]
          workingDir: "."
      file:
        workingDir: "."
        read:
          enabled: true
          paths: ["**/*.go", "**/*.md"]
        write:
          enabled: true
          paths: ["docs/**/*.md"]
      web:
        get:
          enabled: true
          urls: ["https://pkg.go.dev"]
```
