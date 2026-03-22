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

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/config"
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

// This ensures auth service and middleware use the same JWT configuration.
func GetJWTManager() *auth.JWTManager {
	jwtOnce.Do(func() {
		cfg := config.Get()
		env := strings.ToLower(os.Getenv("APP_ENV"))
		if cfg != nil && cfg.App.Env != "" {
			env = strings.ToLower(cfg.App.Env)
		}

		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" && cfg != nil {
			jwtSecret = cfg.Auth.JWT.Secret
		}
		if jwtSecret == "" && env != "production" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err == nil {
				jwtSecret = hex.EncodeToString(b)
			}
		}
		if len(jwtSecret) < 32 && env != "production" {
			pad := make([]byte, 16)
			rand.Read(pad)
			jwtSecret += hex.EncodeToString(pad)
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
				log.Printf("JWT access token TTL from env: %s", d)
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
				log.Printf("JWT refresh token TTL from env: %s", d)
			}
		} else if cfg != nil && cfg.Auth.JWT.RefreshTokenTTL > 0 {
			globalJWTManager.SetRefreshTokenDuration(cfg.Auth.JWT.RefreshTokenTTL)
		}
	})

	return globalJWTManager
}
