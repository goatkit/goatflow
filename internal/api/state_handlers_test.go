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

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestGetStates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	getTestDB(t) // Initialize test database

	router := gin.New()
	router.GET("/api/states", handleGetStates)

	t.Run("successful get states", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/states", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data, ok := response["data"].([]interface{})
		require.True(t, ok, "data should be an array")
		require.NotEmpty(t, data, "should have states from seed data")

		first := data[0].(map[string]interface{})
		assert.NotNil(t, first["id"])
		assert.NotEmpty(t, first["name"])
	})
}

func TestCreateState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.POST("/api/states", handleCreateState)

	testName := "test_state_" + time.Now().Format("150405")

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_state WHERE name = ?"), testName)
	}()

	t.Run("successful create state", func(t *testing.T) {
		body := map[string]interface{}{
			"name":     testName,
			"type_id":  3,
			"comments": "Test state for testing",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/states", bytes.NewBuffer(bodyBytes))
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
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE name = ?"), testName).Scan(&dbName)
		require.NoError(t, err)
		assert.Equal(t, testName, dbName)
	})

	t.Run("missing required fields", func(t *testing.T) {
		body := map[string]interface{}{
			"comments": "Test",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/states", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Name and type_id are required", response["error"])
	})
}

func TestUpdateState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.PUT("/api/states/:id", handleUpdateState)

	testName := "update_state_" + time.Now().Format("150405")
	var testID int64

	if database.IsMySQL() {
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO ticket_state (name, type_id, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 2, 'Test comments', 1, NOW(), 1, NOW(), 1)
		`), testName)
		require.NoError(t, err)
		testID, _ = result.LastInsertId()
	} else {
		err := db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket_state (name, type_id, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 2, 'Test comments', 1, NOW(), 1, NOW(), 1) RETURNING id
		`), testName).Scan(&testID)
		require.NoError(t, err)
	}

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_state WHERE id = ?"), testID)
	}()

	t.Run("successful update state", func(t *testing.T) {
		updatedName := testName + "_updated"
		body := map[string]interface{}{
			"name":     updatedName,
			"comments": "Updated comments",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/states/"+strconv.FormatInt(testID, 10), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		var dbName string
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = ?"), testID).Scan(&dbName)
		require.NoError(t, err)
		assert.Equal(t, updatedName, dbName)
	})

	t.Run("invalid state ID", func(t *testing.T) {
		body := map[string]interface{}{"name": "test"}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/states/abc", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid state ID", response["error"])
	})

	t.Run("state not found", func(t *testing.T) {
		body := map[string]interface{}{"name": "test"}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/states/99999", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "State not found", response["error"])
	})
}

func TestDeleteState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.DELETE("/api/states/:id", handleDeleteState)

	testName := "delete_state_" + time.Now().Format("150405")
	var testID int64

	if database.IsMySQL() {
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO ticket_state (name, type_id, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 2, 'To be deleted', 1, NOW(), 1, NOW(), 1)
		`), testName)
		require.NoError(t, err)
		testID, _ = result.LastInsertId()
	} else {
		err := db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket_state (name, type_id, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 2, 'To be deleted', 1, NOW(), 1, NOW(), 1) RETURNING id
		`), testName).Scan(&testID)
		require.NoError(t, err)
	}

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_state WHERE id = ?"), testID)
	}()

	t.Run("successful delete state (soft delete)", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/states/"+strconv.FormatInt(testID, 10), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "State deleted successfully", response["message"])

		var validID int
		err = db.QueryRow(database.ConvertPlaceholders("SELECT valid_id FROM ticket_state WHERE id = ?"), testID).Scan(&validID)
		require.NoError(t, err)
		assert.Equal(t, 2, validID)
	})

	t.Run("invalid state ID", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/states/xyz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid state ID", response["error"])
	})

	t.Run("state not found", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/states/99999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "State not found", response["error"])
	})
}
