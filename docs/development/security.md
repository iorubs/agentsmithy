# Security Model

## Threat Model

agentsmithy serves an LLM-driven agent that can call tools and
delegate to other agents on behalf of the user (or service) that
launched it. Unlike a read-only context server, the agent's outputs
feed back into its own next action; a compromised conversation can
in principle escalate into tool calls. The primary risks are:

1. **Credential leakage.** Provider API keys end up in logs, error
   messages, or accessible to the agent itself.
2. **Untrusted tool surface.** A misconfigured agent gains access
   to MCP or A2A endpoints beyond the operator's intent.
3. **Prompt injection escalation.** Untrusted input in the
   conversation steers the model into invoking tools the operator
   would not have authorised.
4. **Network exposure.** `a2a` and `mcp-http` transports listen on
   a network port; without authentication in front, anyone reachable
   on that port can drive the agent.

The `.agentsmithy.yaml` file is **trusted**. Anyone who can modify
it controls which models, tools, and transports the agent uses.
Reviewing the config is the operator's responsibility.

## Mitigations

### Credentials never touch disk or stdout

- Provider credentials are resolved from the environment at process
  start. agentsmithy never persists them to disk and never echoes
  them to logs or error messages.
- All logs go to stderr via `slog`. Tool-call params at
  `--log-level=debug` may contain user input; production agents
  should run at `info`.

### Tool surface

Tool access is gated by what the config declares:

- The `tools:` catalog enumerates every MCP and A2A endpoint the
  agent may reach. The set is closed at startup.
- Each pipeline agent's `tools: [name, ...]` further narrows that
  catalog to the names this specific agent is allowed to call.
- Validation runs through the same `schema.Process` pass that
  validates everything else; there is no separate runtime
  permission system to drift out of sync.

### Skills

Skills (`shell`, `file`, `web`) extend the tool surface with
side-effecting capabilities. Each kind ships its own boundary so a
declared skill cannot reach beyond what the config allows:

- **Shell.** Each entry is a fixed `command` array; the runtime
  spawns it with `exec.Command` (no `sh -c`), in the entry's
  `workingDir`, with an empty environment (`cmd.Env = []string{}`).
  Validation rejects shell-interpreter `command[0]` (`sh`, `bash`,
  `zsh`, etc.), spliced templates inside arguments, and reserved
  arg names. Templates may render whole `command[]` elements but
  never partial strings, so user input cannot be concatenated into
  a command line.
- **File.** All reads and writes go through `pipeline/skills/sandbox`,
  which resolves `workingDir` to an absolute path (following
  symlinks) and rejects any access whose resolved path doesn't sit
  under it. Per-op `paths:` allowlists narrow access further.
  Symlink traversal and `..` segments are rejected.
- **Web.** `web` only accepts URLs whose
  `scheme://host` prefix appears in the per-skill allowlist.
  Response bodies are capped at 10 MB. Redirects are not followed
  (the HTTP client returns an error on any 3xx) so the allowlist
  cannot be bypassed by a redirect to an off-list host.

### Stderr-only logging

All logs go to stderr. Stdout is reserved for stdio transport
traffic. This prevents log injection into protocol messages.

## What Is NOT Mitigated

- **Config trust.** A `.agentsmithy.yaml` declaring an agent with
  broad tool access and an unauthenticated public-network transport
  will be served as written.
- **Prompt injection.** The agent receives untrusted input by
  design. The schema-declared tool surface is the only enforced
  boundary between a compromised conversation and a destructive
  tool call.
- **Model trust.** The chosen provider sees the conversation. If
  that provider is hostile or compromised, agentsmithy cannot
  protect against it.
- **Transport authentication.** The `a2a` and `mcp-http` transports
  have no built-in auth in v0.1. Bind to loopback unless an
  authenticating proxy stands in front.
