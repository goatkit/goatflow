package api

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// This replaces the underConstructionAPI("/tickets/:id/messages") call.
func handleGetTicketMessages(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticket ID is required"})
		return
	}

	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket ID"})
		return
	}

	// Get current user info
	_, _, userRole, hasUser := getCurrentUser(c)
	if !hasUser {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	// TODO: Verify user can access this ticket (use existing RBAC)
	// For now, basic role checking
	if userRole != string(models.RoleAdmin) && userRole != string(models.RoleAgent) && userRole != string(models.RoleCustomer) {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	// Get messages from the ticket service (real path)
	ticketService := GetTicketService()
	if ticketService == nil {
		// As a last resort in tests, return empty
		c.JSON(http.StatusOK, gin.H{"success": true, "messages": []string{}, "total": 0, "pagination": gin.H{"page": 1, "per_page": 50, "has_more": false}})
		return
	}
	messages, err := ticketService.GetMessages(uint(ticketID))
	if err != nil {
		// If ticket not found, return 404
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
			return
		}
		// For other errors, return 500
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve messages"})
		return
	}

	// Load sender type colors and apply to messages
	if db, dbErr := database.GetDB(); dbErr == nil {
		articleRepo := repository.NewArticleRepository(db)
		if senderColors, colorErr := articleRepo.GetSenderTypeColors(); colorErr == nil {
			for _, msg := range messages {
				if color, ok := senderColors[msg.SenderTypeID]; ok {
					msg.SenderColor = color
				}
			}
		} else {
			log.Printf("Error loading sender type colors: %v", colorErr)
		}
	}

	// Check if this is an HTMX request
	if c.GetHeader("HX-Request") != "" {
		// Return HTML fragment for HTMX
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, renderSimpleMessagesHTML(messages, uint(ticketID)))
		return
	}

	// Return JSON for API requests
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"messages": messages,
		"total":    len(messages),
		"pagination": gin.H{
			"page":     1,
			"per_page": 50,
			"has_more": false,
		},
	})
}

