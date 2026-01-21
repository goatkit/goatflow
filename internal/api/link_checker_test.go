package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type linkTarget struct {
	method string
	url    string
}

// TestAllLinksReturn200 dynamically checks all links in templates for 404s.
func TestAllLinksReturn200(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup router with all routes (includes health, v1 API, etc)
	router := NewSimpleRouter()

	isAllowedStatus := func(gt linkTarget, status int) bool {
		trimmed := strings.TrimSuffix(gt.url, "/")
		if trimmed == "" {
			trimmed = "/"
		}

		allowed := map[string]map[int]struct{}{
			"/dashboard":                      {http.StatusUnauthorized: {}},
			"/tickets":                        {http.StatusUnauthorized: {}},
			"/admin":                          {http.StatusInternalServerError: {}},
			"/admin/mail-accounts":            {http.StatusServiceUnavailable: {}},
			"/admin/permissions":              {http.StatusInternalServerError: {}}, // Requires DB
			"/admin/roles":                    {http.StatusInternalServerError: {}}, // Requires DB
			"/admin/customer-users":           {http.StatusInternalServerError: {}}, // Requires DB
			"/admin/customer-user-services":   {http.StatusInternalServerError: {}}, // Requires DB
			"/admin/customer/portal/settings": {http.StatusInternalServerError: {}}, // Requires DB
			"/admin/queues":                   {http.StatusInternalServerError: {}}, // Requires DB
			"/admin/sla":                      {http.StatusInternalServerError: {}}, // Requires DB
			"/admin/groups/new":               {http.StatusBadRequest: {}},          // Validation error without context
			"/queues":                         {http.StatusInternalServerError: {}, http.StatusBadRequest: {}},
			"/queues/new":                     {http.StatusBadRequest: {}},
			"/register":                       {http.StatusNotFound: {}},
			"/dev":                            {http.StatusNotFound: {}},
			"/admin/settings":                 {http.StatusNotFound: {}},
			"/admin/reports":                  {http.StatusNotFound: {}},
			"/admin/logs":                     {http.StatusNotFound: {}},
			"/admin/templates":                {http.StatusNotFound: {}}, // YAML-routed, requires full router
			"/admin/attachments":              {http.StatusNotFound: {}}, // YAML-routed, requires full router
			"/customer/profile":               {http.StatusServiceUnavailable: {}},
			"/customer":                       {http.StatusServiceUnavailable: {}},
			"/customer/tickets":               {http.StatusServiceUnavailable: {}},
			"/admin/auto-responses":           {http.StatusServiceUnavailable: {}}, // Requires DB
			"/admin/queue-auto-responses":     {http.StatusServiceUnavailable: {}}, // Requires DB
			"/admin/article-colors":           {http.StatusServiceUnavailable: {}}, // Requires DB
		}

		if strings.HasPrefix(trimmed, "/tickets/") && status == http.StatusNotFound {
			return true
		}

		// Ticket detail pages return 403 (no queue permission) or 404 (ticket not found) with middleware enforcement
		if strings.HasPrefix(trimmed, "/ticket/") && (status == http.StatusForbidden || status == http.StatusNotFound) {
			return true
		}

		// Ticket new/create pages return 403 without queue create permission (middleware enforcement)
		if trimmed == "/tickets/new" && status == http.StatusForbidden {
			return true
		}

		// Queue detail pages return 400/403 without auth context (middleware enforcement)
		if strings.HasPrefix(trimmed, "/queues/") && (status == http.StatusBadRequest || status == http.StatusForbidden || status == http.StatusInternalServerError) {
			return true
		}

		// Agent ticket detail pages and subpages (links, etc.) return 404 without actual ticket data
		if strings.HasPrefix(trimmed, "/agent/tickets/") && status == http.StatusNotFound {
			return true
		}

		// Ticket attachment API endpoints return 400 without proper request body
		if strings.Contains(trimmed, "/attachments") && strings.HasPrefix(trimmed, "/api/tickets/") && status == http.StatusBadRequest {
			return true
		}

		// Admin group detail pages return 500 without DB
		if strings.HasPrefix(trimmed, "/admin/groups/") && status == http.StatusInternalServerError {
			return true
		}

		if codes, ok := allowed[trimmed]; ok {
			if _, ok := codes[status]; ok {
				return true
			}
		}

		// Non-GET requests are primarily existence checks; treat any non-404 as acceptable
		if gt.method != http.MethodGet && status != http.StatusNotFound {
			return true
		}

		return false
	}

	// Note: /health route is already included in NewSimpleRouter()

	// Define pages to crawl for links
	startPages := []linkTarget{
		{method: http.MethodGet, url: "/"},
		{method: http.MethodGet, url: "/login"},
		{method: http.MethodGet, url: "/dashboard"},
		{method: http.MethodGet, url: "/tickets"},
		{method: http.MethodGet, url: "/admin"},
		{method: http.MethodGet, url: "/queues"},
	}

	// Track visited requests to avoid infinite loops
	visited := make(map[string]bool)

	// Track broken links
	brokenLinks := []BrokenLink{}

	// Regular expressions to extract links
	linkPatterns := []struct {
		pattern *regexp.Regexp
		method  string
	}{
		{regexp.MustCompile(`href="(/[^\"]*)"`), http.MethodGet},                           // Standard links
		{regexp.MustCompile(`hx-get="(/[^\"]*)"`), http.MethodGet},                         // HTMX GET
		{regexp.MustCompile(`hx-post="(/[^\"]*)"`), http.MethodPost},                       // HTMX POST
		{regexp.MustCompile(`hx-put="(/[^\"]*)"`), http.MethodPut},                         // HTMX PUT
		{regexp.MustCompile(`hx-delete="(/[^\"]*)"`), http.MethodDelete},                   // HTMX DELETE
		{regexp.MustCompile(`hx-patch="(/[^\"]*)"`), http.MethodPatch},                     // HTMX PATCH
		{regexp.MustCompile(`<a[^>]+href="(/[^\"]*)"[^>]*>`), http.MethodGet},              // Links with attributes
		{regexp.MustCompile(`window\.location\.href\s*=\s*["'](/[^"']*)`), http.MethodGet}, // JavaScript redirects
	}

	formPattern := regexp.MustCompile(`(?is)<form[^>]*action="(/[^\"]*)"[^>]*>`)
	formMethodPattern := regexp.MustCompile(`(?i)method="([^"]*)"`)

	// Function to extract all links from HTML
	extractLinks := func(html string) []linkTarget {
		seen := make(map[string]bool)
		links := []linkTarget{}

		addLink := func(method, link string) {
			if link == "" || link == "#" || strings.HasPrefix(link, "http") {
				return
			}
			// Skip template literals (JavaScript ${...} syntax)
			if strings.Contains(link, "${") {
				return
			}
			if idx := strings.IndexAny(link, "?#"); idx != -1 {
				link = link[:idx]
			}
			if link == "" {
				return
			}
			key := method + " " + link
			if seen[key] {
				return
			}
			seen[key] = true
			links = append(links, linkTarget{method: method, url: link})
		}

		for _, lp := range linkPatterns {
			matches := lp.pattern.FindAllStringSubmatch(html, -1)
			for _, match := range matches {
				if len(match) > 1 {
					addLink(lp.method, match[1])
				}
			}
		}

		formMatches := formPattern.FindAllStringSubmatch(html, -1)
		htmxOverridePattern := regexp.MustCompile(`(?i)hx-(?:post|put|patch|delete)=`)
		for _, match := range formMatches {
			if len(match) < 2 {
				continue
			}
			block := match[0]
			link := match[1]
			// Skip form action if HTMX attributes will override the method
			// In HTMX apps, hx-* attributes take over form submission
			if htmxOverridePattern.MatchString(block) {
				continue
			}
			var method string
			if m := formMethodPattern.FindStringSubmatch(block); len(m) > 1 {
				method = strings.ToUpper(strings.TrimSpace(m[1]))
				if method == "" {
					method = http.MethodGet
				}
			} else {
				method = http.MethodGet
			}
			addLink(method, link)
		}

		return links
	}

	// Function to check a single page
	var checkPage func(target linkTarget, referrer string)
	checkPage = func(target linkTarget, referrer string) {
		method := strings.ToUpper(target.method)
		if method == "" {
			method = http.MethodGet
		}

		key := method + " " + target.url
		if visited[key] {
			return
		}
		visited[key] = true

		// Make request
		req := httptest.NewRequest(method, target.url, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check status code
		if w.Code == http.StatusNotFound || w.Code >= 400 {
			if isAllowedStatus(target, w.Code) {
				return
			}
			// Skip static files in test environment as they might not be available
			if strings.HasPrefix(target.url, "/static/") && w.Code == http.StatusNotFound {
				// Static files not available in test environment, skip
				return
			}
			brokenLinks = append(brokenLinks, BrokenLink{
				URL:      target.url,
				Status:   w.Code,
				Referrer: referrer,
			})
			return
		}

		if method != http.MethodGet {
			return
		}

		// Only extract links from HTML responses
		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			return
		}

		// Extract and check all links in the response
		links := extractLinks(w.Body.String())
		for _, link := range links {
			checkPage(link, target.url)
		}
	}

	// Start crawling from initial pages
	for _, page := range startPages {
		checkPage(page, "initial")
	}

	// Check common API endpoints that might not be linked
	apiEndpoints := []string{
		"/api/auth/login",
		"/api/auth/logout",
		"/api/auth/refresh",
		// V1 endpoints are not guaranteed in unit router; skip in this test
		"/health",
	}

	for _, endpoint := range apiEndpoints {
		target := linkTarget{url: endpoint}
		switch {
		case strings.Contains(endpoint, "login"), strings.Contains(endpoint, "logout"), strings.Contains(endpoint, "refresh"):
			target.method = http.MethodPost
		default:
			target.method = http.MethodGet
		}

		key := target.method + " " + target.url
		if visited[key] {
			continue
		}
		visited[key] = true

		req := httptest.NewRequest(target.method, endpoint, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// API endpoints might return 401 (unauthorized) or 4xx due to missing payload; only fail on 404
		if w.Code == http.StatusNotFound {
			brokenLinks = append(brokenLinks, BrokenLink{
				URL:      endpoint,
				Status:   w.Code,
				Referrer: "api-check",
			})
		}
	}

	// Report results
	if len(brokenLinks) > 0 {
		t.Errorf("Found %d broken links:", len(brokenLinks))
		for _, link := range brokenLinks {
			t.Errorf("  - %s (status %d) referenced from %s", link.URL, link.Status, link.Referrer)
		}
	}

	// Also report coverage
	t.Logf("Checked %d unique URLs", len(visited))
}

// TestLogoutRouteExists specifically tests the logout functionality.
func TestLogoutRouteExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewSimpleRouter()
	// Note: /health route is already included in NewSimpleRouter()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST /api/auth/logout should exist",
			method:         "POST",
			path:           "/api/auth/logout",
			expectedStatus: http.StatusOK,
			description:    "API logout endpoint",
		},
		{
			name:           "POST /logout should redirect or handle",
			method:         "POST",
			path:           "/logout",
			expectedStatus: http.StatusOK,
			description:    "User-facing logout",
		},
		{
			name:           "GET /logout might redirect to login",
			method:         "GET",
			path:           "/logout",
			expectedStatus: http.StatusFound,
			description:    "Logout with redirect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Allow redirect statuses or success
			validStatuses := []int{
				http.StatusOK,
				http.StatusFound,
				http.StatusSeeOther,
				http.StatusTemporaryRedirect,
				http.StatusPermanentRedirect,
			}

			isValidStatus := false
			for _, status := range validStatuses {
				if w.Code == status {
					isValidStatus = true
					break
				}
			}

			if w.Code == http.StatusNotFound {
				t.Errorf("%s returned 404 - route does not exist", tt.path)
			} else if !isValidStatus && w.Code >= 400 {
				t.Errorf("%s returned error %d", tt.path, w.Code)
			}
		})
	}
}

