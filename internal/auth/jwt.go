package auth

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	Login    string `json:"login,omitempty"`
	Role     string `json:"role"`
	IsAdmin  bool   `json:"is_admin,omitempty"` // User is in admin group (for nav display)
	TenantID uint   `json:"tenant_id,omitempty"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secretKey            []byte
	tokenDuration        time.Duration
	refreshTokenDuration time.Duration
}

func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	rejectInsecureJWTSecret(secretKey)
	return &JWTManager{
		secretKey:            []byte(secretKey),
		tokenDuration:        tokenDuration,
		refreshTokenDuration: 7 * 24 * time.Hour, // default 7 days
	}
}

// rejectInsecureJWTSecret logs a fatal error in production if the JWT secret
// looks like a development placeholder or is shorter than 32 characters.
// Skipped when TEST_DB_DRIVER or GO_TEST are set (test environment).
func rejectInsecureJWTSecret(secret string) {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	if env != "production" && env != "prod" {
		return
	}
	// Skip in test harness — tests may set APP_ENV=production for coverage
	// but use short throwaway secrets.
	if flag.Lookup("test.v") != nil {
		return
	}
	if len(secret) < 32 {
		log.Fatalf("FATAL: JWT_SECRET is too short (%d chars). Production requires at least 32 characters.", len(secret))
	}
	lower := strings.ToLower(secret)
	for _, bad := range []string{"dev-secret", "change-me", "placeholder", "example", "insecure", "default"} {
		if strings.Contains(lower, bad) {
			log.Fatalf("FATAL: JWT_SECRET contains %q — this is not safe for production. Generate a real secret.", bad)
		}
	}
}

// SetRefreshTokenDuration sets the refresh token TTL.
func (m *JWTManager) SetRefreshTokenDuration(d time.Duration) {
	if d > 0 {
		m.refreshTokenDuration = d
	}
}

func (m *JWTManager) GenerateToken(userID uint, email, role string, tenantID uint) (string, error) {
	return m.GenerateTokenWithLogin(userID, email, email, role, false, tenantID)
}

// GenerateTokenWithAdmin creates a JWT with explicit isAdmin flag.
func (m *JWTManager) GenerateTokenWithAdmin(userID uint, email, role string, isAdmin bool, tenantID uint) (string, error) {
	return m.GenerateTokenWithLogin(userID, email, email, role, isAdmin, tenantID)
}

// GenerateTokenWithLogin creates a JWT with explicit login and email values.
func (m *JWTManager) GenerateTokenWithLogin(userID uint, login, email, role string, isAdmin bool, tenantID uint) (string, error) {
	return m.GenerateTokenWithDuration(userID, login, email, role, isAdmin, tenantID, m.tokenDuration)
}

// GenerateTokenWithDuration creates a JWT with a specific duration.
// If duration is 0, the system default is used.
func (m *JWTManager) GenerateTokenWithDuration(userID uint, login, email, role string, isAdmin bool, tenantID uint, duration time.Duration) (string, error) {
	if duration <= 0 {
		duration = m.tokenDuration
	}
	claims := Claims{
		UserID:   userID,
		Email:    email,
		Login:    login,
		Role:     role,
		IsAdmin:  isAdmin,
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "goatflow",
			Subject:   login,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}

func (m *JWTManager) GenerateRefreshToken(userID uint, email string) (string, error) {
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshTokenDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    "goatflow",
		Subject:   email,
		ID:        fmt.Sprintf("%d", userID), // Store user ID in JWT ID field
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

func (m *JWTManager) ValidateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}

func (m *JWTManager) TokenDuration() time.Duration { return m.tokenDuration }
