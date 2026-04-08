---
sidebar_position: 3
---

# Guided Setup

`agentsmithy setup` starts an MCP server without requiring an existing
`.agentsmithy.yaml`. It exposes two tools designed specifically for
config-authoring sessions:

- **`config_guide`**; returns an overview of the config structure:
  the top-level sections, how they relate, and an annotated minimal
  example. The right first call for any setup session.
- **`config_section`**; returns a deep reference for one config
  section (`models`, `tools`, `pipeline`): all fields, types,
  defaults, valid values, and a realistic example.

Both tools are generated from the same schema that drives the
validator; they are always accurate for the installed binary version.

## Workflow

1. Run `agentsmithy setup` in your project root.
1. Connect your MCP-compatible agent (VS Code, Cursor, Claude, etc.).
1. Give the agent a prompt describing what you want to set up. See the
[Use Cases](./use-cases/simple-chat) section for tailored prompts
for each scenario.
1. The agent calls `config_guide` and `config_section` to understand the schema, reads your project with its own file tools, and writes `.agentsmithy.yaml`.
1. Validate: `agentsmithy validate`
1. Switch to serve: `agentsmithy serve`

## Notes

- The agent writes the file using its own file tools; agentsmithy stays read-only throughout.
- `config_guide` and `config_section` are only available in setup mode. They are not exposed by `agentsmithy serve`.
- If you already have a config and want to improve a specific section, skip `config_guide` and call `config_section` directly for the section you're working on.
