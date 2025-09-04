package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/redis/go-redis/v9"
)

// Deprecated in MVP: thumbnails disabled unless explicitly enabled
// var thumbnailService *service.ThumbnailService

// InitThumbnailService initializes the thumbnail service with Redis
/* func InitThumbnailService() error {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "valkey:6379", // Using valkey service name from docker-compose
		Password: "",             // No password in dev
		DB:       0,              // Default DB
	})
	
	// Test connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis/Valkey: %w", err)
	}
	
	thumbnailService = service.NewThumbnailService(redisClient)
	return nil
}

// handleAttachmentThumbnail generates or retrieves a thumbnail for an attachment
func handleAttachmentThumbnail(c *gin.Context) {
	// Get attachment ID from URL
	attachmentIDStr := c.Param("id")
	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}
	
	// Parse query parameters for thumbnail options
	width, _ := strconv.Atoi(c.DefaultQuery("w", "200"))
	height, _ := strconv.Atoi(c.DefaultQuery("h", "200"))
	quality, _ := strconv.Atoi(c.DefaultQuery("q", "85"))
	format := c.DefaultQuery("f", "jpeg")
	
	// Validate parameters
	if width < 50 || width > 800 {
		width = 200
	}
	if height < 50 || height > 800 {
		height = 200
	}
	if quality < 10 || quality > 100 {
		quality = 85
	}
	if format != "jpeg" && format != "png" {
		format = "jpeg"
	}
	
	opts := service.ThumbnailOptions{
		Width:   width,
		Height:  height,
		Quality: quality,
		Format:  format,
	}
	
	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	
	// Query attachment details
	var filename, contentType string
	var content []byte
	
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT filename, COALESCE(content_type, 'application/octet-stream'), content
		FROM article_data_mime_attachment
		WHERE id = $1
	`), attachmentID).Scan(&filename, &contentType, &content)
	
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve attachment"})
		}
		return
	}
	
	// Check if content is a file path (legacy format)
	contentStr := string(content)
	if len(content) < 500 && (contentStr[0] == '/' || (len(contentStr) > 1 && contentStr[0] == '.' && contentStr[1] == '/')) {
		// Return a placeholder for legacy attachments
		placeholder, placeholderType := service.GetPlaceholderThumbnail(contentType)
		c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
		c.Data(http.StatusOK, placeholderType, placeholder)
		return
	}
	
	// Initialize thumbnail service if not already done
    /* if thumbnailService == nil {
		if err := InitThumbnailService(); err != nil {
			// Fallback to placeholder if Redis is not available
			placeholder, placeholderType := service.GetPlaceholderThumbnail(contentType)
			c.Header("Cache-Control", "public, max-age=3600")
			c.Data(http.StatusOK, placeholderType, placeholder)
			return
		}
    } */
	
	// Check if this is an image that can be thumbnailed
	if !service.IsSupportedImageType(contentType) {
		// Return placeholder for non-image files
		placeholder, placeholderType := service.GetPlaceholderThumbnail(contentType)
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, placeholderType, placeholder)
		return
	}
	
	// Generate or get cached thumbnail
    // Thumbnails disabled: return placeholder for now
    placeholder, placeholderType := service.GetPlaceholderThumbnail(contentType)
    c.Header("Cache-Control", "public, max-age=86400")
    c.Data(http.StatusOK, placeholderType, placeholder)
    return
	
	// Set caching headers
	c.Header("Cache-Control", "public, max-age=604800") // Cache for 7 days
	c.Header("ETag", fmt.Sprintf(`"%d-%dx%d-q%d"`, attachmentID, width, height, quality))
	
	// Check if client has cached version
	if match := c.GetHeader("If-None-Match"); match != "" {
		expectedETag := fmt.Sprintf(`"%d-%dx%d-q%d"`, attachmentID, width, height, quality)
		if match == expectedETag {
			c.Status(http.StatusNotModified)
			return
		}
	}
	
	// Serve the thumbnail
    // c.Data(http.StatusOK, thumbnailType, thumbnailData)
}

// handleBulkThumbnails generates thumbnails for multiple attachments (for preloading)
func handleBulkThumbnails(c *gin.Context) {
	// Parse request body
	var req struct {
		AttachmentIDs []int `json:"attachment_ids" binding:"required"`
		Width         int   `json:"width"`
		Height        int   `json:"height"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	// Set defaults
	if req.Width == 0 {
		req.Width = 200
	}
	if req.Height == 0 {
		req.Height = 200
	}
	
	// Initialize service if needed
    // Thumbnails disabled in MVP; return computed URLs only
	
	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	
	results := make(map[int]string)
	opts := service.ThumbnailOptions{
		Width:   req.Width,
		Height:  req.Height,
		Quality: 85,
		Format:  "jpeg",
	}
	
	for _, attachmentID := range req.AttachmentIDs {
		// Query attachment
		var contentType string
		var content []byte
		
        err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT COALESCE(content_type, 'application/octet-stream'), content
			FROM article_data_mime_attachment
			WHERE id = $1
		`), attachmentID).Scan(&contentType, &content)
		
		if err != nil {
			continue // Skip this attachment
		}
		
        // Check if it's an image
        if !service.IsSupportedImageType(contentType) {
			results[attachmentID] = fmt.Sprintf("/api/attachments/%d/thumbnail", attachmentID)
			continue
		}
        // Return URL; generation disabled
		// Add to results regardless of success (URL will work either way)
		results[attachmentID] = fmt.Sprintf("/api/attachments/%d/thumbnail?w=%d&h=%d", 
			attachmentID, req.Width, req.Height)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"thumbnails": results,
		"cached":     true,
	})
}