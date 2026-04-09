package commands

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// ServeCmd starts the MCP server.
type ServeCmd struct {
	Transport string `help:"Transport to use." default:"stdio" enum:"stdio,http"`
	Addr      string `help:"Listen address (HTTP transport only)." default:":8080"`
	Watch     bool   `help:"Watch config file and hot-reload on change." default:"false"`
}

// Run executes the serve command.
func (cmd *ServeCmd) Run(ctx context.Context, cli *CLI) error {
	cfg, root, err := cli.LoadConfig()
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "TODO: implement engine server")

	if cmd.Watch {
		go watchConfig(ctx, cli)
	}

	slog.InfoContext(ctx, "ready", "project", cfg.Project.Name, "root", root)
	return nil
}

// watchConfig polls the config file for mtime changes and hot-reloads on change.
func watchConfig(ctx context.Context, cli *CLI) {
	const pollInterval = 2 * time.Second
	const debounceDelay = 500 * time.Millisecond

	info, err := os.Stat(cli.Config)
	if err != nil {
		slog.ErrorContext(ctx, "watch: cannot stat config", "err", err)
		return
	}
	lastMod := info.ModTime()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var debounce *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi, err := os.Stat(cli.Config)
			if err != nil || !fi.ModTime().After(lastMod) {
				continue
			}
			lastMod = fi.ModTime()
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(debounceDelay, func() {
				slog.InfoContext(ctx, "reload: TODO engine swapped")
			})
		}
	}
}
