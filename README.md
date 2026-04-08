# AgentSmithy

> Build AI Agents on the fly from yaml`.agentsmithy.yaml`.
> No custom code required.

![agentsmithy forge](docs/images/forge.png)

**AgentSmithy** is a single Go binary that reads a declarative YAML
config file and serves a fully functional Agent server. It works for
any software project — no language or framework assumptions baked in.

## Quick Start

```bash
# Build
docker build -t agentsmithy:latest .

# Or from source (Go 1.26+)
go build -o bin/agentsmithy ./cmd/agentsmithy
```

See the [Install guide](docs/user/getting-started/install.md) for
editor integration and detailed setup.

## Documentation

### For Users

| | |
|---|---|
| [Docs site](https://iorubs.github.io/agentsmithy/) | Documentation overview |

### For Contributors

| | |
|---|---|
| [.agentsmithy.yaml](.agentsmithy.yaml) | Project sources, conventions, tools, and commands for AI assistants |
| [Development Guide](docs/development/README.md) | Architecture, CLI, config schema, testing, and security |
