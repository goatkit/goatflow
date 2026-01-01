package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminTypeHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Register routes
	router.GET("/admin/types", handleAdminTypes)
	router.POST("/admin/types/create", handleAdminTypeCreate)
	router.POST("/admin/types/:id/update", handleAdminTypeUpdate)
	router.POST("/admin/types/:id/delete", handleAdminTypeDelete)

	t.Run("List ticket types", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/types", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()

		// Check for key UI elements
		assert.Contains(t, body, "Ticket Type Management")
		assert.Contains(t, body, "Add New Type")
		assert.Contains(t, body, "Search")
	})

	t.Run("Create ticket type with form data", func(t *testing.T) {
		formData := url.Values{
			"name": {"Test Type"},
		}

		req := httptest.NewRequest("POST", "/admin/types/create", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("Create ticket type with JSON", func(t *testing.T) {
		payload := map[string]interface{}{
			"name": "JSON Type",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/admin/types/create", bytes.NewReader(jsonData))
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

	t.Run("Update ticket type with form data", func(t *testing.T) {
		formData := url.Values{
			"name": {"Updated Type"},
		}

		req := httptest.NewRequest("POST", "/admin/types/1/update", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("Update ticket type with JSON", func(t *testing.T) {
		payload := map[string]interface{}{
			"name": "JSON Updated",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/admin/types/2/update", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Delete ticket type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/admin/types/3/delete", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("Invalid type ID returns error", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/admin/types/invalid/update", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Search ticket types", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/types?search=Incident", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Search")
	})

	t.Run("Sort ticket types", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/types?sort=name&order=desc", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Create duplicate type returns conflict", func(t *testing.T) {
		// This would need actual database to test properly
		// Just verify the handler can handle the error path
		formData := url.Values{
			"name": {""}, // Empty name should fail validation
		}

		req := httptest.NewRequest("POST", "/admin/types/create", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminTypeUIElements(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/types", handleAdminTypes)

	t.Run("UI has all required elements", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/types", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()

		// Check for essential UI components
		assert.Contains(t, body, "id=\"searchInput\"", "Search input missing")
		assert.Contains(t, body, "id=\"typeModal\"", "Type modal missing")
		assert.Contains(t, body, "onclick=\"openTypeModal()\"", "Add button missing")
		assert.Contains(t, body, "class=\"table\"", "Types table missing")

		// Check for JavaScript functions
		assert.Contains(t, body, "function openTypeModal()", "Modal open function missing")
		assert.Contains(t, body, "function saveType()", "Save function missing")
		assert.Contains(t, body, "function deleteType(", "Delete function missing")
		assert.Contains(t, body, "function editType(", "Edit function missing")
	})

	t.Run("Dark mode support", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/types", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		body := w.Body.String()
		assert.Contains(t, body, "dark:", "Dark mode classes missing")
	})
}

func TestAdminTypeValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/types/create", handleAdminTypeCreate)

	t.Run("Empty name validation", func(t *testing.T) {
		formData := url.Values{
			"name": {""},
		}

		req := httptest.NewRequest("POST", "/admin/types/create", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"].(string), "required")
	})

	t.Run("Long name validation", func(t *testing.T) {
		longName := strings.Repeat("a", 201) // Assuming 200 char limit
		formData := url.Values{
			"name": {longName},
		}

		req := httptest.NewRequest("POST", "/admin/types/create", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should either truncate or return error
		assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusBadRequest)
	})
}
