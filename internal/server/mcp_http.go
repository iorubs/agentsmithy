package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/iorubs/agentsmithy/internal/pipeline"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPHTTP exposes the pipeline's root agent as an MCP server over
// streamable HTTP. Same tool surface as MCPStdio; supports multiple
// concurrent clients and is addressable by URL. When the pipeline
// declares any borrowed model the connecting client must advertise
// the `sampling` capability.
func MCPHTTP(ctx context.Context, p *pipeline.Pipeline, addr string, requireSampling bool) error {
	srv, toolName, err := buildMCPServer(p, requireSampling)
	if err != nil {
		return err
	}

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		nil,
	)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           withCtxValues(ctx, handler),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.InfoContext(ctx, "MCP http server starting",
		"name", p.Name,
		"tool", toolName,
		"addr", addr,
		"requireSampling", requireSampling,
	)

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("mcp-http shutdown: %w", err)
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
