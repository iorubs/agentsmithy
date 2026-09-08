package models

import (
	"encoding/json"
	"strings"

	"google.golang.org/genai"
)

// jsonSchemaFor extracts a tool's parameter schema as a JSON Schema object.
// Providers that speak JSON Schema on the wire (anthropic, bedrock) share it.
func jsonSchemaFor(fd *genai.FunctionDeclaration) map[string]any {
	var raw map[string]any
	if fd.ParametersJsonSchema != nil {
		raw = toMapStringAny(fd.ParametersJsonSchema)
	} else if fd.Parameters != nil {
		raw = toMapStringAny(fd.Parameters)
	}
	if raw == nil {
		return nil
	}
	normalizeSchemaTypes(raw)
	if raw["type"] == nil || raw["type"] == "" {
		raw["type"] = "object"
	}
	return raw
}

// normalizeSchemaTypes lowercases the "type" field throughout a schema
// tree. genai.Schema uses uppercase enums ("OBJECT", "STRING", etc.)
// but JSON Schema requires lowercase.
func normalizeSchemaTypes(schema map[string]any) {
	if t, ok := schema["type"].(string); ok {
		schema["type"] = strings.ToLower(t)
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for _, v := range props {
			if sub, ok := v.(map[string]any); ok {
				normalizeSchemaTypes(sub)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		normalizeSchemaTypes(items)
	}
}

func toMapStringAny(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	}
}
