package skills

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/google/jsonschema-go/jsonschema"
	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/iorubs/agentsmithy/internal/pipeline/skills/sandbox"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// shellResult is the wire shape returned to the LLM and to template
// callers when a shell skill runs.
type shellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

func buildShell(projectRoot, name string, sk v1.ShellSkill) (adktool.Tool, Helper, error) {
	wd := sk.WorkingDir
	if !filepath.IsAbs(wd) {
		wd = filepath.Join(projectRoot, wd)
	}
	sb, err := sandbox.New(wd)
	if err != nil {
		return nil, nil, fmt.Errorf("workingDir %q: %w", sk.WorkingDir, err)
	}

	templates := make([]*template.Template, len(sk.Command))
	for i, elem := range sk.Command {
		if !strings.Contains(elem, "{{") {
			templates[i] = nil
			continue
		}
		t, err := template.New(fmt.Sprintf("%s.cmd[%d]", name, i)).Parse(elem)
		if err != nil {
			return nil, nil, fmt.Errorf("command[%d] template: %w", i, err)
		}
		templates[i] = t
	}

	run := func(ctx context.Context, named map[string]any) (string, error) {
		filled, err := fillDefaults(sk.Args, named)
		if err != nil {
			return "", err
		}
		rendered := make([]string, len(sk.Command))
		for i, elem := range sk.Command {
			if templates[i] == nil {
				rendered[i] = elem
				continue
			}
			var buf bytes.Buffer
			if err := templates[i].Execute(&buf, filled); err != nil {
				return "", fmt.Errorf("rendering command[%d]: %w", i, err)
			}
			rendered[i] = buf.String()
		}
		cmd := exec.CommandContext(ctx, rendered[0], rendered[1:]...)
		cmd.Dir = sb.Root()
		cmd.Env = []string{}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		exitCode := 0
		if runErr != nil {
			if ee, ok := runErr.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				return "", fmt.Errorf("exec: %w", runErr)
			}
		}
		_ = exitCode
		// For helper return value; tool path returns a struct map.
		return stdout.String(), nil
	}

	helper := func(ctx context.Context, args ...any) (any, error) {
		named, err := positionalToNamed(sk.Args, args)
		if err != nil {
			return nil, err
		}
		return run(ctx, named)
	}

	schema := buildArgsSchema(sk.Args)
	tool, err := functiontool.New(functiontool.Config{
		Name:        name,
		Description: shellDescription(sk),
		InputSchema: schema,
	}, func(tc adktool.Context, input map[string]any) (shellResult, error) {
		filled, err := fillDefaults(sk.Args, input)
		if err != nil {
			return shellResult{}, err
		}
		rendered := make([]string, len(sk.Command))
		for i, elem := range sk.Command {
			if templates[i] == nil {
				rendered[i] = elem
				continue
			}
			var buf bytes.Buffer
			if err := templates[i].Execute(&buf, filled); err != nil {
				return shellResult{}, fmt.Errorf("rendering command[%d]: %w", i, err)
			}
			rendered[i] = buf.String()
		}
		cmd := exec.CommandContext(toolCtx(tc), rendered[0], rendered[1:]...)
		cmd.Dir = sb.Root()
		cmd.Env = []string{}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		res := shellResult{Stdout: stdout.String(), Stderr: stderr.String()}
		if runErr != nil {
			if ee, ok := runErr.(*exec.ExitError); ok {
				res.ExitCode = ee.ExitCode()
				return res, nil
			}
			return res, fmt.Errorf("exec: %w", runErr)
		}
		return res, nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("function tool: %w", err)
	}

	return tool, helper, nil
}

func shellDescription(sk v1.ShellSkill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run %q in %q.", strings.Join(sk.Command, " "), sk.WorkingDir)
	for _, a := range sk.Args {
		req := ""
		if a.Required {
			req = " (required)"
		}
		if a.Description != "" {
			fmt.Fprintf(&b, " %s%s: %s.", a.Name, req, a.Description)
		}
	}
	return b.String()
}

func buildArgsSchema(args []v1.Param) *jsonschema.Schema {
	s := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{},
	}
	for _, a := range args {
		s.Properties[a.Name] = paramSchema(a)
		if a.Required {
			s.Required = append(s.Required, a.Name)
		}
	}
	return s
}

func paramSchema(a v1.Param) *jsonschema.Schema {
	out := &jsonschema.Schema{Description: a.Description}
	switch a.Type {
	case v1.ParamTypeNumber:
		out.Type = "number"
	case v1.ParamTypeBool:
		out.Type = "boolean"
	case v1.ParamTypeArray:
		out.Type = "array"
	default:
		out.Type = "string"
	}
	if a.Constraints != nil {
		if len(a.Constraints.Enum) > 0 {
			out.Enum = a.Constraints.Enum
		}
		out.Minimum = a.Constraints.Min
		out.Maximum = a.Constraints.Max
	}
	return out
}

func positionalToNamed(params []v1.Param, args []any) (map[string]any, error) {
	if len(args) > len(params) {
		return nil, fmt.Errorf("too many arguments: got %d, expected at most %d", len(args), len(params))
	}
	out := make(map[string]any, len(params))
	for i, v := range args {
		out[params[i].Name] = v
	}
	return out, nil
}

func fillDefaults(params []v1.Param, named map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(params))
	for k, v := range named {
		out[k] = v
	}
	for _, p := range params {
		if _, ok := out[p.Name]; ok {
			if err := p.Validate(out[p.Name]); err != nil {
				return nil, fmt.Errorf("arg %q: %w", p.Name, err)
			}
			continue
		}
		if p.Required {
			return nil, fmt.Errorf("missing required arg %q", p.Name)
		}
		if p.Default != nil {
			out[p.Name] = p.Default
		} else {
			out[p.Name] = p.Zero()
		}
	}
	return out, nil
}

// toolCtx extracts the underlying context from an ADK tool.Context.
// ADK's tool.Context embeds context.Context.
func toolCtx(tc adktool.Context) context.Context {
	if c, ok := tc.(context.Context); ok {
		return c
	}
	return context.Background()
}
