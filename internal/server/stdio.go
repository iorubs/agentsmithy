package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/iorubs/agentsmithy/internal/pipeline"
	"github.com/iorubs/agentsmithy/internal/pipeline/obs"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Stdio runs the pipeline interactively over stdin/stdout: a minimal
// line REPL for dev and CI. When once is non-empty, it sends that
// single prompt, prints the reply, and exits. Rich UX lives in
// smithy-cli's chat TUI; this is the bare scripting/dev entry point.
func Stdio(ctx context.Context, p *pipeline.Pipeline, once string, verbose bool) error {
	r, err := runner.New(runner.Config{
		AppName:           p.Name,
		Agent:             p.Root,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return fmt.Errorf("runner: %w", err)
	}

	sessionID := "stdio-" + p.Name
	userID := "stdio-user"

	if once != "" {
		reply, err := stdioSendOne(ctx, r, userID, sessionID, once, verbose)
		if err != nil {
			return err
		}
		fmt.Println(reply)
		return nil
	}

	return stdioLoop(ctx, r, p.Name, userID, sessionID, verbose)
}

// stdioLoop runs the interactive line REPL until EOF, /exit, or ctx cancellation.
func stdioLoop(ctx context.Context, r *runner.Runner, appName, userID, sessionID string, verbose bool) error {
	fmt.Fprintf(os.Stderr, "chatting with %q (type /exit or ^C to quit)\n", appName)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type readResult struct {
		line string
		err  error
		eof  bool
	}
	lines := make(chan readResult)
	go func() {
		for {
			if !scanner.Scan() {
				lines <- readResult{err: scanner.Err(), eof: true}
				return
			}
			lines <- readResult{line: scanner.Text()}
		}
	}()

	for {
		fmt.Fprint(os.Stderr, "you> ")
		var rr readResult
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr)
			return nil
		case rr = <-lines:
		}
		if rr.eof {
			fmt.Fprintln(os.Stderr)
			return rr.err
		}
		line := strings.TrimSpace(rr.line)
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			return nil
		}
		reply, err := stdioSendOne(ctx, r, userID, sessionID, line, verbose)
		if err != nil {
			if ctx.Err() != nil {
				fmt.Fprintln(os.Stderr)
				return nil
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		fmt.Fprint(os.Stderr, "agent> ")
		fmt.Println(reply)
	}
}

// stdioSendOne sends one user turn and accumulates the final-response text.
func stdioSendOne(ctx context.Context, r *runner.Runner, userID, sessionID, prompt string, verbose bool) (string, error) {
	msg := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: prompt}},
	}
	var final strings.Builder
	for ev, err := range r.Run(ctx, userID, sessionID, msg, agent.RunConfig{}) {
		if err != nil {
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
	return out, nil
}
