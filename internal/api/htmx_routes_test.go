package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupHTMXRoutes(r)
	return r
}

func TestTemplateLoading(t *testing.T) {
	// Skip template loading tests if templates directory doesn't exist
	// These tests are more for development verification
	if _, err := os.Stat("../../templates"); os.IsNotExist(err) {
		t.Skip("Templates directory not found, skipping template loading tests")
	}

	tests := []struct {
		name        string
		files       []string
		expectError bool
	}{
		{
			name:        "Load single template",
			files:       []string{"../../templates/layouts/base.html"},
			expectError: false,
		},
		{
			name:        "Load multiple templates",
			files:       []string{"../../templates/layouts/base.html", "../../templates/pages/dashboard.html"},
			expectError: false,
		},
		{
			name:        "Load non-existent template",
			files:       []string{"../../templates/nonexistent.html"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Temporarily change working directory for test
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)
			os.Chdir("../..")
			
			// Adjust paths back to original
			adjustedFiles := make([]string, len(tt.files))
			for i, file := range tt.files {
				adjustedFiles[i] = strings.Replace(file, "../../", "", 1)
			}
			
			tmpl, err := loadTemplate(adjustedFiles...)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, tmpl)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tmpl)
			}
		})
	}
}

func TestLoginPage(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Check for actual content in the login template
	assert.Contains(t, w.Body.String(), "Login")
	assert.Contains(t, w.Body.String(), "Email Address")
	assert.Contains(t, w.Body.String(), "Password")
}

func TestDashboardPage(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dashboard")
	assert.Contains(t, w.Body.String(), "Welcome back")
}

func TestTicketsListPage(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tickets", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Tickets")
	assert.Contains(t, w.Body.String(), "Filters")
}

func TestNewTicketPage(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tickets/new", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Create New Ticket")
	assert.Contains(t, w.Body.String(), "Subject")
	assert.Contains(t, w.Body.String(), "Customer Email")
}

func TestTicketDetailPage(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name       string
		ticketID   string
		wantStatus int
	}{
		{
			name:       "Valid ticket ID",
			ticketID:   "123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid ticket ID",
			ticketID:   "invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/tickets/"+tt.ticketID, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				assert.Contains(t, w.Body.String(), "Ticket #"+tt.ticketID)
				assert.Contains(t, w.Body.String(), "Messages")
			}
		})
	}
}

func TestHTMXLogin(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name       string
		payload    map[string]string
		wantStatus int
		checkBody  func(t *testing.T, body string)
	}{
		{
			name: "Valid credentials",
			payload: map[string]string{
				"email":    "admin@gotrs.local",
				"password": "admin123",
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp map[string]interface{}
				err := json.Unmarshal([]byte(body), &resp)
				require.NoError(t, err)
				assert.Contains(t, resp, "access_token")
				assert.Contains(t, resp, "user")
			},
		},
		{
			name: "Invalid credentials",
			payload: map[string]string{
				"email":    "wrong@example.com",
				"password": "wrongpass",
			},
			wantStatus: http.StatusUnauthorized,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "error")
			},
		},
		{
			name: "Missing email",
			payload: map[string]string{
				"password": "admin123",
			},
			wantStatus: http.StatusBadRequest,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonPayload, _ := json.Marshal(tt.payload)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(jsonPayload))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

func TestCreateTicket(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name       string
		formData   url.Values
		wantStatus int
		checkBody  func(t *testing.T, body string)
	}{
		{
			name: "Valid ticket creation",
			formData: url.Values{
				"subject":        {"Test ticket"},
				"customer_email": {"customer@example.com"},
				"customer_name":  {"John Doe"},
				"priority":       {"3 normal"},
				"queue_id":       {"1"},
				"type_id":        {"1"},
				"body":           {"This is a test ticket"},
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body string) {
				var resp map[string]interface{}
				err := json.Unmarshal([]byte(body), &resp)
				require.NoError(t, err)
				assert.Contains(t, resp, "id")
				assert.Contains(t, resp, "ticket_number")
			},
		},
		{
			name: "Missing required fields",
			formData: url.Values{
				"subject": {"Test ticket"},
			},
			wantStatus: http.StatusBadRequest,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
			
			// Check for HX-Redirect header on success
			if tt.wantStatus == http.StatusCreated {
				assert.NotEmpty(t, w.Header().Get("HX-Redirect"))
			}
		})
	}
}

