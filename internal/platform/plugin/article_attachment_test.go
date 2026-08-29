package plugin

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// newAttachmentTestHost boots a ProdHostAPI over an in-memory SQLite DB with
// the article + attachment tables a plugin interacts with.
func newAttachmentTestHost(t *testing.T) *ProdHostAPI {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ddl := []string{
		`CREATE TABLE article (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			article_sender_type_id INTEGER,
			communication_channel_id INTEGER,
			is_visible_for_customer INTEGER,
			insert_fingerprint TEXT,
			create_time TEXT,
			create_by INTEGER,
			change_time TEXT,
			change_by INTEGER)`,
		`CREATE TABLE article_data_mime_attachment (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id INTEGER NOT NULL,
			filename TEXT,
			content_size TEXT,
			content_type TEXT,
			disposition TEXT,
			content BLOB,
			create_time TEXT,
			create_by INTEGER,
			change_time TEXT,
			change_by INTEGER)`,
	}
	for _, q := range ddl {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("create table: %v (%s)", err, q)
		}
	}
	if _, err := db.Exec(`INSERT INTO article (ticket_id, create_by, change_by) VALUES (1, 1, 1)`); err != nil {
		t.Fatalf("seed article: %v", err)
	}
	return NewProdHostAPI(WithDB("default", db))
}

func TestCreateArticleAttachment(t *testing.T) {
	h := newAttachmentTestHost(t)
	ctx := context.Background()

	id, err := h.CreateArticleAttachment(ctx, 1, 7, "invite.ics", "text/calendar", []byte("BEGIN:VCALENDAR"))
	if err != nil {
		t.Fatalf("CreateArticleAttachment: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive attachment id, got %d", id)
	}

	atts, err := h.ListArticleAttachments(ctx, 1)
	if err != nil {
		t.Fatalf("ListArticleAttachments: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(atts))
	}
	a := atts[0]
	if a.Filename != "invite.ics" || a.ContentType != "text/calendar" || a.Size != int64(len("BEGIN:VCALENDAR")) || a.CreatedBy != 7 {
		t.Errorf("unexpected attachment metadata: %+v", a)
	}
}

func TestCreateArticleAttachmentMissingArticle(t *testing.T) {
	h := newAttachmentTestHost(t)
	_, err := h.CreateArticleAttachment(context.Background(), 9999, 7, "a.bin", "application/octet-stream", []byte("x"))
	if err == nil {
		t.Fatal("expected error for missing article")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateArticleAttachmentSizeLimit(t *testing.T) {
	h := newAttachmentTestHost(t)
	// Larger than the implicit 10 MiB default.
	_, err := h.CreateArticleAttachment(context.Background(), 1, 7, "big.bin", "application/octet-stream", make([]byte, 11<<20))
	if err == nil || !strings.Contains(err.Error(), "exceeds size limit") {
		t.Fatalf("expected size-limit error, got %v", err)
	}
}

func TestDeleteArticleAttachment(t *testing.T) {
	h := newAttachmentTestHost(t)
	ctx := context.Background()
	id, err := h.CreateArticleAttachment(ctx, 1, 7, "photo.jpg", "image/jpeg", []byte("jpeg"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := h.DeleteArticleAttachment(ctx, 1, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	atts, _ := h.ListArticleAttachments(ctx, 1)
	if len(atts) != 0 {
		t.Errorf("expected 0 attachments after delete, got %d", len(atts))
	}
	// Deleting a different article's id or a missing id errors.
	if err := h.DeleteArticleAttachment(ctx, 1, id); err == nil {
		t.Error("expected error deleting an already-deleted attachment")
	}
}
