package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/dbconfig"
)

// MinJWTSecretLength is the shortest JWT signing secret accepted in production.
const MinJWTSecretLength = 32

type SecretValidator struct {
	config   *Config
	errors   []string
	warnings []string
}

func NewSecretValidator(cfg *Config) *SecretValidator {
	return &SecretValidator{
		config:   cfg,
		errors:   []string{},
		warnings: []string{},
	}
}

// Validate checks secrets. In production a missing, placeholder or weak
// secret is an error; elsewhere it is logged as a warning.
func (v *SecretValidator) Validate() error {
	isProduction := IsProductionEnv(v.config)

	v.validateJWTSecret(isProduction)
	v.validateDatabasePassword(isProduction)
	v.validateSessionSecret(isProduction)
	v.validateAPIKeys(isProduction)
	v.validateZincPassword(isProduction)

	if len(v.errors) > 0 {
		return fmt.Errorf("secret validation failed:\n%s", strings.Join(v.errors, "\n"))
	}

	if len(v.warnings) > 0 {
		log.Printf("Security warnings:\n%s", strings.Join(v.warnings, "\n"))
	}

	return nil
}

func (v *SecretValidator) validateJWTSecret(isProduction bool) {
	raw := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if raw == "" && v.config != nil {
		raw = strings.TrimSpace(v.config.Auth.JWT.Secret)
	}
	if raw != "" && isPlaceholderJWTSecret(raw) {
		v.addError("JWT_SECRET is a published placeholder value; set a random secret of at least 32 characters", isProduction)
		return
	}

	secret := JWTSecret(v.config)
	if secret == "" {
		v.addError("JWT_SECRET is not set (or set GOATFLOW_AUTH_JWT_SECRET / auth.jwt.secret)", isProduction)
		return
	}

	// In development/test, allow prefixed secrets
	if !isProduction && (strings.HasPrefix(secret, "dev-") || strings.HasPrefix(secret, "test-")) {
		return
	}

	if len(secret) < MinJWTSecretLength {
		v.addError(fmt.Sprintf("JWT_SECRET must be at least %d characters long", MinJWTSecretLength), isProduction)
	}
}

func (v *SecretValidator) validateDatabasePassword(isProduction bool) {
	password := dbconfig.Env("PASSWORD")
	if password == "" {
		v.addWarning("DB password is not set (DB_MYSQL_PASSWORD / DB_PGSQL_PASSWORD)")
		return
	}

	// Check for example value
	if password == "goatflow_password" {
		v.addError("DB password is using the default example value", isProduction)
		return
	}

	if len(password) < 12 {
		v.addWarning("DB password should be at least 12 characters long")
	}
}

func (v *SecretValidator) validateSessionSecret(isProduction bool) {
	secret := os.Getenv("SESSION_SECRET")

	if secret == "" {
		return
	}

	// Check for example value
	if secret == "your-session-secret-here" {
		v.addError("SESSION_SECRET is using the default example value", isProduction)
		return
	}

	// In development/test, allow prefixed secrets
	if !isProduction && (strings.HasPrefix(secret, "dev-") || strings.HasPrefix(secret, "test-")) {
		return
	}

	if len(secret) < 32 {
		v.addWarning("SESSION_SECRET should be at least 32 characters long")
	}
}

func (v *SecretValidator) validateAPIKeys(isProduction bool) {
	apiKeys := []string{
		"API_KEY_INTERNAL",
		"WEBHOOK_SECRET",
		"GITHUB_WEBHOOK_SECRET",
		"SLACK_SIGNING_SECRET",
	}

	for _, key := range apiKeys {
		value := os.Getenv(key)
		if value == "" {
			continue
		}

		// In development/test, allow prefixed secrets
		if !isProduction && (strings.HasPrefix(value, "dev-") || strings.HasPrefix(value, "test-")) {
			continue
		}

		if len(value) < 16 {
			v.addWarning(fmt.Sprintf("%s should be at least 16 characters long", key))
		}
	}
}

func (v *SecretValidator) validateZincPassword(isProduction bool) {
	password := os.Getenv("ZINC_PASSWORD")

	if password == "" {
		return
	}

	// Check for example value
	if password == "ChangeThisZincPassword123!" {
		v.addError("ZINC_PASSWORD is using the default example value", isProduction)
		return
	}

	if len(password) < 12 {
		v.addWarning("ZINC_PASSWORD should be at least 12 characters long")
	}
}

func (v *SecretValidator) addError(message string, isProduction bool) {
	if isProduction {
		v.errors = append(v.errors, "   "+message)
	} else {
		v.warnings = append(v.warnings, "   "+message)
	}
}

func (v *SecretValidator) addWarning(message string) {
	v.warnings = append(v.warnings, "   "+message)
}

// ValidateSecrets fails in production when a required secret is missing,
// a published placeholder, or too weak. Call it at startup before serving.
func ValidateSecrets(cfg *Config) error {
	validator := NewSecretValidator(cfg)
	return validator.Validate()
}
