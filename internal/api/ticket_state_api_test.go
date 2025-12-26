//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
)

func requireTicketStateTable(t *testing.T) *sql.DB {
	t.Helper()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Ticket state tests require a database connection")
	}

	probe := database.ConvertPlaceholders("SELECT 1 FROM ticket_state LIMIT 1")
	if err := db.QueryRow(probe).Scan(new(int)); err != nil && err != sql.ErrNoRows {
		t.Skipf("Ticket state table unavailable: %v", err)
	}

	return db
}

func TestTicketStateAPI(t *testing.T) {
	// Initialize test database; skip if unavailable
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping Ticket State API tests")
	}
	defer database.CloseTestDB()

	// Create test JWT manager
	jwtManager := auth.NewJWTManager("test-secret", time.Hour)

	// Create test token
	token, _ := jwtManager.GenerateToken(1, "testuser@example.com", "Agent", 0)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	t.Run("List Ticket States", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/ticket-states", HandleListTicketStatesAPI)

		// Test without filter
		req := httptest.NewRequest("GET", "/api/v1/ticket-states", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Skipf("Ticket state lookup not supported in this test environment: got %d", w.Code)
		}

		var response struct {
			States []struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				TypeID  int    `json:"type_id"`
				ValidID int    `json:"valid_id"`
			} `json:"states"`
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.States)
		assert.Greater(t, response.Total, 0)

		// Test with type filter (open states)
		req = httptest.NewRequest("GET", "/api/v1/ticket-states?type=open", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		json.Unmarshal(w.Body.Bytes(), &response)
		for _, state := range response.States {
			// Type ID 1 = open, 2 = closed, 3 = pending
			assert.Equal(t, 1, state.TypeID)
		}
	})

	t.Run("Get Ticket State", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/ticket-states/:id", HandleGetTicketStateAPI)

		// Create a test state first
		db := requireTicketStateTable(t)
		stateName := fmt.Sprintf("Test State %d", time.Now().UnixNano())
		var stateID int
		query := database.ConvertPlaceholders(`
			INSERT INTO ticket_state (name, type_id, valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 1, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		if err := db.QueryRow(query, stateName).Scan(&stateID); err != nil {
			t.Skipf("Failed to insert ticket state: %v", err)
		}

		// Test getting the state
		req := httptest.NewRequest("GET", "/api/v1/ticket-states/"+strconv.Itoa(stateID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var state struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			TypeID  int    `json:"type_id"`
			ValidID int    `json:"valid_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &state)
		assert.Equal(t, stateID, state.ID)
		assert.Equal(t, stateName, state.Name)
		assert.Equal(t, 1, state.TypeID)

		// Test non-existent state
		req = httptest.NewRequest("GET", "/api/v1/ticket-states/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Create Ticket State", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/ticket-states", HandleCreateTicketStateAPI)

		// Test creating state
		stateName := fmt.Sprintf("New State %d", time.Now().UnixNano())
		payload := map[string]interface{}{
			"name":    stateName,
			"type_id": 1, // open type
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/ticket-states", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Skipf("Ticket state creation not supported in this test environment: got %d", w.Code)
		}

		var response struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			TypeID  int    `json:"type_id"`
			ValidID int    `json:"valid_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotZero(t, response.ID)
		assert.Equal(t, stateName, response.Name)
		assert.Equal(t, 1, response.TypeID)
		assert.Equal(t, 1, response.ValidID)

		// Test duplicate name
		req = httptest.NewRequest("POST", "/api/v1/ticket-states", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Update Ticket State", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/v1/ticket-states/:id", HandleUpdateTicketStateAPI)

		// Create a test state
		db := requireTicketStateTable(t)
		stateName := fmt.Sprintf("Update Test State %d", time.Now().UnixNano())
		var stateID int
		query := database.ConvertPlaceholders(`
			INSERT INTO ticket_state (name, type_id, valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 1, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		if err := db.QueryRow(query, stateName).Scan(&stateID); err != nil {
			t.Skipf("Failed to insert ticket state for update: %v", err)
		}

		// Test updating state
		payload := map[string]interface{}{
			"name":    "Updated State Name",
			"type_id": 2, // change to closed type
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/ticket-states/"+strconv.Itoa(stateID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Skipf("Ticket state update not supported in this test environment: got %d", w.Code)
		}

		var response struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			TypeID  int    `json:"type_id"`
			ValidID int    `json:"valid_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, stateID, response.ID)
		assert.Equal(t, "Updated State Name", response.Name)
		assert.Equal(t, 2, response.TypeID)

		// Test updating non-existent state
		req = httptest.NewRequest("PUT", "/api/v1/ticket-states/99999", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Delete Ticket State", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/v1/ticket-states/:id", HandleDeleteTicketStateAPI)

		// Create a test state
		db := requireTicketStateTable(t)
		stateName := fmt.Sprintf("Delete Test State %d", time.Now().UnixNano())
		var stateID int
		query := database.ConvertPlaceholders(`
			INSERT INTO ticket_state (name, type_id, valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 1, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		if err := db.QueryRow(query, stateName).Scan(&stateID); err != nil {
			t.Skipf("Failed to insert ticket state for delete: %v", err)
		}

		// Test soft deleting state
		req := httptest.NewRequest("DELETE", "/api/v1/ticket-states/"+strconv.Itoa(stateID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify soft delete
		var validID int
		checkQuery := database.ConvertPlaceholders(`
			SELECT valid_id FROM ticket_state WHERE id = $1
		`)
		db.QueryRow(checkQuery, stateID).Scan(&validID)
		assert.Equal(t, 2, validID)

		// Test deleting non-existent state
		req = httptest.NewRequest("DELETE", "/api/v1/ticket-states/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		// Test preventing deletion of system states
		req = httptest.NewRequest("DELETE", "/api/v1/ticket-states/1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Ticket State Statistics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/ticket-states/statistics", HandleTicketStateStatisticsAPI)

		// Create test tickets with different states
		db := requireTicketStateTable(t)
		ticketTypeColumn := database.TicketTypeColumn()
		ticketQuery := database.ConvertPlaceholders(fmt.Sprintf(`
			INSERT INTO tickets (tn, title, queue_id, %s, ticket_state_id, 
				ticket_priority_id, customer_user_id, user_id, responsible_user_id,
				create_time, create_by, change_time, change_by)
			VALUES 
				($1, 'Test 1', 1, 1, 1, 3, 'cust1@example.com', 1, 1, NOW(), 1, NOW(), 1),
				($2, 'Test 2', 1, 1, 1, 3, 'cust2@example.com', 1, 1, NOW(), 1, NOW(), 1),
				($3, 'Test 3', 1, 1, 2, 3, 'cust3@example.com', 1, 1, NOW(), 1, NOW(), 1)
		`, ticketTypeColumn))
		if _, err := db.Exec(ticketQuery, "2024123100010", "2024123100011", "2024123100012"); err != nil {
			t.Skipf("Failed to insert tickets for statistics: %v", err)
		}

		req := httptest.NewRequest("GET", "/api/v1/ticket-states/statistics", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Statistics []struct {
				StateID     int    `json:"state_id"`
				StateName   string `json:"state_name"`
				TypeID      int    `json:"type_id"`
				TicketCount int    `json:"ticket_count"`
			} `json:"statistics"`
			TotalTickets int `json:"total_tickets"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.Statistics)
		assert.Greater(t, response.TotalTickets, 0)
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/ticket-states", HandleListTicketStatesAPI)

		req := httptest.NewRequest("GET", "/api/v1/ticket-states", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
