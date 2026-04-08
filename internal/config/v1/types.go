// Package v1 defines the v1 config schema types for .agentsmithy.yaml.
//
// This is currently the latest (and only) config version. When v2 is
// introduced, create internal/config/v2/ with its own types and update
// Schema.Parse here to convert v1 → v2 via the new version's converter.
package v1

// Version is the schema version this package handles.
const Version = "1"

// Config is the root of .agentsmithy.yaml.
type Config struct {
	// Config schema version. Must be "1".
	Version string `yaml:"version" agentsmithy:"required"`
	// Project identity, description,
	Project Project `yaml:"project" agentsmithy:"required"`
}

// Project tells the AI what this codebase is.
type Project struct {
	// Human-readable project name.
	Name string `yaml:"name" agentsmithy:"required"`
	// Brief summary of what this project does. Shown alongside the name.
	Description string `yaml:"description" agentsmithy:"required"`
}
