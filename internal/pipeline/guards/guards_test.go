package guards

import (
	"strings"
	"testing"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// TestBuild_RequireToolCall lowers the requireToolCall enum value
// into the matching ResponseGuard.
func TestBuild_RequireToolCall(t *testing.T) {
	got, err := Build([]v1.Guard{v1.GuardRequireToolCall})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(got) != 1 || got[0].Name != "requireToolCall" {
		t.Fatalf("got %+v; want one requireToolCall guard", got)
	}
}

// TestBuild_Empty returns nil for an empty list (no callback wiring).
func TestBuild_Empty(t *testing.T) {
	got, err := Build(nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got != nil {
		t.Fatalf("got %v; want nil", got)
	}
}

// TestBuild_Unknown surfaces an explicit error so misconfiguration
// can't silently disable a guard.
func TestBuild_Unknown(t *testing.T) {
	_, err := Build([]v1.Guard{"bogus"})
	if err == nil || !strings.Contains(err.Error(), `unknown guard "bogus"`) {
		t.Fatalf("err = %v; want unknown guard", err)
	}
}

// TestRequireToolCall_PassesWithFunctionCall accepts a response that
// contains at least one function call.
func TestRequireToolCall_PassesWithFunctionCall(t *testing.T) {
	g := RequireToolCall()
	resp := &model.LLMResponse{Content: &genai.Content{
		Parts: []*genai.Part{
			{Text: "calling tool"},
			{FunctionCall: &genai.FunctionCall{Name: "search"}},
		},
	}}
	if msg := g.Check(resp); msg != nil {
		t.Fatalf("Check = %q; want nil (passes)", *msg)
	}
}

// TestRequireToolCall_RejectsTextOnly returns a corrective message
// when the response carries only text.
func TestRequireToolCall_RejectsTextOnly(t *testing.T) {
	g := RequireToolCall()
	resp := &model.LLMResponse{Content: &genai.Content{
		Parts: []*genai.Part{{Text: "the answer is 42"}},
	}}
	msg := g.Check(resp)
	if msg == nil {
		t.Fatal("Check = nil; want corrective message")
	}
	if !strings.Contains(*msg, "tool") {
		t.Errorf("corrective = %q; want mention of tool", *msg)
	}
}

// TestRequireToolCall_NilResponse no-ops on nil/empty response (the
// callback layer treats nil as accept and surfaces upstream errors).
func TestRequireToolCall_NilResponse(t *testing.T) {
	g := RequireToolCall()
	if msg := g.Check(nil); msg != nil {
		t.Errorf("Check(nil) = %q; want nil", *msg)
	}
	if msg := g.Check(&model.LLMResponse{}); msg != nil {
		t.Errorf("Check(empty) = %q; want nil", *msg)
	}
}

// TestCallback_NilWhenEmpty avoids attaching a no-op callback.
func TestCallback_NilWhenEmpty(t *testing.T) {
	if cb := Callback(nil, 0); cb != nil {
		t.Errorf("Callback(nil) = non-nil; want nil")
	}
}
