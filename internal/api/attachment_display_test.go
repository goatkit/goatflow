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
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	t.Setenv("APP_ENV", "integration")
	t.Setenv("HTMX_HANDLER_TEST_MODE", "0")
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

func TestAttachmentDownloadHandler(t *testing.T) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping integration test")
	}
	t.Setenv("ATTACHMENTS_USE_DB", "1")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/tickets/:id/attachments/:attachment_id", handleDownloadAttachment)

	ticketID, articleID := createAttachmentTestArticle(t, db, "Download Article", "Download body")
	testContent := []byte("Test download content")

	t.Run("Download existing attachment", func(t *testing.T) {
		attachmentID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO article_data_mime_attachment
			(article_id, filename, content_type, content_size, content, disposition,
			 create_time, create_by, change_time, change_by)
			VALUES (?, 'test-download.txt', 'text/plain', ?, ?, 'attachment', NOW(), 1, NOW(), 1)
			RETURNING id
		`), articleID, fmt.Sprint(len(testContent)), testContent)
		require.NoError(t, err)
		t.Cleanup(func() {
			db.Exec(database.ConvertPlaceholders("DELETE FROM article_data_mime_attachment WHERE id = ?"), attachmentID)
		})

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/tickets/%d/attachments/%d", ticketID, attachmentID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "test-download.txt")
		assert.Equal(t, string(testContent), w.Body.String())
	})

	t.Run("Download non-existent attachment", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/tickets/%d/attachments/99999999", ticketID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
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

func TestGetMessagesWithAttachments(t *testing.T) {
	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping integration test")
	}

	t.Run("GetMessages includes attachment data from database", func(t *testing.T) {
		// Re-check DB availability (defensive)
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available, skipping integration test")
		}

		// Create a test ticket
		ticketID, articleID := createAttachmentTestArticle(t, db, "Test Article", "Test article body")

		// Create an attachment for the article
		attachmentID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO article_data_mime_attachment
			(article_id, filename, content_type, content_size, content, disposition,
			 create_time, create_by, change_time, change_by)
			VALUES (?, 'document.pdf', 'application/pdf', '11', ?, 'attachment', NOW(), 1, NOW(), 1)
			RETURNING id
		`), articleID, []byte("PDF content"))
		require.NoError(t, err)

		// Get the ticket service and retrieve messages
		ticketService := GetTicketService()
		messages, err := ticketService.GetMessages(uint(ticketID))

		require.NoError(t, err)
		require.NotEmpty(t, messages, "Should have at least one message")

		// Verify the attachment is included
		message := messages[0]
		assert.Equal(t, "Test Article", message.Subject)
		assert.Equal(t, "Test article body", message.Body)
		require.NotEmpty(t, message.Attachments, "Message should have attachments")

		attachment := message.Attachments[0]
		assert.Equal(t, "document.pdf", attachment.Filename)
		assert.Equal(t, "application/pdf", attachment.ContentType)
		assert.Equal(t, int64(11), attachment.Size)
		assert.Contains(t, attachment.URL, fmt.Sprintf("/%d/", attachmentID))
	})
}
