package shared

import (
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/config"
)

// APP_ENV=production must win over the shipped app.env "development" and the
// published default.yaml placeholder must never become the signing key.
func TestSigningSecretProductionRefusesPlaceholder(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Env = "development"
	cfg.Auth.JWT.Secret = config.JWTPlaceholderSecret

	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")
	if s, err := signingSecret(cfg); err == nil {
		t.Fatalf("production with placeholder secret must fail, got secret %q", s)
	}

	t.Setenv("JWT_SECRET", "short")
	if _, err := signingSecret(cfg); err == nil {
		t.Fatal("production with a short secret must fail")
	}

	strong := strings.Repeat("s", config.MinJWTSecretLength)
	t.Setenv("JWT_SECRET", strong)
	s, err := signingSecret(cfg)
	if err != nil || s != strong {
		t.Fatalf("production with strong secret: got %q, %v", s, err)
	}
}

func TestSigningSecretDevelopmentNeverUsesPlaceholder(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.JWT.Secret = config.JWTPlaceholderSecret
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "")
	s, err := signingSecret(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, config.JWTPlaceholderSecret) || len(s) < config.MinJWTSecretLength {
		t.Fatalf("development secret must be random, got %q", s)
	}
}
