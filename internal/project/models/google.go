package models

import (
	"errors"

	"github.com/iorubs/agentsmithy/internal/config"
)

// newGoogle returns an error; the Google provider is not yet implemented.
func newGoogle(_ config.ModelEntry) (LLM, error) {
	return nil, errors.New("provider google: not implemented yet")
}
