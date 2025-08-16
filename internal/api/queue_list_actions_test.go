package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQueueListPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display queue list with action buttons",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should contain the New Queue button with HTMX
				assert.Contains(t, body, "New Queue")
				assert.Contains(t, body, `hx-get="/queues/new"`)
				
				// Should contain queue items
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				
				// Each queue should have Edit and Delete buttons
				assert.Contains(t, body, "Edit")
				assert.Contains(t, body, "Delete")
				
				// Edit buttons should have HTMX attributes
				assert.Contains(t, body, `hx-get="/queues/1/edit"`)
				assert.Contains(t, body, `hx-get="/queues/2/edit"`)
				
				// Delete buttons should have HTMX attributes  
				assert.Contains(t, body, `hx-get="/queues/1/delete"`)
				assert.Contains(t, body, `hx-get="/queues/2/delete"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestNewQueueButtonInteraction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should render new queue form when New Queue button clicked",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should render the create queue modal form
				assert.Contains(t, body, "Create New Queue")
				assert.Contains(t, body, `<form`)
				assert.Contains(t, body, `hx-post="/api/queues"`)
				assert.Contains(t, body, `name="name"`)
				assert.Contains(t, body, `name="comment"`)
				assert.Contains(t, body, "Create Queue")
				assert.Contains(t, body, "Cancel")
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

func TestQueueListEditActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should render edit form when Edit button clicked from list",
			queueID:        "1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should render the edit queue modal form
				assert.Contains(t, body, "Edit Queue: Raw")
				assert.Contains(t, body, `<form`)
				assert.Contains(t, body, `hx-put="/api/queues/1"`)
				assert.Contains(t, body, `value="Raw"`)
				assert.Contains(t, body, "All new tickets are placed in this queue by default")
				assert.Contains(t, body, "Save Changes")
				assert.Contains(t, body, "Cancel")
			},
		},
		{
			name:           "should render edit form for different queue",
			queueID:        "2", 
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should render edit form for Junk queue
				assert.Contains(t, body, "Edit Queue: Junk")
				assert.Contains(t, body, `hx-put="/api/queues/2"`)
				assert.Contains(t, body, `value="Junk"`)
				assert.Contains(t, body, "Spam and junk emails")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/:id/edit", handleEditQueueForm)

			req, _ := http.NewRequest("GET", "/queues/"+tt.queueID+"/edit", nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX request from list
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueListDeleteActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show delete confirmation for empty queue from list",
			queueID:        "3", // Misc queue has no tickets
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show delete confirmation modal
				assert.Contains(t, body, "Delete Queue")
				assert.Contains(t, body, "Are you sure")
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, "cannot be undone")
				
				// Should have delete and cancel buttons
				assert.Contains(t, body, `hx-delete="/api/queues/3"`)
				assert.Contains(t, body, "Delete Queue")
				assert.Contains(t, body, "Cancel")
			},
		},
		{
			name:           "should show warning for queue with tickets from list",
			queueID:        "1", // Raw queue has tickets
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show warning message
				assert.Contains(t, body, "Queue Cannot Be Deleted")
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "contains tickets")
				assert.Contains(t, body, "2 tickets")
				
				// Should NOT have delete button (only close)
				assert.NotContains(t, body, "Delete Queue")
				assert.Contains(t, body, "Close")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/:id/delete", handleDeleteQueueConfirmation)

			req, _ := http.NewRequest("GET", "/queues/"+tt.queueID+"/delete", nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX request from list
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueListRefreshAfterActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		method         string
		endpoint       string
		formData       string
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "should refresh queue list after successful creation",
			method:         "POST",
			endpoint:       "/api/queues",
			formData:       "name=Test+Queue&comment=Test+description",
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should trigger queue list refresh
				assert.Contains(t, w.Header().Get("HX-Trigger"), "queue-created")
				assert.Equal(t, "/queues", w.Header().Get("HX-Redirect"))
			},
		},
		{
			name:           "should refresh queue list after successful update",
			method:         "PUT", 
			endpoint:       "/api/queues/1",
			formData:       "name=Raw+Updated&comment=Updated+description",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should trigger queue list refresh
				assert.Contains(t, w.Header().Get("HX-Trigger"), "queue-updated")
				// For updates, we might redirect to detail or back to list
				assert.NotEmpty(t, w.Header().Get("HX-Redirect"))
			},
		},
		{
			name:           "should refresh queue list after successful deletion",
			method:         "DELETE",
			endpoint:       "/api/queues/3", // Empty queue that can be deleted
			formData:       "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should trigger queue list refresh 
				assert.Contains(t, w.Header().Get("HX-Trigger"), "queue-deleted")
				assert.Equal(t, "/queues", w.Header().Get("HX-Redirect"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			
			// Register the handlers
			router.POST("/api/queues", handleCreateQueueWithHTMX)
			router.PUT("/api/queues/:id", handleUpdateQueueWithHTMX)
			router.DELETE("/api/queues/:id", handleDeleteQueue)

			var req *http.Request
			if tt.formData != "" {
				req, _ = http.NewRequest(tt.method, tt.endpoint, strings.NewReader(tt.formData))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req, _ = http.NewRequest(tt.method, tt.endpoint, nil)
			}
			req.Header.Set("HX-Request", "true") // Simulate HTMX request from list
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestQueueListActions_UIInteractions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		endpoint       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "queue list should have action buttons in proper positions",
			endpoint:       "/queues",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have properly positioned action buttons
				assert.Contains(t, body, "New Queue")
				
				// Each queue item should have action buttons
				// Looking for button containers and proper spacing
				assert.Contains(t, body, "Edit")
				assert.Contains(t, body, "Delete")
				
				// Buttons should have proper styling classes
				assert.Contains(t, body, "inline-flex items-center") // Button styling classes
				
				// Should maintain list structure with actions
				assert.Contains(t, body, "<li>") // List items
				assert.Contains(t, body, "role=\"list\"") // Accessibility
			},
		},
		{
			name:           "queue list always shows New Queue button",
			endpoint:       "/queues",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should always show New Queue button
				assert.Contains(t, body, "New Queue")
				assert.Contains(t, body, `hx-get="/queues/new"`)
				
				// Should have queue management header
				assert.Contains(t, body, "Queue Management")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

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