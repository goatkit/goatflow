package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/pdfthumb"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/storage"
)

// Article attachments are addressed like OTRS does: ticket, article, file id
// (/tickets/:id/articles/:article_id/attachments/:file_id). The file id is
// backend specific (see storage.Attachment.FileID), so it is only meaningful
// together with its article.

// ticketAttachment is one attachment of one of a ticket's articles.
type ticketAttachment struct {
	storage.Attachment
	TicketID int
}

// listTicketAttachments returns the user-visible attachments of every article
// of a ticket (oldest article first). HTML body parts are not attachments.
// With customerOnly, articles not visible to customers are skipped.
func listTicketAttachments(ctx context.Context, db *sql.DB, ticketID int, customerOnly bool) ([]ticketAttachment, error) {
	query := "SELECT id FROM article WHERE ticket_id = ?"
	if customerOnly {
		query += " AND is_visible_for_customer = 1"
	}
	query += " ORDER BY id"
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(query), ticketID)
	if err != nil {
		return nil, fmt.Errorf("list articles of ticket %d: %w", ticketID, err)
	}
	var articleIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list articles of ticket %d: %w", ticketID, err)
		}
		articleIDs = append(articleIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list articles of ticket %d: %w", ticketID, err)
	}

	store := storage.ForDB(db)
	list := make([]ticketAttachment, 0)
	for _, articleID := range articleIDs {
		atts, err := store.ListAttachments(ctx, articleID)
		if err != nil {
			return nil, err
		}
		for _, a := range atts {
			if storage.IsHTMLBody(a) {
				continue
			}
			list = append(list, ticketAttachment{Attachment: a, TicketID: ticketID})
		}
	}
	return list, nil
}

// ticketHasArticle reports whether articleID belongs to ticketID (and, with
// customerOnly, is visible to customers).
func ticketHasArticle(ctx context.Context, db *sql.DB, ticketID int, articleID int64, customerOnly bool) (bool, error) {
	query := "SELECT 1 FROM article WHERE id = ? AND ticket_id = ?"
	if customerOnly {
		query += " AND is_visible_for_customer = 1"
	}
	var one int
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders(query), articleID, ticketID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check article %d of ticket %d: %w", articleID, ticketID, err)
	}
	return true, nil
}

// getTicketAttachment loads one attachment after checking that its article
// belongs to the ticket. A foreign or missing article/attachment is
// storage.ErrNotFound.
func getTicketAttachment(
	ctx context.Context, db *sql.DB, ticketID int, articleID, fileID int64, customerOnly bool,
) (storage.Attachment, []byte, error) {
	ok, err := ticketHasArticle(ctx, db, ticketID, articleID, customerOnly)
	if err != nil {
		return storage.Attachment{}, nil, err
	}
	if !ok {
		return storage.Attachment{}, nil, storage.ErrNotFound
	}
	return storage.ForDB(db).GetAttachment(ctx, articleID, fileID)
}

// attachmentRefParams parses the :article_id and :file_id route params.
func attachmentRefParams(c *gin.Context) (articleID, fileID int64, ok bool) {
	articleID, err := strconv.ParseInt(c.Param("article_id"), 10, 64)
	if err != nil || articleID <= 0 {
		return 0, 0, false
	}
	fileID, err = strconv.ParseInt(c.Param("file_id"), 10, 64)
	if err != nil || fileID <= 0 {
		return 0, 0, false
	}
	return articleID, fileID, true
}

// attachmentURL builds the URL of an attachment under base, e.g.
// attachmentURL("/api/tickets/1001", 5, 2) = "/api/tickets/1001/articles/5/attachments/2".
func attachmentURL(base string, articleID, fileID int64) string {
	return fmt.Sprintf("%s/articles/%d/attachments/%d", base, articleID, fileID)
}

