package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/storage"
)

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
	"text/calendar":            true,
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

// errTicketNotFound is returned by resolveTicketID when no ticket matches.
var errTicketNotFound = errors.New("ticket not found")

// resolveTicketID resolves a :id path param, which is a ticket number (TN,
// tried first because TNs are numeric too) or a ticket id.
func resolveTicketID(ctx context.Context, db *sql.DB, idStr string) (int, error) {
	var id int
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT id FROM ticket WHERE tn = ?"), idStr).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("resolve ticket %q: %w", idStr, err)
	}
	n, convErr := strconv.Atoi(idStr)
	if convErr != nil || n <= 0 {
		return 0, errTicketNotFound
	}
	err = db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT id FROM ticket WHERE id = ?"), n).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errTicketNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("resolve ticket %q: %w", idStr, err)
	}
	return id, nil
}

// attachmentServerError logs err and answers a generic 500.
func attachmentServerError(c *gin.Context, err error, message string) {
	log.Printf("attachments: %s: %v", message, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": message})
}

// attachmentTicket returns the database and the ticket of the :id param, or
// writes the error response and returns ok=false.
func attachmentTicket(c *gin.Context) (db *sql.DB, ticketID int, ok bool) {
	db, err := database.GetDB()
	if err != nil {
		attachmentServerError(c, err, "Database unavailable")
		return nil, 0, false
	}
	ticketID, err = resolveTicketID(c.Request.Context(), db, c.Param("id"))
	if errors.Is(err, errTicketNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return nil, 0, false
	}
	if err != nil {
		attachmentServerError(c, err, "Failed to load ticket")
		return nil, 0, false
	}
	return db, ticketID, true
}

// attachmentRef resolves :id, :article_id and :file_id and loads the
// attachment, or writes the error response and returns ok=false.
func attachmentRef(c *gin.Context) (db *sql.DB, ticketID int, att storage.Attachment, content []byte, ok bool) {
	db, ticketID, ok = attachmentTicket(c)
	if !ok {
		return nil, 0, storage.Attachment{}, nil, false
	}
	articleID, fileID, ok := attachmentRefParams(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return nil, 0, storage.Attachment{}, nil, false
	}
	att, content, err := getTicketAttachment(c.Request.Context(), db, ticketID, articleID, fileID, false)
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return nil, 0, storage.Attachment{}, nil, false
	}
	if err != nil {
		attachmentServerError(c, err, "Failed to load attachment")
		return nil, 0, storage.Attachment{}, nil, false
	}
	return db, ticketID, att, content, true
}

// agentTicketAttachmentBase is the URL prefix of the agent attachment routes
// of the ticket addressed by the request (TN or id, as given).
func agentTicketAttachmentBase(c *gin.Context) string {
	return "/api/tickets/" + url.PathEscape(c.Param("id"))
}

// hasThumbnail reports whether serveThumbnail renders a real preview (rather
// than a type placeholder) for the type.
func hasThumbnail(contentType string) bool {
	ct := normalizeMimeType(contentType)
	return ct == "application/pdf" || strings.HasPrefix(ct, "image/")
}

// attachmentUploadArticle returns the article an agent upload is attached to:
// the ticket's latest article, or a new agent note visible to the customer
// when the ticket has no article yet.
func attachmentUploadArticle(db *sql.DB, ticketID, userID int) (int64, error) {
	repo := repository.NewArticleRepository(db)
	latest, err := repo.GetLatestArticleForTicket(uint(ticketID))
	if err != nil {
		return 0, fmt.Errorf("latest article of ticket %d: %w", ticketID, err)
	}
	if latest != nil && latest.ID > 0 {
		return int64(latest.ID), nil
	}
	article := &models.Article{
		TicketID:               ticketID,
		ArticleTypeID:          constants.ArticleTypeNoteExternal,
		SenderTypeID:           constants.ArticleSenderAgent,
		CommunicationChannelID: constants.CommunicationChannelInternal,
		IsVisibleForCustomer:   1,
		Subject:                "Attachment",
		Body:                   "",
		CreateBy:               userID,
		ChangeBy:               userID,
	}
	if err := repo.Create(article); err != nil {
		return 0, fmt.Errorf("create article for ticket %d: %w", ticketID, err)
	}
	return int64(article.ID), nil
}

