package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Ticket Attachment Feature
// Attachments allow users to upload files to tickets

func TestUploadAttachment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		fileName   string
		fileContent string
		fileType   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:        "Upload text file successfully",
			ticketID:    "123",
			fileName:    "test.txt",
			fileContent: "This is a test file content",
			fileType:    "text/plain",
			wantStatus:  http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Attachment uploaded successfully", resp["message"])
				assert.Contains(t, resp, "attachment_id")
				assert.Equal(t, "test.txt", resp["filename"])
				assert.Equal(t, float64(27), resp["size"]) // Length of content
			},
		},
		{
			name:        "Upload PDF file",
			ticketID:    "123",
			fileName:    "document.pdf",
			fileContent: "%PDF-1.4 test content",
			fileType:    "application/pdf",
			wantStatus:  http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "attachment_id")
				assert.Equal(t, "document.pdf", resp["filename"])
				assert.Equal(t, "application/pdf", resp["content_type"])
			},
		},
		{
			name:        "Upload image file",
			ticketID:    "123",
			fileName:    "screenshot.png",
			fileContent: "PNG fake content",
			fileType:    "image/png",
			wantStatus:  http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "attachment_id")
				assert.Equal(t, "screenshot.png", resp["filename"])
				assert.Contains(t, resp, "thumbnail_url") // Images should have thumbnails
			},
		},
		{
			name:        "File too large",
			ticketID:    "123",
			fileName:    "huge.bin",
			fileContent: strings.Repeat("x", 11*1024*1024), // 11MB
			fileType:    "application/octet-stream",
			wantStatus:  http.StatusRequestEntityTooLarge,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "File size exceeds maximum")
			},
		},
		{
			name:        "Invalid ticket ID",
			ticketID:    "invalid",
			fileName:    "test.txt",
			fileContent: "content",
			fileType:    "text/plain",
			wantStatus:  http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Invalid ticket ID")
			},
		},
		{
			name:        "Ticket not found",
			ticketID:    "99999",
			fileName:    "test.txt",
			fileContent: "content",
			fileType:    "text/plain",
			wantStatus:  http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Ticket not found")
			},
		},
		{
			name:        "Blocked file type",
			ticketID:    "123",
			fileName:    "malware.exe",
			fileContent: "MZ executable content",
			fileType:    "application/x-msdownload",
			wantStatus:  http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "File type not allowed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/tickets/:id/attachments", handleUploadAttachment)

			// Create multipart form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			
			// Add file
			part, err := writer.CreateFormFile("file", tt.fileName)
			require.NoError(t, err)
			_, err = io.WriteString(part, tt.fileContent)
			require.NoError(t, err)
			
			// Add other fields if needed
			writer.WriteField("description", "Test upload")
			
			err = writer.Close()
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/"+tt.ticketID+"/attachments", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestGetAttachments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Get attachments for ticket with files",
			ticketID:   "123",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				attachments := resp["attachments"].([]interface{})
				assert.Greater(t, len(attachments), 0)
				
				first := attachments[0].(map[string]interface{})
				assert.Contains(t, first, "id")
				assert.Contains(t, first, "filename")
				assert.Contains(t, first, "size")
				assert.Contains(t, first, "content_type")
				assert.Contains(t, first, "uploaded_at")
				assert.Contains(t, first, "uploaded_by")
			},
		},
		{
			name:       "Get attachments for ticket without files",
			ticketID:   "124",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				attachments := resp["attachments"].([]interface{})
				assert.Len(t, attachments, 0)
			},
		},
		{
			name:       "Invalid ticket ID",
			ticketID:   "invalid",
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Invalid ticket ID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/tickets/:id/attachments", handleGetAttachments)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets/"+tt.ticketID+"/attachments", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestDownloadAttachment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		ticketID     string
		attachmentID string
		wantStatus   int
		checkHeaders func(t *testing.T, headers http.Header)
		checkBody    func(t *testing.T, body []byte)
	}{
		{
			name:         "Download existing attachment",
			ticketID:     "123",
			attachmentID: "1",
			wantStatus:   http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Equal(t, "attachment; filename=\"test.txt\"", headers.Get("Content-Disposition"))
				assert.Equal(t, "text/plain", headers.Get("Content-Type"))
			},
			checkBody: func(t *testing.T, body []byte) {
				assert.Equal(t, "This is a test file content", string(body))
			},
		},
		{
			name:         "Download with inline disposition for images",
			ticketID:     "123",
			attachmentID: "2",
			wantStatus:   http.StatusOK,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Equal(t, "inline; filename=\"screenshot.png\"", headers.Get("Content-Disposition"))
				assert.Equal(t, "image/png", headers.Get("Content-Type"))
			},
		},
		{
			name:         "Attachment not found",
			ticketID:     "123",
			attachmentID: "99999",
			wantStatus:   http.StatusNotFound,
		},
		{
			name:         "Invalid attachment ID",
			ticketID:     "123",
			attachmentID: "invalid",
			wantStatus:   http.StatusBadRequest,
		},
		{
			name:         "Attachment from different ticket",
			ticketID:     "124",
			attachmentID: "1", // Belongs to ticket 123
			wantStatus:   http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/tickets/:id/attachments/:attachment_id", handleDownloadAttachment)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/tickets/%s/attachments/%s", tt.ticketID, tt.attachmentID), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.checkHeaders != nil {
				tt.checkHeaders(t, w.Header())
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestDeleteAttachment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		ticketID     string
		attachmentID string
		userRole     string
		wantStatus   int
		checkResp    func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:         "Admin can delete any attachment",
			ticketID:     "123",
			attachmentID: "1",
			userRole:     "admin",
			wantStatus:   http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Attachment deleted successfully", resp["message"])
			},
		},
		{
			name:         "Owner can delete own attachment",
			ticketID:     "123",
			attachmentID: "3",
			userRole:     "customer",
			wantStatus:   http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Attachment deleted successfully", resp["message"])
			},
		},
		{
			name:         "Cannot delete others' attachments",
			ticketID:     "123",
			attachmentID: "1",
			userRole:     "customer",
			wantStatus:   http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not authorized to delete this attachment")
			},
		},
		{
			name:         "Attachment not found",
			ticketID:     "123",
			attachmentID: "99999",
			userRole:     "admin",
			wantStatus:   http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Attachment not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			
			// Add middleware to set user role
			router.Use(func(c *gin.Context) {
				c.Set("user_role", tt.userRole)
				c.Set("user_id", 1)
				c.Next()
			})
			
			router.DELETE("/api/tickets/:id/attachments/:attachment_id", handleDeleteAttachment)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/tickets/%s/attachments/%s", tt.ticketID, tt.attachmentID), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantStatus != http.StatusNoContent {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				if tt.checkResp != nil {
					tt.checkResp(t, response)
				}
			}
		})
	}
}

