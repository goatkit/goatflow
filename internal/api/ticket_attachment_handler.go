package api

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"image"
	_ "image/gif"
	_ "image/jpeg"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"

	// Register additional decoders for thumbnails.
	"bytes"
	"image/png"
	"log"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// Attachment represents a file attached to a ticket.
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

// Mock storage for attachments (in production, this would be in database/filesystem).
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

// File upload limits.
const (
	MaxFileSize    = 10 * 1024 * 1024 // 10MB per file
	MaxTotalSize   = 50 * 1024 * 1024 // 50MB total per ticket
	MaxAttachments = 20               // Max 20 attachments per ticket
)

// Blocked file extensions.
var blockedExtensions = []string{
	".exe", ".com", ".bat", ".cmd", ".ps1", ".sh", ".vbs", ".js",
	".jar", ".app", ".deb", ".rpm", ".msi", ".dll", ".so",
}

// Allowed MIME types.
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
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"image/jpeg":               true,
	"image/png":                true,
	"image/gif":                true,
	"image/svg+xml":            true,
	"image/webp":               true,
	"image/avif":               true,
	"image/tiff":               true,
	"image/bmp":                true,
	"image/x-icon":             true,
	"image/vnd.microsoft.icon": true,
	"image/heic":               true,
	"image/heif":               true,
	"image/apng":               true,
	// optional modern formats
	"image/jxl":  true,
	"video/mp4":  true,
	"video/webm": true,
	"audio/mpeg": true,
	"audio/wav":  true,
}

// attachmentsDB returns a usable database connection for attachments or nil when mock data should be used.
func attachmentsDB() *sql.DB {
	useDB := true
	if strings.EqualFold(os.Getenv("APP_ENV"), "test") && !database.IsTestDBOverride() {
		switch strings.ToLower(strings.TrimSpace(os.Getenv("ATTACHMENTS_USE_DB"))) {
		case "", "0", "false", "no", "off":
			useDB = false
		}
	}
	if !useDB {
		return nil
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil
	}

	return db
}

// normalizeMimeType maps common aliases and strips parameters; returns lowercased canonical type.
func normalizeMimeType(ct string) string {
	if ct == "" {
		return ct
	}
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	switch ct {
	case "image/jpg", "image/pjpeg":
		return "image/jpeg"
	case "image/x-png", "image/apng":
		return "image/png"
	case "image/svg":
		return "image/svg+xml"
	case "image/x-webp":
		return "image/webp"
	case "image/x-ms-bmp":
		return "image/bmp"
	case "image/vnd.microsoft.icon", "image/ico":
		return "image/x-icon"
	case "image/x-tiff", "image/tif":
		return "image/tiff"
	case "image/heic-sequence":
		return "image/heic"
	case "image/heif-sequence":
		return "image/heif"
	}
	return ct
}

// resolveTicketID resolves the :id path param which can be a numeric TN or a DB ID.
func resolveTicketID(idStr string) (int, error) {
	// Prefer resolving by TN if DB is available (handles numeric TNs)
	if db := attachmentsDB(); db != nil {
		var realID int
		// Try TN first
		row := db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket WHERE tn = $1 LIMIT 1`), idStr)
		if err := row.Scan(&realID); err == nil {
			return realID, nil
		}
		// If not a TN match, allow numeric ID fallback
		if n, convErr := strconv.Atoi(idStr); convErr == nil {
			// Verify the ID exists
			row2 := db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket WHERE id = $1 LIMIT 1`), n)
			if err := row2.Scan(&realID); err == nil {
				return realID, nil
			}
		}
		return 0, fmt.Errorf("ticket not found")
	}
	// No DB available: only numeric IDs are meaningful
	if n, err := strconv.Atoi(idStr); err == nil {
		return n, nil
	}
	return 0, fmt.Errorf("invalid ticket id")
}

