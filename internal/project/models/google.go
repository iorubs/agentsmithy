package models

import (
	"context"
	"errors"
	"iter"
	"os"

	"github.com/iorubs/agentsmithy/internal/config"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

const defaultGoogleKeyEnv = "GOOGLE_API_KEY"

func newGoogle(ctx context.Context, entry config.ModelEntry) (LLM, error) {
	if entry.Model == "" {
		return nil, errors.New("google: model is required")
	}

	keyEnv := entry.APIKeyEnv
	if keyEnv == "" {
		keyEnv = defaultGoogleKeyEnv
	}

	apiKey := os.Getenv(keyEnv)
	if apiKey == "" {
		return nil, errors.New("google: API key not found in " + keyEnv)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	return &googleLLM{
		entry:  entry,
		client: client,
	}, nil
}

type googleLLM struct {
	entry  config.ModelEntry
	client *genai.Client
}

func (m *googleLLM) Name() string { return m.entry.Model }

func (m *googleLLM) GenerateContent(
	ctx context.Context,
	req *adkmodel.LLMRequest,
	_ bool,
) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		if req.Config == nil {
			req.Config = &genai.GenerateContentConfig{}
		}

		resp, err := m.client.Models.GenerateContent(ctx, m.entry.Model, req.Contents, req.Config)
		if err != nil {
			yield(nil, err)
			return
		}
		if resp == nil || len(resp.Candidates) == 0 {
			yield(nil, errors.New("google: empty response from model"))
			return
		}

		for _, candidate := range resp.Candidates {
			if candidate == nil || candidate.Content == nil {
				continue
			}
			if !yield(&adkmodel.LLMResponse{
				Content:      candidate.Content,
				ModelVersion: resp.ModelVersion,
			}, nil) {
				return
			}
		}
	}
}
