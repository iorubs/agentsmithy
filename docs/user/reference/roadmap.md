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

---

### Session lifecycle management

**Problem:** Each agent process keeps an in-memory map of conversation sessions keyed by contextID. Every new context adds an entry that lives until the process restarts; there is no eviction policy, no upper bound on the map, and no idle timeout. Fine for local development against a single user, but a long-running deployment serving many users will accumulate sessions until memory pressure forces a restart. The deferred a2a / mcp-http transport tests are also gated on this; meaningful tests need to exercise multi-session behaviour with eviction semantics.

**Value:** A bounded LRU cap and idle-TTL eviction would make agent processes safe to run as long-lived shared services. Operators get predictable memory ceilings; idle conversations get reclaimed without manual intervention; and the transport test suite can finally exercise concurrent sessions with realistic lifetimes. The same module would centralise the place where future persistence backends (durable session stores, cross-process resumption) plug in.

**Why parked:** Today's deployments are single-user local processes (smithy-cli, individual developers); the ADK in-memory store handles that workload without the leak ever mattering in practice. The eviction policy needs at least one real shared deployment to size correctly (cap and TTL defaults), so building it speculatively risks tuning against hypothetical workloads. Revisit when a concrete deployment runs the agent as a multi-user shared service or when the deferred transport test work is scheduled.

---

### Native provider implementations

**Problem:** Several providers ship as registered stubs that return "not implemented yet" when called. The registry, config schema, and pipeline wiring are all in place, but the actual API clients are not:

- **Anthropic** native Claude API.
- **Google** native (Gemini direct API via `google.golang.org/genai`).
- **AWS Bedrock** (Partial) (covers Claude, Llama, Titan, Mistral on AWS via SDK v2 + SigV4).
- **Vertex AI** (Gemini and Anthropic on Vertex via `genai` in Vertex mode + GCP service-account auth).

**Value:** Users get first-class access to each vendor without routing through OpenAI-compatible shims. Bedrock and Vertex specifically unlock managed deployments where the model lives behind cloud auth (IAM, service accounts) rather than API keys, which is a hard requirement for enterprise stacks. Each one drops into the existing registry without touching `models.go`, `build.go`, or call sites.

**Why parked:** OpenAI-compatible providers (including Ollama, vLLM, LM Studio, Together, Groq) cover every concrete model the project has needed so far. The borrowed-via-MCP path also covers the "use the host's model" case for Copilot-style integrations. Building four full provider clients before there's a forcing use case ties up effort that pays off only when a deployment specifically requires native Anthropic / Bedrock / Vertex auth. Revisit when a concrete deployment requires one of them; the stub-to-real transition is intentionally low-risk because the surrounding wiring already works.

---

### Chat history auth

**Problem:** The `GET /sessions/{id}/messages` REST shim in the a2a server is a workaround for a2a-go's `tasks/list` requiring an authenticator hook; the underlying task store returns `ErrUnauthenticated` when its authenticator rejects the context. The shim bypasses that path. Fine on loopback where the only caller is the local TUI, but if the agent is exposed beyond `127.0.0.1`, anyone who can guess a contextID can read the conversation transcript. The main `/` JSON-RPC dispatch path has the same exposure shape.

**Value:** A real authentication boundary on the agent's HTTP surface. The intended direction is to terminate auth upstream (reverse proxy or sidecar injects `X-Forwarded-User` / a verified identity header), plug a real authenticator into the a2a-go store so `tasks/list` works through the standard path, then drop the REST shim entirely. The same middleware would gate `/` on the JSON-RPC side. This is the prerequisite for running an agentsmithy deployment on a non-loopback bind address without leaking transcripts.

**Why parked:** Today's deployments all run on loopback (smithy-cli daemon, individual developers); the shim is harmless in that environment and the a2a-go store's authenticator hook is generic enough that committing to a specific identity contract before there's a real deployment risks designing for the wrong model. Revisit when a deployment needs to expose the agent beyond `127.0.0.1`; the bind-address default should flip to loopback at the same time so opting into remote exposure becomes the explicit, deliberate step.
