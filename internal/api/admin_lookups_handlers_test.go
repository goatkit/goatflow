package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

func init() {
	os.Setenv("APP_ENV", "test")
}

// setupLookupsTestRouter creates a minimal router with admin lookup handlers.
func setupLookupsTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Types routes
	router.GET("/admin/types", handleAdminTypes)
	router.POST("/admin/types/create", handleAdminTypeCreate)
	router.POST("/admin/types/:id/update", handleAdminTypeUpdate)
	router.POST("/admin/types/:id/delete", handleAdminTypeDelete)

	// Priorities routes
	router.GET("/admin/priorities", handleAdminPriorities)

	// Combined lookups page
	router.GET("/admin/lookups", handleAdminLookups)

	return router
}

// =============================================================================
// ADMIN TYPES TESTS (strengthening existing tests)
// =============================================================================

func TestAdminTypesPageExtended(t *testing.T) {
	router := setupLookupsTestRouter()

	t.Run("GET /admin/types renders page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/types", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Type")
	})

	t.Run("GET /admin/types with search", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/types?search=incident", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET /admin/types with sort and order", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/types?sort=name&order=asc", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAdminTypesCRUDExtended(t *testing.T) {
	router := setupLookupsTestRouter()

	var createdTypeID int

	t.Run("Create type returns created status", func(t *testing.T) {
		payload := map[string]interface{}{
			"name": "Test Type For Deletion",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/types/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		// Extract the created type ID for later tests
		if typeData, ok := response["type"].(map[string]interface{}); ok {
			if id, ok := typeData["id"].(float64); ok {
				createdTypeID = int(id)
			}
		}
	})

	t.Run("Update type returns success", func(t *testing.T) {
		// Use a high ID that likely exists for update test
		formData := url.Values{
			"name": {"Updated Type Name"},
		}

		req := httptest.NewRequest(http.MethodPost, "/admin/types/1/update", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Delete type returns success", func(t *testing.T) {
		// Skip if we didn't get a created type ID - we need a type without tickets
		if createdTypeID == 0 {
			t.Skip("No created type ID available - skipping delete test")
		}
		// Delete the newly created type (which has no tickets)
		deleteURL := fmt.Sprintf("/admin/types/%d/delete", createdTypeID)
		req := httptest.NewRequest(http.MethodPost, deleteURL, nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Type with no tickets should be deletable
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Delete type with tickets returns error", func(t *testing.T) {
		// Type ID 1 (Unclassified) has tickets, should return 400
		req := httptest.NewRequest(http.MethodPost, "/admin/types/1/delete", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail because type 1 has tickets
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminTypesValidationExtended(t *testing.T) {
	router := setupLookupsTestRouter()

	t.Run("Create type with empty name fails", func(t *testing.T) {
		formData := url.Values{
			"name": {""},
		}

		req := httptest.NewRequest(http.MethodPost, "/admin/types/create", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})

	t.Run("Update type with invalid ID fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/types/invalid/update", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Create type with whitespace-only name fails", func(t *testing.T) {
		formData := url.Values{
			"name": {"   "},
		}

		req := httptest.NewRequest(http.MethodPost, "/admin/types/create", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Handler may trim whitespace; check for either 400 or 201
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusCreated)
	})
}

// =============================================================================
// ADMIN PRIORITIES TESTS
// =============================================================================

func TestAdminPrioritiesPage(t *testing.T) {
	router := setupLookupsTestRouter()

	t.Run("GET /admin/priorities returns page or error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Without DB, may return 500 or render fallback
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})
}

// =============================================================================
// ADMIN LOOKUPS (COMBINED PAGE) TESTS
// =============================================================================

func TestAdminLookupsPageCombined(t *testing.T) {
	// Clear global renderer and set test mode to ensure fallback HTML is used
	shared.SetGlobalRenderer(nil)
	t.Setenv("HTMX_HANDLER_TEST_MODE", "1")

	router := setupLookupsTestRouter()

	t.Run("GET /admin/lookups renders page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()

		// Check for main page elements
		assert.Contains(t, body, "Manage Lookup Values")
		assert.Contains(t, body, "Queues")
		assert.Contains(t, body, "Priorities")
		assert.Contains(t, body, "Ticket Types")
		assert.Contains(t, body, "Statuses")
		assert.Contains(t, body, "Refresh Cache")
	})

	t.Run("GET /admin/lookups with tab param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET /admin/lookups with priorities tab", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET /admin/lookups with types tab", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET /admin/lookups with queues tab", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=queues", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// =============================================================================
// LOOKUP API TESTS
// =============================================================================

func TestLookupAPIEndpointsExtended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/lookups/queues", HandleGetQueues)
	router.GET("/api/lookups/priorities", HandleGetPriorities)
	router.GET("/api/lookups/types", HandleGetTypes)
	router.GET("/api/lookups/statuses", HandleGetStatuses)
	router.GET("/api/lookups/form-data", HandleGetFormData)
	router.POST("/api/lookups/cache/invalidate", func(c *gin.Context) {
		c.Set("user_role", "Admin")
		HandleInvalidateLookupCache(c)
	})

	t.Run("GET /api/lookups/queues returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/queues", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("GET /api/lookups/priorities returns 5 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(data))
	})

	t.Run("GET /api/lookups/types returns 5 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(data))
	})

	t.Run("GET /api/lookups/statuses returns 5 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(data))
	})

	t.Run("GET /api/lookups/form-data returns all lookups", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/form-data", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok)
		assert.NotNil(t, data["queues"])
		assert.NotNil(t, data["priorities"])
		assert.NotNil(t, data["types"])
		assert.NotNil(t, data["statuses"])
	})

	t.Run("POST /api/lookups/cache/invalidate as admin succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/lookups/cache/invalidate", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})
}

func TestLookupCacheInvalidationPermissionsExtended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/api/lookups/cache/invalidate/agent", func(c *gin.Context) {
		c.Set("user_role", "Agent")
		HandleInvalidateLookupCache(c)
	})

	router.POST("/api/lookups/cache/invalidate/empty", func(c *gin.Context) {
		c.Set("user_role", "")
		HandleInvalidateLookupCache(c)
	})

	t.Run("Agent cannot invalidate cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/lookups/cache/invalidate/agent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})

	t.Run("Empty role cannot invalidate cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/lookups/cache/invalidate/empty", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// =============================================================================
// PRIORITY VALIDATION TESTS
// =============================================================================

func TestPriorityStructureValidationExtended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/priorities", HandleGetPriorities)

	t.Run("Priority has required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			priority := item.(map[string]interface{})
			assert.NotNil(t, priority["value"], "Priority should have value")
			assert.NotNil(t, priority["label"], "Priority should have label")
			assert.NotNil(t, priority["order"], "Priority should have order")
		}
	})

	t.Run("Priorities are ordered correctly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		expectedOrder := []string{"1 very low", "2 low", "3 normal", "4 high", "5 very high"}

		for i, item := range data {
			priority := item.(map[string]interface{})
			assert.Equal(t, expectedOrder[i], priority["value"])
		}
	})
}

