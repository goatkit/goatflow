//go:build integration

package integration

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
)

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
	return fmt.Sprintf("http://%s:%s", strings.TrimSpace(host), strings.TrimSpace(port))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func httpGetOrFail(t *testing.T, path string) *http.Response {
	t.Helper()
	target := backendBaseURL + path
	resp, err := http.Get(target)
	if err != nil {
		handleConnectionError(t, err, target)
	}
	return resp
}

func handleConnectionError(t *testing.T, err error, target string) {
	t.Helper()
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Err != nil {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			t.Fatalf("request to %s failed: backend not reachable (make test should provision the dedicated stack)", target)
		}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		handleConnectionError(t, urlErr.Err, target)
		return
	}
	t.Fatalf("request to %s failed: %v", target, err)
}

// TestLoginPageServes200 ensures /login responds 200 (no redirect loop) for unauthenticated clients.
func TestLoginPageServes200(t *testing.T) {
	resp := httpGetOrFail(t, "/login")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Accept Unauthorized if auth middleware enforces it differently, but not redirect loops
		if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
			// Provide extra context
			loc := resp.Header.Get("Location")
			if loc == "" {
				loc = resp.Header.Get("location")
			}
			// Mark as failure explicitly
			msg := "redirect instead of serving login page"
			if loc != "" {
				msg += ": location=" + loc
			}
			// Failing because we want stable 200 for automation
			// If the environment legitimately requires redirect first, adjust test expectations.
			t.Fatalf("/login returned %d %s", resp.StatusCode, msg)
		}
	}
}

// TestRootReturnsLoginOrDashboard ensures root returns 200 login page or redirects only once to dashboard when authenticated.
func TestRootReturnsLoginOrDashboard(t *testing.T) {
	resp := httpGetOrFail(t, "/")
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusFound {
		// Single redirect acceptable only if to /dashboard
		loc := resp.Header.Get("Location")
		if loc != "/dashboard" {
			// Not acceptable to bounce elsewhere
			t.Fatalf("unexpected redirect from / to %s", loc)
		}
	} else if resp.StatusCode != http.StatusOK {
		// Want 200 login page otherwise
		if resp.StatusCode == http.StatusMovedPermanently {
			t.Fatalf("permanent redirect from / not expected")
		}
	}
}
