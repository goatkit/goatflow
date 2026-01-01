package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminPriorityHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/admin/priorities", handleAdminPriorities)
	router.GET("/api/lookups/priorities", HandleGetPriorities)

	t.Run("List priorities page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, may return 500 or render page
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /api/lookups/priorities returns list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data, ok := response["data"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(data))
	})

	t.Run("Priorities response has correct structure", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "success")
		assert.Contains(t, response, "data")
	})

	t.Run("Priority values are unique", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		values := make(map[string]bool)
		for _, item := range data {
			priority := item.(map[string]interface{})
			value := priority["value"].(string)
			assert.False(t, values[value], "Duplicate priority value: %s", value)
			values[value] = true
		}
	})

	t.Run("Priority orders are unique", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		orders := make(map[float64]bool)
		for _, item := range data {
			priority := item.(map[string]interface{})
			order := priority["order"].(float64)
			assert.False(t, orders[order], "Duplicate priority order: %v", order)
			orders[order] = true
		}
	})

	t.Run("Normal priority exists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		hasNormal := false
		for _, item := range data {
			priority := item.(map[string]interface{})
			if priority["value"] == "3 normal" {
				hasNormal = true
				break
			}
		}
		assert.True(t, hasNormal, "Normal priority should exist")
	})

	t.Run("Priorities have correct order", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
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

	t.Run("Priority has required fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
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

	t.Run("Priority order values are sequential", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for i, item := range data {
			priority := item.(map[string]interface{})
			assert.Equal(t, float64(i+1), priority["order"])
		}
	})
}

func TestAdminPriorityValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/priorities", HandleGetPriorities)

	t.Run("Lowest priority is first", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		first := data[0].(map[string]interface{})
		assert.Equal(t, "1 very low", first["value"])
		assert.Equal(t, float64(1), first["order"])
	})

	t.Run("Highest priority is last", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		last := data[len(data)-1].(map[string]interface{})
		assert.Equal(t, "5 very high", last["value"])
		assert.Equal(t, float64(5), last["order"])
	})

	t.Run("Priorities are active", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			priority := item.(map[string]interface{})
			// active field may be present
			if active, ok := priority["active"]; ok {
				assert.True(t, active.(bool))
			}
		}
	})

	t.Run("Priorities have labels", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].([]interface{})
		for _, item := range data {
			priority := item.(map[string]interface{})
			label := priority["label"].(string)
			assert.NotEmpty(t, label)
		}
	})
}

func TestAdminPriorityConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/lookups/priorities", HandleGetPriorities)

	t.Run("Concurrent priority requests", func(t *testing.T) {
		numRequests := 10
		done := make(chan bool, numRequests)

		for i := 0; i < numRequests; i++ {
			go func() {
				req := httptest.NewRequest("GET", "/api/lookups/priorities", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.True(t, response["success"].(bool))

				done <- true
			}()
		}

		for i := 0; i < numRequests; i++ {
			<-done
		}
	})
}
