package cmd

import (
	"context"
	"log/slog"

	"github.com/iorubs/agentsmithy/internal/setup"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SetupCmd starts an MCP server for config-authoring sessions.
// It does not require an existing .agentsmithy.yaml.
type SetupCmd struct{}

// Run starts the setup MCP server on stdio.
func (s *SetupCmd) Run(ctx context.Context) error {
	slog.InfoContext(ctx, "setup server running on stdio; connect your agent to write .agentsmithy.yaml")
	slog.InfoContext(ctx, "when done: agentsmithy validate; then: agentsmithy serve")
	srv := setup.BuildServer()
	return srv.Run(ctx, &mcp.StdioTransport{})
}
