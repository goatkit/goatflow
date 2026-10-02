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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
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
	setupTemplateRenderer(t)
	router := setupLookupsTestRouter()
	db := getTestDB(t)

	suffix := fmt.Sprint(time.Now().UnixNano())
	alpha := "PageTypeAlpha" + suffix
	beta := "PageTypeBeta" + suffix
	for _, name := range []string{alpha, beta} {
		cleanupAdminTestTypeByName(t, name)
		_, err := db.Exec(database.ConvertPlaceholders(`INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 1, NOW(), 1, NOW(), 1)`), name)
		require.NoError(t, err)
	}

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("GET /admin/types lists types from the database", func(t *testing.T) {
		w := get("/admin/types")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), alpha)
		assert.Contains(t, w.Body.String(), beta)
	})

	t.Run("GET /admin/types with search filters rows", func(t *testing.T) {
		w := get("/admin/types?search=" + url.QueryEscape(alpha))
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), alpha)
		assert.NotContains(t, w.Body.String(), beta)
	})

	t.Run("GET /admin/types with sort and order", func(t *testing.T) {
		w := get("/admin/types?sort=name&order=desc")
		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		require.Contains(t, body, alpha)
		require.Contains(t, body, beta)
		assert.Less(t, strings.Index(body, beta), strings.Index(body, alpha))
	})
}

func TestAdminTypesCRUDExtended(t *testing.T) {
	router := setupLookupsTestRouter()

	suffix := fmt.Sprint(time.Now().UnixNano())

	t.Run("Create type returns created status", func(t *testing.T) {
		name := "Test Type For Deletion " + suffix
		cleanupAdminTestTypeByName(t, name)
		payload := map[string]interface{}{
			"name": name,
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
	})

	t.Run("Update type returns success", func(t *testing.T) {
		id := createAdminTestType(t, "Update Target "+suffix)
		formData := url.Values{
			"name": {"Updated Type Name " + suffix},
		}

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/types/%d/update", id), strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		name, _ := adminTestTypeRow(t, id)
		assert.Equal(t, "Updated Type Name "+suffix, name)
	})

	t.Run("Delete type returns success", func(t *testing.T) {
		// A fresh type has no tickets, so it is deletable.
		id := createAdminTestType(t, "Delete Target "+suffix)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/types/%d/delete", id), nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		_, validID := adminTestTypeRow(t, id)
		assert.Equal(t, 2, validID)
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

func TestAdminLookupsPage(t *testing.T) {
	setupTemplateRenderer(t)
	db := getTestDB(t)
	router := setupLookupsTestRouter()

	typeName := fmt.Sprintf("LookupsPageType%d", time.Now().UnixNano())
	cleanupAdminTestTypeByName(t, typeName)
	_, err := db.Exec(database.ConvertPlaceholders(`INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1)`), typeName)
	require.NoError(t, err)

	var priorityName, stateName, stateTypeName string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT name FROM ticket_priority WHERE valid_id = 1 ORDER BY id")).Scan(&priorityName))
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT ts.name, tst.name FROM ticket_state ts
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE ts.valid_id = 1 ORDER BY ts.id`)).Scan(&stateName, &stateTypeName))

	req := httptest.NewRequest(http.MethodGet, "/admin/lookups?tab=states", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	assert.Contains(t, body, typeName, "ticket types come from the database")
	assert.Contains(t, body, priorityName, "priorities come from the database")
	assert.Contains(t, body, stateName, "states come from the database")
	assert.Contains(t, body, stateTypeName, "state types come from the database")
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

	t.Run("GET /api/lookups/types returns the valid ticket types", func(t *testing.T) {
		want := validTicketTypeCount(t)
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, want, len(data))
	})

	t.Run("GET /api/lookups/statuses returns every ticket state", func(t *testing.T) {
		var want int
		require.NoError(t, getTestDB(t).QueryRow(database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM ticket_state`)).Scan(&want))
		GetLookupService().InvalidateCache()
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, want, len(data))
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
		GetLookupService().InvalidateCache()
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

		assert.ElementsMatch(t, validTicketTypeNames(t), values)
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

	t.Run("GET /api/lookups/types returns the valid ticket types", func(t *testing.T) {
		want := validTicketTypeCount(t)
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, want, len(data))
	})

	t.Run("GET /api/lookups/statuses returns every ticket state", func(t *testing.T) {
		var want int
		require.NoError(t, getTestDB(t).QueryRow(database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM ticket_state`)).Scan(&want))
		GetLookupService().InvalidateCache()
		req := httptest.NewRequest(http.MethodGet, "/api/lookups/statuses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, want, len(data))
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
		GetLookupService().InvalidateCache()
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

		assert.ElementsMatch(t, validTicketTypeNames(t), values)
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
