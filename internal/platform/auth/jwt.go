package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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

// Token types carried in the "typ" claim. Access and refresh tokens are signed
// with the same key, so the claim is what keeps one from being used as the other.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Account kinds carried by refresh tokens: agents live in users, customers in
// customer_user, and their ids overlap.
const (
	AccountKindAgent    = "agent"
	AccountKindCustomer = "customer"
)

type Claims struct {
	UserID    uint   `json:"user_id"`
	Email     string `json:"email"`
	Login     string `json:"login,omitempty"`
	Role      string `json:"role"`
	IsAdmin   bool   `json:"is_admin,omitempty"` // User is in admin group (for nav display)
	TenantID  uint   `json:"tenant_id,omitempty"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// RefreshClaims are the claims of a refresh token. Subject is the account login.
type RefreshClaims struct {
	UserID    uint   `json:"user_id"`
	Kind      string `json:"kind"` // AccountKindAgent or AccountKindCustomer
	TokenType string `json:"typ"`
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
func rejectInsecureJWTSecret(secret string) {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	if env != "production" && env != "prod" {
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
		UserID:    userID,
		Email:     email,
		Login:     login,
		Role:      role,
		IsAdmin:   isAdmin,
		TenantID:  tenantID,
		TokenType: TokenTypeAccess,
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

// ValidateToken validates an access token. Refresh tokens are rejected.
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, m.keyFunc)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.TokenType != TokenTypeAccess {
		return nil, ErrInvalidToken
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}

// GenerateRefreshToken creates a refresh token for the account (kind, userID,
// login), valid for the refresh token TTL. Each token gets a random id (jti), so
// two tokens issued in the same second differ.
func (m *JWTManager) GenerateRefreshToken(kind string, userID uint, login string) (string, error) {
	if kind != AccountKindAgent && kind != AccountKindCustomer {
		return "", fmt.Errorf("unknown account kind %q", kind)
	}
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", fmt.Errorf("generate refresh token id: %w", err)
	}
	now := time.Now()
	claims := RefreshClaims{
		UserID:    userID,
		Kind:      kind,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "goatflow",
			Subject:   login,
			ID:        hex.EncodeToString(jti),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ValidateRefreshToken validates a refresh token. Access tokens are rejected.
func (m *JWTManager) ValidateRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, m.keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid || claims.TokenType != TokenTypeRefresh || claims.UserID == 0 || claims.Subject == "" ||
		(claims.Kind != AccountKindAgent && claims.Kind != AccountKindCustomer) || claims.ExpiresAt == nil {
		return nil, ErrInvalidToken
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}

func (m *JWTManager) keyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, ErrInvalidToken
	}
	return m.secretKey, nil
}

// RefreshTokenDuration returns the refresh token TTL.
func (m *JWTManager) RefreshTokenDuration() time.Duration { return m.refreshTokenDuration }

func (m *JWTManager) TokenDuration() time.Duration { return m.tokenDuration }