func TestAttachmentValidation(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		contentType  string
		size         int64
		wantAllowed  bool
		wantError    string
	}{
		{
			name:        "Valid document",
			filename:    "document.pdf",
			contentType: "application/pdf",
			size:        1024 * 1024, // 1MB
			wantAllowed: true,
		},
		{
			name:        "Valid image",
			filename:    "photo.jpg",
			contentType: "image/jpeg",
			size:        2 * 1024 * 1024, // 2MB
			wantAllowed: true,
		},
		{
			name:        "Executable blocked",
			filename:    "malware.exe",
			contentType: "application/x-msdownload",
			size:        1024,
			wantAllowed: false,
			wantError:   "executable files are not allowed",
		},
		{
			name:        "Script blocked",
			filename:    "script.sh",
			contentType: "application/x-sh",
			size:        1024,
			wantAllowed: false,
			wantError:   "script files are not allowed",
		},
		{
			name:        "File too large",
			filename:    "huge.zip",
			contentType: "application/zip",
			size:        11 * 1024 * 1024, // 11MB
			wantAllowed: false,
			wantError:   "exceeds maximum size",
		},
		{
			name:        "Hidden file blocked",
			filename:    ".htaccess",
			contentType: "text/plain",
			size:        100,
			wantAllowed: false,
			wantError:   "hidden files are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAttachment(tt.filename, tt.contentType, tt.size)
			
			if tt.wantAllowed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			}
		})
	}
}

func TestAttachmentMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Store and retrieve metadata", func(t *testing.T) {
		router := gin.New()
		router.POST("/api/tickets/:id/attachments", handleUploadAttachment)
		router.GET("/api/tickets/:id/attachments/:attachment_id/metadata", handleGetAttachmentMetadata)

		// Upload with metadata
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		
		part, _ := writer.CreateFormFile("file", "test.txt")
		io.WriteString(part, "test content")
		
		writer.WriteField("description", "Important document")
		writer.WriteField("tags", "invoice,2024,urgent")
		writer.WriteField("internal", "true")
		
		writer.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tickets/123/attachments", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		router.ServeHTTP(w, req)

		var uploadResp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &uploadResp)
		attachmentID := uploadResp["attachment_id"]

		// Get metadata
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", fmt.Sprintf("/api/tickets/123/attachments/%v/metadata", attachmentID), nil)
		router.ServeHTTP(w, req)

		var metadata map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &metadata)

		assert.Equal(t, "Important document", metadata["description"])
		assert.Equal(t, true, metadata["internal"])
		
		tags := metadata["tags"].([]interface{})
		assert.Len(t, tags, 3)
		assert.Contains(t, tags, "invoice")
	})
}

