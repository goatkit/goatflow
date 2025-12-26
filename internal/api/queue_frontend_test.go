//go:build db

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestEditQueueForm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return edit queue form",
			queueID:        "1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should contain form elements for editing
				assert.Contains(t, body, `<form`)
				assert.Contains(t, body, `name="name"`)
				assert.Contains(t, body, `name="comment"`)
				assert.Contains(t, body, `name="system_address"`)

				// Should be pre-populated with existing data
				assert.Contains(t, body, `value="Raw"`)
				assert.Contains(t, body, "All new tickets are placed in this queue by default")

				// Should have submit button
				assert.Contains(t, body, `type="submit"`)
				assert.Contains(t, body, "Save Changes")

				// Should have cancel button
				assert.Contains(t, body, "Cancel")
			},
		},
		{
			name:           "should return 404 for non-existent queue",
			queueID:        "999",
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue not found")
			},
		},
		{
			name:           "should return 400 for invalid queue ID",
			queueID:        "invalid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "invalid queue id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/:id/edit", handleEditQueueForm)

			req, _ := http.NewRequest("GET", "/queues/"+tt.queueID+"/edit", nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestCreateQueueForm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return create queue form",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should contain form elements
				assert.Contains(t, body, `<form`)
				assert.Contains(t, body, `name="name"`)
				assert.Contains(t, body, `name="comment"`)
				assert.Contains(t, body, `name="system_address"`)
				assert.Contains(t, body, `name="first_response_time"`)

				// Should have form action pointing to create endpoint
				assert.Contains(t, body, `hx-post="/api/queues"`)

				// Should have submit button
				assert.Contains(t, body, `type="submit"`)
				assert.Contains(t, body, "Create Queue")

				// Should have cancel button
				assert.Contains(t, body, "Cancel")

				// Form should be empty (no pre-filled values)
				assert.NotContains(t, body, `value="Raw"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/new", handleNewQueueForm)

			req, _ := http.NewRequest("GET", "/queues/new", nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestDeleteQueueConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return delete confirmation for empty queue",
			queueID:        "3", // Misc queue has no tickets
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show queue name in confirmation
				assert.Contains(t, body, "Misc")

				// Should warn about deletion
				assert.Contains(t, body, "Are you sure")
				assert.Contains(t, body, "delete")

				// Should have confirm and cancel buttons
				assert.Contains(t, body, "Delete Queue")
				assert.Contains(t, body, "Cancel")

				// Should use HTMX to call delete API
				assert.Contains(t, body, `hx-delete="/api/queues/3"`)
			},
		},
		{
			name:           "should show warning for queue with tickets",
			queueID:        "1", // Raw queue has tickets
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show queue name
				assert.Contains(t, body, "Raw")

				// Should warn about tickets
				assert.Contains(t, body, "cannot be deleted")
				assert.Contains(t, body, "contains tickets")
				assert.Contains(t, body, "2 tickets")

				// Should NOT have delete button (only close)
				assert.NotContains(t, body, "Delete Queue")
				assert.Contains(t, body, "Close")
			},
		},
		{
			name:           "should return 404 for non-existent queue",
			queueID:        "999",
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/:id/delete", handleDeleteQueueConfirmation)

			req, _ := http.NewRequest("GET", "/queues/"+tt.queueID+"/delete", nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueFormSubmission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		endpoint       string
		method         string
		formData       string
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "should handle successful form submission with HTMX response",
			endpoint:       "/api/queues",
			method:         "POST",
			formData:       "name=New+Queue&comment=Test+queue&system_address=test@example.com",
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should return HTMX response headers for success
				assert.Contains(t, w.Header().Get("HX-Trigger"), "queue-created")

				// Should redirect or show success message
				body := w.Body.String()
				if w.Header().Get("HX-Redirect") != "" {
					assert.NotEmpty(t, w.Header().Get("HX-Redirect"))
				} else {
					assert.Contains(t, body, "success")
				}
			},
		},
		{
			name:           "should handle form validation errors with HTMX",
			endpoint:       "/api/queues",
			method:         "POST",
			formData:       "comment=Missing+name+field", // Missing required name
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				body := w.Body.String()
				// Should return form with validation errors highlighted
				assert.Contains(t, body, "error")
				assert.Contains(t, strings.ToLower(body), "name")
				assert.Contains(t, strings.ToLower(body), "required")
			},
		},
		{
			name:           "should handle edit queue form submission",
			endpoint:       "/api/queues/1",
			method:         "PUT",
			formData:       "name=Raw+Updated&comment=Updated+description",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should return HTMX response for successful update
				assert.Contains(t, w.Header().Get("HX-Trigger"), "queue-updated")

				body := w.Body.String()
				if w.Header().Get("HX-Redirect") != "" {
					assert.NotEmpty(t, w.Header().Get("HX-Redirect"))
				} else {
					assert.Contains(t, body, "success")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Register both API endpoints and form handlers
			router.POST("/api/queues", handleCreateQueueWithHTMX)
			router.PUT("/api/queues/:id", handleUpdateQueueWithHTMX)

			req, _ := http.NewRequest(tt.method, tt.endpoint, strings.NewReader(tt.formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true") // Simulate HTMX form submission
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestQueuePaginationNavigation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		queryParams    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should handle pagination with HTMX",
			queueID:        "1",
			queryParams:    "?page=1&limit=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show pagination controls
				assert.Contains(t, body, "pagination")

				// Should show correct page info
				assert.Contains(t, body, "page 1")

				// Should have working HTMX navigation links
				assert.Contains(t, body, `hx-get`)
				assert.Contains(t, body, "page=2")

				// Should show only 1 ticket (due to limit=1)
				ticketCount := strings.Count(body, "TICKET-")
				assert.Equal(t, 1, ticketCount, "Should show exactly 1 ticket per page")
			},
		},
		{
			name:           "should handle last page correctly",
			queueID:        "1",
			queryParams:    "?page=2&limit=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show pagination controls
				assert.Contains(t, body, "pagination")

				// Should show second page
				assert.Contains(t, body, "page 2")

				// Should have previous button but no next button
				assert.Contains(t, body, "page=1")    // Previous page link
				assert.NotContains(t, body, "page=3") // No next page
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/:id/tickets", handleQueueTicketsWithHTMX)

			req, _ := http.NewRequest("GET", "/queues/"+tt.queueID+"/tickets"+tt.queryParams, nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX pagination click
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}