// handleUploadAttachment handles file upload to a ticket.
func handleUploadAttachment(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := resolveTicketID(ticketIDStr)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid ticket id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		}
		return
	}

	// Basic check passed by resolver; proceed

	// In DB-less test mode, simulate not-found for unknown numeric IDs (e.g., 99999)
	if os.Getenv("APP_ENV") == "test" {
		if _, ok := attachmentsByTicket[ticketID]; !ok {
			// Known mock tickets listed in attachmentsByTicket; others treated as missing
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
	}

	// Get the file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer func() { _ = file.Close() }()

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

	// Initialize storage service (shared)
	storageService := GetStorageService()
	// Storage path will be computed after we know the target article (OTRS layout)
	storagePath := ""

	// Detect and normalize content type
	// 1) Start with browser-provided header (may include parameters and aliases)
	contentType := normalizeMimeType(header.Header.Get("Content-Type"))
	// 2) If empty or generic, sniff from content
	if contentType == "" || contentType == "application/octet-stream" {
		buf := make([]byte, 512)
		if n, _ := file.Read(buf); n > 0 {
			contentType = detectContentType(header.Filename, buf[:n])
		}
		file.Seek(0, 0)
		// Normalize again in case of aliases from extension-based detection
		contentType = normalizeMimeType(contentType)
	}

	// Enforce config-based limits and allowed types if configured
	if cfg := config.Get(); cfg != nil {
		max := cfg.Storage.Attachments.MaxSize
		if max > 0 && header.Size > max {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("File size exceeds maximum of %dMB", max/(1024*1024)),
			})
			return
		}
		if len(cfg.Storage.Attachments.AllowedTypes) > 0 && contentType != "" && contentType != "application/octet-stream" {
			allowed := map[string]struct{}{}
			for _, t := range cfg.Storage.Attachments.AllowedTypes {
				allowed[strings.ToLower(t)] = struct{}{}
			}
			if _, ok := allowed[strings.ToLower(contentType)]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "File type not allowed"})
				return
			}
		}
	}

	// Determine uploader ID from auth context (default to 1)
	uploaderID := 1
	if v, ok := c.Get("user_id"); ok {
		switch t := v.(type) {
		case int:
			uploaderID = t
		case int64:
			uploaderID = int(t)
		case uint:
			uploaderID = int(t)
		case uint64:
			uploaderID = int(t)
		case string:
			if n, err := strconv.Atoi(t); err == nil {
				uploaderID = n
			}
		}
	}

	// Create attachment record
	attachment := &Attachment{
		ID:          nextAttachmentID,
		TicketID:    ticketID,
		Filename:    header.Filename,
		ContentType: contentType,
		Size:        header.Size,
		StoragePath: storagePath,
		Description: c.PostForm("description"),
		UploadedBy:  uploaderID,
		UploadedAt:  time.Now(),
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

	// If DB is available and storage is DB, use storage abstraction
	if db := attachmentsDB(); db != nil {
		// Find or create an article for this ticket
		articleRepo := repository.NewArticleRepository(db)
		latest, latestErr := articleRepo.GetLatestArticleForTicket(uint(ticketID))
		if latestErr != nil || latest == nil || latest.ID == 0 {
			art := &models.Article{
				TicketID:               int(ticketID),
				ArticleTypeID:          2,
				SenderTypeID:           3,
				CommunicationChannelID: 1,
				IsVisibleForCustomer:   1,
				Subject:                "Attachment",
				Body:                   "",
				CreateBy:               uploaderID,
				ChangeBy:               uploaderID,
			}
			if err := articleRepo.Create(art); err != nil {
				// Surface some context for logs
				fmt.Printf("ERROR: create article for ticket %d failed: %v\n", ticketID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article for attachment"})
				return
			}
			latest = art
		}

		// Use configured storage service; if it's DB, it will insert the attachment row itself.
		if storageService != nil {
			ctx := c.Request.Context()
			// Pass article_id for DB backend
			ctx = context.WithValue(ctx, service.CtxKeyArticleID, latest.ID)
			// Pass user_id for audit fields if backend supports it
			ctx = service.WithUserID(ctx, uploaderID)
			// Compute OTRS-style storage path now that we know article ID
			storagePath = service.GenerateOTRSStoragePath(ticketID, latest.ID, header.Filename)
			// Ensure we start from beginning
			file.Seek(0, 0)
			if _, err := storageService.Store(ctx, file, header, storagePath); err != nil {
				fmt.Printf("ERROR: storage Store failed for ticket %d article %d: %v\n", ticketID, latest.ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store attachment"})
				return
			}

			// If storage backend is local (not DB), also create DB metadata row for listing/download
			if _, isDB := storageService.(*service.DatabaseStorageService); !isDB {
				// Read content bytes for DB row
				file.Seek(0, 0)
				contentBytes, rerr := io.ReadAll(file)
				if rerr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
					return
				}
				// Fallback content type
				ct := contentType
				if ct == "" {
					ct = "application/octet-stream"
				}
				_, ierr := db.Exec(database.ConvertPlaceholders(`
					INSERT INTO article_data_mime_attachment (
						article_id, filename, content_type, content_size, content,
						disposition, create_time, create_by, change_time, change_by
					) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`),
					latest.ID,
					header.Filename,
					ct,
					int64(len(contentBytes)),
					contentBytes,
					"attachment",
					time.Now(), uploaderID, time.Now(), uploaderID,
				)
				if ierr != nil {
					fmt.Printf("ERROR: attachment metadata insert failed for ticket %d article %d: %v\n", ticketID, latest.ID, ierr)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record attachment"})
					return
				}
			}
		}

		// Let client know something changed for HTMX triggers
		c.Header("HX-Trigger", "attachments-updated")
	} else {
		// Fallback in-memory mock storage to keep API usable without DB
		attachments[nextAttachmentID] = attachment
		if attachmentsByTicket[ticketID] == nil {
			attachmentsByTicket[ticketID] = []int{}
		}
		attachmentsByTicket[ticketID] = append(attachmentsByTicket[ticketID], nextAttachmentID)
		nextAttachmentID++
		// Also attempt to store on filesystem for manual inspection
		if storageService != nil {
			// Use OTRS layout even in mock (article id unknown => 0)
			mockPath := service.GenerateOTRSStoragePath(ticketID, 0, header.Filename)
			storageService.Store(c.Request.Context(), file, header, mockPath)
		}
	}

	// Generate response
	response := gin.H{
		"message":       "Attachment uploaded successfully",
		"attachment_id": attachment.ID,
		"filename":      attachment.Filename,
		"size":          attachment.Size,
		"content_type":  attachment.ContentType,
	}

	// Add thumbnail URL for images
	if strings.HasPrefix(attachment.ContentType, "image/") {
		response["thumbnail_url"] = fmt.Sprintf("/api/tickets/%d/attachments/%d/thumbnail", ticketID, attachment.ID)
	}

	// For HTMX, do not replace content; return minimal JSON
	c.JSON(http.StatusCreated, response)
}

// handleGetAttachments returns list of attachments for a ticket.
func handleGetAttachments(c *gin.Context) {
	ticketIDStr := c.Param("id")
	// Resolve TN or ID (handles numeric-looking TNs)
	ticketID, err := resolveTicketID(ticketIDStr)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid ticket id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		}
		return
	}

	// Get database connection
	db := attachmentsDB()
	if db == nil {
		// Fallback to mock attachments when DB unavailable
		list := []gin.H{}
		if ids, ok := attachmentsByTicket[ticketID]; ok {
			for _, id := range ids {
				if att, ok := attachments[id]; ok {
					list = append(list, gin.H{
						"id":             att.ID,
						"filename":       att.Filename,
						"size":           att.Size,
						"size_formatted": formatFileSize(att.Size),
						"content_type":   att.ContentType,
						"uploaded_at":    att.UploadedAt.Format("Jan 2, 2006 3:04 PM"),
						"uploaded_by":    att.UploadedBy,
						"article_id":     0,
						"download_url":   fmt.Sprintf("/api/attachments/%d/download", att.ID),
					})
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"attachments": list, "total": len(list)})
		return
	}

	// Query attachments from database - get all attachments for all articles of this ticket
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT att.id, att.filename,
		       COALESCE(att.content_type, 'application/octet-stream'),
		       COALESCE(att.content_size, 0),
		       att.create_time, att.create_by,
		       att.article_id
		FROM article_data_mime_attachment att
		INNER JOIN article a ON att.article_id = a.id
		WHERE a.ticket_id = $1
		ORDER BY att.id
	`), ticketID)
	if err != nil {
		sendGuruMeditation(c, err, "Failed to query attachments")
		return
	}
	defer func() { _ = rows.Close() }()

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
			// Keep download URL under /api/tickets/:id/attachments/:attachment_id for consistency
			"download_url": fmt.Sprintf("/api/tickets/%s/attachments/%d", ticketIDStr, attID),
		}

		// Add thumbnail URL for images
		if strings.HasPrefix(contentType, "image/") {
			publicAtt["thumbnail_url"] = fmt.Sprintf("/api/tickets/%s/attachments/%d/thumbnail", ticketIDStr, attID)
		}

		result = append(result, publicAtt)
	}

	// Check if this is an HTMX request
	if c.GetHeader("HX-Request") == "true" {
		// Return HTML partial for HTMX
		html := renderAttachmentListHTML(result)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	} else {
		// Return JSON for API requests
		c.JSON(http.StatusOK, gin.H{
			"attachments": result,
			"total":       len(result),
		})
	}
}

// handleDownloadAttachment serves the attachment file.
func handleDownloadAttachment(c *gin.Context) {
	ticketIDStr := c.Param("id")
	attachmentIDStr := c.Param("attachment_id")

	// Resolve ticket ID (supports numeric TN)
	ticketID, err := resolveTicketID(ticketIDStr)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid ticket id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		}
		return
	}

	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// Try DB-backed retrieval first
	if db := attachmentsDB(); db != nil {
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
			WHERE att.id = $1 AND a.ticket_id = $2
			LIMIT 1`), attachmentID, ticketID)
		if scanErr := row.Scan(&filename, &contentType, &contentSize, &contentBytes); scanErr == nil {
			// If DB content is empty (local FS backend), try to fetch from local storage by scanning path
			if len(contentBytes) == 0 {
				if buf, ok := findLocalStoredAttachmentBytes(ticketID, filename); ok {
					contentBytes = buf
					contentSize = len(buf)
					// Improve content type if generic
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
			c.Header("Content-Length", strconv.FormatInt(int64(contentSize), 10))
			c.Data(http.StatusOK, contentType, contentBytes)
			return
		} else {
			// Strict join failed; try to locate by att id -> filename, and ensure ticket matches, then scan local storage
			var fn string
			var dbTicketID int
			row2 := db.QueryRow(database.ConvertPlaceholders(`
				SELECT att.filename, a.ticket_id
				FROM article_data_mime_attachment att
				JOIN article a ON a.id = att.article_id
				WHERE att.id = $1 LIMIT 1`), attachmentID)
			if e2 := row2.Scan(&fn, &dbTicketID); e2 == nil && dbTicketID == ticketID {
				if buf, ok := findLocalStoredAttachmentBytes(ticketID, fn); ok {
					ct := detectContentType(fn, buf)
					disposition := "attachment"
					if strings.HasPrefix(ct, "image/") || ct == "application/pdf" {
						disposition = "inline"
					}
					c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, fn))
					c.Header("Content-Type", ct)
					c.Header("Content-Length", strconv.Itoa(len(buf)))
					c.Data(http.StatusOK, ct, buf)
					return
				}
			}
		}
		// else: fall through to mock storage
	}

	// Fallback to in-memory mock attachment
	attachment, exists := attachments[attachmentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}
	if attachment.TicketID != ticketID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Attachment does not belong to this ticket"})
		return
	}

	// Initialize storage service for mock attachments
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/tmp"
	}
	storageService, err := service.NewLocalStorageService(storagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize storage"})
		return
	}

	// Retrieve file from storage for mock
	fileReader, err := storageService.Retrieve(c.Request.Context(), attachment.StoragePath)
	if err != nil {
		var content []byte
		if attachment.ID == 1 {
			content = []byte("This is a test file content")
		} else if attachment.ID == 2 {
			content = []byte("PNG image content")
		} else {
			content = []byte("File content")
		}
		disposition := "attachment"
		if strings.HasPrefix(attachment.ContentType, "image/") || attachment.ContentType == "application/pdf" {
			disposition = "inline"
		}
		c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, attachment.Filename))
		c.Header("Content-Type", attachment.ContentType)
		c.Header("Content-Length", strconv.FormatInt(int64(len(content)), 10))
		c.Data(http.StatusOK, attachment.ContentType, content)
		return
	}
	defer fileReader.Close()

	disposition := "attachment"
	if strings.HasPrefix(attachment.ContentType, "image/") || attachment.ContentType == "application/pdf" {
		disposition = "inline"
	}
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, attachment.Filename))
	c.Header("Content-Type", attachment.ContentType)
	c.Header("Content-Length", strconv.FormatInt(attachment.Size, 10))
	if _, err := io.Copy(c.Writer, fileReader); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream file"})
		return
	}
}

