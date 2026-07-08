package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/goatkit/goatflow/internal/platform/models"
)

// OidcConfig holds OIDC provider configuration.
type OidcConfig struct {
	DiscoveryURL  string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        string
	ClaimEmail    string
	ClaimName     string
	ClaimGroups   string
	AutoProvision bool
	UserTable     string
}

// oidcProvider implements both AuthProvider and OIDCProvider interfaces.
type oidcProvider struct {
	discoveryURL  string
	clientID      string
	clientSecret  string
	redirectURL   string
	scopes        string
	claimEmail    string
	claimName     string
	claimGroups   string
	autoProvision bool
	userTable     string
	db            *sql.DB
	provider      *oidc.Provider
	stateStore    StateStore
	oidcClient    *http.Client
	userRepo      UserLookup
	name          string
}

// NewOidcProvider creates a new OIDC provider.
func NewOidcProvider(cfg *OidcConfig, deps ProviderDependencies) *oidcProvider {
	if cfg == nil {
		cfg = &OidcConfig{}
	}
	scopes := cfg.Scopes
	if scopes == "" {
		scopes = "openid profile email"
	}
	discoveryURL := strings.TrimSuffix(cfg.DiscoveryURL, "/.well-known/openid-configuration")
	discoveryURL = strings.TrimSuffix(discoveryURL, "/")
	return &oidcProvider{
		discoveryURL:  discoveryURL,
		clientID:      cfg.ClientID,
		clientSecret:  cfg.ClientSecret,
		redirectURL:   cfg.RedirectURL,
		scopes:        scopes,
		claimEmail:    cfg.ClaimEmail,
		claimName:     cfg.ClaimName,
		claimGroups:   cfg.ClaimGroups,
		autoProvision: cfg.AutoProvision,
		userTable:     cfg.UserTable,
		db:            deps.DB,
		stateStore:    deps.StateStore,
		oidcClient:    deps.OIDCClient,
		userRepo:      deps.UserRepo,
		name:          "oidc",
	}
}

// splitScopes splits a space-separated scopes string.
func splitScopes(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, " ")
}

// Name returns the provider name.
func (p *oidcProvider) Name() string {
	return p.name
}

// Priority returns the provider priority (lower = higher priority).
func (p *oidcProvider) Priority() int {
	return 3
}

// Authenticate returns an error — OIDC uses redirects, not passwords.
func (p *oidcProvider) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
	_ = ctx
	_ = username
	_ = password
	return nil, ErrAuthBackendFailed
}

// GetUser looks up a user by identifier (email or login).
func (p *oidcProvider) GetUser(ctx context.Context, identifier string) (*models.User, error) {
	return p.userRepo.GetByLogin(identifier)
}

// ValidateToken delegates to the JWT manager for token validation.
func (p *oidcProvider) ValidateToken(_ context.Context, token string) (*models.User, error) {
	_ = token
	return nil, fmt.Errorf("ValidateToken not implemented for OIDC")
}

// StartAuthFlow generates an authorization URL using the provided state and PKCE code verifier.
func (p *oidcProvider) StartAuthFlow(ctx context.Context, state, codeVerifier string) (string, error) {
	if p.discoveryURL == "" {
		return "", fmt.Errorf("discovery URL required")
	}

	provider, err := oidc.NewProvider(ctx, p.discoveryURL)
	if err != nil {
		return "", fmt.Errorf("discover OIDC provider: %w", err)
	}
	p.provider = provider

	oauthCfg := oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Scopes:       splitScopes(p.scopes),
		RedirectURL:  p.redirectURL,
		Endpoint:     provider.Endpoint(),
	}

	return oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(codeVerifier)), nil
}

