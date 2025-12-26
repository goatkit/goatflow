//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Set test environment variables if not already set
	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost")
		os.Setenv("DB_PORT", "5432")
		os.Setenv("DB_USER", "gotrs_user")
		os.Setenv("DB_PASSWORD", "gotrs_password123")
		os.Setenv("DB_NAME", "gotrs")
		os.Setenv("DB_SSLMODE", "disable")
	}
}

// Test using the REAL database to reproduce the exact 500 error
func TestTicketCreationWithAttachments500Fix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Initialize database connection for testing
	err := database.InitDB()
	if err != nil {
		t.Skipf("Database not available for testing: %v", err)
		return
	}

	t.Run("Reproduce and fix 500 error with attachments", func(t *testing.T) {
		router := gin.New()

		// Use the ACTUAL handleCreateTicket function
		router.POST("/api/tickets", func(c *gin.Context) {
			// Set mock user context (would come from auth middleware in production)
			c.Set("user_role", "Agent")
			c.Set("user_id", uint(1))
			c.Set("user_email", "admin@example.com")

			// Call the real handler
			handleCreateTicket(c)
		})

		// Create multipart form with attachment
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add all required fields
		writer.WriteField("title", "Bug Report with Screenshot")
		writer.WriteField("customer_email", "customer@example.com")
		writer.WriteField("customer_name", "Test Customer")
		writer.WriteField("body", "Application crashes when clicking submit button. See attached screenshot.")
		writer.WriteField("priority", "high")
		writer.WriteField("queue_id", "1")
		writer.WriteField("type_id", "1")

		// Add file attachment - THIS is what causes the current issue
		part, err := writer.CreateFormFile("attachment", "screenshot.png")
		require.NoError(t, err)
		_, err = io.WriteString(part, "fake-png-data-for-testing")
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Make the request
		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		// Execute the request
		router.ServeHTTP(w, req)

		// Parse response
		var resp map[string]interface{}
		if w.Body.Len() > 0 {
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Logf("Response body: %s", w.Body.String())
			}
		}

		t.Logf("Response status: %d", w.Code)
		t.Logf("Response: %v", resp)

		// The current implementation should either:
		// 1. Return 500 if attachment processing fails
		// 2. Return 201 but ignore the attachment (wrong behavior)
		// After fix: Should return 201 with attachment properly saved

		if w.Code == http.StatusInternalServerError {
			t.Errorf("Got 500 error as reported by user: %v", resp["error"])
			t.Log("This confirms the bug - ticket creation fails with attachments")
		} else if w.Code == http.StatusCreated {
			// Check if attachment was actually processed
			if attachments, ok := resp["attachments"]; ok {
				t.Logf("Attachments processed: %v", attachments)
			} else {
				t.Error("Ticket created but attachments were ignored!")
			}
		} else {
			t.Errorf("Unexpected status: %d, error: %v", w.Code, resp["error"])
		}
	})

	t.Run("Verify ticket creation works without attachments", func(t *testing.T) {
		router := gin.New()
		router.POST("/api/tickets", func(c *gin.Context) {
			c.Set("user_role", "Agent")
			c.Set("user_id", uint(1))
			c.Set("user_email", "admin@example.com")
			handleCreateTicket(c)
		})

		// Simple form without attachments
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "Simple ticket without attachments")
		writer.WriteField("customer_email", "user@example.com")
		writer.WriteField("body", "This should work fine")
		writer.WriteField("priority", "normal")
		writer.Close()

		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code, "Basic ticket creation should work")

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp, "id", "Should return ticket ID")
		assert.Contains(t, resp, "message", "Should return success message")
	})
}

// Test what needs to be implemented for attachment handling
func TestAttachmentHandlingImplementation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Handler should process multipart files", func(t *testing.T) {
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			// This is what handleCreateTicket SHOULD do but doesn't

			// 1. Parse the multipart form
			form, err := c.MultipartForm()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
				return
			}

			// 2. Get uploaded files
			files := form.File["attachment"]
			attachmentInfo := []map[string]interface{}{}

			for _, file := range files {
				// 3. Process each file
				t.Logf("Processing file: %s (size: %d)", file.Filename, file.Size)

				// 4. Save file to storage (would use storage service in real implementation)
				attachmentInfo = append(attachmentInfo, map[string]interface{}{
					"filename": file.Filename,
					"size":     file.Size,
					"saved":    true,
				})
			}

			c.JSON(http.StatusOK, gin.H{
				"message":     "Files processed",
				"attachments": attachmentInfo,
			})
		})

		// Create test request with file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, _ := writer.CreateFormFile("attachment", "test.txt")
		io.WriteString(part, "test content")
		writer.Close()

		req := httptest.NewRequest("POST", "/test", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp, "attachments")

		attachments := resp["attachments"].([]interface{})
		assert.Len(t, attachments, 1, "Should have processed one attachment")
	})
}
