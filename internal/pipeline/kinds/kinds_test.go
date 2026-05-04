package kinds

import (
	"strings"
	"testing"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/project/models"
)

// TestNew_DispatchesByKind confirms every kind block routes through
// the matching constructor.
func TestNew_DispatchesByKind(t *testing.T) {
	llm, err := models.New(
		v1.ModelRef{Provider: v1.ProviderOpenAI, Name: "default"},
		v1.ModelEntry{Model: "gpt-4o-mini"},
	)
	if err != nil {
		t.Fatalf("models.New: %v", err)
	}

	child, err := New(
		Node{Autonomous: &v1.Autonomous{}},
		Deps{Name: "child", Instruction: "help", LLM: llm},
	)
	if err != nil {
		t.Fatalf("child agent: %v", err)
	}

	tests := []struct {
		name    string
		node    Node
		deps    Deps
		wantErr string
	}{
		{
			name: "autonomous",
			node: Node{Autonomous: &v1.Autonomous{}},
			deps: Deps{Name: "root", Instruction: "be useful", LLM: llm},
		},
		{
			name: "sequential",
			node: Node{Sequential: &v1.Sequential{}},
			deps: Deps{Name: "seq", Instruction: "x", Subagents: []Agent{child}},
		},
		{
			name: "parallel",
			node: Node{Parallel: &v1.Parallel{}},
			deps: Deps{Name: "par", Instruction: "x", Subagents: []Agent{child}},
		},
		{
			name: "loop",
			node: Node{Loop: &v1.Loop{MaxIterations: 3}},
			deps: Deps{Name: "lp", Instruction: "x", Subagents: []Agent{child}},
		},
		{
			name: "orchestrator",
			node: Node{Orchestrator: &v1.Orchestrator{
				Steps:  []v1.OrchestratorStep{{Name: "echo", Run: "ok"}},
				Output: "{{ .echo.output }}",
			}},
			deps: Deps{Name: "root", Instruction: "x"},
		},
		{
			name:    "no kind set",
			node:    Node{},
			deps:    Deps{Name: "root", Instruction: "x"},
			wantErr: "no kind block set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := New(tt.node, tt.deps)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v; want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v; want nil", err)
			}
			if a == nil {
				t.Fatal("agent = nil; want non-nil")
			}
			if a.Name() != tt.deps.Name {
				t.Errorf("agent.Name() = %q; want %q", a.Name(), tt.deps.Name)
			}
		})
	}
}

// TestNewAutonomous_RequiresLLM guards the precondition: an
// autonomous agent without a resolved model is a config bug, not a
// runtime fallback.
func TestNewAutonomous_RequiresLLM(t *testing.T) {
	_, err := New(
		Node{Autonomous: &v1.Autonomous{}},
		Deps{Name: "root", Instruction: "x"},
	)
	if err == nil || !strings.Contains(err.Error(), "requires a resolved model") {
		t.Fatalf("err = %v; want resolved-model error", err)
	}
}

// TestNew_RequiresSubagents collapses the three near-identical
// composition checks into one matrix so a regression flips one
// row rather than removing a whole test.
func TestNew_RequiresSubagents(t *testing.T) {
	tests := []struct {
		name string
		node Node
	}{
		{"sequential", Node{Sequential: &v1.Sequential{}}},
		{"parallel", Node{Parallel: &v1.Parallel{}}},
		{"loop", Node{Loop: &v1.Loop{MaxIterations: 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.node, Deps{Name: tt.name, Instruction: "x"})
			if err == nil || !strings.Contains(err.Error(), "requires at least one subagent") {
				t.Fatalf("err = %v; want subagent error", err)
			}
		})
	}
}