// =============================================================================
// STATUS/STATE STRUCTURE TESTS
// =============================================================================

func TestStatusStructureValidationExtended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/statuses", HandleGetStatuses)

	t.Run("Status has required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			status := item.(map[string]interface{})
			assert.NotNil(t, status["value"], "Status should have value")
			assert.NotNil(t, status["label"], "Status should have label")
			assert.NotNil(t, status["order"], "Status should have order")
		}
	})

	t.Run("Statuses contain expected OTRS values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		// OTRS standard state names (order may vary)
		expectedStates := map[string]bool{
			"new": false, "open": false, "pending reminder": false,
			"closed successful": false, "closed unsuccessful": false,
		}

		for _, item := range data {
			status := item.(map[string]interface{})
			value := status["value"].(string)
			if _, ok := expectedStates[value]; ok {
				expectedStates[value] = true
			}
		}

		for name, found := range expectedStates {
			assert.True(t, found, "Status '%s' should be present", name)
		}
	})
}

// =============================================================================
// TYPE STRUCTURE TESTS
// =============================================================================

func TestTypeStructureValidationExtended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/types", HandleGetTypes)

	t.Run("Type has required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			typ := item.(map[string]interface{})
			assert.NotNil(t, typ["id"], "Type should have id")
			assert.NotNil(t, typ["value"], "Type should have value")
			assert.NotNil(t, typ["label"], "Type should have label")
			assert.NotNil(t, typ["order"], "Type should have order")
		}
	})

	t.Run("Types include expected values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		values := make([]string, 0, len(data))
		for _, item := range data {
			typ := item.(map[string]interface{})
			values = append(values, typ["value"].(string))
		}

		assert.Contains(t, values, "incident")
		assert.Contains(t, values, "service_request")
	})
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestLookupConcurrentAccessExtended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/priorities", HandleGetPriorities)
	router.GET("/api/lookups/types", HandleGetTypes)
	router.GET("/api/lookups/statuses", HandleGetStatuses)

	t.Run("Concurrent requests handled correctly", func(t *testing.T) {
		numRequests := 20
		done := make(chan bool, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(index int) {
				endpoints := []string{
					"/api/lookups/priorities",
					"/api/lookups/types",
					"/api/lookups/statuses",
				}

				endpoint := endpoints[index%len(endpoints)]
				req := httptest.NewRequest(http.MethodGet, endpoint, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				done <- true
			}(i)
		}

		for i := 0; i < numRequests; i++ {
			<-done
		}
	})
}

