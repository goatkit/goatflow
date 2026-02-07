// +build integration

package acl

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/goatkit/goatflow/internal/models"
)

// TestACLIntegration tests ACL loading and evaluation against a real database.
// Run with: go test -tags=integration -v ./internal/services/acl/...
func TestACLIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "root:changeme@tcp(localhost:3306)/otrs?parseTime=true"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: cannot connect to database: %v", err)
	}

	svc := NewService(db)
	ctx := context.Background()

	// Refresh cache to load ACLs
	if err := svc.RefreshCache(ctx); err != nil {
		t.Fatalf("Failed to refresh ACL cache: %v", err)
	}

	t.Run("ACLs loaded from database", func(t *testing.T) {
		acls, err := svc.getACLs(ctx)
		if err != nil {
			t.Fatalf("Failed to get ACLs: %v", err)
		}
		if len(acls) == 0 {
			t.Fatal("Expected at least one ACL to be loaded")
		}
		t.Logf("Loaded %d ACL(s)", len(acls))
		for _, acl := range acls {
			t.Logf("  - %s (ID: %d, StopAfterMatch: %v)", acl.Name, acl.ID, acl.StopAfterMatch)
		}
	})

	t.Run("FilterOptions with matching context", func(t *testing.T) {
		// Test with state 1 (new) - should block state 3 (closed successful)
		aclCtx := &models.ACLContext{
			UserID:  1,
			StateID: 1, // new
		}

		// All states
		options := map[int]string{
			1: "new",
			2: "open",
			3: "closed successful",
			4: "closed unsuccessful",
			5: "pending reminder",
		}

		filtered, err := svc.FilterOptions(ctx, aclCtx, "Ticket", "State", options)
		if err != nil {
			t.Fatalf("FilterOptions failed: %v", err)
		}

		t.Logf("Original options: %v", options)
		t.Logf("Filtered options: %v", filtered)

		// State 3 should be removed by TestACL-BlockClosedWhenNew
		if _, exists := filtered[3]; exists {
			t.Error("Expected state 3 (closed successful) to be filtered out when current state is new")
		}

		// Other states should still be present
		if _, exists := filtered[1]; !exists {
			t.Error("State 1 (new) should still be present")
		}
		if _, exists := filtered[2]; !exists {
			t.Error("State 2 (open) should still be present")
		}
	})

	t.Run("FilterOptions with non-matching context", func(t *testing.T) {
		// Test with state 2 (open) - ACL should not match, no filtering
		aclCtx := &models.ACLContext{
			UserID:  1,
			StateID: 2, // open - doesn't match ACL condition
		}

		options := map[int]string{
			1: "new",
			2: "open",
			3: "closed successful",
			4: "closed unsuccessful",
		}

		filtered, err := svc.FilterOptions(ctx, aclCtx, "Ticket", "State", options)
		if err != nil {
			t.Fatalf("FilterOptions failed: %v", err)
		}

		t.Logf("Original options: %v", options)
		t.Logf("Filtered options: %v", filtered)

		// All states should be present since ACL doesn't match
		if len(filtered) != len(options) {
			t.Errorf("Expected all %d options to be present, got %d", len(options), len(filtered))
		}
	})
}
