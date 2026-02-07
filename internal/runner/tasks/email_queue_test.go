package tasks

import (
	"context"
	"errors"
	"io"
	"log"
	"net/textproto"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/mailqueue"
)

func TestConstants(t *testing.T) {
	if MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", MaxRetries)
	}
	if RetryDelayBase != 5 {
		t.Errorf("expected RetryDelayBase 5, got %d", RetryDelayBase)
	}
}

func TestEmailQueueTask_Name(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping test")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping test")
	}

	task := NewEmailQueueTask(db, &config.EmailConfig{})

	if name := task.Name(); name != "email-queue-processor" {
		t.Errorf("expected Name 'email-queue-processor', got %s", name)
	}
}

func TestEmailQueueTask_Schedule(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping test")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping test")
	}

	task := NewEmailQueueTask(db, &config.EmailConfig{})

	schedule := task.Schedule()
	if schedule != "*/30 * * * * *" {
		t.Errorf("expected Schedule '*/30 * * * * *', got %s", schedule)
	}
}

func TestEmailQueueTask_Timeout(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping test")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping test")
	}

	task := NewEmailQueueTask(db, &config.EmailConfig{})

	timeout := task.Timeout()
	if timeout != 5*time.Minute {
		t.Errorf("expected Timeout 5m, got %v", timeout)
	}
}

func TestEmailQueueTask_Run_Disabled(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping test")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping test")
	}

	task := &EmailQueueTask{
		repo:   mailqueue.NewMailQueueRepository(db),
		cfg:    &config.EmailConfig{Enabled: false},
		logger: log.New(io.Discard, "", 0),
	}

	err = task.Run(context.Background())
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestEmailQueueTask_Run_NoPendingEmails(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping test")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping test")
	}

	task := &EmailQueueTask{
		repo:   mailqueue.NewMailQueueRepository(db),
		cfg:    &config.EmailConfig{Enabled: true},
		logger: log.New(io.Discard, "", 0),
	}

	// Run with empty queue should succeed
	err = task.Run(context.Background())
	if err != nil {
		t.Errorf("expected no error with empty queue, got %v", err)
	}
}