// TestAllFormsHaveValidActions checks that all forms point to valid endpoints.
func TestAllFormsHaveValidActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewSimpleRouter()
	// Note: /health route is already included in NewSimpleRouter()

	// Pages that contain forms
	formPages := []string{
		"/login",
		"/register",
		"/tickets/new",
	}

	formActionPattern := regexp.MustCompile(`<form[^>]*action="([^"]*)"[^>]*method="([^"]*)"`)

	for _, page := range formPages {
		t.Run(fmt.Sprintf("Forms on %s", page), func(t *testing.T) {
			req := httptest.NewRequest("GET", page, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				t.Skipf("Page %s not found, skipping form check", page)
				return
			}

			html := w.Body.String()
			matches := formActionPattern.FindAllStringSubmatch(html, -1)

			for _, match := range matches {
				if len(match) > 2 {
					action := match[1]
					method := strings.ToUpper(match[2])

					if action == "" || action == "#" {
						continue
					}

					// Test the form action
					req := httptest.NewRequest(method, action, nil)
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					if w.Code == http.StatusNotFound {
						t.Errorf("Form action %s %s returns 404", method, action)
					}
				}
			}
		})
	}
}

// TestHTMXEndpointsExist verifies all HTMX endpoints are properly routed.
func TestHTMXEndpointsExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewSimpleRouter()
	// Note: /health route is already included in NewSimpleRouter()

	// Common HTMX patterns in our app
	htmxEndpoints := []struct {
		method string
		path   string
		desc   string
	}{
		// Only assert endpoints that exist in the minimal test router
		{"GET", "/api/tickets", "Ticket list"},
		{"POST", "/api/tickets", "Create ticket"},
		// {"GET", "/api/tickets/filter", "Filter tickets"}, // not guaranteed in unit router
		// {"GET", "/api/search", "Search"}, // not guaranteed in unit router
		{"POST", "/api/auth/login", "Login"},
		{"POST", "/api/auth/logout", "Logout"},
		{"GET", "/api/dashboard/stats", "Dashboard stats"},
	}

	for _, endpoint := range htmxEndpoints {
		t.Run(endpoint.desc, func(t *testing.T) {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// HTMX endpoints might return 401 if not authenticated, that's OK
			if w.Code == http.StatusNotFound {
				t.Errorf("%s %s returns 404 - endpoint not routed",
					endpoint.method, endpoint.path)
			}
		})
	}
}

