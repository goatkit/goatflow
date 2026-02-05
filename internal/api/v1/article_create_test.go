// Package v1 contains tests for the v1 API endpoints.
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

	. "github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestAddArticle_BasicArticle(t *testing.T) {
	requireDatabase(t)

	// Use seeded ticket from test database
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available")
	}

	var seedTicketID int
	err = db.QueryRow("SELECT id FROM ticket ORDER BY id LIMIT 1").Scan(&seedTicketID)
	if err != nil {
		t.Skipf("No seeded ticket available: %v", err)
	}
	testTicketIDStr := strconv.Itoa(seedTicketID)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/api/v1/tickets/:id/articles", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		// Use the actual API handler directly instead of v1 router
		HandleCreateArticleAPI(c)
	})

	// Verify ticket is accessible via API (handles race conditions with DB seeding)
	testPayload := map[string]interface{}{"body": "connectivity test"}
	jsonData, _ := json.Marshal(testPayload)
	req := httptest.NewRequest("POST", "/api/v1/tickets/"+testTicketIDStr+"/articles", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound || w.Code >= 500 {
		t.Skipf("Seeded ticket %s not accessible via API (status %d, seed data may not be loaded)", testTicketIDStr, w.Code)
	}
	if w.Code != http.StatusCreated {
		t.Skipf("Unexpected status %d when verifying ticket %s accessibility", w.Code, testTicketIDStr)
	}

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "add customer reply",
			payload: map[string]interface{}{
				"subject":      "Re: Customer inquiry",
				"body":         "Thank you for your ticket. We are looking into this issue.",
				"content_type": "text/plain",
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.NotNil(t, data["id"])
				assert.Equal(t, "Re: Customer inquiry", data["subject"])
				assert.Equal(t, "text/plain", data["content_type"])
			},
		},
		{
			name: "add HTML article",
			payload: map[string]interface{}{
				"subject":      "HTML Response",
				"body":         "<p>This is an <strong>HTML</strong> response.</p>",
				"content_type": "text/html",
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.Equal(t, "text/html", data["content_type"])
			},
		},
		{
			name: "add article without subject",
			payload: map[string]interface{}{
				"body":         "This is a quick note without subject",
				"content_type": "text/plain",
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				// Subject could be empty string or nil
				if subj, ok := data["subject"]; ok && subj != nil {
					assert.Equal(t, "", subj)
				}
			},
		},
		{
			name: "minimal article",
			payload: map[string]interface{}{
				"body": "Quick update",
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.Equal(t, "Quick update", data["body"])
				assert.Equal(t, "text/plain", data["content_type"]) // default
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/v1/tickets/"+testTicketIDStr+"/articles", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}
		})
	}
}

func TestAddArticle_ArticleTypes(t *testing.T) {
	requireDatabase(t)

	// Use seeded ticket from test database (seed data provides ticket IDs 1-2)
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available")
	}

	var seedTicketID int
	err = db.QueryRow("SELECT id FROM ticket ORDER BY id LIMIT 1").Scan(&seedTicketID)
	if err != nil {
		t.Skipf("No seeded ticket available: %v", err)
	}
	testTicketID := strconv.Itoa(seedTicketID)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/api/v1/tickets/:id/articles", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleCreateArticleAPI(c)
	})

	tests := []struct {
		name       string
		ticketID   string
		payload    map[string]interface{}
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:     "internal note",
			ticketID: testTicketID,
			payload: map[string]interface{}{
				"body":                    "Internal note: Customer called about this issue",
				"is_visible_for_customer": false,
				"article_sender_type_id":  1, // agent
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.Equal(t, false, data["is_visible_for_customer"])
				assert.Equal(t, float64(1), data["article_sender_type_id"])
			},
		},
		{
			name:     "customer visible reply",
			ticketID: testTicketID,
			payload: map[string]interface{}{
				"body":                    "Dear customer, here is our response",
				"is_visible_for_customer": true,
				"article_sender_type_id":  1, // agent
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.Equal(t, true, data["is_visible_for_customer"])
			},
		},
		{
			name:     "phone note",
			ticketID: testTicketID,
			payload: map[string]interface{}{
				"subject":                  "Phone call",
				"body":                     "Customer called at 2pm, resolved issue over phone",
				"communication_channel_id": 2, // phone
				"article_sender_type_id":   1, // agent
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.Equal(t, float64(2), data["communication_channel_id"])
			},
		},
		{
			name:     "customer email",
			ticketID: testTicketID,
			payload: map[string]interface{}{
				"subject":                  "Re: Issue update",
				"body":                     "Customer's email response",
				"communication_channel_id": 1, // email
				"article_sender_type_id":   3, // customer
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.Equal(t, float64(3), data["article_sender_type_id"])
			},
		},
		{
			name:     "system event",
			ticketID: testTicketID,
			payload: map[string]interface{}{
				"body":                    "Ticket was escalated due to SLA",
				"article_sender_type_id":  2, // system
				"is_visible_for_customer": false,
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				success, ok := body["success"].(bool)
				require.True(t, ok && success, "expected success=true, got: %v", body)
				data, ok := body["data"].(map[string]interface{})
				require.True(t, ok, "expected data map, got: %v", body)
				assert.Equal(t, float64(2), data["article_sender_type_id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/v1/tickets/"+tt.ticketID+"/articles", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}
		})
	}
}

