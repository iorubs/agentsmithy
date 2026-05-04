package pipeline

import (
	"strings"
	"testing"

	"github.com/iorubs/agentsmithy/internal/config"
	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
)

// baseConfig returns a minimal valid Config the build tests can
// mutate. The default catalog has one openai entry under "default".
func baseConfig() *config.Config {
	return &config.Config{
		Version: v1.Version,
		Project: v1.Project{
			Name:        "demo",
			Instruction: "be useful",
			Models: config.Models{
				OpenAI: map[string]config.ModelEntry{
					"default": {Model: "gpt-4o-mini"},
				},
			},
		},
		Pipeline: v1.Pipeline{
			Autonomous: &v1.Autonomous{
				Model: &config.ModelRef{Provider: config.ProviderOpenAI, Name: "default"},
			},
		},
	}
}

// TestBuild_AutonomousRoot exercises the happy path: root pipeline is
// autonomous, model ref resolves, no sub-agents, no tools/skills.
func TestBuild_AutonomousRoot(t *testing.T) {
	p, err := Build(baseConfig())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if p.Name != "demo" {
		t.Errorf("Name = %q; want demo", p.Name)
	}
	if p.Root == nil {
		t.Fatal("Root = nil")
	}
	if p.Root.Name() != "demo" {
		t.Errorf("Root.Name() = %q; want demo", p.Root.Name())
	}
}

// TestBuild_AutonomousWithSubagent confirms sub-agents are built
// post-order and handed in via Deps.Subagents.
func TestBuild_AutonomousWithSubagent(t *testing.T) {
	cfg := baseConfig()
	cfg.Pipeline.Autonomous.Subagents = []v1.SubAgent{{
		Name: "helper",
		Autonomous: &v1.Autonomous{
			Instruction: "do one thing",
			Model:       &config.ModelRef{Provider: config.ProviderOpenAI, Name: "default"},
		},
	}}

	p, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	subs := p.Root.SubAgents()
	if len(subs) != 1 {
		t.Fatalf("subagents = %d; want 1", len(subs))
	}
	if subs[0].Name() != "helper" {
		t.Errorf("subagent name = %q; want helper", subs[0].Name())
	}
}

// TestBuild_UnknownModelRef surfaces the catalog-miss diagnostic the
// loader can't catch (loader only checks structural validity, not
// catalog membership at build time).
func TestBuild_UnknownModelRef(t *testing.T) {
	cfg := baseConfig()
	cfg.Pipeline.Autonomous.Model = &config.ModelRef{Provider: config.ProviderOpenAI, Name: "missing"}
	_, err := Build(cfg)
	if err == nil || !strings.Contains(err.Error(), "not in catalog") {
		t.Fatalf("err = %v; want not-in-catalog", err)
	}
}

// TestBuild_RefusesUnsupported lists the surfaces the pipeline
// package intentionally rejects because they are not yet supported.
// Each surface gets a row so a regression flips one error rather
// than silently dropping config.
func TestBuild_RefusesUnsupported(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(a *v1.Autonomous)
		wantErr string
	}{
		{
			name:    "missing tool reference",
			mutate:  func(a *v1.Autonomous) { a.Tools = []string{"web"} },
			wantErr: `tool "web" not in catalog`,
		},
		{
			name: "memory",
			mutate: func(a *v1.Autonomous) {
				retain := true
				a.Memory.Retain = &retain
			},
			wantErr: "memory: not implemented yet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			tt.mutate(cfg.Pipeline.Autonomous)
			_, err := Build(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v; want containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestBuild_SubAgentInheritsModel exercises the inherits resolution
// path: the sub-agent omits its own `model:` and pulls the parent's
// via `inherits: [model]`.
func TestBuild_SubAgentInheritsModel(t *testing.T) {
	cfg := baseConfig()
	cfg.Pipeline.Autonomous.Subagents = []v1.SubAgent{{
		Name: "helper",
		Autonomous: &v1.Autonomous{
			Instruction: "x",
			Inherits:    []v1.InheritField{v1.InheritModel},
		},
	}}
	p, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	subs := p.Root.SubAgents()
	if len(subs) != 1 || subs[0].Name() != "helper" {
		t.Fatalf("subagents = %v; want [helper]", subs)
	}
}

// TestBuild_SubAgentInheritsUnresolvable surfaces the diagnostic
// when `inherits:` points at a field no ancestor declares.
func TestBuild_SubAgentInheritsUnresolvable(t *testing.T) {
	cfg := baseConfig()
	cfg.Pipeline.Autonomous.Subagents = []v1.SubAgent{{
		Name: "helper",
		Autonomous: &v1.Autonomous{
			Instruction: "x",
			Model:       &config.ModelRef{Provider: config.ProviderOpenAI, Name: "default"},
			Inherits:    []v1.InheritField{v1.InheritTools},
		},
	}}
	_, err := Build(cfg)
	if err == nil || !strings.Contains(err.Error(), `inherits "tools"`) {
		t.Fatalf("err = %v; want unresolvable inherits", err)
	}
}