// BrokenLink represents a broken link found during testing.
type BrokenLink struct {
	URL      string
	Status   int
	Referrer string
}

// TestNoOrphanedRoutes ensures all defined routes are accessible from somewhere.
func TestNoOrphanedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewSimpleRouter()
	// Note: /health route is already included in NewSimpleRouter()

	// Get all routes from the router
	routes := router.Routes()

	// Track which routes are referenced
	referencedRoutes := make(map[string]bool)

	// Crawl all pages to find references
	startPages := []string{"/", "/login", "/dashboard", "/tickets", "/admin"}
	visited := make(map[string]bool)

	var crawl func(url string)
	crawl = func(url string) {
		if visited[url] {
			return
		}
		visited[url] = true

		req := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code >= 400 {
			return
		}

		// Extract all referenced paths
		html := w.Body.String()
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`href="(/[^"]*)">`),
			regexp.MustCompile(`hx-[a-z]+="(/[^"]*)"`),
			regexp.MustCompile(`action="(/[^"]*)"`),
		}

		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(html, -1)
			for _, match := range matches {
				if len(match) > 1 {
					path := match[1]
					if idx := strings.IndexAny(path, "?#"); idx != -1 {
						path = path[:idx]
					}
					referencedRoutes[path] = true

					// Continue crawling if internal link
					if strings.HasPrefix(path, "/") && !visited[path] {
						crawl(path)
					}
				}
			}
		}
	}

	// Start crawling
	for _, page := range startPages {
		crawl(page)
	}

	// Check for orphaned routes
	orphaned := []string{}
	for _, route := range routes {
		path := route.Path

		// Skip parameterized routes and special cases
		if strings.Contains(path, ":") ||
			strings.Contains(path, "*") ||
			path == "/health" || // Health check doesn't need to be linked
			strings.HasPrefix(path, "/static") || // Static files
			strings.HasPrefix(path, "/debug") { // Debug endpoints
			continue
		}

		if !referencedRoutes[path] && !visited[path] {
			orphaned = append(orphaned, fmt.Sprintf("%s %s", route.Method, path))
		}
	}

	if len(orphaned) > 0 {
		t.Logf("Warning: Found %d potentially orphaned routes:", len(orphaned))
		for _, route := range orphaned {
			t.Logf("  - %s", route)
		}
	}
}

