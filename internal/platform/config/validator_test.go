package config

import (
	"strings"
	"testing"
)

func prodConfig(secret string) *Config {
	c := &Config{}
	c.App.Env = "development" // shipped default; APP_ENV must override it
	c.Auth.JWT.Secret = secret
	return c
}

func TestValidateSecretsProductionJWT(t *testing.T) {
	strong := strings.Repeat("k", MinJWTSecretLength)
	cases := []struct {
		name      string
		appEnv    string
		envSecret string
		cfgSecret string
		wantErr   bool
	}{
		{"missing secret in production", "production", "", "", true},
		{"placeholder from default.yaml in production", "production", "", JWTPlaceholderSecret, true},
		{"example value via env in production", "prod", "CHANGE_THIS_SECRET_KEY_BEFORE_USE", "", true},
		{"short secret in production", "production", "short-secret", "", true},
		{"dev- prefix is not a pass in production", "production", "dev-abc", "", true},
		{"strong env secret in production", "production", strong, "", false},
		{"strong config secret in production", "PRODUCTION", "", strong, false},
		{"missing secret in development only warns", "development", "", "", false},
		{"placeholder in development only warns", "", "", JWTPlaceholderSecret, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.appEnv)
			t.Setenv("JWT_SECRET", tc.envSecret)
			t.Setenv("DB_DRIVER", "mysql")
			t.Setenv("TEST_DB_DRIVER", "")
			t.Setenv("SESSION_SECRET", "")
			t.Setenv("ZINC_PASSWORD", "")
			t.Setenv("DB_MYSQL_PASSWORD", "a-long-db-password")
			err := ValidateSecrets(prodConfig(tc.cfgSecret))
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateSecrets() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestAppEnvPrecedence(t *testing.T) {
	c := &Config{}
	c.App.Env = "development"

	t.Setenv("APP_ENV", "production")
	if got := AppEnv(c); got != EnvProduction {
		t.Fatalf("APP_ENV=production with app.env=development: AppEnv = %q, want production", got)
	}
	t.Setenv("APP_ENV", "prod")
	if !IsProductionEnv(c) {
		t.Fatal("APP_ENV=prod must count as production")
	}
	t.Setenv("APP_ENV", "")
	c.App.Env = "Production"
	if !IsProductionEnv(c) {
		t.Fatal("app.env=Production without APP_ENV must count as production")
	}
	c.App.Env = ""
	if got := AppEnv(c); got != EnvDevelopment {
		t.Fatalf("nothing set: AppEnv = %q, want development", got)
	}
}

func TestJWTSecretIgnoresPlaceholder(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if got := JWTSecret(prodConfig(JWTPlaceholderSecret)); got != "" {
		t.Fatalf("placeholder must resolve to empty, got %q", got)
	}
	t.Setenv("JWT_SECRET", "env-wins-over-config-0123456789abcdef")
	if got := JWTSecret(prodConfig("from-config")); got != "env-wins-over-config-0123456789abcdef" {
		t.Fatalf("JWT_SECRET must win, got %q", got)
	}
}

func TestCustomerFEOnlyParsing(t *testing.T) {
	for v, want := range map[string]bool{
		"1": true, "true": true, " TRUE ": true, "yes": true, "on": true,
		"": false, "0": false, "false": false, "no": false, "maybe": false,
	} {
		t.Setenv("CUSTOMER_FE_ONLY", v)
		if got := CustomerFEOnly(); got != want {
			t.Errorf("CUSTOMER_FE_ONLY=%q: got %v, want %v", v, got, want)
		}
	}
}
