package api

import (
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// handleServeFile serves files from storage with authorization checks
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

	// Detect content type from filename
	contentType := "application/octet-stream"
	filename := metadata.OriginalName
	if strings.HasSuffix(filename, ".pdf") {
		contentType = "application/pdf"
	} else if strings.HasSuffix(filename, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(filename, ".gif") {
		contentType = "image/gif"
	} else if strings.HasSuffix(filename, ".txt") {
		contentType = "text/plain"
	} else if strings.HasSuffix(filename, ".html") {
		contentType = "text/html"
	} else if strings.HasSuffix(filename, ".csv") {
		contentType = "text/csv"
	} else if strings.HasSuffix(filename, ".json") {
		contentType = "application/json"
	} else if strings.HasSuffix(filename, ".xml") {
		contentType = "application/xml"
	} else if strings.HasSuffix(filename, ".zip") {
		contentType = "application/zip"
	} else if strings.HasSuffix(filename, ".doc") {
		contentType = "application/msword"
	} else if strings.HasSuffix(filename, ".docx") {
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	} else if strings.HasSuffix(filename, ".xls") {
		contentType = "application/vnd.ms-excel"
	} else if strings.HasSuffix(filename, ".xlsx") {
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

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
		// Error already started writing, can't send JSON error
		c.Abort()
		return
	}
}