// CompleteAuthFlow exchanges an authorization code for a user.
func (p *oidcProvider) CompleteAuthFlow(ctx context.Context, code, state string) (*models.User, error) {
	_, _, _, codeVerifier, ok := p.stateStore.ConsumeState(state)
	if !ok {
		return nil, fmt.Errorf("invalid or expired state token")
	}

	if p.provider == nil {
		var err error
		p.provider, err = oidc.NewProvider(ctx, p.discoveryURL)
		if err != nil {
			return nil, fmt.Errorf("discover OIDC provider: %w", err)
		}
	}

	config := oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Scopes:       splitScopes(p.scopes),
		RedirectURL:  p.redirectURL,
		Endpoint:     p.provider.Endpoint(),
	}

	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("exchange code for token: %w", err)
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("no ID token received")
	}

	rawIDToken, err := p.provider.Verifier(&oidc.Config{
		ClientID: p.clientID,
	}).Verify(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("verify ID token: %w", err)
	}

	emailClaimKey := p.claimEmail
	if emailClaimKey == "" {
		emailClaimKey = "email"
	}
	nameClaimKey := p.claimName
	if nameClaimKey == "" {
		nameClaimKey = "name"
	}
	groupsClaimKey := p.claimGroups
	if groupsClaimKey == "" {
		groupsClaimKey = "groups"
	}

	claimsMap := map[string]interface{}{}
	if err := rawIDToken.Claims(&claimsMap); err != nil {
		return nil, fmt.Errorf("parse ID token claims: %w", err)
	}

	email, _ := claimsMap[emailClaimKey].(string)
	if email == "" {
		return nil, fmt.Errorf("no %s claim in ID token", emailClaimKey)
	}

	givenName := ""
	if gn, ok := claimsMap["given_name"].(string); ok && gn != "" {
		givenName = gn
	} else if fn, _ := claimsMap[nameClaimKey].(string); fn != "" {
		if idx := strings.Index(fn, " "); idx > 0 {
			givenName = fn[:idx]
		} else {
			givenName = fn
		}
	}

	familyName := ""
	if fn, ok := claimsMap["family_name"].(string); ok && fn != "" {
		familyName = fn
	} else if givenName == "" {
		if fn, _ := claimsMap[nameClaimKey].(string); fn != "" {
			if idx := strings.Index(fn, " "); idx > 0 {
				familyName = fn[idx+1:]
			}
		}
	}

	var groups []string
	if g, ok := claimsMap[groupsClaimKey].([]interface{}); ok {
		groups = make([]string, len(g))
		for i, v := range g {
			if s, ok := v.(string); ok {
				groups[i] = s
			}
		}
	}
	honorific := ""
	if h, ok := claimsMap["honorificPrefix"].(string); ok && h != "" {
		honorific = h
	} else if t, ok := claimsMap["title"].(string); ok && t != "" {
		honorific = t
	}

	return p.lookupOrProvisionUser(ctx, email, givenName, familyName, honorific, groups)
}

// lookupOrProvisionUser finds an existing user or creates one.
func (p *oidcProvider) lookupOrProvisionUser(_ context.Context, email, givenName, familyName, honorific string, groups []string) (*models.User, error) {
	var user *models.User
	var err error

	if p.userTable == "customer" {
		if !p.autoProvision {
			return nil, fmt.Errorf("user not found and auto-provision is disabled")
		}
		user, err = p.createOAuthUser(email, givenName, familyName)
		if err != nil {
			return nil, fmt.Errorf("provision customer: %w", err)
		}
	} else {
		existing, repoErr := p.userRepo.GetByLogin(email)
		if repoErr == nil && existing != nil {
			user = existing
		} else if !p.autoProvision {
			return nil, fmt.Errorf("user not found and auto-provision is disabled")
		} else {
			user, err = p.createOAuthUser(email, givenName, familyName)
			if err != nil {
				return nil, fmt.Errorf("provision agent: %w", err)
			}
		}
	}

	if user == nil {
		return nil, fmt.Errorf("failed to lookup or provision user")
	}

	// Update title (honorific prefix) if changed since last login
	if honorific != "" && user.Title != honorific {
		user.Title = honorific
		if p.db != nil {
			p.db.Exec("UPDATE users SET title = ?, change_time = NOW() WHERE id = ?", honorific, int(user.ID))
		}
	}

	if len(groups) > 0 && p.userRepo != nil {
		p.userRepo.SyncGroups(user.ID, groups)
	}

	return user, nil
}

// createOAuthUser persists a new OIDC user.
func (p *oidcProvider) createOAuthUser(email, givenName, familyName string) (*models.User, error) {
	switch p.userTable {
	case "customer":
		return p.provisionCustomer(email, givenName+" "+familyName, email)
	default:
		return p.provisionAgent(email, givenName, familyName)
	}
}

// provisionAgent creates a user in the users table with proper name mapping.
func (p *oidcProvider) provisionAgent(email, givenName, familyName string) (*models.User, error) {
	now := time.Now()
	user := &models.User{
		Login:      email,
		Title:      "",
		FirstName:  givenName,
		LastName:   familyName,
		Role:       "Agent",
		ValidID:    1,
		CreateTime: now,
		CreateBy:   1,
		ChangeTime: now,
		ChangeBy:   1,
	}
	if err := p.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("create OIDC user: %w", err)
	}
	user.Email = email

	groupName := "users"
	if p.db != nil {
		var gid int64
		err := p.db.QueryRow("SELECT id FROM groups WHERE name = ?", groupName).Scan(&gid)
		if err == nil && gid > 0 {
			p.db.Exec("INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (?, ?, 'rw', NOW(), 1, NOW(), 1)", int(user.ID), gid)
		}
	}

	return user, nil
}

// provisionCustomer creates a customer in the service_customer_user table.
func (p *oidcProvider) provisionCustomer(login, name, email string) (*models.User, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database unavailable for customer provisioning")
	}
	now := time.Now()
	query := `INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by) VALUES (?, 1, ?, ?)`
	result, err := p.db.Exec(query, login, now, 1)
	if err != nil {
		return nil, fmt.Errorf("create OIDC customer: %w", err)
	}
	customerID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get customer ID: %w", err)
	}
	return &models.User{
		ID:    uint(customerID),
		Login: login,
		Email: email,
		Title: name,
		Role:  "Customer",
	}, nil
}

// Register OIDC provider factory.
func init() {
	RegisterProvider("oidc", func(deps ProviderDependencies) (AuthProvider, error) {
		return NewOidcProvider(&OidcConfig{}, deps), nil
	})
}
