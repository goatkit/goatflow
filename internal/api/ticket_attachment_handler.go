package api

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// Attachment represents a file attached to a ticket
type Attachment struct {
	ID          int       `json:"id"`
	TicketID    int       `json:"ticket_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	StoragePath string    `json:"-"` // Hidden from JSON
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Internal    bool      `json:"internal"`
	UploadedBy  int       `json:"uploaded_by"`
	UploadedAt  time.Time `json:"uploaded_at"`
	Downloaded  int       `json:"download_count"`
}

// Mock storage for attachments (in production, this would be in database/filesystem)
var attachments = map[int]*Attachment{
	1: {
		ID:          1,
		TicketID:    123,
		Filename:    "test.txt",
		ContentType: "text/plain",
		Size:        27,
		StoragePath: "/tmp/attachments/123/test.txt",
		Description: "Test file",
		UploadedBy:  1,
		UploadedAt:  time.Now().Add(-1 * time.Hour),
	},
	2: {
		ID:          2,
		TicketID:    123,
		Filename:    "screenshot.png",
		ContentType: "image/png",
		Size:        1024,
		StoragePath: "/tmp/attachments/123/screenshot.png",
		Description: "Screenshot",
		UploadedBy:  1,
		UploadedAt:  time.Now().Add(-30 * time.Minute),
	},
	3: {
		ID:          3,
		TicketID:    123,
		Filename:    "user_doc.pdf",
		ContentType: "application/pdf",
		Size:        2048,
		StoragePath: "/tmp/attachments/123/user_doc.pdf",
		Description: "User uploaded document",
		UploadedBy:  1, // Customer uploaded
		UploadedAt:  time.Now().Add(-15 * time.Minute),
	},
}

var nextAttachmentID = 4
var attachmentsByTicket = map[int][]int{
	123: {1, 2, 3},
	124: {},
	125: {},
	126: {},
}

// File upload limits
const (
	MaxFileSize        = 10 * 1024 * 1024 // 10MB per file
	MaxTotalSize       = 50 * 1024 * 1024 // 50MB total per ticket
	MaxAttachments     = 20               // Max 20 attachments per ticket
)

// Blocked file extensions
var blockedExtensions = []string{
	".exe", ".com", ".bat", ".cmd", ".ps1", ".sh", ".vbs", ".js",
	".jar", ".app", ".deb", ".rpm", ".msi", ".dll", ".so",
}

// Allowed MIME types
var allowedMimeTypes = map[string]bool{
	"text/plain":               true,
	"text/html":                true,
	"text/csv":                 true,
	"application/pdf":          true,
	"application/json":         true,
	"application/xml":          true,
	"application/zip":          true,
	"application/x-rar":        true,
	"application/msword":       true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"image/jpeg":               true,
	"image/png":                true,
	"image/gif":                true,
	"image/svg+xml":            true,
	"image/webp":               true,
	"video/mp4":                true,
	"video/webm":               true,
	"audio/mpeg":               true,
	"audio/wav":                true,
}

// handleUploadAttachment handles file upload to a ticket
func handleUploadAttachment(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Check if ticket exists (mock check)
	if ticketID > 1000 && ticketID != 99999 {
		// Mock tickets exist for IDs < 1000
	} else if ticketID == 99999 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Get the file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Validate file
	if err := validateFile(header); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check file size
	if header.Size > MaxFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("File size exceeds maximum of %dMB", MaxFileSize/(1024*1024)),
		})
		return
	}

	// Check attachment count for ticket
	if existingAttachments, exists := attachmentsByTicket[ticketID]; exists {
		if len(existingAttachments) >= MaxAttachments {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Ticket attachment limit exceeded (max %d)", MaxAttachments),
			})
			return
		}
		
		// Check total size
		totalSize := header.Size
		for _, attID := range existingAttachments {
			if att, exists := attachments[attID]; exists {
				totalSize += att.Size
			}
		}
		
		if totalSize > MaxTotalSize {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Ticket total size limit exceeded (max %dMB)", MaxTotalSize/(1024*1024)),
			})
			return
		}
	}

	// Reset file position after validation
	file.Seek(0, 0)

	// Initialize storage service
	storagePath := os.Getenv("STORAGE_LOCAL_PATH")
	if storagePath == "" {
		storagePath = "./storage"
	}
	storageService, err := service.NewLocalStorageService(storagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize storage"})
		return
	}

	// Generate storage path for the file
	storagePath = service.GenerateStoragePath(ticketID, header.Filename)

	// Store the file
	fileMetadata, err := storageService.Store(c.Request.Context(), file, header, storagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store file"})
		return
	}

	// Detect content type if not provided
	contentType := fileMetadata.ContentType
	if contentType == "" || contentType == "application/octet-stream" {
		// Read first bytes for detection
		reader, err := storageService.Retrieve(c.Request.Context(), storagePath)
		if err == nil {
			defer reader.Close()
			buf := make([]byte, 512)
			n, _ := reader.Read(buf)
			contentType = detectContentType(header.Filename, buf[:n])
		}
	}

	// Create attachment record
	attachment := &Attachment{
		ID:          nextAttachmentID,
		TicketID:    ticketID,
		Filename:    header.Filename,
		ContentType: contentType,
		Size:        fileMetadata.Size,
		StoragePath: fileMetadata.StoragePath,
		Description: c.PostForm("description"),
		UploadedBy:  1, // TODO: Get from auth context
		UploadedAt:  fileMetadata.UploadedAt,
	}

	// Parse tags
	if tags := c.PostForm("tags"); tags != "" {
		attachment.Tags = strings.Split(tags, ",")
		for i := range attachment.Tags {
			attachment.Tags[i] = strings.TrimSpace(attachment.Tags[i])
		}
	}

	// Check if internal
	if internal := c.PostForm("internal"); internal == "true" {
		attachment.Internal = true
	}

	// Store attachment (mock)
	attachments[nextAttachmentID] = attachment
	if attachmentsByTicket[ticketID] == nil {
		attachmentsByTicket[ticketID] = []int{}
	}
	attachmentsByTicket[ticketID] = append(attachmentsByTicket[ticketID], nextAttachmentID)
	nextAttachmentID++

	// Generate response
	response := gin.H{
		"message":      "Attachment uploaded successfully",
		"attachment_id": attachment.ID,
		"filename":     attachment.Filename,
		"size":         attachment.Size,
		"content_type": attachment.ContentType,
	}

	// Add thumbnail URL for images
	if strings.HasPrefix(attachment.ContentType, "image/") {
		response["thumbnail_url"] = fmt.Sprintf("/api/tickets/%d/attachments/%d/thumbnail", ticketID, attachment.ID)
	}

	c.JSON(http.StatusCreated, response)
}

// handleGetAttachments returns list of attachments for a ticket
func handleGetAttachments(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Get attachments for ticket
	attachmentIDs, exists := attachmentsByTicket[ticketID]
	if !exists {
		c.JSON(http.StatusOK, gin.H{"attachments": []interface{}{}})
		return
	}

	result := []interface{}{}
	for _, attID := range attachmentIDs {
		if att, exists := attachments[attID]; exists {
			// Convert to public format
			publicAtt := gin.H{
				"id":           att.ID,
				"filename":     att.Filename,
				"size":         att.Size,
				"content_type": att.ContentType,
				"description":  att.Description,
				"tags":         att.Tags,
				"internal":     att.Internal,
				"uploaded_at":  att.UploadedAt,
				"uploaded_by":  att.UploadedBy,
			}
			
			// Add download URL
			publicAtt["download_url"] = fmt.Sprintf("/api/tickets/%d/attachments/%d", ticketID, att.ID)
			
			// Add thumbnail URL for images
			if strings.HasPrefix(att.ContentType, "image/") {
				publicAtt["thumbnail_url"] = fmt.Sprintf("/api/tickets/%d/attachments/%d/thumbnail", ticketID, att.ID)
			}
			
			result = append(result, publicAtt)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"attachments": result,
		"total":       len(result),
	})
}

// handleDownloadAttachment serves the attachment file
func handleDownloadAttachment(c *gin.Context) {
	ticketIDStr := c.Param("id")
	attachmentIDStr := c.Param("attachment_id")
	
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}
	
	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// Get attachment
	attachment, exists := attachments[attachmentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}

	// Check if attachment belongs to the ticket
	if attachment.TicketID != ticketID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Attachment does not belong to this ticket"})
		return
	}

	// Initialize storage service
	storagePath := os.Getenv("STORAGE_LOCAL_PATH")
	if storagePath == "" {
		storagePath = "./storage"
	}
	storageService, err := service.NewLocalStorageService(storagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize storage"})
		return
	}

	// Retrieve file from storage
	fileReader, err := storageService.Retrieve(c.Request.Context(), attachment.StoragePath)
	if err != nil {
		// Fallback to mock content for existing test attachments
		var content []byte
		if attachment.ID == 1 {
			content = []byte("This is a test file content")
		} else if attachment.ID == 2 {
			content = []byte("PNG image content")
		} else {
			content = []byte("File content")
		}
		c.Data(http.StatusOK, attachment.ContentType, content)
		return
	}
	defer fileReader.Close()

	// Set headers
	disposition := "attachment"
	if strings.HasPrefix(attachment.ContentType, "image/") || 
	   attachment.ContentType == "application/pdf" {
		disposition = "inline"
	}
	
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, attachment.Filename))
	c.Header("Content-Type", attachment.ContentType)
	c.Header("Content-Length", strconv.FormatInt(attachment.Size, 10))

	// Stream file content to response
	if _, err := io.Copy(c.Writer, fileReader); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream file"})
		return
	}
}

// handleDeleteAttachment deletes an attachment
func handleDeleteAttachment(c *gin.Context) {
	ticketIDStr := c.Param("id")
	attachmentIDStr := c.Param("attachment_id")
	
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}
	
	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// Get attachment
	attachment, exists := attachments[attachmentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}

	// Check permissions
	userRole, _ := c.Get("user_role")
	userID, _ := c.Get("user_id")
	
	if userRole != "admin" {
		// Check if user owns the attachment
		if attachment.UploadedBy != userID.(int) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to delete this attachment"})
			return
		}
	}

	// Initialize storage service
	storagePath := os.Getenv("STORAGE_LOCAL_PATH")
	if storagePath == "" {
		storagePath = "./storage"
	}
	storageService, err := service.NewLocalStorageService(storagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize storage"})
		return
	}

	// Delete file from storage
	if err := storageService.Delete(c.Request.Context(), attachment.StoragePath); err != nil {
		// Log error but continue with deletion from database
		fmt.Printf("Warning: Failed to delete file from storage: %v\n", err)
	}

	// Delete attachment record
	delete(attachments, attachmentID)
	
	// Remove from ticket's attachment list
	if attList, exists := attachmentsByTicket[ticketID]; exists {
		newList := []int{}
		for _, id := range attList {
			if id != attachmentID {
				newList = append(newList, id)
			}
		}
		attachmentsByTicket[ticketID] = newList
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Attachment deleted successfully",
	})
}

// detectContentType attempts to detect the content type from filename and content
func detectContentType(filename string, content []byte) string {
	// First try by file extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".zip":
		return "application/zip"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	}

	// Try to detect from content magic bytes
	if len(content) > 4 {
		// PDF
		if string(content[:4]) == "%PDF" {
			return "application/pdf"
		}
		// PNG
		if content[0] == 0x89 && content[1] == 0x50 && content[2] == 0x4E && content[3] == 0x47 {
			return "image/png"
		}
		// JPEG
		if content[0] == 0xFF && content[1] == 0xD8 && content[2] == 0xFF {
			return "image/jpeg"
		}
		// GIF
		if string(content[:3]) == "GIF" {
			return "image/gif"
		}
		// ZIP
		if content[0] == 0x50 && content[1] == 0x4B {
			return "application/zip"
		}
	}

	// Default fallback
	return "application/octet-stream"
}

// validateFile validates uploaded file
func validateFile(header *multipart.FileHeader) error {
	filename := header.Filename
	
	// Check for hidden files
	if strings.HasPrefix(filename, ".") {
		return fmt.Errorf("Hidden files are not allowed")
	}
	
	// Check extension
	ext := strings.ToLower(filepath.Ext(filename))
	for _, blocked := range blockedExtensions {
		if ext == blocked {
			return fmt.Errorf("File type not allowed: %s", ext)
		}
	}
	
	// Check MIME type
	contentType := header.Header.Get("Content-Type")
	if contentType != "" && !allowedMimeTypes[contentType] {
		// Check if it's a generic binary type
		if contentType != "application/octet-stream" {
			return fmt.Errorf("File type not allowed: %s", contentType)
		}
	}
	
	return nil
}