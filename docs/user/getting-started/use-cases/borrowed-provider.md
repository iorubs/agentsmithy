---
sidebar_position: 5
---

# Borrowed Provider

`provider: borrowed`; the agent has no LLM credentials of its own.
Instead, it asks the connecting MCP client to do the LLM call on its
behalf via MCP sampling (`createMessage`). The host application keeps
control of model choice, billing, and policy; the agent supplies
the prompt, tools, and orchestration.

This makes it possible to ship an AgentSmithy agent into any MCP-aware
host without distributing API keys.

> _Available once the `borrowed` provider lands._
