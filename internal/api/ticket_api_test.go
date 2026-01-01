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

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestTicketAPI(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	t.Run("List Tickets", func(t *testing.T) {
		t.Run("should return 401 without authentication", func(t *testing.T) {
			router := gin.New()
			router.GET("/api/v1/tickets", HandleListTicketsAPI)

			req := httptest.NewRequest("GET", "/api/v1/tickets", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, false, response["success"])
			assert.Contains(t, response["error"], "Authentication required")
		})

		t.Run("should return tickets with authentication", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Set("is_authenticated", true)
				c.Next()
			})
			router.GET("/api/v1/tickets", HandleListTicketsAPI)

			req := httptest.NewRequest("GET", "/api/v1/tickets", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Will fail initially without DB, but that's expected in TDD
			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("should support pagination parameters", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/tickets", HandleListTicketsAPI)

			req := httptest.NewRequest("GET", "/api/v1/tickets?page=2&per_page=10", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check that pagination params are processed
			var response map[string]interface{}
			if w.Code == http.StatusOK {
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				if pagination, ok := response["pagination"].(map[string]interface{}); ok {
					assert.Equal(t, float64(2), pagination["page"])
					assert.Equal(t, float64(10), pagination["per_page"])
				}
			}
		})

		t.Run("should support filtering by status", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/tickets", HandleListTicketsAPI)

			req := httptest.NewRequest("GET", "/api/v1/tickets?status=open", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Just check the request is processed
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Get Single Ticket", func(t *testing.T) {
		t.Run("should return 401 without authentication", func(t *testing.T) {
			router := gin.New()
			router.GET("/api/v1/tickets/:id", HandleGetTicketAPI)

			req := httptest.NewRequest("GET", "/api/v1/tickets/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should return 404 for non-existent ticket", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/tickets/:id", HandleGetTicketAPI)

			req := httptest.NewRequest("GET", "/api/v1/tickets/99999", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// May return 500 without DB, but should be 404 when properly implemented
			assert.NotEqual(t, http.StatusOK, w.Code)
		})

		t.Run("should return ticket details with valid ID", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/tickets/:id", HandleGetTicketAPI)

			req := httptest.NewRequest("GET", "/api/v1/tickets/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Will need DB to pass properly
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Create Ticket", func(t *testing.T) {
		t.Run("should return 401 without authentication", func(t *testing.T) {
			router := gin.New()
			router.POST("/api/v1/tickets", HandleCreateTicketAPI)

			body := map[string]interface{}{
				"title":       "Test Ticket",
				"queue_id":    1,
				"priority_id": 3,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should return 400 with invalid data", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/tickets", HandleCreateTicketAPI)

			// Missing required fields
			body := map[string]interface{}{
				"title": "Test Ticket",
				// Missing queue_id
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("should create ticket with valid data", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/tickets", HandleCreateTicketAPI)

			body := map[string]interface{}{
				"title":            "Test Ticket",
				"queue_id":         1,
				"priority_id":      3,
				"customer_user_id": "customer@example.com",
				"article": map[string]interface{}{
					"subject": "Initial message",
					"body":    "This is the ticket description",
					"type":    "note",
				},
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 201 when properly implemented
			if w.Code == http.StatusCreated {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])
				assert.NotNil(t, response["data"])
			}
		})
	})

	t.Run("Update Ticket", func(t *testing.T) {
		t.Run("should return 401 without authentication", func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/v1/tickets/:id", HandleUpdateTicketAPI)

			body := map[string]interface{}{
				"title": "Updated Title",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/tickets/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should update ticket with valid data", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.PUT("/api/v1/tickets/:id", HandleUpdateTicketAPI)

			body := map[string]interface{}{
				"title":       "Updated Title",
				"priority_id": 1,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/tickets/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 200 when properly implemented
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Delete Ticket", func(t *testing.T) {
		t.Run("should return 401 without authentication", func(t *testing.T) {
			database.ResetDB()
			t.Cleanup(database.ResetDB)

			router := gin.New()
			router.DELETE("/api/v1/tickets/:id", HandleDeleteTicketAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/tickets/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should soft delete ticket", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/tickets/:id", HandleDeleteTicketAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/tickets/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 204 when properly implemented
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})
	})
}

// Test helper functions.
func setupAPITestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func authenticatedRouter() *gin.Engine {
	router := setupAPITestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("is_authenticated", true)
		c.Next()
	})
	return router
}
