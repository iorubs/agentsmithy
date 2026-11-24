package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2acompat/a2av0"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/iorubs/agentsmithy/internal/pipeline"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/server/adka2a/v2"
	"google.golang.org/adk/session"
)

// legacyAgentCardPath is the v0.2/v0.3 well-known agent card location.
// v1.0 clients look at [a2asrv.WellKnownAgentCardPath]; both serve the
// same card, serialised as a union of the v0.3 and v1.0 formats.
const legacyAgentCardPath = "/.well-known/agent.json"

// v1Path is where the A2A v1.0 JSON-RPC binding is served. The v0.3
// binding stays at / so existing clients keep working unchanged.
const v1Path = "/a2a/v1"

// A2A serves the pipeline's root agent as an A2A JSON-RPC service
// over HTTP. Publishes an AgentCard at /.well-known/agent.json and
// /.well-known/agent-card.json, dispatches v0.3 JSON-RPC requests at /
// and v1.0 JSON-RPC requests at /a2a/v1. Sessions are kept in-memory.
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
		Name:    p.Name,
		Version: "1.0.0",
		SupportedInterfaces: []*a2a.AgentInterface{
			// First v0.3 entry becomes the legacy `url`/`preferredTransport`.
			{URL: cardURL, ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: a2av0.Version},
			{URL: cardURL + v1Path[1:], ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: a2a.Version},
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Capabilities:       a2a.AgentCapabilities{Streaming: true},
	}
	cardHandler := a2asrv.NewAgentCardHandler(a2av0.NewStaticAgentCardProducer(card))

	mux := http.NewServeMux()
	mux.Handle("GET "+legacyAgentCardPath, cardHandler)
	mux.Handle("GET "+a2asrv.WellKnownAgentCardPath, cardHandler)
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
	mux.Handle(v1Path, a2asrv.NewJSONRPCHandler(handler))
	mux.Handle("/", a2av0.NewJSONRPCHandler(handler))

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
