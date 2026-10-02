package api

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/storage"
)

// Customers see the attachments of the customer-visible articles of their own
// tickets. A single attachment is addressed like on the agent side:
// /customer/tickets/:id/articles/:article_id/attachments/:file_id.

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

// customerAttachment is one entry of the customer attachment list.
type customerAttachment struct {
	ArticleID     int64  `json:"article_id"`
	FileID        int64  `json:"file_id"`
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	Size          int64  `json:"size"`
	SizeFormatted string `json:"size_formatted"`
	UploadedAt    string `json:"uploaded_at"`
	UploadedBy    int    `json:"uploaded_by"`
	DownloadURL   string `json:"download_url"`
	ViewURL       string `json:"view_url"`
	ThumbnailURL  string `json:"thumbnail_url"`
}

// customerTicketBase is the URL prefix of a ticket in the customer portal.
func customerTicketBase(ticketID int) string {
	return fmt.Sprintf("/customer/tickets/%d", ticketID)
}

// handleCustomerGetAttachments returns list of attachments for a customer's ticket.
func handleCustomerGetAttachments(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		ticketID, ok := verifyCustomerOwnsTicket(c, db, c.Param("id"), c.GetString("username"))
		if !ok {
			return
		}

		atts, err := listTicketAttachments(c.Request.Context(), db, ticketID, true)
		if err != nil {
			log.Printf("customer attachments: list ticket %d: %v", ticketID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query attachments"})
			return
		}

		base := customerTicketBase(ticketID)
		result := make([]customerAttachment, 0, len(atts))
		for _, a := range atts {
			url := attachmentURL(base, a.ArticleID, a.FileID)
			result = append(result, customerAttachment{
				ArticleID:     a.ArticleID,
				FileID:        a.FileID,
				Filename:      a.Filename,
				ContentType:   a.ContentType,
				Size:          a.Size,
				SizeFormatted: formatFileSize(a.Size),
				UploadedAt:    a.CreateTime.Format("Jan 2, 2006 3:04 PM"),
				UploadedBy:    a.CreateBy,
				DownloadURL:   url,
				ViewURL:       url + "/view",
				ThumbnailURL:  url + "/thumbnail",
			})
		}

		if c.GetHeader("HX-Request") == "true" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderCustomerAttachmentListHTML(result)))
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"attachments": result,
			"total":       len(result),
		})
	}
}

// handleCustomerUploadAttachment handles file upload for customer tickets.
// Files go to the latest customer-visible article; a ticket without one gets
// a new customer article to carry them.
func handleCustomerUploadAttachment(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		systemUserID := 1 // System user for create_by/change_by

		ticketID, ok := verifyCustomerOwnsTicket(c, db, c.Param("id"), username)
		if !ok {
			return
		}

		if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}
		files := getFormFiles(c.Request.MultipartForm)
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
			return
		}

		ctx := c.Request.Context()
		var articleID int
		err := db.QueryRowContext(ctx, database.ConvertPlaceholders(`
			SELECT id FROM article
			WHERE ticket_id = ? AND is_visible_for_customer = 1
			ORDER BY id DESC LIMIT 1
		`), ticketID).Scan(&articleID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			articleID, err = createCustomerAttachmentArticle(db, ticketID, username, systemUserID)
			if err != nil {
				log.Printf("customer attachments: create article for ticket %d: %v", ticketID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article for attachment"})
				return
			}
		case err != nil:
			log.Printf("customer attachments: find article of ticket %d: %v", ticketID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find article for attachment"})
			return
		}

		processFormAttachments(files, attachmentProcessParams{
			ctx:       ctx,
			db:        db,
			ticketID:  ticketID,
			articleID: articleID,
			userID:    systemUserID,
		})

		c.Header("HX-Trigger", "attachments-updated")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Attachments uploaded successfully",
		})
	}
}

// createCustomerAttachmentArticle creates the customer-visible article that
// carries attachments uploaded to a ticket without one.
func createCustomerAttachmentArticle(db *sql.DB, ticketID int, from string, userID int) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }() //nolint:errcheck // no-op after commit

	now := time.Now()
	articleID, err := database.GetAdapter().InsertWithReturningTx(tx, database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, search_index_needs_rebuild,
			create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, 1, 1, ?, ?, ?, ?) RETURNING id
	`), ticketID, constants.ArticleSenderCustomer, constants.CommunicationChannelEmail, now, userID, now, userID)
	if err != nil {
		return 0, fmt.Errorf("insert article: %w", err)
	}
	if _, err := tx.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (
			article_id, a_from, a_subject, a_body, a_content_type, content_path,
			incoming_time, create_time, create_by, change_time, change_by
		) VALUES (?, ?, 'Attachment', '', 'text/plain', ?, ?, ?, ?, ?, ?)
	`), articleID, from, storage.ContentPath(now), now.Unix(), now, userID, now, userID); err != nil {
		return 0, fmt.Errorf("insert article_data_mime: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(articleID), nil
}

// loadCustomerAttachment resolves the ticket (owned by the customer) and the
// attachment (on one of its customer-visible articles). It writes the error
// response and returns ok=false when either is not accessible.
func loadCustomerAttachment(c *gin.Context, db *sql.DB) (ticketID int, a storage.Attachment, content []byte, ok bool) {
	if !requireCustomerAuth(c) {
		return 0, a, nil, false
	}
	ticketID, ok = verifyCustomerOwnsTicket(c, db, c.Param("id"), c.GetString("username"))
	if !ok {
		return 0, a, nil, false
	}
	articleID, fileID, ok := attachmentRefParams(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment reference"})
		return 0, a, nil, false
	}
	a, content, err := getTicketAttachment(c.Request.Context(), db, ticketID, articleID, fileID, true)
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return 0, a, nil, false
	}
	if err != nil {
		log.Printf("customer attachments: load %d/%d of ticket %d: %v", articleID, fileID, ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load attachment"})
		return 0, a, nil, false
	}
	return ticketID, a, content, true
}

// handleCustomerDownloadAttachment serves the attachment file for customers.
func handleCustomerDownloadAttachment(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, a, content, ok := loadCustomerAttachment(c, db); ok {
			serveAttachment(c, a, content, c.Query("download") == "1")
		}
	}
}

