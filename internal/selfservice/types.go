// Package selfservice implements self-service authentication:
// password recovery, customer registration, email verification, and CAPTCHA.
package selfservice

import (
	"time"
)

// Token types.
const (
	TokenPasswordReset       = "password_reset"
	TokenEmailVerify         = "email_verify"
	TokenRegistrationApprove = "registration_approve"
)

// User types.
const (
	UserAgent    = "agent"
	UserCustomer = "customer"
)

// Registration statuses.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// CAPTCHA providers.
const (
	CAPTCHANone      = ""
	CAPTCHARecaptcha = "recaptcha_v3"
	CAPTCHAHCaptcha  = "hcaptcha"
)

// AuthToken represents a row in gk_auth_token.
type AuthToken struct {
	ID            int64      `json:"id" db:"id"`
	Token         string     `json:"token" db:"token"`
	TokenType     string     `json:"token_type" db:"token_type"`
	UserType      string     `json:"user_type" db:"user_type"`
	UserID        *int       `json:"user_id,omitempty" db:"user_id"`
	CustomerLogin *string    `json:"customer_login,omitempty" db:"customer_login"`
	Email         string     `json:"email" db:"email"`
	ExpiresAt     time.Time  `json:"expires_at" db:"expires_at"`
	UsedAt        *time.Time `json:"used_at,omitempty" db:"used_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// IsExpired returns true if the token has expired.
func (t *AuthToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsUsed returns true if the token has been consumed.
func (t *AuthToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsValid returns true if the token is not expired and not used.
func (t *AuthToken) IsValid() bool {
	return !t.IsExpired() && !t.IsUsed()
}

// RegistrationRequest represents a row in gk_registration_request.
type RegistrationRequest struct {
	ID             int64      `json:"id" db:"id"`
	Email          string     `json:"email" db:"email"`
	FirstName      string     `json:"first_name" db:"first_name"`
	LastName       string     `json:"last_name" db:"last_name"`
	CustomerID     *string    `json:"customer_id,omitempty" db:"customer_id"`
	Status         string     `json:"status" db:"status"`
	ApprovalToken  *string    `json:"approval_token,omitempty" db:"approval_token"`
	ApprovedBy     *int       `json:"approved_by,omitempty" db:"approved_by"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty" db:"approved_at"`
	RejectedReason *string    `json:"rejected_reason,omitempty" db:"rejected_reason"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

// CAPTCHAConfig holds CAPTCHA provider configuration.
type CAPTCHAConfig struct {
	Provider  string  `json:"provider"`   // recaptcha_v3, hcaptcha, or empty (disabled)
	SiteKey   string  `json:"site_key"`   // public key for frontend
	SecretKey string  `json:"secret_key"` // server-side verification key
	Threshold float64 `json:"threshold"`  // minimum score for reCAPTCHA v3 (default: 0.5)
}

// DefaultTokenExpiry is the expiry duration for password reset tokens.
const DefaultTokenExpiry = 1 * time.Hour

// DefaultVerifyExpiry is the expiry duration for email verification tokens.
const DefaultVerifyExpiry = 24 * time.Hour