// handleDeleteAttachment deletes an attachment.
func handleDeleteAttachment(c *gin.Context) {
	ticketIDStr := c.Param("id")
	attachmentIDStr := c.Param("attachment_id")

	// Resolve ticket ID
	ticketID, err := resolveTicketID(ticketIDStr)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid ticket id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		}
		return
	}

	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// If DB available, delete using a guarded join on ticket
	if db := attachmentsDB(); db != nil {
		// Basic permission check: require authenticated user
		if _, ok := c.Get("user_id"); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}
		res, derr := db.Exec(database.ConvertPlaceholders(`
			DELETE FROM article_data_mime_attachment att
			USING article a
			WHERE att.id = $1 AND a.id = att.article_id AND a.ticket_id = $2`), attachmentID, ticketID)
		if derr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete attachment"})
			return
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted successfully"})
		return
	}

	// Fallback to mock in-memory attachments
	attachment, exists := attachments[attachmentID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}
	// Check permissions (best-effort)
	userRole, _ := c.Get("user_role")
	userID, _ := c.Get("user_id")
	if userRole != "admin" {
		if uid, ok := userID.(int); ok && attachment.UploadedBy != uid {
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to delete this attachment"})
			return
		}
	}

	// Initialize storage service
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/tmp"
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

// handleViewAttachment renders common types inline: images, text, markdown, html, pdf.
func handleViewAttachment(c *gin.Context) {
	ticketIDStr := c.Param("id")
	attachmentIDStr := c.Param("attachment_id")
	ticketID, err := resolveTicketID(ticketIDStr)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid ticket id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		}
		return
	}
	attID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// Debug: trace entry
	log.Printf("ATTACH: view start path=%s tn=%s ticketID=%d attID=%d", c.Request.URL.Path, ticketIDStr, ticketID, attID)

	// If raw=1, serve the previous inline/binary behavior for embedding
	if c.Query("raw") == "1" {
		serveAttachmentInlineRaw(c, ticketIDStr, ticketID, attID)
		return
	}

	// Viewer shell: build a minimal HTML wrapper with overlay arrows and ESC-to-exit
	// Determine filename and content type without loading full content
	filename := "attachment"
	contentType := "application/octet-stream"
	// Determine prev/next attachment ids for this ticket
	prevID, nextID := 0, 0
	// Try DB metadata path first
	if db := attachmentsDB(); db != nil {
		// content type + filename
		_ = db.QueryRow(database.ConvertPlaceholders(`
			SELECT att.filename, COALESCE(att.content_type,'')
			FROM article_data_mime_attachment att
			JOIN article a ON a.id = att.article_id
			WHERE att.id = $1 AND a.ticket_id = $2 LIMIT 1`), attID, ticketID).Scan(&filename, &contentType)

		// compute neighbors by ordered id
		rows, qerr := db.Query(database.ConvertPlaceholders(`
			SELECT att.id
			FROM article_data_mime_attachment att
			JOIN article a ON a.id = att.article_id
			WHERE a.ticket_id = $1
			ORDER BY att.id`), ticketID)
		if qerr == nil {
			ids := []int{}
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			// find index
			for i, id := range ids {
				if id == attID {
					if i > 0 {
						prevID = ids[i-1]
					}
					if i < len(ids)-1 {
						nextID = ids[i+1]
					}
					break
				}
			}
		}
	} else {
		// Fallback to mock storage for neighbors and meta
		if att, ok := attachments[attID]; ok {
			filename = att.Filename
			contentType = att.ContentType
		}
		if ids, ok := attachmentsByTicket[ticketID]; ok {
			for i, id := range ids {
				if id == attID {
					if i > 0 {
						prevID = ids[i-1]
					}
					if i < len(ids)-1 {
						nextID = ids[i+1]
					}
					break
				}
			}
		}
	}

	ct := strings.ToLower(contentType)
	// Build the inner content element source using the download endpoint which handles inline types
	contentURL := fmt.Sprintf("/api/tickets/%s/attachments/%d", ticketIDStr, attID)
	// Decide embed tag
	var embed string
	switch {
	case strings.HasPrefix(ct, "image/"):
		// Escape percent signs in fmt format string for CSS percentages
		embed = fmt.Sprintf(`<img src="%s" alt="%s" style="max-width: 100%%; max-height: 100%%; object-fit: contain;" />`, contentURL, htmlEscape(filename))
	case ct == "application/pdf":
		// Use native PDF inline via iframe; PDF.js can be integrated later
		embed = fmt.Sprintf(`<iframe src="%s" style="width:100%%; height:100%%; border:0; background:#1a1a1a;"></iframe>`, contentURL)
	case strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "application/xml":
		// Show in iframe to preserve formatting; could be enhanced to fetch and render in <pre>
		embed = fmt.Sprintf(`<iframe src="%s" style="width:100%%; height:100%%; border:0; background:#111;"></iframe>`, contentURL)
	default:
		// Fallback: try iframe; browser will handle or prompt download
		embed = fmt.Sprintf(`<iframe src="%s" style="width:100%%; height:100%%; border:0; background:#111;"></iframe>`, contentURL)
	}

	// Navigation URLs
	prevURL := ""
	nextURL := ""
	if prevID > 0 {
		prevURL = fmt.Sprintf("/api/tickets/%s/attachments/%d/view", ticketIDStr, prevID)
	}
	if nextID > 0 {
		nextURL = fmt.Sprintf("/api/tickets/%s/attachments/%d/view", ticketIDStr, nextID)
	}

	// Build HTML page
	html := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s · Attachment Viewer</title>
  <style>
	html, body { height: 100%%; margin: 0; background: #0b0b0b; color: #e5e5e5; }
	.viewer { position: fixed; inset: 0; display: grid; place-items: center; }
	.nav { position: fixed; top: 0; bottom: 0; width: 15%%; max-width: 180px; display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity .25s ease; pointer-events: none; }
	.nav.visible { opacity: .95; pointer-events: auto; }
	.nav.left { left: 0; background: linear-gradient(90deg, rgba(0,0,0,.35), rgba(0,0,0,0)); }
	.nav.right { right: 0; background: linear-gradient(270deg, rgba(0,0,0,.35), rgba(0,0,0,0)); }
	.btn { font: 700 28px/1 system-ui, -apple-system, Segoe UI, Roboto, sans-serif; color: #fff; text-decoration: none; padding: 12px 16px; border-radius: 8px; background: rgba(0,0,0,.35); border: 1px solid rgba(255,255,255,.2); }
	.meta { position: fixed; left: 12px; top: 12px; font: 500 12px/1.4 system-ui, -apple-system, Segoe UI, Roboto, sans-serif; color: #cfcfcf; opacity: .85; background: rgba(0,0,0,.35); padding: 6px 10px; border-radius: 6px; border: 1px solid rgba(255,255,255,.15); }
	.content { width: 100%%; height: 100%%; display: grid; place-items: center; }
  </style>
</head>
<body>
  <div class="viewer" id="viewer">
	<div class="meta">%s · %s</div>
	<div class="content">%s</div>
	%s
	%s
  </div>
  <script>
  (function(){
	const left = document.createElement('div');
	const right = document.createElement('div');
	%s
	%s
	const viewer = document.getElementById('viewer');
	let hideTimer = null;
	function showNav(){
	  if (left) left.classList.add('visible');
	  if (right) right.classList.add('visible');
	  if (hideTimer) clearTimeout(hideTimer);
	  hideTimer = setTimeout(()=>{
		if (left) left.classList.remove('visible');
		if (right) right.classList.remove('visible');
	  }, 1000);
	}
	window.addEventListener('mousemove', showNav, {passive:true});
		window.addEventListener('keydown', function(e){
			// Ignore hotkeys when user is typing in an input, textarea, or contenteditable editor
			var ae = document.activeElement;
			if (ae && (ae.isContentEditable || ae.tagName === 'INPUT' || ae.tagName === 'TEXTAREA')) { return; }
			if (e.key === 'Escape') {
		window.location.href = '/tickets/%s';
	  } else if (e.key === 'ArrowLeft' && %t) {
		window.location.href = '%s';
	  } else if (e.key === 'ArrowRight' && %t) {
		window.location.href = '%s';
	  }
	});
	// Initialize once on load
	showNav();
  })();
  </script>
</body>
</html>`,
		htmlEscape(filename),
		htmlEscape(filename), htmlEscape(contentType),
		embed,
		// left and right overlays
		func() string {
			if prevURL == "" {
				return ""
			}
			return fmt.Sprintf(`<div class="nav left"><a class="btn" href="%s" aria-label="Previous">&#x2039;</a></div>`, prevURL)
		}(),
		func() string {
			if nextURL == "" {
				return ""
			}
			return fmt.Sprintf(`<div class="nav right"><a class="btn" href="%s" aria-label="Next">&#x203A;</a></div>`, nextURL)
		}(),
		// create elements so we can toggle classes even if absent
		func() string {
			if prevURL == "" {
				return "/* no prev */"
			}
			return "const leftEl = document.querySelector('.nav.left'); if (leftEl) left = leftEl;"
		}(),
		func() string {
			if nextURL == "" {
				return "/* no next */"
			}
			return "const rightEl = document.querySelector('.nav.right'); if (rightEl) right = rightEl;"
		}(),
		// go-back target uses original id param for nicer TN links
		ticketIDStr,
		prevURL != "", prevURL,
		nextURL != "", nextURL,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.String(200, html)
	return
}

// serveAttachmentInlineRaw serves the original inline content for /view?raw=1 and internal use.
func serveAttachmentInlineRaw(c *gin.Context, ticketIDStr string, ticketID int, attID int) {
	// DB path first
	if db := attachmentsDB(); db != nil {
		var filename, contentType string
		var content []byte
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT att.filename, COALESCE(att.content_type,''), att.content
			FROM article_data_mime_attachment att
			JOIN article a ON a.id = att.article_id
			WHERE att.id = $1 AND a.ticket_id = $2 LIMIT 1`), attID, ticketID)
		if err := row.Scan(&filename, &contentType, &content); err == nil {
			if len(content) == 0 {
				if ss := GetStorageService(); ss != nil {
					// Attempt OTRS-style path (article id unknown here -> 0)
					sp := service.GenerateOTRSStoragePath(ticketID, 0, filename)
					if rc, rerr := ss.Retrieve(c.Request.Context(), sp); rerr == nil {
						defer rc.Close()
						if buf, berr := io.ReadAll(rc); berr == nil {
							content = buf
						}
					}
				}
				if len(content) == 0 {
					if buf, ok := findLocalStoredAttachmentBytes(ticketID, filename); ok {
						content = buf
					}
				}
			}
			ct := strings.ToLower(contentType)
			if ct == "" || ct == "application/octet-stream" {
				detected := detectContentType(filename, content)
				if detected != "" {
					contentType = detected
					ct = strings.ToLower(detected)
				}
			}
			disposition := "inline"
			if !(strings.HasPrefix(ct, "image/") || ct == "application/pdf" || strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "application/xml" || strings.Contains(ct, "html")) {
				disposition = "attachment"
			}
			c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, filename))
			c.Header("Content-Type", contentType)
			c.Header("Cache-Control", "no-store")
			c.Header("Content-Length", strconv.Itoa(len(content)))
			c.Data(200, contentType, content)
			return
		}
	}
	// Fallback to mock/local storage path
	if att, ok := attachments[attID]; ok {
		ct := strings.ToLower(att.ContentType)
		disposition := "inline"
		if !(strings.HasPrefix(ct, "image/") || ct == "application/pdf" || strings.HasPrefix(ct, "text/")) {
			disposition = "attachment"
		}
		c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, att.Filename))
		c.Header("Content-Type", att.ContentType)
		c.Header("Cache-Control", "no-store")
		// Attempt to stream from local storage if present
		storagePath := os.Getenv("STORAGE_PATH")
		if storagePath == "" {
			storagePath = "/tmp"
		}
		if storageService, err := service.NewLocalStorageService(storagePath); err == nil {
			if rc, err := storageService.Retrieve(c.Request.Context(), att.StoragePath); err == nil {
				defer rc.Close()
				io.Copy(c.Writer, rc)
				return
			}
		}
		// Fallback small content
		c.String(200, "Attachment")
		return
	}
	c.JSON(404, gin.H{"error": "Attachment not viewable"})

	// DB path first
	if db := attachmentsDB(); db != nil {
		var filename, contentType string
		var content []byte
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT att.filename, COALESCE(att.content_type,''), att.content
			FROM article_data_mime_attachment att
			JOIN article a ON a.id = att.article_id
			WHERE att.id = $1 AND a.ticket_id = $2 LIMIT 1`), attID, ticketID)
		if err := row.Scan(&filename, &contentType, &content); err == nil {
			// Fallback: if DB content is empty (e.g., local FS backend), try retrieving from storage
			if len(content) == 0 {
				// First try storage service with a best-effort lookup using the exact stored path (unknown here)
				// Since GenerateStoragePath uses current time, it likely won't match; so additionally search on disk
				if ss := GetStorageService(); ss != nil {
					// Attempt naive retrieve with generated path (may fail)
					sp := service.GenerateOTRSStoragePath(ticketID, 0, filename)
					if rc, rerr := ss.Retrieve(c.Request.Context(), sp); rerr == nil {
						defer rc.Close()
						if buf, berr := io.ReadAll(rc); berr == nil {
							content = buf
						}
					}
				}
				// If still empty, try to find the stored file under storage/tickets/<ticketID>/**
				if len(content) == 0 {
					if buf, ok := findLocalStoredAttachmentBytes(ticketID, filename); ok {
						content = buf
					}
				}
			}
			// Fallback detect content type when missing or generic
			ct := strings.ToLower(contentType)
			if ct == "" || ct == "application/octet-stream" {
				detected := detectContentType(filename, content)
				if detected != "" {
					contentType = detected
					ct = strings.ToLower(detected)
				}
			}
			switch {
			case strings.HasPrefix(ct, "image/"):
				c.Header("Content-Type", contentType)
				c.Header("Cache-Control", "no-store")
				c.Header("X-Content-Type-Options", "nosniff")
				c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
				c.Header("Content-Length", strconv.Itoa(len(content)))
				c.Data(200, contentType, content)
				log.Printf("ATTACH: view served inline image type=%s bytes=%d attID=%d ticketID=%d", contentType, len(content), attID, ticketID)
				return
			case ct == "application/pdf":
				c.Header("Content-Type", "application/pdf")
				c.Header("Cache-Control", "no-store")
				c.Header("X-Content-Type-Options", "nosniff")
				c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
				c.Header("Content-Length", strconv.Itoa(len(content)))
				c.Data(200, "application/pdf", content)
				log.Printf("ATTACH: view served inline pdf bytes=%d attID=%d ticketID=%d", len(content), attID, ticketID)
				return
			case strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "application/xml":
				// Show plain text in a simple pre block
				c.Header("Content-Type", "text/plain; charset=utf-8")
				c.Header("Cache-Control", "no-store")
				c.Header("Content-Length", strconv.Itoa(len(content)))
				c.Data(200, "text/plain; charset=utf-8", content)
				log.Printf("ATTACH: view served text bytes=%d attID=%d ticketID=%d", len(content), attID, ticketID)
				return
			default:
				// Try markdown by extension
				ext := strings.ToLower(filepath.Ext(filename))
				if ext == ".md" || ext == ".markdown" {
					// Minimal markdown viewer (no external deps): wrap in <pre> for now
					c.Header("Content-Type", "text/plain; charset=utf-8")
					c.Header("Cache-Control", "no-store")
					c.Data(200, "text/plain; charset=utf-8", content)
					log.Printf("ATTACH: view served markdown-as-text bytes=%d attID=%d ticketID=%d", len(content), attID, ticketID)
					return
				}
				if ext == ".html" || ext == ".htm" || strings.Contains(ct, "html") {
					// Very conservative HTML sanitize: strip script tags
					safe := strings.ReplaceAll(string(content), "<script", "&lt;script")
					c.Header("Content-Type", "text/html; charset=utf-8")
					c.Header("Cache-Control", "no-store")
					c.String(200, safe)
					log.Printf("ATTACH: view served html bytes=%d attID=%d ticketID=%d", len(content), attID, ticketID)
					return
				}
			}
		} else {
			// If the strict join query failed, try to at least get the filename by attachment id
			var fn string
			var dbTicketID int
			row2 := db.QueryRow(database.ConvertPlaceholders(`
				SELECT att.filename, a.ticket_id
				FROM article_data_mime_attachment att
				JOIN article a ON a.id = att.article_id
				WHERE att.id = $1 LIMIT 1`), attID)
			if e2 := row2.Scan(&fn, &dbTicketID); e2 == nil && dbTicketID == ticketID {
				// Try to locate and serve from local storage
				if buf, ok := findLocalStoredAttachmentBytes(ticketID, fn); ok {
					ct := detectContentType(fn, buf)
					if strings.HasPrefix(ct, "image/") || ct == "application/pdf" || strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "application/xml" || strings.Contains(ct, "html") || strings.HasSuffix(strings.ToLower(fn), ".md") || strings.HasSuffix(strings.ToLower(fn), ".markdown") {
						// Normalize content type and respond inline
						switch {
						case strings.HasPrefix(ct, "image/"):
							c.Header("Content-Type", ct)
							c.Header("Cache-Control", "no-store")
							c.Header("X-Content-Type-Options", "nosniff")
							c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fn))
							c.Header("Content-Length", strconv.Itoa(len(buf)))
							c.Data(200, ct, buf)
							log.Printf("ATTACH: view served (fallback image/pdf) type=%s bytes=%d attID=%d ticketID=%d", ct, len(buf), attID, ticketID)
							return
						case ct == "application/pdf":
							c.Header("Content-Type", ct)
							c.Header("Cache-Control", "no-store")
							c.Header("X-Content-Type-Options", "nosniff")
							c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fn))
							c.Header("Content-Length", strconv.Itoa(len(buf)))
							c.Data(200, ct, buf)
							log.Printf("ATTACH: view served (fallback pdf) bytes=%d attID=%d ticketID=%d", len(buf), attID, ticketID)
							return
						case strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "application/xml":
							c.Header("Content-Type", "text/plain; charset=utf-8")
							c.Header("Cache-Control", "no-store")
							c.Header("Content-Length", strconv.Itoa(len(buf)))
							c.Data(200, "text/plain; charset=utf-8", buf)
							log.Printf("ATTACH: view served (fallback text) bytes=%d attID=%d ticketID=%d", len(buf), attID, ticketID)
							return
						default:
							// markdown or html by extension/content
							ext := strings.ToLower(filepath.Ext(fn))
							if ext == ".md" || ext == ".markdown" {
								c.Header("Content-Type", "text/plain; charset=utf-8")
								c.Header("Cache-Control", "no-store")
								c.Data(200, "text/plain; charset=utf-8", buf)
								log.Printf("ATTACH: view served (fallback md as text) bytes=%d attID=%d ticketID=%d", len(buf), attID, ticketID)
								return
							}
							if ext == ".html" || ext == ".htm" || strings.Contains(ct, "html") {
								safe := strings.ReplaceAll(string(buf), "<script", "&lt;script")
								c.Header("Content-Type", "text/html; charset=utf-8")
								c.Header("Cache-Control", "no-store")
								c.String(200, safe)
								log.Printf("ATTACH: view served (fallback html) bytes=%d attID=%d ticketID=%d", len(buf), attID, ticketID)
								return
							}
						}
						// If we found bytes but can't confidently render inline, fall through to 404 below
					}
				}
			}
		}
	}

	// Fallback to mock/local storage path
	if att, ok := attachments[attID]; ok {
		ct := strings.ToLower(att.ContentType)
		if strings.HasPrefix(ct, "image/") || ct == "application/pdf" || strings.HasPrefix(ct, "text/") {
			// Read from local storage service
			storagePath := os.Getenv("STORAGE_PATH")
			if storagePath == "" {
				storagePath = "/tmp"
			}
			storageService, err := service.NewLocalStorageService(storagePath)
			if err == nil {
				rc, err := storageService.Retrieve(c.Request.Context(), att.StoragePath)
				if err == nil {
					defer rc.Close()
					buf, _ := io.ReadAll(rc)
					c.Header("Content-Type", att.ContentType)
					c.Data(200, att.ContentType, buf)
					return
				}
			}
		}
	}
	log.Printf("ATTACH: view not viewable attID=%d ticketID=%d (returning 404 JSON)", attID, ticketID)
	c.JSON(404, gin.H{"error": "Attachment not viewable"})
}

