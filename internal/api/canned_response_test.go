package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Canned Response Feature
// Canned responses allow agents to quickly insert pre-written responses

func TestCreateCannedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name: "Create personal canned response",
			payload: map[string]interface{}{
				"name":     "New Thank You",
				"category": "General",
				"content":  "Thank you for contacting support. We'll review your request and respond shortly.",
				"tags":     []string{"greeting", "acknowledgment"},
				"scope":    "personal",
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Canned response created successfully", resp["message"])
				assert.Contains(t, resp, "id")
				response := resp["response"].(map[string]interface{})
				assert.Equal(t, "New Thank You", response["name"])
				assert.Equal(t, "personal", response["scope"])
			},
		},
		{
			name: "Create team canned response",
			payload: map[string]interface{}{
				"name":     "New Password Reset Instructions",
				"category": "Account",
				"content": `To reset your password:
1. Go to {{login_url}}
2. Click "Forgot Password"
3. Enter your email: {{customer_email}}
4. Check your email for reset instructions`,
				"tags":         []string{"password", "account", "instructions"},
				"scope":        "team",
				"team_id":      1,
				"placeholders": []string{"login_url", "customer_email"},
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				response := resp["response"].(map[string]interface{})
				assert.Equal(t, "team", response["scope"])
				assert.Equal(t, float64(1), response["team_id"])
				placeholders := response["placeholders"].([]interface{})
				assert.Len(t, placeholders, 2)
			},
		},
		{
			name: "Create global canned response (admin only)",
			payload: map[string]interface{}{
				"name":     "New Service Maintenance",
				"category": "System",
				"content":  "We are currently performing scheduled maintenance. Services will be restored by {{time}}.",
				"scope":    "global",
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				response := resp["response"].(map[string]interface{})
				assert.Equal(t, "global", response["scope"])
			},
		},
		{
			name: "Create with rich text content",
			payload: map[string]interface{}{
				"name":         "Welcome Message",
				"category":     "Onboarding",
				"content":      "<h3>Welcome!</h3><p>We're glad to have you as a customer.</p><ul><li>Check our <a href='{{kb_url}}'>knowledge base</a></li><li>Contact support at {{support_email}}</li></ul>",
				"content_type": "html",
				"scope":        "personal",
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				response := resp["response"].(map[string]interface{})
				assert.Equal(t, "html", response["content_type"])
			},
		},
		{
			name: "Missing required fields",
			payload: map[string]interface{}{
				"category": "General",
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Name and content are required")
			},
		},
		{
			name: "Duplicate name in same scope",
			payload: map[string]interface{}{
				"name":    "New Thank You", // Already exists from first test
				"content": "Different content",
				"scope":   "personal",
			},
			wantStatus: http.StatusConflict,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Canned response with this name already exists")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", "admin") // Admin for global responses
				c.Set("team_id", 1)
				c.Next()
			})
			router.POST("/api/canned-responses", handleCreateCannedResponse)

			body, _ := json.Marshal(tt.payload)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/canned-responses", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestGetCannedResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Get all accessible responses",
			query:      "",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				responses := resp["responses"].([]interface{})
				assert.Greater(t, len(responses), 0)
				// Should include personal, team, and global responses
			},
		},
		{
			name:       "Filter by category",
			query:      "?category=General",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				responses := resp["responses"].([]interface{})
				for _, r := range responses {
					response := r.(map[string]interface{})
					assert.Equal(t, "General", response["category"])
				}
			},
		},
		{
			name:       "Filter by scope",
			query:      "?scope=personal",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				responses := resp["responses"].([]interface{})
				for _, r := range responses {
					response := r.(map[string]interface{})
					assert.Equal(t, "personal", response["scope"])
				}
			},
		},
		{
			name:       "Search responses",
			query:      "?search=password",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				responses := resp["responses"].([]interface{})
				assert.Greater(t, len(responses), 0)
				// Should find password-related responses
			},
		},
		{
			name:       "Filter by tags",
			query:      "?tags=greeting",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				responses := resp["responses"].([]interface{})
				for _, r := range responses {
					response := r.(map[string]interface{})
					tags := response["tags"].([]interface{})
					found := false
					for _, tag := range tags {
						if tag == "greeting" {
							found = true
							break
						}
					}
					assert.True(t, found)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", tt.userRole)
				c.Set("team_id", 1)
				c.Next()
			})
			router.GET("/api/canned-responses", handleGetCannedResponses)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/canned-responses"+tt.query, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestUpdateCannedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		responseID string
		payload    map[string]interface{}
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Update response content",
			responseID: "1",
			payload: map[string]interface{}{
				"content": "Updated: Thank you for your patience. We're working on your request.",
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Canned response updated successfully", resp["message"])
			},
		},
		{
			name:       "Update response category and tags",
			responseID: "1",
			payload: map[string]interface{}{
				"category": "Support",
				"tags":     []string{"updated", "support"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				response := resp["response"].(map[string]interface{})
				assert.Equal(t, "Support", response["category"])
			},
		},
		{
			name:       "Add placeholders",
			responseID: "2",
			payload: map[string]interface{}{
				"content":      "Hello {{customer_name}}, thank you for ticket #{{ticket_id}}",
				"placeholders": []string{"customer_name", "ticket_id"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				response := resp["response"].(map[string]interface{})
				placeholders := response["placeholders"].([]interface{})
				assert.Len(t, placeholders, 2)
			},
		},
		{
			name:       "Response not found",
			responseID: "99999",
			payload: map[string]interface{}{
				"content": "Test",
			},
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Canned response not found")
			},
		},
		{
			name:       "Cannot update others' personal response",
			responseID: "3", // Belongs to another user
			payload: map[string]interface{}{
				"content": "Trying to update",
			},
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "You can only edit your own personal responses")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", "agent")
				c.Next()
			})
			router.PUT("/api/canned-responses/:id", handleUpdateCannedResponse)

			body, _ := json.Marshal(tt.payload)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/canned-responses/%s", tt.responseID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestDeleteCannedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		responseID string
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Delete own personal response",
			responseID: "1",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Canned response deleted successfully", resp["message"])
			},
		},
		{
			name:       "Admin delete global response",
			responseID: "4",
			userRole:   "admin",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Canned response deleted successfully", resp["message"])
			},
		},
		{
			name:       "Cannot delete others' personal response",
			responseID: "3",
			userRole:   "agent",
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "You can only delete your own personal responses")
			},
		},
		{
			name:       "Response not found",
			responseID: "99999",
			userRole:   "admin",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Canned response not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", tt.userRole)
				c.Next()
			})
			router.DELETE("/api/canned-responses/:id", handleDeleteCannedResponse)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/canned-responses/%s", tt.responseID), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestUseCannedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Ensure test data exists
	cannedResponses[1] = &CannedResponse{
		ID:          1,
		Name:        "Thank You",
		Category:    "General",
		Content:     "Thank you for contacting support. We'll review your request and respond shortly.",
		ContentType: "text",
		Scope:       "personal",
		OwnerID:     1,
	}
	cannedResponses[2] = &CannedResponse{
		ID:           2,
		Name:         "Ticket Update",
		Category:     "General",
		Content:      "Dear {{customer_name}}, your ticket {{ticket_id}} has been updated.",
		ContentType:  "text",
		Scope:        "personal",
		OwnerID:      1,
		Placeholders: []string{"customer_name", "ticket_id"},
	}

	tests := []struct {
		name       string
		responseID string
		payload    map[string]interface{}
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Use response without placeholders",
			responseID: "1",
			payload:    map[string]interface{}{},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "content")
				assert.Contains(t, resp["content"].(string), "Thank you")
			},
		},
		{
			name:       "Use response with placeholders",
			responseID: "2",
			payload: map[string]interface{}{
				"placeholders": map[string]string{
					"customer_name": "John Doe",
					"ticket_id":     "12345",
				},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				content := resp["content"].(string)
				assert.Contains(t, content, "John Doe")
				assert.Contains(t, content, "12345")
				assert.NotContains(t, content, "{{")
			},
		},
		{
			name:       "Missing required placeholders",
			responseID: "2",
			payload: map[string]interface{}{
				"placeholders": map[string]string{
					"customer_name": "John Doe",
					// Missing ticket_id
				},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Missing required placeholder: ticket_id")
			},
		},
		{
			name:       "Include metadata",
			responseID: "1",
			payload: map[string]interface{}{
				"include_metadata": true,
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "content")
				assert.Contains(t, resp, "used_at")
				assert.Contains(t, resp, "used_count")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Next()
			})
			router.POST("/api/canned-responses/:id/use", handleUseCannedResponse)

			body, _ := json.Marshal(tt.payload)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/canned-responses/%s/use", tt.responseID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestCannedResponseCategories(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Get all categories", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/canned-responses/categories", handleGetCannedResponseCategories)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/canned-responses/categories", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		categories := response["categories"].([]interface{})
		assert.Greater(t, len(categories), 0)

		for _, c := range categories {
			category := c.(map[string]interface{})
			assert.Contains(t, category, "name")
			assert.Contains(t, category, "count")
			assert.Contains(t, category, "color")
		}
	})
}

