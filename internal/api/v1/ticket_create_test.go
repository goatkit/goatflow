package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/gotrs-io/gotrs-ce/internal/api"
)

func TestCreateTicket_Integration(t *testing.T) {
	requireDatabase(t)
	ensureTicketFixtures(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Mock authentication setup (no longer needed for direct API handler calls)

	// Setup routes
	v1 := router.Group("/api/v1")

	// Add mock authentication middleware
	v1.Use(func(c *gin.Context) {
		// Set mock user context
		c.Set("user_id", 1)
		c.Set("user_login", "testuser")
		c.Set("user_email", "test@example.com")
		c.Set("is_authenticated", true)
		c.Next()
	})

	tickets := v1.Group("/tickets")
	tickets.POST("", HandleCreateTicketAPI)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		wantError  bool
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "valid ticket with minimal fields",
			payload: map[string]interface{}{
				"title":    "Test Ticket",
				"queue_id": 1,
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.NotEmpty(t, data["id"])
				assert.NotEmpty(t, data["tn"]) // ticket number
				assert.Equal(t, "Test Ticket", data["title"])
			},
		},
		{
			name: "valid ticket with article",
			payload: map[string]interface{}{
				"title":            "Test Ticket with Article",
				"queue_id":         1,
				"type_id":          1,
				"state_id":         1,
				"priority_id":      3,
				"customer_user_id": "customer@example.com",
				"article": map[string]interface{}{
					"subject":      "Initial message",
					"body":         "This is the ticket description",
					"content_type": "text/plain",
				},
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.NotEmpty(t, data["id"])
				assert.Equal(t, "Test Ticket with Article", data["title"])
			},
		},
		{
			name: "missing required title",
			payload: map[string]interface{}{
				"queue_id": 1,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "missing required queue_id",
			payload: map[string]interface{}{
				"title": "Test Ticket",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "invalid queue_id",
			payload: map[string]interface{}{
				"title":    "Test Ticket",
				"queue_id": 99999,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request
			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			// Record response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check status
			assert.Equal(t, tt.wantStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Check response
			if tt.wantError {
				assert.False(t, response["success"].(bool))
				assert.NotEmpty(t, response["error"])
			} else {
				if tt.checkBody != nil {
					tt.checkBody(t, response)
				}
			}
		})
	}
}

func TestCreateTicket_Validation(t *testing.T) {
	requireDatabase(t)
	ensureTicketFixtures(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		payload   interface{}
		wantError string
	}{
		{
			name:      "invalid JSON",
			payload:   "not json",
			wantError: "Invalid ticket request",
		},
		{
			name:      "empty payload",
			payload:   map[string]interface{}{},
			wantError: "Invalid ticket request",
		},
		{
			name: "title too long",
			payload: map[string]interface{}{
				"title":    string(make([]byte, 256)), // 256 chars
				"queue_id": 1,
			},
			wantError: "Title too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			router.POST("/api/v1/tickets", func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
				HandleCreateTicketAPI(c)
			})

			var jsonData []byte
			var err error

			if str, ok := tt.payload.(string); ok {
				jsonData = []byte(str)
			} else {
				jsonData, err = json.Marshal(tt.payload)
				require.NoError(t, err)
			}

			req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.False(t, response["success"].(bool))
			assert.Contains(t, response["error"].(string), tt.wantError)
		})
	}
}
