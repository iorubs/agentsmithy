// Package tmpl is the shared template engine for agentsmithy pipelines.
//
// All template-bearing config fields (output:, until:, steps[].run)
// parse and execute through this package. Pure helpers (dict, list,
// coalesce, contains, trim, lower, upper, replace, split, join,
// fromJSON) are always available alongside Go's text/template
// builtins (printf, urlquery, index, len, eq, and, or, not, ...).
// Side-effecting helpers (tool, agent, skill, prompt) are bound at
// execution time by the caller via RuntimeFuncs.
package tmpl

import (
	"bytes"
	"maps"
	"strings"
	"text/template"
)

// ParseFuncs returns stub functions for parse-time validation. The
// bodies are no-ops; only names and arities matter so template.Parse
// accepts references to known helpers.
func ParseFuncs() template.FuncMap {
	return template.FuncMap{
		"tool":       func(string, ...any) (string, error) { return "", nil },
		"agent":      func(string, ...any) (string, error) { return "", nil },
		"skill":      func(string, ...any) (any, error) { return nil, nil },
		"prompt":     func(string, ...any) (string, error) { return "", nil },
		"exit_error": func(string) (string, error) { return "", nil },
		"coalesce":   coalesceFunc,
		"dict":       dictFunc,
		"list":       listFunc,
		"contains":   strings.Contains,
		"trim":       strings.TrimSpace,
		"lower":      strings.ToLower,
		"upper":      strings.ToUpper,
		"replace":    replaceFunc,
		"split":      splitFunc,
		"join":       joinFunc,
		"fromJSON":   fromJSONFunc,
	}
}

// RuntimeFuncs are the side-effecting helpers callers supply at
// execution time. Each maps a helper name to its live implementation.
type RuntimeFuncs map[string]any

// Render parses and executes a template body against the given data.
// runtime may be nil when no side-effecting helpers are needed (e.g.
// until: predicates that only use pure helpers and comparisons).
func Render(body string, data map[string]any, runtime RuntimeFuncs) (string, error) {
	funcs := ParseFuncs()
	maps.Copy(funcs, runtime)
	t, err := template.New("render").Funcs(funcs).Parse(body)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// IsTruthy follows Go's text/template truthiness plus the convention
// from D28: false / 0 / nil / "" / empty collection → false.
// Used by until: predicates and inline {{if}} checks.
func IsTruthy(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s != "" && s != "false" && s != "0" && s != "no"
}
