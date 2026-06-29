package models

import (
	"database/sql"
	"time"
)

// APITokenUserType represents the type of user for API tokens
type APITokenUserType string

const (
	APITokenUserAgent    APITokenUserType = "agent"
	APITokenUserCustomer APITokenUserType = "customer"
)

// APIToken represents a personal access token for API authentication
type APIToken struct {
	ID            int64            `json:"id" db:"id"`
	UserID        int              `json:"user_id" db:"user_id"`
	UserType      APITokenUserType `json:"user_type" db:"user_type"`
	Name          string           `json:"name" db:"name"`
	Prefix        string           `json:"prefix" db:"prefix"`
	TokenHash     string           `json:"-" db:"token_hash"` // Never expose hash
	Scopes        []string         `json:"scopes,omitempty"`  // Parsed from JSON
	ScopesJSON    sql.NullString   `json:"-" db:"scopes"`     // Raw JSON from DB
	ExpiresAt     sql.NullTime     `json:"expires_at,omitempty" db:"expires_at"`
	LastUsedAt    sql.NullTime     `json:"last_used_at,omitempty" db:"last_used_at"`
	LastUsedIP    sql.NullString   `json:"last_used_ip,omitempty" db:"last_used_ip"`
	RateLimit     int              `json:"rate_limit" db:"rate_limit"`
	CreatedAt     time.Time        `json:"created_at" db:"created_at"`
	CreatedBy     sql.NullInt64    `json:"created_by,omitempty" db:"created_by"`
	RevokedAt     sql.NullTime     `json:"revoked_at,omitempty" db:"revoked_at"`
	RevokedBy     sql.NullInt64    `json:"revoked_by,omitempty" db:"revoked_by"`
	CustomerLogin string           `json:"customer_login,omitempty"` // For customer tokens: login from customer_user
}

// IsExpired returns true if the token has expired
func (t *APIToken) IsExpired() bool {
	if !t.ExpiresAt.Valid {
		return false // Never expires
	}
	return time.Now().After(t.ExpiresAt.Time)
}

// IsRevoked returns true if the token has been revoked
func (t *APIToken) IsRevoked() bool {
	return t.RevokedAt.Valid
}

// IsActive returns true if the token is valid for use
func (t *APIToken) IsActive() bool {
	return !t.IsRevoked() && !t.IsExpired()
}

// HasScope returns true if the token has the specified scope
// If scopes is nil/empty, token has all permissions (inherits from user)
func (t *APIToken) HasScope(scope string) bool {
	if len(t.Scopes) == 0 {
		return true // Full access
	}
	for _, s := range t.Scopes {
		if s == "*" || s == scope {
			return true
		}
		// Check wildcard scopes (e.g., "tickets:*" matches "tickets:read")
		if len(s) > 2 && s[len(s)-2:] == ":*" {
			prefix := s[:len(s)-1] // "tickets:"
			if len(scope) > len(prefix) && scope[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

// APITokenCreateRequest represents a request to create a new token
type APITokenCreateRequest struct {
	Name      string   `json:"name" binding:"required,min=1,max=100"`
	Scopes    []string `json:"scopes,omitempty"`
	ExpiresIn string   `json:"expires_in,omitempty"` // "30d", "90d", "1y", "never"
}

// APITokenCreateResponse includes the full token (shown only once)
type APITokenCreateResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	Token     string    `json:"token"` // Full token - shown only at creation
	Scopes    []string  `json:"scopes,omitempty"`
	ExpiresAt *string   `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Warning   string    `json:"warning"`
}

// APITokenListItem represents a token in list responses (no secret)
type APITokenListItem struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes,omitempty"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
	IsActive   bool     `json:"is_active"`
}

// ValidScopes defines the allowed scope values
var ValidScopes = map[string]string{
	"*":              "Full access (inherits all user permissions)",
	"tickets:read":   "View tickets",
	"tickets:write":  "Create and update tickets",
	"tickets:delete": "Delete tickets",
	"articles:read":  "Read ticket articles",
	"articles:write": "Add articles and replies",
	"users:read":     "View user information",
	"queues:read":    "View queue information",
	"admin:*":        "Admin operations (agents only)",
}

// TokenPrefix is the prefix for all API tokens
const TokenPrefix = "gf_"

// TokenRandomLength is the length of the random part of the token
const TokenRandomLength = 32

// TokenPrefixLength is the length of the identifier prefix (after gf_)
const TokenPrefixLength = 8

// DefaultRateLimit is the default rate limit for new tokens (requests per hour)
const DefaultRateLimit = 1000
