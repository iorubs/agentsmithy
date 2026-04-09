// Command agentsmithy is a config-driven MCP tool server.
// It reads .agentsmithy.yaml and serves fully ready AI Agents.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/iorubs/agentsmithy/internal/commands"
)

func main() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "--help")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var cli commands.CLI
	kctx := kong.Parse(&cli,
		kong.Name("agentsmithy"),
		kong.Description("Project-agnostic Agent server. Reads .agentsmithy.yaml and serves an AI Agent."),
		kong.UsageOnError(),
		kong.BindTo(ctx, (*context.Context)(nil)),
	)

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: commands.ParseLogLevel(cli.LogLevel),
	})))

	kctx.FatalIfErrorf(kctx.Run(&cli))
}
