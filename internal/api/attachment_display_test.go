package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

// Note: Uses centralized GetTestAuthToken() and AddTestAuthCookie() from test_helpers.go

type attachmentTicketNumberGenerator struct {
	mu  sync.Mutex
	seq int64
}

func (g *attachmentTicketNumberGenerator) Name() string      { return "Random" }
func (g *attachmentTicketNumberGenerator) IsDateBased() bool { return true }

func (g *attachmentTicketNumberGenerator) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return fmt.Sprintf("99%s%04d", time.Now().Format("060102150405"), g.seq), nil
}

type attachmentCounterStore struct{}

func (attachmentCounterStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return offset, nil
}

func TestAttachmentDisplayInTicketDetail(t *testing.T) {
	// Check if database is available first
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping integration test")
	}

	// Reset database to canonical state to ensure proper permissions
	// This must be called BEFORE getting the db connection we'll use for queries
	WithCleanDB(t)

	// Get fresh db connection AFTER WithCleanDB
	db, err = database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available after reset")
	}

	repository.SetTicketNumberGenerator(&attachmentTicketNumberGenerator{}, attachmentCounterStore{})
	t.Cleanup(func() { require.NoError(t, initTestTicketNumberGenerator()) })

	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	t.Setenv("APP_ENV", "integration")
	SetupHTMXRoutes(router)

	token := GetTestAuthToken(t)

	// Test creating a ticket with attachment
	t.Run("Create ticket with attachment and verify display", func(t *testing.T) {
		// Create a multipart form with file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add form fields
		writer.WriteField("title", "Test Ticket with Attachment")
		writer.WriteField("customer_email", "test@example.com")
		writer.WriteField("body", "This ticket has an attachment")
		writer.WriteField("priority", "normal")
		writer.WriteField("queue_id", "1")

		// Create a test file to upload
		testFileName := "test-attachment.txt"
		testFileContent := []byte("This is a test attachment content")

		// Add file to form
		part, err := writer.CreateFormFile("attachment", testFileName)
		require.NoError(t, err)
		_, err = io.Copy(part, bytes.NewReader(testFileContent))
		require.NoError(t, err)

		// Close the writer to finalize the form
		err = writer.Close()
		require.NoError(t, err)

		// Create the request
		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		AddTestAuthCookie(req, token)

		// Record the response
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check that ticket was created successfully
		if !assert.Equal(t, http.StatusCreated, w.Code) {
			t.Logf("ticket create response: %s", w.Body.String())
		}

		var created struct {
			ID           int    `json:"id"`
			TicketNumber string `json:"ticket_number"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			require.NoError(t, err, "create response should be JSON")
		}
		if created.ID == 0 {
			require.FailNow(t, "ticket id missing in create response")
		}

		// Now fetch the ticket messages to verify attachment is included
		messagesURL := fmt.Sprintf("/api/tickets/%d/messages", created.ID)
		req2 := httptest.NewRequest("GET", messagesURL, nil)
		AddTestAuthCookie(req2, token)
		req2.Header.Set("HX-Request", "true") // Request as HTMX

		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		// Check response
		assert.Equal(t, http.StatusOK, w2.Code)
		responseBody := w2.Body.String()

		// Verify that attachment information is in the response
		// The attachment should be visible in the messages HTML
		assert.Contains(t, responseBody, testFileName, "Attachment filename should be in the response")

		// Confirm ticket history recorded creation entry
		historyQuery := database.ConvertPlaceholders(`
			SELECT COUNT(*)
			FROM ticket_history th
			JOIN ticket_history_type tht ON th.history_type_id = tht.id
			WHERE th.ticket_id = ? AND tht.name = ?
		`)
		var historyCount int
		err = db.QueryRow(historyQuery, created.ID, history.TypeNewTicket).Scan(&historyCount)
		require.NoError(t, err)
		assert.Greater(t, historyCount, 0, "ticket creation should record a history entry")

		var historyName string
		historyNameQuery := database.ConvertPlaceholders(`
			SELECT th.name
			FROM ticket_history th
			JOIN ticket_history_type tht ON th.history_type_id = tht.id
			WHERE th.ticket_id = ? AND tht.name = ?
			ORDER BY th.id DESC
			LIMIT 1
		`)
		err = db.QueryRow(historyNameQuery, created.ID, history.TypeNewTicket).Scan(&historyName)
		require.NoError(t, err)
		assert.Contains(t, historyName, created.TicketNumber, "history payload should reference the new ticket number")
	})
}

// createAttachmentTestArticle inserts a ticket and an article (subject/body in
// article_data_mime) using the real schema, removing them when the test ends.
func createAttachmentTestArticle(t *testing.T, db *sql.DB, subject, body string) (ticketID, articleID int64) {
	t.Helper()
	tn := fmt.Sprintf("TEST-ATT-%d", time.Now().UnixNano())
	ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, type_id, ticket_state_id, ticket_priority_id,
		                    ticket_lock_id, user_id, responsible_user_id,
		                    timeout, until_time, escalation_time, escalation_update_time,
		                    escalation_response_time, escalation_solution_time,
		                    create_time, create_by, change_time, change_by)
		VALUES (?, 'Test Ticket for Attachments', 1, 1, 1, 1, 1, 1, 1,
		        0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)
		RETURNING id
	`), tn)
	require.NoError(t, err)

	articleID, err = database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
		                     is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, 3, 1, 1, NOW(), 1, NOW(), 1)
		RETURNING id
	`), ticketID)
	require.NoError(t, err)

	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_subject, a_body, a_content_type, incoming_time,
		                               create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 'text/plain', 0, NOW(), 1, NOW(), 1)
	`), articleID, subject, body)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime_attachment WHERE article_id = ?"), articleID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime WHERE article_id = ?"), articleID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM article WHERE id = ?"), articleID)
		db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id = ?"), ticketID)
	})
	return ticketID, articleID
}
