package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/service"
)

// mimeByExtension maps file extensions to MIME types.
var mimeByExtension = map[string]string{
	".pdf":  "application/pdf",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".txt":  "text/plain",
	".html": "text/html",
	".csv":  "text/csv",
	".json": "application/json",
	".xml":  "application/xml",
	".zip":  "application/zip",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

// detectMimeType returns the MIME type for a filename based on extension.
func detectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mime, ok := mimeByExtension[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// handleServeFile serves files from storage with authorization checks.
func handleServeFile(c *gin.Context) {
	// Get the file path from URL
	filePath := c.Param("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File path is required"})
		return
	}

	// Remove leading slash if present
	filePath = strings.TrimPrefix(filePath, "/")

	// Security check: ensure path starts with "tickets/"
	if !strings.HasPrefix(filePath, "tickets/") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Extract ticket ID from path (format: tickets/{ticket_id}/...)
	pathParts := strings.Split(filePath, "/")
	if len(pathParts) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}

	// TODO: Add authorization check here
	// ticketID := pathParts[1]
	// Check if user has access to this ticket

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

	// Check if file exists
	exists, err := storageService.Exists(c.Request.Context(), filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check file"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Get file metadata
	metadata, err := storageService.GetMetadata(c.Request.Context(), filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file metadata"})
		return
	}

	// Retrieve file
	fileReader, err := storageService.Retrieve(c.Request.Context(), filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file"})
		return
	}
	defer fileReader.Close()

	filename := metadata.OriginalName
	contentType := detectMimeType(filename)

	// Set headers
	disposition := "inline"
	if c.Query("download") == "true" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", disposition+"; filename=\""+filename+"\"")
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, max-age=3600")

	// Stream file to response
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, fileReader); err != nil {
		c.Abort()
		return
	}
}
