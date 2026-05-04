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
