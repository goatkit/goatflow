package utils

import (
	"testing"
)

func TestFilterUnicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No Unicode",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "With emoji",
			input:    "Hello 🚀 World",
			expected: "Hello  World",
		},
		{
			name:     "Multiple emojis",
			input:    "Test 🎉 with 🌟 multiple 🚀 emojis 💻",
			expected: "Test  with  multiple  emojis",
		},
		{
			name:     "Accented characters preserved",
			input:    "Café résumé naïve",
			expected: "Café résumé naïve",
		},
		{
			name:     "Mixed content",
			input:    "Hello 🌟 café 🚀 world!",
			expected: "Hello  café  world!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterUnicode(tt.input)
			if result != tt.expected {
				t.Errorf("FilterUnicode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
