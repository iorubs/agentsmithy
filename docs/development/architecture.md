# Architecture

## Overview

agentsmithy is a single Go binary that reads a declarative YAML
agent descriptor (`.agentsmithy.yaml`) and serves a fully functional
AI agent over a configurable transport. All agent behaviour (model,
kind, instruction, tools, pipeline shape) is declared in YAML; the
binary is the same regardless of which agent it serves.

## Design Principles

1. **Zero per-agent code.** All behaviour is declared in YAML.
2. **Minimal dependencies.** stdlib-first; provider SDKs and the
   MCP SDK are added deliberately as they become needed.
3. **Two surfaces, one implementation.** `pkg/cmd` is an embeddable
   CLI; `pkg/api` is the public Go API. Both reach the same
   `internal/*` code paths so the standalone binary and any host
   that embeds the CLI behave identically.
4. **Per-agent transport.** Transport is a property of the agent,
   not the host. The same agent definition is portable between
   standalone use and a multi-agent stack.
5. **Everything else internal.** Runtime, providers, schema
   processor, setup tools all live under `internal/` and are not
   importable from outside the module.

## Package Dependencies

Dependencies flow in one direction; leaf packages know nothing
about the packages that import them. `internal/config/schema`
depends on nothing internal; versioned schemas (`v1`, future `v2`)
call into it; the version-routing loader in `internal/config`
consumes them.

## Concurrency Model

- **stdio / mcp-stdio.** One goroutine reads requests; the dispatch
  loop handles them one at a time. A mutex guards writes against
  any future concurrent writers.
- **a2a / mcp-http.** Concurrent by Go's `net/http` design. Each
  incoming request is handled in its own goroutine; per-session
  state is the only shared mutable surface.

## Error Handling

- **Config errors.** Fatal at startup; the agent will not serve
  with a broken config.
- **Tool execution errors.** Returned as a tool error to the model,
  not a transport-level error, so the agent can observe the failure
  and recover within its loop.
- **Protocol errors.** On MCP transports, JSON-RPC errors with
  standard codes (`-32700`, `-32601`, `-32602`). On `a2a`, HTTP
  status codes plus a JSON error body.
