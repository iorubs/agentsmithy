package tmpl

import (
	"encoding/json"
	"fmt"
	"strings"
)

// dictFunc builds a map literal inside a template from interleaved
// key-value pairs. Keys must be strings.
func dictFunc(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: expected even number of args, got %d", len(pairs))
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key %d is not a string (%T)", i, pairs[i])
		}
		m[k] = pairs[i+1]
	}
	return m, nil
}

// listFunc builds a slice literal inside a template.
func listFunc(items ...any) []any { return items }

// coalesceFunc returns the first argument that is not nil and not an
// empty-or-whitespace string. When everything is empty it returns "".
func coalesceFunc(values ...any) any {
	for _, v := range values {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		return v
	}
	return ""
}

// replaceFunc orders args (old, new, s) so the call reads naturally
// in a pipe: `{{ .x | replace "a" "b" }}`.
func replaceFunc(old, new, s string) string { return strings.ReplaceAll(s, old, new) }

// splitFunc orders args (sep, s) so the call reads naturally in a
// pipe: `{{ .x | split "," }}`.
func splitFunc(sep, s string) []string { return strings.Split(s, sep) }

// joinFunc joins []string or []any with sep. Useful for piping
// `{{ list ... | join ", " }}` and post-`split` reassembly.
func joinFunc(sep string, parts any) (string, error) {
	switch v := parts.(type) {
	case []string:
		return strings.Join(v, sep), nil
	case []any:
		out := make([]string, len(v))
		for i, p := range v {
			out[i] = fmt.Sprint(p)
		}
		return strings.Join(out, sep), nil
	default:
		return "", fmt.Errorf("join: expected []string or []any, got %T", parts)
	}
}

// fromJSONFunc parses a JSON string into a Go value (map, slice, or
// scalar). Most useful for upstream steps that emit structured args
// for downstream tool calls: `{{ tool "x" (fromJSON .planner.output) }}`.
func fromJSONFunc(s string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("fromJSON: %w", err)
	}
	return v, nil
}
