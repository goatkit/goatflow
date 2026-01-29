package convert

import "testing"

func TestToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		fallback int
		want     int
	}{
		{"int", 42, 0, 42},
		{"int8", int8(42), 0, 42},
		{"int16", int16(42), 0, 42},
		{"int32", int32(42), 0, 42},
		{"int64", int64(42), 0, 42},
		{"uint", uint(42), 0, 42},
		{"uint8", uint8(42), 0, 42},
		{"uint16", uint16(42), 0, 42},
		{"uint32", uint32(42), 0, 42},
		{"uint64", uint64(42), 0, 42},
		{"float32", float32(42.9), 0, 42},
		{"float64", float64(42.9), 0, 42},
		{"string valid", "42", 0, 42},
		{"string invalid", "abc", 99, 99},
		{"nil", nil, 99, 99},
		{"negative int", -5, 0, -5},
		{"negative int8", int8(-5), 0, -5},
		{"empty string", "", 99, 99},
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
	tests := []struct {
		name     string
		input    interface{}
		fallback uint
		want     uint
	}{
		{"int positive", 42, 0, 42},
		{"int negative", -5, 99, 99},
		{"int8 positive", int8(42), 0, 42},
		{"int8 negative", int8(-5), 99, 99},
		{"int16 positive", int16(42), 0, 42},
		{"int16 negative", int16(-5), 99, 99},
		{"int32 positive", int32(42), 0, 42},
		{"int32 negative", int32(-5), 99, 99},
		{"int64 positive", int64(42), 0, 42},
		{"int64 negative", int64(-5), 99, 99},
		{"uint", uint(42), 0, 42},
		{"uint8", uint8(42), 0, 42},
		{"uint16", uint16(42), 0, 42},
		{"uint32", uint32(42), 0, 42},
		{"uint64", uint64(42), 0, 42},
		{"float32 positive", float32(42.9), 0, 42},
		{"float32 negative", float32(-5.0), 99, 99},
		{"float64 positive", float64(42.9), 0, 42},
		{"float64 negative", float64(-5.0), 99, 99},
		{"string valid", "42", 0, 42},
		{"string invalid", "abc", 99, 99},
		{"nil", nil, 99, 99},
		{"empty string", "", 99, 99},
		{"zero", 0, 0, 0},
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
	tests := []struct {
		name     string
		input    interface{}
		fallback string
		want     string
	}{
		{"string", "hello", "", "hello"},
		{"int", 42, "", "42"},
		{"int64", int64(42), "", "42"},
		{"uint", uint(42), "", "42"},
		{"uint64", uint64(42), "", "42"},
		{"float64", float64(42.5), "", "42.5"},
		{"bool true", true, "", "true"},
		{"bool false", false, "", "false"},
		{"nil", nil, "fallback", "fallback"},
		{"negative int", -5, "", "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToString(tt.input, tt.fallback); got != tt.want {
				t.Errorf("ToString(%v, %q) = %q, want %q", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}
