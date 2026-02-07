package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// verifyCustomerOwnsTicket checks if the authenticated customer owns the specified ticket.
// Returns the ticket ID if valid, or 0 and sends an error response if not.
func verifyCustomerOwnsTicket(c *gin.Context, db *sql.DB, ticketIDStr, username string) (int, bool) {
	// Try to parse as numeric ID first
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		// Maybe it's a ticket number (TN)
		row := db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket WHERE tn = ? LIMIT 1`), ticketIDStr)
		if scanErr := row.Scan(&ticketID); scanErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return 0, false
		}
	}

	// Verify customer owns this ticket
	var exists bool
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT EXISTS(SELECT 1 FROM ticket WHERE id = ? AND customer_user_id = ?)
	`), ticketID, username).Scan(&exists)

	if err != nil || !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return 0, false
	}

	return ticketID, true
}

// handleCustomerGetAttachments returns list of attachments for a customer's ticket.
func handleCustomerGetAttachments(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		ticketIDStr := c.Param("id")

		ticketID, ok := verifyCustomerOwnsTicket(c, db, ticketIDStr, username)
		if !ok {
			return
		}

		// Query attachments from database - only customer-visible articles
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT att.id, att.filename,
			       COALESCE(att.content_type, 'application/octet-stream'),
			       COALESCE(att.content_size, 0),
			       att.create_time, att.create_by,
			       att.article_id
			FROM article_data_mime_attachment att
			INNER JOIN article a ON att.article_id = a.id
			WHERE a.ticket_id = ? AND a.is_visible_for_customer = 1
			ORDER BY att.id
		`), ticketID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query attachments"})
			return
		}
		defer rows.Close()

		result := []gin.H{}
		for rows.Next() {
			var attID, articleID, createBy int
			var filename, contentType string
			var contentSize int64
			var createTime time.Time

			err := rows.Scan(&attID, &filename, &contentType, &contentSize, &createTime, &createBy, &articleID)
			if err != nil {
				continue
			}

			publicAtt := gin.H{
				"id":             attID,
				"filename":       filename,
				"size":           contentSize,
				"size_formatted": formatFileSize(contentSize),
				"content_type":   contentType,
				"uploaded_at":    createTime.Format("Jan 2, 2006 3:04 PM"),
				"uploaded_by":    createBy,
				"article_id":     articleID,
				"download_url":   fmt.Sprintf("/customer/tickets/%d/attachments/%d", ticketID, attID),
			}

			// Add thumbnail URL for images
			if strings.HasPrefix(contentType, "image/") {
				publicAtt["thumbnail_url"] = fmt.Sprintf("/customer/tickets/%d/attachments/%d/thumbnail", ticketID, attID)
			}

			result = append(result, publicAtt)
		}
		if err := rows.Err(); err != nil {
			log.Printf("Error iterating attachments: %v", err)
		}

		// Check if this is an HTMX request
		if c.GetHeader("HX-Request") == "true" {
			html := renderCustomerAttachmentListHTML(result, ticketID)
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		} else {
			c.JSON(http.StatusOK, gin.H{
				"attachments": result,
				"total":       len(result),
			})
		}
	}
}

// handleCustomerUploadAttachment handles file upload for customer tickets.
func handleCustomerUploadAttachment(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		ticketIDStr := c.Param("id")
		systemUserID := 1 // System user for create_by/change_by

		ticketID, ok := verifyCustomerOwnsTicket(c, db, ticketIDStr, username)
		if !ok {
			return
		}

		// Parse multipart form
		if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}

		// Get files from form
		files := getFormFiles(c.Request.MultipartForm)
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
			return
		}

		// Get the latest customer-visible article for this ticket to attach files to
		var articleID int
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT id FROM article
			WHERE ticket_id = ? AND is_visible_for_customer = 1
			ORDER BY id DESC LIMIT 1
		`), ticketID).Scan(&articleID)
		if err != nil {
			// No article exists, create one
			result, insertErr := db.Exec(database.ConvertPlaceholders(`
				INSERT INTO article (
					ticket_id, article_sender_type_id, communication_channel_id,
					is_visible_for_customer, search_index_needs_rebuild,
					create_time, create_by, change_time, change_by
				) VALUES (?, 3, 1, 1, 1, NOW(), ?, NOW(), ?)
			`), ticketID, systemUserID, systemUserID)
			if insertErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article for attachment"})
				return
			}
			articleIDInt64, _ := result.LastInsertId()
			articleID = int(articleIDInt64)

			// Insert article_data_mime record
			_, _ = db.Exec(database.ConvertPlaceholders(`
				INSERT INTO article_data_mime (
					article_id, a_from, a_subject, a_body, a_content_type,
					incoming_time, create_time, create_by, change_time, change_by
				) VALUES (?, ?, 'Attachment', '', 'text/plain',
					UNIX_TIMESTAMP(), NOW(), ?, NOW(), ?)
			`), articleID, username, systemUserID, systemUserID)
		}

		// Process attachments using the shared helper
		processFormAttachments(files, attachmentProcessParams{
			ctx:       context.Background(),
			db:        db,
			ticketID:  ticketID,
			articleID: articleID,
			userID:    systemUserID,
		})

		// Return success with HTMX trigger
		c.Header("HX-Trigger", "attachments-updated")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Attachments uploaded successfully",
		})
	}
}

