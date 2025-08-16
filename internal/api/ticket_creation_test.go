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

func TestTicketCreationForm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display ticket creation form",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have form elements
				assert.Contains(t, body, `<form`)
				assert.Contains(t, body, `id="ticket-form"`)
				
				// Should have title field
				assert.Contains(t, body, `name="title"`)
				assert.Contains(t, body, `placeholder="Brief description of the issue"`)
				
				// Should have queue selection
				assert.Contains(t, body, `name="queue_id"`)
				assert.Contains(t, body, `<option value="1">Raw</option>`)
				
				// Should have priority selection
				assert.Contains(t, body, `name="priority"`)
				assert.Contains(t, body, `value="low"`)
				assert.Contains(t, body, `value="normal"`)
				assert.Contains(t, body, `value="high"`)
				assert.Contains(t, body, `value="urgent"`)
				
				// Should have description textarea
				assert.Contains(t, body, `name="description"`)
				assert.Contains(t, body, `<textarea`)
				
				// Should have customer email field
				assert.Contains(t, body, `name="customer_email"`)
				assert.Contains(t, body, `type="email"`)
				
				// Should have submit button
				assert.Contains(t, body, `type="submit"`)
				assert.Contains(t, body, "Create Ticket")
				
				// Should have HTMX attributes
				assert.Contains(t, body, `hx-post="/tickets/create"`)
				assert.Contains(t, body, `hx-target`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets/new", handleTicketNew)

			req, _ := http.NewRequest("GET", "/tickets/new", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketCreationValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		formData       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should reject empty title",
			formData:       "title=&queue_id=1&priority=normal&description=Test",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Title is required")
			},
		},
		{
			name:           "should reject title longer than 200 characters",
			formData:       "title=" + strings.Repeat("a", 201) + "&queue_id=1&priority=normal",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Title must be less than 200 characters")
			},
		},
		{
			name:           "should reject missing queue",
			formData:       "title=Test&priority=normal&description=Test",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Queue selection is required")
			},
		},
		{
			name:           "should reject invalid priority",
			formData:       "title=Test&queue_id=1&priority=invalid&description=Test",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid priority")
			},
		},
		{
			name:           "should reject invalid email format",
			formData:       "title=Test&queue_id=1&priority=normal&customer_email=notanemail",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid email format")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/tickets/create", handleTicketCreate)

			req, _ := http.NewRequest("POST", "/tickets/create", strings.NewReader(tt.formData))
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

func TestTicketCreationSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		formData       url.Values
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name: "should create ticket with all fields",
			formData: url.Values{
				"title":          {"Server is down"},
				"queue_id":       {"1"},
				"priority":       {"high"},
				"description":    {"Production server is not responding"},
				"customer_email": {"customer@example.com"},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show success message
				assert.Contains(t, body, "Ticket created successfully")
				assert.Contains(t, body, "Ticket #")
			},
			checkHeaders: func(t *testing.T, headers http.Header) {
				// Should trigger list refresh
				assert.Contains(t, headers.Get("HX-Trigger"), "ticket-created")
			},
		},
		{
			name: "should create ticket with minimal fields",
			formData: url.Values{
				"title":    {"Simple issue"},
				"queue_id": {"1"},
				"priority": {"normal"},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Ticket created successfully")
			},
		},
		{
			name: "should auto-assign to current agent if specified",
			formData: url.Values{
				"title":       {"Assigned ticket"},
				"queue_id":    {"1"},
				"priority":    {"normal"},
				"auto_assign": {"true"},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Ticket created and assigned to you")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/tickets/create", handleTicketCreate)

			req, _ := http.NewRequest("POST", "/tickets/create", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
			if tt.checkHeaders != nil {
				tt.checkHeaders(t, w.Header())
			}
		})
	}
}

func TestTicketListDisplay(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display ticket list",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have ticket list container
				assert.Contains(t, body, `id="ticket-list"`)
				
				// Should display ticket information
				assert.Contains(t, body, "Ticket #")
				assert.Contains(t, body, "Title")
				assert.Contains(t, body, "Queue")
				assert.Contains(t, body, "Priority")
				assert.Contains(t, body, "Status")
				assert.Contains(t, body, "Created")
				assert.Contains(t, body, "Updated")
				
				// Should have action buttons
				assert.Contains(t, body, "View")
				assert.Contains(t, body, "Edit")
			},
		},
		{
			name:           "should show empty state when no tickets",
			query:          "?status=archived",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "No tickets found")
				assert.Contains(t, body, "Create your first ticket")
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

func TestTicketListFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should filter by status",
			query:          "?status=open",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should only show open tickets
				assert.Contains(t, body, `class="status-open"`)
				assert.NotContains(t, body, `class="status-closed"`)
			},
		},
		{
			name:           "should filter by priority",
			query:          "?priority=high",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should only show high priority tickets
				assert.Contains(t, body, `class="priority-high"`)
				assert.NotContains(t, body, `class="priority-low"`)
			},
		},
		{
			name:           "should filter by queue",
			query:          "?queue_id=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should only show tickets from specified queue
				assert.Contains(t, body, "Raw")
				assert.NotContains(t, body, "Support")
			},
		},
		{
			name:           "should filter by assigned agent",
			query:          "?assigned_to=me",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should only show tickets assigned to current user
				assert.Contains(t, body, "Assigned to you")
			},
		},
		{
			name:           "should support multiple filters",
			query:          "?status=open&priority=high&queue_id=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should apply all filters
				assert.Contains(t, body, `class="status-open"`)
				assert.Contains(t, body, `class="priority-high"`)
				assert.Contains(t, body, "Raw")
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

func TestTicketQuickActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		ticketID       string
		action         string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should quick assign ticket",
			ticketID:       "1",
			action:         "assign",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Ticket assigned")
			},
		},
		{
			name:           "should quick close ticket",
			ticketID:       "1",
			action:         "close",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Ticket closed")
			},
		},
		{
			name:           "should quick update priority",
			ticketID:       "1",
			action:         "priority-high",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Priority updated")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/tickets/:id/quick-action", handleTicketQuickAction)

			req, _ := http.NewRequest("POST", "/tickets/"+tt.ticketID+"/quick-action", 
				strings.NewReader("action="+tt.action))
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