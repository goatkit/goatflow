//go:build db

package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDFName generates a unique alphanumeric-only test field name
// OTRS requires field names to contain only [a-zA-Z0-9]
func testDFName(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}

func setupDynamicFieldTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", "Admin")
		c.Next()
	})

	router.GET("/admin/dynamic-fields", handleAdminDynamicFields)
	router.GET("/admin/dynamic-fields/new", handleAdminDynamicFieldNew)
	router.GET("/admin/dynamic-fields/:id", handleAdminDynamicFieldEdit)
	router.POST("/api/dynamic-fields", handleCreateDynamicField)
	router.PUT("/api/dynamic-fields/:id", handleUpdateDynamicField)
	router.DELETE("/api/dynamic-fields/:id", handleDeleteDynamicField)
	router.GET("/api/dynamic-fields", handleGetDynamicFields)

	return router
}

func createTestDynamicField(t *testing.T, db *sql.DB, name string, fieldType string, objectType string) int64 {
	var fieldID int64
	now := time.Now()
	config := "---\nDefaultValue: test\n"

	if database.IsMySQL() {
		result, err := db.Exec(`
			INSERT INTO dynamic_field (internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
			VALUES (0, ?, ?, 1, ?, ?, ?, 1, ?, 1, ?, 1)
		`, name, name, fieldType, objectType, config, now, now)
		require.NoError(t, err, "Failed to create test dynamic field")
		fieldID, err = result.LastInsertId()
		require.NoError(t, err)
		return fieldID
	}

	err := db.QueryRow(database.ConvertPlaceholders(`
		INSERT INTO dynamic_field (internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, $1, $2, 1, $3, $4, $5, 1, $6, 1, $7, 1)
		RETURNING id
	`), name, name, fieldType, objectType, config, now, now).Scan(&fieldID)
	require.NoError(t, err, "Failed to create test dynamic field")

	return fieldID
}

func cleanupTestDynamicField(t *testing.T, db *sql.DB, fieldID int64) {
	_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM dynamic_field_value WHERE field_id = $1"), fieldID)
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM dynamic_field WHERE id = $1"), fieldID)
	require.NoError(t, err, "Failed to cleanup test dynamic field")
}

func cleanupTestDynamicFieldByName(t *testing.T, db *sql.DB, name string) {
	var fieldID int64
	_ = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM dynamic_field WHERE name = $1"), name).Scan(&fieldID)
	if fieldID > 0 {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM dynamic_field_value WHERE field_id = $1"), fieldID)
	}
	_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM dynamic_field WHERE name = $1"), name)
}

func TestAdminDynamicFieldsPage(t *testing.T) {
	router := setupDynamicFieldTestRouter()

	t.Run("GET /admin/dynamic-fields renders page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dynamic-fields", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, "Dynamic Field")
		}
	})

	t.Run("Page contains field type tabs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dynamic-fields", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, "Ticket")
			assert.Contains(t, body, "Article")
		}
	})
}

func TestAdminDynamicFieldNewPage(t *testing.T) {
	router := setupDynamicFieldTestRouter()

	t.Run("GET /admin/dynamic-fields/new renders form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dynamic-fields/new", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, "form")
			assert.Contains(t, body, "name")
			assert.Contains(t, body, "field_type")
		}
	})

	t.Run("Form contains all field types", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dynamic-fields/new", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			for _, ft := range ValidFieldTypes() {
				assert.Contains(t, body, ft)
			}
		}
	})
}

