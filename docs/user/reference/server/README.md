# Server

## agentsmithy

Project-agnostic AI Agent server. Reads .agentsmithy.yaml and serves an AI Agent.

```
agentsmithy <command> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-h, --help` | `bool` | — | Show context-sensitive help. |
| `-l, --log-level` | `enum(debug,info,warn,error)` | `info` | Log level (one of: debug,info,warn,error). |


### serve

Start the agent server.

```
agentsmithy serve [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-c, --config` | `string` | `.agentsmithy.yaml` | Path to config. |
| `--transport` | `enum(a2a,stdio,mcp-stdio,mcp-http)` | `a2a` | Transport to use. |
| `--addr` | `string` | `:8080` | Listen address (HTTP-like transports). |
| `--watch` | `bool` | `false` | Watch config file and hot-reload on change. |
| `-o, --once` | `string` | — | (stdio only) Send a single prompt, print the reply, then exit. |
| `-v, --verbose` | `bool` | — | (stdio only) Print tool calls and intermediate steps. |


### validate

Validate config file.

```
agentsmithy validate [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-c, --config` | `string` | `.agentsmithy.yaml` | Path to config. |


### setup

Start the config-authoring MCP assistant.

```
agentsmithy setup [flags]
```
