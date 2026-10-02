//go:build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// These tests talk to a running GoatFlow server. `make test` starts the dedicated
// test stack (backend-test) and passes TEST_BACKEND_BASE_URL; to run them by hand
// against any server: TEST_BACKEND_BASE_URL=http://localhost:8080 go test -tags integration
// -run 'TestLoginPage|TestRootRedirects' ./tests/integration
var backendBaseURL = resolveBackendBaseURL()

func resolveBackendBaseURL() string {
	base := strings.TrimSpace(os.Getenv("TEST_BACKEND_BASE_URL"))
	if base != "" {
		return strings.TrimRight(base, "/")
	}
	host := firstNonEmpty(
		os.Getenv("TEST_BACKEND_SERVICE_HOST"),
		os.Getenv("TEST_BACKEND_HOST"),
	)
	if host == "" {
		host = "backend-test"
	}
	port := firstNonEmpty(
		os.Getenv("TEST_BACKEND_CONTAINER_PORT"),
		os.Getenv("TEST_BACKEND_PORT"),
	)
	if port == "" {
		port = "8080"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// noRedirectClient returns each response as-is so redirects can be asserted.
var noRedirectClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func getNoRedirect(t *testing.T, path string) (*http.Response, string) {
	t.Helper()
	target := backendBaseURL + path
	resp, err := noRedirectClient.Get(target)
	if err != nil {
		t.Fatalf("GET %s failed: %v (start the test stack with `make test-stack-up`, or set TEST_BACKEND_BASE_URL to a running server)", target, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", target, err)
	}
	return resp, string(body)
}

// TestLoginPageServes200 ensures /login renders the login form directly for an
// unauthenticated client (no redirect, so no loop).
func TestLoginPageServes200(t *testing.T) {
	resp, body := getNoRedirect(t, "/login")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/login returned %d (Location %q), want 200", resp.StatusCode, resp.Header.Get("Location"))
	}
	if !strings.Contains(body, `action="/api/auth/login"`) {
		t.Fatalf("/login did not render the login form posting to /api/auth/login")
	}
}

// TestRootRedirectsAnonymousToLogin ensures an unauthenticated GET / redirects
// exactly once, to /login, and that target serves 200.
func TestRootRedirectsAnonymousToLogin(t *testing.T) {
	resp, _ := getNoRedirect(t, "/")
	switch resp.StatusCode {
	case http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect:
	default:
		t.Fatalf("GET / returned %d, want a temporary redirect to /login", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("GET / Location %q: %v", resp.Header.Get("Location"), err)
	}
	if loc.Path != "/login" {
		t.Fatalf("GET / redirected to %q, want /login", loc.String())
	}

	next, _ := getNoRedirect(t, loc.RequestURI())
	if next.StatusCode != http.StatusOK {
		t.Fatalf("redirect target %s returned %d (Location %q): redirect chain does not settle", loc.RequestURI(), next.StatusCode, next.Header.Get("Location"))
	}
}
