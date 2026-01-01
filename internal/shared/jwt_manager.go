package shared

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/config"
)

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

		systemMax := GetSystemSessionMaxTime()
		systemIdle := GetSystemSessionIdleTime()
		tokenDuration := time.Duration(systemMax) * time.Second
		if tokenDuration <= 0 {
			tokenDuration = 15 * time.Minute
		}

		if cfg != nil && cfg.Auth.JWT.AccessTokenTTL > 0 {
			tokenDuration = cfg.Auth.JWT.AccessTokenTTL
		}

		if systemMax > 0 {
			maxDuration := time.Duration(systemMax) * time.Second
			if tokenDuration <= 0 || maxDuration < tokenDuration {
				tokenDuration = maxDuration
			}
		}

		if systemIdle > 0 {
			idleDuration := time.Duration(systemIdle) * time.Second
			if idleDuration < tokenDuration || tokenDuration <= 0 {
				tokenDuration = idleDuration
			}
		}

		if tokenDuration <= 0 {
			tokenDuration = 15 * time.Minute
		}

		globalJWTManager = auth.NewJWTManager(jwtSecret, tokenDuration)
	})

	return globalJWTManager
}
