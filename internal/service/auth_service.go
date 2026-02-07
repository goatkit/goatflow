// Package service provides business logic services for the application.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/yamlmgmt"
)

// AuthService handles authentication and authorization.
type AuthService struct {
	authenticator *auth.Authenticator
	jwtManager    *auth.JWTManager
	db            *sql.DB
}

// NewAuthService creates a new authentication service with a JWT manager.
func NewAuthService(db *sql.DB, jwtManager *auth.JWTManager) *AuthService {
	providers := []auth.AuthProvider{}
	order := getConfiguredProviderOrder()
	deps := auth.ProviderDependencies{DB: db}
	for _, name := range order {
		p, err := auth.CreateProvider(name, deps)
		if err != nil {
			// Skip missing/disabled providers quietly
			log.Printf("auth: provider '%s' skipped: %v", name, err)
			continue
		}
		providers = append(providers, p)
	}
	if len(providers) == 0 {
		// Always fall back to direct database provider
		p, err := auth.CreateProvider("database", deps)
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
	// Expect slice
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
func (s *AuthService) Login(ctx context.Context, username, password string) (*models.User, string, string, error) {
	// Authenticate user
	user, err := s.authenticator.Authenticate(ctx, username, password)
	if err != nil {
		return nil, "", "", err
	}

	// Check if user is in admin group (for JWT claim)
	isAdmin := s.checkAdminGroup(user.ID)

	// Generate tokens using the JWT manager
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

// checkAdminGroup checks if user is in admin group.
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
func (s *AuthService) ValidateToken(tokenString string) (*models.User, error) {
	// Validate token using JWT manager
	claims, err := s.jwtManager.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Create user object from token claims
	user := &models.User{
		ID:    claims.UserID,
		Login: claims.Email, // Use email as login for now
		Email: claims.Email,
		Role:  claims.Role,
	}

	return user, nil
}

// RefreshToken generates a new access token from a refresh token.
func (s *AuthService) RefreshToken(refreshToken string) (string, error) {
	// Validate refresh token using JWT manager
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	// Generate new access token
	// Note: We need to get the full user details, for now using basic info from claims
	return s.jwtManager.GenerateToken(0, claims.Subject, "User", 0) // TODO: Get actual user ID and role
}

// Token generation methods removed - now using JWTManager

// GetUser retrieves user information by identifier.
func (s *AuthService) GetUser(ctx context.Context, identifier string) (*models.User, error) {
	return s.authenticator.GetUser(ctx, identifier)
}
