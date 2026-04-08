# CLI Design

## Overview

agentsmithy uses [Kong](https://github.com/alecthomas/kong) for CLI
parsing. Commands are declared as Go structs with Kong struct tags —
no imperative flag registration. The struct layout is the source of
truth; `gen-docs` reads it to produce the user-facing reference docs
in [`docs/user/reference/cli/`](../user/reference/cli/README.md).

Do not document flags or command behaviour here. That lives in the
generated reference docs and must stay in sync with the code
automatically.

## Package Layout

```
cmd/agentsmithy/main.go  → Entry point: kong.Parse() only, no logic
pkg/cmd/                 → One file per subcommand; root in cmd.go
```

The `Commands` struct is the unit of embedding. Standalone, the
root binary's `CLI` struct embeds `Commands` directly. A host
(smithy-cli) embeds `Commands` into its own command struct and adds
host-local subcommands beside it. Field-shadowing on the host wins
over the embedded definition.

## Conventions

- **stdout is reserved for protocol traffic.** Subcommands that
  serve a stdio transport (`serve`, `setup`) must never write
  application output to stdout. All output goes to stderr via the
  `slog` logger.
- **No startup banners or plain `fmt` writes.** Use structured
  `slog` calls so output is consistent and filterable.
- **`ConfigFlag` is the standard mixin** for any subcommand that
  loads `.agentsmithy.yaml`. It carries `-c / --config` so the same
  flag works under `agentsmithy <cmd>` and `smithy agent <cmd>`.
- **Kong struct tags define the CLI surface.** Help text, defaults,
  enums, and short flags are declared in the tag, not in `Run`.

## Adding a New Command

1. Create `pkg/cmd/<name>.go` with a struct exposing `Run() error`
   (add the `ConfigFlag` mixin if it loads config).
2. Add the struct as a field on `Commands` in `cmd.go` with `cmd:""`
   and `help:""` tags.
3. Run `go run ./cmd/gen-docs` to regenerate the reference docs.