// handleCustomerDownloadAttachment serves the attachment file for customers.
func handleCustomerDownloadAttachment(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		ticketIDStr := c.Param("id")
		attachmentIDStr := c.Param("attachment_id")

		ticketID, ok := verifyCustomerOwnsTicket(c, db, ticketIDStr, username)
		if !ok {
			return
		}

		attachmentID, err := strconv.Atoi(attachmentIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
			return
		}

		// Get attachment - ensure it's from a customer-visible article
		var (
			filename     string
			contentType  string
			contentSize  int
			contentBytes []byte
		)
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT att.filename, COALESCE(att.content_type,'application/octet-stream'),
				   COALESCE(att.content_size,0), att.content
			FROM article_data_mime_attachment att
			INNER JOIN article a ON att.article_id = a.id
			WHERE att.id = ? AND a.ticket_id = ? AND a.is_visible_for_customer = 1
			LIMIT 1`), attachmentID, ticketID)

		if scanErr := row.Scan(&filename, &contentType, &contentSize, &contentBytes); scanErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
			return
		}

		// If content is empty, try local storage
		if len(contentBytes) == 0 {
			if buf, ok := findLocalStoredAttachmentBytes(ticketID, filename); ok {
				contentBytes = buf
				contentSize = len(buf)
				if contentType == "" || contentType == "application/octet-stream" {
					contentType = detectContentType(filename, buf)
				}
			}
		}

		disposition := "attachment"
		if strings.HasPrefix(contentType, "image/") || contentType == "application/pdf" {
			disposition = "inline"
		}
		c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, filename))
		c.Header("Content-Type", contentType)
		c.Header("Content-Length", strconv.Itoa(contentSize))
		c.Data(http.StatusOK, contentType, contentBytes)
	}
}

// handleCustomerGetThumbnail serves thumbnails for image attachments.
func handleCustomerGetThumbnail(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		ticketIDStr := c.Param("id")
		attachmentIDStr := c.Param("attachment_id")

		ticketID, ok := verifyCustomerOwnsTicket(c, db, ticketIDStr, username)
		if !ok {
			return
		}

		attachmentID, err := strconv.Atoi(attachmentIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
			return
		}

		// Get attachment content - ensure it's from a customer-visible article
		var (
			filename     string
			contentType  string
			contentBytes []byte
		)
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT att.filename, COALESCE(att.content_type,'application/octet-stream'), att.content
			FROM article_data_mime_attachment att
			INNER JOIN article a ON att.article_id = a.id
			WHERE att.id = ? AND a.ticket_id = ? AND a.is_visible_for_customer = 1
			LIMIT 1`), attachmentID, ticketID)

		if scanErr := row.Scan(&filename, &contentType, &contentBytes); scanErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
			return
		}

		// If content is empty, try local storage
		if len(contentBytes) == 0 {
			if buf, ok := findLocalStoredAttachmentBytes(ticketID, filename); ok {
				contentBytes = buf
				if contentType == "" || contentType == "application/octet-stream" {
					contentType = detectContentType(filename, buf)
				}
			}
		}

		// Only generate thumbnails for images
		if !strings.HasPrefix(contentType, "image/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Not an image"})
			return
		}

		// Serve the image with cache headers - browser will handle display scaling
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, contentType, contentBytes)
	}
}

