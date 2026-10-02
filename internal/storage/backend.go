// Package storage stores article attachments and raw (plain) emails, following
// OTRS/Znuny's Ticket::Article::Backend::MIMEBase::ArticleStorage semantics.
//
// Two backends exist:
//   - DB (ArticleStorageDB): article_data_mime_attachment / article_data_mime_plain.
//   - FS (ArticleStorageFS): <article dir>/<content_path>/<article_id>/<file>, with
//     <file>.content_type, .content_id, .content_alternative and .disposition
//     sidecar files and plain.txt for the raw email. An existing OTRS/Znuny
//     var/article tree can be mounted and read as-is.
//
// The article body stays in article_data_mime.a_body for both backends, exactly
// as in OTRS; it is not handled here.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Backend names.
const (
	BackendDB = "DB"
	BackendFS = "FS"
)

// ErrNotFound is returned when an article, attachment or plain email does not exist.
var ErrNotFound = errors.New("not found")

// Attachment describes one attachment of an article (an OTRS attachment index entry).
type Attachment struct {
	ArticleID int64
	// FileID addresses the attachment within its article: the
	// article_data_mime_attachment id for DB, the 1-based OTRS index (position
	// in the sorted file list) for FS.
	FileID             int64
	Filename           string
	ContentType        string
	ContentID          string
	ContentAlternative string
	Disposition        string
	Size               int64
	// CreateTime is the row's create_time (DB) or the file's modification time (FS).
	CreateTime time.Time
	// CreateBy is the creating user (DB); 0 for FS, which does not record it.
	CreateBy int
}

// NewAttachment is an attachment to be written.
type NewAttachment struct {
	Filename           string
	ContentType        string
	ContentID          string
	ContentAlternative string
	Disposition        string
	Content            []byte
	CreateBy           int
}

// ArticleStore reads and writes article attachments and raw emails.
type ArticleStore interface {
	// Backend returns BackendDB or BackendFS.
	Backend() string

	// ListAttachments returns the article's attachments in index order.
	ListAttachments(ctx context.Context, articleID int64) ([]Attachment, error)

	// GetAttachment returns one attachment and its content, or ErrNotFound.
	GetAttachment(ctx context.Context, articleID, fileID int64) (Attachment, []byte, error)

	// WriteAttachment stores an attachment. A filename already used by the
	// article gets an OTRS-style "-N" suffix.
	WriteAttachment(ctx context.Context, articleID int64, a NewAttachment) (Attachment, error)

	// DeleteAttachment removes one attachment, or returns ErrNotFound.
	DeleteAttachment(ctx context.Context, articleID, fileID int64) error

	// WritePlain stores the raw email of an article, replacing any previous one.
	WritePlain(ctx context.Context, articleID int64, raw []byte, createBy int) error

	// ReadPlain returns the raw email of an article, or ErrNotFound.
	ReadPlain(ctx context.Context, articleID int64) ([]byte, error)

	// DeleteArticle removes every attachment and the raw email of an article.
	DeleteArticle(ctx context.Context, articleID int64) error

	// WithTx returns a store whose database work runs on tx.
	WithTx(tx *sql.Tx) ArticleStore
}

// querier is the subset of *sql.DB / *sql.Tx used by the backends.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// ContentPath returns the OTRS article content path ("YYYY/MM/DD") for t.
func ContentPath(t time.Time) string {
	return t.Format("2006/01/02")
}

// normalizeAttachment applies the OTRS write-time rules shared by both backends.
func normalizeAttachment(a NewAttachment) NewAttachment {
	// Content-ID in angle brackets.
	if id := a.ContentID; id != "" && !strings.HasPrefix(id, "<") && !strings.HasSuffix(id, ">") {
		a.ContentID = "<" + id + ">"
	}
	// Only the disposition type is kept ("inline; filename=x" -> "inline").
	if i := strings.IndexByte(a.Disposition, ';'); i >= 0 {
		a.Disposition = a.Disposition[:i]
	}
	a.Disposition = strings.TrimSpace(a.Disposition)
	if a.ContentType == "" {
		a.ContentType = "application/octet-stream"
	}
	if a.CreateBy == 0 {
		a.CreateBy = systemUserID
	}
	return a
}

// systemUserID is the OTRS system user (root@localhost), used when no creator is given.
const systemUserID = 1

// defaultDisposition is OTRS's read-time default when no disposition is stored.
func defaultDisposition(filename, contentType, contentID string) string {
	switch {
	case contentID != "" && strings.Contains(strings.ToLower(contentType), "image"):
		return "inline"
	case strings.Contains(filename, "file-1") || strings.Contains(filename, "file-2"):
		return "inline"
	default:
		return "attachment"
	}
}

// uniqueFilename returns name, or the first OTRS-style "name-N.ext" variant not in used.
func uniqueFilename(name string, used map[string]bool) string {
	if !used[name] {
		return name
	}
	base, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > 0 && i < len(name)-1 {
		base, ext = name[:i], name[i:]
	}
	for n := 1; ; n++ {
		candidate := base + "-" + strconv.Itoa(n) + ext
		if !used[candidate] {
			return candidate
		}
	}
}

// HTMLBodyFilename is the attachment name GoatFlow uses for an article's HTML
// body (OTRS uses "file-2").
const HTMLBodyFilename = "html-body.html"

// IsHTMLBody reports whether an attachment is the article's HTML body rather
// than a user-visible attachment (OTRS ExcludeHTMLBody: an inline text/html
// part named file-2, or GoatFlow's html-body.html).
func IsHTMLBody(a Attachment) bool {
	return strings.EqualFold(a.Disposition, "inline") &&
		strings.HasPrefix(strings.ToLower(a.ContentType), "text/html") &&
		(a.Filename == HTMLBodyFilename || a.Filename == "file-2" || a.Filename == "")
}
