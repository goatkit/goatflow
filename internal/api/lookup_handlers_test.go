//go:build db

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGetQueues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Lookup service pulls from DB; if DB unavailable, handler returns defaults
	_ = database.InitTestDB()

	tests := []struct {
		name           string
		expectedStatus int
		validateBody   func(*testing.T, map[string]interface{})
	}{
		{
			name:           "Successfully returns queue list",
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))

				data, ok := body["data"].([]interface{})
				require.True(t, ok, "data should be an array")
				assert.NotEmpty(t, data)

				// Check first queue structure (JSON tags are lowercase)
				firstQueue := data[0].(map[string]interface{})
				assert.NotNil(t, firstQueue["id"])
				assert.NotEmpty(t, firstQueue["name"])
				assert.NotEmpty(t, firstQueue["description"])
				assert.NotNil(t, firstQueue["active"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/lookups/queues", HandleGetQueues)

			req, _ := http.NewRequest("GET", "/api/lookups/queues", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			tt.validateBody(t, response)
		})
	}
}

func TestHandleGetPriorities(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/lookups/priorities", HandleGetPriorities)
	req, _ := http.NewRequest("GET", "/api/lookups/priorities", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))

	data, ok := response["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	assert.Equal(t, 4, len(data)) // low, normal, high, urgent

	// Verify priority order (JSON tags are lowercase)
	priorities := data
	expectedValues := []string{"low", "normal", "high", "urgent"}
	for i, p := range priorities {
		priority := p.(map[string]interface{})
		assert.Equal(t, expectedValues[i], priority["value"])
		assert.NotEmpty(t, priority["label"])
		assert.Equal(t, float64(i+1), priority["order"])
	}
}

func TestHandleGetTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/lookups/types", HandleGetTypes)
	req, _ := http.NewRequest("GET", "/api/lookups/types", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))

	data, ok := response["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	assert.Equal(t, 5, len(data)) // incident, service_request, change_request, problem, question

	// Check structure (JSON tags are lowercase)
	for _, item := range data {
		typ := item.(map[string]interface{})
		assert.NotNil(t, typ["id"])
		assert.NotEmpty(t, typ["value"])
		assert.NotEmpty(t, typ["label"])
		assert.NotNil(t, typ["order"])
		// Check active field exists and is a bool
		if active, ok := typ["active"]; ok {
			assert.IsType(t, true, active)
		}
	}
}

func TestHandleGetStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/lookups/statuses", HandleGetStatuses)

	req, _ := http.NewRequest("GET", "/api/lookups/statuses", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))

	data, ok := response["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	assert.Equal(t, 5, len(data)) // new, open, pending, resolved, closed

	// Verify status workflow order (JSON tags are lowercase)
	expectedValues := []string{"new", "open", "pending", "resolved", "closed"}
	for i, s := range data {
		status := s.(map[string]interface{})
		assert.Equal(t, expectedValues[i], status["value"])
		assert.NotEmpty(t, status["label"])
		assert.Equal(t, float64(i+1), status["order"])
	}
}

func TestHandleGetFormData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/lookups/form-data", HandleGetFormData)
	req, _ := http.NewRequest("GET", "/api/lookups/form-data", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))

	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "data should be an object")

	// Check all required fields are present
	assert.NotNil(t, data["queues"])
	assert.NotNil(t, data["priorities"])
	assert.NotNil(t, data["types"])
	assert.NotNil(t, data["statuses"])

	// Verify each is an array with items
	queues := data["queues"].([]interface{})
	assert.NotEmpty(t, queues)

	priorities := data["priorities"].([]interface{})
	assert.Equal(t, 4, len(priorities))

	types := data["types"].([]interface{})
	assert.Equal(t, 5, len(types))

	statuses := data["statuses"].([]interface{})
	assert.Equal(t, 5, len(statuses))
}

func TestHandleInvalidateLookupCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userRole       string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "Admin can invalidate cache",
			userRole:       "Admin",
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"message": "Lookup cache invalidated successfully",
			},
		},
		{
			name:           "Non-admin cannot invalidate cache",
			userRole:       "Agent",
			expectedStatus: http.StatusForbidden,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Admin access required",
			},
		},
		{
			name:           "Empty role cannot invalidate cache",
			userRole:       "",
			expectedStatus: http.StatusForbidden,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Admin access required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_role", tt.userRole)
				c.Next()
			})
			router.POST("/api/lookups/cache/invalidate", HandleInvalidateLookupCache)

			req, _ := http.NewRequest("POST", "/api/lookups/cache/invalidate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedBody["success"], response["success"])
			if message, exists := tt.expectedBody["message"]; exists {
				assert.Equal(t, message, response["message"])
			}
			if errorMsg, exists := tt.expectedBody["error"]; exists {
				assert.Equal(t, errorMsg, response["error"])
			}
		})
	}
}

func TestHandleAdminLookups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkContent   []string
	}{
		{
			name:           "Renders admin lookups page",
			expectedStatus: http.StatusOK,
			checkContent: []string{
				"Manage Lookup Values",
				"Queues",
				"Priorities",
				"Ticket Types",
				"Statuses",
				"Refresh Cache",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/admin/lookups", handleAdminLookups)

			req, _ := http.NewRequest("GET", "/admin/lookups", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			body := w.Body.String()
			for _, content := range tt.checkContent {
				assert.Contains(t, body, content)
			}
		})
	}
}

func TestLookupEndpointIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test that all endpoints use the same service instance (singleton)
	router := gin.New()
	router.GET("/api/lookups/queues", HandleGetQueues)
	router.GET("/api/lookups/priorities", HandleGetPriorities)
	router.POST("/api/lookups/cache/invalidate", func(c *gin.Context) {
		c.Set("user_role", "Admin")
		HandleInvalidateLookupCache(c)
	})

	// Get initial data
	req1, _ := http.NewRequest("GET", "/api/lookups/queues", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	var response1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &response1)
	queues1 := response1["data"].([]interface{})

	// Invalidate cache
	req2, _ := http.NewRequest("POST", "/api/lookups/cache/invalidate", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Get data again - should be refreshed but same content
	req3, _ := http.NewRequest("GET", "/api/lookups/queues", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	var response3 map[string]interface{}
	json.Unmarshal(w3.Body.Bytes(), &response3)
	queues3 := response3["data"].([]interface{})

	// Data should be the same (content-wise)
	assert.Equal(t, len(queues1), len(queues3))
}

func TestConcurrentAPIAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/lookups/queues", HandleGetQueues)
	router.GET("/api/lookups/priorities", HandleGetPriorities)
	router.GET("/api/lookups/types", HandleGetTypes)
	router.GET("/api/lookups/statuses", HandleGetStatuses)
	router.GET("/api/lookups/form-data", HandleGetFormData)

	// Make concurrent requests
	numRequests := 50
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			endpoints := []string{
				"/api/lookups/queues",
				"/api/lookups/priorities",
				"/api/lookups/types",
				"/api/lookups/statuses",
				"/api/lookups/form-data",
			}

			endpoint := endpoints[index%len(endpoints)]
			req, _ := http.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.True(t, response["success"].(bool))

			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
}

// Helper function to parse JSON response
// parseJSONResponse is a local test helper; currently unused.
//
//nolint:unused
func parseJSONResponse(t *testing.T, body []byte) map[string]interface{} {
	var response map[string]interface{}
	err := json.Unmarshal(body, &response)
	require.NoError(t, err)
	return response
}

// Helper function to create a test TicketFormData
// createTestFormData constructs a sample form payload; currently unused.
//
//nolint:unused
func createTestFormData() *models.TicketFormData {
	return &models.TicketFormData{
		Queues: []models.QueueInfo{
			{ID: 1, Name: "Test Queue", Description: "Test", Active: true},
		},
		Priorities: []models.LookupItem{
			{ID: 1, Value: "low", Label: "Low", Order: 1, Active: true},
		},
		Types: []models.LookupItem{
			{ID: 1, Value: "incident", Label: "Incident", Order: 1, Active: true},
		},
		Statuses: []models.LookupItem{
			{ID: 1, Value: "new", Label: "New", Order: 1, Active: true},
		},
	}
}
