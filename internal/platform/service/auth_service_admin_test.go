package service

import (
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// A failed admin-group lookup must fail token issuance: a token issued with
// a guessed (non-admin) role would silently demote an administrator.
func TestIssueTokensFailsWhenAdminLookupFails(t *testing.T) {
	db, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/none")
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close() // every query now fails with "sql: database is closed"

	svc := NewAuthService(db, testJWTManager(t), nil, nil)
	access, refresh, err := svc.issueTokens(&platformmodels.User{ID: 7, Login: "agent7", Role: "Agent"})
	if err == nil {
		t.Fatalf("expected error, got tokens %q / %q", access, refresh)
	}
	if access != "" || refresh != "" {
		t.Fatalf("no tokens may be issued on lookup failure, got %q / %q", access, refresh)
	}

	svc = NewAuthService(nil, testJWTManager(t), nil, nil)
	if _, _, err := svc.issueTokens(&platformmodels.User{ID: 7, Login: "agent7", Role: "Agent"}); err == nil {
		t.Fatal("expected error without a database")
	}
}