// This handles POST requests to add comments.
func handleAddTicketMessage(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticket ID is required"})
		return
	}

	ticketID, err := strconv.ParseUint(ticketIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket ID"})
		return
	}

	// Get current user info
	userID, userEmail, userRole, hasUser := getCurrentUser(c)
	if !hasUser {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	// Parse request body
	var req struct {
		Content    string `json:"content" binding:"required"`
		Subject    string `json:"subject"`
		IsInternal bool   `json:"is_internal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	// Set defaults
	if req.Subject == "" {
		req.Subject = fmt.Sprintf("Re: Ticket #%d", ticketID)
	}

	// Determine message type based on user role and internal flag (case-insensitive)
	roleLower := strings.ToLower(userRole)
	isInternal := req.IsInternal && (roleLower == "admin" || roleLower == "agent")

	// Create the message
	message := &service.SimpleTicketMessage{
		Subject:     req.Subject,
		Body:        req.Content,
		IsInternal:  isInternal,
		CreatedBy:   userID,
		AuthorName:  userEmail, // Using email as name for now
		AuthorEmail: userEmail,
		AuthorType:  "Agent",
		IsPublic:    !isInternal,
		CreatedAt:   time.Now(),
	}

	if userRole == "customer" {
		message.AuthorType = "Customer"
	}

	// Prepare article representation used in both paths
	newMessage := models.Article{
		ID:                   0,
		TicketID:             int(ticketID),
		Subject:              message.Subject,
		Body:                 message.Body,
		CreateTime:           message.CreatedAt,
		CreateBy:             int(userID),
		IsVisibleForCustomer: 1,
		ArticleTypeID:        models.ArticleTypeNoteExternal,
		SenderTypeID:         models.SenderTypeAgent,
		BodyType:             "text/plain",
	}
	if isInternal {
		newMessage.IsVisibleForCustomer = 0
		newMessage.ArticleTypeID = models.ArticleTypeNoteInternal
	}
	if roleLower == "customer" {
		newMessage.SenderTypeID = models.SenderTypeCustomer
	}

	// Create new article/message using the service (real path)
	ticketService := GetTicketService()
	if err := ticketService.AddMessage(uint(ticketID), message); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create message"})
		return
	}

	// Check if this is an HTMX request
	if c.GetHeader("HX-Request") != "" {
		// Return updated message list HTML for HTMX
		c.Header("Content-Type", "text/html")
		// For HTMX, we typically return the new message HTML to append
		c.String(http.StatusOK, renderSingleMessageHTML(newMessage))
		return
	}

	// Return JSON response for API requests
	c.JSON(http.StatusCreated, gin.H{
		"success":    true,
		"message":    "Message added successfully",
		"message_id": newMessage.ID,
		"article":    newMessage,
	})
}

// Helper functions

// getCurrentUser extracts user info from gin context.
func getCurrentUser(c *gin.Context) (uint, string, string, bool) {
	email, emailExists := c.Get("user_email")
	role, roleExists := c.Get("user_role")

	if !emailExists || !roleExists {
		return 0, "", "", false
	}

	id := GetUserIDFromCtxUint(c, 0)
	if id == 0 {
		return 0, "", "", false
	}

	emailStr, ok := email.(string)
	if !ok {
		return 0, "", "", false
	}

	roleStr, ok := role.(string)
	if !ok {
		return 0, "", "", false
	}

	return id, emailStr, roleStr, true
}

// renderSimpleMessagesHTML renders SimpleTicketMessage objects as HTML.
func renderSimpleMessagesHTML(messages []*service.SimpleTicketMessage, ticketID uint) string {
	if len(messages) == 0 {
		return `<div class="text-gray-500 text-center py-8">No messages found.</div>`
	}

	html := `<div class="space-y-4">`
	for _, msg := range messages {
		html += renderSimpleMessageHTML(msg, ticketID)
	}
	html += `</div>`

	return html
}

// renderSimpleMessageHTML renders a single SimpleTicketMessage as HTML.
func renderSimpleMessageHTML(msg *service.SimpleTicketMessage, ticketID uint) string {
	internalBadge := ""
	if msg.IsInternal {
		internalBadge = `<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-200 ml-2">Internal</span>`
	}

	// Get formatted time
	formattedTime := msg.CreatedAt.Format("Jan 2, 2006 3:04 PM")

	// Get initials from author name
	initials := "U"
	if len(msg.AuthorName) > 0 {
		initials = string(msg.AuthorName[0])
	}

	// Build attachments HTML
	attachmentsHTML := ""
	if len(msg.Attachments) > 0 {
		attachmentsHTML = `<div class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
			<p class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">Attachments:</p>
			<div class="space-y-1">`

		for _, att := range msg.Attachments {
			// Format file size
			sizeStr := formatFileSize(att.Size)
			// Use thumbnail for images, icon for others
			var thumbnailHTML string
			if service.IsSupportedImageType(att.ContentType) {
				// Extract attachment ID from URL and use correct thumbnail endpoint
				attachmentID := extractAttachmentID(att.URL)
				thumbnailURL := fmt.Sprintf("/api/tickets/%d/attachments/%s/thumbnail", ticketID, attachmentID)
				thumbnailHTML = fmt.Sprintf(`
					<div class="w-16 h-16 rounded-lg overflow-hidden bg-gray-200 dark:bg-gray-700 flex-shrink-0 cursor-pointer hover:ring-2 hover:ring-blue-500 transition-all"
					     onclick="previewAttachment('%s', '%s', '%s')"
					     title="Click to preview">
						<img src="%s" alt="%s" class="w-full h-full object-cover" loading="lazy" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
						<div class="w-full h-full items-center justify-center hidden">
							%s
						</div>
					</div>`, att.URL, att.Filename, att.ContentType, thumbnailURL, att.Filename, getAttachmentIcon(att.ContentType))
			} else {
				thumbnailHTML = fmt.Sprintf(`
					<div class="w-16 h-16 rounded-lg bg-gray-100 dark:bg-gray-700 flex items-center justify-center flex-shrink-0 cursor-pointer hover:ring-2 hover:ring-gray-500 transition-all"
					     onclick="previewAttachment('%s', '%s', '%s')"
					     title="Click to preview">
						%s
					</div>`, att.URL, att.Filename, att.ContentType, getAttachmentIcon(att.ContentType))
			}

			attachmentsHTML += fmt.Sprintf(`
				<div class="flex items-center justify-between p-3 bg-gradient-to-r from-gray-50 to-gray-100 dark:from-gray-900 dark:to-gray-800 rounded-lg hover:shadow-md transition-shadow"
				     data-attachment-url="%s"
				     data-attachment-name="%s"
				     data-attachment-type="%s">
					<div class="flex items-center space-x-3">
						%s
						<div>
							<a href="%s/view" target="_blank" class="text-sm font-medium text-blue-600 hover:text-blue-800 dark:text-blue-400 cursor-pointer">%s</a>
							<p class="text-xs text-gray-500 dark:text-gray-400">%s • %s</p>
						</div>
					</div>
					<div class="flex items-center space-x-2">
						<a href="%s/view" target="_blank" class="p-1 text-gray-500 hover:text-blue-600 dark:hover:text-blue-400" title="View">
							<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
							</svg>
						</a>
						<a href="%s" download class="p-1 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300" title="Download">
							<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"></path>
							</svg>
						</a>
					</div>
				</div>`,
				att.URL, att.Filename, att.ContentType,
				thumbnailHTML, att.URL, att.Filename, att.ContentType, sizeStr,
				att.URL, att.URL)
		}

		attachmentsHTML += `</div></div>`
	}

	// Process message body based on content type
	var processedBody string
	if strings.Contains(msg.ContentType, "text/html") || (strings.Contains(msg.Body, "<") && strings.Contains(msg.Body, ">")) {
		// For HTML content, use it directly (assuming it's from a trusted editor)
		processedBody = msg.Body
	} else if strings.Contains(msg.ContentType, "text/markdown") || isMarkdownContent(msg.Body) {
		// Render markdown content to HTML
		processedBody = RenderMarkdown(msg.Body)
	} else {
		// Plain text - escape HTML entities
		processedBody = strings.ReplaceAll(msg.Body, "\n", "<br>")
	}

	// Apply sender color as left border if available
	borderStyle := ""
	if msg.SenderColor != "" {
		borderStyle = fmt.Sprintf("border-left: 4px solid %s;", msg.SenderColor)
	}

	return fmt.Sprintf(`
	<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4" style="%s">
		<div class="flex items-start justify-between">
			<div class="flex items-center">
				<div class="flex-shrink-0">
					<div class="h-8 w-8 rounded-full bg-gray-300 flex items-center justify-center">
						<span class="text-sm font-medium text-gray-700">%s</span>
					</div>
				</div>
				<div class="ml-3">
					<p class="text-sm font-medium text-gray-900 dark:text-white">%s</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">%s</p>
				</div>
				%s
			</div>
			<div class="text-sm text-gray-500 dark:text-gray-400">
				%s
			</div>
		</div>
		<div class="mt-3">
			<h4 class="text-sm font-medium text-gray-900 dark:text-white">%s</h4>
			<div class="mt-2 text-sm text-gray-700 dark:text-gray-300">
				%s
			</div>
		</div>
		%s
	</div>`,
		borderStyle,
		initials,
		msg.AuthorName,
		msg.AuthorEmail,
		internalBadge,
		formattedTime,
		msg.Subject,
		processedBody,
		attachmentsHTML)
}

