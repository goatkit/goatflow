//go:build integration

package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
)

func TestCannedResponseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	testData := setupCannedResponseTestData(t, db)
	defer cleanupCannedResponseTestData(t, db, testData)

	t.Run("CreateCannedResponse", func(t *testing.T) {
		testCreateCannedResponse(t, db)
	})

	t.Run("GetCannedResponses", func(t *testing.T) {
		testGetCannedResponses(t, db, testData)
	})

	t.Run("UpdateCannedResponse", func(t *testing.T) {
		testUpdateCannedResponse(t, db, testData)
	})

	t.Run("UseCannedResponse", func(t *testing.T) {
		testUseCannedResponse(t, db, testData)
	})

	t.Run("DeleteCannedResponse", func(t *testing.T) {
		testDeleteCannedResponse(t, db, testData)
	})

	t.Run("CopyCannedResponse", func(t *testing.T) {
		testCopyCannedResponse(t, db, testData)
	})

	t.Run("GetCategories", func(t *testing.T) {
		testGetCategories(t, db)
	})
}

type cannedResponseTestData struct {
	personalResponseID int
	teamResponseID     int
	globalResponseID   int
	testUserID         int
}

func setupCannedResponseTestData(t *testing.T, db *sql.DB) *cannedResponseTestData {
	t.Helper()

	data := &cannedResponseTestData{testUserID: 1}

	// Cleanup any leftover test data
	db.Exec(`DELETE FROM canned_response WHERE name LIKE 'IntTest%'`)

	// Create personal response
	result, err := db.Exec(`
		INSERT INTO canned_response 
		(name, category, content, content_type, tags, scope, owner_id, placeholders, usage_count, valid_id, create_time, create_by, change_time, change_by)
		VALUES ('IntTest Personal Response', 'General', 'Thank you for contacting support.', 'text', '["greeting"]', 'personal', 1, '[]', 0, 1, NOW(), 1, NOW(), 1)
	`)
	require.NoError(t, err)
	id, _ := result.LastInsertId()
	data.personalResponseID = int(id)

	// Create team response with placeholders
	result, err = db.Exec(`
		INSERT INTO canned_response 
		(name, category, content, content_type, tags, scope, owner_id, team_id, placeholders, usage_count, valid_id, create_time, create_by, change_time, change_by)
		VALUES ('IntTest Team Response', 'Account', 'Hello {{customer_name}}, your ticket #{{ticket_id}} has been updated.', 'text', '["ticket","update"]', 'team', 1, 1, '["customer_name","ticket_id"]', 5, 1, NOW(), 1, NOW(), 1)
	`)
	require.NoError(t, err)
	id, _ = result.LastInsertId()
	data.teamResponseID = int(id)

	// Create global response
	result, err = db.Exec(`
		INSERT INTO canned_response 
		(name, category, content, content_type, tags, scope, owner_id, placeholders, usage_count, valid_id, create_time, create_by, change_time, change_by)
		VALUES ('IntTest Global Response', 'System', 'System maintenance in progress.', 'text', '["system"]', 'global', 1, '[]', 10, 1, NOW(), 1, NOW(), 1)
	`)
	require.NoError(t, err)
	id, _ = result.LastInsertId()
	data.globalResponseID = int(id)

	return data
}

func cleanupCannedResponseTestData(t *testing.T, db *sql.DB, data *cannedResponseTestData) {
	t.Helper()
	db.Exec(`DELETE FROM canned_response WHERE name LIKE 'IntTest%'`)
}

//nolint:unparam // userID is intentionally always 1 in tests for simplicity
func createTestRouter(userID int, userRole string, teamID int) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", userRole)
		if teamID > 0 {
			c.Set("team_id", teamID)
		}
		c.Next()
	})
	RegisterCannedResponseHandlers(r.Group("/api"))
	return r
}

func testCreateCannedResponse(t *testing.T, db *sql.DB) {
	router := createTestRouter(1, "agent", 1)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name: "Create personal canned response",
			payload: map[string]interface{}{
				"name":     "IntTest New Personal",
				"category": "General",
				"content":  "This is a new personal response.",
				"tags":     []string{"new", "test"},
				"scope":    "personal",
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Canned response created successfully", resp["message"])
				assert.Contains(t, resp, "id")
				response := resp["response"].(map[string]interface{})
				assert.Equal(t, "IntTest New Personal", response["name"])
				assert.Equal(t, "personal", response["scope"])
			},
		},
		{
			name: "Create with placeholders auto-extracted",
			payload: map[string]interface{}{
				"name":    "IntTest With Placeholders",
				"content": "Hello {{name}}, your order {{order_id}} is ready.",
				"scope":   "personal",
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				response := resp["response"].(map[string]interface{})
				placeholders := response["placeholders"].([]interface{})
				assert.Contains(t, placeholders, "name")
				assert.Contains(t, placeholders, "order_id")
			},
		},
		{
			name: "Fail on missing required fields",
			payload: map[string]interface{}{
				"category": "General",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "Fail on duplicate name in same scope",
			payload: map[string]interface{}{
				"name":    "IntTest New Personal",
				"content": "Duplicate name test",
				"scope":   "personal",
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/canned-responses", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.checkResp != nil {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				tt.checkResp(t, resp)
			}
		})
	}

	// Cleanup test-created responses
	db.Exec(`DELETE FROM canned_response WHERE name LIKE 'IntTest New%' OR name LIKE 'IntTest With%'`)
}

