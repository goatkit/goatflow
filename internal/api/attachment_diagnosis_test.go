package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Diagnostic test to understand why ticket creation with attachments fails.
func TestAttachmentProcessingDiagnosis(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Analyze multipart form processing in handleCreateTicket", func(t *testing.T) {
		// Create a test handler that mimics the beginning of handleCreateTicket
		// to see where the attachment processing fails

		router := gin.New()
		router.POST("/api/tickets", func(c *gin.Context) {
			// This mimics the struct and binding from handleCreateTicket
			var req struct {
				Title         string `json:"title" form:"title"`
				Subject       string `json:"subject" form:"subject"`
				CustomerEmail string `json:"customer_email" form:"customer_email" binding:"required,email"`
				CustomerName  string `json:"customer_name" form:"customer_name"`
				Priority      string `json:"priority" form:"priority"`
				QueueID       string `json:"queue_id" form:"queue_id"`
				TypeID        string `json:"type_id" form:"type_id"`
				Body          string `json:"body" form:"body" binding:"required"`
			}

			// This is the exact binding call from handleCreateTicket
			if err := c.ShouldBind(&req); err != nil {
				// This is likely where the 500 error occurs with attachments
				t.Logf("ShouldBind error with multipart: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Binding failed: " + err.Error()})
				return
			}

			// Try to access the uploaded files (this is missing in current handler)
			form, err := c.MultipartForm()
			if err != nil {
				t.Logf("MultipartForm error: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Multipart form error: " + err.Error()})
				return
			}

			files := form.File["attachment"]
			t.Logf("Found %d uploaded files", len(files))

			for i, file := range files {
				t.Logf("File %d: %s (%d bytes)", i, file.Filename, file.Size)
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Successfully processed multipart form with attachments",
				"title":   req.Title,
				"files":   len(files),
			})
		})

		// Create multipart form with attachment
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add form fields
		writer.WriteField("title", "Test ticket")
		writer.WriteField("customer_email", "user@example.com")
		writer.WriteField("body", "Test body")
		writer.WriteField("priority", "normal")

		// Add file attachment
		part, err := writer.CreateFormFile("attachment", "test.txt")
		require.NoError(t, err)
		_, err = io.WriteString(part, "test file content")
		require.NoError(t, err)

		writer.Close()

		// Make request
		req := httptest.NewRequest("POST", "/api/tickets", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		t.Logf("Response status: %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())

		// This test shows us what happens with multipart processing
		assert.NotEqual(t, http.StatusInternalServerError, w.Code, "Should not return 500 error")
	})

	t.Run("Test ShouldBind behavior with files", func(t *testing.T) {
		// Test what happens when ShouldBind encounters file uploads
		router := gin.New()
		router.POST("/test", func(c *gin.Context) {
			var req struct {
				Title string `form:"title" binding:"required"`
				Body  string `form:"body" binding:"required"`
			}

			// Test if ShouldBind handles multipart with files correctly
			err := c.ShouldBind(&req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
					"type":  "ShouldBind failed with multipart",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "ShouldBind succeeded"})
		})

		// Create multipart with file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("title", "Test")
		writer.WriteField("body", "Test body")

		part, _ := writer.CreateFormFile("attachment", "file.txt")
		io.WriteString(part, "content")
		writer.Close()

		req := httptest.NewRequest("POST", "/test", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		t.Logf("ShouldBind test status: %d", w.Code)
		t.Logf("ShouldBind test body: %s", w.Body.String())
	})
}
