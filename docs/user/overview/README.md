---
slug: /
---

# Overview

![agentsmithy](../static/img/body.png)

> **AgentSmithy** is a single Go binary that reads a declarative YAML
> config file (`.agentsmithy.yaml`) and serves a fully functional AI
> agent over the transport of your choice.

## Why does it exist?

Plenty of tools let you build a single agent from YAML. AgentSmithy's
differentiator is **complex multi-agent flows declared entirely in
config**: sequential pipelines, parallel fan-outs, and bounded
refinement loops, with sub-agents that can themselves be agents,
tools, or remote A2A endpoints; no glue code required.

It is the agent counterpart to
[mcpsmithy](https://iorubs.github.io/mcpsmithy/), and slots into
[smithy-cli](https://iorubs.github.io/smithy-cli/) as the runtime
behind `smithy agent ...`.

## How it works

1. Author a `.agentsmithy.yaml` describing one agent: its model(s),
   tools, system prompt, and how it should be served.
2. Run `agentsmithy validate` to catch errors before you start.
3. Run `agentsmithy serve` to expose the agent over a transport, or
   `agentsmithy chat` to talk to it directly from a terminal.

AgentSmithy serves one root agent per process. That root may be a
single autonomous agent or an orchestrator (`sequential`, `parallel`,
`loop`) coordinating sub-agents inside the same process. To run
multiple independent agents alongside each other (and alongside MCP
servers), compose them at the **stack** level via
[smithy-cli](https://iorubs.github.io/smithy-cli/).

## Reference

- [Config Reference](../reference/config/README.md)
- [CLI Reference](../reference/server/README.md)