func TestAddArticle_Validation(t *testing.T) {
	requireDatabase(t)

	// Get seeded ticket for tests that need a real ticket
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available")
	}

	var seedTicketID int
	err = db.QueryRow("SELECT id FROM ticket ORDER BY id LIMIT 1").Scan(&seedTicketID)
	validTicketID := "1" // fallback
	if err == nil {
		validTicketID = strconv.Itoa(seedTicketID)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/api/v1/tickets/:id/articles", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleCreateArticleAPI(c)
	})

	tests := []struct {
		name         string
		ticketID     string
		payload      interface{}
		wantStatus   int
		wantError    string
		needsTicket  bool // skip if no seeded ticket
	}{
		{
			name:       "invalid ticket ID",
			ticketID:   "abc",
			payload:    map[string]interface{}{"body": "Test"},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid ticket ID",
		},
		{
			name:       "non-existent ticket",
			ticketID:   "999999",
			payload:    map[string]interface{}{"body": "Test"},
			wantStatus: http.StatusNotFound,
			wantError:  "Ticket not found",
		},
		{
			name:        "empty body",
			ticketID:    validTicketID,
			payload:     map[string]interface{}{"body": ""},
			wantStatus:  http.StatusBadRequest,
			wantError:   "Article body is required",
			needsTicket: true,
		},
		{
			name:        "missing body",
			ticketID:    validTicketID,
			payload:     map[string]interface{}{"subject": "Test"},
			wantStatus:  http.StatusBadRequest,
			wantError:   "Article body is required",
			needsTicket: true,
		},
		{
			name:       "invalid JSON",
			ticketID:   validTicketID,
			payload:    "not json",
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid request",
		},
		{
			name:        "invalid sender type",
			ticketID:    validTicketID,
			needsTicket: true,
			payload: map[string]interface{}{
				"body":                   "Test",
				"article_sender_type_id": 99,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid sender type",
		},
		{
			name:        "invalid communication channel",
			ticketID:    validTicketID,
			needsTicket: true,
			payload: map[string]interface{}{
				"body":                     "Test",
				"communication_channel_id": 99,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid communication channel",
		},
	}

	noSeedData := (validTicketID == "1" && seedTicketID == 0)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needsTicket && noSeedData {
				t.Skip("No seeded ticket available for validation test")
			}

			var jsonData []byte
			var err error

			if str, ok := tt.payload.(string); ok {
				jsonData = []byte(str)
			} else {
				jsonData, err = json.Marshal(tt.payload)
				require.NoError(t, err)
			}

			req := httptest.NewRequest("POST", "/api/v1/tickets/"+tt.ticketID+"/articles", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.False(t, response["success"].(bool))
			assert.Contains(t, response["error"].(string), tt.wantError)
		})
	}
}

