# Development Guide

## Overview

agentsmithy is a config-driven AI agent runtime written in Go. All
agent behaviour (model selection, kind, system prompt, tools,
guards, transport) is declared in YAML. The binary contains no
project-specific logic. See the project [README](../../README.md) for
user-facing documentation.

## Development Docs

| Document | Scope |
|----------|-------|
| [Architecture](architecture.md) | Package layout, dependency graph, data flow, agent kinds, error handling |
| [Config](config.md) | `.agentsmithy.yaml` format reference, struct tag conventions, versioning, how to extend |
| [CLI Design](cli.md) | Kong-based CLI, subcommands, embedding contract |
| [Security](security.md) | Credential handling, transport surface, tool/skill sandboxing |
| [Testing](testing.md) | Testing conventions and guidelines |
| [Roadmap](../user/reference/roadmap.md) | Tracked enhancements and parked features |

## Dependencies

The project follows a **stdlib-first** approach. Each addition should
be justified by significant value over a stdlib solution, and vetted
for maintenance status, transitive dependencies, and API stability.

- `github.com/alecthomas/kong`. CLI parsing. Provides declarative struct-tag-based argument definitions with no runtime dependencies.
  Same choice and same rationale as the rest of the smithy stack; revisit once the CLI surface stabilises.
- `go.yaml.in/yaml/v4`. YAML config parsing. Maintained by the YAML spec maintainers with zero transitive dependencies.
- `github.com/modelcontextprotocol/go-sdk`. MCP client + server primitives. Maintained by the MCP project (a Linux Foundation series of projects).
  Required for the `mcp-stdio` transport and for the `borrowed` model provider's `sampling/createMessage` round-trip.
- `google.golang.org/adk`. Agent engine: turns a configured `LLM`, tools, and an instruction into a session-driven loop with streaming events, tool dispatch, and sub-agent transfer.
  We adopt ADK's `model.LLM` interface as the provider contract (`internal/project/models`) so providers slot directly into the runner.
  ADK is large; we depend on it because re-implementing the run loop, callback shape, tool dispatch, and event stream would duplicate a substantial body of work.
  **Revisit** once the runtime API stabilises: if our usage narrows to a small subset, a hand-rolled loop over the provider interface may be cheaper than carrying ADK's transitive surface (OpenTelemetry, gRPC, Google cloud auth helpers).
- `google.golang.org/genai`. Content / part / function-call types used by ADK's `LLMRequest` and `LLMResponse`.
  Pulled in transitively by ADK; we reference it directly in provider implementations to translate to and from each provider's wire format.
- github.com/aws/aws-sdk-go-v2/config. AWS SDK configuration. OfficialAWS-maintained library for loading credentials and regional defaults.Required to authenticate and route requests to AWS services.github.com/aws/aws-sdk-go-v2/service/bedrockruntime.
- Bedrock runtimeclient. The official SDK for interacting with the Bedrock modelinference API. Used within the provider implementation to mapmodel.LLM requests to AWS infrastructure.

Runtime dependencies (model SDKs, MCP SDK, agent engine) are added
per phase as the runtime lands, behind the same vetting bar.

## Naming Conventions

Follow standard Go naming conventions throughout the codebase:

- **Packages.** Short, lowercase, single-word names. The package
  name should describe what it provides, not what it contains.
- **Exported identifiers.** Use `MixedCaps`. Exports should form the
  public API of the package; keep it small and intentional. Anything
  under `pkg/` is treated as a stable public surface; see
  [Architecture](architecture.md) for what that means in practice.
- **Unexported identifiers.** Use `mixedCaps`. Prefer short names
  for local variables with small scopes.
- **Interfaces.** Name after the behaviour they describe, not the
  implementation. Single-method interfaces use the method name plus
  `-er` suffix when it reads naturally.
- **Files.** Use `snake_case.go`. Test files use `_test.go` suffix.
  Keep one primary type or concept per file.
