package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// DatabaseBackend implements article storage in the database (OTRS ArticleStorageDB).
type DatabaseBackend struct {
	db *sql.DB
}

// NewDatabaseBackend creates a new database storage backend.
func NewDatabaseBackend(db *sql.DB) *DatabaseBackend {
	return &DatabaseBackend{
		db: db,
	}
}

// Store saves article content to the database.
func (d *DatabaseBackend) Store(ctx context.Context, articleID int64, content *ArticleContent) (*StorageReference, error) {
	// Calculate checksum
	hash := sha256.Sum256(content.Content)
	checksum := hex.EncodeToString(hash[:])

	// Begin transaction
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check if this is the main article body or an attachment
	if content.FileName == "" || content.FileName == "body" {
		// Update the existing body row if present, otherwise insert a new one.
		var mimeID int64
		err = tx.QueryRowContext(ctx, database.ConvertPlaceholders(
			"SELECT id FROM article_data_mime WHERE article_id = ? ORDER BY id LIMIT 1"), articleID).Scan(&mimeID)
		switch {
		case err == nil:
			_, err = tx.ExecContext(ctx, database.ConvertPlaceholders(`
                UPDATE article_data_mime
                SET a_body = ?, a_content_type = ?, change_time = ?, change_by = ?
                WHERE id = ?`),
				content.Content,
				content.ContentType,
				time.Now(),
				content.CreatedBy,
				mimeID,
			)
		case err == sql.ErrNoRows:
			mimeID, err = database.GetAdapter().InsertWithReturningTx(tx, database.ConvertPlaceholders(`
            INSERT INTO article_data_mime (
                article_id, a_subject, a_body, a_content_type,
                incoming_time, create_time, create_by, change_time, change_by
            ) VALUES (
                ?, ?, ?, ?, ?, ?, ?, ?, ?
            ) RETURNING id`),
				articleID,
				content.Metadata["subject"],
				content.Content,
				content.ContentType,
				time.Now().Unix(),
				content.CreatedTime,
				content.CreatedBy,
				time.Now(),
				content.CreatedBy,
			)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to store article body: %w", err)
		}

		location := fmt.Sprintf("article_data_mime:%d", mimeID)

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		return &StorageReference{
			ID:          mimeID,
			ArticleID:   articleID,
			Backend:     "DB",
			Location:    location,
			ContentType: content.ContentType,
			FileName:    "body",
			FileSize:    content.FileSize,
			Checksum:    checksum,
			CreatedTime: content.CreatedTime,
		}, nil
	}

	// Store as attachment
	attachmentID, err := database.GetAdapter().InsertWithReturningTx(tx, database.ConvertPlaceholders(`
        INSERT INTO article_data_mime_attachment (
            article_id, filename, content_type, content_size,
            content, content_id, content_alternative, disposition,
            create_time, create_by, change_time, change_by
        ) VALUES (
            ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
        ) RETURNING id`),
		articleID,
		content.FileName,
		content.ContentType,
		fmt.Sprintf("%d", content.FileSize),
		content.Content,
		content.Metadata["content_id"],
		content.Metadata["content_alternative"],
		content.Metadata["disposition"],
		content.CreatedTime,
		content.CreatedBy,
		time.Now(),
		content.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store attachment: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	location := fmt.Sprintf("attachment:%d", attachmentID)

	return &StorageReference{
		ID:          attachmentID,
		ArticleID:   articleID,
		Backend:     "DB",
		Location:    location,
		ContentType: content.ContentType,
		FileName:    content.FileName,
		FileSize:    content.FileSize,
		Checksum:    checksum,
		CreatedTime: content.CreatedTime,
	}, nil
}

// Retrieve gets article content from the database.
func (d *DatabaseBackend) Retrieve(ctx context.Context, ref *StorageReference) (*ArticleContent, error) {
	content := &ArticleContent{
		ArticleID: ref.ArticleID,
		Metadata:  make(map[string]string),
	}

	// Check if this is article body or attachment
	if ref.FileName == "body" || strings.HasPrefix(ref.Location, "article_data_mime:") {
		// Retrieve from article_data_mime
		query := database.ConvertPlaceholders(`
			SELECT 
				a_body, a_content_type, a_subject, 
				create_time, create_by
			FROM article_data_mime
			WHERE article_id = ?`)

		var subject sql.NullString
		err := d.db.QueryRowContext(ctx, query, ref.ArticleID).Scan(
			&content.Content,
			&content.ContentType,
			&subject,
			&content.CreatedTime,
			&content.CreatedBy,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("article body not found")
			}
			return nil, fmt.Errorf("failed to retrieve article body: %w", err)
		}

		if subject.Valid {
			content.Metadata["subject"] = subject.String
		}
		content.FileName = "body"
		content.FileSize = int64(len(content.Content))

		return content, nil
	}

	// Retrieve attachment
	query := database.ConvertPlaceholders(`
		SELECT 
			content, content_type, filename, content_size,
			content_id, content_alternative, disposition,
			create_time, create_by
		FROM article_data_mime_attachment
		WHERE article_id = ? AND filename = ?`)

	var contentID, contentAlt, disposition sql.NullString
	var contentSize string

	err := d.db.QueryRowContext(ctx, query, ref.ArticleID, ref.FileName).Scan(
		&content.Content,
		&content.ContentType,
		&content.FileName,
		&contentSize,
		&contentID,
		&contentAlt,
		&disposition,
		&content.CreatedTime,
		&content.CreatedBy,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attachment not found")
		}
		return nil, fmt.Errorf("failed to retrieve attachment: %w", err)
	}

	if contentID.Valid {
		content.Metadata["content_id"] = contentID.String
	}
	if contentAlt.Valid {
		content.Metadata["content_alternative"] = contentAlt.String
	}
	if disposition.Valid {
		content.Metadata["disposition"] = disposition.String
	}

	content.FileSize = int64(len(content.Content))

	return content, nil
}

