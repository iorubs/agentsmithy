package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"github.com/iorubs/agentsmithy/internal/pipeline"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/server/adka2a"
	"google.golang.org/adk/session"
)

// A2A serves the pipeline's root agent as an A2A JSON-RPC service
// over HTTP. Publishes an AgentCard at /.well-known/agent.json and
// dispatches JSON-RPC requests at /. Sessions are kept in-memory.
func A2A(ctx context.Context, p *pipeline.Pipeline, addr string) error {
	if addr == "" {
		addr = ":8080"
	}

	sessions := session.InMemoryService()
	executor := adka2a.NewExecutor(adka2a.ExecutorConfig{
		RunnerConfig: runner.Config{
			AppName:           p.Name,
			Agent:             p.Root,
			SessionService:    sessions,
			AutoCreateSession: true,
		},
	})
	handler := a2asrv.NewHandler(executor)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	actualAddr := listener.Addr().String()

	host, port, _ := net.SplitHostPort(actualAddr)
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}
	cardURL := fmt.Sprintf("http://%s:%s/", host, port)
	card := &a2a.AgentCard{
		Name:               p.Name,
		URL:                cardURL,
		Version:            "1.0.0",
		ProtocolVersion:    "0.2.0",
		PreferredTransport: a2a.TransportProtocolJSONRPC,
		AdditionalInterfaces: []a2a.AgentInterface{
			{Transport: a2a.TransportProtocolJSONRPC, URL: cardURL},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(card); err != nil {
			slog.WarnContext(r.Context(), "a2a: encode agent card", "error", err)
		}
	})
	// GET /sessions/{id}/messages: flatten ADK session events to a
	// simple chat transcript. Used by `smithy agent chat` to recover
	// history across CLI invocations. The a2a `tasks/list` RPC is
	// gated behind an authenticator we don't run, so we read the
	// session service directly.
	mux.HandleFunc("GET /sessions/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("id")
		if sid == "" {
			http.Error(w, "missing session id", http.StatusBadRequest)
			return
		}
		uid := "A2A_USER_" + sid
		resp, err := sessions.Get(r.Context(), &session.GetRequest{
			AppName: p.Name, UserID: uid, SessionID: sid,
		})
		w.Header().Set("Content-Type", "application/json")
		if err != nil || resp == nil || resp.Session == nil {
			if err := json.NewEncoder(w).Encode(map[string]any{"messages": []any{}}); err != nil {
				slog.WarnContext(r.Context(), "a2a: encode empty messages", "error", err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"messages": extractMessages(resp.Session.Events()),
		}); err != nil {
			slog.WarnContext(r.Context(), "a2a: encode messages", "error", err)
		}
	})
	mux.Handle("/", a2asrv.NewJSONRPCHandler(handler))

	srv := &http.Server{
		Handler:           withCtxValues(ctx, mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.InfoContext(ctx, "A2A server listening",
		"name", p.Name,
		"addr", actualAddr,
	)

	go func() { <-ctx.Done(); _ = srv.Close() }()
	return srv.Serve(listener)
}
