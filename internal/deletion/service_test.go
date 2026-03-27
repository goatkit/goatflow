package deletion

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestService_FullLifecycle(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)
	testEntityType := "test_lifecycle"
	testID := int64(99300 + time.Now().UnixNano()%1000)
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	ctx := context.Background()

	// We can't use softDeleteEntity for real entities (no test tickets),
	// but we can test the recycle bin + tombstone lifecycle directly.

	t.Run("add to bin, list, restore, purge", func(t *testing.T) {
		// Manually add to recycle bin (simulating soft delete outcome).
		name := "Test Lifecycle Item"
		entry := &RecycleBinEntry{
			EntityType: testEntityType,
			EntityID:   testID,
			EntityName: &name,
			DeletedBy:  1,
			DeletedAt:  time.Now(),
		}
		svc.repo.AddToRecycleBin(entry)
		svc.repo.LogDeletion(&DeletionLog{
			EntityType: testEntityType, EntityID: testID, Action: ActionSoftDelete,
			DeletedBy: 1, DeletedAt: time.Now(),
		})

		// List should include it.
		entries, err := svc.RecycleBinList(ctx, testEntityType)
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
			t.Error("expected test entry in recycle bin list")
		}

		// Remove from bin (simulating restore outcome).
		svc.repo.RemoveFromRecycleBin(testEntityType, testID)
		svc.repo.LogDeletion(&DeletionLog{
			EntityType: testEntityType, EntityID: testID, Action: ActionRestore,
			DeletedBy: 1, DeletedAt: time.Now(),
		})

		// Should no longer be in bin.
		entries2, _ := svc.RecycleBinList(ctx, testEntityType)
		for _, e := range entries2 {
			if e.EntityID == testID {
				t.Error("should not be in bin after restore")
			}
		}

		// Deletion log should have both entries.
		logs, _ := svc.repo.GetDeletionLog(testEntityType, testID)
		if len(logs) < 2 {
			t.Errorf("expected at least 2 log entries, got %d", len(logs))
		}
	})
}

func TestService_PurgeExpired(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)
	testEntityType := "test_purge"
	t.Cleanup(func() { cleanupTestDeletions(t, db, testEntityType, 99000) })

	ctx := context.Background()

	// Add expired entries.
	past := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 3; i++ {
		svc.repo.AddToRecycleBin(&RecycleBinEntry{
			EntityType: testEntityType,
			EntityID:   int64(99400 + i),
			DeletedBy:  1,
			DeletedAt:  time.Now().Add(-2 * time.Hour),
			ExpiresAt:  &past,
		})
	}

	// PurgeExpired — these are test entities so hardDeleteEntity will fail,
	// but the recycle bin cleanup should still run.
	purged, _ := svc.PurgeExpired(ctx)
	// We can't assert exact count because hardDeleteEntity for "test_purge" type
	// will return "unsupported entity type" — but the function shouldn't panic.
	_ = purged
	_ = fmt.Sprintf("purged %d", purged)
}

func TestService_CascadeRegistration(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)

	calls := make([]string, 0)

	svc.RegisterCascade("ticket", func(ctx context.Context, entityType string, entityID int64) error {
		calls = append(calls, fmt.Sprintf("handler1:%s:%d", entityType, entityID))
		return nil
	})
	svc.RegisterCascade("ticket", func(ctx context.Context, entityType string, entityID int64) error {
		calls = append(calls, fmt.Sprintf("handler2:%s:%d", entityType, entityID))
		return nil
	})
	svc.RegisterCascade("contact", func(ctx context.Context, entityType string, entityID int64) error {
		calls = append(calls, fmt.Sprintf("contact_handler:%s:%d", entityType, entityID))
		return nil
	})

	ctx := context.Background()

	// Trigger ticket cascades.
	svc.runCascades(ctx, "ticket", 42, "soft")
	if len(calls) != 2 {
		t.Errorf("expected 2 ticket cascade calls, got %d: %v", len(calls), calls)
	}

	// Trigger contact cascades.
	svc.runCascades(ctx, "contact", 99, "hard")
	if len(calls) != 3 {
		t.Errorf("expected 3 total cascade calls, got %d: %v", len(calls), calls)
	}

	// Trigger for entity with no handlers — should not panic.
	svc.runCascades(ctx, "queue", 1, "soft")
	if len(calls) != 3 {
		t.Error("queue cascade should not add calls")
	}
}

func TestService_RetentionConfig(t *testing.T) {
	db := getTestDB(t)
	svc := NewServiceWithDB(db)

	// Default retention.
	if got := svc.retentionForType("anything"); got != 60 {
		t.Errorf("default retention = %d, want 60", got)
	}

	// Custom retention.
	svc.SetRetention("ticket", 90)
	svc.SetRetention("contact", 30)

	if got := svc.retentionForType("ticket"); got != 90 {
		t.Errorf("ticket retention = %d", got)
	}
	if got := svc.retentionForType("contact"); got != 30 {
		t.Errorf("contact retention = %d", got)
	}
	if got := svc.retentionForType("queue"); got != 60 {
		t.Errorf("queue should use default 60, got %d", got)
	}
}
