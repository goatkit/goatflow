// Package auth provides the credentials the GoatFlow Go SDK sends with each
// request. GoatFlow accepts two kinds, both in the Authorization header as
// "Bearer <token>": API tokens (gf_..., created on the /settings/tokens page or
// via /api/v1/tokens) and JWT access tokens returned by POST /api/v1/auth/login
// and POST /api/v1/auth/refresh.
package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Authenticator supplies the Authorization header value for a request.
type Authenticator interface {
	// AuthorizationHeader returns the value for the Authorization header,
	// refreshing the credential first if it has expired.
	AuthorizationHeader(ctx context.Context) (string, error)
}

// APIKeyAuth authenticates with a GoatFlow API token (gf_...).
type APIKeyAuth struct {
	Token string
}

// NewAPIKeyAuth returns an authenticator for a GoatFlow API token.
func NewAPIKeyAuth(token string) *APIKeyAuth {
	return &APIKeyAuth{Token: token}
}

// AuthorizationHeader returns "Bearer <token>".
func (a *APIKeyAuth) AuthorizationHeader(context.Context) (string, error) {
	return "Bearer " + a.Token, nil
}

// RefreshFunc exchanges a refresh token for a new access token, a new
// (rotated) refresh token and the access token's expiry.
// client.AuthService.RefreshFunc returns one backed by POST /api/v1/auth/refresh.
type RefreshFunc func(ctx context.Context, refreshToken string) (token, newRefreshToken string, expiresAt time.Time, err error)

// JWTAuth authenticates with a JWT access token. When ExpiresAt is set and
// lies less than a minute ahead, the refresh function is called before the
// request. It is safe for concurrent use; concurrent requests share one
// refresh.
type JWTAuth struct {
	mu           sync.Mutex
	token        string
	refreshToken string
	expiresAt    time.Time
	refresh      RefreshFunc
}

// NewJWTAuth returns an authenticator for a JWT access token. A zero
// expiresAt means the token is used until the API rejects it; refresh may be
// nil, in which case an expired token is an error.
func NewJWTAuth(token, refreshToken string, expiresAt time.Time, refresh RefreshFunc) *JWTAuth {
	return &JWTAuth{token: token, refreshToken: refreshToken, expiresAt: expiresAt, refresh: refresh}
}

// AuthorizationHeader returns "Bearer <token>", refreshing it first if it is
// about to expire.
func (a *JWTAuth) AuthorizationHeader(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.expiresAt.IsZero() && time.Now().After(a.expiresAt.Add(-time.Minute)) {
		if a.refresh == nil {
			return "", fmt.Errorf("goatflow: access token expired at %s and no refresh function is configured", a.expiresAt.Format(time.RFC3339))
		}
		if a.refreshToken == "" {
			return "", fmt.Errorf("goatflow: access token expired at %s and no refresh token is available", a.expiresAt.Format(time.RFC3339))
		}
		token, refreshToken, expiresAt, err := a.refresh(ctx, a.refreshToken)
		if err != nil {
			return "", fmt.Errorf("goatflow: refreshing access token: %w", err)
		}
		a.token, a.refreshToken, a.expiresAt = token, refreshToken, expiresAt
	}
	return "Bearer " + a.token, nil
}
