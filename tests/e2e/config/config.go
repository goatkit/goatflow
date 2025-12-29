package config

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// TestConfig holds all configuration for E2E tests
type TestConfig struct {
	BaseURL       string
	Timeout       time.Duration
	Headless      bool
	SlowMo        int
	Screenshots   bool
	Videos        bool
	AdminEmail    string
	AdminPassword string
}

var loadOnce sync.Once

// loadDotEnv loads simple KEY=VALUE lines from .env if present.
// Existing environment variables take precedence and are not overwritten.
func loadDotEnv() {
	paths := []string{".env"}
	for _, p := range paths {
		f, err := os.Open(p)
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
	if os.Getenv("E2E_BASEURL_AUTODETECT") != "false" {
		baseURL = detectReachableBaseURLVerbose(baseURL)
		baseURL = fallbackBackendHost(baseURL)
	}
	log.Printf("[e2e-config] Resolved BaseURL=%s (RAW_BASE_URL=%s)", baseURL, os.Getenv("RAW_BASE_URL"))

	adminEmail := firstNonEmpty(
		os.Getenv("DEMO_ADMIN_EMAIL"),
		os.Getenv("ADMIN_USER"),
		os.Getenv("TEST_USERNAME"),
		"root@localhost",
	)
	adminPassword := firstNonEmpty(
		os.Getenv("DEMO_ADMIN_PASSWORD"),
		os.Getenv("ADMIN_PASSWORD"),
		os.Getenv("TEST_PASSWORD"),
		os.Getenv("TEST_MYSQL_ADMIN_PASSWORD"),
		os.Getenv("TEST_PG_ADMIN_PASSWORD"),
	)
	if adminPassword == "" {
		log.Fatal("[e2e-config] ERROR: DEMO_ADMIN_PASSWORD must be set in .env")
	}

	headless := os.Getenv("HEADLESS") != "false"
	slowMo := 0
	if os.Getenv("SLOW_MO") != "" {
		// Parse slow mo if needed
		slowMo = 100 // Default to 100ms for debugging
	}

	return &TestConfig{
		BaseURL:       baseURL,
		Timeout:       30 * time.Second,
		Headless:      headless,
		SlowMo:        slowMo,
		Screenshots:   os.Getenv("SCREENSHOTS") != "false",
		Videos:        os.Getenv("VIDEOS") == "true",
		AdminEmail:    adminEmail,
		AdminPassword: adminPassword,
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

// detectReachableBaseURL attempts to find a responsive backend if the provided baseURL is not reachable.
func detectReachableBaseURLVerbose(initial string) string {
	start := time.Now()
	// Fast path: if initial works, keep it.
	if reachable(initial) {
		return initial
	}

	// Build candidate list: start with initial (already failed), then variations.
	tried := []string{initial}
	candidates := []string{}

	// Extract host/port from initial for port permutations.
	u, err := url.Parse(initial)
	if err == nil {
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "8080"
		}
		// If host contains underscores (compose service) also try localhost / 127.0.0.1 with same port first.
		basePorts := []string{port, "8082", "8080", "18080", "8081"}
		if testPort := os.Getenv("TEST_BACKEND_PORT"); testPort != "" {
			basePorts = append([]string{testPort}, basePorts...)
		}
		// Preserve order but de-dupe later.
		if host != "localhost" && host != "127.0.0.1" {
			for _, p := range basePorts {
				candidates = append(candidates, "http://localhost:"+p)
			}
			for _, p := range basePorts {
				candidates = append(candidates, "http://127.0.0.1:"+p)
			}
		}
		// If original host looked like backend service include its canonical variants.
		if strings.Contains(host, "backend") {
			for _, p := range []string{"8080", "18080", "8081"} {
				candidates = append(candidates, "http://backend:"+p)
			}
		}
	}
	if tb := os.Getenv("TEST_BACKEND_BASE_URL"); tb != "" {
		candidates = append([]string{tb}, candidates...)
	}
	// Always ensure plain localhost:8080 present.
	candidates = append(candidates, "http://localhost:8080")

	// De-dupe while preserving order and skipping the failed initial.
	seen := map[string]struct{}{initial: {}}
	uniq := []string{}
	for _, c := range candidates {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		uniq = append(uniq, c)
	}

	for _, c := range uniq {
		tried = append(tried, c)
		if reachable(c) {
			log.Printf("[e2e-config] Auto-detect switched BaseURL %s -> %s (%.0fms; order=%v)", initial, c, time.Since(start).Seconds()*1000, tried)
			return c
		}
	}
	log.Printf("[e2e-config] Auto-detect kept unreachable BaseURL=%s (no reachable candidates; tried=%v in %.0fms)", initial, tried, time.Since(start).Seconds()*1000)
	return initial
}

func reachable(base string) bool {
	// TCP probe
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	d := net.Dialer{Timeout: 250 * time.Millisecond}
	conn, err := d.Dial("tcp", host)
	if err != nil {
		return false
	}
	_ = conn.Close()
	client := &http.Client{Timeout: 800 * time.Millisecond}
	// Prefer /healthz quick check
	for _, path := range []string{"/healthz", "/login"} {
		req, _ := http.NewRequest("GET", base+path, nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			return true
		}
	}
	return false
}

// fallbackBackendHost converts host 'backend' to 'localhost' if 'backend' is not reachable but localhost alternative is.
func fallbackBackendHost(current string) string {
	u, err := url.Parse(current)
	if err != nil {
		return current
	}
	if u.Hostname() != "backend" {
		return current
	}
	// Try backend first
	if reachable(current) {
		return current
	}
	// Replace with localhost: same port
	port := u.Port()
	if port == "" {
		port = "8080"
	}
	candidate := u.Scheme + "://localhost:" + port
	if reachable(candidate) {
		return candidate
	}
	return current
}
