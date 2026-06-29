package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/yamlmgmt"
)

// helper to build a minimal config adapter with Auth::Providers list.
func testConfigAdapter(t *testing.T, providers []string) *yamlmgmt.ConfigAdapter {
	t.Helper()
	dir := t.TempDir()
	vm := yamlmgmt.NewVersionManager(dir)

	// Create a Config.yaml file that the adapter can import
	providersList := `["` + strings.Join(providers, `", "`) + `"]`
	configYAML := `version: "1.0"
settings:
  - name: "Auth::Providers"
    default: ` + providersList + `
`
	configPath := filepath.Join(dir, "Config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ca := yamlmgmt.NewConfigAdapter(vm)
	if err := ca.ImportConfigYAML(configPath); err != nil {
		t.Fatalf("import config: %v", err)
	}
	return ca
}

// minimal jwt manager constructor (reuse existing shared constructor if available); here we just create one directly.
func testJWTManager(t *testing.T) *auth.JWTManager {
	t.Helper()
	return auth.NewJWTManager("test-secret", time.Hour)
}

func TestAuthService_UsesStaticProviderFirst(t *testing.T) {
	os.Setenv("GOATFLOW_STATIC_USERS", "alpha:pw:Agent")
	defer os.Unsetenv("GOATFLOW_STATIC_USERS")

	ca := testConfigAdapter(t, []string{"static", "database"})
	SetConfigAdapter(ca)
	svc := NewAuthService(nil, testJWTManager(t))

	// Should authenticate via static provider; db provider skipped (nil DB)
	user, _, _, err := svc.Login(context.Background(), "alpha", "pw")
	if err != nil {
		t.Fatalf("expected static auth success, got %v", err)
	}
	if user.Login != "alpha" {
		t.Fatalf("unexpected user login %s", user.Login)
	}
}

func TestAuthService_FallbackNoProviders(t *testing.T) {
	ca := testConfigAdapter(t, []string{"bogus1", "bogus2"})
	SetConfigAdapter(ca)
	svc := NewAuthService(nil, testJWTManager(t))
	_, _, _, err := svc.Login(context.Background(), "any", "x")
	if err == nil {
		t.Fatalf("expected failure with no providers")
	}
}
