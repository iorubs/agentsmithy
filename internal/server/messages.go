package server

import (
	"strings"
	"time"

	"google.golang.org/adk/session"
)

// SessionMessage is one visible chat turn returned by
// GET /sessions/{id}/messages. Tool calls and function responses are
// dropped; only user input and assistant text replies are kept.
type SessionMessage struct {
	Role      string    `json:"role"` // "user" | "assistant"
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

func extractMessages(events session.Events) []SessionMessage {
	if events == nil {
		return nil
	}
	out := make([]SessionMessage, 0, events.Len())
	for e := range events.All() {
		if e == nil || e.LLMResponse.Content == nil {
			continue
		}
		var sb strings.Builder
		for _, p := range e.LLMResponse.Content.Parts {
			if p == nil || p.FunctionCall != nil || p.FunctionResponse != nil {
				continue
			}
			if p.Text != "" {
				sb.WriteString(p.Text)
			}
		}
		text := strings.TrimSpace(sb.String())
		if text == "" {
			continue
		}
		switch e.LLMResponse.Content.Role {
		case "user":
			out = append(out, SessionMessage{Role: "user", Text: text, Timestamp: e.Timestamp})
		case "model", "assistant", "":
			out = append(out, SessionMessage{Role: "assistant", Text: text, Timestamp: e.Timestamp})
		}
	}
	return out
}
