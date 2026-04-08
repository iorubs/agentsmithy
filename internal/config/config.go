// Package config loads and version-routes .agentsmithy.yaml configs.
//
// Each config schema version lives in its own sub-package (v1/, v2/, …)
// and satisfies the [VersionSchema] interface. The loader reads raw YAML,
// detects the "version" field, and delegates to the correct version.
//
// Adding a new version:
//  1. Create the vN/ sub-package satisfying [VersionSchema]
//  2. Register it in the versions map in this file
//
// Type aliases below re-export the latest version's types so that
// consumers can keep importing "internal/config" without change.
package config

import (
	"fmt"
	"maps"
	"slices"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"go.yaml.in/yaml/v4"
)

// Versions is the single source of truth for which schema versions are
// accepted. Each entry satisfies [VersionSchema].
var Versions = map[string]VersionSchema{
	v1.Version: v1.Schema{},
}

// VersionSchema is the contract each config version must satisfy.
// The Parse method must return the latest Config type (converting if needed).
type VersionSchema interface {
	Parse([]byte) (*Config, error)
	RootType() any
	TypesSources() []string
}

// TypesSources returns the raw Go source files for the latest version's types.
// Callers that only need the latest version can use this directly.
var TypesSources = v1.TypesSources

// Type aliases: always point to the latest version.
type (
	Config = v1.Config
)

// Parse parses raw YAML bytes, detects the version, delegates to the
// correct versioned parser, and converts the result to the latest
// Config type.
func Parse(data []byte) (*Config, error) {
	var header struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	e, ok := Versions[header.Version]
	if !ok {
		return nil, fmt.Errorf("unsupported config version %q (supported: %s)",
			header.Version, slices.Sorted(maps.Keys(Versions)))
	}
	return e.Parse(data)
}
