package api

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func TestQueueListAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	WithCleanDB(t)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should return all active queues",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should contain HTML with queue list (canonical queues from migrations)
				assert.Contains(t, body, "Postmaster")
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Misc")

				// Should show ticket counts (HTML formatted)
				assert.Contains(t, body, ">2</span> tickets") // Raw queue has 2 tickets
				assert.Contains(t, body, ">1</span> ticket")  // Junk queue has 1 ticket
				assert.Contains(t, body, ">0</span> tickets") // Misc and Postmaster have 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues", handleGetQueuesAPI)
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
	WithCleanDB(t)

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
				// Every queue (seeded ones included; the seed differs per
				// test database) is listed.
				db, err := database.GetDB()
				require.NoError(t, err)
				var queueCount int
				require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM queue")).Scan(&queueCount))
				assert.Equal(t, queueCount, len(response.Data))

				// Check specific queue data (Raw is id=2 in migrations)
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
			router.GET("/api/queues", handleGetQueuesAPI)

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

// Queue names are HTML-escaped in the fragment, one well-formed <li> per queue.
func TestQueueListHTMLEscapesNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)

	name := "<b>q" + strconv.FormatInt(time.Now().UnixNano()%1000000, 10) + "</b>"
	queueID := createTestQueue(t, db, name)
	t.Cleanup(func() { cleanupTestQueue(t, db, queueID) })

	router := gin.New()
	router.GET("/api/queues", handleGetQueuesAPI)
	req := httptest.NewRequest(http.MethodGet, "/api/queues?search="+url.QueryEscape(name), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "  <li>"+html.EscapeString(name)+" <span>0</span> tickets</li>")
	assert.NotContains(t, body, name)
	assert.NotContains(t, body, "<li=")
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
				// Should only contain active queues (canonical: Postmaster, Raw, Junk, Misc)
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Misc")
			},
		},
		{
			name:           "should search by name",
			queryParams:    "?search=misc",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Misc")
				assert.NotContains(t, body, "Raw")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues", handleGetQueuesAPI)

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
			router.GET("/api/queues", handleGetQueuesAPI)

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
