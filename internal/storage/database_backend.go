package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// DatabaseStore keeps attachments in article_data_mime_attachment and raw
// emails in article_data_mime_plain (OTRS ArticleStorageDB).
type DatabaseStore struct {
	db *sql.DB
	tx *sql.Tx
}

// NewDatabaseStore returns a DB-backed article store.
func NewDatabaseStore(db *sql.DB) *DatabaseStore {
	return &DatabaseStore{db: db}
}

// Backend returns BackendDB.
func (s *DatabaseStore) Backend() string { return BackendDB }

// WithTx returns a store running on tx.
func (s *DatabaseStore) WithTx(tx *sql.Tx) ArticleStore {
	return &DatabaseStore{db: s.db, tx: tx}
}

func (s *DatabaseStore) q() querier {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

// ListAttachments returns the article's attachments ordered by id.
func (s *DatabaseStore) ListAttachments(ctx context.Context, articleID int64) ([]Attachment, error) {
	rows, err := s.q().QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id, filename, content_type, content_id, content_alternative, disposition,
			COALESCE(octet_length(content), 0), create_time, create_by
		FROM article_data_mime_attachment
		WHERE article_id = ?
		ORDER BY id`), articleID)
	if err != nil {
		return nil, fmt.Errorf("list attachments of article %d: %w", articleID, err)
	}
	defer rows.Close()

	list := make([]Attachment, 0)
	for rows.Next() {
		a, err := scanAttachment(rows.Scan, articleID)
		if err != nil {
			return nil, fmt.Errorf("list attachments of article %d: %w", articleID, err)
		}
		list = append(list, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list attachments of article %d: %w", articleID, err)
	}
	return list, nil
}

func scanAttachment(scan func(...interface{}) error, articleID int64, extra ...interface{}) (Attachment, error) {
	a := Attachment{ArticleID: articleID}
	var filename, contentType, contentID, contentAlt, disposition sql.NullString
	dest := append([]interface{}{
		&a.FileID, &filename, &contentType, &contentID, &contentAlt, &disposition, &a.Size, &a.CreateTime, &a.CreateBy,
	}, extra...)
	if err := scan(dest...); err != nil {
		return a, err
	}
	a.Filename = filename.String
	a.ContentType = contentType.String
	a.ContentID = contentID.String
	a.ContentAlternative = contentAlt.String
	a.Disposition = disposition.String
	if a.Disposition == "" {
		a.Disposition = defaultDisposition(a.Filename, a.ContentType, a.ContentID)
	}
	return a, nil
}

// GetAttachment returns one attachment row and its content.
func (s *DatabaseStore) GetAttachment(ctx context.Context, articleID, fileID int64) (Attachment, []byte, error) {
	var content []byte
	row := s.q().QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT id, filename, content_type, content_id, content_alternative, disposition,
			COALESCE(octet_length(content), 0), create_time, create_by, content
		FROM article_data_mime_attachment
		WHERE id = ? AND article_id = ?`), fileID, articleID)
	a, err := scanAttachment(row.Scan, articleID, &content)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, nil, ErrNotFound
	}
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("get attachment %d of article %d: %w", fileID, articleID, err)
	}
	if content == nil {
		content = []byte{}
	}
	return a, content, nil
}

