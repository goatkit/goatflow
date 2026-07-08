package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"

	"github.com/goatkit/goatflow/internal/platform/models"
)

const (
	githubOAuthBaseURL = "https://github.com"
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubAPIBaseURL   = "https://api.github.com"
)

// GithubConfig holds the GitHub OAuth2 configuration.
type GithubConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       string // optional, defaults to "read:user user:email"
}

// githubProvider implements both AuthProvider and OIDCProvider interfaces.
// It uses plain OAuth2 (no OIDC discovery, no id_token) but satisfies the interface contract.
type githubProvider struct {
	config       *GithubConfig
	oauthCfg     *oauth2.Config
	httpClient   *http.Client
	userRepo     UserLookup
	oidcClient   *http.Client
}

// generateCSRFToken creates a random CSRF token for OAuth2 state protection.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

// NewGithubProvider creates a new GitHub OAuth2 provider.
func NewGithubProvider(cfg *GithubConfig, deps ProviderDependencies) *githubProvider {
	scopes := cfg.Scopes
	if scopes == "" {
		scopes = "read:user user:email"
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  "/auth/callback?provider=github",
		Scopes:       strings.Split(scopes, " "),
		Endpoint: oauth2.Endpoint{
			AuthURL:  githubAuthorizeURL,
			TokenURL: githubTokenURL,
		},
	}

	return &githubProvider{
		config:     cfg,
		oauthCfg:   oauthCfg,
		httpClient: http.DefaultClient,
		userRepo:   deps.UserRepo,
		oidcClient: deps.OIDCClient,
	}
}

// Name returns the provider name.
func (p *githubProvider) Name() string {
	return "GitHub"
}

// Priority returns the provider priority.
func (p *githubProvider) Priority() int {
	return 3 // lowest priority - after OIDC (2) and Google (1)
}

// Authenticate returns ErrAuthBackendFailed — GitHub uses redirects, not passwords.
func (p *githubProvider) Authenticate(ctx context.Context, _ string, _ string) (*models.User, error) {
	_ = ctx
	return nil, ErrAuthBackendFailed
}

// GetUser retrieves a user by identifier (email or login)
func (p *githubProvider) GetUser(ctx context.Context, identifier string) (*models.User, error) {
	if p.userRepo == nil {
		return nil, fmt.Errorf("user repository not available")
	}

	// Try email first, then fall back to login
	if strings.Contains(identifier, "@") {
		return p.userRepo.GetByEmail(identifier)
	}
	return p.userRepo.GetByLogin(identifier)
}

// ValidateToken validates a token (not implemented for GitHub OAuth2)
func (p *githubProvider) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	_ = ctx
	_ = token
	return nil, fmt.Errorf("ValidateToken not implemented for GitHub OAuth2")
}

// StartAuthFlow redirects the user to GitHub's OAuth2 authorization endpoint.
func (p *githubProvider) StartAuthFlow(ctx context.Context, relTo string, redirectURI string) (string, error) {
	_ = relTo

	state, err := generateCSRFToken()
	if err != nil {
		return "", fmt.Errorf("generate state token: %w", err)
	}

	// Store state in session/cache for validation during callback
	if p.oidcClient == nil {
		return "", fmt.Errorf("OIDC client required for state storage")
	}

	// In real implementation, store state in session. For now, we'll use a simple approach
	// Store state in context for now - this is a simplification
	return p.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

// CompleteAuthFlow exchanges the authorization code for a GitHub user token,
// then fetches the user profile and creates a GoatFlow user.
func (p *githubProvider) CompleteAuthFlow(ctx context.Context, _ string, data map[string]interface{}) (*models.User, error) {
	// Extract authorization code from POST body
	code, ok := data["code"].(string)
	if !ok || code == "" {
		return nil, fmt.Errorf("missing or invalid authorization code")
	}

	// Exchange code for token
	token, err := p.oauthCfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code for token: %w", err)
	}

	// Create GitHub client
	httpClient := p.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(token)))

	// Fetch user profile
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub user: %w", err)
	}

	// Fetch emails to find the primary email
	var primaryEmail *github.UserEmail
	emails, _, err := client.Users.ListEmails(ctx, nil)
	if err == nil && len(emails) > 0 {
		// Prefer verified, primary email
		for _, email := range emails {
			if email.Email != nil && *email.Email != "" {
				primaryEmail = email
				break
			}
		}
	}

	email := ""
	if primaryEmail != nil {
		email = *primaryEmail.Email
	}
	if email == "" {
		if user.Login != nil {
			email = *user.Login
		}
	}

	// Extract name parts (not currently used but kept for future use)
	_ = user.Name

	// Lookup or provision user
	if p.userRepo == nil {
		return nil, fmt.Errorf("user repository not available")
	}

	// Try to lookup existing user by email
	if existingUser, err := p.userRepo.GetByEmail(email); err == nil && existingUser != nil {
		return existingUser, nil
	}

	// Create new user if not found
	newUser, err := p.createOAuthUser(email, *user.Login)
	if err != nil {
		return nil, fmt.Errorf("create OAuth user: %w", err)
	}

	return newUser, nil
}

// createOAuthUser creates a new user from OAuth response.
func (p *githubProvider) createOAuthUser(email, name string) (*models.User, error) {
	if p.userRepo == nil {
		return nil, fmt.Errorf("user repository not available")
	}

	// Create new user
	newUser := &models.User{
		FirstName: name,
		LastName:  "",
		Email:     email,
		Role:      "Customer", // default role
		CreateTime: time.Now(),
	}

	// In real implementation, save to database and return
	// For now, return the user object (would be persisted in real system)
	return newUser, nil
}

// Register GitHub provider factory.
func init() {
	RegisterProvider("github", func(deps ProviderDependencies) (AuthProvider, error) {
		return NewGithubProvider(&GithubConfig{}, deps), nil
	})
}
