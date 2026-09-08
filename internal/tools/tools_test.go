package tools

import (
	"errors"
	"slices"
	"testing"

	"google.golang.org/adk/agent"
	adktool "google.golang.org/adk/tool"
)

func TestNormaliseMCPEndpoint(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http://localhost:8080/", "http://localhost:8080/"},
		{"https://example.com/mcp", "https://example.com/mcp"},
		{"localhost:8080", "http://localhost:8080/"},
		{":8080", "http://127.0.0.1:8080/"},
	}
	for _, c := range cases {
		if got := normaliseMCPEndpoint(c.in); got != c.want {
			t.Errorf("normaliseMCPEndpoint(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// fakeTool is a minimal adktool.Tool that only needs a name.
type fakeTool struct {
	adktool.Tool
	name string
}

func (f fakeTool) Name() string { return f.name }

// fakeToolset returns a fixed set of tools.
type fakeToolset struct {
	tools []adktool.Tool
	err   error
}

func (fakeToolset) Name() string { return "fake" }

func (f fakeToolset) Tools(agent.ReadonlyContext) ([]adktool.Tool, error) {
	return f.tools, f.err
}

func TestAllowToolset_Tools(t *testing.T) {
	inner := fakeToolset{tools: []adktool.Tool{
		fakeTool{name: "search"},
		fakeTool{name: "fetch"},
		fakeTool{name: "write"},
	}}
	cases := []struct {
		name  string
		allow []string
		want  []string
	}{
		{"subset", []string{"search", "write"}, []string{"search", "write"}},
		{"single", []string{"fetch"}, []string{"fetch"}},
		{"unknown name dropped", []string{"search", "nope"}, []string{"search"}},
		{"all unknown", []string{"nope"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ts := &allowToolset{server: "docs", inner: inner, allow: c.allow}
			got, err := ts.Tools(nil)
			if err != nil {
				t.Fatalf("Tools() error: %v", err)
			}
			var names []string
			for _, tool := range got {
				names = append(names, tool.Name())
			}
			if !slices.Equal(names, c.want) {
				t.Errorf("Tools() = %v, want %v", names, c.want)
			}
		})
	}
}

func TestAllowToolset_PropagatesError(t *testing.T) {
	ts := &allowToolset{server: "docs", inner: fakeToolset{err: errors.New("boom")}, allow: []string{"search"}}
	if _, err := ts.Tools(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestMCP_NoAllowListLeavesToolsetUnwrapped(t *testing.T) {
	r, err := MCP("docs", "http://localhost:9999/", nil)
	if err != nil {
		t.Fatalf("MCP() error: %v", err)
	}
	if _, wrapped := r.Toolset.(*allowToolset); wrapped {
		t.Error("toolset wrapped despite empty allow list")
	}
	r, err = MCP("docs", "http://localhost:9999/", []string{"search"})
	if err != nil {
		t.Fatalf("MCP() error: %v", err)
	}
	if _, wrapped := r.Toolset.(*allowToolset); !wrapped {
		t.Error("toolset not wrapped despite allow list")
	}
}