- **Acronyms.** Use consistent casing: `ID`, `URL`, `HTTP`, `LLM`,
  `MCP`, not `Id`, `Url`, `Http`, `Llm`, `Mcp`.

## Comment Conventions

Only add comments when they provide meaningful value beyond the code
itself. Specifically:

- **Package comments.** Every package should have a `// Package ...`
  comment on the primary file, describing the package's purpose.
- **Exported symbols.** Document all exported types, functions, and
  methods with a `// Name ...` comment that explains what and why,
  not how. Anything in `pkg/api` is a public contract; its godoc
  is what downstream consumers (including smithy-cli) read first.
- **Non-obvious logic.** Comment complex algorithms, non-trivial
  design decisions, or workarounds. If the next reader would need
  to stop and reason about the code, a comment helps.
- **Skip the obvious.** Do not comment trivial getters, setters,
  or straightforward control flow. The code is the documentation.

## Logs and Output

- **stdout is reserved for protocol traffic** when running an
  `mcp-stdio` transport. Never write application output there.
- **stderr** receives all log output via the `log/slog` package.
- Use the context-aware `slog` functions (`slog.InfoContext`,
  `slog.WarnContext`, etc.) for all log calls. Do not pass
  `*slog.Logger` as a function parameter or store it in structs.
  Instead, call `slog.SetDefault` once at startup and use the
  package-level `slog.*Context(ctx, ...)` functions everywhere.
  This keeps function signatures clean and lets handlers extract
  enrichment from `ctx` (request IDs, session IDs, trace spans).

### Log Levels

The CLI flag `--log-level` (`-l`) sets the minimum level. Default is
`info`. The principle: **info is what an operator sees in normal
production; debug is what a developer turns on to diagnose a
specific problem.**

| Level | Use for | Examples |
|-------|---------|----------|
| **Error** | Failures that stop or degrade the current operation. The agent or command cannot continue normally. | Config load failure, runtime build failure, provider auth failure at startup, unrecoverable transport error. |
| **Warn** | Unexpected conditions that are recoverable. The operation continues but the result may be incomplete. | Guard retry, single-step failure inside a loop kind, malformed JSON-RPC line, protocol version mismatch, session evicted under pressure. |
| **Info** | Significant lifecycle events and operations involving network or I/O that an operator would want to see. One line per event, not per item. | Agent serving on transport X, session opened/closed, hot-reload swap, model call dispatched (one line, not token-level). |
| **Debug** | Per-item detail, internal decisions, protocol wire traffic, token streaming. Useful for diagnosing but too noisy for normal operation. | Per-token streaming, tool call params and results, JSON-RPC recv/send, guard evaluation, template render input. |

### Level Decision Rules

1. **Network or I/O action starting** → Info (the operator should know
   something external is happening).
2. **Per-token / per-chunk streaming output** → Debug (one info line
   per call is enough; the stream itself is the user-visible output).
3. **Skipping something / cache hit** → Debug (nothing happened, only
   interesting when diagnosing).
4. **Per-item progress within a batch** → Debug (the summary at the
   end covers info).
5. **Summary or total at end of a phase** → Info (one line, not N).
6. **Protocol-level wire messages** → Debug.
7. **Something failed but we continue** → Warn.
8. **Something failed and we stop** → Error.

## Error Messages

Error messages follow a consistent style:

- **Start with lowercase.** Go convention.
- **Lead with the operation, then the cause.** Use a colon separator
  when wrapping (`load config: open file: ...`).
- **Be specific about what went wrong**, not prescriptive about how
  to fix it.
- **Include context for wrapped errors.** Identify the agent, step,
  tool, or session involved when it is not obvious from the chain.
- **Wrap only when it adds value.** If the error already identifies
  the operation and location clearly, return it as-is. Redundant
  wrapping (`"reading file: %w"` when the underlying error already
  says which file) adds noise without aiding diagnosis.