func TestAdminDynamicFieldEditPage(t *testing.T) {
	db := getTestDB(t)
	router := setupDynamicFieldTestRouter()

	testFieldName := testDFName("TestDFEdit")
	fieldID := createTestDynamicField(t, db, testFieldName, DFTypeText, DFObjectTicket)
	defer cleanupTestDynamicField(t, db, fieldID)

	t.Run("GET /admin/dynamic-fields/:id loads existing field", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/dynamic-fields/%d", fieldID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, testFieldName)
		}
	})

	t.Run("GET /admin/dynamic-fields/:id returns 404 for invalid ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dynamic-fields/999999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestCreateDynamicFieldWithDB(t *testing.T) {
	db := getTestDB(t)
	router := setupDynamicFieldTestRouter()

	t.Run("POST creates Text field successfully", func(t *testing.T) {
		testFieldName := testDFName("TestDFCreate")
		defer cleanupTestDynamicFieldByName(t, db, testFieldName)

		form := url.Values{}
		form.Set("name", testFieldName)
		form.Set("label", "Test Label")
		form.Set("field_type", DFTypeText)
		form.Set("object_type", DFObjectTicket)
		form.Set("field_order", "1")

		req := httptest.NewRequest(http.MethodPost, "/api/dynamic-fields", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated || w.Code == http.StatusSeeOther,
			"Expected success status, got %d: %s", w.Code, w.Body.String())
	})

	t.Run("POST creates Dropdown field with options", func(t *testing.T) {
		testFieldName := testDFName("TestDFDropdown")
		defer cleanupTestDynamicFieldByName(t, db, testFieldName)

		form := url.Values{}
		form.Set("name", testFieldName)
		form.Set("label", "Priority Level")
		form.Set("field_type", DFTypeDropdown)
		form.Set("object_type", DFObjectTicket)
		form.Set("field_order", "2")
		form.Set("possible_values", "High\nMedium\nLow")

		req := httptest.NewRequest(http.MethodPost, "/api/dynamic-fields", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated || w.Code == http.StatusSeeOther,
			"Expected success status, got %d", w.Code)
	})

	t.Run("POST fails with invalid field type", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "InvalidTypeField")
		form.Set("label", "Test")
		form.Set("field_type", "InvalidType")
		form.Set("object_type", DFObjectTicket)

		req := httptest.NewRequest(http.MethodPost, "/api/dynamic-fields", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST fails with missing name", func(t *testing.T) {
		form := url.Values{}
		form.Set("label", "Test Label")
		form.Set("field_type", DFTypeText)
		form.Set("object_type", DFObjectTicket)

		req := httptest.NewRequest(http.MethodPost, "/api/dynamic-fields", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST fails with non-alphanumeric name", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "invalid-name")
		form.Set("label", "Test Label")
		form.Set("field_type", DFTypeText)
		form.Set("object_type", DFObjectTicket)

		req := httptest.NewRequest(http.MethodPost, "/api/dynamic-fields", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST fails with duplicate name", func(t *testing.T) {
		testFieldName := testDFName("TestDFDupe")
		fieldID := createTestDynamicField(t, db, testFieldName, DFTypeText, DFObjectTicket)
		defer cleanupTestDynamicField(t, db, fieldID)

		form := url.Values{}
		form.Set("name", testFieldName)
		form.Set("label", "Duplicate")
		form.Set("field_type", DFTypeText)
		form.Set("object_type", DFObjectTicket)

		req := httptest.NewRequest(http.MethodPost, "/api/dynamic-fields", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestUpdateDynamicFieldWithDB(t *testing.T) {
	db := getTestDB(t)
	router := setupDynamicFieldTestRouter()

	t.Run("PUT updates field label", func(t *testing.T) {
		testFieldName := testDFName("TestDFUpdate")
		fieldID := createTestDynamicField(t, db, testFieldName, DFTypeText, DFObjectTicket)
		defer cleanupTestDynamicField(t, db, fieldID)

		form := url.Values{}
		form.Set("name", testFieldName)
		form.Set("label", "Updated Label")
		form.Set("field_type", DFTypeText)
		form.Set("object_type", DFObjectTicket)
		form.Set("field_order", "5")

		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/dynamic-fields/%d", fieldID), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNoContent,
			"Expected success status, got %d", w.Code)

		var label string
		_ = db.QueryRow(database.ConvertPlaceholders("SELECT label FROM dynamic_field WHERE id = $1"), fieldID).Scan(&label)
		assert.Equal(t, "Updated Label", label)
	})

	t.Run("PUT returns 404 for non-existent field", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "NonExistent")
		form.Set("label", "Test")
		form.Set("field_type", DFTypeText)
		form.Set("object_type", DFObjectTicket)

		req := httptest.NewRequest(http.MethodPut, "/api/dynamic-fields/999999", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestDeleteDynamicFieldWithDB(t *testing.T) {
	db := getTestDB(t)
	router := setupDynamicFieldTestRouter()

	t.Run("DELETE removes field", func(t *testing.T) {
		testFieldName := testDFName("TestDFDelete")
		fieldID := createTestDynamicField(t, db, testFieldName, DFTypeText, DFObjectTicket)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/dynamic-fields/%d", fieldID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNoContent,
			"Expected success status, got %d", w.Code)

		var count int
		_ = db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM dynamic_field WHERE id = $1"), fieldID).Scan(&count)
		assert.Equal(t, 0, count)
	})

	t.Run("DELETE returns 404 for non-existent field", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/dynamic-fields/999999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetDynamicFieldsAPI(t *testing.T) {
	db := getTestDB(t)
	router := setupDynamicFieldTestRouter()

	testFieldName := testDFName("TestDFGet")
	fieldID := createTestDynamicField(t, db, testFieldName, DFTypeCheckbox, DFObjectArticle)
	defer cleanupTestDynamicField(t, db, fieldID)

	t.Run("GET returns all fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/dynamic-fields", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), testFieldName)
	})

	t.Run("GET filters by object_type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/dynamic-fields?object_type=Article", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), testFieldName)
	})

	t.Run("GET filters by field_type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/dynamic-fields?field_type=Checkbox", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), testFieldName)
	})
}

