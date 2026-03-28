package selfservice

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Test database not available")
	}
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'gk_auth_token'",
	).Scan(&count)
	if err != nil || count == 0 {
		t.Skipf("gk_auth_token table not found — run migration 000014")
	}
	return db
}

func cleanupTestTokens(t *testing.T, db *sql.DB, email string) {
	t.Helper()
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_auth_token WHERE email = ?"), email)
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_registration_request WHERE email = ?"), email)
}

func TestAuthToken_Validity(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		token := &AuthToken{
			ExpiresAt: time.Now().Add(1 * time.Hour),
			UsedAt:    nil,
		}
		if !token.IsValid() {
			t.Error("expected valid")
		}
		if token.IsExpired() {
			t.Error("should not be expired")
		}
		if token.IsUsed() {
			t.Error("should not be used")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		token := &AuthToken{
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		if token.IsValid() {
			t.Error("expired token should not be valid")
		}
		if !token.IsExpired() {
			t.Error("should be expired")
		}
	})

	t.Run("used token", func(t *testing.T) {
		now := time.Now()
		token := &AuthToken{
			ExpiresAt: time.Now().Add(1 * time.Hour),
			UsedAt:    &now,
		}
		if token.IsValid() {
			t.Error("used token should not be valid")
		}
		if !token.IsUsed() {
			t.Error("should be used")
		}
	})
}

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("token length = %d, want 64", len(token1))
	}

	token2, _ := GenerateToken()
	if token1 == token2 {
		t.Error("tokens should be unique")
	}
}

func TestRepository_TokenLifecycle(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	testEmail := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestTokens(t, db, testEmail) })

	token, _ := GenerateToken()

	t.Run("create and get token", func(t *testing.T) {
		err := repo.CreateToken(&AuthToken{
			Token:     token,
			TokenType: TokenPasswordReset,
			UserType:  UserCustomer,
			Email:     testEmail,
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("CreateToken: %v", err)
		}

		got, err := repo.GetToken(token)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatal("expected non-nil token")
		}
		if got.Email != testEmail {
			t.Errorf("email = %q", got.Email)
		}
		if got.TokenType != TokenPasswordReset {
			t.Errorf("type = %q", got.TokenType)
		}
		if !got.IsValid() {
			t.Error("token should be valid")
		}
	})

	t.Run("consume token", func(t *testing.T) {
		err := repo.ConsumeToken(token)
		if err != nil {
			t.Fatal(err)
		}

		got, _ := repo.GetToken(token)
		if got == nil {
			t.Fatal("token should still exist")
		}
		if !got.IsUsed() {
			t.Error("token should be used")
		}
		if got.IsValid() {
			t.Error("used token should not be valid")
		}
	})

	t.Run("double consume fails", func(t *testing.T) {
		err := repo.ConsumeToken(token)
		if err == nil {
			t.Error("expected error on double consume")
		}
	})

	t.Run("get nonexistent token", func(t *testing.T) {
		got, err := repo.GetToken("nonexistent_token_xyz")
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Error("expected nil")
		}
	})
}

func TestRepository_RegistrationLifecycle(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	testEmail := fmt.Sprintf("reg_%d@example.com", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestTokens(t, db, testEmail) })

	t.Run("create and list pending", func(t *testing.T) {
		approvalToken, _ := GenerateToken()
		id, err := repo.CreateRegistration(&RegistrationRequest{
			Email:         testEmail,
			FirstName:     "Test",
			LastName:      "User",
			Status:        StatusPending,
			ApprovalToken: &approvalToken,
			CreatedAt:     time.Now(),
		})
		if err != nil {
			t.Fatalf("CreateRegistration: %v", err)
		}
		if id == 0 {
			t.Fatal("expected non-zero ID")
		}

		pending, err := repo.ListPendingRegistrations()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range pending {
			if r.Email == testEmail {
				found = true
			}
		}
		if !found {
			t.Error("expected registration in pending list")
		}
	})

	t.Run("approve registration", func(t *testing.T) {
		pending, _ := repo.ListPendingRegistrations()
		var regID int64
		for _, r := range pending {
			if r.Email == testEmail {
				regID = r.ID
			}
		}
		if regID == 0 {
			t.Skip("no pending registration found")
		}

		err := repo.ApproveRegistration(regID, 1)
		if err != nil {
			t.Fatal(err)
		}

		got, _ := repo.GetRegistration(regID)
		if got == nil || got.Status != StatusApproved {
			t.Error("expected approved status")
		}
	})
}

func TestRepository_RejectRegistration(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	testEmail := fmt.Sprintf("rej_%d@example.com", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestTokens(t, db, testEmail) })

	id, _ := repo.CreateRegistration(&RegistrationRequest{
		Email: testEmail, FirstName: "Reject", LastName: "Me",
		Status: StatusPending, CreatedAt: time.Now(),
	})

	err := repo.RejectRegistration(id, "spam account", 1)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := repo.GetRegistration(id)
	if got == nil || got.Status != StatusRejected {
		t.Error("expected rejected status")
	}
	if got.RejectedReason == nil || *got.RejectedReason != "spam account" {
		t.Error("expected rejection reason")
	}
}

func TestVerifyCAPTCHA_Disabled(t *testing.T) {
	// Nil config = disabled.
	err := VerifyCAPTCHA(nil, "anything")
	if err != nil {
		t.Errorf("disabled CAPTCHA should pass: %v", err)
	}

	// Empty provider = disabled.
	err = VerifyCAPTCHA(&CAPTCHAConfig{}, "anything")
	if err != nil {
		t.Errorf("empty provider should pass: %v", err)
	}
}

func TestVerifyCAPTCHA_UnsupportedProvider(t *testing.T) {
	err := VerifyCAPTCHA(&CAPTCHAConfig{Provider: "bogus", SecretKey: "key"}, "token")
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}
