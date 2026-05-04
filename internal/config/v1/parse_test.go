package v1

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		want    *Config
	}{
		{
			name: "happy path autonomous",
			yaml: `version: "1"
project:
  name: sample-assistant
  instruction: |
    Research assistant for sample.
  models:
    openai:
      default:
        model: qwen2.5:7b-instruct
        baseUrl: http://localhost:11434/v1
        temperature: 0.2
        maxTokens: 2048
tools:
  mcp:
    docs: "http://localhost:8080/"
  a2a:
    reviewer: "http://localhost:9090/"
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    tools: [docs, reviewer]
    skills:
      guards: [requireToolCall]
    maxIterations: 3
`,
			want: &Config{
				Version: "1",
				Project: Project{
					Name:        "sample-assistant",
					Instruction: "Research assistant for sample.\n",
					Models: Models{
						OpenAI: map[string]ModelEntry{
							"default": {
								Model:       "qwen2.5:7b-instruct",
								BaseURL:     "http://localhost:11434/v1",
								Temperature: new(0.2),
								MaxTokens:   new(2048),
							},
						},
					},
				},
				Tools: Tools{
					MCP: map[string]string{"docs": "http://localhost:8080/"},
					A2A: map[string]string{"reviewer": "http://localhost:9090/"},
				},
				Pipeline: Pipeline{
					Autonomous: &Autonomous{
						Model: &ModelRef{Provider: ProviderOpenAI, Name: "default"},
						Tools: []string{"docs", "reviewer"},
						Skills: Skills{
							Guards: []Guard{GuardRequireToolCall},
						},
						MaxIterations: 3,
					},
				},
			},
		},
		{
			name: "sequential with sub-agents",
			yaml: `version: "1"
project:
  name: pipeline
  instruction: root
  models:
    openai:
      default: { model: m }
pipeline:
  sequential:
    model: { provider: openai, name: default }
    subagents:
      - name: drafter
        autonomous:
          instruction: draft
          inherits: [model]
      - name: reviewer
        autonomous:
          instruction: review
          inherits: [model, tools]
`,
			want: &Config{
				Version: "1",
				Project: Project{
					Name:        "pipeline",
					Instruction: "root",
					Models: Models{
						OpenAI: map[string]ModelEntry{"default": {Model: "m"}},
					},
				},
				Pipeline: Pipeline{
					Sequential: &Sequential{
						Model: &ModelRef{Provider: ProviderOpenAI, Name: "default"},
						Subagents: []SubAgent{
							{
								Name: "drafter",
								Autonomous: &Autonomous{
									Instruction: "draft",
									Inherits:    []InheritField{InheritModel},
								},
							},
							{
								Name: "reviewer",
								Autonomous: &Autonomous{
									Instruction: "review",
									Inherits:    []InheritField{InheritModel, InheritTools},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "orchestrator with steps",
			yaml: `version: "1"
project:
  name: orch
  instruction: root
  models:
    anthropic:
      default: { model: claude-3-5-sonnet }
pipeline:
  orchestrator:
    model: { provider: anthropic, name: default }
    steps:
      - name: research
        run: '{{ tool "docs" .input }}'
      - name: review
        run: '{{ tool "reviewer" .research.output }}'
    output: '{{ coalesce .review.output .research.output }}'
`,
			want: &Config{
				Version: "1",
				Project: Project{
					Name:        "orch",
					Instruction: "root",
					Models: Models{
						Anthropic: map[string]ModelEntry{"default": {Model: "claude-3-5-sonnet"}},
					},
				},
				Pipeline: Pipeline{
					Orchestrator: &Orchestrator{
						Model: &ModelRef{Provider: ProviderAnthropic, Name: "default"},
						Steps: []OrchestratorStep{
							{Name: "research", Run: `{{ tool "docs" .input }}`},
							{Name: "review", Run: `{{ tool "reviewer" .research.output }}`},
						},
						Output: `{{ coalesce .review.output .research.output }}`,
					},
				},
			},
		},
		{
			name: "loop with memory overrides",
			yaml: `version: "1"
project:
  name: loop-pipeline
  instruction: root
  models:
    openai:
      default: { model: m }
pipeline:
  loop:
    model: { provider: openai, name: default }
    maxIterations: 5
    until: '{{ skill "tests-pass" .codegen.output }}'
    memory:
      retain: false
      inherit: true
    subagents:
      - name: codegen
        autonomous:
          instruction: write
          inherits: [model]
`,
			want: &Config{
				Version: "1",
				Project: Project{
					Name:        "loop-pipeline",
					Instruction: "root",
					Models: Models{
						OpenAI: map[string]ModelEntry{"default": {Model: "m"}},
					},
				},
				Pipeline: Pipeline{
					Loop: &Loop{
						Model:         &ModelRef{Provider: ProviderOpenAI, Name: "default"},
						MaxIterations: 5,
						Until:         `{{ skill "tests-pass" .codegen.output }}`,
						Memory: Memory{
							Retain:  new(false),
							Inherit: new(true),
						},
						Subagents: []SubAgent{
							{
								Name: "codegen",
								Autonomous: &Autonomous{
									Instruction: "write",
									Inherits:    []InheritField{InheritModel},
								},
							},
						},
					},
				},
			},
		},

		{
			name:    "malformed YAML",
			yaml:    "version: \"1\"\nname: [bad",
			wantErr: "parsing config",
		},
		{
			name: "unknown field rejected",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    bogus: nope
`,
			wantErr: "parsing config",
		},
		{
			name: "missing required name",
			yaml: `version: "1"
project:
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  autonomous:
    model: { provider: openai, name: default }
`,
			wantErr: "name is required",
		},
		{
			name: "missing required instruction at project level",
			yaml: `version: "1"
project:
  name: x
  models:
    openai:
      default: { model: m }
pipeline:
  autonomous:
    model: { provider: openai, name: default }
`,
			wantErr: "instruction is required",
		},
		{
			name: "no kind set on pipeline",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline: {}
`,
			wantErr: "must set one of",
		},
		{
			name: "two kinds set on pipeline",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  autonomous:
    model: { provider: openai, name: default }
  sequential:
    model: { provider: openai, name: default }
    subagents:
      - name: c
        autonomous: { instruction: i }
`,
			wantErr: "mutually exclusive",
		},
		{
			name: "subagent without kind",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  sequential:
    model: { provider: openai, name: default }
    subagents:
      - name: child
`,
			wantErr: "must set one of",
		},
		{
			name: "invalid provider",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  autonomous:
    model: { provider: bogus, name: default }
`,
			wantErr: "must be one of [openai, anthropic, google, bedrock, vertex, borrowed]",
		},
		{
			name: "subagent missing instruction",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  sequential:
    model: { provider: openai, name: default }
    subagents:
      - name: child
        autonomous: {}
`,
			wantErr: "instruction is required",
		},
		{
			name: "loop missing maxIterations",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  loop:
    model: { provider: openai, name: default }
    subagents:
      - name: child
        autonomous: { instruction: i }
`,
			wantErr: "maxIterations is required",
		},
		{
			name: "sequential missing subagents",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  sequential:
    model: { provider: openai, name: default }
`,
			wantErr: "subagents is required",
		},
		{
			name: "orchestrator missing steps",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  orchestrator:
    model: { provider: openai, name: default }
    output: '{{ .input }}'
`,
			wantErr: "steps is required",
		},
		{
			name: "orchestrator missing output",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  orchestrator:
    model: { provider: openai, name: default }
    steps:
      - name: s1
        run: '{{ .input }}'
`,
			wantErr: "output is required",
		},
		{
			name: "orchestrator step missing run",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  orchestrator:
    model: { provider: openai, name: default }
    output: '{{ .s1.output }}'
    steps:
      - name: s1
`,
			wantErr: "run is required",
		},
		{
			name: "model name not in catalog",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  autonomous:
    model: { provider: openai, name: bogus }
`,
			wantErr: `"bogus" does not match any declared key`,
		},
		{
			name: "subagent name reserved",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  sequential:
    model: { provider: openai, name: default }
    subagents:
      - name: agentsmithy
        autonomous: { instruction: i }
`,
			wantErr: "reserved name",
		},
		{
			name: "template syntax error in step run",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  orchestrator:
    model: { provider: openai, name: default }
    output: '{{ .s1.output }}'
    steps:
      - name: s1
        run: '{{ tool "docs"'
`,
			wantErr: "template:",
		},
		{
			name: "unknown helper in output",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  orchestrator:
    model: { provider: openai, name: default }
    output: '{{ bogus .input }}'
    steps:
      - name: s1
        run: '{{ .input }}'
`,
			wantErr: `function "bogus" not defined`,
		},
		{
			name: "tool name not in catalog",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
tools:
  mcp:
    docs: http://localhost:7000
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    tools: [doc]
`,
			wantErr: `"doc" does not match any declared key`,
		},
		{
			name: "tool name resolves via mcp or a2a",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
tools:
  mcp:
    docs: http://localhost:7000
  a2a:
    reviewer: http://localhost:7100
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    tools: [docs, reviewer]
`,
		},
		{
			name: "invalid guard name",
			yaml: `version: "1"
project:
  name: x
  instruction: i
  models:
    openai:
      default: { model: m }
pipeline:
  autonomous:
    model: { provider: openai, name: default }
    skills:
      guards: [bogusGuard]
`,
			wantErr: `must be one of [requireToolCall]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Schema{}.Parse([]byte(tt.yaml))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.want != nil && !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("Parse() mismatch\ngot:  %+v\nwant: %+v", cfg, tt.want)
			}
		})
	}
}

func TestSchemaRootType(t *testing.T) {
	s := Schema{}
	rt := s.RootType()
	if _, ok := rt.(Config); !ok {
		t.Errorf("expected Config, got %T", rt)
	}
}

func new[T any](v T) *T { return &v }
