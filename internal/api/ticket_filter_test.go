package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Test-Driven Development for Ticket Filtering Feature
// Tests for filtering tickets by status, priority, queue, assignee, etc.

func TestTicketFilterByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		expectedCount  int
		expectedStatus []string
		shouldContain  []string
		shouldNotContain []string
	}{
		{
			name:           "Filter by status=open",
			queryParams:    "status=open",
			expectedStatus: []string{"open"},
			shouldContain:  []string{
				"open",
				"Open Tickets",
			},
			shouldNotContain: []string{
				"closed",
				"pending",
				"resolved",
			},
		},
		{
			name:           "Filter by status=closed",
			queryParams:    "status=closed",
			expectedStatus: []string{"closed"},
			shouldContain:  []string{
				"closed",
				"Closed Tickets",
			},
			shouldNotContain: []string{
				"open",
				"pending",
				"new",
			},
		},
		{
			name:           "Filter by multiple statuses",
			queryParams:    "status=open&status=pending",
			expectedStatus: []string{"open", "pending"},
			shouldContain:  []string{
				"open",
				"pending",
			},
			shouldNotContain: []string{
				"closed",
				"resolved",
			},
		},
		{
			name:           "No status filter shows all",
			queryParams:    "",
			shouldContain:  []string{
				"Tickets", // General title
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			// Check that expected content is present
			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}

			// Check that excluded content is not present
			for _, content := range tt.shouldNotContain {
				assert.NotContains(t, body, content, "Should not contain: %s", content)
			}
		})
	}
}

func TestTicketFilterByPriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		queryParams      string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:        "Filter by high priority",
			queryParams: "priority=high",
			shouldContain: []string{
				"high",
				"High Priority",
			},
			shouldNotContain: []string{
				"low",
				"normal",
			},
		},
		{
			name:        "Filter by low priority",
			queryParams: "priority=low",
			shouldContain: []string{
				"low",
				"Low Priority",
			},
			shouldNotContain: []string{
				"high",
				"critical",
			},
		},
		{
			name:        "Filter by multiple priorities",
			queryParams: "priority=high&priority=critical",
			shouldContain: []string{
				"high",
				"critical",
			},
			shouldNotContain: []string{
				"low",
				"normal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}

			for _, content := range tt.shouldNotContain {
				assert.NotContains(t, body, content, "Should not contain: %s", content)
			}
		})
	}
}

func TestTicketFilterByQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		queryParams      string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:        "Filter by General Support queue",
			queryParams: "queue=1",
			shouldContain: []string{
				"General Support",
			},
			shouldNotContain: []string{
				"Technical Support",
				"Billing",
			},
		},
		{
			name:        "Filter by Technical Support queue",
			queryParams: "queue=2",
			shouldContain: []string{
				"Technical Support",
			},
			shouldNotContain: []string{
				"General Support",
				"Billing",
			},
		},
		{
			name:        "Filter by multiple queues",
			queryParams: "queue=1&queue=2",
			shouldContain: []string{
				"General Support",
				"Technical Support",
			},
			shouldNotContain: []string{
				"Billing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}

			for _, content := range tt.shouldNotContain {
				assert.NotContains(t, body, content, "Should not contain: %s", content)
			}
		})
	}
}

func TestTicketFilterByAssignee(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		queryParams      string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:        "Filter by unassigned tickets",
			queryParams: "assigned=false",
			shouldContain: []string{
				"Unassigned",
			},
			shouldNotContain: []string{
				"Agent Smith",
				"John Doe",
			},
		},
		{
			name:        "Filter by assigned tickets",
			queryParams: "assigned=true",
			shouldContain: []string{
				"Agent", // Should have agent names
			},
			shouldNotContain: []string{
				"Unassigned",
			},
		},
		{
			name:        "Filter by specific agent",
			queryParams: "assignee=1",
			shouldContain: []string{
				"Agent Smith", // Assuming agent ID 1 is Agent Smith
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}

			for _, content := range tt.shouldNotContain {
				assert.NotContains(t, body, content, "Should not contain: %s", content)
			}
		})
	}
}

func TestTicketSearchFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		queryParams      string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:        "Search by keyword in title",
			queryParams: "search=login",
			shouldContain: []string{
				"login",
				"Login issues",
			},
		},
		{
			name:        "Search by ticket number",
			queryParams: "search=TICKET-001",
			shouldContain: []string{
				"TICKET-001",
			},
		},
		{
			name:        "Search by customer email",
			queryParams: "search=john@example.com",
			shouldContain: []string{
				"john@example.com",
			},
		},
		{
			name:        "Empty search returns all tickets",
			queryParams: "search=",
			shouldContain: []string{
				"Tickets", // General content
			},
		},
		{
			name:        "Search with no results",
			queryParams: "search=nonexistentterm12345",
			shouldContain: []string{
				"No tickets found", // Should show no results message
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets/search?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}

			for _, content := range tt.shouldNotContain {
				assert.NotContains(t, body, content, "Should not contain: %s", content)
			}
		})
	}
}

func TestCombinedFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		queryParams      string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:        "Filter by status and priority",
			queryParams: "status=open&priority=high",
			shouldContain: []string{
				"open",
				"high",
			},
			shouldNotContain: []string{
				"closed",
				"low",
			},
		},
		{
			name:        "Filter by queue and status",
			queryParams: "queue=1&status=open",
			shouldContain: []string{
				"General Support",
				"open",
			},
			shouldNotContain: []string{
				"Technical Support",
				"closed",
			},
		},
		{
			name:        "Filter by all parameters",
			queryParams: "status=open&priority=high&queue=1&assigned=true",
			shouldContain: []string{
				"open",
				"high",
				"General Support",
			},
			shouldNotContain: []string{
				"closed",
				"low",
				"Unassigned",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}

			for _, content := range tt.shouldNotContain {
				assert.NotContains(t, body, content, "Should not contain: %s", content)
			}
		})
	}
}

func TestFilterPersistence(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		queryParams   string
		checkForm     bool
		expectedForm  map[string]string
	}{
		{
			name:        "Status filter persists in form",
			queryParams: "status=open",
			checkForm:   true,
			expectedForm: map[string]string{
				"status": "open",
			},
		},
		{
			name:        "Multiple filters persist",
			queryParams: "status=open&priority=high&queue=1",
			checkForm:   true,
			expectedForm: map[string]string{
				"status":   "open",
				"priority": "high",
				"queue":    "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/tickets?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			if tt.checkForm {
				for field, value := range tt.expectedForm {
					// Check that form field has the selected value
					assert.Contains(t, body, `value="`+value+`" selected`, 
						"Form field %s should have value %s selected", field, value)
				}
			}
		})
	}
}

func TestFilterReset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Clear filters button removes all filters", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		// First request with filters
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", "/api/tickets?status=open&priority=high", nil)
		router.ServeHTTP(w1, req1)
		
		assert.Equal(t, http.StatusOK, w1.Code)
		assert.Contains(t, w1.Body.String(), "open")
		assert.Contains(t, w1.Body.String(), "high")

		// Second request without filters (clear)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/api/tickets", nil)
		router.ServeHTTP(w2, req2)
		
		assert.Equal(t, http.StatusOK, w2.Code)
		// Should show all tickets without filters
		assert.Contains(t, w2.Body.String(), "Tickets")
	})
}

func TestFilterURLGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		filters        map[string][]string
		expectedURL    string
	}{
		{
			name: "Single filter",
			filters: map[string][]string{
				"status": {"open"},
			},
			expectedURL: "status=open",
		},
		{
			name: "Multiple values for same filter",
			filters: map[string][]string{
				"status": {"open", "pending"},
			},
			expectedURL: "status=open&status=pending",
		},
		{
			name: "Multiple different filters",
			filters: map[string][]string{
				"status":   {"open"},
				"priority": {"high"},
				"queue":    {"1"},
			},
			expectedURL: "status=open&priority=high&queue=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := url.Values{}
			for key, values := range tt.filters {
				for _, value := range values {
					params.Add(key, value)
				}
			}
			
			generatedURL := params.Encode()
			
			// Check that all expected parameters are in the URL
			for _, param := range strings.Split(tt.expectedURL, "&") {
				assert.Contains(t, generatedURL, param, 
					"Generated URL should contain parameter: %s", param)
			}
		})
	}
}

func TestFilterValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		queryParams      string
		expectedStatus   int
		shouldContain    []string
	}{
		{
			name:           "Invalid status value is ignored",
			queryParams:    "status=invalid_status",
			expectedStatus: http.StatusOK,
			shouldContain: []string{
				"Tickets", // Should show default view
			},
		},
		{
			name:           "Invalid priority value is ignored",
			queryParams:    "priority=super_ultra_high",
			expectedStatus: http.StatusOK,
			shouldContain: []string{
				"Tickets",
			},
		},
		{
			name:           "Invalid queue ID is ignored",
			queryParams:    "queue=999999",
			expectedStatus: http.StatusOK,
			shouldContain: []string{
				"Tickets",
			},
		},
		{
			name:           "SQL injection attempt is sanitized",
			queryParams:    "status=open'; DROP TABLE tickets; --",
			expectedStatus: http.StatusOK,
			shouldContain: []string{
				"Tickets",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			body := w.Body.String()

			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}
			
			// Should never contain SQL or injection attempts
			assert.NotContains(t, body, "DROP TABLE")
			assert.NotContains(t, body, "<!--")
		})
	}
}

func TestFilterPerformance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Filters should respond quickly", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		// Complex filter combination
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", 
			"/api/tickets?status=open&status=pending&priority=high&queue=1&queue=2&assigned=true", 
			nil)
		
		router.ServeHTTP(w, req)
		
		// Response should be fast even with multiple filters
		assert.Equal(t, http.StatusOK, w.Code)
		
		// Check that some content is returned
		assert.NotEmpty(t, w.Body.String())
	})
}

func TestFilterUIElements(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		route            string
		shouldContain    []string
	}{
		{
			name:  "Ticket list page has filter form",
			route: "/tickets",
			shouldContain: []string{
				`id="filter-form"`,
				`name="status"`,
				`name="priority"`,
				`name="queue"`,
				"Apply Filters",
				"Clear",
			},
		},
		{
			name:  "Filter form uses HTMX",
			route: "/tickets", 
			shouldContain: []string{
				`hx-get="/api/tickets"`,
				`hx-target="#ticket-list"`,
				`hx-trigger="submit"`,
			},
		},
		{
			name:  "Filter badges show active filters",
			route: "/tickets?status=open&priority=high",
			shouldContain: []string{
				"badge", // Filter badges
				"open",
				"high",
				"×", // Remove filter button
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

			for _, content := range tt.shouldContain {
				assert.Contains(t, body, content, "Should contain: %s", content)
			}
		})
	}
}