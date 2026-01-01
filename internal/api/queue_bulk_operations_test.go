package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQueueBulkSelectionInterface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display bulk selection checkboxes",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have select all checkbox
				assert.Contains(t, body, `type="checkbox"`)
				assert.Contains(t, body, `id="select-all-queues"`)
				assert.Contains(t, body, `Select All`)

				// Each queue should have a selection checkbox
				assert.Contains(t, body, `name="queue-select"`)
				assert.Contains(t, body, `value="1"`) // Queue ID as value
				assert.Contains(t, body, `value="2"`)
				assert.Contains(t, body, `value="3"`)

				// Should have bulk actions toolbar (hidden by default)
				assert.Contains(t, body, `id="bulk-actions-toolbar"`)
				assert.Contains(t, body, `style="display: none"`) // Hidden initially
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

func TestQueueBulkActionsToolbar(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		selectedCount  string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show bulk actions when queues selected",
			selectedCount:  "2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show selection count
				assert.Contains(t, body, "2 queues selected")

				// Should show bulk action buttons
				assert.Contains(t, body, "Activate Selected")
				assert.Contains(t, body, "Deactivate Selected")
				assert.Contains(t, body, "Delete Selected")

				// Delete button should have warning style
				assert.Contains(t, body, "bg-red-600")

				// Should have cancel selection button
				assert.Contains(t, body, "Cancel Selection")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/bulk-toolbar", handleBulkActionsToolbar)

			req, _ := http.NewRequest("GET", "/queues/bulk-toolbar?count="+tt.selectedCount, nil)
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

func TestQueueBulkStatusChange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		action         string
		queueIDs       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should activate multiple queues",
			action:         "activate",
			queueIDs:       "queue_ids=2&queue_ids=3", // Junk and Misc queues
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
					Updated int    `json:"updated"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Equal(t, 2, response.Updated)
				assert.Contains(t, response.Message, "2 queues activated")
			},
		},
		{
			name:           "should deactivate multiple queues",
			action:         "deactivate",
			queueIDs:       "queue_ids=1&queue_ids=4", // Raw and Support queues
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
					Updated int    `json:"updated"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Equal(t, 2, response.Updated)
				assert.Contains(t, response.Message, "2 queues deactivated")
			},
		},
		{
			name:           "should handle invalid action",
			action:         "invalid",
			queueIDs:       "queue_ids=1",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Error string `json:"error"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Error, "Invalid action")
			},
		},
		{
			name:           "should handle missing queue IDs",
			action:         "activate",
			queueIDs:       "",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Error string `json:"error"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Error, "No queues selected")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/queues/bulk/:action", handleBulkQueueAction)

			req, _ := http.NewRequest("PUT", "/api/queues/bulk/"+tt.action, strings.NewReader(tt.queueIDs))
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

func TestQueueBulkDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueIDs       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should delete multiple empty queues",
			queueIDs:       "queue_ids=3", // Misc queue (empty)
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Debug: print the response body
				t.Logf("Response body: %s", body)

				var response struct {
					Success bool     `json:"success"`
					Message string   `json:"message"`
					Deleted int      `json:"deleted"`
					Skipped []string `json:"skipped"`
					Error   string   `json:"error"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				if response.Error != "" {
					t.Errorf("Got error: %s", response.Error)
				}
				assert.True(t, response.Success)
				assert.Equal(t, 1, response.Deleted)
				assert.Empty(t, response.Skipped)
				assert.Contains(t, response.Message, "1 queue deleted")
			},
		},
		{
			name:           "should skip queues with tickets",
			queueIDs:       "queue_ids=1&queue_ids=3", // Raw (has tickets) and Misc (empty)
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool     `json:"success"`
					Message string   `json:"message"`
					Deleted int      `json:"deleted"`
					Skipped []string `json:"skipped"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Equal(t, 1, response.Deleted) // Only Misc deleted
				assert.Equal(t, 1, len(response.Skipped))
				assert.Contains(t, response.Skipped[0], "Raw")
				assert.Contains(t, response.Message, "1 queue deleted, 1 skipped")
			},
		},
		{
			name:           "should handle all queues with tickets",
			queueIDs:       "queue_ids=1&queue_ids=2", // Both have tickets
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool     `json:"success"`
					Error   string   `json:"error"`
					Skipped []string `json:"skipped"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.False(t, response.Success)
				assert.Equal(t, 2, len(response.Skipped))
				assert.Contains(t, response.Error, "No queues could be deleted")
			},
		},
		{
			name:           "should require confirmation for bulk delete",
			queueIDs:       "queue_ids=3&confirm=false",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Error string `json:"error"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Error, "Confirmation required")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.DELETE("/api/queues/bulk", handleBulkQueueDelete)

			// Add confirm=true unless test specifies otherwise
			queueIDs := tt.queueIDs
			if !strings.Contains(queueIDs, "confirm=") {
				if queueIDs != "" {
					queueIDs += "&"
				}
				queueIDs += "confirm=true"
			}

			t.Logf("Sending form data: %s", queueIDs)

			req, _ := http.NewRequest("DELETE", "/api/queues/bulk", strings.NewReader(queueIDs))
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

