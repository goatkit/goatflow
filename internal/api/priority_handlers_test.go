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

func TestGetPriorities(t *testing.T) {
	gin.SetMode(gin.TestMode)

	getTestDB(t) // Initialize test database

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	router.GET("/api/priorities", HandleListPrioritiesAPI)

	t.Run("successful get priorities", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/priorities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data, ok := response["data"].([]interface{})
		require.True(t, ok, "data should be an array")
		require.NotEmpty(t, data, "should have priorities from seed data")

		first := data[0].(map[string]interface{})
		assert.NotNil(t, first["id"])
		assert.NotEmpty(t, first["name"])
	})
}

func TestCreatePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	router.POST("/api/priorities", HandleCreatePriorityAPI)

	testName := "test_priority_" + time.Now().Format("150405")

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_priority WHERE name = ?"), testName)
	}()

	t.Run("successful create priority", func(t *testing.T) {
		body := map[string]interface{}{
			"name":  testName,
			"color": "#ff00ff",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/priorities", bytes.NewBuffer(bodyBytes))
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
		assert.Equal(t, "#ff00ff", data["color"])
		assert.NotNil(t, data["id"])

		var dbName string
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_priority WHERE name = ?"), testName).Scan(&dbName)
		require.NoError(t, err)
		assert.Equal(t, testName, dbName)
	})

	t.Run("missing name", func(t *testing.T) {
		body := map[string]interface{}{
			"color": "#ff00ff",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/priorities", bytes.NewBuffer(bodyBytes))
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

func TestUpdatePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	router.PUT("/api/priorities/:id", HandleUpdatePriorityAPI)

	testName := "update_test_" + time.Now().Format("150405")
	var testID int64

	if database.IsMySQL() {
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO ticket_priority (name, color, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, '#aaaaaa', 1, NOW(), 1, NOW(), 1)
		`), testName)
		require.NoError(t, err)
		testID, _ = result.LastInsertId()
	} else {
		err := db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket_priority (name, color, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, '#aaaaaa', 1, NOW(), 1, NOW(), 1) RETURNING id
		`), testName).Scan(&testID)
		require.NoError(t, err)
	}

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_priority WHERE id = ?"), testID)
	}()

	t.Run("successful update priority", func(t *testing.T) {
		updatedName := testName + "_updated"
		body := map[string]interface{}{
			"name":  updatedName,
			"color": "#bbbbbb",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/priorities/"+strconv.FormatInt(testID, 10), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		var dbName, dbColor string
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name, color FROM ticket_priority WHERE id = ?"), testID).Scan(&dbName, &dbColor)
		require.NoError(t, err)
		assert.Equal(t, updatedName, dbName)
		assert.Equal(t, "#bbbbbb", dbColor)
	})

	t.Run("invalid priority ID", func(t *testing.T) {
		body := map[string]interface{}{"name": "test"}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/priorities/abc", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid priority ID", response["error"])
	})

	t.Run("priority not found", func(t *testing.T) {
		body := map[string]interface{}{"name": "test"}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest("PUT", "/api/priorities/99999", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Priority not found", response["error"])
	})
}

func TestDeletePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	router.DELETE("/api/priorities/:id", HandleDeletePriorityAPI)

	testName := "delete_test_" + time.Now().Format("150405")
	var testID int64

	if database.IsMySQL() {
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO ticket_priority (name, color, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, '#cccccc', 1, NOW(), 1, NOW(), 1)
		`), testName)
		require.NoError(t, err)
		testID, _ = result.LastInsertId()
	} else {
		err := db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket_priority (name, color, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, '#cccccc', 1, NOW(), 1, NOW(), 1) RETURNING id
		`), testName).Scan(&testID)
		require.NoError(t, err)
	}

	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_priority WHERE id = ?"), testID)
	}()

	t.Run("successful delete priority (soft delete)", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/priorities/"+strconv.FormatInt(testID, 10), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Priority deleted successfully", response["message"])

		var validID int
		err = db.QueryRow(database.ConvertPlaceholders("SELECT valid_id FROM ticket_priority WHERE id = ?"), testID).Scan(&validID)
		require.NoError(t, err)
		assert.Equal(t, 2, validID)
	})

	t.Run("invalid priority ID", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/priorities/xyz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid priority ID", response["error"])
	})

	t.Run("priority not found", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/priorities/99999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Priority not found", response["error"])
	})
}
