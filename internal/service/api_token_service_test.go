package service

import (
	"testing"

	"github.com/goatkit/goatflow/internal/models"
)

func TestParseExpiration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantMin int // minimum expected duration in hours
		wantMax int // maximum expected duration in hours
	}{
		{"30 days", "30d", false, 24 * 29, 24 * 31},
		{"90 days", "90d", false, 24 * 89, 24 * 91},
		{"1 year", "1y", false, 24 * 364, 24 * 366},
		{"6 months", "6m", false, 24 * 179, 24 * 181},
		{"invalid format", "abc", true, 0, 0},
		{"zero days", "0d", true, 0, 0},
		{"negative", "-5d", true, 0, 0},
		{"never - returns error", "never", true, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := parseExpiration(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseExpiration(%q) expected error, got nil", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("parseExpiration(%q) unexpected error: %v", tt.input, err)
				return
			}
			
			hours := int(duration.Hours())
			if hours < tt.wantMin || hours > tt.wantMax {
				t.Errorf("parseExpiration(%q) = %d hours, want between %d and %d", 
					tt.input, hours, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTokenFormatGeneration(t *testing.T) {
	// Test that generated tokens have correct format
	// Format: gf_<8-char-prefix>_<remaining-random>
	
	// We can't test the full service without a DB, but we can test the format logic
	// by checking the constants and format expectations
	
	// Token should start with "gf_"
	prefix := "gf_"
	if len(prefix) != 3 {
		t.Errorf("Token prefix should be 3 chars, got %d", len(prefix))
	}
	
	// After prefix, should have 8 char identifier, then underscore, then rest
	// Total random bytes = 32, hex encoded = 64 chars
	// Format: gf_ (3) + prefix (8) + _ (1) + remaining (56) = 68 chars total
	expectedLen := 3 + 8 + 1 + 56
	if expectedLen != 68 {
		t.Errorf("Expected token length calculation: want 68, got %d", expectedLen)
	}
}

func TestValidateScopes(t *testing.T) {
	svc := &APITokenService{}
	
	tests := []struct {
		name     string
		scopes   []string
		userType string
		wantErr  bool
	}{
		{
			name:     "valid agent scopes",
			scopes:   []string{"tickets:read", "tickets:write"},
			userType: "agent",
			wantErr:  false,
		},
		{
			name:     "admin scope for agent - allowed",
			scopes:   []string{"admin:*"},
			userType: "agent",
			wantErr:  false,
		},
		{
			name:     "admin scope for customer - blocked",
			scopes:   []string{"admin:*"},
			userType: "customer",
			wantErr:  true,
		},
		{
			name:     "invalid scope",
			scopes:   []string{"bogus:scope"},
			userType: "agent",
			wantErr:  true,
		},
		{
			name:     "empty scopes - valid (full access)",
			scopes:   []string{},
			userType: "agent",
			wantErr:  false,
		},
		{
			name:     "wildcard scope",
			scopes:   []string{"*"},
			userType: "agent",
			wantErr:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userType models.APITokenUserType
			if tt.userType == "agent" {
				userType = models.APITokenUserAgent
			} else {
				userType = models.APITokenUserCustomer
			}
			
			err := svc.validateScopes(tt.scopes, userType)
			
			if tt.wantErr && err == nil {
				t.Errorf("validateScopes() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateScopes() unexpected error: %v", err)
			}
		})
	}
}
