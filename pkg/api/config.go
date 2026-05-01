// Package api is the public, stable surface for embedding agentsmithy
// into other Go programs (notably smithy-cli). It re-exports the small
// set of operations needed to load config, run a server, or chat with
// an agent, all without depending on the internal/* packages.
package api

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iorubs/agentsmithy/internal/config"
)

// LoadConfig reads and parses the config file at path and returns the
// parsed config along with the resolved project root (the directory containing the file).
func LoadConfig(path string) (*config.Config, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("config: %w", err)
	}

	cfg, err := config.Parse(data)
	if err != nil {
		return nil, "", fmt.Errorf("config: %w", err)
	}

	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, "", fmt.Errorf("resolving project root: %w", err)
	}

	return cfg, root, nil
}
