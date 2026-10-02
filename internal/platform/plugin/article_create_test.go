package plugin

import (
	"context"
	"database/sql"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// articleCreateFixture is a real ticket plus the agent that writes on it.
type articleCreateFixture struct {
	h        *ProdHostAPI
	db       *sql.DB
	userID   int64
	ticketID int64
}

func newArticleCreateFixture(t *testing.T) *articleCreateFixture {
	t.Helper()
	db := requireHostTestDB(t)
	f := &articleCreateFixture{db: db}
	f.userID = insertHostTestUser(t, db)
	f.ticketID = insertHostTestTicket(t, db, seededStateNew)
	f.h = NewProdHostAPI(WithDB("default", db))
	return f
}

func TestCreateArticle(t *testing.T) {
	f := newArticleCreateFixture(t)
	ctx := context.Background()

	id, err := f.h.CreateArticle(ctx, f.ticketID, f.userID, "Action Items", "# Heading\n\n- item", true)
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive article id, got %d", id)
	}

	var ticketID, visible, sender, channel, createBy, changeBy int64
	if err := f.db.QueryRow(database.ConvertPlaceholders(`SELECT ticket_id, is_visible_for_customer, article_sender_type_id, communication_channel_id, create_by, change_by FROM article WHERE id = ?`), id).
		Scan(&ticketID, &visible, &sender, &channel, &createBy, &changeBy); err != nil {
		t.Fatalf("load article: %v", err)
	}
	if ticketID != f.ticketID {
		t.Errorf("article on ticket %d, want %d", ticketID, f.ticketID)
	}
	if visible != 1 {
		t.Errorf("expected visible_to_customer=1, got %d", visible)
	}
	if sender != 1 || channel != 3 {
		t.Errorf("expected agent sender (1) + internal channel (3), got sender=%d channel=%d", sender, channel)
	}
	if createBy != f.userID || changeBy != f.userID {
		t.Errorf("article create_by=%d change_by=%d, want %d", createBy, changeBy, f.userID)
	}

	var subject, body, ct string
	if err := f.db.QueryRow(database.ConvertPlaceholders(`SELECT a_subject, a_body, a_content_type FROM article_data_mime WHERE article_id = ?`), id).
		Scan(&subject, &body, &ct); err != nil {
		t.Fatalf("load mime: %v", err)
	}
	if subject != "Action Items" || body != "# Heading\n\n- item" || ct != "text/plain" {
		t.Errorf("unexpected mime: subject=%q body=%q ct=%q", subject, body, ct)
	}
}

func TestCreateArticleInvisible(t *testing.T) {
	f := newArticleCreateFixture(t)
	ctx := context.Background()

	id, err := f.h.CreateArticle(ctx, f.ticketID, f.userID, "Draft", "internal only", false)
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	var visible int
	if err := f.db.QueryRow(database.ConvertPlaceholders(`SELECT is_visible_for_customer FROM article WHERE id = ?`), id).Scan(&visible); err != nil {
		t.Fatalf("load article: %v", err)
	}
	if visible != 0 {
		t.Errorf("expected visible_to_customer=0, got %d", visible)
	}
}

func TestCreateArticleMissingTicket(t *testing.T) {
	f := newArticleCreateFixture(t)
	ctx := context.Background()

	missing := unusedID(t, f.db, "ticket")
	if _, err := f.h.CreateArticle(ctx, missing, f.userID, "S", "b", false); err == nil {
		t.Fatal("expected error for missing ticket")
	}
	var n int
	if err := f.db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM article WHERE ticket_id = ?`), missing).Scan(&n); err != nil {
		t.Fatalf("count articles: %v", err)
	}
	if n != 0 {
		t.Fatalf("no article may be written for a missing ticket, got %d", n)
	}
}

func TestCreateArticleSanitizesAstralRunes(t *testing.T) {
	f := newArticleCreateFixture(t)
	ctx := context.Background()

	// 4-byte rune (astral plane) must be stripped for utf8mb3 article_data_mime.
	body := "intro \U0001F600 out"
	id, err := f.h.CreateArticle(ctx, f.ticketID, f.userID, "s \U0001F600", body, false)
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	var subject, stored string
	if err := f.db.QueryRow(database.ConvertPlaceholders(`SELECT a_subject, a_body FROM article_data_mime WHERE article_id = ?`), id).Scan(&subject, &stored); err != nil {
		t.Fatalf("load mime: %v", err)
	}
	if stored != "intro  out" || subject != "s " {
		t.Errorf("expected astral rune stripped, got subject=%q body=%q", subject, stored)
	}
}
