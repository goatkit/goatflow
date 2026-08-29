package plugin

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// articleAttachmentSizeLimit returns the max attachment size (bytes) from the
// platform storage config; falls back to 10 MiB when unset.
func articleAttachmentSizeLimit() int64 {
	if c := config.Get(); c != nil && c.Storage.Attachments.MaxSize > 0 {
		return c.Storage.Attachments.MaxSize
	}
	return 10 * 1024 * 1024
}

// CreateArticleAttachment attaches a file to an article's thread. The content
// is stored as an article_data_mime_attachment row (visible in the ticket page
// and, for customer-visible articles, the portal). createdBy must be a valid
// users.id (the acting coach), consistent with how plugins already write
// article rows. The platform enforces the configured size limit.
func (h *ProdHostAPI) CreateArticleAttachment(ctx context.Context, articleID, createdBy int64, filename, contentType string, content []byte) (int64, error) {
	db, err := h.getDB("")
	if err != nil {
		return 0, err
	}
	if articleID <= 0 {
		return 0, fmt.Errorf("invalid article_id %d", articleID)
	}
	if createdBy <= 0 {
		return 0, fmt.Errorf("invalid created_by %d", createdBy)
	}
	if filename == "" {
		return 0, fmt.Errorf("filename required")
	}

	var exists int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT 1 FROM article WHERE id = ?`), articleID).Scan(&exists); err != nil {
		return 0, fmt.Errorf("article %d not found: %w", articleID, err)
	}

	if max := articleAttachmentSizeLimit(); int64(len(content)) > max {
		return 0, fmt.Errorf("attachment %d bytes exceeds size limit %d", len(content), max)
	}

	// content_size is VARCHAR in the OTRS-legacy schema, so it is stored as a
	// string. disposition is fixed to "attachment" (mirrors the upload path).
	now := time.Now()
	res, err := db.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO article_data_mime_attachment
			(article_id, filename, content_size, content_type, disposition, content,
			 create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, 'attachment', ?, ?, ?, ?, ?)`),
		articleID, filename, strconv.FormatInt(int64(len(content)), 10), contentType,
		content, now, createdBy, now, createdBy)
	if err != nil {
		return 0, fmt.Errorf("insert article attachment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("attachment inserted but id unavailable: %w", err)
	}
	return id, nil
}

// ListArticleAttachments returns metadata for every attachment on an article,
// newest first. Content is excluded (downloads go through the platform's
// article attachment endpoint).
func (h *ProdHostAPI) ListArticleAttachments(ctx context.Context, articleID int64) ([]pkgplugin.ArticleAttachment, error) {
	db, err := h.getDB("")
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id, article_id, filename, content_type, content_size, create_time, create_by
		FROM article_data_mime_attachment WHERE article_id = ? ORDER BY id DESC`), articleID)
	if err != nil {
		return nil, fmt.Errorf("list article attachments: %w", err)
	}
	defer rows.Close()

	var out []pkgplugin.ArticleAttachment
	for rows.Next() {
		var (
			att        pkgplugin.ArticleAttachment
			sizeStr    string
			createTime sql.NullString
		)
		if err := rows.Scan(&att.ID, &att.ArticleID, &att.Filename, &att.ContentType, &sizeStr, &createTime, &att.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan article attachment: %w", err)
		}
		att.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
		if createTime.Valid {
			att.CreatedAt = createTime.String
		}
		out = append(out, att)
	}
	return out, rows.Err()
}

// DeleteArticleAttachment removes one attachment from an article.
func (h *ProdHostAPI) DeleteArticleAttachment(ctx context.Context, articleID, attachmentID int64) error {
	db, err := h.getDB("")
	if err != nil {
		return err
	}
	res, err := db.ExecContext(ctx, database.ConvertPlaceholders(
		`DELETE FROM article_data_mime_attachment WHERE id = ? AND article_id = ?`),
		attachmentID, articleID)
	if err != nil {
		return fmt.Errorf("delete article attachment: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("attachment %d not found on article %d", attachmentID, articleID)
	}
	return nil
}