// handleCustomerViewAttachment serves an attachment viewer page.
func handleCustomerViewAttachment(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		ticketIDStr := c.Param("id")
		attachmentIDStr := c.Param("attachment_id")

		ticketID, ok := verifyCustomerOwnsTicket(c, db, ticketIDStr, username)
		if !ok {
			return
		}

		attachmentID, err := strconv.Atoi(attachmentIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
			return
		}

		// Get attachment details - ensure it's from a customer-visible article
		var filename, contentType string
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT att.filename, COALESCE(att.content_type,'application/octet-stream')
			FROM article_data_mime_attachment att
			INNER JOIN article a ON att.article_id = a.id
			WHERE att.id = ? AND a.ticket_id = ? AND a.is_visible_for_customer = 1
			LIMIT 1`), attachmentID, ticketID)

		if scanErr := row.Scan(&filename, &contentType); scanErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
			return
		}

		// Render viewer HTML
		downloadURL := fmt.Sprintf("/customer/tickets/%d/attachments/%d", ticketID, attachmentID)
		html := renderAttachmentViewerHTML(filename, contentType, downloadURL)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}
}

// renderCustomerAttachmentListHTML renders attachment list as HTML for HTMX.
// Note: Customers cannot delete attachments, so no delete button is shown.
func renderCustomerAttachmentListHTML(attachments []gin.H, ticketID int) string {
	if len(attachments) == 0 {
		return `<div class="text-center py-4 text-sm" style="color: var(--gk-text-muted);">No attachments found</div>`
	}

	html := `<div class="space-y-2">`
	for _, att := range attachments {
		filename, _ := att["filename"].(string)
		sizeFormatted, _ := att["size_formatted"].(string)
		contentType, _ := att["content_type"].(string)
		downloadURL, _ := att["download_url"].(string)

		// Icon/thumb based on content type
		icon := `<div class="w-10 h-10 rounded flex items-center justify-center" style="background: var(--gk-bg-elevated);">
			<svg class="w-5 h-5" style="color: var(--gk-text-muted);" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"></path>
			</svg>
		</div>`

		if strings.HasPrefix(contentType, "image/") {
			if th, ok := att["thumbnail_url"]; ok {
				if thStr, ok := th.(string); ok {
					icon = fmt.Sprintf(`<img src="%s" alt="thumb" class="w-10 h-10 rounded object-cover" style="border: 1px solid var(--gk-border);"/>`, thStr)
				}
			} else {
				icon = `<div class="w-10 h-10 rounded flex items-center justify-center" style="background: var(--gk-primary-subtle);">
					<svg class="w-5 h-5" style="color: var(--gk-primary);" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2z"></path>
					</svg>
				</div>`
			}
		} else if contentType == "application/pdf" {
			icon = `<div class="w-10 h-10 rounded flex items-center justify-center" style="background: var(--gk-error-subtle);">
				<svg class="w-5 h-5" style="color: var(--gk-error);" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path>
				</svg>
			</div>`
		}

		html += fmt.Sprintf(`
		<div class="flex items-center justify-between p-3 rounded-lg transition-colors"
		     style="background: var(--gk-bg-surface); border: 1px solid var(--gk-border);"
		     onmouseover="this.style.background='var(--gk-bg-elevated)'"
		     onmouseout="this.style.background='var(--gk-bg-surface)'">
			<div class="flex items-center space-x-3">
				%s
				<div>
					<a href="%s/view" target="_blank" class="text-sm font-medium transition-colors"
					   style="color: var(--gk-primary);"
					   onmouseover="this.style.textDecoration='underline'"
					   onmouseout="this.style.textDecoration='none'">
						%s
					</a>
					<p class="text-xs" style="color: var(--gk-text-muted);">%s</p>
				</div>
			</div>
			<div class="flex items-center space-x-2">
				<a href="%s/view" target="_blank" class="p-2 rounded transition-colors"
				   style="color: var(--gk-text-muted);"
				   onmouseover="this.style.color='var(--gk-primary)';this.style.background='var(--gk-primary-subtle)'"
				   onmouseout="this.style.color='var(--gk-text-muted)';this.style.background='transparent'"
				   title="View">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
					</svg>
				</a>
				<a href="%s" class="p-2 rounded transition-colors"
				   style="color: var(--gk-text-muted);"
				   onmouseover="this.style.color='var(--gk-success)';this.style.background='var(--gk-success-subtle)'"
				   onmouseout="this.style.color='var(--gk-text-muted)';this.style.background='transparent'"
				   title="Download" download>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"></path>
					</svg>
				</a>
			</div>
		</div>`, icon, downloadURL, filename, sizeFormatted, downloadURL, downloadURL)
	}
	html += `</div>`

	return html
}

// renderAttachmentViewerHTML renders an HTML page for viewing attachments.
func renderAttachmentViewerHTML(filename, contentType, downloadURL string) string {
	var content string
	if strings.HasPrefix(contentType, "image/") {
		content = fmt.Sprintf(`<img src="%s" alt="%s" style="max-width: 100%%; max-height: 90vh; object-fit: contain;">`, downloadURL, filename)
	} else if contentType == "application/pdf" {
		content = fmt.Sprintf(`<iframe src="%s" style="width: 100%%; height: 90vh; border: none;"></iframe>`, downloadURL)
	} else if strings.HasPrefix(contentType, "text/") {
		content = fmt.Sprintf(`<iframe src="%s" style="width: 100%%; height: 90vh; border: 1px solid var(--gk-border); border-radius: 8px; background: var(--gk-bg-surface);"></iframe>`, downloadURL)
	} else {
		content = fmt.Sprintf(`
			<div style="text-align: center; padding: 40px;">
				<svg style="width: 64px; height: 64px; color: var(--gk-text-muted); margin-bottom: 16px;" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"></path>
				</svg>
				<p style="color: var(--gk-text-secondary); margin-bottom: 16px;">Preview not available for this file type.</p>
				<a href="%s" download class="gk-btn-neon" style="display: inline-flex; padding: 8px 16px;">Download File</a>
			</div>`, downloadURL)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>%s - GoatFlow</title>
	<link rel="stylesheet" href="/static/css/output.css">
	<link rel="stylesheet" href="/static/themes/builtin/synthwave/theme.css">
	<style>
		body {
			margin: 0;
			padding: 20px;
			background: var(--gk-bg-base);
			color: var(--gk-text-primary);
			display: flex;
			flex-direction: column;
			align-items: center;
			min-height: 100vh;
		}
		.viewer-header {
			width: 100%%;
			max-width: 1200px;
			display: flex;
			justify-content: space-between;
			align-items: center;
			margin-bottom: 20px;
			padding: 12px 16px;
			background: var(--gk-bg-surface);
			border-radius: 8px;
			border: 1px solid var(--gk-border);
		}
		.viewer-content {
			flex: 1;
			display: flex;
			align-items: center;
			justify-content: center;
			width: 100%%;
			max-width: 1200px;
		}
	</style>
</head>
<body>
	<div class="viewer-header">
		<span style="font-weight: 500;">%s</span>
		<a href="%s" download class="gk-btn-secondary" style="padding: 6px 12px; font-size: 14px;">
			<svg class="w-4 h-4 inline mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"></path>
			</svg>
			Download
		</a>
	</div>
	<div class="viewer-content">
		%s
	</div>
</body>
</html>`, filename, filename, downloadURL, content)
}
