package kinds

import (
	"testing"

	"google.golang.org/genai"
)

func TestUserInputText(t *testing.T) {
	tests := []struct {
		name string
		uc   *genai.Content
		want string
	}{
		{"nil content", nil, ""},
		{"no parts", &genai.Content{}, ""},
		{
			name: "concatenates parts skipping nil and thoughts",
			uc: &genai.Content{Parts: []*genai.Part{
				{Text: "hello "},
				nil,
				{Text: "thinking", Thought: true},
				{Text: "world"},
			}},
			want: "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userInputText(tt.uc); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}
