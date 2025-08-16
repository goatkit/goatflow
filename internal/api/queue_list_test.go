package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQueueListAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return all active queues",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should contain HTML with queue list
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Support")
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, "Junk")
				
				// Should show ticket counts (HTML formatted)
				assert.Contains(t, body, ">2</span> tickets") // Raw queue has 2 tickets
				assert.Contains(t, body, ">1</span> ticket")  // Junk queue has 1 ticket
				assert.Contains(t, body, ">0</span> tickets") // Misc and Support have 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues", handleQueuesAPI)

			req, _ := http.NewRequest("GET", "/api/queues", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueListJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		acceptHeader   string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return JSON when requested",
			acceptHeader:   "application/json",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool `json:"success"`
					Data    []struct {
						ID          int    `json:"id"`
						Name        string `json:"name"`
						Comment     string `json:"comment"`
						TicketCount int    `json:"ticket_count"`
						Status      string `json:"status"`
					} `json:"data"`
				}
				
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Equal(t, 4, len(response.Data))
				
				// Check specific queue data
				foundRaw := false
				for _, queue := range response.Data {
					if queue.Name == "Raw" {
						foundRaw = true
						assert.Equal(t, 2, queue.TicketCount)
						assert.Equal(t, "active", queue.Status)
					}
				}
				assert.True(t, foundRaw, "Raw queue not found in response")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues", handleQueuesAPI)

			req, _ := http.NewRequest("GET", "/api/queues", nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueListErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupError     bool
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should handle database errors gracefully",
			setupError:     true,
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues", handleQueuesAPI)

			req, _ := http.NewRequest("GET", "/api/queues", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueListFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should filter by status",
			queryParams:    "?status=active",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should only contain active queues
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Support")
			},
		},
		{
			name:           "should search by name",
			queryParams:    "?search=support",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Support")
				assert.NotContains(t, body, "Raw")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues", handleQueuesAPI)

			req, _ := http.NewRequest("GET", "/api/queues"+tt.queryParams, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueListHTMXHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		htmxRequest    bool
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "should return HTML fragment for HTMX requests",
			htmxRequest:    true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
				body := w.Body.String()
				// Should be HTML fragment, not full page
				assert.NotContains(t, body, "<html>")
				assert.NotContains(t, body, "<head>")
				assert.Contains(t, body, "queue")
			},
		},
		{
			name:           "should return full page for regular requests",
			htmxRequest:    false,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues", handleQueuesAPI)

			req, _ := http.NewRequest("GET", "/api/queues", nil)
			if tt.htmxRequest {
				req.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}