func TestCannedResponseStatistics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Get response usage statistics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/canned-responses/statistics", handleGetCannedResponseStatistics)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/canned-responses/statistics", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		stats := response["statistics"].(map[string]interface{})
		assert.Contains(t, stats, "total_responses")
		assert.Contains(t, stats, "personal_count")
		assert.Contains(t, stats, "team_count")
		assert.Contains(t, stats, "global_count")
		assert.Contains(t, stats, "most_used")
		assert.Contains(t, stats, "recently_used")
		assert.Contains(t, stats, "usage_by_category")
	})
}

func TestCannedResponseSharing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Share personal response with team", func(t *testing.T) {
		// Reset mock data state before each test
		resetCannedResponsesTestData()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Set("team_id", 1)
			c.Next()
		})
		router.POST("/api/canned-responses/:id/share", handleShareCannedResponse)

		payload := map[string]interface{}{
			"scope":   "team",
			"team_id": 1,
		}

		body, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/canned-responses/1/share", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Canned response shared successfully", response["message"])
	})

	t.Run("Copy shared response to personal", func(t *testing.T) {
		// Reset mock data state before each test
		resetCannedResponsesTestData()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/canned-responses/:id/copy", handleCopyCannedResponse)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/canned-responses/4/copy", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Canned response copied successfully", response["message"])
		assert.Contains(t, response, "id")
	})
}

