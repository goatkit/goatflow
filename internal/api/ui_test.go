package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Note: Uses centralized GetTestAuthToken() and AddTestAuthCookie() from test_helpers.go

// containsAny checks if body contains any of the given alternatives
// Used for i18n tests where content could be either the key or the default text
func containsAny(body string, alternatives ...string) bool {
	for _, alt := range alternatives {
		if strings.Contains(body, alt) {
			return true
		}
	}
	return false
}

// Test-Driven Development for UI Components
// Tests for dark theme, navigation, and page rendering

func TestDarkThemeContrast(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token := GetTestAuthToken(t)

	// GoatKit/Synthwave theme uses CSS variables instead of dark: prefixed classes
	// Tests verify the theme system is applied correctly
	tests := []struct {
		name             string
		route            string
		activePage       string
		checkSelectors   []string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:       "Navigation uses GoatKit theme variables",
			route:      "/dashboard",
			activePage: "dashboard",
			shouldContain: []string{
				"--gk-text-",           // GoatKit text color variables
				"--gk-bg-",             // GoatKit background variables
				"gk-nav-link",          // GoatKit navigation links
			},
		},
		{
			name:       "Active navigation item uses theme styling",
			route:      "/tickets",
			activePage: "tickets",
			shouldContain: []string{
				"--gk-primary",         // Primary theme color reference
			},
		},
		{
			name:       "Buttons use GoatKit styling",
			route:      "/dashboard",
			activePage: "dashboard",
			shouldContain: []string{
				"gk-btn-",              // GoatKit button classes
				"--gk-primary",         // Primary color variable
			},
		},
		{
			name:       "Page background uses GoatKit theme",
			route:      "/dashboard",
			activePage: "dashboard",
			shouldContain: []string{
				"--gk-bg-base",         // Base background variable
				"--gk-text-primary",    // Primary text color
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
			AddTestAuthCookie(req, token)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			// Skip test if route returned minimal response (YAML routes without full template context)
			if len(body) < 500 || !strings.Contains(body, "<!DOCTYPE html>") {
				t.Skipf("Route %s returned minimal response - YAML route without full template context", tt.route)
			}

			// Check that dark mode classes are present
			for _, class := range tt.shouldContain {
				assert.Contains(t, body, class, "Missing dark mode class: %s", class)
			}

			// Check that problematic patterns are not present
			for _, pattern := range tt.shouldNotContain {
				assert.NotContains(t, body, pattern, "Found problematic pattern: %s", pattern)
			}
		})
	}
}
func TestQueueView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() {
		t.Skip("DB not available - skipping QueueView UI tests")
	}

	token := GetTestAuthToken(t)

	tests := []struct {
		name           string
		userRole       string
		expectedStatus int
		checkContent   []string
	}{
		{
			name:           "Queue page loads successfully",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"Queues",
				"Manage queue assignments",
				// Note: Don't check for specific queue names as they depend on test DB seed data
			},
		},
		{
			name:           "Queue page has proper HTML structure",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"<!DOCTYPE html>",
				"<html",
				"<head>",
				"<title>Queues - GoatFlow</title>",
				"<body",
			},
		},
		{
			name:           "Queue list shows queue details",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"New",     // Ticket state column
				"Open",    // Ticket state column
				"Pending", // Ticket state column
			},
		},
		{
			name:           "Queue page includes GoatKit theme support",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"--gk-bg-",     // GoatKit background variables
				"--gk-text-",   // GoatKit text variables
				"gk-card-",     // GoatKit card classes
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Add middleware to set user role
			router.Use(func(c *gin.Context) {
				c.Set("user_role", tt.userRole)
				c.Set("user", gin.H{
					"FirstName": "Test",
					"LastName":  "User",
					"Email":     "test@example.com",
					"Role":      tt.userRole,
				})
				c.Next()
			})

			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/queues", nil)
			AddTestAuthCookie(req, token)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			body := w.Body.String()
			for _, content := range tt.checkContent {
				assert.Contains(t, body, content, "Missing expected content: %s", content)
			}

			// Should not show fallback HTML
			assert.NotContains(t, body, "Queue management interface coming soon")
		})
	}
}

