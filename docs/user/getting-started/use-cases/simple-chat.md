---
sidebar_position: 1
---

# Simple Chat

## User story

As a **developer**, I want to spin up a chat assistant in a minute —
backed by a local or hosted model, optionally with one or two tools
or sandboxed skills — so I can put it in front of users (or
myself) and iterate on the prompt without writing any code.

## Goals

- One YAML file, one process, one transport — no glue code
- Use whatever model is convenient: Ollama / LM Studio locally, or any
  OpenAI-compatible endpoint
- Add tools or skills incrementally as the agent's job grows
- Talk to it from the terminal (`stdio`) or expose it
  over MCP / A2A for an editor or another agent to call

## Technical overview

**Kind:** `autonomous`. One LLM that decides its own tool calls.
This is the default shape and what every other use case builds on.

**Transport:** `stdio` for local chat. Switch to `mcp-stdio` if you
want a code editor (Claude Desktop, VS Code) to launch the agent
as an MCP server, or `a2a` to expose it as a network service other
agents can call.

**Model:** anything OpenAI-compatible. Pointing `baseUrl` at a local
Ollama / LM Studio means no API key. Pointing it at OpenAI proper
means setting `OPENAI_API_KEY` and leaving `baseUrl` empty.

**Tools (optional):** declare an MCP server under `tools.mcp` and
add its name to the agent's `tools:` list. The classic pairing is
agentsmithy + mcpsmithy — your agent gets ranked search and
convention lookup with one extra YAML block.

**Skills (optional):** sandboxed local capabilities the runtime
exposes as ADK tools without an external server.
- `shell` — allow-listed commands the agent can run (e.g. `go test`)
- `file` — read/write within a `workingDir` with a path allowlist
- `web` — single-page scrape against an explicit URL allowlist

**Guards (optional):** `skills.guards: [requireToolCall]` forces the
LLM to call a tool before answering, which is useful when you want
research-first behavior. Pair with `maxIterations:` to cap retries.

For copy-paste YAML, see:
- [Local OpenAI-compatible](../../reference/config/README.md#local-openai-compatible-ollama--lm-studio)
- [Autonomous (single-agent chat)](../../reference/config/README.md#autonomous-single-agent-chat)
- [MCP toolset (paired mcpsmithy server)](../../reference/config/README.md#mcp-toolset-paired-mcpsmithy-server)
- [Skills: shell + file + web sandboxes](../../reference/config/README.md#skills-shell--file--web-sandboxes)

:::tip Generate this config with your agent
Run `agentsmithy setup`, then describe what you want. Before
prompting, have ready:

- The model you want (local Ollama tag, hosted endpoint, or "borrowed"
  if you want the host's LLM to do the work)
- Any MCP servers the agent should call (URL or stdio command)
- Any local capabilities you want gated as skills (commands to allow,
  files to allow, URLs to allow)

Then use a prompt like:

> Set up a single autonomous agent that runs locally against
> Ollama. Give it the `docs` MCP server I'm running on
> `localhost:8080/`. Allow `go test ./...` as a shell skill so it
> can verify changes. Stdio transport — I want to chat with it from
> the terminal.

See [Assisted setup](../guided-setup.md) for the full workflow.
:::
