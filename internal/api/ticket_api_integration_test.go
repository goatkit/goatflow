//go:build integration

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

// TestTicketAPIIntegration tests ticket endpoints with actual database.
func TestTicketAPIIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	t.Run("Complete Ticket Workflow", func(t *testing.T) {
		t.Setenv("APP_ENV", "integration")

		router := setupAuthenticatedRouter(t)

		// Step 1: Create a ticket
		createBody := map[string]interface{}{
			"title":            "Integration Test Ticket",
			"queue_id":         1,
			"priority_id":      3,
			"state_id":         1,
			"customer_user_id": "test@example.com",
			"article": map[string]interface{}{
				"subject": "Test Article",
				"body":    "This is a test article body",
				"type":    "note",
			},
		}
		jsonBody, _ := json.Marshal(createBody)

		req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should create successfully
		assert.Equal(t, http.StatusCreated, w.Code)

		var createResponse map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &createResponse)
		require.NoError(t, err)

		// Extract ticket ID
		data := createResponse["data"].(map[string]interface{})
		ticketID := int(data["id"].(float64))

		// Step 2: Get the created ticket
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/tickets/%d", ticketID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Step 3: Update the ticket
		updateBody := map[string]interface{}{
			"title":       "Updated Integration Test Ticket",
			"priority_id": 1,
		}
		jsonBody, _ = json.Marshal(updateBody)

		req = httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/tickets/%d", ticketID), bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Step 4: Close the ticket
		closeBody := map[string]interface{}{
			"resolution": "resolved",
			"comment":    "Test completed",
		}
		jsonBody, _ = json.Marshal(closeBody)

		req = httptest.NewRequest("POST", fmt.Sprintf("/api/v1/tickets/%d/close", ticketID), bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Step 5: Verify ticket is closed
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/tickets/%d", ticketID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var getResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &getResponse)
		require.NoError(t, err)
		ticketData := getResponse["data"].(map[string]interface{})
		assert.Equal(t, float64(2), ticketData["state_id"]) // Closed state
	})
}

func setupAuthenticatedRouter(t *testing.T) *gin.Engine {
	t.Helper()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	require.NoError(t, err)

	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() {
		repository.SetTicketNumberGenerator(nil, nil)
	})

	router := gin.New()

	// Add authentication middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("user_email", "integration@test.local")
		c.Set("user_role", "Agent")
		c.Set("is_authenticated", true)
		c.Next()
	})

	// Register routes
	v1 := router.Group("/api/v1")
	{
		// Ticket CRUD
		v1.GET("/tickets", HandleListTicketsAPI)
		v1.GET("/tickets/:id", HandleGetTicketAPI)
		v1.POST("/tickets", HandleCreateTicketAPI)
		v1.PUT("/tickets/:id", HandleUpdateTicketAPI)
		v1.DELETE("/tickets/:id", HandleDeleteTicketAPI)

		// Ticket actions
		v1.POST("/tickets/:id/close", HandleCloseTicketAPI)
		v1.POST("/tickets/:id/reopen", HandleReopenTicketAPI)
		v1.POST("/tickets/:id/assign", HandleAssignTicketAPI)
	}

	return router
}
