# AgentSmithy

Build AI agents on the fly from a `.agentsmithy.yaml` config, no custom code required. AgentSmithy reads a declarative YAML config file and serves a fully functional agent server for any software project.

## Usage

Run it against your project, mounted read-only, as an A2A server over HTTP (the default transport):

```bash
docker run --rm \
  -v "$(pwd)":/project:ro \
  -w /project \
  -p 8080:8080 \
  smithylabs/agentsmithy:latest \
  serve
```

### MCP (VS Code, Claude Desktop, etc.)

Pass `--transport mcp-stdio` to expose the agent as an MCP tool over stdin/stdout instead. Add to `.vscode/mcp.json`:

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

## Tags

- `latest` — most recent stable release
- `X`, `X.Y`, `X.Y.Z` — semver-pinned releases

## Links

- [GitHub](https://github.com/iorubs/agentsmithy)
- [Docs](https://iorubs.github.io/agentsmithy/)
