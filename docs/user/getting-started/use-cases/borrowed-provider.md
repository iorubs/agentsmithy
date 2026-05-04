---
sidebar_position: 3
---

# Borrowed Provider

## User story

As a **developer using an MCP-aware host** (VS Code Copilot, Claude
Desktop, Cursor) or as a **vendor shipping an agent into one**, I want
to package a custom agent — with its own prompt, tools, skills, and
flow — that uses **the host's LLM** for completions instead of an
API key I have to ship and bill against. The host stays in charge of
model choice and policy; I stay in charge of structure.

## Goals

- Run an agent inside any MCP-aware client without distributing API
  keys or model credentials
- Let the host's user (or admin) pick the model — your agent doesn't
  care which LLM does the work
- Keep full control over the flow: tools, skills, guards, loops,
  orchestrator step graphs all work the same as any other agent
- Ship one config that works against Copilot, Claude Desktop, Cursor,
  or any other MCP host that supports `sampling/createMessage`

## Technical overview

**Provider:** `borrowed`. Each completion is delegated to the
connecting MCP client via `sampling/createMessage`. The client's own
LLM does the work; your agent supplies the prompt and orchestration.

**Transport:** must be MCP — `mcp-stdio` for local hosts, `mcp-http`
for networked ones. `a2a` and plain `stdio` have no MCP session to
route sampling through, so they cannot use borrowed providers.

**One important limit:** the MCP sampling spec carries messages and a
system prompt but **has no field for tool definitions**. So a
borrowed *autonomous* agent with `tools:` or `subagents:` won't work
— the remote LLM never sees what's callable and will hallucinate
fake calls in plain text. Two clean ways around this:

1. **Mix providers.** Use `borrowed` only on text-generating
   sub-agents. Keep direct providers (`openai`, etc.) on autonomous
   agents that need real tool calls. The same config can run both.
2. **Use orchestrator or loop.** These kinds call tools and
   sub-agents from templates — `{{ tool "name" arg }}`,
   `{{ agent "name" arg }}` — bypassing the LLM's tool-call
   negotiation entirely. The borrowed LLM is only used for
   `{{ prompt "..." }}` text generation inside steps.

### Looping under host control

A `loop` with a borrowed text generator and an `until:` predicate is
a clean way to ship iterative refinement into a host: the host's LLM
drafts and revises, the runtime decides when to stop, and your agent
keeps the convergence policy out of the LLM's hands.

### Deterministic orchestration under host control

`orchestrator` lets you ship a fixed step graph with the host's LLM
as the text-generation engine for `{{ prompt }}` calls inside steps.
Tool invocations and sub-agent hand-offs happen as template-driven
side effects, so the entire flow is reproducible regardless of which
model the host is using.

### Skills are still yours

Even when the LLM is borrowed, `skills.shell` / `skills.file` /
`skills.web` execute locally inside the agent process. So you can
ship an agent that runs commands, edits files, or hits an API
allowlist on the host machine while the LLM lives elsewhere. Pair
this with the orchestrator pattern above (skills called from
`run:` / `output:` templates) for full control.

**Guards still apply** to the autonomous-with-direct-provider parts
of a mixed config — `skills.guards: [requireToolCall]` works
exactly the same as in [Simple Chat](simple-chat.md).

For copy-paste YAML, see:
- [Borrowed provider (host-LLM via MCP sampling)](../../reference/config/README.md#borrowed-provider-host-llm-via-mcp-sampling)
- [Loop with `requireToolCall` guard](../../reference/config/README.md#loop-with-requiretoolcall-guard)
- [Orchestrator (explicit step graph)](../../reference/config/README.md#orchestrator-explicit-step-graph)
- [Skills: shell + file + web sandboxes](../../reference/config/README.md#skills-shell--file--web-sandboxes)

:::tip Generate this config with your agent
Run `agentsmithy setup`, then describe what you want shipped into
the host. Before prompting, have ready:

- The host you're targeting (VS Code, Claude Desktop, Cursor) — picks
  between `mcp-stdio` (local) and `mcp-http` (network)
- The flow: pure text generation, loop until a condition, or
  orchestrator step graph
- Any tools or skills the orchestrator should drive — these run
  locally even though the LLM is borrowed
- Whether parts of the agent need real tool calls (which means a
  mixed config: borrowed text agents + direct-provider autonomous
  agents)

Then use a prompt like:

> Set up an agent that ships into Claude Desktop. The orchestrator
> runs four steps: search the docs via the `docs` MCP server,
> summarise via a `{{ prompt }}` call against the borrowed LLM, fact-
> check by calling the `fact-checker` sub-agent, and emit a final
> rendered answer. Use `mcp-stdio` so Claude Desktop can launch us.

See [Assisted setup](../guided-setup.md) for the full workflow.
:::
