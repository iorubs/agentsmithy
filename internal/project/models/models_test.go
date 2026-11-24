package models

import (
	"context"
	"strings"
	"testing"

	"github.com/iorubs/agentsmithy/internal/config"
)

// TestNew_ProvidersResolve confirms every provider key dispatches
// through New and returns a working LLM once its credentials are
// present. google keeps a credential-less row to prove the missing-key
// path reports the env var it looked at.
func TestNew_ProvidersResolve(t *testing.T) {
	maxTokens := 256
	tests := []struct {
		provider config.Provider
		entry    config.ModelEntry
		env      map[string]string
		wantErr  string
	}{
		{config.ProviderOpenAI, config.ModelEntry{Model: "gpt-4o-mini"}, nil, ""},
		{config.ProviderBorrowed, config.ModelEntry{MaxTokens: &maxTokens}, nil, ""},
		{config.ProviderBedrock, config.ModelEntry{Model: "anthropic.claude-3-5-sonnet-20241022-v2:0"}, nil, ""},
		{config.ProviderAnthropic, config.ModelEntry{Model: "claude-sonnet-4-5", APIKeyEnv: "TEST_ANTHROPIC_KEY"},
			map[string]string{"TEST_ANTHROPIC_KEY": "sk-test"}, ""},
		{config.ProviderGoogle, config.ModelEntry{Model: "x", APIKeyEnv: "TEST_GOOGLE_UNSET_KEY"}, nil, "API key not found"},
		{config.ProviderVertex, config.ModelEntry{Model: "gemini-2.5-flash", APIKeyEnv: "TEST_VERTEX_KEY"},
			map[string]string{"TEST_VERTEX_KEY": "vk-test"}, ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			llm, err := New(context.Background(), config.ModelRef{Provider: tt.provider, Name: "default"}, tt.entry)
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
	if _, err := New(context.Background(), config.ModelRef{Provider: config.ProviderOpenAI, Name: "x"}, config.ModelEntry{}); err == nil {
		t.Fatal("openai with empty Model: err = nil; want error")
	}
}

// TestNew_AnthropicRequiresAPIKey verifies the anthropic provider
// names the env var it looked at rather than failing at first call.
func TestNew_AnthropicRequiresAPIKey(t *testing.T) {
	_, err := New(context.Background(),
		config.ModelRef{Provider: config.ProviderAnthropic, Name: "x"},
		config.ModelEntry{Model: "claude-sonnet-4-5", APIKeyEnv: "TEST_ANTHROPIC_UNSET_KEY"})
	if err == nil || !strings.Contains(err.Error(), "TEST_ANTHROPIC_UNSET_KEY") {
		t.Fatalf("err = %v; want missing-key error naming the env var", err)
	}
}

// TestNew_UnknownProvider surfaces a clear error rather than a nil
// LLM when ref.Provider does not match any known kind.
func TestNew_UnknownProvider(t *testing.T) {
	_, err := New(context.Background(), config.ModelRef{Provider: "ollama", Name: "x"}, config.ModelEntry{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("err = %v; want unknown provider error", err)
	}
}
