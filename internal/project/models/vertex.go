package models

import (
	"errors"

	"github.com/iorubs/agentsmithy/internal/config"
)

// newVertex returns an error; the Vertex AI provider is not yet implemented.
func newVertex(_ config.ModelEntry) (LLM, error) {
	return nil, errors.New("provider vertex: not implemented yet")
}
