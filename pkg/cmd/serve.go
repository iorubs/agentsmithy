package cmd

import (
	"context"
	"errors"
	"log/slog"
)

// ServeCmd starts the agent server.
//
// Phase 1: stub. The runtime is wired in Phase 4. Transport/Addr/Watch
// flags are defined now so the embed surface is stable.
type ServeCmd struct {
	ConfigFlag
	Transport string `help:"Transport to use." enum:"a2a,stdio,mcp-stdio,mcp-http" default:"a2a"`
	Addr      string `help:"Listen address (HTTP-like transports)." default:":8080"`
	Watch     bool   `help:"Watch config file and hot-reload on change." default:"false"`
}

// Run executes the serve command.
func (cmd *ServeCmd) Run(ctx context.Context) error {
	slog.InfoContext(ctx, "serve: not implemented (Phase 4)", "config", cmd.Config, "transport", cmd.Transport)
	return errors.New("serve: not implemented")
}
