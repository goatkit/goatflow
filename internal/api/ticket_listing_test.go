package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTicketListSorting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should sort by creation date descending (default)",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Most recent tickets should appear first
				ticket1Pos := strings.Index(body, "TICK-2024-001")
				ticket2Pos := strings.Index(body, "TICK-2024-002")
				if ticket1Pos != -1 && ticket2Pos != -1 {
					assert.True(t, ticket2Pos < ticket1Pos, "Newer ticket should appear first")
				}
			},
		},
		{
			name:           "should sort by priority",
			query:          "?sort=priority",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Urgent should appear before high, high before normal, etc.
				urgentPos := strings.Index(body, `class="priority-urgent"`)
				highPos := strings.Index(body, `class="priority-high"`)
				normalPos := strings.Index(body, `class="priority-normal"`)
				
				if urgentPos != -1 && highPos != -1 {
					assert.True(t, urgentPos < highPos, "Urgent should appear before high")
				}
				if highPos != -1 && normalPos != -1 {
					assert.True(t, highPos < normalPos, "High should appear before normal")
				}
			},
		},
		{
			name:           "should sort by status",
			query:          "?sort=status",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Open tickets should appear before closed
				openPos := strings.Index(body, `class="status-open"`)
				closedPos := strings.Index(body, `class="status-closed"`)
				
				if openPos != -1 && closedPos != -1 {
					assert.True(t, openPos < closedPos, "Open should appear before closed")
				}
			},
		},
		{
			name:           "should sort by last updated",
			query:          "?sort=updated",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Recently updated tickets should appear first
				assert.Contains(t, body, "Last updated")
			},
		},
		{
			name:           "should sort by title alphabetically",
			query:          "?sort=title",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Tickets should be in alphabetical order by title
				assert.Contains(t, body, "Title")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets", handleTicketsList)

			req, _ := http.NewRequest("GET", "/tickets"+tt.query, nil)
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

func TestTicketSearchFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should search by ticket number",
			query:          "?search=TICK-2024-001",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "TICK-2024-001")
				assert.NotContains(t, body, "TICK-2024-002")
			},
		},
		{
			name:           "should search by title",
			query:          "?search=server",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Server")
				// Should highlight search term
				assert.Contains(t, body, `<mark>server</mark>`)
			},
		},
		{
			name:           "should search by customer email",
			query:          "?search=customer@example.com",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "customer@example.com")
			},
		},
		{
			name:           "should show no results message",
			query:          "?search=nonexistent",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "No tickets found")
				assert.Contains(t, body, "Try adjusting your search")
			},
		},
		{
			name:           "should maintain search across pagination",
			query:          "?search=test&page=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Pagination links should preserve search
				assert.Contains(t, body, "search=test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets", handleTicketsList)

			req, _ := http.NewRequest("GET", "/tickets"+tt.query, nil)
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

func TestTicketListPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show first page by default",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Page 1")
				assert.Contains(t, body, "Next")
				assert.NotContains(t, body, "Previous")
			},
		},
		{
			name:           "should navigate to page 2",
			query:          "?page=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Page 2")
				assert.Contains(t, body, "Previous")
				assert.Contains(t, body, "Next")
			},
		},
		{
			name:           "should allow changing items per page",
			query:          "?per_page=25",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, `value="25" selected`)
				// Should show up to 25 items
			},
		},
		{
			name:           "should show page info",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Showing")
				assert.Contains(t, body, "of")
				assert.Contains(t, body, "tickets")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets", handleTicketsList)

			req, _ := http.NewRequest("GET", "/tickets"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketStatusIndicators(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show status badges",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Status badges with colors
				assert.Contains(t, body, `class="badge badge-new"`)
				assert.Contains(t, body, `class="badge badge-open"`)
				assert.Contains(t, body, `class="badge badge-pending"`)
				assert.Contains(t, body, `class="badge badge-resolved"`)
				assert.Contains(t, body, `class="badge badge-closed"`)
			},
		},
		{
			name:           "should show priority indicators",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Priority indicators with colors
				assert.Contains(t, body, `class="priority-urgent"`)
				assert.Contains(t, body, `class="priority-high"`)
				assert.Contains(t, body, `class="priority-normal"`)
				assert.Contains(t, body, `class="priority-low"`)
			},
		},
		{
			name:           "should show SLA status",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// SLA indicators
				assert.Contains(t, body, "Due in")
				assert.Contains(t, body, `class="sla-warning"`)
				assert.Contains(t, body, `class="sla-breach"`)
			},
		},
		{
			name:           "should show unread indicator",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Unread message indicator
				assert.Contains(t, body, `class="unread-indicator"`)
				assert.Contains(t, body, "New message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets", handleTicketsList)

			req, _ := http.NewRequest("GET", "/tickets", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketBulkActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		formData       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should bulk assign tickets",
			formData:       "ticket_ids=1,2,3&action=assign&agent_id=5",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "3 tickets assigned")
			},
		},
		{
			name:           "should bulk close tickets",
			formData:       "ticket_ids=1,2&action=close",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "2 tickets closed")
			},
		},
		{
			name:           "should bulk change priority",
			formData:       "ticket_ids=1,2,3,4&action=set_priority&priority=high",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Priority updated for 4 tickets")
			},
		},
		{
			name:           "should bulk move to queue",
			formData:       "ticket_ids=1,2&action=move_queue&queue_id=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "2 tickets moved")
			},
		},
		{
			name:           "should reject invalid bulk action",
			formData:       "ticket_ids=1&action=invalid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid action")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/tickets/bulk-action", handleTicketBulkAction)

			req, _ := http.NewRequest("POST", "/tickets/bulk-action", strings.NewReader(tt.formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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