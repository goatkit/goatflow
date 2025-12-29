package config

import (
	"fmt"
	"os"
	"strings"
)

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

func (v *SecretValidator) Validate() error {
	appEnv := os.Getenv("APP_ENV")
	isProduction := appEnv == "production" || appEnv == "prod"

	v.validateJWTSecret(isProduction)
	v.validateDatabasePassword(isProduction)
	v.validateSessionSecret(isProduction)
	v.validateAPIKeys(isProduction)
	v.validateZincPassword(isProduction)
	v.validateLDAPPassword(isProduction)

	if len(v.errors) > 0 {
		return fmt.Errorf("secret validation failed:\n%s", strings.Join(v.errors, "\n"))
	}

	if len(v.warnings) > 0 && isProduction {
		fmt.Printf("⚠️  Security warnings:\n%s\n", strings.Join(v.warnings, "\n"))
	}

	return nil
}

func (v *SecretValidator) validateJWTSecret(isProduction bool) {
	secret := os.Getenv("JWT_SECRET")

	if secret == "" {
		v.addError("JWT_SECRET is not set", isProduction)
		return
	}

	// Check if it matches the example value
	if secret == "CHANGE_THIS_SECRET_KEY_BEFORE_USE" {
		v.addError("JWT_SECRET is using the default example value", isProduction)
		return
	}

	// In development/test, allow prefixed secrets
	if !isProduction && (strings.HasPrefix(secret, "dev-") || strings.HasPrefix(secret, "test-")) {
		// These are fine for non-production
		return
	}

	if len(secret) < 32 {
		v.addError("JWT_SECRET must be at least 32 characters long", isProduction)
		return
	}
}

func (v *SecretValidator) validateDatabasePassword(isProduction bool) {
	password := os.Getenv("DB_PASSWORD")

	if password == "" {
		v.addWarning("DB_PASSWORD is not set")
		return
	}

	// Check for example value
	if password == "gotrs_password" {
		v.addError("DB_PASSWORD is using the default example value", isProduction)
		return
	}

	if len(password) < 12 {
		v.addWarning("DB_PASSWORD should be at least 12 characters long")
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

func (v *SecretValidator) validateLDAPPassword(isProduction bool) {
	passwords := []string{
		"LDAP_BIND_PASSWORD",
		"LDAP_ADMIN_PASSWORD",
	}

	for _, key := range passwords {
		value := os.Getenv(key)
		if value == "" {
			continue
		}

		// Todo add check for weak passwords here
	}
}

func (v *SecretValidator) addError(message string, isProduction bool) {
	if isProduction {
		v.errors = append(v.errors, "   ❌ "+message)
	} else {
		v.warnings = append(v.warnings, "   ⚠️  "+message)
	}
}

func (v *SecretValidator) addWarning(message string) {
	v.warnings = append(v.warnings, "   ⚠️  "+message)
}

func ValidateSecrets(cfg *Config) error {
	validator := NewSecretValidator(cfg)
	return validator.Validate()
}
