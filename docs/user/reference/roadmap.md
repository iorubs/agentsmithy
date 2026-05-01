---
sidebar_position: 1
---

# Roadmap

Tracked feature ideas, technical debt, and enhancements for
AgentSmithy.

## Improvements

### Plugin / module system

**Problem:** Skills are limited to the built-in set (`file`, `git`, `github`, `sandbox`, `search`, `web`, `loop`), providers are limited to the shipped enum, and tool kinds are a fixed list (`mcp`, `a2a`). There is no way to add an external skill, a new model provider, or a custom tool kind without modifying core code. These extension points share the same shape; config-driven, registered at startup, invoked through a uniform interface; but there is no shared registration mechanism that would allow logic to live outside the main binary.

**Value:** The core stays short and security-auditable; config parsing, validation, agent-kind compilation, transport, session management, MCP protocol. Specialized logic like sandboxed shell execution, GitHub API integration, or a niche model provider could migrate to officially maintained plugins. Community authors could add skills and providers without forking the project.

**Why parked:** The current built-in skills, providers, and tool kinds cover all concrete use cases today. A plugin system is an architecture decision with long-lived API commitments; designing the contract, versioning story, and security boundary before there is a forcing use case risks over-engineering.
