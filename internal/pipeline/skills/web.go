package skills

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/skills/htmltext"
	"github.com/iorubs/agentsmithy/internal/pipeline/skills/urlallow"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// webMaxBodyBytes caps response size for web_scrape. The HTML→text
// pass typically shrinks pages substantially, but we still need a
// hard ceiling to keep one bad URL from wedging the LLM context.
const webMaxBodyBytes = 10 * 1024 * 1024

// scrapeResult is the wire shape returned by web_scrape. Body is
// readable text/markdown when the response was HTML; otherwise the
// raw text passes through.
type scrapeResult struct {
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Body        string `json:"body"`
}

func buildWeb(ws v1.WebSkill) ([]adktool.Tool, map[string]Helper) {
	if !ws.Enabled {
		return nil, nil
	}
	allowed := urlallow.Parse(ws.URLs)

	t, _ := functiontool.New(functiontool.Config{
		Name: "web_scrape",
		Description: "Fetch a single allowlisted URL and return its content. " +
			"HTML responses are converted to readable text/markdown; " +
			"plain text and markdown responses pass through. " +
			"Redirects are not followed; sub-pages are not crawled.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"url": {Type: "string", Description: "Full URL; scheme+host must match the allowlist."},
			},
			Required: []string{"url"},
		},
	}, func(tc adktool.Context, in map[string]any) (scrapeResult, error) {
		u, _ := in["url"].(string)
		return doScrape(toolCtx(tc), allowed, u)
	})

	helpers := map[string]Helper{
		"web_scrape": func(ctx context.Context, args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("web_scrape: expected 1 arg (url), got %d", len(args))
			}
			u, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("web_scrape: url must be a string")
			}
			return doScrape(ctx, allowed, u)
		},
	}
	return []adktool.Tool{t}, helpers
}

func doScrape(ctx context.Context, allowed map[string]bool, url string) (scrapeResult, error) {
	if err := urlallow.Check(url, allowed); err != nil {
		return scrapeResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return scrapeResult{}, err
	}
	req.Header.Set("Accept", "text/html, text/plain, text/markdown")
	req.Header.Set("User-Agent", "agentsmithy/1.0")

	resp, err := urlallow.NoRedirectClient.Do(req)
	if err != nil {
		return scrapeResult{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, webMaxBodyBytes+1))
	if err != nil {
		return scrapeResult{}, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > webMaxBodyBytes {
		return scrapeResult{}, fmt.Errorf("response body exceeds %d bytes", webMaxBodyBytes)
	}

	ct := resp.Header.Get("Content-Type")
	out := string(body)
	if isHTML(ct) {
		out = htmltext.Convert(out)
	} else {
		out = strings.TrimSpace(out)
	}
	return scrapeResult{Status: resp.StatusCode, ContentType: ct, Body: out}, nil
}

// isHTML mirrors the heuristic used by mcpsmithy's scrape source:
// an explicit text/html wins, plain text and markdown opt out, and
// an unknown/missing Content-Type defaults to HTML treatment.
func isHTML(contentType string) bool {
	if strings.Contains(contentType, "text/html") {
		return true
	}
	if strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/markdown") {
		return false
	}
	return contentType == ""
}