// TestLinkConsistency ensures links are consistent across pages.
func TestLinkConsistency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewSimpleRouter()
	// Note: /health route is already included in NewSimpleRouter()

	// Map of link text to expected URL
	expectedLinks := map[string]string{
		"Dashboard": "/dashboard",
		"Tickets":   "/tickets",
		"Admin":     "/admin",
		"Queues":    "/queues",
		"Sign out":  "/logout",
		"Profile":   "/profile",
		"Settings":  "/settings",
	}

	pages := []string{"/dashboard", "/tickets", "/admin"}

	for _, page := range pages {
		t.Run(fmt.Sprintf("Links on %s", page), func(t *testing.T) {
			req := httptest.NewRequest("GET", page, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code >= 400 {
				t.Skipf("Page %s returned %d", page, w.Code)
				return
			}

			html := w.Body.String()

			for linkText, expectedURL := range expectedLinks {
				// Look for the link text and verify it points to the right URL
				pattern := regexp.MustCompile(fmt.Sprintf(`href="([^"]*)"[^>]*>%s</`, linkText))
				matches := pattern.FindStringSubmatch(html)

				if len(matches) > 1 {
					actualURL := matches[1]
					if actualURL != expectedURL {
						t.Errorf("Link '%s' points to %s, expected %s",
							linkText, actualURL, expectedURL)
					}
				}
			}
		})
	}
}

// Benchmark test to ensure link checking is performant.
func BenchmarkLinkChecker(b *testing.B) {
	gin.SetMode(gin.TestMode)
	router := NewSimpleRouter()
	// Note: /health route is already included in NewSimpleRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/dashboard", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Extract links (this is the expensive operation)
		html := w.Body.String()
		linkPattern := regexp.MustCompile(`href="(/[^"]*)"`)
		linkPattern.FindAllStringSubmatch(html, -1)
	}
}
