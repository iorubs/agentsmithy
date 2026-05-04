package cmd

import (
	"context"

	"github.com/iorubs/agentsmithy/pkg/api"
)

// ServeCmd starts the agent server.
//
// `--transport` selects the wire protocol:
//   - a2a (default): A2A JSON-RPC server over HTTP.
//   - stdio: line REPL over stdin/stdout for dev/CI use.
//   - mcp-stdio: expose the agent as an MCP tool over stdin/stdout.
//     This is the path VS Code, Claude Desktop, etc. spawn. Required
//     for `provider: borrowed`, which round-trips completions back to
//     the connecting client via sampling/createMessage.
//   - mcp-http: same MCP surface over streamable HTTP.
type ServeCmd struct {
	ConfigFlag
	Transport string `help:"Transport to use (one of: ${enum})." enum:"a2a,stdio,mcp-stdio,mcp-http" default:"a2a"`
	Addr      string `help:"Listen address (HTTP-like transports)." default:":8080"`
	Watch     bool   `help:"Watch config file and hot-reload on change." default:"false"`
	Once      string `help:"(stdio only) Send a single prompt, print the reply, then exit." short:"o"`
	Verbose   bool   `help:"(stdio only) Print tool calls and intermediate steps." short:"v"`
}

// Run executes the serve command.
func (cmd *ServeCmd) Run(ctx context.Context) error {
	cfg, root, err := api.LoadConfig(cmd.Config)
	if err != nil {
		return err
	}
	return api.Serve(ctx, cfg, api.ServeOptions{
		Root:      root,
		Transport: cmd.Transport,
		Addr:      cmd.Addr,
		Once:      cmd.Once,
		Verbose:   cmd.Verbose,
	})
}
