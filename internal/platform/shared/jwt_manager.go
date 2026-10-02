package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/config"
)

// parseDuration extends time.ParseDuration with support for "d" (days) suffix.
// e.g. "7d" → 168h, "30d" → 720h, "4h" → 4h, "15m" → 15m.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

var (
	globalJWTManager *auth.JWTManager
	jwtOnce          sync.Once
)

// signingSecret returns the JWT signing key for cfg. In production it is the
// configured secret or an error (never a placeholder, never generated). Outside
// production an unset or short secret is completed with random bytes.
func signingSecret(cfg *config.Config) (string, error) {
	jwtSecret := config.JWTSecret(cfg)
	if config.IsProductionEnv(cfg) {
		if len(jwtSecret) < config.MinJWTSecretLength {
			return "", fmt.Errorf("JWT_SECRET must be set to a random value of at least %d characters in production", config.MinJWTSecretLength)
		}
		return jwtSecret, nil
	}
	if jwtSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		jwtSecret = hex.EncodeToString(b)
	}
	if len(jwtSecret) < config.MinJWTSecretLength {
		pad := make([]byte, 16)
		if _, err := rand.Read(pad); err != nil {
			return "", err
		}
		jwtSecret += hex.EncodeToString(pad)
	}
	return jwtSecret, nil
}

// This ensures auth service and middleware use the same JWT configuration.
func GetJWTManager() *auth.JWTManager {
	jwtOnce.Do(func() {
		cfg := config.Get()
		jwtSecret, err := signingSecret(cfg)
		if err != nil {
			// config.ValidateSecrets refuses to start the server first; this
			// guard keeps any other entry point from signing with a weak key.
			log.Fatalf("FATAL: %v", err)
		}

		// Determine token duration. Priority:
		// 1. JWT_ACCESS_TOKEN_EXPIRY env var (e.g. "4h", "30m", "24h")
		// 2. Config file auth.jwt.access_token_ttl
		// 3. System session max/idle time
		// 4. Default: 15 minutes
		tokenDuration := time.Duration(0)

		// Check JWT_ACCESS_TOKEN_EXPIRY env var first.
		if envTTL := os.Getenv("JWT_ACCESS_TOKEN_EXPIRY"); envTTL != "" {
			if d, err := parseDuration(envTTL); err == nil && d > 0 {
				tokenDuration = d
				log.Printf("JWT access token TTL from env: %s", d) // #nosec G706 -- d is a parsed time.Duration; its String() cannot carry newlines or control characters
			}
		}

		// Fall back to config file.
		if tokenDuration <= 0 && cfg != nil && cfg.Auth.JWT.AccessTokenTTL > 0 {
			tokenDuration = cfg.Auth.JWT.AccessTokenTTL
		}

		// Fall back to system session settings.
		if tokenDuration <= 0 {
			systemMax := GetSystemSessionMaxTime()
			if systemMax > 0 {
				tokenDuration = time.Duration(systemMax) * time.Second
			}
		}
		if tokenDuration <= 0 {
			systemIdle := GetSystemSessionIdleTime()
			if systemIdle > 0 {
				tokenDuration = time.Duration(systemIdle) * time.Second
			}
		}

		// Default.
		if tokenDuration <= 0 {
			tokenDuration = 15 * time.Minute
		}

		globalJWTManager = auth.NewJWTManager(jwtSecret, tokenDuration)

		// Check JWT_REFRESH_TOKEN_EXPIRY env var.
		if envRefresh := os.Getenv("JWT_REFRESH_TOKEN_EXPIRY"); envRefresh != "" {
			if d, err := parseDuration(envRefresh); err == nil && d > 0 {
				globalJWTManager.SetRefreshTokenDuration(d)
				log.Printf("JWT refresh token TTL from env: %s", d) // #nosec G706 -- d is a parsed time.Duration; its String() cannot carry newlines or control characters
			}
		} else if cfg != nil && cfg.Auth.JWT.RefreshTokenTTL > 0 {
			globalJWTManager.SetRefreshTokenDuration(cfg.Auth.JWT.RefreshTokenTTL)
		}
	})

	return globalJWTManager
}
