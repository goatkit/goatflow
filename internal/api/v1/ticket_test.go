package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/goatkit/goatflow/internal/api"
)

// Acceptance Test: As an agent, I can create a new ticket.
func TestCreateTicket_AcceptanceTest(t *testing.T) {
	requireDatabase(t)
	ensureTicketFixtures(t)

	router := setupTestRouter()

	// Test data matching OTRS structure
	ticketData := map[string]interface{}{
		"title":            "Test ticket for customer issue",
		"queue_id":         1, // Raw queue
		"type_id":          1, // Incident
		"state_id":         1, // new
		"priority_id":      3, // 3 normal
		"customer_user_id": "customer@example.com",
		"customer_id":      "ACME Corp",
		"article": map[string]interface{}{
			"subject":         "Initial problem description",
			"body":            "Customer reports that the system is not working properly.",
			"article_type_id": 1, // email-external
			"sender_type_id":  3, // customer
		},
	}

	jsonData, _ := json.Marshal(ticketData)
	req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code, "Should return 201 Created")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})

	// Verify ticket number format (OTRS style: YYYYMMDDHHMMSS + counter)
	assert.Regexp(t, `^\d{14}\d+$`, data["ticket_number"])
	assert.NotNil(t, data["id"])
	assert.Equal(t, "Test ticket for customer issue", data["title"])
	assert.Equal(t, float64(1), data["queue_id"])
	assert.Equal(t, float64(1), data["state_id"])
}

// Acceptance Test: As an agent, I can list tickets with filters.
func TestListTickets_AcceptanceTest(t *testing.T) {
	requireDatabase(t)
	ensureTicketFixtures(t)

	router := setupTestRouter()

	// Create some test tickets first
	createTestTickets(t, router, 5)

	// Test listing with filters
	req := httptest.NewRequest("GET", "/api/v1/tickets?state_id=1&queue_id=1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))

	// Check pagination
	assert.NotNil(t, response["page"])
	assert.NotNil(t, response["per_page"])
	assert.NotNil(t, response["total"])

	// Check data structure
	data := response["data"].([]interface{})
	assert.GreaterOrEqual(t, len(data), 1, "Should have at least one ticket")

	// Verify ticket structure
	firstTicket := data[0].(map[string]interface{})
	assert.NotNil(t, firstTicket["id"])
	assert.NotNil(t, firstTicket["ticket_number"])
	assert.NotNil(t, firstTicket["title"])
	assert.NotNil(t, firstTicket["state_id"])
	assert.NotNil(t, firstTicket["create_time"])
}

// Acceptance Test: As an agent, I can view ticket details with articles.
func TestGetTicketDetails_AcceptanceTest(t *testing.T) {
	requireDatabase(t)
	ensureTicketFixtures(t)

	router := setupTestRouter()

	// Create a ticket first
	ticketID := createTestTicket(t, router)

	// Get ticket details
	req := httptest.NewRequest("GET", "/api/v1/tickets/"+ticketID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})

	// Verify complete ticket structure
	assert.Equal(t, ticketID, data["id"])
	assert.NotNil(t, data["ticket_number"])
	assert.NotNil(t, data["title"])
	assert.NotNil(t, data["queue"])
	assert.NotNil(t, data["state"])
	assert.NotNil(t, data["priority"])
	assert.NotNil(t, data["articles"], "Should include articles")

	// Check articles
	articles := data["articles"].([]interface{})
	assert.GreaterOrEqual(t, len(articles), 1, "Should have at least initial article")
}

// Acceptance Test: As an agent, I can update ticket state.
func TestUpdateTicketState_AcceptanceTest(t *testing.T) {
	requireDatabase(t)
	ensureTicketFixtures(t)

	router := setupTestRouter()

	// Create a ticket
	ticketID := createTestTicket(t, router)

	// Update state to open
	updateData := map[string]interface{}{
		"state_id": 4, // open
		"article": map[string]interface{}{
			"subject":         "Working on ticket",
			"body":            "I'm investigating this issue.",
			"article_type_id": 8, // note-internal
			"sender_type_id":  1, // agent
		},
	}

	jsonData, _ := json.Marshal(updateData)
	req := httptest.NewRequest("PUT", "/api/v1/tickets/"+ticketID, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	assert.Equal(t, float64(4), data["state_id"], "State should be updated to open")
}

// Helper functions.
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Add auth middleware mock
	r.Use(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "" {
			c.Set("user_id", 1)
			c.Set("user_email", "agent@test.com")
			c.Set("user_role", "Agent")
		}
		c.Next()
	})

	// Setup routes
	v1 := r.Group("/api/v1")
	{
		v1.POST("/tickets", HandleCreateTicketAPI)
		v1.GET("/tickets", HandleListTicketsAPI)
		v1.GET("/tickets/:id", HandleGetTicketAPI)
		v1.PUT("/tickets/:id", HandleUpdateTicketAPI)
	}

	return r
}

func createTestTicket(t *testing.T, router *gin.Engine) string {
	ticketData := map[string]interface{}{
		"title":            "Test ticket",
		"queue_id":         1,
		"type_id":          1,
		"state_id":         1,
		"priority_id":      3,
		"customer_user_id": "test@example.com",
		"article": map[string]interface{}{
			"subject":         "Test",
			"body":            "Test body",
			"article_type_id": 1,
			"sender_type_id":  3,
		},
	}

	jsonData, _ := json.Marshal(ticketData)
	req := httptest.NewRequest("POST", "/api/v1/tickets", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	if v, ok := data["id"].(string); ok {
		return v
	}
	if v, ok := data["id"].(float64); ok {
		return strconv.FormatInt(int64(v), 10)
	}
	t.Fatalf("unexpected ticket id type %T", data["id"])
	return ""
}

func createTestTickets(t *testing.T, router *gin.Engine, count int) {
	for i := 0; i < count; i++ {
		createTestTicket(t, router)
	}
}
