package plugin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// ArticleAttachmentStore is the product's article content store (DB or OTRS
// ArticleStorageFS) as seen by the plugin host. It is injected with
// WithArticleAttachmentStore so this platform package does not import product
// code. Attachments are addressed by (articleID, fileID).
type ArticleAttachmentStore interface {
	ListArticleAttachments(ctx context.Context, db *sql.DB, articleID int64) ([]pkgplugin.ArticleAttachment, error)
	WriteArticleAttachment(ctx context.Context, db *sql.DB, articleID, createdBy int64, filename, contentType string, content []byte) (int64, error)
	// DeleteArticleAttachment returns ErrAttachmentNotFound when the
	// attachment does not exist on the article.
	DeleteArticleAttachment(ctx context.Context, db *sql.DB, articleID, fileID int64) error
}

// ErrAttachmentNotFound is returned by ArticleAttachmentStore.DeleteArticleAttachment.
var ErrAttachmentNotFound = errors.New("attachment not found")

var errNoAttachmentStore = errors.New("article attachment store not configured")

// WithArticleAttachmentStore sets the article attachment store.
func WithArticleAttachmentStore(s ArticleAttachmentStore) ProdHostAPIOption {
	return func(h *ProdHostAPI) {
		h.attachments = s
	}
}

// articleAttachmentSizeLimit returns the max attachment size (bytes) from the
// platform storage config; falls back to 10 MiB when unset.
func articleAttachmentSizeLimit() int64 {
	if c := config.Get(); c != nil && c.Storage.Attachments.MaxSize > 0 {
		return c.Storage.Attachments.MaxSize
	}
	return 10 * 1024 * 1024
}

// CreateArticleAttachment attaches a file to an article's thread (visible in
// the ticket page and, for customer-visible articles, the portal). createdBy
// must be a valid users.id (the acting coach), consistent with how plugins
// already write article rows. The platform enforces the configured size limit.
func (h *ProdHostAPI) CreateArticleAttachment(ctx context.Context, articleID, createdBy int64, filename, contentType string, content []byte) (int64, error) {
	db, err := h.getDB("")
	if err != nil {
		return 0, err
	}
	if h.attachments == nil {
		return 0, errNoAttachmentStore
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

	id, err := h.attachments.WriteArticleAttachment(ctx, db, articleID, createdBy, filename, contentType, content)
	if err != nil {
		return 0, fmt.Errorf("write article attachment: %w", err)
	}
	return id, nil
}

// ListArticleAttachments returns metadata for every attachment on an article
// in storage order. Content is excluded (downloads go through the platform's
// article attachment endpoint).
func (h *ProdHostAPI) ListArticleAttachments(ctx context.Context, articleID int64) ([]pkgplugin.ArticleAttachment, error) {
	db, err := h.getDB("")
	if err != nil {
		return nil, err
	}
	if h.attachments == nil {
		return nil, errNoAttachmentStore
	}
	atts, err := h.attachments.ListArticleAttachments(ctx, db, articleID)
	if err != nil {
		return nil, fmt.Errorf("list article attachments: %w", err)
	}
	return atts, nil
}

// DeleteArticleAttachment removes one attachment from an article.
func (h *ProdHostAPI) DeleteArticleAttachment(ctx context.Context, articleID, attachmentID int64) error {
	db, err := h.getDB("")
	if err != nil {
		return err
	}
	if h.attachments == nil {
		return errNoAttachmentStore
	}
	if err := h.attachments.DeleteArticleAttachment(ctx, db, articleID, attachmentID); err != nil {
		if errors.Is(err, ErrAttachmentNotFound) {
			return fmt.Errorf("attachment %d not found on article %d", attachmentID, articleID)
		}
		return fmt.Errorf("delete article attachment: %w", err)
	}
	return nil
}
