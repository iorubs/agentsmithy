package tmpl

import (
	"fmt"
	"strings"
	"testing"
)

func TestRender_PureHelpers(t *testing.T) {
	tests := []struct {
		name string
		body string
		data map[string]any
		want string
	}{
		{
			name: "plain text",
			body: "hello world",
			want: "hello world",
		},
		{
			name: "input variable",
			body: "{{ .input }}",
			data: map[string]any{"input": "question"},
			want: "question",
		},
		{
			name: "child output",
			body: "{{ .researcher.output }}",
			data: map[string]any{"researcher": map[string]any{"output": "found it"}},
			want: "found it",
		},
		{
			name: "dict helper",
			body: `{{ $d := dict "a" "1" "b" "2" }}{{ $d.a }}-{{ $d.b }}`,
			want: "1-2",
		},
		{
			name: "list helper",
			body: `{{ $l := list "x" "y" }}{{ index $l 0 }}-{{ index $l 1 }}`,
			want: "x-y",
		},
		{
			name: "coalesce first non-empty",
			body: `{{ coalesce "" "" "third" "fourth" }}`,
			want: "third",
		},
		{
			name: "coalesce all empty",
			body: `{{ coalesce "" "" "" }}`,
			want: "",
		},
		{
			name: "coalesce skips nil and whitespace",
			body: `{{ coalesce .missing "   " "value" }}`,
			want: "value",
		},
		{
			name: "contains true",
			body: `{{ if contains .input "error" }}yes{{ else }}no{{ end }}`,
			data: map[string]any{"input": "found an error here"},
			want: "yes",
		},
		{
			name: "contains false",
			body: `{{ if contains .input "error" }}yes{{ else }}no{{ end }}`,
			data: map[string]any{"input": "all good"},
			want: "no",
		},
		{
			name: "builtin eq",
			body: `{{ if eq .input "done" }}yes{{ else }}no{{ end }}`,
			data: map[string]any{"input": "done"},
			want: "yes",
		},
		{
			name: "builtin and/or/not",
			body: `{{ if and (not (eq .input "")) (eq .input "go") }}match{{ else }}nope{{ end }}`,
			data: map[string]any{"input": "go"},
			want: "match",
		},
		{
			name: "trim",
			body: `{{ trim "  hi  " }}`,
			want: "hi",
		},
		{
			name: "lower/upper",
			body: `{{ lower "HI" }}-{{ upper "lo" }}`,
			want: "hi-LO",
		},
		{
			name: "replace",
			body: `{{ replace "a" "b" "banana" }}`,
			want: "bbnbnb",
		},
		{
			name: "split + index",
			body: `{{ index (split "," "a,b,c") 1 }}`,
			want: "b",
		},
		{
			name: "join",
			body: `{{ join "-" (list "a" "b" "c") }}`,
			want: "a-b-c",
		},
		{
			name: "join []string round-trip after split",
			body: `{{ split "," "a,b,c" | join "|" }}`,
			want: "a|b|c",
		},
		{
			name: "fromJSON map field",
			body: `{{ index (fromJSON .raw) "k" }}`,
			data: map[string]any{"raw": `{"k":"v"}`},
			want: "v",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.body, tt.data, nil)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestRender_RuntimeFuncs(t *testing.T) {
	rf := RuntimeFuncs{
		"tool": func(name string, args ...any) (string, error) {
			return "tool-result:" + name, nil
		},
		"prompt": func(text string, args ...any) (string, error) {
			return "llm-says:" + text, nil
		},
	}
	body := `{{ tool "search" "query" }} | {{ prompt "summarize this" }}`
	got, err := Render(body, nil, rf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "tool-result:search") || !strings.Contains(got, "llm-says:summarize") {
		t.Errorf("got %q; want tool and prompt results", got)
	}
}

func TestRender_ParseError(t *testing.T) {
	_, err := Render(`{{ .broken `, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "template parse") {
		t.Fatalf("err = %v; want parse error", err)
	}
}

func TestRender_ExecError(t *testing.T) {
	rf := RuntimeFuncs{
		"tool": func(name string, args ...any) (string, error) {
			return "", fmt.Errorf("boom")
		},
	}
	_, err := Render(`{{ tool "fail" }}`, nil, rf)
	if err == nil || !strings.Contains(err.Error(), "template exec") {
		t.Fatalf("err = %v; want exec error", err)
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"  ", false},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"No", false},
		{"true", true},
		{"1", true},
		{"yes", true},
		{"anything", true},
		{"done", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsTruthy(tt.input); got != tt.want {
				t.Errorf("IsTruthy(%q) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDictFunc_OddArgs(t *testing.T) {
	_, err := dictFunc("a", 1, "b")
	if err == nil || !strings.Contains(err.Error(), "even number") {
		t.Fatalf("err = %v; want even-number error", err)
	}
}

func TestDictFunc_NonStringKey(t *testing.T) {
	_, err := dictFunc(42, "val")
	if err == nil || !strings.Contains(err.Error(), "not a string") {
		t.Fatalf("err = %v; want non-string-key error", err)
	}
}

func TestFromJSONFunc_Error(t *testing.T) {
	_, err := fromJSONFunc(`{not json`)
	if err == nil || !strings.Contains(err.Error(), "fromJSON") {
		t.Fatalf("err = %v; want fromJSON error", err)
	}
}

func TestJoinFunc_BadType(t *testing.T) {
	_, err := joinFunc(",", 42)
	if err == nil || !strings.Contains(err.Error(), "join") {
		t.Fatalf("err = %v; want join type error", err)
	}
}