// findLocalStoredAttachmentBytes tries to locate a file stored by LocalStorageService for a given ticket and filename
// when the exact generated storage path is unknown (due to timestamp uniqueness). It walks under
// <storage_base>/tickets/<ticketID>/** and returns the first file that matches the sanitized base + _<digits> + ext pattern.
func findLocalStoredAttachmentBytes(ticketID int, filename string) ([]byte, bool) {
	// Resolve storage base path similar to service initialization
	base := os.Getenv("STORAGE_PATH")
	if base == "" {
		if cfg := config.Get(); cfg != nil && cfg.Storage.Local.Path != "" {
			base = cfg.Storage.Local.Path
		} else {
			base = "./storage"
		}
	}

	// OTRS-style layout under var/article/YYYY/MM/DD/<ticketID>/**/<filename>
	{
		root := filepath.Join(base, "var", "article")
		if fi, err := os.Stat(root); err == nil && fi.IsDir() {
			safeFile := sanitizeFilenameForMatch(filename)
			ticketSeg := string(os.PathSeparator) + strconv.Itoa(ticketID) + string(os.PathSeparator)
			var found []byte
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				// quick filter by filename
				if strings.EqualFold(d.Name(), safeFile) || strings.EqualFold(d.Name(), filepath.Base(filename)) {
					// ensure the ticketID appears as a path segment
					if strings.Contains(path, ticketSeg) {
						if b, rerr := os.ReadFile(path); rerr == nil {
							found = b
							return io.EOF
						}
					}
				}
				return nil
			})
			if found != nil {
				return found, true
			}
		}
	}

	return nil, false
}

