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

	"github.com/goatkit/goatflow/internal/database"
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
	t.Setenv("APP_ENV", "test")

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.POST("/api/types", handleCreateType)

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
	t.Setenv("APP_ENV", "test")

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.PUT("/api/types/:id", handleUpdateType)

	testName := "update_type_" + time.Now().Format("150405")
	var testID int64

	// Note: ticket_type table doesn't have a comments column
	if database.IsMySQL() {
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 1, NOW(), 1, NOW(), 1)
		`), testName)
		require.NoError(t, err)
		testID, _ = result.LastInsertId()
	} else {
		err := db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 1, NOW(), 1, NOW(), 1) RETURNING id
		`), testName).Scan(&testID)
		require.NoError(t, err)
	}

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

func TestDeleteType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.DELETE("/api/types/:id", handleDeleteType)

	testName := "delete_type_" + time.Now().Format("150405")
	var testID int64

	// Note: ticket_type table doesn't have a comments column
	if database.IsMySQL() {
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 1, NOW(), 1, NOW(), 1)
		`), testName)
		require.NoError(t, err)
		testID, _ = result.LastInsertId()
	} else {
		err := db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 1, NOW(), 1, NOW(), 1) RETURNING id
		`), testName).Scan(&testID)
		require.NoError(t, err)
	}

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
