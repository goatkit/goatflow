package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAgentDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show agent dashboard with key metrics",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Check for key dashboard sections
				assert.Contains(t, body, "Agent Dashboard")
				assert.Contains(t, body, "My Open Tickets")
				assert.Contains(t, body, "Queue Overview")
				assert.Contains(t, body, "Recent Activity")
				assert.Contains(t, body, "Performance Metrics")
			},
		},
		{
			name:           "should display ticket statistics",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Ticket counts
				assert.Contains(t, body, `data-metric="open-tickets"`)
				assert.Contains(t, body, `data-metric="pending-tickets"`)
				assert.Contains(t, body, `data-metric="resolved-today"`)
				assert.Contains(t, body, `data-metric="avg-response-time"`)
			},
		},
		{
			name:           "should show assigned tickets list",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Assigned to Me")
				assert.Contains(t, body, "TICK-2024")
				assert.Contains(t, body, "Priority:")
				assert.Contains(t, body, "Due:")
			},
		},
		{
			name:           "should display queue performance",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Queue Performance")
				assert.Contains(t, body, "Support Queue")
				assert.Contains(t, body, "Sales Queue")
				assert.Contains(t, body, "tickets in queue")
			},
		},
		{
			name:           "should have SSE connection endpoint",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Check for SSE setup script
				assert.Contains(t, body, `new EventSource('/dashboard/events')`)
				assert.Contains(t, body, `data-sse-target`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/dashboard/agent", handleAgentDashboard)

			req, _ := http.NewRequest("GET", "/dashboard/agent", nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestAgentDashboardMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		endpoint       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return open tickets count",
			endpoint:       "/dashboard/metrics/open-tickets",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.NotNil(t, response["count"])
				assert.NotNil(t, response["trend"])
			},
		},
		{
			name:           "should return response time metrics",
			endpoint:       "/dashboard/metrics/response-time",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.NotNil(t, response["average"])
				assert.NotNil(t, response["median"])
				assert.NotNil(t, response["p95"])
			},
		},
		{
			name:           "should return SLA compliance",
			endpoint:       "/dashboard/metrics/sla-compliance",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.NotNil(t, response["compliance_rate"])
				assert.NotNil(t, response["at_risk"])
				assert.NotNil(t, response["breached"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/dashboard/metrics/:type", handleDashboardMetrics)

			req, _ := http.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestDashboardSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Simple test that SSE endpoint returns correct headers and some data
	router := gin.New()
	router.GET("/dashboard/events", handleDashboardSSE)
	
	req, _ := http.NewRequest("GET", "/dashboard/events", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	// Check headers
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	
	// Check that we got some SSE data
	body := w.Body.String()
	assert.Contains(t, body, "data:")
	assert.Contains(t, body, "ticket_updated")
	assert.Contains(t, body, "queue_status")
	assert.Contains(t, body, "metrics_update")
	assert.Contains(t, body, "heartbeat")
}


func TestDashboardActivityFeed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show recent activity",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Recent Activity")
				assert.Contains(t, body, "ticket created")
				assert.Contains(t, body, "status changed")
				assert.Contains(t, body, "assigned to")
				assert.Contains(t, body, "ago")
			},
		},
		{
			name:           "should filter by activity type",
			query:          "?type=ticket_created",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "ticket created")
				assert.NotContains(t, body, "status changed")
			},
		},
		{
			name:           "should paginate activity",
			query:          "?page=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "page=1")
				assert.Contains(t, body, "page=3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/dashboard/activity", handleDashboardActivity)

			req, _ := http.NewRequest("GET", "/dashboard/activity"+tt.query, nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestDashboardNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show notification bell with count",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, `data-notification-count`)
				assert.Contains(t, body, `class="notification-bell"`)
				assert.Contains(t, body, `class="notification-badge"`)
			},
		},
		{
			name:           "should list unread notifications",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "New ticket assigned")
				assert.Contains(t, body, "SLA warning")
				assert.Contains(t, body, "Customer response")
				assert.Contains(t, body, "unread")
			},
		},
		{
			name:           "should have mark as read functionality",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, `hx-post="/notifications/mark-read"`)
				assert.Contains(t, body, `Mark all as read`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/dashboard/notifications", handleDashboardNotifications)

			req, _ := http.NewRequest("GET", "/dashboard/notifications", nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestDashboardQuickActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display quick action buttons",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Quick Actions")
				assert.Contains(t, body, "New Ticket")
				assert.Contains(t, body, "Search Tickets")
				assert.Contains(t, body, "My Profile")
				assert.Contains(t, body, "Reports")
			},
		},
		{
			name:           "should have keyboard shortcuts",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, `data-shortcut="n"`) // New ticket
				assert.Contains(t, body, `data-shortcut="/"`) // Search
				assert.Contains(t, body, `data-shortcut="g d"`) // Go to dashboard
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/dashboard/quick-actions", handleQuickActions)

			req, _ := http.NewRequest("GET", "/dashboard/quick-actions", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}