---
sidebar_position: 1
---

# Install

## Binary

### Download from GitHub Releases

Download `agentsmithy-<os>-<arch>` from the
[latest release](https://github.com/iorubs/agentsmithy/releases/latest).

### go install

```sh
go install github.com/iorubs/agentsmithy/cmd/agentsmithy@latest
```

### Connect your agent

Optionally, move the binary to a directory in your `PATH`.

**VS Code**; add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "agentsmithy": {
      "command": "agentsmithy",
      "args": ["serve", "--transport", "mcp-stdio"]
    }
  }
}
```

<!-- TODO: add binary connection examples for Claude Desktop, Cursor, GitHub Copilot CLI -->

## Docker

### Connect your agent

**VS Code**; add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "agentsmithy": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "${workspaceFolder}:/project:ro",
        "-w", "/project",
        "smithylabs/agentsmithy:latest",
        "serve", "--transport", "mcp-stdio"
      ]
    }
  }
}
```

<!-- TODO: add Docker connection examples for Claude Desktop, Cursor, GitHub Copilot CLI -->

## Next steps

Next you'll need a `.agentsmithy.yaml` config. See the
[Use Cases](./use-cases/simple-chat.md) section to find a scenario
that fits your needs, then follow the tip at the bottom to generate
your config with your agent.
