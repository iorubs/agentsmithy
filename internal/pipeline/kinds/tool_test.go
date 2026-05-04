package kinds

import (
	"strings"
	"testing"
)

func TestArgsToMap(t *testing.T) {
	tests := []struct {
		name    string
		args    []any
		want    map[string]any
		wantErr string
	}{
		{
			name: "empty args yield empty map",
			args: nil,
			want: map[string]any{},
		},
		{
			name: "single map passes through",
			args: []any{map[string]any{"q": "x"}},
			want: map[string]any{"q": "x"},
		},
		{
			name: "single non-map lands under request",
			args: []any{"hello"},
			want: map[string]any{"request": "hello"},
		},
		{
			name:    "multiple args error with dict hint",
			args:    []any{"a", "b"},
			wantErr: "use dict",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := argsToMap(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v; want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v; want nil", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d; want %d (got %v)", len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("got[%q] = %v; want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestCoerceToolOutput(t *testing.T) {
	tests := []struct {
		name string
		out  map[string]any
		want string
	}{
		{"empty map yields empty string", map[string]any{}, ""},
		{"single result string unwraps", map[string]any{"result": "hello"}, "hello"},
		{
			name: "result non-string falls back to JSON",
			out:  map[string]any{"result": 42},
			want: `{"result":42}`,
		},
		{
			name: "multi-key map JSON-marshals",
			out:  map[string]any{"a": "x"},
			want: `{"a":"x"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coerceToolOutput(tt.out); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}
