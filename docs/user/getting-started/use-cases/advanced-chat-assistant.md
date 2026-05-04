---
sidebar_position: 2
---

# Advanced Chat Assistant

## User story

As a **product or platform engineer**, I want answers that are better
than what one LLM round-trip can give me — running specialists in
sequence, fanning out to a panel and synthesising, or iterating
until a quality bar is met — so the assistant can handle questions
that need research + critique + rewrite, or multiple perspectives,
without exposing that orchestration to the user.

## Goals

- Compose multiple agents into one externally-facing service
- Mix kinds: a `sequential` outer flow with `parallel` fan-out
  inside, or a `loop` wrapping a code-writing agent that retries
  until tests pass
- Reuse one model alias across the tree (sub-agents inherit it)
- Keep the same transport surface as Simple Chat — the caller
  doesn't know whether they're talking to one agent or twelve

## Technical overview

**Kinds beyond autonomous:**
- **`sequential`** — children run in declaration order, each seeing
  the prior's output. Use for research → review → refine flows
  where order matters.
- **`parallel`** — children run concurrently against the same input.
  Use for "panel of specialists" or fan-out across data sources;
  shape the merged answer with the parent's `output:` template.
- **`loop`** — repeats its body until `until:` renders non-empty,
  the body emits `exit_loop`, or `maxIterations` is hit. Use for
  iterative refinement with an explicit exit condition.
- **`orchestrator`** — explicit step graph wired with Go templates.
  Reach for this only when the other kinds don't compose what you
  need; see [Borrowed Provider](borrowed-provider.md) for an
  introduction.

**Quality gates with loops + guards:** wrap an autonomous code-writer
in a `loop` whose `until:` predicate calls a `tests-pass` skill, and
configure the inner agent with `skills.guards: [requireToolCall]` so
each iteration must actually call a tool rather than answer in
plain text. The loop exits the moment the tests pass; the
`maxIterations` field caps the retries.

**Inheritance keeps the YAML small:** declare `model:` once on the
parent kind and let children pull it via `inherits: [model]`.
Same trick works for `tools:` and `skills:`.

**Output composition:** each child's rendered `output:` is exposed
to the parent as `.<childName>.output`. A `sequential`'s default
output is the last child's; a `parallel`'s default is a
`name → output` map. Override `output:` on the parent to combine
them however you want.

For copy-paste YAML, see:
- [Sequential pipeline (researcher → reviewer → refiner)](../../reference/config/README.md#sequential-pipeline-researcher-%E2%86%92-reviewer-%E2%86%92-refiner)
- [Parallel fan-out with synthesis](../../reference/config/README.md#parallel-fan-out-with-synthesis)
- [Loop with `requireToolCall` guard](../../reference/config/README.md#loop-with-requiretoolcall-guard)
- [Multi-tier model catalog](../../reference/config/README.md#multi-tier-model-catalog)

:::tip Generate this config with your agent
Run `agentsmithy setup`, then describe the flow you want. Before
prompting, have ready:

- The shape (linear hand-off / fan-out + synthesis / loop until
  done)
- What each sub-agent does in its own words
- Any tools or skills the children need (research API, file access,
  shell)
- A model alias plan: one default, or a `default` + `fast` split if
  some sub-agents are cheap classifiers

Then use a prompt like:

> Set up a sequential pipeline: a researcher gathers facts, a
> reviewer critiques the gaps, and a refiner produces the final
> answer. All three should inherit the model from the parent. The
> researcher needs the `docs` MCP server. Expose this over A2A so
> another agent can call us.

See [Assisted setup](../guided-setup.md) for the full workflow.
:::
