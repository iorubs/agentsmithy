// Package skills lowers a v1.Skills block into runtime ADK tools and
// template helpers. Each declared skill (shell entries, file ops,
// web ops) becomes one or more adktool.Tool callable by autonomous
// agents and one or more Helpers callable from `{{ skill "name" ...}}`
// in composition templates.
package skills

import (
	"context"
	"fmt"

	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	adktool "google.golang.org/adk/tool"
)

// Helper is the runtime form of a skill invoked from a template. Args
// are positional, matching the order used at the template call site.
type Helper func(ctx context.Context, args ...any) (any, error)

// Built is the lowered result of one node's Skills block.
type Built struct {
	// Tools is the ADK tool list to append to the node's Tools.
	Tools []adktool.Tool
	// Helpers maps skill (or built-in op) name to its template helper.
	Helpers map[string]Helper
}

// Build lowers s into ADK tools + template helpers. projectRoot is
// used to resolve any relative WorkingDir field.
func Build(agentName, projectRoot string, s v1.Skills) (Built, error) {
	out := Built{Helpers: map[string]Helper{}}

	for name, sk := range s.Shell {
		if err := sk.Validate(name); err != nil {
			return Built{}, err
		}
		t, h, err := buildShell(projectRoot, name, sk)
		if err != nil {
			return Built{}, fmt.Errorf("agent %q: shell skill %q: %w", agentName, name, err)
		}
		if _, dup := out.Helpers[name]; dup {
			return Built{}, fmt.Errorf("agent %q: skill %q: name already used", agentName, name)
		}
		out.Tools = append(out.Tools, t)
		out.Helpers[name] = h
	}

	if s.File != nil {
		ft, fh, err := buildFile(projectRoot, *s.File)
		if err != nil {
			return Built{}, fmt.Errorf("agent %q: file skill: %w", agentName, err)
		}
		out.Tools = append(out.Tools, ft...)
		for k, v := range fh {
			if _, dup := out.Helpers[k]; dup {
				return Built{}, fmt.Errorf("agent %q: skill %q: name already used", agentName, k)
			}
			out.Helpers[k] = v
		}
	}

	if s.Web != nil {
		if err := s.Web.Validate(); err != nil {
			return Built{}, fmt.Errorf("agent %q: web skill: %w", agentName, err)
		}
		wt, wh := buildWeb(*s.Web)
		out.Tools = append(out.Tools, wt...)
		for k, v := range wh {
			if _, dup := out.Helpers[k]; dup {
				return Built{}, fmt.Errorf("agent %q: skill %q: name already used", agentName, k)
			}
			out.Helpers[k] = v
		}
	}

	return out, nil
}
