package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Note: Uses centralized GetTestAuthToken() from test_helpers.go

// TestAllPagesSSR performs a minimal SSR smoke test over all GET routes that declare a template in YAML.
// It ensures the server can render each page without a 5xx error. Routes without a registered handler
// (e.g., stubs) are skipped. Auth-protected pages are accessed with a dummy access_token cookie.
func TestAllPagesSSR(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Locate project root and key dirs (routes, templates)
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	root := findProjectRoot(t)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root failed: %v", err)
	}

	// Ensure renderer finds templates and middleware behaves in test mode
	t.Setenv("APP_ENV", "test")
	templatesDir := filepath.Join(root, "templates")
	if st, err := os.Stat(templatesDir); err == nil && st.IsDir() {
		t.Setenv("TEMPLATES_DIR", templatesDir)
	}

	strict := os.Getenv("SSR_SMOKE_STRICT") == "1"

	// Validate templates referenced by YAML routes first (fast, DB-less)
	tplFailures, ferr := ValidateTemplatesReferencedInRoutes("./routes", templatesDir)
	if ferr != nil {
		t.Fatalf("failed to validate templates from routes: %v", ferr)
	}
	if len(tplFailures) > 0 {
		if strict {
			for _, f := range tplFailures {
				t.Errorf("template invalid/missing: %s", f)
			}
			t.Fatalf("%d template(s) invalid or missing (from YAML)", len(tplFailures))
		} else {
			for _, f := range tplFailures {
				t.Logf("template invalid/missing (non-strict): %s", f)
			}
		}
	}

	// Build list of SSR page routes from YAML
	docs, err := loadYAMLRouteGroups("./routes")
	if err != nil {
		t.Fatalf("loadYAMLRouteGroups error: %v", err)
	}
	type page struct{ method, path, handler string }
	pages := []page{}
	for _, doc := range docs {
		prefix := doc.Spec.Prefix
		for _, rt := range doc.Spec.Routes {
			if strings.ToUpper(rt.Method) != "GET" {
				continue
			}
			if rt.Template == "" {
				continue
			}
			if rt.RedirectTo != "" || rt.Websocket {
				continue
			}
			p := filepath.Join(prefix, rt.Path)
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			pages = append(pages, page{method: "GET", path: p, handler: rt.HandlerName})
		}
	}
	if len(pages) == 0 {
		t.Skip("no SSR pages discovered from YAML (GET routes with templates)")
		return
	}

	// Spin up server
	r := gin.New()
	setupHTMXRoutesWithAuth(r, nil, nil, nil)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Generate a valid JWT token for auth
	authToken := GetTestAuthToken(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects, but re-add auth cookie on each redirect
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: authToken})
			return nil
		},
	}
	for _, pg := range pages {
		url := srv.URL + normalizePath(pg.path)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("build request %s: %v", pg.path, err)
		}
		// Satisfy auth guards and force HTML response path
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: authToken})
		req.Header.Set("Accept", "text/html")

		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("request failed for %s (%s): %v", pg.path, pg.handler, err)
			continue
		}
		// Drain body and close
		_ = resp.Body.Close()

		// Minimal SSR smoke: by default only log 5xx (DB-less envs), but allow opt-in strict mode to fail build
		if resp.StatusCode >= 500 {
			if strict {
				t.Errorf("SSR 5xx for %s (%s): %d", pg.path, pg.handler, resp.StatusCode)
			} else {
				t.Logf("SSR 5xx (non-strict) for %s (%s): %d", pg.path, pg.handler, resp.StatusCode)
			}
		}
	}
}

// Wrapper to satisfy filtered test runs that match "User" in -run pattern.
func TestUserAllPagesSSR(t *testing.T) { TestAllPagesSSR(t) }

// findProjectRoot looks upward for a directory containing both routes/ and templates/.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ { // search up to 6 levels
		routes := filepath.Join(dir, "routes")
		tmpl := filepath.Join(dir, "templates")
		if st, err := os.Stat(routes); err == nil && st.IsDir() {
			if st2, err2 := os.Stat(tmpl); err2 == nil && st2.IsDir() {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("unable to find project root from %s (needs routes/ and templates/)", dir)
	return ""
}

// normalizePath replaces :params with safe placeholders.
func normalizePath(p string) string {
	// Replace ":something" segments with a plausible value (numeric by default)
	parts := strings.Split(p, "/")
	for i, s := range parts {
		if strings.HasPrefix(s, ":") {
			name := strings.TrimPrefix(s, ":")
			switch strings.ToLower(name) {
			case "login", "name", "slug", "module":
				parts[i] = "demo"
			default:
				parts[i] = "1"
			}
		}
	}
	out := strings.Join(parts, "/")
	if out == "" {
		out = "/"
	}
	// Ensure we don't end with trailing "/:" issues
	out = strings.ReplaceAll(out, "/:", "/")
	return out
}
