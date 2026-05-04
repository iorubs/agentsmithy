// Package cmd implements the CLI subcommands.
//
// Commands are designed to be embeddable: smithy-cli embeds [Commands]
// directly into its own root CLI so that `smithy agent <subcommand>`
// routes to the same code paths as `agentsmithy <subcommand>`.
package cmd

import (
	"log/slog"
)

// LogLevel represents a supported log verbosity level.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// CLI is the root Kong CLI struct for the standalone agentsmithy binary.
type CLI struct {
	LogLevel LogLevel `help:"Log level (one of: ${enum})." default:"info" enum:"debug,info,warn,error" short:"l"`
	Commands
}

// Commands holds the subcommands, safe to embed into a host CLI.
type Commands struct {
	Serve    ServeCmd    `cmd:"" help:"Start the agent server."`
	Validate ValidateCmd `cmd:"" help:"Validate config file."`
	Setup    SetupCmd    `cmd:"" help:"Start the config-authoring MCP assistant."`
}

// ConfigFlag is the standard config-path mixin for subcommands that load an .agentsmithy.yaml file.
type ConfigFlag struct {
	Config string `help:"Path to config." default:".agentsmithy.yaml" type:"path" short:"c"`
}

// ParseLogLevel maps the CLI log-level flag to slog.Level.
func ParseLogLevel(l LogLevel) slog.Level {
	switch l {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
