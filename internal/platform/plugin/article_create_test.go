package plugin

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// newArticleCreateTestHost boots a ProdHostAPI over an in-memory SQLite DB
// with the ticket + article + mime tables CreateArticle touches.
func newArticleCreateTestHost(t *testing.T) *ProdHostAPI {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ddl := []string{
		`CREATE TABLE ticket (id INTEGER PRIMARY KEY, customer_user_id TEXT)`,
		`CREATE TABLE article (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			article_sender_type_id INTEGER,
			communication_channel_id INTEGER,
			is_visible_for_customer INTEGER,
			create_time DATETIME,
			create_by INTEGER,
			change_time DATETIME,
			change_by INTEGER)`,
		`CREATE TABLE article_data_mime (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id INTEGER NOT NULL,
			a_from TEXT,
			a_subject TEXT,
			a_body TEXT,
			a_content_type TEXT,
			incoming_time INTEGER,
			create_time DATETIME,
			create_by INTEGER,
			change_time DATETIME,
			change_by INTEGER)`,
	}
	for _, q := range ddl {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("exec ddl %q: %v", q, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO ticket (id, customer_user_id) VALUES (1, 'j@x.com')`); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}
	return NewProdHostAPI(WithDB("default", db))
}

func TestCreateArticle(t *testing.T) {
	h := newArticleCreateTestHost(t)
	ctx := context.Background()

	id, err := h.CreateArticle(ctx, 1, 7, "Action Items", "# Heading\n\n- item", true)
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive article id, got %d", id)
	}

	db, _ := h.getDB("")
	var visible int
	var sender, channel int
	if err := db.QueryRow(`SELECT is_visible_for_customer, article_sender_type_id, communication_channel_id FROM article WHERE id = ?`, id).
		Scan(&visible, &sender, &channel); err != nil {
		t.Fatalf("load article: %v", err)
	}
	if visible != 1 {
		t.Errorf("expected visible_to_customer=1, got %d", visible)
	}
	if sender != 1 || channel != 3 {
		t.Errorf("expected agent sender (1) + internal channel (3), got sender=%d channel=%d", sender, channel)
	}

	var subject, body, ct string
	if err := db.QueryRow(`SELECT a_subject, a_body, a_content_type FROM article_data_mime WHERE article_id = ?`, id).
		Scan(&subject, &body, &ct); err != nil {
		t.Fatalf("load mime: %v", err)
	}
	if subject != "Action Items" || body != "# Heading\n\n- item" || ct != "text/plain" {
		t.Errorf("unexpected mime: subject=%q body=%q ct=%q", subject, body, ct)
	}
}

func TestCreateArticleInvisible(t *testing.T) {
	h := newArticleCreateTestHost(t)
	ctx := context.Background()

	id, err := h.CreateArticle(ctx, 1, 7, "Draft", "internal only", false)
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	db, _ := h.getDB("")
	var visible int
	if err := db.QueryRow(`SELECT is_visible_for_customer FROM article WHERE id = ?`, id).Scan(&visible); err != nil {
		t.Fatalf("load article: %v", err)
	}
	if visible != 0 {
		t.Errorf("expected visible_to_customer=0, got %d", visible)
	}
}

func TestCreateArticleMissingTicket(t *testing.T) {
	h := newArticleCreateTestHost(t)
	ctx := context.Background()

	if _, err := h.CreateArticle(ctx, 999, 7, "S", "b", false); err == nil {
		t.Fatal("expected error for missing ticket")
	}
}

func TestCreateArticleSanitizesAstralRunes(t *testing.T) {
	h := newArticleCreateTestHost(t)
	ctx := context.Background()

	// 4-byte rune (astral plane) must be stripped for utf8mb3 article_data_mime.
	body := "intro \U0001F600 out"
	id, err := h.CreateArticle(ctx, 1, 7, "s", body, false)
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	db, _ := h.getDB("")
	var stored string
	if err := db.QueryRow(`SELECT a_body FROM article_data_mime WHERE article_id = ?`, id).Scan(&stored); err != nil {
		t.Fatalf("load mime: %v", err)
	}
	if stored != "intro  out" {
		t.Errorf("expected astral rune stripped, got %q", stored)
	}
}
