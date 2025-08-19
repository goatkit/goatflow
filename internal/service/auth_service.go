package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// AuthService handles authentication and authorization
type AuthService struct {
	authenticator *auth.Authenticator
	jwtSecret     []byte
}

// NewAuthService creates a new authentication service
func NewAuthService(db *sql.DB) *AuthService {
	// Get JWT secret from environment or use default for development
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "development-secret-change-in-production"
	}
	
	// Create authenticator with database provider by default
	authenticator := auth.NewAuthenticator(
		auth.NewDatabaseAuthProvider(db),
	)
	
	// Add LDAP provider if configured
	if os.Getenv("LDAP_ENABLED") == "true" {
		ldapConfig := &auth.LDAPConfig{
			Server:     os.Getenv("LDAP_SERVER"),
			Port:       389, // TODO: Parse from env
			BaseDN:     os.Getenv("LDAP_BASE_DN"),
			BindDN:     os.Getenv("LDAP_BIND_DN"),
			BindPass:   os.Getenv("LDAP_BIND_PASSWORD"),
			UserFilter: os.Getenv("LDAP_USER_FILTER"),
			TLS:        os.Getenv("LDAP_TLS") == "true",
		}
		authenticator.AddProvider(auth.NewLDAPAuthProvider(ldapConfig))
	}
	
	return &AuthService{
		authenticator: authenticator,
		jwtSecret:     []byte(jwtSecret),
	}
}

// Login authenticates a user and returns JWT tokens
func (s *AuthService) Login(ctx context.Context, username, password string) (*models.User, string, string, error) {
	// Authenticate user
	user, err := s.authenticator.Authenticate(ctx, username, password)
	if err != nil {
		return nil, "", "", err
	}
	
	// Generate tokens
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate access token: %w", err)
	}
	
	refreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	
	return user, accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token and returns the user
func (s *AuthService) ValidateToken(tokenString string) (*models.User, error) {
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	
	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	
	// Extract user information from claims
	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id in token")
	}
	
	username, ok := claims["username"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid username in token")
	}
	
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)
	
	// Create user object from token claims
	user := &models.User{
		ID:    uint(userID),
		Login: username,
		Email: email,
		Role:  role,
	}
	
	return user, nil
}

// RefreshToken generates a new access token from a refresh token
func (s *AuthService) RefreshToken(refreshToken string) (string, error) {
	// Validate refresh token
	user, err := s.ValidateToken(refreshToken)
	if err != nil {
		return "", err
	}
	
	// Generate new access token
	return s.generateAccessToken(user)
}

// generateAccessToken creates a JWT access token for the user
func (s *AuthService) generateAccessToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Login,
		"email":    user.Email,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 hour expiry
		"iat":      time.Now().Unix(),
		"type":     "access",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// generateRefreshToken creates a JWT refresh token for the user
func (s *AuthService) generateRefreshToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Login,
		"exp":      time.Now().Add(time.Hour * 24 * 30).Unix(), // 30 day expiry
		"iat":      time.Now().Unix(),
		"type":     "refresh",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// GetUser retrieves user information by identifier
func (s *AuthService) GetUser(ctx context.Context, identifier string) (*models.User, error) {
	return s.authenticator.GetUser(ctx, identifier)
}