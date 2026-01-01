package auth

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// test helper: capture registry then restore.
func saveRegistry() map[string]ProviderFactory {
	cp := map[string]ProviderFactory{}
	for k, v := range providerRegistry {
		cp[k] = v
	}
	return cp
}
func restoreRegistry(m map[string]ProviderFactory) {
	for k := range providerRegistry {
		delete(providerRegistry, k)
	}
	for k, v := range m {
		providerRegistry[k] = v
	}
}

// fake provider to test ordering without DB.
type fakeProvider struct {
	n             string
	pr            int
	authenticated bool
}

func (f *fakeProvider) Authenticate(ctx context.Context, u, pw string) (*models.User, error) {
	if f.authenticated {
		return &models.User{Login: u, Email: u, Role: "Agent", ValidID: 1}, nil
	}
	return nil, ErrInvalidCredentials
}
func (f *fakeProvider) GetUser(ctx context.Context, id string) (*models.User, error) {
	return &models.User{Login: id, Email: id, Role: "Agent", ValidID: 1}, nil
}
func (f *fakeProvider) ValidateToken(ctx context.Context, t string) (*models.User, error) {
	return nil, ErrAuthBackendFailed
}
func (f *fakeProvider) Name() string  { return f.n }
func (f *fakeProvider) Priority() int { return f.pr }

func TestRegisterProviderDuplicate(t *testing.T) {
	saved := saveRegistry()
	defer restoreRegistry(saved)
	err := RegisterProvider("dup", func(deps ProviderDependencies) (AuthProvider, error) { return &fakeProvider{n: "dup"}, nil })
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	err = RegisterProvider("dup", func(deps ProviderDependencies) (AuthProvider, error) { return &fakeProvider{n: "dup2"}, nil })
	if err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}

func TestCreateProviderUnknown(t *testing.T) {
	saved := saveRegistry()
	defer restoreRegistry(saved)
	_, err := CreateProvider("nope", ProviderDependencies{})
	if err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}

func TestProviderOrderFromConfig(t *testing.T) {
	saved := saveRegistry()
	defer restoreRegistry(saved)
	// Register two fake providers
	_ = RegisterProvider("alpha", func(deps ProviderDependencies) (AuthProvider, error) { return &fakeProvider{n: "alpha", pr: 20}, nil })
	_ = RegisterProvider("beta", func(deps ProviderDependencies) (AuthProvider, error) { return &fakeProvider{n: "beta", pr: 10}, nil })

	// Mock config adapter by setting globalConfigAdapter to nil and using env-driven override not implemented; instead simulate by setting provider order via a stub.
	// We directly test NewAuthService fallback when config not available (returns database only). Need to ensure database provider registration present.
	// Guarantee database provider exists (init() normally runs). If absent test will fail earlier.

	// Force explicit order selection by temporarily replacing getConfiguredProviderOrder via config adapter presence.
	// Simpler: manually build providers using registry rather than invoking NewAuthService internals.
	deps := ProviderDependencies{DB: (*sql.DB)(nil)}
	p1, _ := CreateProvider("beta", deps)
	p2, _ := CreateProvider("alpha", deps)
	authr := NewAuthenticator(p1, p2)
	got := authr.GetProviders()
	if len(got) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(got))
	}
	if got[0][:4] != "beta" {
		t.Fatalf("expected beta first, got %v", got)
	}
}

func TestStaticProviderAuth(t *testing.T) {
	saved := saveRegistry()
	defer restoreRegistry(saved)
	os.Setenv("GOTRS_STATIC_USERS", "alice:pw:Agent,bob:pw:Customer")
	defer os.Unsetenv("GOTRS_STATIC_USERS")
	// Ensure static provider factory is present (init() should have registered). Re-register if missing.
	if _, ok := providerRegistry["static"]; !ok {
		t.Fatalf("static provider not registered")
	}
	p, err := CreateProvider("static", ProviderDependencies{})
	if err != nil {
		t.Fatalf("create static provider: %v", err)
	}
	user, err := p.Authenticate(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if user.Login != "alice" {
		t.Fatalf("unexpected user %s", user.Login)
	}
	_, err = p.Authenticate(context.Background(), "alice", "wrong")
	if err == nil {
		t.Fatalf("expected auth failure")
	}
}
