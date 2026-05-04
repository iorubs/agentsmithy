package obs

import (
	"fmt"
	"io"
	"strings"

	"google.golang.org/adk/session"
)

// EventText concatenates every text part on ev's LLMResponse. Returns
// "" when ev or its content is nil.
func EventText(ev *session.Event) string {
	if ev == nil || ev.LLMResponse.Content == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range ev.LLMResponse.Content.Parts {
		if p != nil && p.Text != "" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// EventIsFinal reports whether ev is a final assistant response: it
// has content parts and none of them are tool calls or tool responses.
func EventIsFinal(ev *session.Event) bool {
	if ev == nil || ev.LLMResponse.Content == nil {
		return false
	}
	for _, p := range ev.LLMResponse.Content.Parts {
		if p == nil {
			continue
		}
		if p.FunctionCall != nil || p.FunctionResponse != nil {
			return false
		}
	}
	return true
}

// WriteTrace prints a human-readable per-event trace to w: tool calls,
// tool responses, and the assistant text. Used by the verbose REPL
// modes; structured logging happens via [Callbacks].
func WriteTrace(w io.Writer, ev *session.Event, text string) {
	if ev == nil || ev.LLMResponse.Content == nil {
		return
	}
	for _, p := range ev.LLMResponse.Content.Parts {
		if p == nil {
			continue
		}
		switch {
		case p.FunctionCall != nil:
			fmt.Fprintf(w, "  [tool-call] %s(%v)\n", p.FunctionCall.Name, p.FunctionCall.Args)
		case p.FunctionResponse != nil:
			fmt.Fprintf(w, "  [tool-resp] %s -> %v\n", p.FunctionResponse.Name, p.FunctionResponse.Response)
		}
	}
	if text != "" {
		fmt.Fprintf(w, "  [text] %s\n", text)
	}
}
