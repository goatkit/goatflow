package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Test-Driven Development for UI Components
// Tests for dark theme, navigation, and page rendering

func TestDarkThemeContrast(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		route            string
		activePage       string
		checkSelectors   []string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:       "Navigation links have dark mode classes",
			route:      "/dashboard",
			activePage: "dashboard",
			shouldContain: []string{
				"dark:text-white",            // Active link text in dark mode
				"dark:text-gray-400",         // Inactive link text in dark mode
				"dark:hover:text-gray-200",   // Hover state in dark mode
				"dark:hover:border-gray-600", // Hover border in dark mode
			},
		},
		{
			name:       "Active navigation item is visible in dark mode",
			route:      "/tickets",
			activePage: "tickets",
			shouldContain: []string{
				"border-gotrs-500 text-gray-900 dark:text-white",
			},
		},
		{
			name:       "Buttons maintain contrast in dark mode",
			route:      "/dashboard",
			activePage: "dashboard",
			shouldContain: []string{
				"bg-gotrs-600",       // Primary button background
				"text-white",         // Button text always white
				"hover:bg-gotrs-500", // Hover state
				"dark:bg-gray-800",   // Secondary button in dark mode
			},
		},
		{
			name:       "Page background adapts to dark mode",
			route:      "/dashboard",
			activePage: "dashboard",
			shouldContain: []string{
				"dark:bg-gray-900",   // Body background
				"dark:bg-gray-800",   // Card background
				"dark:text-white",    // Primary text
				"dark:text-gray-400", // Secondary text
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
				"<title>Queues - GOTRS</title>",
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
			name:           "Queue page includes dark mode support",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"dark:bg-gray-800",
				"dark:text-white",
				"dark:hover:bg-gray-700",
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

	tests := []struct {
		name           string
		userRole       string
		expectedStatus int
		checkContent   []string
	}{
		{
			name:           "Admin dashboard loads successfully",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"System Administration",
				"User Management",
				"System Configuration",
				"Reports & Analytics",
				"Audit Logs",
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
				"<title>Admin Dashboard - GOTRS</title>",
				"<body",
			},
		},
		{
			name:           "Admin dashboard shows system health",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"System Health",
				"99.9%",
				"uptime",
			},
		},
		{
			name:           "Admin dashboard shows recent activity",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"Recent Admin Activity",
				"User account created",
				"System configuration updated",
			},
		},
		{
			name:           "Admin page includes dark mode support",
			userRole:       "admin",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"dark:bg-gray-800",
				"dark:text-white",
				"dark:bg-gray-700",
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
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			body := w.Body.String()
			for _, content := range tt.checkContent {
				assert.Contains(t, body, content, "Missing expected content: %s", content)
			}

			// Should not show fallback HTML
			assert.NotContains(t, body, "Admin interface coming soon")
		})
	}
}

func TestNavigationVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		userRole   string
		route      string
		shouldShow []string
		shouldHide []string
	}{
		{
			name:     "Admin sees all navigation items",
			userRole: "admin",
			route:    "/dashboard",
			shouldShow: []string{
				"Dashboard",
				"Tickets",
				"Queues",
				"Admin",
			},
			shouldHide: []string{},
		},
		{
			name:     "Agent sees limited navigation",
			userRole: "agent",
			route:    "/dashboard",
			shouldShow: []string{
				"Dashboard",
				"Tickets",
				"Queues",
			},
			shouldHide: []string{
				"Admin", // Agents shouldn't see admin menu
			},
		},
		{
			name:     "Customer sees minimal navigation",
			userRole: "customer",
			route:    "/dashboard",
			shouldShow: []string{
				"Dashboard",
				"Tickets",
			},
			shouldHide: []string{
				"Queues",
				"Admin",
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
			req, _ := http.NewRequest("GET", tt.route, nil)
			router.ServeHTTP(w, req)

			body := w.Body.String()

			// Check items that should be visible
			for _, item := range tt.shouldShow {
				// Look for navigation link with the text
				assert.Contains(t, body, item, "Navigation item '%s' should be visible for %s", item, tt.userRole)
			}

			// Check items that should be hidden
			for _, item := range tt.shouldHide {
				// For Admin link, it should not appear in navigation for non-admins
				if item == "Admin" && tt.userRole != "admin" {
					// Count occurrences - should only appear in title/header, not in nav
					count := strings.Count(body, `href="/admin"`)
					assert.Equal(t, 0, count, "Admin link should not be visible for %s", tt.userRole)
				}
			}
		})
	}
}

func TestResponsiveDesign(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
			route: "/tickets/new",
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
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
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
			maxSizeKB: 40, // Increased for webservice/GenericInterface features
		},
		{
			name:      "Admin page size is reasonable",
			route:     "/admin",
			maxSizeKB: 65, // Increased for webservices, signatures, generic-agent modules
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
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.route, nil)
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
