package main

import "testing"

func TestValidPluginName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"my-plugin", true},
		{"plugin_2", true},
		{"9lives", true},
		{"", false},
		{"..", false},
		{"../escape", false},
		{"a/b", false},
		{`a\b`, false},
		{"-leading", false},
		{".hidden", false},
		{"Upper", false},
	}
	for _, tc := range tests {
		if got := validPluginName(tc.name); got != tc.want {
			t.Errorf("validPluginName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
