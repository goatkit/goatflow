//go:build ignore
package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// handleAttachmentDownload serves attachment files for download (unused in MVP)
// func handleAttachmentDownload(c *gin.Context) {
	// Get attachment ID from URL
	attachmentIDStr := c.Param("id")
	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Query attachment details
	var filename, contentType, contentSize string
	var content []byte
	var disposition sql.NullString

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT filename, COALESCE(content_type, 'application/octet-stream'), 
		       COALESCE(content_size, '0'), content, disposition
		FROM article_data_mime_attachment
		WHERE id = $1
	`), attachmentID).Scan(&filename, &contentType, &contentSize, &content, &disposition)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve attachment"})
		}
		return
	}

	// The content field may contain either actual file content or a file path (for backwards compatibility)
	// Check if content looks like a file path (starts with / or ./ and is relatively short)
	contentStr := string(content)
	if len(content) < 500 && (contentStr[0] == '/' || (len(contentStr) > 1 && contentStr[0] == '.' && contentStr[1] == '/')) {
		// Legacy attachment - content is a file path
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Legacy attachment format not supported. Please re-upload the attachment.",
			"details": "This attachment was created with an older version and needs to be re-uploaded.",
		})
		return
	}
	
	// Modern attachment - content is the actual file data
	// Set appropriate headers
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", len(content)))
	
	// Determine disposition (inline for images/PDFs, attachment for others)
	dispositionType := "attachment"
	if disposition.Valid && disposition.String == "inline" {
		dispositionType = "inline"
	} else if contentType == "image/png" || contentType == "image/jpeg" || 
	          contentType == "image/gif" || contentType == "application/pdf" {
		dispositionType = "inline"
	}
	
	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, dispositionType, filename))

	// Serve the content directly from database
	c.Data(http.StatusOK, contentType, content)
// }

// handleAttachmentPreview generates a preview or thumbnail for an attachment (unused in MVP)
// func handleAttachmentPreview(c *gin.Context) {
	// For now, just redirect to download
	// In the future, this could generate thumbnails for images
	attachmentID := c.Param("id")
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/api/attachments/%s/download", attachmentID))
// }

// handleAttachmentDelete removes an attachment (admin only) (unused in MVP)
// func handleAttachmentDelete(c *gin.Context) {
	// Get attachment ID
	attachmentIDStr := c.Param("id")
	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// TODO: Add proper permission check here
	// For now, allow all authenticated users to delete

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Delete from database (content is now stored in DB, not on disk)
	result, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM article_data_mime_attachment WHERE id = $1`), attachmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete attachment record"})
		return
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify deletion"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted successfully"})
// }

// GetAttachmentInfo retrieves basic info about an attachment
func GetAttachmentInfo(attachmentID int) (map[string]interface{}, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	var filename, contentType, contentSize string
	var articleID int

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT article_id, filename, 
		       COALESCE(content_type, 'application/octet-stream'),
		       COALESCE(content_size, '0')
		FROM article_data_mime_attachment
		WHERE id = $1
	`), attachmentID).Scan(&articleID, &filename, &contentType, &contentSize)

	if err != nil {
		return nil, err
	}

	size, _ := strconv.ParseInt(contentSize, 10, 64)

	return map[string]interface{}{
		"ID":          attachmentID,
		"ArticleID":   articleID,
		"Filename":    filename,
		"ContentType": contentType,
		"Size":        size,
		"SizeFormatted": formatFileSize(size),
		"URL":         fmt.Sprintf("/api/attachments/%d/download", attachmentID),
	}, nil
}