// Delete removes article content from the database.
func (d *DatabaseBackend) Delete(ctx context.Context, ref *StorageReference) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check if this is article body or attachment
	if ref.FileName == "body" || (len(ref.Location) > 17 && ref.Location[:17] == "article_data_mime") {
		// Delete from article_data_mime
		_, err = tx.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM article_data_mime WHERE article_id = ?"), ref.ArticleID)
		if err != nil {
			return fmt.Errorf("failed to delete article body: %w", err)
		}
	} else {
		// Delete attachment
		_, err = tx.ExecContext(ctx,
			database.ConvertPlaceholders("DELETE FROM article_data_mime_attachment WHERE article_id = ? AND filename = ?"),
			ref.ArticleID, ref.FileName)
		if err != nil {
			return fmt.Errorf("failed to delete attachment: %w", err)
		}
	}

	return tx.Commit()
}

// Exists checks if article content exists in the database.
func (d *DatabaseBackend) Exists(ctx context.Context, ref *StorageReference) (bool, error) {
	var exists bool

	if ref.FileName == "body" {
		query := database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM article_data_mime WHERE article_id = ?)")
		err := d.db.QueryRowContext(ctx, query, ref.ArticleID).Scan(&exists)
		return exists, err
	}

	query := database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM article_data_mime_attachment WHERE article_id = ? AND filename = ?)")
	err := d.db.QueryRowContext(ctx, query, ref.ArticleID, ref.FileName).Scan(&exists)
	return exists, err
}

