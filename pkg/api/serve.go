package api

import (
	"context"
	"fmt"

	"github.com/iorubs/agentsmithy/internal/config"
	"github.com/iorubs/agentsmithy/internal/pipeline"
	"github.com/iorubs/agentsmithy/internal/server"
)

// ServeOptions controls server behaviour.
//
// Transport selects the wire protocol:
//   - stdio: line REPL over stdin/stdout for dev/CI use.
//   - mcp-stdio: agent exposed as an MCP tool over stdin/stdout (the
//     transport VS Code, Claude Desktop, etc. spawn). Required for
//     `provider: borrowed`.
//   - mcp-http: same MCP surface over streamable HTTP.
//   - a2a: A2A JSON-RPC server over HTTP, with AgentCard at
//     /.well-known/agent.json.
type ServeOptions struct {
	// Root is the directory the server is rooted in.
	Root string
	// Transport is the wire protocol; one of stdio|mcp-stdio|a2a|mcp-http.
	Transport string
	// Addr is the listening address for HTTP-style transports.
	Addr string
	// Once, when non-empty and Transport is stdio, sends a single prompt and exits.
	Once string
	// Verbose enables tool-call/intermediate-step tracing on stdio.
	Verbose bool
}

// Serve builds the pipeline from cfg and runs it on the requested
// transport. It blocks until ctx is cancelled or the transport
// terminates. Used both by `agentsmithy serve` and by smithy-cli when
// it embeds the binary in-process.
func Serve(ctx context.Context, cfg *config.Config, opts ServeOptions) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	p, err := pipeline.Build(cfg)
	if err != nil {
		return fmt.Errorf("build pipeline: %w", err)
	}

	transport := opts.Transport
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "stdio":
		return server.Stdio(ctx, p, opts.Once, opts.Verbose)
	case "mcp-stdio":
		requireSampling := len(cfg.Project.Models.Borrowed) > 0
		return server.MCPStdio(ctx, p, requireSampling)
	case "mcp-http":
		requireSampling := len(cfg.Project.Models.Borrowed) > 0
		return server.MCPHTTP(ctx, p, opts.Addr, requireSampling)
	case "a2a":
		return server.A2A(ctx, p, opts.Addr)
	default:
		return fmt.Errorf("unknown transport %q", transport)
	}
}
