package models

import (
	"database/sql"
	"testing"
	"time"
)

func TestAPIToken_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt sql.NullTime
		want      bool
	}{
		{
			name:      "no expiration",
			expiresAt: sql.NullTime{Valid: false},
			want:      false,
		},
		{
			name:      "future expiration",
			expiresAt: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true},
			want:      false,
		},
		{
			name:      "past expiration",
			expiresAt: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &APIToken{ExpiresAt: tt.expiresAt}
			if got := token.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIToken_IsRevoked(t *testing.T) {
	tests := []struct {
		name      string
		revokedAt sql.NullTime
		want      bool
	}{
		{
			name:      "not revoked",
			revokedAt: sql.NullTime{Valid: false},
			want:      false,
		},
		{
			name:      "revoked",
			revokedAt: sql.NullTime{Time: time.Now(), Valid: true},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &APIToken{RevokedAt: tt.revokedAt}
			if got := token.IsRevoked(); got != tt.want {
				t.Errorf("IsRevoked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIToken_IsActive(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		expiresAt sql.NullTime
		revokedAt sql.NullTime
		want      bool
	}{
		{
			name:      "active - no expiry, not revoked",
			expiresAt: sql.NullTime{Valid: false},
			revokedAt: sql.NullTime{Valid: false},
			want:      true,
		},
		{
			name:      "active - future expiry, not revoked",
			expiresAt: sql.NullTime{Time: now.Add(time.Hour), Valid: true},
			revokedAt: sql.NullTime{Valid: false},
			want:      true,
		},
		{
			name:      "inactive - expired",
			expiresAt: sql.NullTime{Time: now.Add(-time.Hour), Valid: true},
			revokedAt: sql.NullTime{Valid: false},
			want:      false,
		},
		{
			name:      "inactive - revoked",
			expiresAt: sql.NullTime{Valid: false},
			revokedAt: sql.NullTime{Time: now, Valid: true},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &APIToken{ExpiresAt: tt.expiresAt, RevokedAt: tt.revokedAt}
			if got := token.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIToken_HasScope(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		scope  string
		want   bool
	}{
		{
			name:   "empty scopes = full access",
			scopes: nil,
			scope:  "tickets:read",
			want:   true,
		},
		{
			name:   "wildcard scope",
			scopes: []string{"*"},
			scope:  "tickets:read",
			want:   true,
		},
		{
			name:   "exact match",
			scopes: []string{"tickets:read", "tickets:write"},
			scope:  "tickets:read",
			want:   true,
		},
		{
			name:   "no match",
			scopes: []string{"tickets:read"},
			scope:  "tickets:write",
			want:   false,
		},
		{
			name:   "wildcard prefix match",
			scopes: []string{"tickets:*"},
			scope:  "tickets:read",
			want:   true,
		},
		{
			name:   "wildcard prefix no match",
			scopes: []string{"tickets:*"},
			scope:  "users:read",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &APIToken{Scopes: tt.scopes}
			if got := token.HasScope(tt.scope); got != tt.want {
				t.Errorf("HasScope(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestTokenFormat(t *testing.T) {
	// Verify token constants
	if TokenPrefix != "gf_" {
		t.Errorf("TokenPrefix = %q, want %q", TokenPrefix, "gf_")
	}
	if TokenPrefixLength != 8 {
		t.Errorf("TokenPrefixLength = %d, want %d", TokenPrefixLength, 8)
	}
	if TokenRandomLength != 32 {
		t.Errorf("TokenRandomLength = %d, want %d", TokenRandomLength, 32)
	}
	if DefaultRateLimit != 1000 {
		t.Errorf("DefaultRateLimit = %d, want %d", DefaultRateLimit, 1000)
	}
}
