package service

import (
    "testing"
    "os"
    "context"
    "time"
    "github.com/gotrs-io/gotrs-ce/internal/yamlmgmt"
    "github.com/gotrs-io/gotrs-ce/internal/auth"
)

// helper to build a minimal config adapter with Auth::Providers list
func testConfigAdapter(t *testing.T, providers []string) *yamlmgmt.ConfigAdapter {
    t.Helper()
    vm := yamlmgmt.NewVersionManager(t.TempDir())
    doc := &yamlmgmt.YAMLDocument{
        APIVersion: "gotrs.io/v1",
        Kind:       string(yamlmgmt.KindConfig),
        Metadata:   yamlmgmt.Metadata{Name: "system-config"},
        Data: map[string]interface{}{
            "settings": []interface{}{
                map[string]interface{}{
                    "name":    "Auth::Providers",
                    "default": providers,
                },
            },
        },
    }
    if _, err := vm.CreateVersion(yamlmgmt.KindConfig, "system-config", doc, "test"); err != nil {
        t.Fatalf("create version: %v", err)
    }
    return yamlmgmt.NewConfigAdapter(vm)
}

// minimal jwt manager constructor (reuse existing shared constructor if available); here we just create one directly
func testJWTManager(t *testing.T) *auth.JWTManager {
    t.Helper()
    return auth.NewJWTManager("test-secret", time.Hour)
}

func TestAuthService_UsesStaticProviderFirst(t *testing.T) {
    os.Setenv("GOTRS_STATIC_USERS", "alpha:pw:Agent")
    defer os.Unsetenv("GOTRS_STATIC_USERS")

    ca := testConfigAdapter(t, []string{"static", "database"})
    SetConfigAdapter(ca)
    svc := NewAuthService(nil, testJWTManager(t))

    // Should authenticate via static provider; db provider skipped (nil DB)
    user, _, _, err := svc.Login(context.Background(), "alpha", "pw")
    if err != nil { t.Fatalf("expected static auth success, got %v", err) }
    if user.Login != "alpha" { t.Fatalf("unexpected user login %s", user.Login) }
}

func TestAuthService_FallbackNoProviders(t *testing.T) {
    ca := testConfigAdapter(t, []string{"bogus1", "bogus2"})
    SetConfigAdapter(ca)
    svc := NewAuthService(nil, testJWTManager(t))
    _, _, _, err := svc.Login(context.Background(), "any", "x")
    if err == nil { t.Fatalf("expected failure with no providers") }
}
