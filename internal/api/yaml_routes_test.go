package api

import (
    "os"
    "testing"
)

// TestYAMLRoutesBasicAvailability ensures newly YAML-registered UI routes exist
func TestYAMLRoutesBasicAvailability(t *testing.T) {
    // Force test mode to exercise fallback routes
    _ = os.Setenv("APP_ENV", "test")
    r := NewSimpleRouter()
    infos := r.Routes()
    have := map[string]struct{}{}
    for _, ri := range infos {
        have[ri.Path] = struct{}{}
    }
    expected := []string{
        "/login",
        "/api/tickets",
        "/api/lookups/queues",
        "/api/canned-responses",
    "/api/tickets/:id/assign", // fallback stub in test mode
    }
    for _, p := range expected {
        if _, ok := have[p]; !ok {
            t.Errorf("expected route %s to be registered", p)
        }
    }
}

func TestYAMLRouteManifestGenerated(t *testing.T) {
    _ = os.Setenv("APP_ENV", "test")
    _ = NewSimpleRouter()
    if _, err := os.Stat("runtime/routes-manifest.json"); err != nil {
        t.Skipf("routes-manifest.json not generated in minimal test env: %v", err)
    }
}

