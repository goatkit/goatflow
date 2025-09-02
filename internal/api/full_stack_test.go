package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	// Import database package directly
	"database/sql"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	_ "github.com/lib/pq"
)

// NOTE: Do not force override DB host/port here; rely on environment.

// TestFullStackTicketCreation tests the complete ticket creation flow with real database
func TestFullStackTicketCreation(t *testing.T) {
    // Skip if DB env not configured or unreachable
    if os.Getenv("DB_HOST") == "" {
        t.Skip("Database not configured; skipping full stack test")
    }
    // Connect to real database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"), os.Getenv("DB_SSLMODE"))
	
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "Database must be available for full stack testing")
	defer db.Close()
	
    err = db.Ping()
    if err != nil {
        t.Skipf("Database not reachable: %v", err)
    }
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// For now, manually register the routes we're testing
	router.POST("/api/tickets", handleCreateTicketWithAttachments)
	router.POST("/api/tickets/:id/messages", handleAddTicketMessage)
	
	t.Run("Create ticket with multiple attachments using full infrastructure", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		
		// Add all form fields
		writer.WriteField("title", "Full Stack Test - Bug Report")
		writer.WriteField("customer_email", "fullstack@test.com")
		writer.WriteField("customer_name", "Test User")
		writer.WriteField("body", "This is a full infrastructure test with real database, storage, and multiple attachments")
		writer.WriteField("priority", "high")
		writer.WriteField("queue_id", "1")
		writer.WriteField("type_id", "1")
		
		// Add multiple test files
		files := []struct {
			name    string
			content string
		}{
			{"error.log", "2024-01-15 10:23:45 ERROR Database connection timeout\n2024-01-15 10:23:46 ERROR Retry failed"},
			{"config.json", `{"version": "1.2.3", "debug": true, "max_connections": 100}`},
			{"screenshot.png", "fake-png-binary-data-would-be-here"},
		}
		
		for _, f := range files {
			part, err := writer.CreateFormFile("attachment", f.name)
			require.NoError(t, err)
			_, err = io.WriteString(part, f.content)
			require.NoError(t, err)
		}
		
		writer.Close()
		
		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		
		// Mock authentication - in production this comes from middleware
		req.Header.Set("X-User-ID", "1")
		req.Header.Set("X-User-Role", "Agent")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Verify response
		assert.Equal(t, http.StatusCreated, w.Code, "Should create ticket successfully")
		
		var resp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		
		// Verify ticket was created
		assert.Contains(t, resp, "id", "Should return ticket ID")
		assert.Contains(t, resp, "ticket_number", "Should return ticket number")
		assert.Equal(t, "Ticket created successfully", resp["message"])
		
		// Verify attachments were processed
		if attachments, ok := resp["attachments"]; ok {
			attachmentList := attachments.([]interface{})
			assert.Len(t, attachmentList, 3, "Should have processed all 3 attachments")
			
			for i, att := range attachmentList {
				attachment := att.(map[string]interface{})
				assert.True(t, attachment["saved"].(bool), "Attachment %d should be saved", i)
				assert.Contains(t, attachment, "path", "Should have storage path")
				
				// Verify file actually exists on disk
				path := attachment["path"].(string)
				_, err := os.Stat(path)
				assert.NoError(t, err, "File should exist at %s", path)
			}
		} else {
			t.Error("No attachments in response despite uploading files")
		}
		
		// Verify ticket exists in database
		ticketID := int(resp["id"].(float64))
		db, _ := database.GetDB()
		
		var count int
        err = db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket WHERE id = $1`), ticketID).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count, "Ticket should exist in database")
		
        // Verify article was created
        err = db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM article WHERE ticket_id = $1`), ticketID).Scan(&count)
		assert.NoError(t, err)
		assert.Greater(t, count, 0, "Article should exist for ticket")
	})
	
	t.Run("Test ticket messages API with full infrastructure", func(t *testing.T) {
		// First create a ticket
		ticketID := createTestTicket(t, router)
		
		// Now test adding a message
		msgBody := &bytes.Buffer{}
		writer := multipart.NewWriter(msgBody)
		writer.WriteField("content", "This is a follow-up message added via the messages API")
		writer.WriteField("subject", "Re: Test Ticket")
		writer.WriteField("is_internal", "false")
		writer.Close()
		
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/tickets/%d/messages", ticketID), msgBody)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-User-ID", "1")
		req.Header.Set("X-User-Role", "Agent")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusCreated, w.Code, "Should add message successfully")
		
		// Verify message in database
		db, _ := database.GetDB()
		var messageCount int
        err := db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM article WHERE ticket_id = $1`), ticketID).Scan(&messageCount)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, messageCount, 2, "Should have at least 2 articles (initial + new message)")
	})
	
	t.Run("Load test with concurrent ticket creation", func(t *testing.T) {
		// Test that our infrastructure handles concurrent requests
		concurrency := 5
		done := make(chan bool, concurrency)
		
		for i := 0; i < concurrency; i++ {
			go func(index int) {
				defer func() { done <- true }()
				
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				writer.WriteField("title", fmt.Sprintf("Concurrent Test Ticket %d", index))
				writer.WriteField("customer_email", fmt.Sprintf("user%d@test.com", index))
				writer.WriteField("body", "Testing concurrent ticket creation")
				writer.Close()
				
				req := httptest.NewRequest("POST", "/api/tickets", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-User-ID", "1")
				req.Header.Set("X-User-Role", "Agent")
				
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				
				assert.Equal(t, http.StatusCreated, w.Code, "Concurrent request %d should succeed", index)
			}(i)
		}
		
		// Wait for all goroutines with timeout
		timeout := time.After(10 * time.Second)
		for i := 0; i < concurrency; i++ {
			select {
			case <-done:
				// Success
			case <-timeout:
				t.Fatal("Timeout waiting for concurrent requests")
			}
		}
		
		// Verify all tickets were created
		db, _ := database.GetDB()
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM ticket WHERE title LIKE 'Concurrent Test Ticket%'").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, concurrency, count, "All concurrent tickets should be created")
	})
	
	t.Run("Test file type validation", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "Security Test")
		writer.WriteField("customer_email", "security@test.com")
		writer.WriteField("body", "Testing blocked file types")
		
		// Try to upload a blocked file type
		part, _ := writer.CreateFormFile("attachment", "malicious.exe")
		io.WriteString(part, "fake executable content")
		writer.Close()
		
		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-User-ID", "1")
		req.Header.Set("X-User-Role", "Agent")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// Should still create ticket but without the blocked attachment
		assert.Equal(t, http.StatusCreated, w.Code, "Should create ticket even with blocked file")
		
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		
		// Verify no attachments were saved
		if attachments, ok := resp["attachments"]; ok {
			attachmentList := attachments.([]interface{})
			assert.Len(t, attachmentList, 0, "Blocked file should not be saved")
		}
	})
}

// Helper function to create a test ticket
func createTestTicket(t *testing.T, router *gin.Engine) int {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("title", "Test Ticket for Messages")
	writer.WriteField("customer_email", "test@example.com")
	writer.WriteField("body", "Initial ticket body")
	writer.Close()
	
	req := httptest.NewRequest("POST", "/api/tickets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Role", "Agent")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	require.Equal(t, http.StatusCreated, w.Code)
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	
	return int(resp["id"].(float64))
}

// TestDatabaseIntegrity verifies our database operations maintain referential integrity
func TestDatabaseIntegrity(t *testing.T) {
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available")
    }
	
    db, err := database.GetDB()
    if err != nil || db == nil {
        t.Skip("Database not available")
    }
	
	t.Run("Verify foreign key constraints work", func(t *testing.T) {
		// Try to create an article for non-existent ticket
        _, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, subject, body, create_by, change_by)
            VALUES (999999, 'Test', 'Test', 1, 1)
        `))
		
		assert.Error(t, err, "Should fail due to foreign key constraint")
		assert.Contains(t, err.Error(), "foreign key", "Error should mention foreign key")
	})
	
	t.Run("Verify cascade deletes work correctly", func(t *testing.T) {
		// Create a test ticket
		var ticketID int
        err := db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket (tn, title, queue_id, ticket_state_id, ticket_priority_id, 
				ticket_lock_id, create_by, change_by, customer_user_id)
			VALUES ('TEST' || EXTRACT(EPOCH FROM NOW())::int, 'Cascade Test', 1, 1, 1, 1, 1, 1, 'test@example.com')
            RETURNING id
        `)).Scan(&ticketID)
		require.NoError(t, err)
		
		// Add an article
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, subject, body, create_by, change_by)
			VALUES ($1, 'Test Article', 'Test Body', 1, 1)
		`), ticketID)
		require.NoError(t, err)
		
		// Delete the ticket
        _, err = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = $1`), ticketID)
		require.NoError(t, err)
		
		// Verify article was also deleted (cascade)
		var count int
        err = db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM article WHERE ticket_id = $1`), ticketID).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 0, count, "Articles should be cascade deleted with ticket")
	})
}

// TestCleanupOldTestData ensures we don't accumulate test data over time
func TestCleanupOldTestData(t *testing.T) {
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available")
    }
	
    db, err := database.GetDB()
    if err != nil || db == nil {
        t.Skip("Database not available")
    }
	
	// Clean up test tickets older than 1 hour
    result, err := db.Exec(database.ConvertPlaceholders(`
		DELETE FROM ticket 
		WHERE (title LIKE 'Test%' OR title LIKE 'Full Stack Test%' OR title LIKE 'Concurrent Test%')
        AND create_time < NOW() - INTERVAL '1 hour'
    `))
	
	if err == nil {
		affected, _ := result.RowsAffected()
		t.Logf("Cleaned up %d old test tickets", affected)
	}
	
	// Clean up test storage files
	// This would clean up old test files from ./internal/api/storage/tickets/
	// Implementation depends on your storage structure
}