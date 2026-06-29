package shared

import "testing"

// These tests verify the shared wrapper functions correctly delegate to the convert package.
// Exhaustive conversion tests are in internal/convert/convert_test.go.

func TestToInt(t *testing.T) {
	// Verify wrapper delegates correctly with representative cases
	tests := []struct {
		name     string
		input    interface{}
		fallback int
		want     int
	}{
		{"int", 42, 0, 42},
		{"uint", uint(42), 0, 42},
		{"string valid", "42", 0, 42},
		{"string invalid", "abc", 99, 99},
		{"nil", nil, 99, 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToInt(tt.input, tt.fallback); got != tt.want {
				t.Errorf("ToInt(%v, %d) = %v, want %v", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestToUint(t *testing.T) {
	// Verify wrapper delegates correctly with representative cases
	tests := []struct {
		name     string
		input    interface{}
		fallback uint
		want     uint
	}{
		{"int positive", 42, 0, 42},
		{"int negative", -5, 99, 99},
		{"uint", uint(42), 0, 42},
		{"string valid", "42", 0, 42},
		{"nil", nil, 99, 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToUint(tt.input, tt.fallback); got != tt.want {
				t.Errorf("ToUint(%v, %d) = %v, want %v", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestToString(t *testing.T) {
	// Verify wrapper delegates correctly with representative cases
	tests := []struct {
		name     string
		input    interface{}
		fallback string
		want     string
	}{
		{"string", "hello", "", "hello"},
		{"int", 42, "", "42"},
		{"nil", nil, "fallback", "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToString(tt.input, tt.fallback); got != tt.want {
				t.Errorf("ToString(%v, %q) = %q, want %q", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}
