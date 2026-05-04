package kinds

import (
	"context"
	"strings"
	"testing"
)

func TestUnsupportedHelper(t *testing.T) {
	h := unsupportedHelper("tool", "self", "until: predicate")
	out, err := h("search", "q")
	if out != "" {
		t.Errorf("out = %q; want empty", out)
	}
	if err == nil {
		t.Fatal("err = nil; want error")
	}
	for _, want := range []string{`agent "self"`, `tool "search"`, "until: predicate", "orchestrator"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err missing %q: %v", want, err)
		}
	}
}

func TestSkillDispatch_Unknown(t *testing.T) {
	s := skillDispatch(t.Context(), "self", map[string]SkillHelper{})
	v, err := s("review", "args")
	if v != nil {
		t.Errorf("v = %v; want nil", v)
	}
	if err == nil || !strings.Contains(err.Error(), "no skills declared") {
		t.Fatalf("err = %v; want no-skills-declared", err)
	}
}

func TestSkillDispatch_NotDeclared(t *testing.T) {
	s := skillDispatch(t.Context(), "self", map[string]SkillHelper{
		"other": func(_ context.Context, _ ...any) (any, error) { return nil, nil },
	})
	_, err := s("review")
	if err == nil || !strings.Contains(err.Error(), `not declared`) {
		t.Fatalf("err = %v; want not-declared", err)
	}
}