// List returns all storage references for an article.
func (d *DatabaseBackend) List(ctx context.Context, articleID int64) ([]*StorageReference, error) {
	refs := make([]*StorageReference, 0)

	// Check for article body
	var hasMime bool
	err := d.db.QueryRowContext(ctx,
		database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM article_data_mime WHERE article_id = ?)"),
		articleID).Scan(&hasMime)
	if err != nil {
		return nil, fmt.Errorf("failed to check article body: %w", err)
	}

	if hasMime {
		// Get article body info
		query := database.ConvertPlaceholders(`
			SELECT 
				id, a_content_type, octet_length(a_body) as size,
				create_time
			FROM article_data_mime
			WHERE article_id = ?`)

		var mimeID int64
		var contentType string
		var size int64
		var createdTime time.Time

		err = d.db.QueryRowContext(ctx, query, articleID).Scan(
			&mimeID, &contentType, &size, &createdTime,
		)
		if err == nil {
			refs = append(refs, &StorageReference{
				ID:          mimeID,
				ArticleID:   articleID,
				Backend:     "DB",
				Location:    fmt.Sprintf("article_data_mime:%d", mimeID),
				ContentType: contentType,
				FileName:    "body",
				FileSize:    size,
				CreatedTime: createdTime,
			})
		}
	}

	// Get attachments
	query := database.ConvertPlaceholders(`
		SELECT 
			id, filename, content_type, COALESCE(content_size, '0'),
			create_time
		FROM article_data_mime_attachment
		WHERE article_id = ?
		ORDER BY id`)

	rows, err := d.db.QueryContext(ctx, query, articleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list attachments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ref StorageReference
		ref.ArticleID = articleID
		ref.Backend = "DB"

		var contentSize string
		err := rows.Scan(
			&ref.ID,
			&ref.FileName,
			&ref.ContentType,
			&contentSize,
			&ref.CreatedTime,
		)
		if err != nil {
			continue
		}
		ref.FileSize, _ = strconv.ParseInt(strings.TrimSpace(contentSize), 10, 64)

		ref.Location = fmt.Sprintf("attachment:%d", ref.ID)
		refs = append(refs, &ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate attachments: %w", err)
	}

	return refs, nil
}

// Migrate is handled by the MixedModeBackend.
func (d *DatabaseBackend) Migrate(ctx context.Context, ref *StorageReference, target Backend) (*StorageReference, error) {
	// Retrieve content
	content, err := d.Retrieve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve content for migration: %w", err)
	}

	// Store in target
	newRef, err := target.Store(ctx, ref.ArticleID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to store content in target: %w", err)
	}

	return newRef, nil
}

// GetInfo returns backend information.
func (d *DatabaseBackend) GetInfo() *BackendInfo {
	stats := &BackendStats{}

	// Get statistics
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Count articles
	d.db.QueryRowContext(ctx, // sql-converted: static query, no placeholders or dialect constructs
		"SELECT COUNT(*) FROM article_data_mime").Scan(&stats.TotalFiles)

	// Count attachments
	var attachments int64
	d.db.QueryRowContext(ctx, // sql-converted: static query, no placeholders or dialect constructs
		"SELECT COUNT(*) FROM article_data_mime_attachment").Scan(&attachments)
	stats.TotalFiles += attachments

	// Get total size
	d.db.QueryRowContext(ctx, // sql-converted: static portable query (octet_length exists on both)
		"SELECT COALESCE(SUM(octet_length(a_body)), 0) FROM article_data_mime").Scan(&stats.TotalSize)

	var attachmentSize int64
	d.db.QueryRowContext(ctx, // sql-converted: static portable query (octet_length exists on both)
		"SELECT COALESCE(SUM(octet_length(content)), 0) FROM article_data_mime_attachment").Scan(&attachmentSize)
	stats.TotalSize += attachmentSize

	return &BackendInfo{
		Name: "DatabaseBackend",
		Type: "DB",
		Capabilities: []string{
			"store",
			"retrieve",
			"delete",
			"list",
			"transactional",
		},
		Status:     "active",
		Statistics: stats,
	}
}

// HealthCheck verifies the database connection.
func (d *DatabaseBackend) HealthCheck(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Register the database backend with the factory.
func init() {
	DefaultFactory.Register("DB", func(config map[string]interface{}) (Backend, error) {
		db, ok := config["db"].(*sql.DB)
		if !ok {
			return nil, fmt.Errorf("database backend requires 'db' configuration")
		}
		return NewDatabaseBackend(db), nil
	})
}
