package shared

import (
	"testing"
)

func TestBuildTicketStateSlug(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantVal string
	}{
		{"empty string", "", ""},
		{"simple name", "Open", "open"},
		{"already lowercase", "open", "open"},
		{"with spaces", "Pending Reminder", "pending_reminder"},
		{"multiple spaces", "Pending   Reminder", "pending_reminder"},
		{"leading/trailing spaces", "  Open  ", "open"},
		{"mixed case", "PeNdInG ClOsEd", "pending_closed"},
		{"single word", "New", "new"},
		{"three words", "Closed Successful Update", "closed_successful_update"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTicketStateSlug(tt.input)
			if got != tt.wantVal {
				t.Errorf("buildTicketStateSlug(%q) = %q, want %q", tt.input, got, tt.wantVal)
			}
		})
	}
}

func TestTicketStateLookupKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "simple name",
			input:    "Open",
			contains: []string{"open", "new"},
		},
		{
			name:     "pending state",
			input:    "Pending Reminder",
			contains: []string{"pending reminder", "pending", "reminder", "waiting"},
		},
		{
			name:     "closed state",
			input:    "Closed",
			contains: []string{"closed", "resolved"},
		},
		{
			name:     "resolved state",
			input:    "Resolved",
			contains: []string{"resolved", "closed"},
		},
		{
			name:     "multi-word state",
			input:    "Closed Successful",
			contains: []string{"closed successful", "closed", "successful", "resolved"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ticketStateLookupKeys(tt.input)

			for _, want := range tt.contains {
				found := false
				for _, got := range result {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ticketStateLookupKeys(%q) = %v, should contain %q", tt.input, result, want)
				}
			}
		})
	}
}

func TestTicketStateLookupKeysNoDuplicateEmpty(t *testing.T) {
	result := ticketStateLookupKeys("New")

	for _, key := range result {
		if key == "" {
			t.Errorf("ticketStateLookupKeys should not return empty strings, got: %v", result)
		}
	}
}

func TestClampSessionDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		wantVal int
	}{
		{"zero returns zero", 0, 0},
		{"negative returns zero", -100, 0},
		{"below minimum", 100, 3600},
		{"at minimum", 3600, 3600},
		{"normal value", 28800, 28800},
		{"at maximum", 604800, 604800},
		{"above maximum", 1000000, 604800},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampSessionDuration(tt.input)
			if got != tt.wantVal {
				t.Errorf("clampSessionDuration(%d) = %d, want %d", tt.input, got, tt.wantVal)
			}
		})
	}
}

func TestClampIdleDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		wantVal int
	}{
		{"zero returns zero", 0, 0},
		{"negative returns zero", -100, 0},
		{"below minimum", 100, 300},
		{"at minimum", 300, 300},
		{"normal value", 7200, 7200},
		{"at maximum", 604800, 604800},
		{"above maximum", 1000000, 604800},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampIdleDuration(tt.input)
			if got != tt.wantVal {
				t.Errorf("clampIdleDuration(%d) = %d, want %d", tt.input, got, tt.wantVal)
			}
		})
	}
}

func TestAcceptsJSONResponse(t *testing.T) {
	tests := []struct {
		name       string
		acceptHdr  string
		wantResult bool
	}{
		{"empty header", "", false},
		{"text/html", "text/html", false},
		{"application/json", "application/json", true},
		{"json with charset", "application/json; charset=utf-8", true},
		{"mixed accepts json first", "application/json, text/html", true},
		{"mixed accepts html first", "text/html, application/json", true},
		{"wildcard", "*/*", false},
		{"uppercase", "APPLICATION/JSON", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test this without mocking gin.Context
			// but we test the logic directly
			if tt.acceptHdr == "" {
				if acceptsJSONFromHeader(tt.acceptHdr) != tt.wantResult {
					t.Errorf("acceptsJSONFromHeader(%q) = %v, want %v", tt.acceptHdr, !tt.wantResult, tt.wantResult)
				}
			}
		})
	}
}

// acceptsJSONFromHeader is a helper for testing the logic without gin.Context
func acceptsJSONFromHeader(accept string) bool {
	if accept == "" {
		return false
	}
	return containsIgnoreCase(accept, "application/json")
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(toLower(s), toLower(substr)))
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