// handleUploadAttachment stores an uploaded file on the ticket's latest article.
func handleUploadAttachment(c *gin.Context) {
	db, ticketID, ok := attachmentTicket(c)
	if !ok {
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	if err := validateFile(header); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if header.Size > MaxFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("File size exceeds maximum of %dMB", MaxFileSize/(1024*1024)),
		})
		return
	}

	content, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read uploaded file"})
		return
	}
	if int64(len(content)) > MaxFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("File size exceeds maximum of %dMB", MaxFileSize/(1024*1024)),
		})
		return
	}
	size := int64(len(content))

	// Browser-provided type (aliases normalized); sniff when missing or generic.
	contentType := normalizeMimeType(header.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = normalizeMimeType(detectContentType(header.Filename, content))
	}

	if cfg := config.Get(); cfg != nil {
		if limit := cfg.Storage.Attachments.MaxSize; limit > 0 && size > limit {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("File size exceeds maximum of %dMB", limit/(1024*1024)),
			})
			return
		}
		if len(cfg.Storage.Attachments.AllowedTypes) > 0 && contentType != "application/octet-stream" {
			allowed := false
			for _, t := range cfg.Storage.Attachments.AllowedTypes {
				if strings.EqualFold(t, contentType) {
					allowed = true
					break
				}
			}
			if !allowed {
				c.JSON(http.StatusBadRequest, gin.H{"error": "File type not allowed"})
				return
			}
		}
	}

	ctx := c.Request.Context()
	existing, err := listTicketAttachments(ctx, db, ticketID, false)
	if err != nil {
		attachmentServerError(c, err, "Failed to load attachments")
		return
	}
	if len(existing) >= MaxAttachments {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Ticket attachment limit exceeded (max %d)", MaxAttachments),
		})
		return
	}
	total := size
	for _, a := range existing {
		total += a.Size
	}
	if total > MaxTotalSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Ticket total size limit exceeded (max %dMB)", MaxTotalSize/(1024*1024)),
		})
		return
	}

	uploaderID, ok := auditUserID(c)
	if !ok {
		return
	}
	articleID, err := attachmentUploadArticle(db, ticketID, uploaderID)
	if err != nil {
		attachmentServerError(c, err, "Failed to create article for attachment")
		return
	}
	att, err := storage.ForDB(db).WriteAttachment(ctx, articleID, storage.NewAttachment{
		Filename:    filepath.Base(header.Filename),
		ContentType: contentType,
		Disposition: "attachment",
		Content:     content,
		CreateBy:    uploaderID,
	})
	if err != nil {
		attachmentServerError(c, err, "Failed to store attachment")
		return
	}

	c.Header("HX-Trigger", "attachments-updated")
	downloadURL := attachmentURL(agentTicketAttachmentBase(c), att.ArticleID, att.FileID)
	resp := gin.H{
		"article_id":   att.ArticleID,
		"file_id":      att.FileID,
		"filename":     att.Filename,
		"size":         att.Size,
		"content_type": att.ContentType,
		"download_url": downloadURL,
	}
	if hasThumbnail(att.ContentType) {
		resp["thumbnail_url"] = downloadURL + "/thumbnail"
	}
	c.JSON(http.StatusCreated, resp)
}