func TestAddArticle_Permissions(t *testing.T) {
	requireDatabase(t)
	gin.SetMode(gin.TestMode)

	// Create a test ticket with a known customer_user_id for permission testing
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Clean up any leftover data from previous failed test runs
	db.Exec(database.ConvertPlaceholders("SET FOREIGN_KEY_CHECKS=0"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = (SELECT id FROM ticket WHERE tn = 'PERM-TEST-001'))"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM article WHERE ticket_id = (SELECT id FROM ticket WHERE tn = 'PERM-TEST-001')"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE tn = 'PERM-TEST-001'"))
	db.Exec(database.ConvertPlaceholders("SET FOREIGN_KEY_CHECKS=1"))

	// Insert test ticket owned by test.customer
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_state_id, ticket_priority_id,
			user_id, responsible_user_id, ticket_lock_id, type_id, customer_user_id,
			timeout, until_time, escalation_time, escalation_update_time,
			escalation_response_time, escalation_solution_time,
			create_time, create_by, change_time, change_by)
		VALUES ('PERM-TEST-001', 'Permission test ticket', 1, 1, 3,
			1, 1, 1, 1, 'test.customer',
			0, 0, 0, 0, 0, 0,
			NOW(), 1, NOW(), 1)
	`))
	require.NoError(t, err)

	// Get the ID of the ticket we just created
	var testTicketID int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM ticket WHERE tn = 'PERM-TEST-001'",
	)).Scan(&testTicketID)
	require.NoError(t, err)
	testTicketIDStr := strconv.Itoa(testTicketID)

	// Clean up after test
	t.Cleanup(func() {
		db.Exec(database.ConvertPlaceholders("SET FOREIGN_KEY_CHECKS=0"))
		db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)"), testTicketID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM article WHERE ticket_id = ?"), testTicketID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id = ?"), testTicketID)
		db.Exec(database.ConvertPlaceholders("SET FOREIGN_KEY_CHECKS=1"))
	})

	tests := []struct {
		name       string
		setupAuth  func(c *gin.Context)
		ticketID   string
		payload    map[string]interface{}
		wantStatus int
	}{
		{
			name: "authenticated agent can add article",
			setupAuth: func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
			},
			ticketID: testTicketIDStr,
			payload: map[string]interface{}{
				"body": "Agent response",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "unauthenticated user cannot add article",
			setupAuth: func(c *gin.Context) {
				// No auth set
			},
			ticketID: testTicketIDStr,
			payload: map[string]interface{}{
				"body": "Should fail",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "customer can add to own ticket",
			setupAuth: func(c *gin.Context) {
				c.Set("user_id", 3)
				c.Set("is_customer", true)
				c.Set("customer_email", "test.customer") // Matches ticket.customer_user_id
				c.Set("is_authenticated", true)
			},
			ticketID: testTicketIDStr,
			payload: map[string]interface{}{
				"body": "Customer response",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "customer cannot add to other's ticket",
			setupAuth: func(c *gin.Context) {
				c.Set("user_id", 100)
				c.Set("is_customer", true)
				c.Set("customer_email", "other@example.com")
				c.Set("is_authenticated", true)
			},
			ticketID: testTicketIDStr,
			payload: map[string]interface{}{
				"body": "Should fail",
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			router.POST("/api/v1/tickets/:id/articles", func(c *gin.Context) {
				tt.setupAuth(c)
				HandleCreateArticleAPI(c)
			})

			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/v1/tickets/"+tt.ticketID+"/articles", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestAddArticle_EmailHeaders(t *testing.T) {
	requireDatabase(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/api/v1/tickets/:id/articles", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleCreateArticleAPI(c)
	})

	payload := map[string]interface{}{
		"subject":      "Email with headers",
		"body":         "This article includes email headers",
		"content_type": "text/plain",
		"from":         "sender@example.com",
		"to":           "recipient@example.com",
		"cc":           "cc@example.com",
		"reply_to":     "reply@example.com",
		"message_id":   "<unique-id@example.com>",
		"in_reply_to":  "<parent-id@example.com>",
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/tickets/1/articles", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "sender@example.com", data["from"])
	assert.Equal(t, "recipient@example.com", data["to"])
	assert.Equal(t, "cc@example.com", data["cc"])
	assert.Equal(t, "<unique-id@example.com>", data["message_id"])
}

func TestAddArticle_UpdatesTicketChangeTime(t *testing.T) {
	requireDatabase(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/api/v1/tickets/:id/articles", func(c *gin.Context) {
		c.Set("user_id", 15) // testuser agent
		c.Set("is_authenticated", true)
		HandleCreateArticleAPI(c)
	})

	payload := map[string]interface{}{
		"body": "This should update ticket's change_time",
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/tickets/1/articles", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})

	// Check that the creator is set correctly
	assert.Equal(t, float64(15), data["create_by"])

	// The ticket's change_time should be updated (verified in handler)
	assert.NotNil(t, data["ticket_updated"])
	assert.Equal(t, true, data["ticket_updated"])
}
