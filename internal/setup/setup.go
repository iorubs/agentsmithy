// Package setup implements the config-authoring MCP engine.
// It exposes two tools (config_guide and config_section) and does
// not require an existing .agentsmithy.yaml file.
package setup

import (
	"context"
	"embed"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/iorubs/agentsmithy/internal/config"
	"github.com/iorubs/agentsmithy/internal/config/schema"
	v1 "github.com/iorubs/agentsmithy/internal/config/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed sections/*.md
var sectionsFS embed.FS

const (
	toolConfigGuide   = "config_guide"
	toolConfigSection = "config_section"
)

// validSections lists the section names accepted by config_section.
var validSections = []string{"project", "tools", "pipeline"}

// sectionTypes maps each config_section name to the schema type names
// included in the field reference appended to the section markdown.
var sectionTypes = map[string]map[string]bool{
	"project": {"Project": true, "Models": true, "ModelEntry": true, "ModelRef": true, "Provider": true},
	"tools":   {"Tools": true},
	"pipeline": {
		"Pipeline":         true,
		"Autonomous":       true,
		"Sequential":       true,
		"Parallel":         true,
		"Loop":             true,
		"Orchestrator":     true,
		"OrchestratorStep": true,
		"SubAgent":         true,
		"Memory":           true,
		"Skills":           true,
		"Guard":            true,
		"InheritField":     true,
		"BuiltinFunc":      true,
		"TemplateString":   true,
	},
}

// Tool describes a setup tool's surface metadata.
type Tool struct {
	Description string
	Params      []ToolParam
}

// ToolParam declares one input the LLM provides when calling a tool.
type ToolParam struct {
	Name        string
	Required    bool
	Description string
}

// Engine is the config-authoring tool engine for agentsmithy.
type Engine struct {
	tools map[string]Tool
}

// New creates a setup Engine.
func New() *Engine {
	sectionParam := ToolParam{
		Name:        "section",
		Required:    true,
		Description: "Config section: " + strings.Join(validSections, ", "),
	}
	return &Engine{tools: map[string]Tool{
		toolConfigGuide: {
			Description: "Returns an overview of the .agentsmithy.yaml config structure: " +
				"top-level keys, how project/tools/pipeline relate, and how to choose " +
				"between the agent kinds. Call this first in any setup session.",
		},
		toolConfigSection: {
			Description: "Returns the full field reference for one config section. " +
				"Use after config_guide when writing or improving a specific section.",
			Params: []ToolParam{sectionParam},
		},
	}}
}

// Execute dispatches to the guide or section handler.
func (e *Engine) Execute(_ context.Context, name string, params map[string]any) (string, error) {
	switch name {
	case toolConfigGuide:
		return readSection("guide")
	case toolConfigSection:
		section, _ := params["section"].(string)
		section = strings.ToLower(strings.TrimSpace(section))
		if section == "" {
			return "", fmt.Errorf("missing required parameter: %q", "section")
		}
		if !slices.Contains(validSections, section) {
			return "", fmt.Errorf("unknown section: %q (valid: %s)",
				section, strings.Join(validSections, ", "))
		}
		body, err := readSection(section)
		if err != nil {
			return "", err
		}
		doc := schema.Describe(v1.Config{}, v1.Version, schema.ParseTypeDocs(config.TypesSources...))
		filtered := schema.FilterTypes(doc, sectionTypes[section])
		var b strings.Builder
		if err := fieldRefTmpl.Execute(&b, filtered); err == nil {
			if ref := b.String(); strings.TrimSpace(ref) != "" {
				body += "\n# Field Reference\n" + ref
			}
		}
		return body, nil
	default:
		return "", fmt.Errorf("unknown tool: %q", name)
	}
}

type guideInput struct{}

type sectionInput struct {
	Section string `json:"section" jsonschema:"the config section: project, tools, or pipeline"`
}

// textOutput wraps a markdown body so the SDK accepts it as the
// structured tool output (the SDK requires an object schema).
type textOutput struct {
	Text string `json:"text"`
}

// BuildServer constructs an *mcp.Server with the two setup tools
// registered. Each tool dispatches to Engine.Execute so the wire adapter stays thin.
func BuildServer() *mcp.Server {
	eng := New()
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "agentsmithy-setup",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(srv,
		&mcp.Tool{Name: toolConfigGuide, Description: eng.tools[toolConfigGuide].Description},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ guideInput) (*mcp.CallToolResult, textOutput, error) {
			return runTool(ctx, eng, toolConfigGuide, nil)
		})

	mcp.AddTool(srv,
		&mcp.Tool{Name: toolConfigSection, Description: eng.tools[toolConfigSection].Description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sectionInput) (*mcp.CallToolResult, textOutput, error) {
			return runTool(ctx, eng, toolConfigSection, map[string]any{"section": in.Section})
		})

	return srv
}

func runTool(ctx context.Context, eng *Engine, name string, params map[string]any) (*mcp.CallToolResult, textOutput, error) {
	text, err := eng.Execute(ctx, name, params)
	if err != nil {
		return nil, textOutput{}, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, textOutput{Text: text}, nil
}

func readSection(name string) (string, error) {
	data, err := sectionsFS.ReadFile("sections/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("setup: section %q not found", name)
	}
	return string(data), nil
}

// fieldRefTmpl produces a plain-text field reference from a SchemaDoc.
var fieldRefTmpl = template.Must(template.New("fieldref").Parse(`
{{- range .Structs}}

## {{.Name}}
{{- if .Doc}}
{{.Doc}}
{{- end}}

Fields:
{{- range .Fields}}
- {{.YAMLName}} ({{.Type}}){{if eq .Required "yes"}} [required]{{end}}{{if and .Default (ne .Default "—")}} default={{.Default}}{{end}}{{if and .Description (ne .Description "—")}} — {{.Description}}{{end}}{{if .Min}} (min: {{.Min}}){{end}}{{if .OneOfGroups}} [mutually exclusive with {{range $i, $g := .OneOfGroups}}{{range $j, $p := $g.Peers}}{{if or $i $j}}, {{end}}{{$p}}{{end}}{{end}}]{{end}}
{{- end}}
{{- end}}
{{- range .Enums}}

## {{.Name}}
{{- if .Doc}}
{{.Doc}}
{{- end}}

Values:
{{- range .Values}}
- {{.Label}}{{if and .Doc (ne .Doc "—")}} — {{.Doc}}{{end}}
{{- end}}
{{- end}}
{{- range .Types}}

## {{.Name}}
{{- if .Doc}}
{{.Doc}}
{{- end}}
{{- end}}
`))
