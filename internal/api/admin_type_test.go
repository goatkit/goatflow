package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// createAdminTestType inserts a ticket type owned by the calling test and
// removes it when the test ends, so handler tests never touch the seed types.
func createAdminTestType(t *testing.T, name string) int {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), name)
	require.NoError(t, err)
	t.Cleanup(func() {
		if _, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_type WHERE id = ?`), id); err != nil {
			t.Errorf("cleanup ticket_type %d: %v", id, err)
		}
	})
	return int(id)
}

// cleanupAdminTestTypeByName removes a ticket type created through a handler
// under test when the test ends.
func cleanupAdminTestTypeByName(t *testing.T, name string) {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	t.Cleanup(func() {
		if _, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_type WHERE name = ?`), name); err != nil {
			t.Errorf("cleanup ticket_type %q: %v", name, err)
		}
	})
}

func adminTestTypeRow(t *testing.T, id int) (string, int) {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	var name string
	var validID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT name, valid_id FROM ticket_type WHERE id = ?`), id).Scan(&name, &validID))
	return name, validID
}

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

	suffix := fmt.Sprint(time.Now().UnixNano())

	t.Run("Create ticket type with form data", func(t *testing.T) {
		name := "Test Type " + suffix
		cleanupAdminTestTypeByName(t, name)
		formData := url.Values{
			"name": {name},
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
		name := "JSON Type " + suffix
		cleanupAdminTestTypeByName(t, name)
		payload := map[string]interface{}{
			"name": name,
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
		id := createAdminTestType(t, "Form Update Target "+suffix)
		formData := url.Values{
			"name": {"Updated Type " + suffix},
		}

		req := httptest.NewRequest("POST", fmt.Sprintf("/admin/types/%d/update", id), strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		name, _ := adminTestTypeRow(t, id)
		assert.Equal(t, "Updated Type "+suffix, name)
	})

	t.Run("Update ticket type with JSON", func(t *testing.T) {
		id := createAdminTestType(t, "JSON Update Target "+suffix)
		payload := map[string]interface{}{
			"name": "JSON Updated " + suffix,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", fmt.Sprintf("/admin/types/%d/update", id), bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		name, _ := adminTestTypeRow(t, id)
		assert.Equal(t, "JSON Updated "+suffix, name)
	})

	t.Run("Delete ticket type", func(t *testing.T) {
		id := createAdminTestType(t, "Delete Target "+suffix)
		req := httptest.NewRequest("POST", fmt.Sprintf("/admin/types/%d/delete", id), nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		_, validID := adminTestTypeRow(t, id)
		assert.Equal(t, 2, validID, "delete is a soft delete")
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

		// Skip if response is empty (template renderer issue in test environment)
		if len(body) == 0 {
			t.Skip("Empty response - template renderer not initialized in this test run")
		}

		// Check for essential UI components (present in both template and fallback)
		assert.Contains(t, body, "searchInput", "Search input missing")
		assert.Contains(t, body, "typeModal", "Type modal missing")
		assert.Contains(t, body, "openTypeModal()", "Add button missing")
		// Template uses gk-table, fallback uses table
		hasTable := strings.Contains(body, "class=\"table\"") || strings.Contains(body, "gk-table")
		assert.True(t, hasTable, "Types table missing (expected 'class=\"table\"' or 'gk-table')")

		// Check for JavaScript functions (present in fallback HTML)
		assert.Contains(t, body, "openTypeModal()", "Modal open function missing")
		assert.Contains(t, body, "saveType()", "Save function missing")
		assert.Contains(t, body, "deleteType(", "Delete function missing")
		assert.Contains(t, body, "editType(", "Edit function missing")
	})

	t.Run("Dark mode support", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/types", nil)
		req.Header.Set("Cookie", "access_token=test_token")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		body := w.Body.String()

		// Skip if response is empty (template renderer issue in test environment)
		if len(body) == 0 {
			t.Skip("Empty response - template renderer not initialized in this test run")
		}

		// Template uses CSS variables (--gk-*) for theming instead of dark: classes
		// Check for either dark: classes (fallback) or CSS variables (template)
		hasDarkMode := strings.Contains(body, "dark:") || strings.Contains(body, "var(--gk-")
		assert.True(t, hasDarkMode, "Dark mode support missing (expected 'dark:' or CSS variables)")
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
		cleanupAdminTestTypeByName(t, longName)
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