// contentDisposition builds a Content-Disposition header value that is safe for
// any filename (RFC 6266: quoted ASCII fallback plus RFC 5987 filename*).
func contentDisposition(dispositionType, filename string) string {
	if v := mime.FormatMediaType(dispositionType, map[string]string{"filename": filename}); v != "" {
		return v
	}
	fallback := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	return fmt.Sprintf(`%s; filename="%s"`, dispositionType, fallback)
}

// contentETag is a strong validator for attachment-derived responses.
func contentETag(content []byte, variant string) string {
	sum := sha256.Sum256(content)
	return `"` + variant + "-" + hex.EncodeToString(sum[:16]) + `"`
}

// notModified answers 304 when the request's If-None-Match carries etag.
func notModified(c *gin.Context, etag string) bool {
	c.Header("ETag", etag)
	for _, candidate := range strings.Split(c.GetHeader("If-None-Match"), ",") {
		if strings.TrimSpace(candidate) == etag {
			c.Status(http.StatusNotModified)
			return true
		}
	}
	return false
}

// attachmentContentType returns the stored content type, or one detected from
// the name and bytes when the stored one is missing or generic.
func attachmentContentType(a storage.Attachment, content []byte) string {
	ct := strings.TrimSpace(a.ContentType)
	if ct == "" || strings.EqualFold(normalizeMimeType(ct), "application/octet-stream") {
		return detectContentType(a.Filename, content)
	}
	return ct
}

// inlineSafe reports whether a type may be shown inline on our origin. SVG and
// HTML can carry script, so they are always downloaded.
func inlineSafe(contentType string) bool {
	ct := normalizeMimeType(contentType)
	return ct == "application/pdf" || (strings.HasPrefix(ct, "image/") && ct != "image/svg+xml")
}

// serveAttachment writes an attachment's bytes. Safe types (images, PDF) are
// served inline unless download is set; everything else as a download.
func serveAttachment(c *gin.Context, a storage.Attachment, content []byte, download bool) {
	if notModified(c, contentETag(content, "a")) {
		return
	}
	ct := attachmentContentType(a, content)
	disposition := "attachment"
	if !download && inlineSafe(ct) {
		disposition = "inline"
	}
	c.Header("Content-Disposition", contentDisposition(disposition, a.Filename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-cache")
	c.Data(http.StatusOK, ct, content)
}

// Thumbnails fit in thumbnailMaxW x thumbnailMaxH.
const (
	thumbnailMaxW = 320
	thumbnailMaxH = 240
)

// serveThumbnail writes a PNG preview: images are scaled down, PDFs show page
// one, anything else (or anything undecodable) gets a type placeholder.
// Responses are validated by an ETag of the content, so no server-side cache
// can go stale when attachments change.
func serveThumbnail(c *gin.Context, a storage.Attachment, content []byte) {
	if notModified(c, contentETag(content, "t")) {
		return
	}
	c.Header("Cache-Control", "private, no-cache")
	ct := normalizeMimeType(attachmentContentType(a, content))
	if png, err := renderThumbnail(ct, content); err == nil {
		c.Data(http.StatusOK, "image/png", png)
		return
	}
	ph, phType := service.GetPlaceholderThumbnail(ct)
	c.Data(http.StatusOK, phType, ph)
}

func renderThumbnail(contentType string, content []byte) ([]byte, error) {
	switch {
	case contentType == "application/pdf":
		return pdfthumb.RenderPage1(content)
	case strings.HasPrefix(contentType, "image/"):
		img, err := vips.NewImageFromBuffer(content)
		if err != nil {
			return nil, err
		}
		defer img.Close()
		w, h := img.Width(), img.Height()
		if w > thumbnailMaxW || h > thumbnailMaxH {
			scale := float64(thumbnailMaxW) / float64(w)
			if sy := float64(thumbnailMaxH) / float64(h); sy < scale {
				scale = sy
			}
			if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
				return nil, err
			}
		}
		png, _, err := img.ExportPng(&vips.PngExportParams{Compression: 6})
		return png, err
	default:
		return nil, fmt.Errorf("no thumbnail for %s", contentType)
	}
}