// WriteAttachment inserts an attachment row.
func (s *DatabaseStore) WriteAttachment(ctx context.Context, articleID int64, in NewAttachment) (Attachment, error) {
	in = normalizeAttachment(in)
	existing, err := s.ListAttachments(ctx, articleID)
	if err != nil {
		return Attachment{}, err
	}
	used := make(map[string]bool, len(existing))
	for _, a := range existing {
		used[a.Filename] = true
	}
	filename := uniqueFilename(in.Filename, used)

	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO article_data_mime_attachment (
			article_id, filename, content_type, content_size, content,
			content_id, content_alternative, disposition,
			create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`)
	args := []interface{}{
		articleID, filename, in.ContentType, strconv.Itoa(len(in.Content)), in.Content,
		nullIfEmpty(in.ContentID), nullIfEmpty(in.ContentAlternative), nullIfEmpty(in.Disposition),
		now, in.CreateBy, now, in.CreateBy,
	}
	var id int64
	if s.tx != nil {
		id, err = database.GetAdapter().InsertWithReturningTx(s.tx, query, args...)
	} else {
		id, err = database.GetAdapter().InsertWithReturning(s.db, query, args...)
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("write attachment of article %d: %w", articleID, err)
	}

	disposition := in.Disposition
	if disposition == "" {
		disposition = defaultDisposition(filename, in.ContentType, in.ContentID)
	}
	return Attachment{
		ArticleID:          articleID,
		FileID:             id,
		Filename:           filename,
		ContentType:        in.ContentType,
		ContentID:          in.ContentID,
		ContentAlternative: in.ContentAlternative,
		Disposition:        disposition,
		Size:               int64(len(in.Content)),
		CreateTime:         now,
		CreateBy:           in.CreateBy,
	}, nil
}

// DeleteAttachment deletes one attachment row.
func (s *DatabaseStore) DeleteAttachment(ctx context.Context, articleID, fileID int64) error {
	res, err := s.q().ExecContext(ctx, database.ConvertPlaceholders(
		"DELETE FROM article_data_mime_attachment WHERE id = ? AND article_id = ?"), fileID, articleID)
	if err != nil {
		return fmt.Errorf("delete attachment %d of article %d: %w", fileID, articleID, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// WritePlain replaces the article's raw email.
func (s *DatabaseStore) WritePlain(ctx context.Context, articleID int64, raw []byte, createBy int) error {
	if createBy == 0 {
		createBy = systemUserID
	}
	if _, err := s.q().ExecContext(ctx, database.ConvertPlaceholders(
		"DELETE FROM article_data_mime_plain WHERE article_id = ?"), articleID); err != nil {
		return fmt.Errorf("replace plain email of article %d: %w", articleID, err)
	}
	now := time.Now()
	if _, err := s.q().ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO article_data_mime_plain (article_id, body, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?)`), articleID, raw, now, createBy, now, createBy); err != nil {
		return fmt.Errorf("write plain email of article %d: %w", articleID, err)
	}
	return nil
}

// ReadPlain returns the article's raw email.
func (s *DatabaseStore) ReadPlain(ctx context.Context, articleID int64) ([]byte, error) {
	var raw []byte
	err := s.q().QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT body FROM article_data_mime_plain WHERE article_id = ? ORDER BY id DESC LIMIT 1"), articleID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read plain email of article %d: %w", articleID, err)
	}
	return raw, nil
}

// DeleteArticle removes the article's attachments and raw email rows.
func (s *DatabaseStore) DeleteArticle(ctx context.Context, articleID int64) error {
	if _, err := s.q().ExecContext(ctx, database.ConvertPlaceholders(
		"DELETE FROM article_data_mime_attachment WHERE article_id = ?"), articleID); err != nil {
		return fmt.Errorf("delete attachments of article %d: %w", articleID, err)
	}
	if _, err := s.q().ExecContext(ctx, database.ConvertPlaceholders(
		"DELETE FROM article_data_mime_plain WHERE article_id = ?"), articleID); err != nil {
		return fmt.Errorf("delete plain email of article %d: %w", articleID, err)
	}
	return nil
}

// Stats counts attachment and plain-email rows and their bytes.
func (s *DatabaseStore) Stats(ctx context.Context) (files, size int64, err error) {
	var n, b int64
	if err = s.q().QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT COUNT(*), COALESCE(SUM(octet_length(content)), 0)
		FROM article_data_mime_attachment`)).Scan(&n, &b); err != nil {
		return 0, 0, fmt.Errorf("count attachments: %w", err)
	}
	files, size = n, b
	if err = s.q().QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT COUNT(*), COALESCE(SUM(octet_length(body)), 0)
		FROM article_data_mime_plain`)).Scan(&n, &b); err != nil {
		return 0, 0, fmt.Errorf("count plain emails: %w", err)
	}
	return files + n, size + b, nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
