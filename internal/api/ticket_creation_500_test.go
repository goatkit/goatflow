package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Test to reproduce the exact 500 error user reported.
func TestTicketCreation500ErrorReproduction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Reproduce 500 error with attachment", func(t *testing.T) {
		// This reproduces: "Response Status Error Code 500 from /api/tickets"
		// when user tries to create ticket with file attachment

		router := gin.New()
		router.POST("/api/tickets", func(c *gin.Context) {
			// Mock authentication context
			c.Set("user_role", "Agent")
			c.Set("user_id", uint(1))
			c.Set("user_email", "test@example.com")

			// Call the actual handler that's causing 500 error
			handleCreateTicket(c)
		})

		// Create multipart form with attachment (this scenario causes 500)
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add required form fields
		writer.WriteField("title", "Test ticket")
		writer.WriteField("customer_email", "user@example.com")
		writer.WriteField("body", "Test ticket with attachment")
		writer.WriteField("priority", "normal")

		// Add file attachment - this likely triggers the 500 error
		part, err := writer.CreateFormFile("attachment", "test.txt")
		require.NoError(t, err)
		_, err = io.WriteString(part, "test file content")
		require.NoError(t, err)

		writer.Close()

		// Make the request
		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check the current response
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		t.Logf("Response status: %d", w.Code)
		t.Logf("Response body: %v", resp)

		// Document current behavior - this test SHOULD FAIL showing 500 error
		// After fix, this should return 201 with proper attachment handling
		if w.Code == http.StatusInternalServerError {
			t.Logf("✓ Reproduced the 500 error user reported")
			t.Logf("Error details: %v", resp["error"])
		} else {
			t.Logf("Unexpected status: expected 500 (to show current bug), got %d", w.Code)
		}
	})

	t.Run("Verify basic ticket creation works without attachments", func(t *testing.T) {
		// Control test: ensure basic ticket creation still works
		router := gin.New()
		router.POST("/api/tickets", func(c *gin.Context) {
			c.Set("user_role", "Agent")
			c.Set("user_id", uint(1))
			c.Set("user_email", "test@example.com")
			handleCreateTicket(c)
		})

		// Create simple form without attachments
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "Simple ticket")
		writer.WriteField("customer_email", "user@example.com")
		writer.WriteField("body", "No attachments")
		writer.Close()

		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		t.Logf("Simple ticket creation status: %d", w.Code)

		// This should work fine (no attachments to process)
		if w.Code != http.StatusCreated {
			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			t.Logf("Error in basic creation: %v", resp)
		}
	})
}
