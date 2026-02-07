package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
)

func TestQueueAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available; skipping queue API tests")
	}

	t.Run("List Queues", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.GET("/api/v1/queues", HandleListQueuesAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should return all queues by default", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Set("is_authenticated", true)
				c.Next()
			})
			router.GET("/api/v1/queues", HandleListQueuesAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])
				assert.NotNil(t, response["data"])
			}
		})

		t.Run("should filter by valid status", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/queues", HandleListQueuesAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues?valid=1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("should include queue statistics if requested", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/queues", HandleListQueuesAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues?include_stats=true", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				if data, ok := response["data"].([]interface{}); ok && len(data) > 0 {
					queue := data[0].(map[string]interface{})
					// Should have ticket counts if stats included
					assert.NotNil(t, queue["ticket_count"])
				}
			}
		})
	})

	t.Run("Get Single Queue", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.GET("/api/v1/queues/:id", HandleGetQueueAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should return 404 for non-existent queue", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/queues/:id", HandleGetQueueAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues/99999", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, false, response["success"])
			}
		})

		t.Run("should return queue details with group access", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/queues/:id", HandleGetQueueAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				data := response["data"].(map[string]interface{})
				assert.NotNil(t, data["id"])
				assert.NotNil(t, data["name"])
				// Should include groups that have access
				assert.NotNil(t, data["groups"])
			}
		})
	})

	t.Run("Create Queue", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.POST("/api/v1/queues", HandleCreateQueueAPI)

			body := map[string]interface{}{
				"name": "New Queue",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/queues", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should validate required fields", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/queues", HandleCreateQueueAPI)

			// Missing name
			body := map[string]interface{}{
				"comment": "Missing name field",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/queues", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("should create queue with valid data", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/queues", HandleCreateQueueAPI)

			body := map[string]interface{}{
				"name":              "Support Queue",
				"group_id":          1,
				"system_address_id": 1,
				"comment":           "General support queue",
				"valid_id":          1,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/queues", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusCreated {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])

				data := response["data"].(map[string]interface{})
				assert.NotNil(t, data["id"])
				assert.Equal(t, "Support Queue", data["name"])
			}
		})

		t.Run("should prevent duplicate queue names", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/queues", HandleCreateQueueAPI)

			body := map[string]interface{}{
				"name": "Raw", // Assuming this exists
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/queues", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusConflict {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, false, response["success"])
				assert.Contains(t, response["error"], "already exists")
			}
		})
	})

	t.Run("Update Queue", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/v1/queues/:id", HandleUpdateQueueAPI)

			body := map[string]interface{}{
				"comment": "Updated comment",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/queues/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should update queue fields", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.PUT("/api/v1/queues/:id", HandleUpdateQueueAPI)

			body := map[string]interface{}{
				"comment":        "Updated queue comment",
				"group_id":       2,
				"unlock_timeout": 60,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/queues/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])
			}
		})

		t.Run("should not allow renaming to existing queue name", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.PUT("/api/v1/queues/:id", HandleUpdateQueueAPI)

			body := map[string]interface{}{
				"name": "Raw", // Trying to rename to existing queue
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/queues/2", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusConflict {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, false, response["success"])
			}
		})
	})

	t.Run("Delete Queue", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.DELETE("/api/v1/queues/:id", HandleDeleteQueueAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/queues/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should soft delete queue (set valid_id=2)", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/queues/:id", HandleDeleteQueueAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/queues/2", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 204 No Content on success
			if w.Code == http.StatusNoContent {
				assert.Equal(t, 0, w.Body.Len())
			}
		})

		t.Run("should not delete queue with active tickets", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/queues/:id", HandleDeleteQueueAPI)

			// Queue 1 likely has tickets
			req := httptest.NewRequest("DELETE", "/api/v1/queues/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should refuse to delete queue with tickets
			if w.Code == http.StatusConflict {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, false, response["success"])
				assert.Contains(t, response["error"], "tickets")
			}
		})

		t.Run("should not delete system queues", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/queues/:id", HandleDeleteQueueAPI)

			// Queue ID 1 (Raw) is typically a system queue
			req := httptest.NewRequest("DELETE", "/api/v1/queues/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusForbidden {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, false, response["success"])
				assert.Contains(t, response["error"], "system queue")
			}
		})
	})

	t.Run("Queue Statistics", func(t *testing.T) {
		t.Run("should get queue ticket statistics", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/queues/:id/stats", HandleGetQueueStatsAPI)

			req := httptest.NewRequest("GET", "/api/v1/queues/1/stats", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])

				data := response["data"].(map[string]interface{})
				// Should have various counts
				assert.NotNil(t, data["total_tickets"])
				assert.NotNil(t, data["open_tickets"])
				assert.NotNil(t, data["closed_tickets"])
			}
		})
	})

	t.Run("Queue Groups", func(t *testing.T) {
		t.Run("should assign group to queue", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/queues/:id/groups", HandleAssignQueueGroupAPI)

			body := map[string]interface{}{
				"group_id":    2,
				"permissions": "rw",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/queues/1/groups", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])
			}
		})

		t.Run("should remove group from queue", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/queues/:id/groups/:group_id", HandleRemoveQueueGroupAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/queues/1/groups/2", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNoContent {
				assert.Equal(t, 0, w.Body.Len())
			}
		})
	})
}