// handleCustomerGetThumbnail serves a PNG preview of an attachment.
func handleCustomerGetThumbnail(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, a, content, ok := loadCustomerAttachment(c, db); ok {
			serveThumbnail(c, a, content)
		}
	}
}

// handleCustomerViewAttachment serves an attachment viewer page; with ?raw=1
// it serves the content for embedding in that page.
func handleCustomerViewAttachment(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID, a, content, ok := loadCustomerAttachment(c, db)
		if !ok {
			return
		}
		if c.Query("raw") == "1" {
			serveAttachmentRaw(c, a, content)
			return
		}
		url := attachmentURL(customerTicketBase(ticketID), a.ArticleID, a.FileID)
		page := renderAttachmentViewerHTML(a.Filename, attachmentContentType(a, content), url)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
	}
}

// renderCustomerAttachmentListHTML renders attachment list as HTML for HTMX.
// Note: Customers cannot delete attachments, so no delete button is shown.
func renderCustomerAttachmentListHTML(attachments []customerAttachment) string {
	if len(attachments) == 0 {
		return `<div class="text-center py-4 text-sm" style="color: var(--gk-text-muted);">No attachments found</div>`
	}

	var b strings.Builder
	b.WriteString(`<div class="space-y-2">`)
	for _, att := range attachments {
		ct := normalizeMimeType(att.ContentType)
		// Icon/thumb based on content type
		icon := `<div class="w-10 h-10 rounded flex items-center justify-center" style="background: var(--gk-bg-elevated);">
			<svg class="w-5 h-5" style="color: var(--gk-text-muted);" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"></path>
			</svg>
		</div>`
		switch {
		case strings.HasPrefix(ct, "image/"):
			icon = fmt.Sprintf(`<img src="%s" alt="thumb" class="w-10 h-10 rounded object-cover" style="border: 1px solid var(--gk-border);"/>`,
				html.EscapeString(att.ThumbnailURL))
		case ct == "application/pdf":
			icon = `<div class="w-10 h-10 rounded flex items-center justify-center" style="background: var(--gk-error-subtle);">
				<svg class="w-5 h-5" style="color: var(--gk-error);" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path>
				</svg>
			</div>`
		}

		fmt.Fprintf(&b, `
		<div class="flex items-center justify-between p-3 rounded-lg transition-colors"
		     style="background: var(--gk-bg-surface); border: 1px solid var(--gk-border);"
		     onmouseover="this.style.background='var(--gk-bg-elevated)'"
		     onmouseout="this.style.background='var(--gk-bg-surface)'">
			<div class="flex items-center space-x-3">
				%s
				<div>
					<a href="%s" target="_blank" class="text-sm font-medium transition-colors"
					   style="color: var(--gk-primary);"
					   onmouseover="this.style.textDecoration='underline'"
					   onmouseout="this.style.textDecoration='none'">
						%s
					</a>
					<p class="text-xs" style="color: var(--gk-text-muted);">%s</p>
				</div>
			</div>
			<div class="flex items-center space-x-2">
				<a href="%s" target="_blank" class="p-2 rounded transition-colors"
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
		</div>`, icon, html.EscapeString(att.ViewURL), html.EscapeString(att.Filename), html.EscapeString(att.SizeFormatted),
			html.EscapeString(att.ViewURL), html.EscapeString(att.DownloadURL+"?download=1"))
	}
	b.WriteString(`</div>`)
	return b.String()
}

// renderAttachmentViewerHTML renders an HTML page for viewing an attachment.
// url is the attachment's download URL; the embedded preview loads its
// framable /view?raw=1 variant.
func renderAttachmentViewerHTML(filename, contentType, url string) string {
	name := html.EscapeString(filename)
	raw := html.EscapeString(url + "/view?raw=1")
	download := html.EscapeString(url + "?download=1")
	ct := normalizeMimeType(contentType)

	var content string
	switch {
	case inlineSafe(ct) && strings.HasPrefix(ct, "image/"):
		content = fmt.Sprintf(`<img src="%s" alt="%s" style="max-width: 100%%; max-height: 90vh; object-fit: contain;">`, raw, name)
	case inlineSafe(ct):
		content = fmt.Sprintf(`<iframe src="%s" style="width: 100%%; height: 90vh; border: none;"></iframe>`, raw)
	case rawAsText(ct):
		content = fmt.Sprintf(`<iframe src="%s" style="width: 100%%; height: 90vh; border: 1px solid var(--gk-border); border-radius: 8px; background: var(--gk-bg-surface);"></iframe>`, raw)
	default:
		content = fmt.Sprintf(`
			<div style="text-align: center; padding: 40px;">
				<svg style="width: 64px; height: 64px; color: var(--gk-text-muted); margin-bottom: 16px;" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"></path>
				</svg>
				<p style="color: var(--gk-text-secondary); margin-bottom: 16px;">Preview not available for this file type.</p>
				<a href="%s" download class="gk-btn-neon" style="display: inline-flex; padding: 8px 16px;">Download File</a>
			</div>`, download)
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
</html>`, name, name, download, content)
}
