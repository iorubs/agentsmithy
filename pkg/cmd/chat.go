package cmd

import (
	"context"
	"errors"
	"log/slog"
)

// ChatCmd is the standalone agentsmithy chat REPL.
//
// This is intentionally minimal: a plain stdio line-reader/writer for
// quickly validating an agent config during development. Rich UX (TUI,
// streaming render, history, multi-agent switching) lives in smithy-cli's
// own AgentChatCmd, which shadows this command in the embed.
//
// Phase 1: stub. Wired to the in-process fake backend in Phase 3 and to the real engine in Phase 4.
type ChatCmd struct {
	ConfigFlag
	Once    string `help:"Single-shot input; print response and exit." short:"o"`
	Verbose bool   `help:"Print tool calls and intermediate steps." short:"v"`
}

// Run executes the chat command.
func (cmd *ChatCmd) Run(ctx context.Context) error {
	slog.InfoContext(ctx, "chat: not implemented (Phase 3/4)", "config", cmd.Config)
	return errors.New("chat: not implemented")
}
