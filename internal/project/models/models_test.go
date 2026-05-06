package models

import (
	"strings"
	"testing"

	"github.com/iorubs/agentsmithy/internal/config"
)

// TestNew_ProvidersResolve confirms every v0.1 provider key dispatches
// through New: openai + borrowed return a working LLM, the other four
// return their not-implemented sentinel.
func TestNew_ProvidersResolve(t *testing.T) {
	maxTokens := 256
	tests := []struct {
		provider config.Provider
		entry    config.ModelEntry
		wantErr  string
	}{
		{config.ProviderOpenAI, config.ModelEntry{Model: "gpt-4o-mini"}, ""},
		{config.ProviderBorrowed, config.ModelEntry{MaxTokens: &maxTokens}, ""},
		{config.ProviderBedrock, config.ModelEntry{Model: "anthropic.claude-3-5-sonnet-20241022-v2:0"}, ""},
		{config.ProviderAnthropic, config.ModelEntry{Model: "x"}, "not implemented yet"},
		{config.ProviderGoogle, config.ModelEntry{Model: "x"}, "not implemented yet"},
		{config.ProviderVertex, config.ModelEntry{Model: "x"}, "not implemented yet"},
	}
	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			llm, err := New(config.ModelRef{Provider: tt.provider, Name: "default"}, tt.entry)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("New(%s): err = %v; want containing %q", tt.provider, err, tt.wantErr)
				}
				if llm != nil {
					t.Errorf("New(%s): llm = %v; want nil on error", tt.provider, llm)
				}
				return
			}
			if err != nil {
				t.Fatalf("New(%s): err = %v; want nil", tt.provider, err)
			}
			if llm == nil {
				t.Fatalf("New(%s): llm = nil; want non-nil", tt.provider)
			}
			if llm.Name() == "" {
				t.Errorf("New(%s).Name() empty", tt.provider)
			}
		})
	}
}

// TestNew_OpenAIRequiresModel verifies the openai provider rejects
// an empty Model at New (rather than at first call).
func TestNew_OpenAIRequiresModel(t *testing.T) {
	if _, err := New(config.ModelRef{Provider: config.ProviderOpenAI, Name: "x"}, config.ModelEntry{}); err == nil {
		t.Fatal("openai with empty Model: err = nil; want error")
	}
}

// TestNew_UnknownProvider surfaces a clear error rather than a nil
// LLM when ref.Provider does not match any known kind.
func TestNew_UnknownProvider(t *testing.T) {
	_, err := New(config.ModelRef{Provider: "ollama", Name: "x"}, config.ModelEntry{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("err = %v; want unknown provider error", err)
	}
}
