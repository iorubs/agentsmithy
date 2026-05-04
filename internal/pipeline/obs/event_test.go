package obs

import (
	"bytes"
	"strings"
	"testing"

	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestEventText(t *testing.T) {
	tests := []struct {
		name string
		ev   *session.Event
		want string
	}{
		{"nil event", nil, ""},
		{"empty event", &session.Event{}, ""},
		{"concatenates parts skipping nil", &session.Event{LLMResponse: adkmodel.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{
			{Text: "hello "},
			nil,
			{Text: "world"},
		}}}}, "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EventText(tt.ev); got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestEventIsFinal(t *testing.T) {
	tests := []struct {
		name string
		ev   *session.Event
		want bool
	}{
		{"nil event", nil, false},
		{"empty event", &session.Event{}, false},
		{"event with tool call", &session.Event{LLMResponse: adkmodel.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{
			{Text: "thinking"},
			{FunctionCall: &genai.FunctionCall{Name: "x"}},
		}}}}, false},
		{"text-only event", &session.Event{LLMResponse: adkmodel.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{
			{Text: "answer"},
		}}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EventIsFinal(tt.ev); got != tt.want {
				t.Fatalf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func TestWriteTrace(t *testing.T) {
	tests := []struct {
		name         string
		ev           *session.Event
		text         string
		wantEmpty    bool
		wantContains []string
	}{
		{"nil event writes nothing", nil, "", true, nil},
		{"tool calls and text", &session.Event{LLMResponse: adkmodel.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{
			{FunctionCall: &genai.FunctionCall{Name: "search", Args: map[string]any{"q": "x"}}},
			{FunctionResponse: &genai.FunctionResponse{Name: "search", Response: map[string]any{"hits": 1}}},
		}}}}, "done", false, []string{"[tool-call] search", "[tool-resp] search", "[text] done"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			WriteTrace(&buf, tt.ev, tt.text)
			if tt.wantEmpty {
				if buf.Len() != 0 {
					t.Fatalf("expected no output, got %q", buf.String())
				}
				return
			}
			out := buf.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(out, want) {
					t.Fatalf("missing %q in:\n%s", want, out)
				}
			}
		})
	}
}
