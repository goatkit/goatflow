package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/goatkit/goatflow/internal/models"
)

// Common errors.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserDisabled       = errors.New("user account is disabled")
	ErrAuthBackendFailed  = errors.New("authentication backend failed")
)

// AuthProvider defines the interface for authentication providers.
type AuthProvider interface {
	// Authenticate attempts to authenticate a user with the given credentials
	// Returns the authenticated user and nil error on success
	Authenticate(ctx context.Context, username, password string) (*models.User, error)

	// GetUser retrieves user details by username/email
	GetUser(ctx context.Context, identifier string) (*models.User, error)

	// ValidateToken validates an existing session/token
	ValidateToken(ctx context.Context, token string) (*models.User, error)

	// Name returns the name of this auth provider
	Name() string

	// Priority returns the priority of this provider (lower = higher priority)
	Priority() int
}

// Authenticator manages multiple authentication providers.
type Authenticator struct {
	providers []AuthProvider
	primary   AuthProvider
}

// NewAuthenticator creates a new authenticator with the given providers.
func NewAuthenticator(providers ...AuthProvider) *Authenticator {
	auth := &Authenticator{
		providers: providers,
	}

	// Set the primary provider (lowest priority number)
	if len(providers) > 0 {
		auth.primary = providers[0]
		for _, p := range providers {
			if p.Priority() < auth.primary.Priority() {
				auth.primary = p
			}
		}
	}

	return auth
}

// Authenticate attempts to authenticate using all configured providers.
func (a *Authenticator) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
	if len(a.providers) == 0 {
		return nil, ErrAuthBackendFailed
	}

	var lastErr error

	// Try each provider in priority order
	for _, provider := range a.providers {
		user, err := provider.Authenticate(ctx, username, password)
		if err == nil && user != nil {
			// Authentication successful
			return user, nil
		}

		// Keep track of the last error
		if err != nil && err != ErrUserNotFound {
			lastErr = err
		}
	}

	// No provider could authenticate the user
	if lastErr != nil {
		return nil, lastErr
	}

	return nil, ErrInvalidCredentials
}

// GetUser retrieves user information from the primary provider.
func (a *Authenticator) GetUser(ctx context.Context, identifier string) (*models.User, error) {
	if a.primary == nil {
		return nil, ErrAuthBackendFailed
	}

	return a.primary.GetUser(ctx, identifier)
}

// ValidateToken validates a token using the primary provider.
func (a *Authenticator) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	if a.primary == nil {
		return nil, ErrAuthBackendFailed
	}

	return a.primary.ValidateToken(ctx, token)
}

// AddProvider adds a new authentication provider.
func (a *Authenticator) AddProvider(provider AuthProvider) {
	a.providers = append(a.providers, provider)

	// Update primary provider if needed
	if a.primary == nil || provider.Priority() < a.primary.Priority() {
		a.primary = provider
	}
}

// GetProviders returns the list of configured providers.
func (a *Authenticator) GetProviders() []string {
	names := make([]string, len(a.providers))
	for i, p := range a.providers {
		names[i] = fmt.Sprintf("%s (priority: %d)", p.Name(), p.Priority())
	}
	return names
}
