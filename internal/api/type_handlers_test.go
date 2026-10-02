package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func TestGetTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	getTestDB(t) // Initialize test database

	router := gin.New()
	router.GET("/api/types", HandleGetTypes)

	t.Run("successful get types", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data, ok := response["data"].([]interface{})
		require.True(t, ok, "data should be an array")
		require.NotEmpty(t, data, "should have types from seed data or fallback")
	})
}

func TestCreateType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := typeCRUDRouter("Admin")
	testName := "test_type_" + time.Now().Format("150405")

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_type WHERE name = ?"), testName)
	}()

	t.Run("successful create type", func(t *testing.T) {
		body := map[string]interface{}{
			"name":     testName,
			"comments": "Test type for testing",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/types", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Equal(t, testName, data["name"])
		assert.NotNil(t, data["id"])

		var dbName string
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_type WHERE name = ?"), testName).Scan(&dbName)
		require.NoError(t, err)
		assert.Equal(t, testName, dbName)
	})

	t.Run("missing name", func(t *testing.T) {
		body := map[string]interface{}{
			"comments": "Test",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/types", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Name is required", response["error"])
	})
}

func TestUpdateType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := typeCRUDRouter("Admin")
	testName := "update_type_" + time.Now().Format("150405")
	// Note: ticket_type table doesn't have a comments column
	testID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1) RETURNING id
	`), testName)
	require.NoError(t, err)

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_type WHERE id = ?"), testID)
	}()

	t.Run("successful update type", func(t *testing.T) {
		updatedName := testName + "_updated"
		body := map[string]interface{}{
			"name":     updatedName,
			"comments": "Updated comments",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/types/"+strconv.FormatInt(testID, 10), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		var dbName string
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_type WHERE id = ?"), testID).Scan(&dbName)
		require.NoError(t, err)
		assert.Equal(t, updatedName, dbName)
	})

	t.Run("invalid type ID", func(t *testing.T) {
		body := map[string]interface{}{"name": "test"}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/types/abc", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid type ID", response["error"])
	})

	t.Run("type not found", func(t *testing.T) {
		body := map[string]interface{}{"name": "test"}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/types/99999", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Type not found", response["error"])
	})
}

// A PUT that omits a field must keep its stored value: omitting name used to
// blank it and omitting valid_id used to re-validate an invalid type.
func TestUpdateTypePartialKeepsOmittedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	router := typeCRUDRouter("Admin")
	name := "partial_type_" + time.Now().Format("150405.000000")
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1) RETURNING id
	`), name)
	require.NoError(t, err)
	renamed := name + "_r"
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_type WHERE id = ?"), id)
	})
	path := "/api/types/" + strconv.FormatInt(id, 10)
	stored := func() (string, int) {
		var n string
		var v int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT name, valid_id FROM ticket_type WHERE id = ?"), id).Scan(&n, &v))
		return n, v
	}
	put := func(body string) map[string]any {
		req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp.Data
	}

	data := put(`{"valid_id":2}`)
	n, v := stored()
	assert.Equal(t, name, n, "omitted name must be kept")
	assert.Equal(t, 2, v)
	assert.Equal(t, name, data["name"])

	data = put(`{"name":"` + renamed + `"}`)
	n, v = stored()
	assert.Equal(t, renamed, n)
	assert.Equal(t, 2, v, "omitted valid_id must be kept")
	assert.EqualValues(t, 2, data["valid_id"])

	req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(`{"name":"  "}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	n, _ = stored()
	assert.Equal(t, renamed, n)
}

func TestDeleteType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := typeCRUDRouter("Admin")
	testName := "delete_type_" + time.Now().Format("150405")
	// Note: ticket_type table doesn't have a comments column
	testID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1) RETURNING id
	`), testName)
	require.NoError(t, err)

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_type WHERE id = ?"), testID)
	}()

	t.Run("successful delete type (soft delete)", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/types/"+strconv.FormatInt(testID, 10), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Type deleted successfully", response["message"])

		var validID int
		err = db.QueryRow(database.ConvertPlaceholders("SELECT valid_id FROM ticket_type WHERE id = ?"), testID).Scan(&validID)
		require.NoError(t, err)
		assert.Equal(t, 2, validID)
	})

	t.Run("invalid type ID", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/types/xyz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid type ID", response["error"])
	})

	t.Run("type not found", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/types/99999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Type not found", response["error"])
	})
}

func typeCRUDRouter(role string) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_role", role)
		c.Set("user_id", 1)
		c.Next()
	})
	router.POST("/api/types", handleCreateType)
	router.PUT("/api/types/:id", handleUpdateType)
	router.DELETE("/api/types/:id", handleDeleteType)
	return router
}

// Non-admins must be refused even when APP_ENV=test (the handlers used to skip the check there).
func TestTypeCRUDRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")

	db := getTestDB(t)
	router := typeCRUDRouter("Agent")

	testName := "perm_type_" + time.Now().Format("150405.000000")
	testID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1) RETURNING id
	`), testName)
	require.NoError(t, err)
	createdName := testName + "_new"
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_type WHERE id = ? OR name = ?"), testID, createdName)
	})
	idPath := "/api/types/" + strconv.FormatInt(testID, 10)

	cases := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/types", `{"name":"` + createdName + `"}`},
		{http.MethodPut, idPath, `{"name":"` + testName + `_renamed","valid_id":2}`},
		{http.MethodDelete, idPath, ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, "%s %s: %s", tc.method, tc.path, w.Body.String())
		assert.JSONEq(t, `{"success":false,"error":"Admin access required"}`, w.Body.String())
	}

	var name string
	var validID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT name, valid_id FROM ticket_type WHERE id = ?"), testID).Scan(&name, &validID))
	assert.Equal(t, testName, name)
	assert.Equal(t, 1, validID)
	var created int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM ticket_type WHERE name = ?"), createdName).Scan(&created))
	assert.Zero(t, created)
}