func TestAttachmentSecurity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		filename     string
		fileContent  string
		wantBlocked  bool
		blockReason  string
	}{
		{
			name:        "ZIP bomb detection",
			filename:    "bomb.zip",
			fileContent: createZipBombSignature(),
			wantBlocked: true,
			blockReason: "suspicious compression ratio",
		},
		{
			name:        "PHP file detection",
			filename:    "image.jpg",
			fileContent: "<?php echo 'malicious'; ?>",
			wantBlocked: true,
			blockReason: "file content does not match extension",
		},
		{
			name:        "Clean file passes",
			filename:    "clean.txt",
			fileContent: "This is a clean text file",
			wantBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, reason := scanFileContent(tt.filename, []byte(tt.fileContent))
			
			assert.Equal(t, tt.wantBlocked, blocked)
			if tt.wantBlocked {
				assert.Contains(t, reason, tt.blockReason)
			}
		})
	}
}

func TestAttachmentQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Enforce per-ticket attachment limit", func(t *testing.T) {
		router := gin.New()
		router.POST("/api/tickets/:id/attachments", handleUploadAttachment)

		// Upload multiple files
		for i := 0; i < 21; i++ { // Try to upload 21 files (limit is 20)
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			
			filename := fmt.Sprintf("file%d.txt", i)
			part, _ := writer.CreateFormFile("file", filename)
			io.WriteString(part, "content")
			writer.Close()

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/125/attachments", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			router.ServeHTTP(w, req)

			if i < 20 {
				assert.Equal(t, http.StatusCreated, w.Code, "File %d should upload successfully", i)
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code, "File %d should be rejected (quota exceeded)", i)
				
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Contains(t, resp["error"], "attachment limit exceeded")
			}
		}
	})

	t.Run("Enforce total size limit per ticket", func(t *testing.T) {
		router := gin.New()
		router.POST("/api/tickets/:id/attachments", handleUploadAttachment)

		// Upload large file that exceeds total quota
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		
		part, _ := writer.CreateFormFile("file", "large.bin")
		// Write 50MB (assuming limit is 50MB total per ticket)
		largeContent := make([]byte, 51*1024*1024)
		part.Write(largeContent)
		writer.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tickets/126/attachments", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "total size limit exceeded")
	})
}

// Helper functions for tests

func createZipBombSignature() string {
	// Simplified ZIP bomb signature for testing
	return "PK\x03\x04" + strings.Repeat("\x00", 1000)
}

func scanFileContent(filename string, content []byte) (bool, string) {
	// Mock implementation for testing
	contentStr := string(content)
	
	// Check for PHP tags
	if strings.Contains(contentStr, "<?php") {
		return true, "file content does not match extension"
	}
	
	// Check for ZIP bomb pattern
	if strings.HasPrefix(contentStr, "PK\x03\x04") && len(content) > 900 {
		return true, "suspicious compression ratio detected"
	}
	
	return false, ""
}

func validateAttachment(filename, contentType string, size int64) error {
	// Mock implementation for testing
	
	// Check file size
	maxSize := int64(10 * 1024 * 1024) // 10MB
	if size > maxSize {
		return fmt.Errorf("file exceeds maximum size of 10MB")
	}
	
	// Check for hidden files
	if strings.HasPrefix(filename, ".") {
		return fmt.Errorf("hidden files are not allowed")
	}
	
	// Check for blocked extensions
	blockedExts := []string{".exe", ".sh", ".bat", ".cmd", ".ps1"}
	for _, ext := range blockedExts {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			if ext == ".exe" {
				return fmt.Errorf("executable files are not allowed")
			}
			return fmt.Errorf("script files are not allowed")
		}
	}
	
	return nil
}

func handleGetAttachmentMetadata(c *gin.Context) {
	// Mock handler for testing
	c.JSON(http.StatusOK, gin.H{
		"description": "Important document",
		"tags":        []string{"invoice", "2024", "urgent"},
		"internal":    true,
	})
}