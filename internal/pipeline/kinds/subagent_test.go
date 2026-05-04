package kinds

import "testing"

func TestJoinArgs(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want string
	}{
		{"empty", nil, ""},
		{"single string", []any{"hello"}, "hello"},
		{"single non-string uses Sprint", []any{42}, "42"},
		{"multiple space-joined", []any{"a", "b", "c"}, "a b c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinArgs(tt.args); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}