func testGetCannedResponses(t *testing.T, db *sql.DB, data *cannedResponseTestData) {
	router := createTestRouter(1, "agent", 1)

	t.Run("List all accessible responses", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/canned-responses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		responses := resp["responses"].([]interface{})
		assert.GreaterOrEqual(t, len(responses), 3)
	})

	t.Run("Filter by category", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/canned-responses?category=General", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		responses := resp["responses"].([]interface{})

		for _, r := range responses {
			response := r.(map[string]interface{})
			assert.Equal(t, "General", response["category"])
		}
	})

	t.Run("Filter by scope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/canned-responses?scope=global", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		responses := resp["responses"].([]interface{})

		for _, r := range responses {
			response := r.(map[string]interface{})
			assert.Equal(t, "global", response["scope"])
		}
	})

	t.Run("Search by text", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/canned-responses?search=maintenance", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		responses := resp["responses"].([]interface{})
		assert.GreaterOrEqual(t, len(responses), 1)
	})
}

func testUpdateCannedResponse(t *testing.T, db *sql.DB, data *cannedResponseTestData) {
	router := createTestRouter(1, "agent", 1)

	t.Run("Update own personal response", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":    "IntTest Personal Response Updated",
			"content": "Updated content here.",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/api/canned-responses/"+itoa(data.personalResponseID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		response := resp["response"].(map[string]interface{})
		assert.Equal(t, "IntTest Personal Response Updated", response["name"])
	})

	t.Run("Cannot update non-existent response", func(t *testing.T) {
		payload := map[string]interface{}{"name": "Test"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/api/canned-responses/999999", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func testUseCannedResponse(t *testing.T, db *sql.DB, data *cannedResponseTestData) {
	router := createTestRouter(1, "agent", 1)

	t.Run("Use response without placeholders", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/canned-responses/"+itoa(data.personalResponseID)+"/use", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["content"], "support")
	})

	t.Run("Use response with placeholder substitution", func(t *testing.T) {
		payload := map[string]interface{}{
			"context": map[string]string{
				"customer_name": "John Doe",
				"ticket_id":     "12345",
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/canned-responses/"+itoa(data.teamResponseID)+"/use", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		content := resp["content"].(string)
		assert.Contains(t, content, "John Doe")
		assert.Contains(t, content, "12345")
		assert.NotContains(t, content, "{{")
	})

	t.Run("Usage count incremented", func(t *testing.T) {
		var usageCount int
		db.QueryRow("SELECT usage_count FROM canned_response WHERE id = ?", data.teamResponseID).Scan(&usageCount)
		initialCount := usageCount

		req := httptest.NewRequest(http.MethodPost, "/api/canned-responses/"+itoa(data.teamResponseID)+"/use", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		db.QueryRow("SELECT usage_count FROM canned_response WHERE id = ?", data.teamResponseID).Scan(&usageCount)
		assert.Equal(t, initialCount+1, usageCount)
	})
}

func testDeleteCannedResponse(t *testing.T, db *sql.DB, data *cannedResponseTestData) {
	// Create a response to delete
	result, _ := db.Exec(`
		INSERT INTO canned_response 
		(name, content, content_type, scope, owner_id, valid_id, create_time, create_by, change_time, change_by)
		VALUES ('IntTest To Delete', 'Delete me', 'text', 'personal', 1, 1, NOW(), 1, NOW(), 1)
	`)
	id, _ := result.LastInsertId()

	router := createTestRouter(1, "agent", 1)

	t.Run("Delete own response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/canned-responses/"+itoa(int(id)), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify soft delete (valid_id = 2)
		var validID int
		db.QueryRow("SELECT valid_id FROM canned_response WHERE id = ?", id).Scan(&validID)
		assert.Equal(t, 2, validID)
	})

	t.Run("Cannot delete non-existent response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/canned-responses/999999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func testCopyCannedResponse(t *testing.T, db *sql.DB, data *cannedResponseTestData) {
	router := createTestRouter(1, "agent", 1)

	t.Run("Copy global response to personal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/canned-responses/"+itoa(data.globalResponseID)+"/copy", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		response := resp["response"].(map[string]interface{})
		assert.Contains(t, response["name"], "(Copy)")
		assert.Equal(t, "personal", response["scope"])
	})

	// Cleanup copied response
	db.Exec(`DELETE FROM canned_response WHERE name LIKE '%Copy%'`)
}

func testGetCategories(t *testing.T, db *sql.DB) {
	router := createTestRouter(1, "agent", 1)

	req := httptest.NewRequest(http.MethodGet, "/api/canned-responses/categories", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	categories := resp["categories"].([]interface{})
	assert.GreaterOrEqual(t, len(categories), 1)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
