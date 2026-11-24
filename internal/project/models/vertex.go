package models

import (
	"context"
	"errors"
	"os"

	"github.com/iorubs/agentsmithy/internal/config"
	"google.golang.org/genai"
)

// newVertex builds an LLM backed by Vertex AI. Project and location come
// from the genai client's own resolution (GOOGLE_CLOUD_PROJECT,
// GOOGLE_CLOUD_LOCATION); credentials default to application default
// credentials unless entry.APIKeyEnv names an express-mode key.
// The wire is identical to the google provider.
func newVertex(ctx context.Context, entry config.ModelEntry) (LLM, error) {
	if entry.Model == "" {
		return nil, errors.New("vertex: model is required")
	}

	cfg := &genai.ClientConfig{Backend: genai.BackendVertexAI}
	if entry.APIKeyEnv != "" {
		apiKey := os.Getenv(entry.APIKeyEnv)
		if apiKey == "" {
			return nil, errors.New("vertex: API key not found in " + entry.APIKeyEnv)
		}
		cfg.APIKey = apiKey
	}

	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &googleLLM{entry: entry, client: client}, nil
}
