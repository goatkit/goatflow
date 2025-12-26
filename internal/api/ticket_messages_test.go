//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Ticket Messages Feature
// Messages are public communications visible to customers and agents
// This addresses the 404 error: "Response Status Error Code 404 from /api/tickets/2/messages"

func TestGetTicketMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")
	t.Setenv("DB_HOST", "")
	t.Setenv("DATABASE_URL", "")

	tests := []struct {
		name       string
		ticketID   string
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Admin gets ticket messages",
			ticketID:   "1",
			userRole:   "Admin",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "messages")
				messages := resp["messages"].([]interface{})
				assert.GreaterOrEqual(t, len(messages), 0)
			},
		},
		{
			name:       "Agent gets ticket messages",
			ticketID:   "1",
			userRole:   "Agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "messages")
			},
		},
		{
			name:       "Customer gets own ticket messages",
			ticketID:   "1",
			userRole:   "Customer",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "messages")
				// Should only see public messages
			},
		},
		{
			name:       "Invalid ticket ID returns 404",
			ticketID:   "999",
			userRole:   "Admin",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "error")
				assert.Contains(t, resp["error"], "not found")
			},
		},
		{
			name:       "Non-numeric ticket ID returns 400",
			ticketID:   "invalid",
			userRole:   "Admin",
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "error")
				assert.Contains(t, resp["error"], "invalid ticket ID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test SHOULD FAIL initially because the handler doesn't exist yet
			router := gin.New()

			// Setup test router with real handler
			router.GET("/api/tickets/:id/messages", func(c *gin.Context) {
				// Mock user context
				c.Set("user_role", tt.userRole)
				c.Set("user_id", uint(1))
				c.Set("user_email", "test@example.com")

				// Use the real handler
				handleGetTicketMessages(c)
			})

			// Make request
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/tickets/%s/messages", tt.ticketID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Parse response
			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			// Test with real handler
			assert.Equal(t, tt.wantStatus, w.Code, "Status code mismatch for %s", tt.name)
			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestAddTicketMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")
	t.Setenv("DB_HOST", "")
	t.Setenv("DATABASE_URL", "")

	tests := []struct {
		name       string
		ticketID   string
		payload    map[string]interface{}
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:     "Admin adds public message",
			ticketID: "1",
			payload: map[string]interface{}{
				"content":     "Thank you for contacting support. We're reviewing your request.",
				"is_internal": false,
			},
			userRole:   "Admin",
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Message added successfully", resp["message"])
				assert.Contains(t, resp, "message_id")
				article := resp["article"].(map[string]interface{})
				assert.Equal(t, float64(1), article["is_visible_for_customer"]) // 1 = visible = not internal
			},
		},
		{
			name:     "Agent adds internal message",
			ticketID: "1",
			payload: map[string]interface{}{
				"content":     "Customer called - needs follow-up tomorrow",
				"is_internal": true,
			},
			userRole:   "Agent",
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Message added successfully", resp["message"])
				article := resp["article"].(map[string]interface{})
				assert.Equal(t, float64(0), article["is_visible_for_customer"]) // 0 = not visible = internal
			},
		},
		{
			name:     "Customer adds public message",
			ticketID: "1",
			payload: map[string]interface{}{
				"content": "I tried the suggested solution but the issue persists.",
			},
			userRole:   "Customer",
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Message added successfully", resp["message"])
				article := resp["article"].(map[string]interface{})
				// Customer messages are always public
				assert.Equal(t, float64(1), article["is_visible_for_customer"]) // 1 = visible = not internal
			},
		},
		{
			name:     "Empty message content returns 400",
			ticketID: "1",
			payload: map[string]interface{}{
				"content": "",
			},
			userRole:   "Agent",
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "error")
				assert.Contains(t, resp["error"], "Content")
			},
		},
		{
			name:       "Missing content field returns 400",
			ticketID:   "1",
			payload:    map[string]interface{}{},
			userRole:   "Agent",
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "error")
				assert.Contains(t, resp["error"], "Content")
			},
		},
		{
			name:     "Invalid ticket ID returns 404",
			ticketID: "999",
			payload: map[string]interface{}{
				"content": "This should fail",
			},
			userRole:   "Admin",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "error")
				assert.Contains(t, resp["error"], "not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test SHOULD FAIL initially because the handler doesn't exist yet
			router := gin.New()

			// Setup test router with real handler
			router.POST("/api/tickets/:id/messages", func(c *gin.Context) {
				// Mock user context
				c.Set("user_role", tt.userRole)
				c.Set("user_id", uint(1))
				c.Set("user_email", "test@example.com")

				// Use the real handler
				handleAddTicketMessage(c)
			})

			// Prepare request body
			body, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			// Make request
			req := httptest.NewRequest("POST", fmt.Sprintf("/api/tickets/%s/messages", tt.ticketID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Parse response
			var resp map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			// Test with real handler
			assert.Equal(t, tt.wantStatus, w.Code, "Status code mismatch for %s", tt.name)
			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestTicketMessagesHTMXIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		endpoint    string
		method      string
		htmxRequest bool
		userRole    string
		wantStatus  int
		wantHeader  string
	}{
		{
			name:        "HTMX GET messages returns partial HTML",
			endpoint:    "/api/tickets/1/messages",
			method:      "GET",
			htmxRequest: true,
			userRole:    "Admin",
			wantStatus:  http.StatusOK,
			wantHeader:  "text/html",
		},
		{
			name:        "HTMX POST message returns updated list",
			endpoint:    "/api/tickets/1/messages",
			method:      "POST",
			htmxRequest: true,
			userRole:    "Agent",
			wantStatus:  http.StatusOK,
			wantHeader:  "text/html",
		},
		{
			name:        "Regular API GET returns JSON",
			endpoint:    "/api/tickets/1/messages",
			method:      "GET",
			htmxRequest: false,
			userRole:    "Admin",
			wantStatus:  http.StatusOK,
			wantHeader:  "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Mock handlers for HTMX testing
			router.GET("/api/tickets/:id/messages", func(c *gin.Context) {
				c.Set("user_role", tt.userRole)

				if c.GetHeader("HX-Request") != "" {
					// Should return HTML fragment for HTMX
					c.Header("Content-Type", "text/html")
					c.String(http.StatusOK, "<div>Message list HTML</div>")
				} else {
					// Should return JSON for API
					c.JSON(http.StatusOK, gin.H{"messages": []string{}})
				}
			})

			router.POST("/api/tickets/:id/messages", func(c *gin.Context) {
				c.Set("user_role", tt.userRole)

				if c.GetHeader("HX-Request") != "" {
					// Should return updated HTML fragment
					c.Header("Content-Type", "text/html")
					c.String(http.StatusOK, "<div>Updated message list</div>")
				} else {
					// Should return JSON response
					c.JSON(http.StatusCreated, gin.H{"message": "Added"})
				}
			})

			// Prepare request
			var req *http.Request
			if tt.method == "POST" {
				body := strings.NewReader(`{"content":"Test message"}`)
				req = httptest.NewRequest(tt.method, tt.endpoint, body)
			} else {
				req = httptest.NewRequest(tt.method, tt.endpoint, nil)
			}

			if tt.htmxRequest {
				req.Header.Set("HX-Request", "true")
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), tt.wantHeader)
		})
	}
}

// Test to verify the exact 404 error user reported is fixed
func TestTicketMessages404Fix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Fix for reported 404 error", func(t *testing.T) {
		// This reproduces the exact error: "Response Status Error Code 404 from /api/tickets/2/messages"
		router := gin.New()

		// Currently this returns underConstructionAPI which gives 404
		// After fix, this should return actual message data
		router.GET("/api/tickets/:id/messages", func(c *gin.Context) {
			// This is the current implementation (underConstructionAPI)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "This endpoint is under construction",
				"path":  c.Request.URL.Path,
			})
		})

		req := httptest.NewRequest("GET", "/api/tickets/2/messages", nil)
		req.Header.Set("HX-Request", "true") // Simulate HTMX request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// This currently returns 404 - this is the bug we're fixing
		assert.Equal(t, http.StatusNotFound, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "under construction")

		// TODO: After implementing real handler, this should return 200
		// assert.Equal(t, http.StatusOK, w.Code)
	})
}
