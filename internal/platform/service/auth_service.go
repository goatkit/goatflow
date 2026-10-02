// Package service provides business logic services for the application.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
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

// getConfiguredProviderOrder returns the auth provider names to try, in order:
// the AUTH_PROVIDERS environment variable (comma separated) when set, else the
// Auth::Providers setting from Config.yaml, else just "database".
func getConfiguredProviderOrder() []string {
	if env := os.Getenv("AUTH_PROVIDERS"); strings.TrimSpace(env) != "" {
		out := []string{}
		for _, name := range strings.Split(env, ",") {
			if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
				out = append(out, name)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
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

// ErrRefreshRejected is returned by Refresh for any refresh token that cannot be
// exchanged: malformed, wrongly signed, expired, an access token, or belonging to
// an account that no longer exists or is no longer valid.
var ErrRefreshRejected = errors.New("refresh token rejected")

// Login authenticates a user and returns the user, an access token and a refresh token.
func (s *AuthService) Login(ctx context.Context, username, password string) (*platformmodels.User, string, string, error) {
	user, err := s.authenticator.Authenticate(ctx, username, password)
	if err != nil {
		return nil, "", "", err
	}
	accessToken, refreshToken, err := s.issueTokens(user)
	if err != nil {
		return nil, "", "", err
	}
	return user, accessToken, refreshToken, nil
}

// Refresh exchanges a refresh token for a new access token and a new refresh
// token. The account is reloaded from the database (users for agents,
// customer_user for customers), so the new access token carries the current id,
// role and admin-group membership, and accounts that were renamed, deleted or
// set invalid (valid_id != 1) are rejected with ErrRefreshRejected. JWTs are not
// tracked server-side, so the presented refresh token stays valid until it expires.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*platformmodels.User, string, string, error) {
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, "", "", ErrRefreshRejected
	}
	if s.db == nil {
		return nil, "", "", fmt.Errorf("refresh: database unavailable")
	}

	user := &platformmodels.User{ID: claims.UserID}
	var email sql.NullString
	var row *sql.Row
	if claims.Kind == auth.AccountKindCustomer {
		row = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(
			`SELECT login, email, first_name, last_name, valid_id FROM customer_user WHERE id = ?`), claims.UserID)
		err = row.Scan(&user.Login, &email, &user.FirstName, &user.LastName, &user.ValidID)
		user.Email = email.String
		user.Role = "Customer"
	} else {
		row = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(
			`SELECT login, first_name, last_name, valid_id FROM users WHERE id = ?`), claims.UserID)
		err = row.Scan(&user.Login, &user.FirstName, &user.LastName, &user.ValidID)
		user.Email = user.Login // agents have no email column; login is the identity
		user.Role = "Agent"
		if err == nil {
			isAdmin, adminErr := s.checkAdminGroup(user.ID)
			if adminErr != nil {
				return nil, "", "", fmt.Errorf("refresh: %w", adminErr)
			}
			if isAdmin {
				user.Role = "Admin"
			}
		}
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, "", "", ErrRefreshRejected
	case err != nil:
		return nil, "", "", fmt.Errorf("refresh: load %s %d: %w", claims.Kind, claims.UserID, err)
	}
	if user.Login != claims.Subject || user.ValidID != 1 {
		return nil, "", "", ErrRefreshRejected
	}

	accessToken, newRefreshToken, err := s.issueTokens(user)
	if err != nil {
		return nil, "", "", err
	}
	return user, accessToken, newRefreshToken, nil
}

// issueTokens creates the access and refresh token pair for an authenticated
// user. Customers (Role "Customer") are customer_user rows: they never carry the
// admin flag, because their ids overlap with agent ids in group_user.
func (s *AuthService) issueTokens(user *platformmodels.User) (string, string, error) {
	kind, isAdmin := auth.AccountKindAgent, false
	if user.Role == "Customer" {
		kind = auth.AccountKindCustomer
	} else if user.ID != 0 {
		// ID 0 is not a users row (e.g. static provider accounts), so it can
		// never be in group_user; every real agent is looked up.
		admin, err := s.checkAdminGroup(user.ID)
		if err != nil {
			return "", "", err
		}
		isAdmin = admin
	}
	accessToken, err := s.jwtManager.GenerateTokenWithAdmin(user.ID, user.Email, user.Role, isAdmin, 0)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(kind, user.ID, user.Login)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return accessToken, refreshToken, nil
}

// checkAdminGroup reports admin-group membership. A failed lookup is an
// error: issuing a token with a guessed role would silently demote admins.
func (s *AuthService) checkAdminGroup(userID uint) (bool, error) {
	if s.db == nil {
		return false, errors.New("admin group lookup: database unavailable")
	}
	var isAdmin bool
	err := s.db.QueryRow(database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN `+"`groups`"+` g ON gu.group_id = g.id
			WHERE gu.user_id = ? AND g.name = 'admin'
		)
	`), userID).Scan(&isAdmin)
	if err != nil {
		return false, fmt.Errorf("admin group lookup for user %d: %w", userID, err)
	}
	return isAdmin, nil
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

// GetUser retrieves user information by identifier.
func (s *AuthService) GetUser(ctx context.Context, identifier string) (*platformmodels.User, error) {
	return s.authenticator.GetUser(ctx, identifier)
}