func TestCannedResponseImportExport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Export canned responses", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/canned-responses/export", handleExportCannedResponses)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/canned-responses/export?format=json", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "responses")
		assert.Contains(t, response, "exported_at")
	})

	t.Run("Import canned responses", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/canned-responses/import", handleImportCannedResponses)

		importData := map[string]interface{}{
			"responses": []map[string]interface{}{
				{
					"name":     "Imported Response",
					"category": "Imported",
					"content":  "This is an imported response",
					"scope":    "personal",
				},
			},
		}

		body, _ := json.Marshal(importData)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/canned-responses/import", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Canned responses imported successfully", response["message"])
		assert.Contains(t, response, "imported_count")
		assert.Contains(t, response, "skipped_count")
	})
}

// resetCannedResponsesTestData resets the mock data to its initial state for test isolation.
func resetCannedResponsesTestData() {
	cannedResponses = map[int]*CannedResponse{
		1: {
			ID:          1,
			Name:        "Thank You",
			Category:    "General",
			Content:     "Thank you for contacting support. We'll review your request and respond shortly.",
			ContentType: "text",
			Tags:        []string{"greeting", "acknowledgment"},
			Scope:       "personal",
			OwnerID:     1,
			UsageCount:  5,
			CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-30 * 24 * time.Hour),
		},
		2: {
			ID:           2,
			Name:         "Password Reset Instructions",
			Category:     "Account",
			Content:      "Hello {{customer_name}}, thank you for ticket #{{ticket_id}}",
			ContentType:  "text",
			Tags:         []string{"password", "account"},
			Scope:        "team",
			OwnerID:      1,
			TeamID:       1,
			Placeholders: []string{"customer_name", "ticket_id"},
			UsageCount:   10,
			CreatedAt:    time.Now().Add(-20 * 24 * time.Hour),
			UpdatedAt:    time.Now().Add(-20 * 24 * time.Hour),
		},
		3: {
			ID:          3,
			Name:        "Another User Response",
			Category:    "General",
			Content:     "This belongs to another user",
			ContentType: "text",
			Scope:       "personal",
			OwnerID:     2, // Different user
			CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-10 * 24 * time.Hour),
		},
		4: {
			ID:          4,
			Name:        "Service Maintenance",
			Category:    "System",
			Content:     "We are currently performing scheduled maintenance.",
			ContentType: "text",
			Scope:       "global",
			OwnerID:     1,
			UsageCount:  20,
			CreatedAt:   time.Now().Add(-15 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-15 * 24 * time.Hour),
		},
	}

	nextCannedResponseID = 5
	cannedResponsesByName = map[string][]int{
		"Thank You":                   {1},
		"Password Reset Instructions": {2},
		"Another User Response":       {3},
		"Service Maintenance":         {4},
	}
}
