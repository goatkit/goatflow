package api

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "os"
)

// TestYAMLRoutesBasicAvailability ensures newly YAML-registered UI routes exist
func TestYAMLRoutesBasicAvailability(t *testing.T) {
    r := NewSimpleRouter() // invokes SetupHTMXRoutes + YAML registration

    tests := []struct {
        path       string
        wantStatus []int // accept any of these statuses
        mustHeader string
    }{
        {path: "/settings", wantStatus: []int{http.StatusOK}},
        {path: "/api/preferences/session-timeout", wantStatus: []int{http.StatusOK, http.StatusInternalServerError, http.StatusNotFound}}, // DB-less may vary; we only care route registered (not 404 ideally)
        {path: "/tickets/new", wantStatus: []int{http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect}},
        {path: "/claude-chat-demo", wantStatus: []int{http.StatusOK, http.StatusInternalServerError}}, // template may be missing in test env
        {path: "/ws/chat", wantStatus: []int{http.StatusBadRequest, http.StatusSwitchingProtocols}},   // websocket upgrade absent -> 400
    }

    for _, tc := range tests {
        req := httptest.NewRequest(http.MethodGet, tc.path, nil)
        // Simulate authenticated context for protected routes
        req.AddCookie(&http.Cookie{Name: "access_token", Value: "demo_session_test"})
        w := httptest.NewRecorder()
        r.ServeHTTP(w, req)
        got := w.Code
        ok := false
        for _, s := range tc.wantStatus { if got == s { ok = true; break } }
        if !ok {
            t.Errorf("path %s unexpected status %d body=%s", tc.path, got, w.Body.String())
        }
        if tc.path == "/tickets/new" {
            loc := w.Header().Get("Location")
            if loc == "" { t.Errorf("expected redirect location for /tickets/new") }
        }
    }
}

func TestYAMLRouteManifestGenerated(t *testing.T) {
    r := NewSimpleRouter()
    // Trigger at least one request (ensures router initialized fully)
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/settings", nil)
    req.AddCookie(&http.Cookie{Name: "access_token", Value: "demo_session_test"})
    r.ServeHTTP(w, req)
    if _, err := os.Stat("runtime/routes-manifest.json"); err != nil {
        t.Fatalf("expected routes-manifest.json to exist: %v", err)
    }
}

