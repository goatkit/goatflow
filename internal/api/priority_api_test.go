package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func cleanupPriorityTestData(t *testing.T, names ...string) {
	t.Helper()
	if len(names) == 0 {
		return
	}
	db, err := database.GetDB()
	require.NoError(t, err)

	placeholders := make([]string, len(names))
	args := make([]interface{}, len(names))
	for i, name := range names {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = name
	}
	query := fmt.Sprintf("DELETE FROM ticket_priority WHERE name IN (%s)", strings.Join(placeholders, ", "))
	query = database.ConvertPlaceholders(query)
	_, err = db.Exec(query, args...)
	require.NoError(t, err)
}

func TestPriorityAPI(t *testing.T) {
	// Initialize test database
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available; skipping priority API test")
	}
	defer database.CloseTestDB()

	// Create test JWT manager
	jwtManager := auth.NewJWTManager("test-secret", time.Hour)

	// Create test token
	token, _ := jwtManager.GenerateToken(1, "testuser@example.com", "Agent", 0)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	t.Run("List Priorities", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/priorities", HandleListPrioritiesAPI)

		// Test without filter
		req := httptest.NewRequest("GET", "/api/v1/priorities", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var deleteResp struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		json.Unmarshal(w.Body.Bytes(), &deleteResp)
		assert.True(t, deleteResp.Success)
		assert.Equal(t, "Priority deleted successfully", deleteResp.Message)

		var response struct {
			Success bool `json:"success"`
			Data    []struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				Color   string `json:"color"`
				ValidID int    `json:"valid_id"`
			} `json:"data"`
			Error string `json:"error"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.True(t, response.Success)
		assert.NotEmpty(t, response.Data)

		// Test with valid filter
		req = httptest.NewRequest("GET", "/api/v1/priorities?valid=true", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		json.Unmarshal(w.Body.Bytes(), &response)
		for _, priority := range response.Data {
			assert.Equal(t, 1, priority.ValidID)
		}
	})

	t.Run("Get Priority", func(t *testing.T) {
		cleanupPriorityTestData(t, "Test Priority")
		t.Cleanup(func() { cleanupPriorityTestData(t, "Test Priority") })

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/priorities/:id", HandleGetPriorityAPI)

		// Create a test priority first
		db, _ := database.GetDB()
		var priorityID int
		query := database.ConvertPlaceholders(`
			INSERT INTO ticket_priority (name, valid_id, color, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, $2, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		db.QueryRow(query, "Test Priority", "#123456").Scan(&priorityID)

		// Test getting the priority
		req := httptest.NewRequest("GET", "/api/v1/priorities/"+strconv.Itoa(priorityID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var priority struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Color   string `json:"color"`
			ValidID int    `json:"valid_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &priority)
		assert.Equal(t, priorityID, priority.ID)
		assert.Equal(t, "Test Priority", priority.Name)
		assert.Equal(t, "#123456", priority.Color)

		// Test non-existent priority
		req = httptest.NewRequest("GET", "/api/v1/priorities/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Create Priority", func(t *testing.T) {
		cleanupPriorityTestData(t, "New Priority")
		t.Cleanup(func() { cleanupPriorityTestData(t, "New Priority") })

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/priorities", HandleCreatePriorityAPI)

		// Test creating priority
		payload := map[string]interface{}{
			"name": "New Priority",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/priorities", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response struct {
			Success bool `json:"success"`
			Data    struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				Color   string `json:"color"`
				ValidID int    `json:"valid_id"`
			} `json:"data"`
			Error string `json:"error"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.True(t, response.Success)
		assert.NotZero(t, response.Data.ID)
		assert.Equal(t, "New Priority", response.Data.Name)
		assert.Equal(t, "#cdcdcd", response.Data.Color)
		assert.Equal(t, 1, response.Data.ValidID)

		// Test duplicate name
		req = httptest.NewRequest("POST", "/api/v1/priorities", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Update Priority", func(t *testing.T) {
		cleanupPriorityTestData(t, "Update Test Priority", "Updated Priority Name")
		t.Cleanup(func() { cleanupPriorityTestData(t, "Update Test Priority", "Updated Priority Name") })

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/v1/priorities/:id", HandleUpdatePriorityAPI)

		// Create a test priority
		db, _ := database.GetDB()
		var priorityID int
		query := database.ConvertPlaceholders(`
			INSERT INTO ticket_priority (name, valid_id, color, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, $2, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		db.QueryRow(query, "Update Test Priority", "#abcdef").Scan(&priorityID)

		// Test updating priority
		payload := map[string]interface{}{
			"name":  "Updated Priority Name",
			"color": "#83bfc8",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/priorities/"+strconv.Itoa(priorityID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool `json:"success"`
			Data    struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				Color   string `json:"color"`
				ValidID int    `json:"valid_id"`
			} `json:"data"`
			Error string `json:"error"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.True(t, response.Success)
		assert.Equal(t, priorityID, response.Data.ID)
		assert.Equal(t, "Updated Priority Name", response.Data.Name)
		assert.Equal(t, "#83bfc8", response.Data.Color)

		// Test updating non-existent priority
		req = httptest.NewRequest("PUT", "/api/v1/priorities/99999", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Delete Priority", func(t *testing.T) {
		cleanupPriorityTestData(t, "Delete Test Priority")
		t.Cleanup(func() { cleanupPriorityTestData(t, "Delete Test Priority") })

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/v1/priorities/:id", HandleDeletePriorityAPI)

		// Create a test priority
		db, _ := database.GetDB()
		var priorityID int
		query := database.ConvertPlaceholders(`
			INSERT INTO ticket_priority (name, valid_id, color, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, $2, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		db.QueryRow(query, "Delete Test Priority", "#654321").Scan(&priorityID)

		// Test soft deleting priority
		req := httptest.NewRequest("DELETE", "/api/v1/priorities/"+strconv.Itoa(priorityID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify soft delete
		var validID int
		checkQuery := database.ConvertPlaceholders(`
			SELECT valid_id FROM ticket_priority WHERE id = $1
		`)
		db.QueryRow(checkQuery, priorityID).Scan(&validID)
		assert.Equal(t, 2, validID)

		// Test deleting non-existent priority
		req = httptest.NewRequest("DELETE", "/api/v1/priorities/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		// Test preventing deletion of system priorities
		req = httptest.NewRequest("DELETE", "/api/v1/priorities/1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/priorities", HandleListPrioritiesAPI)

		req := httptest.NewRequest("GET", "/api/v1/priorities", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