func TestAllFieldTypesCanBeCreated(t *testing.T) {
	db := getTestDB(t)
	router := setupDynamicFieldTestRouter()

	fieldTypes := []struct {
		fieldType      string
		extraFormField string
		extraValue     string
	}{
		{DFTypeText, "", ""},
		{DFTypeTextArea, "", ""},
		{DFTypeCheckbox, "", ""},
		{DFTypeDropdown, "possible_values", "Option1\nOption2"},
		{DFTypeMultiselect, "possible_values", "Option1\nOption2\nOption3"},
		{DFTypeDate, "", ""},
		{DFTypeDateTime, "", ""},
	}

	for _, tc := range fieldTypes {
		t.Run(fmt.Sprintf("Create %s field", tc.fieldType), func(t *testing.T) {
			testFieldName := testDFName("TestDF" + tc.fieldType)
			defer cleanupTestDynamicFieldByName(t, db, testFieldName)

			form := url.Values{}
			form.Set("name", testFieldName)
			form.Set("label", fmt.Sprintf("Test %s", tc.fieldType))
			form.Set("field_type", tc.fieldType)
			form.Set("object_type", DFObjectTicket)
			form.Set("field_order", "1")

			if tc.extraFormField != "" {
				form.Set(tc.extraFormField, tc.extraValue)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/dynamic-fields", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated || w.Code == http.StatusSeeOther,
				"Expected success for %s, got %d: %s", tc.fieldType, w.Code, w.Body.String())
		})
	}
}

func TestScreenConfigHandler(t *testing.T) {
	router := setupScreenConfigTestRouter()

	t.Run("GET screen config page renders", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dynamic-fields/screens", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Screen Configuration")
	})

	t.Run("GET screen config page with object_type filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dynamic-fields/screens?object_type=Article", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func setupScreenConfigTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", "Admin")
		c.Next()
	})

	router.GET("/admin/dynamic-fields/screens", handleAdminDynamicFieldScreenConfig)
	router.PUT("/api/dynamic-fields/:id/screens", handleAdminDynamicFieldScreenConfigSave)
	router.POST("/api/dynamic-fields/:id/screen", handleAdminDynamicFieldScreenConfigSingle)

	return router
}
