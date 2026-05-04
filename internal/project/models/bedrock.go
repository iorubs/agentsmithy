package models

import (
	"errors"

	"github.com/iorubs/agentsmithy/internal/config"
)

// newBedrock returns an error; the Bedrock provider is not yet implemented.
func newBedrock(_ config.ModelEntry) (LLM, error) {
	return nil, errors.New("provider bedrock: not implemented yet")
}
