//go:build db

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

func TestQueueDetailAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() {
		t.Skip("DB not available - skipping queue detail API tests")
	}

	tests := []struct {
		name           string
		queueID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return queue details with tickets",
			queueID:        "1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should contain queue info and associated tickets
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "All new tickets are placed in this queue by default")
				assert.Contains(t, body, ">2</span> tickets") // Raw queue has 2 tickets (HTML formatted)
			},
		},
		{
			name:           "should return empty queue details",
			queueID:        "3",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show Misc queue with 0 tickets
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, ">0</span> tickets")
				assert.Contains(t, body, "No tickets in this queue")
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
			router.GET("/api/queues/:id", handleQueueDetailJSON)

			req, _ := http.NewRequest("GET", "/api/queues/"+tt.queueID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueDetailJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() {
		t.Skip("DB not available - skipping queue JSON tests")
	}

	tests := []struct {
		name           string
		queueID        string
		acceptHeader   string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return JSON queue details",
			queueID:        "1",
			acceptHeader:   "application/json",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool `json:"success"`
					Data    struct {
						ID          int    `json:"id"`
						Name        string `json:"name"`
						Comment     string `json:"comment"`
						TicketCount int    `json:"ticket_count"`
						Status      string `json:"status"`
						Tickets     []struct {
							ID     int    `json:"id"`
							Number string `json:"number"`
							Title  string `json:"title"`
							Status string `json:"status"`
						} `json:"tickets"`
					} `json:"data"`
				}

				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Equal(t, 1, response.Data.ID)
				assert.Equal(t, "Raw", response.Data.Name)
				assert.Equal(t, 2, response.Data.TicketCount)
				assert.Equal(t, 2, len(response.Data.Tickets))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues/:id", handleQueueDetailJSON)

			req, _ := http.NewRequest("GET", "/api/queues/"+tt.queueID, nil)
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

func TestQueueTicketsAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() {
		t.Skip("DB not available - skipping queue tickets API tests")
	}

	tests := []struct {
		name           string
		queueID        string
		queryParams    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return tickets for queue",
			queueID:        "1",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should contain tickets for Raw queue
				assert.Contains(t, body, "TICKET-001")
				assert.Contains(t, body, "TICKET-003")
				// Should show 2 tickets total
				assert.Equal(t, 2, strings.Count(body, "TICKET-"))
			},
		},
		{
			name:           "should filter tickets by status",
			queueID:        "1",
			queryParams:    "?status=new",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should only show new tickets from Raw queue
				assert.Contains(t, body, "new")
				assert.NotContains(t, body, "open")
			},
		},
		{
			name:           "should paginate tickets",
			queueID:        "1",
			queryParams:    "?page=1&limit=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show pagination controls
				assert.Contains(t, body, "pagination")
				// Should show only 1 ticket per page
				assert.Equal(t, 1, strings.Count(body, "TICKET-"))
			},
		},
		{
			name:           "should return empty for queue with no tickets",
			queueID:        "3",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "No tickets in this queue")
				assert.Equal(t, 0, strings.Count(body, "TICKET-"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues/:id/tickets", handleQueueTickets)

			req, _ := http.NewRequest("GET", "/api/queues/"+tt.queueID+"/tickets"+tt.queryParams, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueDetailHTMX(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() {
		t.Skip("DB not available - skipping queue detail HTMX tests")
	}

	tests := []struct {
		name           string
		queueID        string
		htmxRequest    bool
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "should return HTML fragment for HTMX requests",
			queueID:        "1",
			htmxRequest:    true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
				body := w.Body.String()
				// Should be HTML fragment, not full page
				assert.NotContains(t, body, "<html>")
				assert.NotContains(t, body, "<head>")
				assert.Contains(t, body, "Raw") // Queue name should be present
			},
		},
		{
			name:           "should return full page for regular requests",
			queueID:        "1",
			htmxRequest:    false,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
				body := w.Body.String()
				// Should be full page
				assert.Contains(t, body, "<html>")
				assert.Contains(t, body, "<head>")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues/:id", handleQueueDetailJSON)

			req, _ := http.NewRequest("GET", "/api/queues/"+tt.queueID, nil)
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

func TestQueueDetailErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() {
		t.Skip("DB not available - skipping queue error handling tests")
	}

	tests := []struct {
		name           string
		queueID        string
		setupError     bool
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should handle non-existent queue gracefully",
			queueID:        "999",
			setupError:     false,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue not found")
			},
		},
		{
			name:           "should handle invalid queue ID gracefully",
			queueID:        "invalid",
			setupError:     false,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "invalid queue id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues/:id", handleQueueDetail)

			req, _ := http.NewRequest("GET", "/api/queues/"+tt.queueID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}
