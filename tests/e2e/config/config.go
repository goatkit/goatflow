package config

import (
	"bufio"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestConfig holds all configuration for E2E tests
type TestConfig struct {
	BaseURL           string
	CustomerPortalURL string
	Timeout           time.Duration
	Headless          bool
	SlowMo            int
	Screenshots       bool
	Videos            bool
	AdminEmail        string
	AdminPassword     string
}

var loadOnce sync.Once

// loadDotEnv loads simple KEY=VALUE lines from .env if present.
// Existing environment variables take precedence and are not overwritten.
func loadDotEnv() {
	paths := []string{".env"}
	for _, p := range paths {
		f, err := os.Open(p) // #nosec G304 -- fixed ".env" path from the constant list above
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") { // skip comments/empty
				continue
			}
			if i := strings.Index(line, "="); i > 0 {
				key := strings.TrimSpace(line[:i])
				val := strings.TrimSpace(line[i+1:])
				if val == "" || key == "" {
					continue
				}
				// Strip optional surrounding quotes
				if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) || (strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
					val = val[1 : len(val)-1]
				}
				if os.Getenv(key) == "" { // don't override existing
					_ = os.Setenv(key, val)
				}
			}
		}
		_ = f.Close()
	}
}

// GetConfig returns the test configuration from environment variables
func GetConfig() *TestConfig {
	loadOnce.Do(loadDotEnv)
	baseURL := os.Getenv("BASE_URL")
	if forced := os.Getenv("RAW_BASE_URL"); forced != "" { // explicit injection hook for tests
		baseURL = forced
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	// No auto-detection: probing alternative hosts could silently aim the suite at
	// the wrong stack (e.g. the dev backend instead of backend-test).
	log.Printf("[e2e-config] BaseURL=%s", baseURL) // #nosec G706 -- operator-set test env var logged to the test runner's own output

	adminEmail := firstNonEmpty(
		os.Getenv("TEST_USERNAME"),
		os.Getenv("DEMO_ADMIN_EMAIL"),
		os.Getenv("ADMIN_USER"),
		"root@localhost",
	)
	adminPassword := firstNonEmpty(
		os.Getenv("TEST_PASSWORD"),
		os.Getenv("DEMO_ADMIN_PASSWORD"),
		os.Getenv("ADMIN_PASSWORD"),
	)
	if adminPassword == "" {
		log.Fatal("[e2e-config] ERROR: TEST_PASSWORD (or DEMO_ADMIN_PASSWORD) must be set in .env")
	}

	headless := os.Getenv("HEADLESS") != "false"
	slowMo := 0
	if v := os.Getenv("SLOW_MO"); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms < 0 {
			log.Fatalf("[e2e-config] ERROR: SLOW_MO must be a non-negative number of milliseconds, got %q", v) // #nosec G706 -- operator-set test env var, %q-quoted
		}
		slowMo = ms
	}

	// Customer portal runs in a separate container on a different port
	customerPortalURL := os.Getenv("CUSTOMER_PORTAL_URL")
	if customerPortalURL == "" {
		// Derive from baseURL - customer portal test container is on port 8084
		// In test environment: backend-test:8080 -> customer-fe-test:8080 (exposed as 8084)
		u, err := url.Parse(baseURL)
		if err == nil {
			host := u.Hostname()
			// Map backend host to customer-fe host
			if strings.Contains(host, "backend") {
				host = strings.Replace(host, "backend", "customer-fe", 1)
			}
			customerPortalURL = u.Scheme + "://" + host + ":" + u.Port()
		}
		if customerPortalURL == "" {
			customerPortalURL = "http://localhost:8084"
		}
	}

	return &TestConfig{
		BaseURL:           baseURL,
		CustomerPortalURL: customerPortalURL,
		Timeout:           30 * time.Second,
		Headless:          headless,
		SlowMo:            slowMo,
		Screenshots:       os.Getenv("SCREENSHOTS") != "false",
		Videos:            os.Getenv("VIDEOS") == "true",
		AdminEmail:        adminEmail,
		AdminPassword:     adminPassword,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// RequireReachable fails the test unless the backend under test answers GET /login.
func (c *TestConfig) RequireReachable(t testing.TB) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(c.BaseURL + "/login")
	if err != nil {
		t.Fatalf("backend under test not reachable at %s/login: %v (set BASE_URL to a running GoatFlow server)", c.BaseURL, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Fatalf("backend under test at %s/login answered %d", c.BaseURL, resp.StatusCode)
	}
}
