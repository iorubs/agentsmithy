package obs

import (
	"testing"

	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestCallbacksReturnsAllFour(t *testing.T) {
	bm, am, bt, at := Callbacks("agent-x")
	if bm == nil || am == nil || bt == nil || at == nil {
		t.Fatalf("expected all four callbacks, got nil")
	}
	var (
		_ llmagent.BeforeModelCallback = bm
		_ llmagent.AfterModelCallback  = am
		_ llmagent.BeforeToolCallback  = bt
		_ llmagent.AfterToolCallback   = at
	)
}

func TestArgKeysSorted(t *testing.T) {
	got := argKeys(map[string]any{"b": 1, "a": 2, "c": 3})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d got %q want %q", i, got[i], want[i])
		}
	}
}

func TestCountToolCalls(t *testing.T) {
	tests := []struct {
		name string
		resp *adkmodel.LLMResponse
		want int
	}{
		{"nil response", nil, 0},
		{"empty response", &adkmodel.LLMResponse{}, 0},
		{"two function calls", &adkmodel.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{
			{Text: "hi"},
			{FunctionCall: &genai.FunctionCall{Name: "search"}},
			{FunctionCall: &genai.FunctionCall{Name: "fetch"}},
		}}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countToolCalls(tt.resp); got != tt.want {
				t.Fatalf("got %d; want %d", got, tt.want)
			}
		})
	}
}
