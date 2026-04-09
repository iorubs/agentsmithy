# Server

## agentsmithy

Project-agnostic AI Agent server. Reads .agentsmithy.yaml and serves an AI Agent.

```
agentsmithy <command> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-h, --help` | `bool` | — | Show context-sensitive help. |
| `-c, --config` | `string` | `.agentsmithy.yaml` | Path to config file. |
| `-l, --log-level` | `enum(debug,info,warn,error)` | `info` | Log level (one of: debug,info,warn,error). |


### serve

Start the Agent server.

```
mcpsmithy serve [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--transport` | `enum(stdio,http)` | `stdio` | Transport to use. |
| `--addr` | `string` | `:8080` | Listen address (HTTP transport only). |
| `--watch` | `bool` | `false` | Watch config file and hot-reload on change. |


### validate

Validate config file.

```
mcpsmithy validate [flags]
```