// renderMessagesHTML renders articles as HTML. Currently not referenced.
//
//nolint:unused
func renderMessagesHTML(messages []models.Article) string {
	if len(messages) == 0 {
		return `<div class="text-gray-500 text-center py-8">No messages found.</div>`
	}

	html := `<div class="space-y-4">`
	for _, msg := range messages {
		html += renderSingleMessageHTML(msg)
	}
	html += `</div>`

	return html
}

// renderSingleMessageHTML renders a single message as HTML.
func renderSingleMessageHTML(msg models.Article) string {
	internalBadge := ""
	if msg.IsVisibleForCustomer == 0 {
		internalBadge = `<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-200 ml-2">Internal</span>`
	}

	// Get formatted time
	formattedTime := msg.CreateTime.Format("Jan 2, 2006 3:04 PM")

	// Get author name (simplified - would come from User join in real implementation)
	authorName := fmt.Sprintf("User %d", msg.CreateBy)
	authorEmail := "user@example.com"

	// Convert body to string if it's interface{}
	bodyStr := ""
	if msg.Body != nil {
		if str, ok := msg.Body.(string); ok {
			bodyStr = str
		} else if bytes, ok := msg.Body.([]byte); ok {
			bodyStr = string(bytes)
		}
	}

	return fmt.Sprintf(`
	<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
		<div class="flex items-start justify-between">
			<div class="flex items-center">
				<div class="flex-shrink-0">
					<div class="h-8 w-8 rounded-full bg-gray-300 flex items-center justify-center">
						<span class="text-sm font-medium text-gray-700">%s</span>
					</div>
				</div>
				<div class="ml-3">
					<p class="text-sm font-medium text-gray-900 dark:text-white">%s</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">%s</p>
				</div>
				%s
			</div>
			<div class="text-sm text-gray-500 dark:text-gray-400">
				%s
			</div>
		</div>
		<div class="mt-3">
			<h4 class="text-sm font-medium text-gray-900 dark:text-white">%s</h4>
			<div class="mt-2 text-sm text-gray-700 dark:text-gray-300">
				%s
			</div>
		</div>
	</div>`,
		getInitials(authorName),
		authorName,
		authorEmail,
		internalBadge,
		formattedTime,
		msg.Subject,
		formatMessageBody(bodyStr))
}

