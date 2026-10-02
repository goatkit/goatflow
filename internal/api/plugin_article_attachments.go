package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/storage"
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// PluginArticleAttachmentStore serves the plugin HostAPI attachment calls from
// the configured article content store (internal/storage).
type PluginArticleAttachmentStore struct{}

var _ plugin.ArticleAttachmentStore = PluginArticleAttachmentStore{}

// ListArticleAttachments implements plugin.ArticleAttachmentStore. HTML body
// parts are not attachments and are left out (OTRS ExcludeHTMLBody).
func (PluginArticleAttachmentStore) ListArticleAttachments(ctx context.Context, db *sql.DB, articleID int64) ([]pkgplugin.ArticleAttachment, error) {
	var ticketID int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT ticket_id FROM article WHERE id = ?`), articleID).Scan(&ticketID); err != nil {
		return nil, fmt.Errorf("article %d: %w", articleID, err)
	}
	atts, err := storage.ForDB(db).ListAttachments(ctx, articleID)
	if err != nil {
		return nil, err
	}
	base := "/api/tickets/" + strconv.Itoa(ticketID)
	out := make([]pkgplugin.ArticleAttachment, 0, len(atts))
	for _, a := range atts {
		if storage.IsHTMLBody(a) {
			continue
		}
		out = append(out, pkgplugin.ArticleAttachment{
			ID:          a.FileID,
			ArticleID:   a.ArticleID,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
			URL:         attachmentURL(base, a.ArticleID, a.FileID),
		})
	}
	return out, nil
}

// WriteArticleAttachment implements plugin.ArticleAttachmentStore.
func (PluginArticleAttachmentStore) WriteArticleAttachment(ctx context.Context, db *sql.DB, articleID, createdBy int64, filename, contentType string, content []byte) (int64, error) {
	a, err := storage.ForDB(db).WriteAttachment(ctx, articleID, storage.NewAttachment{
		Filename:    filename,
		ContentType: contentType,
		Disposition: "attachment",
		Content:     content,
		CreateBy:    int(createdBy),
	})
	if err != nil {
		return 0, err
	}
	return a.FileID, nil
}

// DeleteArticleAttachment implements plugin.ArticleAttachmentStore.
func (PluginArticleAttachmentStore) DeleteArticleAttachment(ctx context.Context, db *sql.DB, articleID, fileID int64) error {
	err := storage.ForDB(db).DeleteAttachment(ctx, articleID, fileID)
	if errors.Is(err, storage.ErrNotFound) {
		return plugin.ErrAttachmentNotFound
	}
	return err
}