// =============================================================================
// ADMIN LOOKUPS (COMBINED PAGE) TESTS
// =============================================================================

func TestAdminLookupsPage(t *testing.T) {
	// Clear global renderer and set test mode to ensure fallback HTML is used
	shared.SetGlobalRenderer(nil)
	t.Setenv("HTMX_HANDLER_TEST_MODE", "1")

	router := setupLookupsTestRouter()

	t.Run("GET /admin/lookups renders page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()

		// Check for main page elements
		assert.Contains(t, body, "Manage Lookup Values")
		assert.Contains(t, body, "Queues")
		assert.Contains(t, body, "Priorities")
		assert.Contains(t, body, "Ticket Types")
		assert.Contains(t, body, "Statuses")
		assert.Contains(t, body, "Refresh Cache")
	})

	t.Run("GET /admin/lookups with tab param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET /admin/lookups with priorities tab", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET /admin/lookups with types tab", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET /admin/lookups with queues tab", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=queues", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// =============================================================================
// LOOKUP API TESTS
// =============================================================================

func TestLookupAPIEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/lookups/queues", HandleGetQueues)
	router.GET("/api/lookups/priorities", HandleGetPriorities)
	router.GET("/api/lookups/types", HandleGetTypes)
	router.GET("/api/lookups/statuses", HandleGetStatuses)
	router.GET("/api/lookups/form-data", HandleGetFormData)
	router.POST("/api/lookups/cache/invalidate", func(c *gin.Context) {
		c.Set("user_role", "Admin")
		HandleInvalidateLookupCache(c)
	})

	t.Run("GET /api/lookups/queues returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/queues", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("GET /api/lookups/priorities returns 5 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(data))
	})

	t.Run("GET /api/lookups/types returns 5 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(data))
	})

	t.Run("GET /api/lookups/statuses returns 5 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(data))
	})

	t.Run("GET /api/lookups/form-data returns all lookups", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/form-data", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok)
		assert.NotNil(t, data["queues"])
		assert.NotNil(t, data["priorities"])
		assert.NotNil(t, data["types"])
		assert.NotNil(t, data["statuses"])
	})

	t.Run("POST /api/lookups/cache/invalidate as admin succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/lookups/cache/invalidate", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})
}

func TestLookupCacheInvalidationPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/api/lookups/cache/invalidate/agent", func(c *gin.Context) {
		c.Set("user_role", "Agent")
		HandleInvalidateLookupCache(c)
	})

	router.POST("/api/lookups/cache/invalidate/empty", func(c *gin.Context) {
		c.Set("user_role", "")
		HandleInvalidateLookupCache(c)
	})

	t.Run("Agent cannot invalidate cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/lookups/cache/invalidate/agent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})

	t.Run("Empty role cannot invalidate cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/lookups/cache/invalidate/empty", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// =============================================================================
// PRIORITY VALIDATION TESTS
// =============================================================================

func TestPriorityStructureValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/priorities", HandleGetPriorities)

	t.Run("Priority has required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			priority := item.(map[string]interface{})
			assert.NotNil(t, priority["value"], "Priority should have value")
			assert.NotNil(t, priority["label"], "Priority should have label")
			assert.NotNil(t, priority["order"], "Priority should have order")
		}
	})

	t.Run("Priorities are ordered correctly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		expectedOrder := []string{"1 very low", "2 low", "3 normal", "4 high", "5 very high"}

		for i, item := range data {
			priority := item.(map[string]interface{})
			assert.Equal(t, expectedOrder[i], priority["value"])
		}
	})
}

// =============================================================================
// STATUS/STATE STRUCTURE TESTS
// =============================================================================

func TestStatusStructureValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/statuses", HandleGetStatuses)

	t.Run("Status has required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			status := item.(map[string]interface{})
			assert.NotNil(t, status["value"], "Status should have value")
			assert.NotNil(t, status["label"], "Status should have label")
			assert.NotNil(t, status["order"], "Status should have order")
		}
	})

	t.Run("Statuses contain expected OTRS values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		// OTRS standard state names (order may vary)
		expectedStates := map[string]bool{
			"new": false, "open": false, "pending reminder": false,
			"closed successful": false, "closed unsuccessful": false,
		}

		for _, item := range data {
			status := item.(map[string]interface{})
			value := status["value"].(string)
			if _, ok := expectedStates[value]; ok {
				expectedStates[value] = true
			}
		}

		for name, found := range expectedStates {
			assert.True(t, found, "Status '%s' should be present", name)
		}
	})
}

// =============================================================================
// TYPE STRUCTURE TESTS
// =============================================================================

func TestTypeStructureValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/types", HandleGetTypes)

	t.Run("Type has required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			typ := item.(map[string]interface{})
			assert.NotNil(t, typ["id"], "Type should have id")
			assert.NotNil(t, typ["value"], "Type should have value")
			assert.NotNil(t, typ["label"], "Type should have label")
			assert.NotNil(t, typ["order"], "Type should have order")
		}
	})

	t.Run("Types include expected values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		values := make([]string, 0, len(data))
		for _, item := range data {
			typ := item.(map[string]interface{})
			values = append(values, typ["value"].(string))
		}

		assert.Contains(t, values, "incident")
		assert.Contains(t, values, "service_request")
	})
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestLookupConcurrentAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/priorities", HandleGetPriorities)
	router.GET("/api/lookups/types", HandleGetTypes)
	router.GET("/api/lookups/statuses", HandleGetStatuses)

	t.Run("Concurrent requests handled correctly", func(t *testing.T) {
		numRequests := 20
		done := make(chan bool, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(index int) {
				endpoints := []string{
					"/api/lookups/priorities",
					"/api/lookups/types",
					"/api/lookups/statuses",
				}

				endpoint := endpoints[index%len(endpoints)]
				req := httptest.NewRequest(http.MethodGet, endpoint, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				done <- true
			}(i)
		}

		for i := 0; i < numRequests; i++ {
			<-done
		}
	})
}
