package tasks

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
)

func TestCleanupFailedEmailsDeletesOnlyOld(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := mailqueue.NewMailQueueRepository(db)
	task := &EmailQueueTask{repo: repo, cfg: &config.EmailConfig{Enabled: true}, logger: log.New(io.Discard, "", 0)}

	ctx := context.Background()
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	recentTime := time.Now().Add(-48 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "insert_fingerprint", "article_id", "attempts", "sender", "recipient",
		"raw_message", "due_time", "last_smtp_code", "last_smtp_message", "create_time",
	}).
		AddRow(int64(1), nil, nil, MaxRetries, nil, "old@example.com", []byte("raw"), nil, nil, "fail", oldTime).
		AddRow(int64(2), nil, nil, MaxRetries, nil, "recent@example.com", []byte("raw"), nil, nil, "fail", recentTime)

	mock.ExpectQuery("SELECT id, insert_fingerprint.*FROM mail_queue.*WHERE attempts >= .*ORDER BY create_time ASC.*LIMIT ?").
		WithArgs(MaxRetries, 100).
		WillReturnRows(rows)

	mock.ExpectExec("DELETE FROM mail_queue WHERE id = ?").
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := task.cleanupFailedEmails(ctx); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCleanupFailedEmailsSkipsNonMaxAttempts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := mailqueue.NewMailQueueRepository(db)
	task := &EmailQueueTask{repo: repo, cfg: &config.EmailConfig{Enabled: true}, logger: log.New(io.Discard, "", 0)}

	ctx := context.Background()
	createTime := time.Now().Add(-10 * 24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "insert_fingerprint", "article_id", "attempts", "sender", "recipient",
		"raw_message", "due_time", "last_smtp_code", "last_smtp_message", "create_time",
	}).
		AddRow(int64(1), nil, nil, MaxRetries-1, nil, "keep@example.com", []byte("raw"), nil, nil, "fail", createTime)

	mock.ExpectQuery("SELECT id, insert_fingerprint.*FROM mail_queue.*WHERE attempts >= .*ORDER BY create_time ASC.*LIMIT ?").
		WithArgs(MaxRetries, 100).
		WillReturnRows(rows)

	if err := task.cleanupFailedEmails(ctx); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