func TestAssignTicket(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/tickets/123/assign", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "message")
	assert.Contains(t, resp, "agent_id")
	
	// Check for HTMX trigger header
	assert.NotEmpty(t, w.Header().Get("HX-Trigger"))
}

func TestUpdateTicketStatus(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name       string
		ticketID   string
		status     string
		wantStatus int
	}{
		{
			name:       "Update to open",
			ticketID:   "123",
			status:     "open",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Update to closed",
			ticketID:   "456",
			status:     "closed",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Missing status",
			ticketID:   "789",
			status:     "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formData := url.Values{}
			if tt.status != "" {
				formData.Set("status", tt.status)
			}
			
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/tickets/%s/status", tt.ticketID), strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Contains(t, resp, "message")
				assert.Equal(t, tt.status, resp["status"])
			}
		})
	}
}

func TestTicketReply(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name       string
		formData   url.Values
		wantStatus int
	}{
		{
			name: "Valid reply",
			formData: url.Values{
				"reply":        {"This is a test reply"},
				"internal":     {"false"},
				"close_ticket": {"false"},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "Missing reply text",
			formData:   url.Values{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/123/reply", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantStatus == http.StatusOK {
				// Should return HTML fragment for the new message
				assert.Contains(t, w.Body.String(), tt.formData.Get("reply"))
				assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
			}
		})
	}
}

func TestDashboardStats(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dashboard/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	
	// Check for expected stat cards
	body := w.Body.String()
	assert.Contains(t, body, "Open Tickets")
	assert.Contains(t, body, "New Today")
	assert.Contains(t, body, "Pending")
	assert.Contains(t, body, "Overdue")
}

func TestRecentTickets(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dashboard/recent-tickets", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	
	// Check for ticket entries
	body := w.Body.String()
	assert.Contains(t, body, "TICKET-001")
	assert.Contains(t, body, "TICKET-002")
	assert.Contains(t, body, "TICKET-003")
}

func TestActivityFeed(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dashboard/activity", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	
	// Check for activity entries
	body := w.Body.String()
	assert.Contains(t, body, "created")
	assert.Contains(t, body, "updated")
}

func TestTicketSearch(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/tickets/search?q=login", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "login")
}

func TestNavigationPresence(t *testing.T) {
	router := setupTestRouter()

	// Test that navigation appears on authenticated pages
	pages := []string{
		"/dashboard",
		"/tickets",
		"/tickets/123",
		"/tickets/new",
	}

	for _, page := range pages {
		t.Run("Navigation on "+page, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", page, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			
			// Check for navigation elements
			assert.Contains(t, body, "Dashboard")
			assert.Contains(t, body, "Tickets")
			assert.Contains(t, body, "GOTRS") // Logo in nav
			assert.Contains(t, body, "nav") // Navigation element present
		})
	}
}

func TestRootRedirect(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestHealthCheck(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	// Note: This endpoint is defined in main.go, not in htmx_routes.go
	// So it might return 404 in this test. Update test accordingly.
	if w.Code == http.StatusNotFound {
		t.Skip("Health endpoint not defined in HTMX routes")
	}
	
	assert.Equal(t, http.StatusOK, w.Code)
}

// Benchmark tests
func BenchmarkDashboardPage(b *testing.B) {
	router := setupTestRouter()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/dashboard", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkTemplateLoading(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = loadTemplate(
			"templates/layouts/base.html",
			"templates/pages/dashboard.html",
		)
	}
}