package tools

import "testing"

func TestNormaliseMCPEndpoint(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http://localhost:8080/", "http://localhost:8080/"},
		{"https://example.com/mcp", "https://example.com/mcp"},
		{"localhost:8080", "http://localhost:8080/"},
		{":8080", "http://127.0.0.1:8080/"},
	}
	for _, c := range cases {
		if got := normaliseMCPEndpoint(c.in); got != c.want {
			t.Errorf("normaliseMCPEndpoint(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