// attachmentListItem is one entry of the ticket attachment list.
type attachmentListItem struct {
	ArticleID     int64     `json:"article_id"`
	FileID        int64     `json:"file_id"`
	Filename      string    `json:"filename"`
	Size          int64     `json:"size"`
	SizeFormatted string    `json:"size_formatted"`
	ContentType   string    `json:"content_type"`
	UploadedAt    time.Time `json:"uploaded_at"`
	UploadedBy    int       `json:"uploaded_by"`
	DownloadURL   string    `json:"download_url"`
	ViewURL       string    `json:"view_url"`
	DeleteURL     string    `json:"delete_url"`
	ThumbnailURL  string    `json:"thumbnail_url,omitempty"`
}

func newAttachmentListItem(base string, a storage.Attachment) attachmentListItem {
	u := attachmentURL(base, a.ArticleID, a.FileID)
	ct := attachmentContentType(a, nil)
	item := attachmentListItem{
		ArticleID:     a.ArticleID,
		FileID:        a.FileID,
		Filename:      a.Filename,
		Size:          a.Size,
		SizeFormatted: formatFileSize(a.Size),
		ContentType:   ct,
		UploadedAt:    a.CreateTime,
		UploadedBy:    a.CreateBy,
		DownloadURL:   u,
		ViewURL:       u + "/view",
		DeleteURL:     u,
	}
	if hasThumbnail(ct) {
		item.ThumbnailURL = u + "/thumbnail"
	}
	return item
}

// handleGetAttachments lists a ticket's attachments as JSON, or as an HTML
// partial for HTMX requests.
func handleGetAttachments(c *gin.Context) {
	db, ticketID, ok := attachmentTicket(c)
	if !ok {
		return
	}
	list, err := listTicketAttachments(c.Request.Context(), db, ticketID, false)
	if err != nil {
		attachmentServerError(c, err, "Failed to load attachments")
		return
	}
	base := agentTicketAttachmentBase(c)
	items := make([]attachmentListItem, 0, len(list))
	for _, a := range list {
		items = append(items, newAttachmentListItem(base, a.Attachment))
	}
	if c.GetHeader("HX-Request") == "true" {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderAttachmentListHTML(items)))
		return
	}
	c.JSON(http.StatusOK, gin.H{"attachments": items, "total": len(items)})
}

// handleDownloadAttachment serves an attachment's bytes.
func handleDownloadAttachment(c *gin.Context) {
	_, _, att, content, ok := attachmentRef(c)
	if !ok {
		return
	}
	serveAttachment(c, att, content, false)
}

// handleDeleteAttachment removes an attachment from its article.
func handleDeleteAttachment(c *gin.Context) {
	if _, ok := c.Get("user_id"); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	db, _, att, _, ok := attachmentRef(c)
	if !ok {
		return
	}
	// The article body is not an attachment (it is not listed either).
	if storage.IsHTMLBody(att) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}
	err := storage.ForDB(db).DeleteAttachment(c.Request.Context(), att.ArticleID, att.FileID)
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}
	if err != nil {
		attachmentServerError(c, err, "Failed to delete attachment")
		return
	}
	c.Header("HX-Trigger", "attachments-updated")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Attachment deleted successfully"})
}

// handleGetThumbnail serves a PNG preview of an attachment.
func handleGetThumbnail(c *gin.Context) {
	_, _, att, content, ok := attachmentRef(c)
	if !ok {
		return
	}
	serveThumbnail(c, att, content)
}

// rawAsText reports whether the raw viewer shows a type as plain text: text
// formats are shown as source, never rendered (HTML could carry script).
func rawAsText(contentType string) bool {
	ct := normalizeMimeType(contentType)
	switch {
	case strings.HasPrefix(ct, "image/"):
		return false
	case strings.HasPrefix(ct, "text/"), ct == "application/json", ct == "application/xml":
		return true
	case strings.HasPrefix(ct, "application/"):
		return strings.HasSuffix(ct, "+json") || strings.HasSuffix(ct, "+xml")
	default:
		return false
	}
}

