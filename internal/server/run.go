package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/iorubs/agentsmithy/internal/pipeline"
	"github.com/iorubs/agentsmithy/internal/pipeline/obs"
	"github.com/iorubs/agentsmithy/internal/pipeline/tmpl"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// RunOnce builds a runner, sends a single trigger to the pipeline,
// and returns the final output. Used by transport=none for
// fire-and-forget agents (bots, cron jobs, event handlers).
func RunOnce(ctx context.Context, p *pipeline.Pipeline, verbose bool) (string, error) {
	r, err := runner.New(runner.Config{
		AppName:           p.Name,
		Agent:             p.Root,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return "", fmt.Errorf("runner: %w", err)
	}

	sessionID := "run-" + p.Name
	userID := "run-user"

	msg := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: "Go ahead with your task."}},
	}

	var final strings.Builder
	for ev, err := range r.Run(ctx, userID, sessionID, msg, agent.RunConfig{}) {
		if err != nil {
			if sig, ok := errors.AsType[*tmpl.ExitSignal](err); ok {
				slog.ErrorContext(ctx, "exit_error", "message", sig.Message)
			} else {
				slog.ErrorContext(ctx, "pipeline error", "error", err)
			}
			return "", err
		}
		text := obs.EventText(ev)
		if verbose {
			obs.WriteTrace(os.Stderr, ev, text)
		}
		if text != "" && obs.EventIsFinal(ev) {
			final.WriteString(text)
		}
	}
	out := strings.TrimSpace(final.String())
	if out == "" {
		return "", errors.New("agent returned no final text")
	}
	slog.DebugContext(ctx, "run complete", "output", out)
	return out, nil
}
