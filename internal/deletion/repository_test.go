package deletion

import (
	"context"
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
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'gk_recycle_bin'",
	).Scan(&count)
	if err != nil || count == 0 {
		t.Skipf("gk_recycle_bin table not found — run migration 000013")
	}
	return db
}

func cleanupTestDeletions(t *testing.T, db *sql.DB, entityType string, minID int64) {
	t.Helper()
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_recycle_bin WHERE entity_type = ? AND entity_id >= ?"), entityType, minID)
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_deletion_log WHERE entity_type = ? AND entity_id >= ?"), entityType, minID)
}

func TestRepository_RecycleBin(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	testEntityType := "test_entity"
	testID := int64(99000 + time.Now().UnixNano()%1000)
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	t.Run("AddToRecycleBin and Get", func(t *testing.T) {
		name := "Test Item"
		expires := time.Now().Add(24 * time.Hour)
		entry := &RecycleBinEntry{
			EntityType: testEntityType,
			EntityID:   testID,
			EntityName: &name,
			DeletedBy:  1,
			DeletedAt:  time.Now(),
			ExpiresAt:  &expires,
		}

		id, err := repo.AddToRecycleBin(entry)
		if err != nil {
			t.Fatalf("AddToRecycleBin: %v", err)
		}
		if id == 0 {
			t.Fatal("expected non-zero ID")
		}

		got, err := repo.GetRecycleBinEntry(testEntityType, testID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil entry")
		}
		if got.EntityID != testID {
			t.Errorf("entity_id = %d, want %d", got.EntityID, testID)
		}
		if got.EntityName == nil || *got.EntityName != "Test Item" {
			t.Errorf("entity_name = %v", got.EntityName)
		}
	})

	t.Run("ListRecycleBin", func(t *testing.T) {
		entries, err := repo.ListRecycleBin(testEntityType, 0)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range entries {
			if e.EntityID == testID {
				found = true
			}
		}
		if !found {
			t.Error("expected to find test entry in list")
		}
	})

	t.Run("RemoveFromRecycleBin", func(t *testing.T) {
		err := repo.RemoveFromRecycleBin(testEntityType, testID)
		if err != nil {
			t.Fatal(err)
		}

		got, _ := repo.GetRecycleBinEntry(testEntityType, testID)
		if got != nil {
			t.Error("expected nil after removal")
		}
	})

	t.Run("GetRecycleBinEntry not found", func(t *testing.T) {
		got, err := repo.GetRecycleBinEntry("nonexistent", 999999)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Error("expected nil")
		}
	})
}

func TestRepository_DeletionLog(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	testEntityType := "test_entity"
	testID := int64(99100 + time.Now().UnixNano()%1000)
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	t.Run("LogDeletion and GetDeletionLog", func(t *testing.T) {
		reason := "GDPR request"
		entry := &DeletionLog{
			EntityType: testEntityType,
			EntityID:   testID,
			Action:     ActionSoftDelete,
			DeletedBy:  1,
			DeletedAt:  time.Now(),
			Reason:     &reason,
		}

		err := repo.LogDeletion(entry)
		if err != nil {
			t.Fatalf("LogDeletion: %v", err)
		}

		logs, err := repo.GetDeletionLog(testEntityType, testID)
		if err != nil {
			t.Fatal(err)
		}
		if len(logs) != 1 {
			t.Fatalf("expected 1 log, got %d", len(logs))
		}
		if logs[0].Action != ActionSoftDelete {
			t.Errorf("action = %q", logs[0].Action)
		}
		if logs[0].Reason == nil || *logs[0].Reason != "GDPR request" {
			t.Errorf("reason = %v", logs[0].Reason)
		}
	})

	t.Run("multiple log entries", func(t *testing.T) {
		repo.LogDeletion(&DeletionLog{
			EntityType: testEntityType, EntityID: testID, Action: ActionRestore,
			DeletedBy: 1, DeletedAt: time.Now(),
		})
		repo.LogDeletion(&DeletionLog{
			EntityType: testEntityType, EntityID: testID, Action: ActionHardDelete,
			DeletedBy: 1, DeletedAt: time.Now(),
		})

		logs, _ := repo.GetDeletionLog(testEntityType, testID)
		if len(logs) < 3 {
			t.Errorf("expected at least 3 logs, got %d", len(logs))
		}
	})
}

func TestRepository_ListExpired(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	testEntityType := "test_expired"
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	// Add an already-expired entry.
	pastTime := time.Now().Add(-1 * time.Hour)
	repo.AddToRecycleBin(&RecycleBinEntry{
		EntityType: testEntityType,
		EntityID:   99200,
		DeletedBy:  1,
		DeletedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt:  &pastTime,
	})

	// Add a not-yet-expired entry.
	futureTime := time.Now().Add(24 * time.Hour)
	repo.AddToRecycleBin(&RecycleBinEntry{
		EntityType: testEntityType,
		EntityID:   99201,
		DeletedBy:  1,
		DeletedAt:  time.Now(),
		ExpiresAt:  &futureTime,
	})

	expired, err := repo.ListExpired()
	if err != nil {
		t.Fatal(err)
	}

	foundExpired := false
	foundFuture := false
	for _, e := range expired {
		if e.EntityType == testEntityType && e.EntityID == 99200 {
			foundExpired = true
		}
		if e.EntityType == testEntityType && e.EntityID == 99201 {
			foundFuture = true
		}
	}
	if !foundExpired {
		t.Error("expected expired entry in list")
	}
	if foundFuture {
		t.Error("future entry should not be in expired list")
	}
}

func TestService_Cascade(t *testing.T) {
	svc := &Service{
		cascadeHandlers:  make(map[string][]CascadeHandler),
		retentionDays:    make(map[string]int),
		defaultRetention: 60,
	}

	cascadeCalled := false
	svc.RegisterCascade("test_type", func(ctx context.Context, entityType string, entityID int64) error {
		cascadeCalled = true
		if entityType != "test_type" || entityID != 42 {
			t.Errorf("cascade got %s/%d", entityType, entityID)
		}
		return nil
	})

	svc.runCascades(context.Background(), "test_type", 42, "soft")
	if !cascadeCalled {
		t.Error("cascade handler was not called")
	}
}

func TestService_RetentionForType(t *testing.T) {
	svc := &Service{
		retentionDays:    map[string]int{"ticket": 90},
		defaultRetention: 60,
	}

	if got := svc.retentionForType("ticket"); got != 90 {
		t.Errorf("ticket retention = %d, want 90", got)
	}
	if got := svc.retentionForType("contact"); got != 60 {
		t.Errorf("contact retention = %d, want 60 (default)", got)
	}
}

func TestService_SetRetention(t *testing.T) {
	svc := &Service{
		retentionDays:    make(map[string]int),
		defaultRetention: 60,
	}

	svc.SetRetention("ticket", 30)
	if svc.retentionDays["ticket"] != 30 {
		t.Errorf("expected 30, got %d", svc.retentionDays["ticket"])
	}
}

// Use a context import to avoid unused import error.
var _ = fmt.Sprintf
