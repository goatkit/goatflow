package auth

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/goatkit/goatflow/internal/database"
)

func TestTOTPSessionCanBeLoadedFromDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	database.SetDB(db)
	t.Cleanup(database.ResetDB)

	manager := NewTOTPSessionManager([]byte("01234567890123456789012345678901"))
	mock.ExpectExec("INSERT INTO gk_totp_pending_session").
		WithArgs(
			sqlmock.AnyArg(),
			42,
			"",
			"nigel",
			false,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			0,
			MaxTOTPAttempts,
			"198.51.100.10",
			"test-agent",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	token, err := manager.CreateAgentSession(42, "nigel", "198.51.100.10", "test-agent")
	if err != nil {
		t.Fatalf("CreateAgentSession: %v", err)
	}

	manager.sessions = map[string]*PendingTOTPSession{}
	created := time.Now().Add(-time.Minute)
	expires := time.Now().Add(time.Minute)
	mock.ExpectQuery("SELECT user_id, user_login, username, is_customer").
		WithArgs(totpSessionKey(token)).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "user_login", "username", "is_customer", "created_at", "expires_at",
			"attempts", "max_attempts", "client_ip", "user_agent",
		}).AddRow(42, "", "nigel", false, created, expires, 0, MaxTOTPAttempts, "198.51.100.10", "test-agent"))

	session := manager.ValidateAndGetSession(token, "198.51.100.10", "test-agent")
	if session == nil {
		t.Fatal("ValidateAndGetSession did not load DB-backed session")
	}
	if session.UserID != 42 || session.Username != "nigel" || session.IsCustomer {
		t.Fatalf("session = userID %d username %q customer %v, want 42/nigel/false", session.UserID, session.Username, session.IsCustomer)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
