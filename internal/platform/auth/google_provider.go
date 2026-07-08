package auth

import (
	"context"

	"github.com/goatkit/goatflow/internal/platform/models"
)

const googleDiscoveryURL = "https://accounts.google.com/.well-known/openid-configuration"

// GoogleConfig holds the configuration for a Google auth provider.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // optional, set by handler from request
	Scopes       string // optional, defaults to "openid email profile"
}

// googleProvider implements both AuthProvider and OIDCProvider interfaces.
// It embeds oidcProvider and overrides Name, Priority, and discovery URL.
type googleProvider struct {
	*oidcProvider // embed to inherit all OIDC functionality
}

// NewGoogleProvider creates a new Google auth provider.
func NewGoogleProvider(cfg *GoogleConfig, deps ProviderDependencies) *googleProvider {
	oidcCfg := &OidcConfig{
		DiscoveryURL: googleDiscoveryURL,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes, // defaults to "openid email profile" if empty
	}
	oidc := NewOidcProvider(oidcCfg, deps)
	oidc.name = "google"
	return &googleProvider{oidcProvider: oidc}
}

// Name returns the provider name.
func (p *googleProvider) Name() string {
	return "Google"
}

// Priority returns the provider priority (lower = higher priority).
func (p *googleProvider) Priority() int {
	return 2 // higher priority than generic OIDC (3)
}

// Authenticate returns an error — Google uses redirects, not passwords.
func (p *googleProvider) Authenticate(ctx context.Context, _ string, _ string) (*models.User, error) {
	_ = ctx
	return nil, ErrAuthBackendFailed
}

// Register Google provider factory.
func init() {
	RegisterProvider("google", func(deps ProviderDependencies) (AuthProvider, error) {
		return NewGoogleProvider(&GoogleConfig{}, deps), nil
	})
}