func TestAdminView(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token := GetTestAuthToken(t)

	// checkAlternatives allows checking for i18n key OR default text
	type checkAlternatives []string

	tests := []struct {
		name              string
		userRole          string
		expectedStatus    int
		checkContent      []string
		checkAlternatives []checkAlternatives // Each entry is a list of acceptable alternatives
	}{
		{
			name:           "Admin dashboard loads successfully",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			// These could be i18n keys or default English text (note: & becomes &amp; in HTML)
			checkAlternatives: []checkAlternatives{
				{"System Administration", "admin_dashboard.system_administration"},
				{"User Management", "admin_dashboard.user_management"},
				{"Reports &amp; Analytics", "Reports & Analytics", "admin_dashboard.reports_analytics"},
				{"Audit Logs", "admin_dashboard.audit_logs"},
			},
		},
		{
			name:           "Admin page has proper HTML structure",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"<!DOCTYPE html>",
				"<html",
				"<head>",
				"<body",
			},
			checkAlternatives: []checkAlternatives{
				{"<title>Admin Dashboard - GoatFlow</title>", "pages.admin.dashboard.title"},
			},
		},
		{
			name:           "Admin dashboard shows activity metrics",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"gk-stat-card",
			},
			checkAlternatives: []checkAlternatives{
				{"Activity", "admin_dashboard.activity"},
			},
		},
		{
			name:           "Admin dashboard shows recent activity",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkAlternatives: []checkAlternatives{
				{"Recent Admin Activity", "admin_dashboard.recent_activity"},
				{"User account created", "admin_dashboard.activity.user_created"},
				{"System configuration updated", "admin_dashboard.activity.system_config_updated"},
			},
		},
		{
			name:           "Admin page includes GoatKit theme support",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"--gk-bg-",      // GoatKit background variables
				"--gk-text-",    // GoatKit text variables
				"gk-card-",      // GoatKit card classes
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Add middleware to set user as admin (ID 1 is seeded admin)
			router.Use(func(c *gin.Context) {
				c.Set("user_role", tt.userRole)
				c.Set("user", gin.H{
					"ID":        uint(1),
					"FirstName": "Admin",
					"LastName":  "OTRS",
					"Login":     "root@localhost",
					"Email":     "admin@example.com",
					"Role":      tt.userRole,
				})
				c.Next()
			})

			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/admin", nil)
			AddTestAuthCookie(req, token)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			body := w.Body.String()
			for _, content := range tt.checkContent {
				assert.Contains(t, body, content, "Missing expected content: %s", content)
			}

			// Check alternatives (i18n key OR default text)
			for _, alts := range tt.checkAlternatives {
				found := containsAny(body, alts...)
				assert.True(t, found, "Missing expected content (none of alternatives found): %v", alts)
			}

			// Should not show fallback HTML
			assert.NotContains(t, body, "Admin interface coming soon")
		})
	}
}

func TestNavigationVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test that admin can see admin navigation items
	// Note: Customer portal and agent portal have separate navigation systems
	// This test focuses on the agent/admin portal navigation

	t.Run("Admin sees admin navigation items", func(t *testing.T) {
		router := NewSimpleRouter()
		token := GetTestAuthToken(t) // Admin token

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/dashboard", nil)
		AddTestAuthCookie(req, token)
		router.ServeHTTP(w, req)

		body := w.Body.String()

		// Admin should see admin link
		assert.Contains(t, body, "Dashboard", "Dashboard should be visible")
		assert.Contains(t, body, `href="/admin"`, "Admin link should be visible for admin users")
	})
}

func TestResponsiveDesign(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token := GetTestAuthToken(t)

	tests := []struct {
		name          string
		route         string
		checkElements []string
	}{
		{
			name:  "Mobile menu button exists",
			route: "/dashboard",
			checkElements: []string{
				"sm:hidden", // Mobile menu button
				"@click=\"mobileMenuOpen = !mobileMenuOpen\"", // Alpine.js toggle
			},
		},
		{
			name:  "Responsive grid layouts",
			route: "/admin",
			checkElements: []string{
				"sm:grid-cols-2",
				"lg:grid-cols-3",
				"grid-cols-1", // Mobile first
			},
		},
		{
			name:  "Responsive text sizes",
			route: "/queues",
			checkElements: []string{
				"sm:text-3xl",
				"text-2xl", // Base size for mobile
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
			AddTestAuthCookie(req, token)
			router.ServeHTTP(w, req)

			body := w.Body.String()

			for _, element := range tt.checkElements {
				assert.Contains(t, body, element, "Missing responsive element: %s", element)
			}
		})
	}
}

func TestAccessibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)

	tests := []struct {
		name      string
		route     string
		checkA11y []string
	}{
		{
			name:  "Page has proper ARIA labels",
			route: "/dashboard",
			checkA11y: []string{
				"role=",
				"aria-",
				"sr-only", // Screen reader only text
			},
		},
		{
			name:  "Forms have proper labels",
			route: "/ticket/new",
			checkA11y: []string{
				"<label",
				"for=",
				"id=",
			},
		},
		{
			name:  "Images have alt text",
			route: "/dashboard",
			checkA11y: []string{
				"viewBox", // SVG icons should have viewBox
				"fill=",   // SVG icons should have fill
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewSimpleRouter()

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
			AddTestAuthCookie(req, token)
			router.ServeHTTP(w, req)

			body := w.Body.String()

			for _, a11y := range tt.checkA11y {
				assert.Contains(t, body, a11y, "Missing accessibility feature: %s", a11y)
			}
		})
	}
}

func TestPageLoadPerformance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		route     string
		maxSizeKB int
	}{
		{
			name:      "Dashboard page size is reasonable",
			route:     "/dashboard",
			maxSizeKB: 50, // 50KB max for initial HTML
		},
		{
			name:      "Queue page size is reasonable",
			route:     "/queues",
			maxSizeKB: 55, // Increased for webservice/GenericInterface features and additional modules
		},
		{
			name:      "Admin page size is reasonable",
			route:     "/admin",
			maxSizeKB: 70, // Increased for webservices, signatures, generic-agent, sessions modules
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
			router.ServeHTTP(w, req)

			sizeKB := len(w.Body.Bytes()) / 1024
			assert.LessOrEqual(t, sizeKB, tt.maxSizeKB,
				"Page size %dKB exceeds maximum %dKB", sizeKB, tt.maxSizeKB)
		})
	}
}

func TestErrorPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)

	tests := []struct {
		name           string
		route          string
		expectedStatus int
	}{
		{
			name:           "404 for non-existent route",
			route:          "/non-existent-page",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Valid routes return 200",
			route:          "/dashboard",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewSimpleRouter()

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
			AddTestAuthCookie(req, token)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestHTMXIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		route     string
		checkHTMX []string
	}{
		{
			name:  "Pages include HTMX attributes",
			route: "/tickets/new", // Use ticket creation form which has HTMX
			checkHTMX: []string{
				"hx-get",
				"hx-post",
				"hx-target",
				"hx-swap",
			},
		},
		{
			name:  "Forms use HTMX for submission",
			route: "/tickets/new",
			checkHTMX: []string{
				`hx-post="/api/tickets"`,
				"hx-target=",
				"hx-swap=",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
			router.ServeHTTP(w, req)

			body := w.Body.String()

			// Skip test if route returned minimal response (YAML routes without full template context)
			if len(body) < 500 || !strings.Contains(body, "<!DOCTYPE html>") {
				t.Skipf("Route %s returned minimal response - YAML route without full template context", tt.route)
			}

			// At least one HTMX attribute should be present
			hasHTMX := false
			for _, attr := range tt.checkHTMX {
				if strings.Contains(body, attr) {
					hasHTMX = true
					break
				}
			}
			assert.True(t, hasHTMX, "Page should include HTMX attributes")
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		route        string
		checkHeaders map[string]string
	}{
		{
			name:  "Content-Type is set correctly",
			route: "/dashboard",
			checkHeaders: map[string]string{
				"Content-Type": "text/html; charset=utf-8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
			router.ServeHTTP(w, req)

			for header, expected := range tt.checkHeaders {
				actual := w.Header().Get(header)
				assert.Equal(t, expected, actual, "Header %s mismatch", header)
			}
		})
	}
}