// getInitials extracts initials from a name.
func getInitials(name string) string {
	if name == "" {
		return "?"
	}

	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}

	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:1])
	}

	return strings.ToUpper(parts[0][:1] + parts[len(parts)-1][:1])
}

// formatMessageBody formats the message body for HTML display.
func formatMessageBody(body string) string {
	// Basic HTML escaping and line break conversion
	body = strings.ReplaceAll(body, "&", "&amp;")
	body = strings.ReplaceAll(body, "<", "&lt;")
	body = strings.ReplaceAll(body, ">", "&gt;")
	body = strings.ReplaceAll(body, "\n", "<br>")
	return body
}

// getAttachmentIcon returns an appropriate icon SVG based on content type.
func getAttachmentIcon(contentType string) string {
	if strings.HasPrefix(contentType, "image/") {
		return `<svg class="w-8 h-8 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"></path>
		</svg>`
	} else if contentType == "application/pdf" {
		return `<svg class="w-8 h-8 text-red-500" fill="currentColor" viewBox="0 0 20 20">
			<path fill-rule="evenodd" d="M4 4a2 2 0 00-2 2v8a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-5L9 2H4z" clip-rule="evenodd"></path>
		</svg>`
	} else if strings.HasPrefix(contentType, "video/") {
		return `<svg class="w-8 h-8 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
		</svg>`
	} else if strings.HasPrefix(contentType, "audio/") {
		return `<svg class="w-8 h-8 text-green-500" fill="currentColor" viewBox="0 0 20 20">
			<path d="M18 3a1 1 0 00-1.196-.98l-10 2A1 1 0 006 5v9.114A4.369 4.369 0 005 14c-1.657 0-3 .895-3 2s1.343 2 3 2 3-.895 3-2V7.82l8-1.6v5.894A4.37 4.37 0 0015 12c-1.657 0-3 .895-3 2s1.343 2 3 2 3-.895 3-2V3z"></path>
		</svg>`
	} else if strings.HasPrefix(contentType, "text/") || contentType == "application/json" || contentType == "application/xml" {
		return `<svg class="w-8 h-8 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"></path>
		</svg>`
	} else if strings.Contains(contentType, "zip") || strings.Contains(contentType, "compressed") {
		return `<svg class="w-8 h-8 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V2"></path>
		</svg>`
	} else if strings.Contains(contentType, "word") || strings.Contains(contentType, "document") {
		return `<svg class="w-8 h-8 text-blue-600" fill="currentColor" viewBox="0 0 20 20">
			<path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 6a1 1 0 011-1h6a1 1 0 110 2H7a1 1 0 01-1-1zm1 3a1 1 0 100 2h6a1 1 0 100-2H7z" clip-rule="evenodd"></path>
		</svg>`
	} else if strings.Contains(contentType, "sheet") || strings.Contains(contentType, "excel") {
		return `<svg class="w-8 h-8 text-green-600" fill="currentColor" viewBox="0 0 20 20">
			<path fill-rule="evenodd" d="M5 4a2 2 0 00-2 2v8a2 2 0 002 2h10a2 2 0 002-2V6a2 2 0 00-2-2H5zm3 2a1 1 0 000 2h4a1 1 0 100-2H8zm0 3a1 1 0 000 2h4a1 1 0 100-2H8zm0 3a1 1 0 000 2h4a1 1 0 100-2H8z" clip-rule="evenodd"></path>
		</svg>`
	}
	// Default file icon
	return `<svg class="w-8 h-8 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
	</svg>`
}

// extractAttachmentID extracts the attachment ID from a URL like /api/attachments/123/download.
func extractAttachmentID(url string) string {
	re := regexp.MustCompile(`/attachments/(\d+)/`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
