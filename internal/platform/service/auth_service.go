// Package service provides business logic services for the application.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/auth"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/yamlmgmt"
)

// AuthService handles authentication and authorization.
type AuthService struct {
	authenticator *auth.Authenticator
	jwtManager    *auth.JWTManager
	db            *sql.DB
}

// NewAuthService creates a new authentication service with a JWT manager.
func NewAuthService(db *sql.DB, jwtManager *auth.JWTManager, oidcClient *http.Client, stateStore auth.StateStore) *AuthService {
	order := getConfiguredProviderOrder()
	providerDeps := auth.ProviderDependencies{
		DB:         db,
		OIDCClient: oidcClient,
		StateStore: stateStore,
	}
	providers := []auth.AuthProvider{}
	for _, name := range order {
		p, err := auth.CreateProvider(name, providerDeps)
		if err != nil {
			log.Printf("auth: provider '%s' skipped: %v", name, err)
			continue
		}
		providers = append(providers, p)
	}
	if len(providers) == 0 {
		p, err := auth.CreateProvider("database", providerDeps)
		if err == nil {
			providers = append(providers, p)
		}
	}
	authenticator := auth.NewAuthenticator(providers...)
	return &AuthService{authenticator: authenticator, jwtManager: jwtManager, db: db}
}

// global accessor injected from main to avoid import cycles.
var globalConfigAdapter *yamlmgmt.ConfigAdapter

func SetConfigAdapter(ca *yamlmgmt.ConfigAdapter) { globalConfigAdapter = ca }

func getConfiguredProviderOrder() []string {
	if globalConfigAdapter == nil {
		return []string{"database"}
	}
	v, err := globalConfigAdapter.GetConfigValue("Auth::Providers")
	if err != nil {
		return []string{"database"}
	}
	switch raw := v.(type) {
	case []interface{}:
		out := []string{}
		for _, r := range raw {
			if s, ok := r.(string); ok {
				out = append(out, strings.ToLower(s))
			}
		}
		if len(out) > 0 {
			return out
		}
	case []string:
		tmp := []string{}
		for _, s := range raw {
			tmp = append(tmp, strings.ToLower(s))
		}
		if len(tmp) > 0 {
			return tmp
		}
	}
	return []string{"database"}
}

// Login authenticates a user and returns JWT tokens.
func (s *AuthService) Login(ctx context.Context, username, password string) (*platformmodels.User, string, string, error) {
	user, err := s.authenticator.Authenticate(ctx, username, password)
	if err != nil {
		return nil, "", "", err
	}
	isAdmin := s.checkAdminGroup(user.ID)
	accessToken, err := s.jwtManager.GenerateTokenWithAdmin(user.ID, user.Email, user.Role, isAdmin, 0)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate access token: %w", err)
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return user, accessToken, refreshToken, nil
}

func (s *AuthService) checkAdminGroup(userID uint) bool {
	if s.db == nil {
		return false
	}
	var isAdmin bool
	err := s.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN `+"`groups`"+` g ON gu.group_id = g.id
			WHERE gu.user_id = ? AND g.name = 'admin'
		)
	`, userID).Scan(&isAdmin)
	if err != nil {
		return false
	}
	return isAdmin
}

// ValidateToken validates a JWT token and returns the user.
func (s *AuthService) ValidateToken(tokenString string) (*platformmodels.User, error) {
	claims, err := s.jwtManager.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	user := &platformmodels.User{
		ID:    claims.UserID,
		Login: claims.Email,
		Email: claims.Email,
		Role:  claims.Role,
	}
	return user, nil
}

// RefreshToken generates a new access token from a refresh token.
func (s *AuthService) RefreshToken(refreshToken string) (string, error) {
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}
	return s.jwtManager.GenerateToken(0, claims.Subject, "User", 0)
}

// GetUser retrieves user information by identifier.
func (s *AuthService) GetUser(ctx context.Context, identifier string) (*platformmodels.User, error) {
	return s.authenticator.GetUser(ctx, identifier)
}