// serveAttachmentRaw serves attachment content for the same-origin viewer
// iframe (/view?raw=1). The global security headers forbid framing, so this
// response allows same-origin framing; script never runs: images and PDFs are
// served as themselves, text formats as text/plain, everything else as a
// download.
func serveAttachmentRaw(c *gin.Context, a storage.Attachment, content []byte) {
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Content-Security-Policy",
		"default-src 'self'; script-src 'none'; style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data: blob:; frame-ancestors 'self'; base-uri 'none'; form-action 'none'")
	c.Header("X-Content-Type-Options", "nosniff")
	if notModified(c, contentETag(content, "r")) {
		return
	}
	c.Header("Cache-Control", "private, no-cache")
	ct := attachmentContentType(a, content)
	switch {
	case inlineSafe(ct):
		c.Header("Content-Disposition", contentDisposition("inline", a.Filename))
		c.Data(http.StatusOK, ct, content)
	case rawAsText(ct):
		c.Header("Content-Disposition", contentDisposition("inline", a.Filename))
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
	default:
		c.Header("Content-Disposition", contentDisposition("attachment", a.Filename))
		c.Data(http.StatusOK, ct, content)
	}
}

// handleViewAttachment renders the attachment viewer page (prev/next walk the
// ticket's attachments); with ?raw=1 it serves the content for the viewer's
// iframe.
func handleViewAttachment(c *gin.Context) {
	db, ticketID, att, content, ok := attachmentRef(c)
	if !ok {
		return
	}
	if c.Query("raw") == "1" {
		serveAttachmentRaw(c, att, content)
		return
	}

	list, err := listTicketAttachments(c.Request.Context(), db, ticketID, false)
	if err != nil {
		attachmentServerError(c, err, "Failed to load attachments")
		return
	}
	base := agentTicketAttachmentBase(c)
	prevURL, nextURL := "", ""
	for i, a := range list {
		if a.ArticleID != att.ArticleID || a.FileID != att.FileID {
			continue
		}
		if i > 0 {
			prevURL = attachmentURL(base, list[i-1].ArticleID, list[i-1].FileID) + "/view"
		}
		if i < len(list)-1 {
			nextURL = attachmentURL(base, list[i+1].ArticleID, list[i+1].FileID) + "/view"
		}
		break
	}

	downloadURL := attachmentURL(base, att.ArticleID, att.FileID)
	page := renderAttachmentViewer(attachmentViewerPage{
		Attachment:  att,
		Content:     content,
		ContentType: attachmentContentType(att, content),
		TicketURL:   "/agent/tickets/" + url.PathEscape(c.Param("id")),
		DownloadURL: downloadURL,
		RawURL:      downloadURL + "/view?raw=1",
		PrevURL:     prevURL,
		NextURL:     nextURL,
	})
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

// attachmentViewerPage is the data of the attachment viewer page.
type attachmentViewerPage struct {
	Attachment  storage.Attachment
	Content     []byte
	ContentType string
	TicketURL   string // where Close / Esc goes
	DownloadURL string
	RawURL      string // framable content (serveAttachmentRaw)
	PrevURL     string
	NextURL     string
}

// renderAttachmentViewer builds the full-screen viewer page.
func renderAttachmentViewer(p attachmentViewerPage) string {
	filename := htmlEscape(p.Attachment.Filename)
	contentType := htmlEscape(p.ContentType)
	rawURL := htmlEscape(p.RawURL)
	ct := normalizeMimeType(p.ContentType)

	var embed string
	switch {
	case isICSSignalled(ct, p.Attachment.Filename):
		// Calendar events as structured cards instead of raw text.
		embed = icsFallbackHTML(rawURL)
		if events := parseICS(p.Content); len(events) > 0 {
			embed = icsEventHTML(events, rawURL)
		}
	case inlineSafe(ct) && strings.HasPrefix(ct, "image/"):
		embed = fmt.Sprintf(`<img src="%s" alt="%s" style="max-width: 100%%; max-height: 100%%; object-fit: contain;" />`, rawURL, filename)
	case ct == "application/pdf":
		embed = fmt.Sprintf(`<iframe src="%s" style="width:100%%; height:100%%; border:0; background:#1a1a1a;"></iframe>`, rawURL)
	default:
		// Text is shown as source; anything else downloads from the iframe.
		embed = fmt.Sprintf(`<iframe src="%s" style="width:100%%; height:100%%; border:0; background:#111;"></iframe>`, rawURL)
	}

	prevNav, nextNav := "", ""
	prevJS, nextJS := "/* no prev */", "/* no next */"
	if p.PrevURL != "" {
		prevNav = fmt.Sprintf(`<div class="nav left"><a class="nav-btn" href="%s" aria-label="Previous">&#x2039;</a></div>`, htmlEscape(p.PrevURL))
		prevJS = "left = document.querySelector('.nav.left');"
	}
	if p.NextURL != "" {
		nextNav = fmt.Sprintf(`<div class="nav right"><a class="nav-btn" href="%s" aria-label="Next">&#x203A;</a></div>`, htmlEscape(p.NextURL))
		nextJS = "right = document.querySelector('.nav.right');"
	}
	ticketURLJS := htmlEscape(jsString(p.TicketURL))

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s · Attachment Viewer</title>
  <style>
	:root { --primary: #0066cc; --primary-hover: #0052a3; }
	@media (prefers-color-scheme: dark) { :root { --primary: #3b82f6; --primary-hover: #2563eb; } }
	html, body { height: 100%%; margin: 0; background: #0b0b0b; color: #e5e5e5; font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif; }
	.header { position: fixed; top: 0; left: 0; right: 0; z-index: 100; background: rgba(17,17,17,.95); backdrop-filter: blur(8px); border-bottom: 1px solid rgba(255,255,255,.1); padding: 12px 16px; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
	.header-left { display: flex; align-items: center; gap: 12px; min-width: 0; }
	.header-right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
	.close-btn { display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; border-radius: 8px; background: var(--primary); color: white; border: none; cursor: pointer; transition: background .15s; flex-shrink: 0; }
	.close-btn:hover { background: var(--primary-hover); }
	.close-btn svg { width: 20px; height: 20px; }
	.file-info { min-width: 0; }
	.filename { font-weight: 600; font-size: 14px; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 400px; }
	.file-meta { font-size: 12px; color: #999; margin-top: 2px; }
	.details-toggle { font-size: 11px; color: #3b82f6; cursor: pointer; margin-left: 8px; }
	.details-toggle:hover { text-decoration: underline; }
	.details-panel { position: fixed; top: 60px; left: 16px; background: rgba(30,30,30,.98); border: 1px solid rgba(255,255,255,.15); border-radius: 8px; padding: 12px 16px; font-size: 12px; z-index: 99; display: none; min-width: 280px; box-shadow: 0 4px 20px rgba(0,0,0,.5); }
	.details-panel.open { display: block; }
	.details-row { display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid rgba(255,255,255,.08); }
	.details-row:last-child { border-bottom: none; }
	.details-label { color: #888; }
	.details-value { color: #ddd; font-weight: 500; }
	.action-btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 14px; border-radius: 6px; font-size: 13px; font-weight: 500; text-decoration: none; transition: all .15s; border: none; cursor: pointer; }
	.btn-primary { background: var(--primary); color: white; }
	.btn-primary:hover { background: var(--primary-hover); }
	.btn-secondary { background: rgba(255,255,255,.1); color: #ddd; }
	.btn-secondary:hover { background: rgba(255,255,255,.15); }
	.viewer { position: fixed; inset: 0; top: 60px; display: grid; place-items: center; }
	.nav { position: fixed; top: 60px; bottom: 0; width: 15%%; max-width: 180px; display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity .25s ease; pointer-events: none; }
	.nav.visible { opacity: .95; pointer-events: auto; }
	.nav.left { left: 0; background: linear-gradient(90deg, rgba(0,0,0,.35), rgba(0,0,0,0)); }
	.nav.right { right: 0; background: linear-gradient(270deg, rgba(0,0,0,.35), rgba(0,0,0,0)); }
	.nav-btn { font: 700 28px/1 system-ui; color: #fff; text-decoration: none; padding: 12px 16px; border-radius: 8px; background: rgba(0,0,0,.35); border: 1px solid rgba(255,255,255,.2); }
	.content { width: 100%%; height: 100%%; display: grid; place-items: center; }
  </style>
</head>
<body>
  <div class="header">
	<div class="header-left">
	  <button class="close-btn" onclick="window.close(); window.location.href=%s;" title="Close (Esc)">
		<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
	  </button>
	  <div class="file-info">
		<div class="filename" title="%s">%s</div>
		<div class="file-meta">
		  %s · %s
		  <span class="details-toggle" onclick="toggleDetails()">▼ More details</span>
		</div>
	  </div>
	</div>
	<div class="header-right">
	  <a href="%s" download class="action-btn btn-primary">
		<svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"/></svg>
		Download
	  </a>
	</div>
  </div>
  <div class="details-panel" id="detailsPanel">
	<div class="details-row"><span class="details-label">Filename</span><span class="details-value">%s</span></div>
	<div class="details-row"><span class="details-label">Type</span><span class="details-value">%s</span></div>
	<div class="details-row"><span class="details-label">Size</span><span class="details-value">%s</span></div>
	<div class="details-row"><span class="details-label">Uploaded</span><span class="details-value">%s</span></div>
	<div class="details-row"><span class="details-label">Article / File</span><span class="details-value">%d / %d</span></div>
  </div>
  <div class="viewer" id="viewer">
	<div class="content">%s</div>
	%s
	%s
  </div>
  <script>
  function toggleDetails() {
	const panel = document.getElementById('detailsPanel');
	panel.classList.toggle('open');
	const toggle = document.querySelector('.details-toggle');
	toggle.textContent = panel.classList.contains('open') ? '▲ Hide details' : '▼ More details';
  }
  (function(){
	let left = null;
	let right = null;
	%s
	%s
	let hideTimer = null;
	function showNav(){
	  if (left) left.classList.add('visible');
	  if (right) right.classList.add('visible');
	  if (hideTimer) clearTimeout(hideTimer);
	  hideTimer = setTimeout(()=>{
		if (left) left.classList.remove('visible');
		if (right) right.classList.remove('visible');
	  }, 1500);
	}
	window.addEventListener('mousemove', showNav, {passive:true});
	window.addEventListener('keydown', function(e){
	  var ae = document.activeElement;
	  if (ae && (ae.isContentEditable || ae.tagName === 'INPUT' || ae.tagName === 'TEXTAREA')) { return; }
	  if (e.key === 'Escape') {
		window.close();
		window.location.href = %s;
	  } else if (e.key === 'ArrowLeft' && left) {
		window.location.href = left.querySelector('a').href;
	  } else if (e.key === 'ArrowRight' && right) {
		window.location.href = right.querySelector('a').href;
	  }
	});
	showNav();
  })();
  </script>
</body>
</html>`,
		filename,
		ticketURLJS,
		filename, filename,
		formatFileSize(p.Attachment.Size), contentType,
		htmlEscape(p.DownloadURL),
		filename, contentType, formatFileSize(p.Attachment.Size),
		htmlEscape(p.Attachment.CreateTime.Format("Jan 2, 2006 at 3:04 PM")),
		p.Attachment.ArticleID, p.Attachment.FileID,
		embed,
		prevNav, nextNav,
		prevJS, nextJS,
		jsString(p.TicketURL),
	)
}

// jsString returns s as a JavaScript string literal that is also safe inside
// an HTML <script> element (<, > and & are \u escaped).
func jsString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	_ = enc.Encode(s) //nolint:errcheck // encoding a string cannot fail
	return strings.TrimSuffix(buf.String(), "\n")
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
	case ".ics", ".ical":
		return "text/calendar"
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

// validateFile validates uploaded file.
func validateFile(header *multipart.FileHeader) error {
	filename := header.Filename

	// SECURITY: Strip null bytes before any validation.
	// Null byte injection: "shell.php\x00.jpg" passes extension check
	// but OS truncates at null, creating "shell.php".
	if strings.ContainsRune(filename, 0) {
		return fmt.Errorf("filename contains illegal characters")
	}

	// Remove path components (anti-traversal).
	filename = filepath.Base(filename)

	// Check for hidden files
	if strings.HasPrefix(filename, ".") {
		return fmt.Errorf("hidden files are not allowed")
	}

	// Check extension
	ext := strings.ToLower(filepath.Ext(filename))
	for _, blocked := range blockedExtensions {
		if ext == blocked {
			return fmt.Errorf("file type not allowed")
		}
	}

	// Check MIME type (be lenient with browser-provided values)
	// - Normalize & strip parameters; allow octet-stream to pass (sniff later)
	if raw := header.Header.Get("Content-Type"); raw != "" {
		ct := normalizeMimeType(raw)
		if ct != "" && ct != "application/octet-stream" {
			if !allowedMimeTypes[ct] {
				return fmt.Errorf("file type not allowed")
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

// Attachment list icons (static markup).
const (
	attachmentIconGeneric = `<svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"></path>
		</svg>`
	attachmentIconPDF = `<svg class="w-5 h-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"></path>
			</svg>`
	attachmentIconCalendar = `<svg class="w-5 h-5 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"></path>
			</svg>`
	attachmentIconText = `<svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
			</svg>`
)

// renderAttachmentListHTML renders the attachment list partial for HTMX. Every
// value taken from the attachment is HTML-escaped; the delete handler
// arguments are JS string literals inside an HTML-escaped attribute.
func renderAttachmentListHTML(items []attachmentListItem) string {
	if len(items) == 0 {
		return `<div class="text-center py-4 text-sm text-gray-500 dark:text-gray-400">No attachments found</div>`
	}

	var b strings.Builder
	b.WriteString(`<div class="space-y-2 p-4">`)
	for _, att := range items {
		icon := attachmentIconGeneric
		ct := normalizeMimeType(att.ContentType)
		switch {
		case att.ThumbnailURL != "" && strings.HasPrefix(ct, "image/"):
			icon = fmt.Sprintf(`<img src="%s" alt="thumb" class="w-10 h-10 rounded object-cover ring-1 ring-gray-200 dark:ring-gray-700"/>`, htmlEscape(att.ThumbnailURL))
		case ct == "application/pdf":
			icon = attachmentIconPDF
		case isICSSignalled(ct, att.Filename):
			icon = attachmentIconCalendar
		case strings.HasPrefix(ct, "text/"):
			icon = attachmentIconText
		}
		onDelete := htmlEscape(fmt.Sprintf("deleteAttachment(%s, %s)", jsString(att.DeleteURL), jsString(att.Filename)))
		viewURL := htmlEscape(att.ViewURL)

		fmt.Fprintf(&b, `
		<div class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors">
			<div class="flex items-center space-x-3">
				%s
				<div>
					<a href="%s" target="_blank" class="text-sm font-medium text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300">
						%s
					</a>
					<p class="text-xs text-gray-500 dark:text-gray-400">%s</p>
				</div>
			</div>
			<div class="flex items-center space-x-2">
				<a href="%s" target="_blank" class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" title="View">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
					</svg>
				</a>
				<a href="%s" class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" title="Download" download>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"></path>
					</svg>
				</a>
				<button type="button" onclick="%s" class="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400" title="Delete">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
					</svg>
				</button>
			</div>
		</div>`,
			icon, viewURL, htmlEscape(att.Filename), htmlEscape(att.SizeFormatted),
			viewURL, htmlEscape(att.DownloadURL), onDelete)
	}
	b.WriteString(`</div>`)
	return b.String()
}