func TestQueueBulkSelectionJavaScript(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should include bulk selection JavaScript",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have JavaScript for select all functionality
				assert.Contains(t, body, "selectAllQueues")
				assert.Contains(t, body, "updateBulkToolbar")

				// Should handle individual checkbox changes
				assert.Contains(t, body, "queue-select")
				assert.Contains(t, body, "addEventListener")

				// Should update toolbar visibility
				assert.Contains(t, body, "bulk-actions-toolbar")
				assert.Contains(t, body, "toolbar.style.display = 'block'")
				assert.Contains(t, body, "display: none")
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

func TestQueueBulkOperationsFeedback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		action         string
		queueIDs       string
		expectedStatus int
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name:           "should trigger list refresh after bulk activate",
			action:         "activate",
			queueIDs:       "queue_ids=2&queue_ids=3",
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				// Should trigger HTMX events for UI update
				assert.Contains(t, headers.Get("HX-Trigger"), "queues-updated")
				assert.Contains(t, headers.Get("HX-Trigger"), "show-toast")
			},
		},
		{
			name:           "should trigger list refresh after bulk delete",
			action:         "delete",
			queueIDs:       "queue_ids=3&confirm=true",
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				// Should trigger refresh and show success message
				assert.Contains(t, headers.Get("HX-Trigger"), "queues-updated")
				assert.Contains(t, headers.Get("HX-Trigger"), "show-toast")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			if tt.action == "delete" {
				router.DELETE("/api/queues/bulk", handleBulkQueueDelete)
			} else {
				router.PUT("/api/queues/bulk/:action", handleBulkQueueAction)
			}

			var req *http.Request
			if tt.action == "delete" {
				req, _ = http.NewRequest("DELETE", "/api/queues/bulk", strings.NewReader(tt.queueIDs))
			} else {
				req, _ = http.NewRequest("PUT", "/api/queues/bulk/"+tt.action, strings.NewReader(tt.queueIDs))
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkHeaders != nil {
				tt.checkHeaders(t, w.Header())
			}
		})
	}
}

func TestQueueBulkOperationsEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		method         string
		endpoint       string
		body           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should handle invalid queue IDs gracefully",
			method:         "PUT",
			endpoint:       "/api/queues/bulk/activate",
			body:           "queue_ids=999&queue_ids=invalid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Error string `json:"error"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Error, "Invalid queue ID")
			},
		},
		{
			name:           "should limit bulk operations to reasonable number",
			method:         "PUT",
			endpoint:       "/api/queues/bulk/activate",
			body:           generateManyQueueIDs(101), // Try to select more than 100
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Error string `json:"error"`
				}
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Error, "Too many queues selected")
				assert.Contains(t, response.Error, "maximum 100")
			},
		},
		{
			name:           "should handle empty action gracefully",
			method:         "PUT",
			endpoint:       "/api/queues/bulk/",
			body:           "queue_ids=1",
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				// Router should return 404 for empty action
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/queues/bulk/:action", handleBulkQueueAction)
			router.DELETE("/api/queues/bulk", handleBulkQueueDelete)

			req, _ := http.NewRequest(tt.method, tt.endpoint, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

// Helper function to generate many queue IDs for testing limits.
func generateManyQueueIDs(count int) string {
	var ids []string
	for i := 1; i <= count; i++ {
		ids = append(ids, "queue_ids="+strconv.Itoa(i))
	}
	return strings.Join(ids, "&")
}
