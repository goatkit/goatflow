package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/storage"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("GOATFLOW_TEST_DB_READY") != "1" {
		t.Skip("needs the test database (GOATFLOW_TEST_DB_READY=1)")
	}
	if err := database.InitTestDB(); err != nil {
		t.Fatalf("init test db: %v", err)
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Fatalf("get test db: %v", err)
	}
	return db
}

func createArticle(t *testing.T, db *sql.DB, created time.Time) int64 {
	t.Helper()
	now := time.Now()
	ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, timeout, until_time, escalation_time,
			escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'cli test', 1, 1, 1, 1, 3, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1) RETURNING id`),
		fmt.Sprintf("cli%d", now.UnixNano()), now, now)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	articleID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 1, 1, ?, 1, ?, 1) RETURNING id`), ticketID, created, now)
	if err != nil {
		t.Fatalf("insert article: %v", err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			"DELETE FROM article_data_mime_attachment WHERE article_id = ?",
			"DELETE FROM article_data_mime_plain WHERE article_id = ?",
			"DELETE FROM article WHERE id = ?",
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), articleID)
		}
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id = ?"), ticketID)
	})
	return articleID
}

type snapshot map[string]string

// contentOf lists an article's items as "name|type|cid|disposition" -> content.
func contentOf(t *testing.T, s storage.ArticleStore, articleID int64) snapshot {
	t.Helper()
	ctx := context.Background()
	list, err := s.ListAttachments(ctx, articleID)
	if err != nil {
		t.Fatalf("%s list %d: %v", s.Backend(), articleID, err)
	}
	snap := snapshot{}
	for _, a := range list {
		meta, content, err := s.GetAttachment(ctx, articleID, a.FileID)
		if err != nil {
			t.Fatalf("%s get %d/%d: %v", s.Backend(), articleID, a.FileID, err)
		}
		snap[strings.Join([]string{meta.Filename, meta.ContentType, meta.ContentID, meta.Disposition}, "|")] = string(content)
	}
	if raw, err := s.ReadPlain(ctx, articleID); err == nil {
		snap["plain.txt"] = string(raw)
	}
	return snap
}

func runCLI(t *testing.T, wantCode int, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := run(args, &out, &errOut); code != wantCode {
		t.Fatalf("goatflow-storage %s: exit %d, want %d\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), code, wantCode, out.String(), errOut.String())
	}
	return out.String()
}

func TestMigrateRoundTrip(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	dbStore := storage.NewDatabaseStore(db)
	fsStore := storage.NewFilesystemStore(dir, db)

	start := time.Now().Add(-time.Second)
	created := time.Date(2025, 11, 30, 10, 0, 0, 0, time.UTC)
	first := createArticle(t, db, created)
	second := createArticle(t, db, created)
	for _, a := range []storage.NewAttachment{
		{Filename: "report.pdf", ContentType: "application/pdf", Disposition: "attachment", Content: []byte("%PDF\x00\xff\x27\x5c")},
		{Filename: "logo.png", ContentType: "image/png", ContentID: "<logo@x>", Disposition: "inline", Content: []byte("\x89PNG")},
		{Filename: "logo.png", ContentType: "image/png", Disposition: "attachment", Content: []byte("\x89PNG other")},
	} {
		if _, err := dbStore.WriteAttachment(ctx, first, a); err != nil {
			t.Fatal(err)
		}
	}
	if err := dbStore.WritePlain(ctx, first, []byte("From: a@example.com\r\n\r\nraw"), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := dbStore.WriteAttachment(ctx, second, storage.NewAttachment{Filename: "notes.txt", ContentType: "text/plain", Content: []byte("n")}); err != nil {
		t.Fatal(err)
	}
	want := map[int64]snapshot{first: contentOf(t, dbStore, first), second: contentOf(t, dbStore, second)}
	if len(want[first]) != 4 || len(want[second]) != 1 {
		t.Fatalf("fixture: %v", want)
	}

	scope := []string{"-article-dir", dir, "-created-after", start.Format(time.RFC3339)}
	args := func(cmd, target string, extra ...string) []string {
		return append(append([]string{cmd, "-target", target}, scope...), extra...)
	}

	out := runCLI(t, 0, args("migrate", "FS")...)
	if !strings.Contains(out, "5 items copied, 0 already present") {
		t.Fatalf("first migrate to FS:\n%s", out)
	}
	for id, snap := range want {
		if got := contentOf(t, fsStore, id); fmt.Sprint(got) != fmt.Sprint(snap) {
			t.Fatalf("FS copy of article %d:\n got  %v\n want %v", id, got, snap)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "2025", "11", "30", fmt.Sprint(first), "report.pdf.content_type")); err != nil {
		t.Fatalf("OTRS layout: %v", err)
	}
	runCLI(t, 0, args("verify", "FS")...)

	out = runCLI(t, 0, args("migrate", "FS")...)
	if !strings.Contains(out, "0 items copied, 5 already present") {
		t.Fatalf("re-run migrate to FS is not idempotent:\n%s", out)
	}

	out = runCLI(t, 0, args("migrate", "FS", "-delete-source")...)
	if !strings.Contains(out, "0 items copied, 5 already present, 2 source copies deleted") {
		t.Fatalf("migrate -delete-source:\n%s", out)
	}
	if got := contentOf(t, dbStore, first); len(got) != 0 {
		t.Fatalf("DB still holds article %d after -delete-source: %v", first, got)
	}
	out = runCLI(t, 1, args("verify", "DB")...)
	if !strings.Contains(out, "5 missing") {
		t.Fatalf("verify DB after delete-source:\n%s", out)
	}

	out = runCLI(t, 0, args("migrate", "DB")...)
	if !strings.Contains(out, "5 items copied, 0 already present") {
		t.Fatalf("migrate back to DB:\n%s", out)
	}
	for id, snap := range want {
		if got := contentOf(t, dbStore, id); fmt.Sprint(got) != fmt.Sprint(snap) {
			t.Fatalf("DB copy of article %d after round trip:\n got  %v\n want %v", id, got, snap)
		}
	}
	runCLI(t, 0, args("verify", "DB")...)
	out = runCLI(t, 0, args("migrate", "DB")...)
	if !strings.Contains(out, "0 items copied, 5 already present") {
		t.Fatalf("re-run migrate to DB is not idempotent:\n%s", out)
	}

	out = runCLI(t, 0, "status", "-article-dir", dir)
	if !strings.Contains(out, "Filesystem (ArticleStorageFS): 5 files") {
		t.Fatalf("status:\n%s", out)
	}
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	start := time.Now().Add(-time.Second)
	id := createArticle(t, db, time.Now())
	if _, err := storage.NewDatabaseStore(db).WriteAttachment(ctx, id, storage.NewAttachment{Filename: "a.txt", Content: []byte("a")}); err != nil {
		t.Fatal(err)
	}
	out := runCLI(t, 0, "migrate", "-target", "FS", "-dry-run", "-article-dir", dir,
		"-created-after", start.Format(time.RFC3339))
	if !strings.Contains(out, fmt.Sprintf("article %d: copy attachment \"a.txt\" to FS", id)) {
		t.Fatalf("dry run output:\n%s", out)
	}
	if got := contentOf(t, storage.NewFilesystemStore(dir, db), id); len(got) != 0 {
		t.Fatalf("dry run wrote %v", got)
	}
}

func TestUsageErrors(t *testing.T) {
	runCLI(t, 2)
	runCLI(t, 2, "frobnicate")
	runCLI(t, 2, "migrate", "-target", "S3")
}