// Minimal copy of service.sanitizeFilename for matching purposes.
func sanitizeFilenameForMatch(name string) string {
	repl := strings.NewReplacer(
		" ", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
		"<", "_",
		">", "_",
		":", "_",
		";", "_",
		",", "_",
		"?", "_",
		"*", "_",
		"|", "_",
		"\\", "_",
		"/", "_",
		"\"", "_",
		"'", "_",
	)
	safe := repl.Replace(name)
	if safe == "" {
		safe = "unnamed_file"
	}
	if len(safe) > 255 {
		safe = safe[:255]
	}
	return safe
}

// detectContentType attempts to detect the content type from filename and content.
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
	case ".avif":
		return "image/avif"
	case ".tif", ".tiff":
		return "image/tiff"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	case ".jxl":
		return "image/jxl"
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

// htmlEscape safely escapes text for embedding in HTML context.
func htmlEscape(s string) string { return html.EscapeString(s) }

// handleGetThumbnail serves a thumbnail version of an image attachment.
func handleGetThumbnail(c *gin.Context) {
	ticketIDStr := c.Param("id")
	attachmentIDStr := c.Param("attachment_id")
	ticketID, err := resolveTicketID(ticketIDStr)
	if err != nil {
		c.JSON(404, gin.H{"error": "Ticket not found"})
		return
	}
	attID, err := strconv.Atoi(attachmentIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid attachment id"})
		return
	}

	// Try DB first: fetch bytes and content type
	if db := attachmentsDB(); db != nil {
		var (
			content     []byte
			contentType string
			filename    string
		)
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT att.content, COALESCE(att.content_type,''), att.filename
			FROM article_data_mime_attachment att
			JOIN article a ON a.id = att.article_id
			WHERE att.id = $1 AND a.ticket_id = $2 LIMIT 1`), attID, ticketID)
		if err := row.Scan(&content, &contentType, &filename); err == nil {
			// If DB content is empty (e.g., local FS backend), try to fetch from storage/local disk
			if len(content) == 0 {
				if ss := GetStorageService(); ss != nil {
					sp := service.GenerateOTRSStoragePath(ticketID, 0, filename)
					if rc, rerr := ss.Retrieve(c.Request.Context(), sp); rerr == nil {
						defer rc.Close()
						if buf, berr := io.ReadAll(rc); berr == nil {
							content = buf
						}
					}
				}
				if len(content) == 0 {
					if buf, ok := findLocalStoredAttachmentBytes(ticketID, filename); ok {
						content = buf
					}
				}
			}
			// Only generate thumbnail for images; detect if DB type missing
			ct := strings.ToLower(contentType)
			if ct == "" || ct == "application/octet-stream" {
				contentType = detectContentType(filename, content)
				ct = strings.ToLower(contentType)
			}
			if !strings.HasPrefix(ct, "image/") {
				c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/api/tickets/%s/attachments/%d", ticketIDStr, attID))
				return
			}
			// Cache key path in local fs cache under ./storage/thumbs/<ticketID>/<attID>.png
			cacheDir := filepath.Join("./storage", "thumbs", strconv.Itoa(ticketID))
			os.MkdirAll(cacheDir, 0755)
			cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.png", attID))
			if fi, err := os.Stat(cachePath); err == nil && fi.Size() > 0 {
				// Serve cached
				f, _ := os.Open(cachePath)
				defer f.Close()
				c.Header("Content-Type", "image/png")
				io.Copy(c.Writer, f)
				return
			}
			// Decode
			img, _, err := image.Decode(bytes.NewReader(content))
			if err != nil {
				// If we cannot decode (e.g., SVG/AVIF/HEIC), return a placeholder thumbnail instead
				ph, phType := service.GetPlaceholderThumbnail(contentType)
				c.Header("Cache-Control", "public, max-age=86400")
				c.Data(http.StatusOK, phType, ph)
				return
			}
			// Simple nearest-neighbor resize to max 320x240 preserving aspect
			w := img.Bounds().Dx()
			h := img.Bounds().Dy()
			maxW, maxH := 320, 240
			scale := 1.0
			if w > maxW || h > maxH {
				sx := float64(maxW) / float64(w)
				sy := float64(maxH) / float64(h)
				if sx < sy {
					scale = sx
				} else {
					scale = sy
				}
			}
			tw := int(float64(w) * scale)
			th := int(float64(h) * scale)
			if tw < 1 {
				tw = 1
			}
			if th < 1 {
				th = 1
			}
			// Manual resize (box filter)
			dst := image.NewRGBA(image.Rect(0, 0, tw, th))
			for y := 0; y < th; y++ {
				for x := 0; x < tw; x++ {
					sx := int(float64(x) / float64(tw) * float64(w))
					sy := int(float64(y) / float64(th) * float64(h))
					dst.Set(x, y, img.At(sx, sy))
				}
			}
			// Encode PNG to cache and serve
			f, err := os.Create(cachePath)
			if err == nil {
				png.Encode(f, dst)
				f.Close()
			}
			c.Header("Content-Type", "image/png")
			png.Encode(c.Writer, dst)
			return
		}
	}

	// Fallback: no DB or retrieval failed — attempt local storage mock
	if att, ok := attachments[attID]; ok {
		if !strings.HasPrefix(strings.ToLower(att.ContentType), "image/") {
			c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/api/tickets/%s/attachments/%d", ticketIDStr, attID))
			return
		}
		// Use storage service to retrieve original then thumbnail it
		storagePath := os.Getenv("STORAGE_PATH")
		if storagePath == "" {
			storagePath = "/tmp"
		}
		storageService, err := service.NewLocalStorageService(storagePath)
		if err != nil {
			c.JSON(500, gin.H{"error": "init storage failed"})
			return
		}
		rc, err := storageService.Retrieve(c.Request.Context(), att.StoragePath)
		if err != nil {
			// Serve placeholder if we cannot read original
			ph, phType := service.GetPlaceholderThumbnail(att.ContentType)
			c.Header("Cache-Control", "public, max-age=86400")
			c.Data(http.StatusOK, phType, ph)
			return
		}
		defer rc.Close()
		buf, _ := io.ReadAll(rc)
		img, _, err := image.Decode(bytes.NewReader(buf))
		if err != nil {
			// Use placeholder for undecodable formats
			ph, phType := service.GetPlaceholderThumbnail(att.ContentType)
			c.Header("Cache-Control", "public, max-age=86400")
			c.Data(http.StatusOK, phType, ph)
			return
		}
		w := img.Bounds().Dx()
		h := img.Bounds().Dy()
		maxW, maxH := 320, 240
		scale := 1.0
		if w > maxW || h > maxH {
			sx := float64(maxW) / float64(w)
			sy := float64(maxH) / float64(h)
			if sx < sy {
				scale = sx
			} else {
				scale = sy
			}
		}
		tw := int(float64(w) * scale)
		th := int(float64(h) * scale)
		if tw < 1 {
			tw = 1
		}
		if th < 1 {
			th = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, tw, th))
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				sx := int(float64(x) / float64(tw) * float64(w))
				sy := int(float64(y) / float64(th) * float64(h))
				dst.Set(x, y, img.At(sx, sy))
			}
		}
		c.Header("Content-Type", "image/png")
		png.Encode(c.Writer, dst)
		return
	}

	c.JSON(404, gin.H{"error": "Attachment not found"})
}

// validateFile validates uploaded file.
func validateFile(header *multipart.FileHeader) error {
	filename := header.Filename

	// Check for hidden files
	if strings.HasPrefix(filename, ".") {
		return fmt.Errorf("hidden files are not allowed")
	}

	// Check extension
	ext := strings.ToLower(filepath.Ext(filename))
	for _, blocked := range blockedExtensions {
		if ext == blocked {
			return fmt.Errorf("File type not allowed")
		}
	}

	// Check MIME type (be lenient with browser-provided values)
	// - Normalize & strip parameters; allow octet-stream to pass (sniff later)
	if raw := header.Header.Get("Content-Type"); raw != "" {
		ct := normalizeMimeType(raw)
		if ct != "" && ct != "application/octet-stream" {
			if !allowedMimeTypes[ct] {
				return fmt.Errorf("File type not allowed")
			}
		}
	}

	return nil
}

// ValidateUploadedFile is a small exported wrapper around validateFile to enable
// focused unit tests from an external test package without importing all api tests.
func ValidateUploadedFile(header *multipart.FileHeader) error {
	return validateFile(header)
}

// renderAttachmentListHTML renders attachment list as HTML for HTMX.
func renderAttachmentListHTML(attachments []gin.H) string {
	if len(attachments) == 0 {
		return `<div class="text-center py-4 text-sm text-gray-500 dark:text-gray-400">No attachments found</div>`
	}

	html := `<div class="space-y-2 p-4">`
	for _, att := range attachments {
		attID := att["id"]
		filename := att["filename"].(string)
		sizeFormatted := att["size_formatted"].(string)
		contentType := att["content_type"].(string)
		downloadURL := att["download_url"].(string)

		// Icon/thumb based on content type
		icon := `<svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"></path>
		</svg>`

		if strings.HasPrefix(contentType, "image/") {
			// Use thumbnail if available
			if th, ok := att["thumbnail_url"]; ok {
				icon = fmt.Sprintf(`<img src="%s" alt="thumb" class="w-10 h-10 rounded object-cover ring-1 ring-gray-200 dark:ring-gray-700"/>`, th.(string))
			} else {
				icon = `<svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2z"></path>
				</svg>`
			}
		} else if contentType == "application/pdf" {
			icon = `<svg class="w-5 h-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path>
			</svg>`
		} else if strings.HasPrefix(contentType, "text/") {
			icon = `<svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
			</svg>`
		}

		html += fmt.Sprintf(`
		<div class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors">
			<div class="flex items-center space-x-3">
				%s
				<div>
					<a href="%s" class="text-sm font-medium text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300" download>
						%s
					</a>
					<p class="text-xs text-gray-500 dark:text-gray-400">%s</p>
				</div>
			</div>
			<div class="flex items-center space-x-2">
				<a href="%s/view" target="_blank" class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" title="View">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14m-6 0l-4.553 2.276A1 1 0 013 15.382V8.618a1 1 0 011.447-.894L9 10m6 0l-6 4m6-4l-6-4" />
					</svg>
				</a>
				<a href="%s" class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" title="Download">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"></path>
					</svg>
				</a>
				<button onclick="deleteAttachment(%v, '%s')" class="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400" title="Delete">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
					</svg>
				</button>
			</div>
		</div>`,
			icon, downloadURL, filename, sizeFormatted, downloadURL, downloadURL, attID, filename)
	}
	html += `</div>`

	return html
}
