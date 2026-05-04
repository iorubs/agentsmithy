package models

import (
	"errors"

	"github.com/iorubs/agentsmithy/internal/config"
)

// newAnthropic returns an error; the Anthropic provider is not yet implemented.
func newAnthropic(_ config.ModelEntry) (LLM, error) {
	return nil, errors.New("provider anthropic: not implemented yet")
}
