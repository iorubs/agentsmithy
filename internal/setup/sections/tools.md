## Tools Section Guide

The `tools` key is the **catalog** of every tool the pipeline can
reach. It is split into two maps by transport:

- `mcp:` for MCP servers reached over SSE/HTTP. Each entry is a
  named endpoint that exposes one or more tools.
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
  one or more tools over SSE/HTTP. The agent treats each MCP tool
  as an individually-named callable.
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
