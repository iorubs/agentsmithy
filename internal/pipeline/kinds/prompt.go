package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/iorubs/agentsmithy/internal/project/models"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

// promptHelper backs the {{ prompt "..." }} template helper. It runs
// a synchronous one-shot LLM call against the agent's model and
// returns the concatenated text response.
func promptHelper(ctx context.Context, selfName string, llm models.LLM) func(string, ...any) (string, error) {
	return func(text string, _ ...any) (string, error) {
		if llm == nil {
			return "", fmt.Errorf("agent %q: prompt: no model in scope", selfName)
		}
		req := &adkmodel.LLMRequest{
			Contents: []*genai.Content{{
				Role:  string(genai.RoleUser),
				Parts: []*genai.Part{{Text: text}},
			}},
			Config: &genai.GenerateContentConfig{},
		}
		var out strings.Builder
		for resp, err := range llm.GenerateContent(ctx, req, false) {
			if err != nil {
				return "", fmt.Errorf("prompt: %w", err)
			}
			if resp == nil || resp.Content == nil {
				continue
			}
			for _, p := range resp.Content.Parts {
				if p == nil || p.Thought {
					continue
				}
				out.WriteString(p.Text)
			}
		}
		return out.String(), nil
	}
}