func TestCleanupFailedEmails_Integration(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping test")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping test")
	}

	ctx := context.Background()
	repo := mailqueue.NewMailQueueRepository(db)
	task := &EmailQueueTask{
		repo:   repo,
		cfg:    &config.EmailConfig{Enabled: true},
		logger: log.New(io.Discard, "", 0),
	}

	// Insert old failed email (> 7 days old with max attempts)
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO mail_queue (recipient, raw_message, attempts, create_time)
		VALUES (?, ?, ?, ?)
	`)
	result, err := db.ExecContext(ctx, insertQuery, "old@test.com", []byte("test message"), MaxRetries, oldTime)
	if err != nil {
		t.Skipf("Could not insert test data: %v", err)
	}
	oldID, _ := result.LastInsertId()

	// Insert recent failed email (< 7 days old)
	recentTime := time.Now().Add(-2 * 24 * time.Hour)
	result, err = db.ExecContext(ctx, insertQuery, "recent@test.com", []byte("test message"), MaxRetries, recentTime)
	if err != nil {
		t.Skipf("Could not insert test data: %v", err)
	}
	recentID, _ := result.LastInsertId()

	// Run cleanup
	err = task.cleanupFailedEmails(ctx)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify old email was deleted
	var count int
	countQuery := database.ConvertPlaceholders("SELECT COUNT(*) FROM mail_queue WHERE id = ?")
	db.QueryRowContext(ctx, countQuery, oldID).Scan(&count)
	if count != 0 {
		t.Error("expected old email to be deleted")
	}

	// Verify recent email still exists
	db.QueryRowContext(ctx, countQuery, recentID).Scan(&count)
	if count != 1 {
		t.Error("expected recent email to still exist")
	}

	// Cleanup test data
	deleteQuery := database.ConvertPlaceholders("DELETE FROM mail_queue WHERE id = ?")
	db.ExecContext(ctx, deleteQuery, recentID)
}

func TestCalculateNextRetryTime(t *testing.T) {
	task := &EmailQueueTask{cfg: &config.EmailConfig{}}

	tests := []struct {
		attempts      int
		expectedDelay time.Duration
		expectNil     bool
	}{
		{1, 5 * time.Minute, false},
		{2, 25 * time.Minute, false},
		{3, 125 * time.Minute, false},
		{4, 625 * time.Minute, false},
		{5, 0, true}, // MaxRetries reached
		{6, 0, true},
	}

	for _, tt := range tests {
		result := task.calculateNextRetryTime(tt.attempts)

		if tt.expectNil {
			if result != nil {
				t.Errorf("attempts=%d: expected nil, got %v", tt.attempts, result)
			}
		} else {
			if result == nil {
				t.Errorf("attempts=%d: expected non-nil", tt.attempts)
				continue
			}

			expectedTime := time.Now().Add(tt.expectedDelay)
			diff := result.Sub(expectedTime)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("attempts=%d: expected delay ~%v, got diff %v", tt.attempts, tt.expectedDelay, diff)
			}
		}
	}
}

func TestStringPtr(t *testing.T) {
	s := "test string"
	ptr := stringPtr(s)

	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *ptr != s {
		t.Errorf("expected %q, got %q", s, *ptr)
	}
}

func TestSmtpStatus_TextprotoError(t *testing.T) {
	err := &textproto.Error{Code: 550, Msg: "User unknown"}
	cfg := &config.EmailConfig{}

	code, msg := smtpStatus(err, cfg)

	if code == nil || *code != 550 {
		t.Errorf("expected code 550, got %v", code)
	}
	if msg != "550 User unknown" {
		t.Errorf("expected msg '550 User unknown', got %q", msg)
	}
}

func TestSmtpStatus_EOF(t *testing.T) {
	cfg := &config.EmailConfig{}
	cfg.SMTP.Port = 25

	code, msg := smtpStatus(io.EOF, cfg)

	if code == nil || *code != 421 {
		t.Errorf("expected code 421, got %v", code)
	}
	if msg != "421 unexpected EOF" {
		t.Errorf("expected '421 unexpected EOF', got %q", msg)
	}
}

func TestSmtpStatus_EOF_TestPort(t *testing.T) {
	cfg := &config.EmailConfig{}
	cfg.SMTP.Port = 25253

	code, msg := smtpStatus(io.EOF, cfg)

	if code == nil || *code != 550 {
		t.Errorf("expected code 550 for test port, got %v", code)
	}
	if msg != "550 unexpected EOF" {
		t.Errorf("expected '550 unexpected EOF', got %q", msg)
	}
}

func TestSmtpStatus_OtherError(t *testing.T) {
	err := errors.New("connection refused")
	cfg := &config.EmailConfig{}

	code, msg := smtpStatus(err, cfg)

	if code != nil {
		t.Errorf("expected nil code, got %v", code)
	}
	if msg != "connection refused" {
		t.Errorf("expected 'connection refused', got %q", msg)
	}
}

func TestSendError(t *testing.T) {
	originalErr := errors.New("network error")
	code := 421
	se := &sendError{code: &code, err: originalErr}

	if se.Error() != "network error" {
		t.Errorf("expected Error() 'network error', got %q", se.Error())
	}

	if se.Unwrap() != originalErr {
		t.Error("expected Unwrap() to return original error")
	}
}

func TestSendError_NilCode(t *testing.T) {
	originalErr := errors.New("timeout")
	se := &sendError{code: nil, err: originalErr}

	if se.Error() != "timeout" {
		t.Errorf("expected Error() 'timeout', got %q", se.Error())
	}
}

func TestLoginAuth_Start(t *testing.T) {
	auth := &loginAuth{username: "user@test.com", password: "secret"}

	proto, resp, err := auth.Start(nil)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if proto != "LOGIN" {
		t.Errorf("expected proto 'LOGIN', got %q", proto)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty response, got %v", resp)
	}
}

func TestLoginAuth_Next(t *testing.T) {
	auth := &loginAuth{username: "user@test.com", password: "secret123"}

	tests := []struct {
		fromServer string
		more       bool
		expected   string
		expectErr  bool
	}{
		{"Username:", true, "user@test.com", false},
		{"Password:", true, "secret123", false},
		{"Unknown:", true, "", true},
		{"", false, "", false},
	}

	for _, tt := range tests {
		resp, err := auth.Next([]byte(tt.fromServer), tt.more)

		if tt.expectErr {
			if err == nil {
				t.Errorf("fromServer=%q: expected error", tt.fromServer)
			}
		} else {
			if err != nil {
				t.Errorf("fromServer=%q: unexpected error: %v", tt.fromServer, err)
			}
			if string(resp) != tt.expected {
				t.Errorf("fromServer=%q: expected %q, got %q", tt.fromServer, tt.expected, string(resp))
			}
		}
	}
}
