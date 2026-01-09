//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestPostmasterFilterRepository_CRUD(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test db: %v", err)
	}
	defer db.Close()

	repo := NewPostmasterFilterRepository(db)
	ctx := context.Background()

	// Clean up any existing test filters
	cleanupTestFilters(t, db)
	defer cleanupTestFilters(t, db)

	// Test Create
	t.Run("Create", func(t *testing.T) {
		filter := &PostmasterFilter{
			Name: "TestFilter1",
			Stop: true,
			Matches: []FilterMatch{
				{Key: "From", Value: ".*@test\\.com", Not: false},
				{Key: "Subject", Value: "\\[TEST\\]", Not: false},
			},
			Sets: []FilterSet{
				{Key: "X-GOTRS-Queue", Value: "Test Queue"},
				{Key: "X-GOTRS-PriorityID", Value: "3"},
			},
		}

		err := repo.Create(ctx, filter)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		filter, err := repo.Get(ctx, "TestFilter1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if filter.Name != "TestFilter1" {
			t.Errorf("expected name 'TestFilter1', got %q", filter.Name)
		}
		if !filter.Stop {
			t.Errorf("expected stop=true")
		}
		if len(filter.Matches) != 2 {
			t.Errorf("expected 2 matches, got %d", len(filter.Matches))
		}
		if len(filter.Sets) != 2 {
			t.Errorf("expected 2 sets, got %d", len(filter.Sets))
		}
	})

	// Test List
	t.Run("List", func(t *testing.T) {
		// Create another filter
		filter2 := &PostmasterFilter{
			Name: "TestFilter2",
			Stop: false,
			Matches: []FilterMatch{
				{Key: "To", Value: "support@", Not: false},
			},
			Sets: []FilterSet{
				{Key: "X-GOTRS-Title", Value: "Support Request"},
			},
		}
		err := repo.Create(ctx, filter2)
		if err != nil {
			t.Fatalf("Create filter2 failed: %v", err)
		}

		filters, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		// Find our test filters
		var found1, found2 bool
		for _, f := range filters {
			if f.Name == "TestFilter1" {
				found1 = true
			}
			if f.Name == "TestFilter2" {
				found2 = true
			}
		}

		if !found1 {
			t.Error("TestFilter1 not found in list")
		}
		if !found2 {
			t.Error("TestFilter2 not found in list")
		}
	})

	// Test Update
	t.Run("Update", func(t *testing.T) {
		updatedFilter := &PostmasterFilter{
			Name: "TestFilter1Updated",
			Stop: false,
			Matches: []FilterMatch{
				{Key: "From", Value: ".*@updated\\.com", Not: true},
			},
			Sets: []FilterSet{
				{Key: "X-GOTRS-Queue", Value: "Updated Queue"},
			},
		}

		err := repo.Update(ctx, "TestFilter1", updatedFilter)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// Verify the update
		filter, err := repo.Get(ctx, "TestFilter1Updated")
		if err != nil {
			t.Fatalf("Get after update failed: %v", err)
		}

		if filter.Name != "TestFilter1Updated" {
			t.Errorf("expected name 'TestFilter1Updated', got %q", filter.Name)
		}
		if filter.Stop {
			t.Errorf("expected stop=false after update")
		}
		if len(filter.Matches) != 1 {
			t.Errorf("expected 1 match after update, got %d", len(filter.Matches))
		}
		if filter.Matches[0].Not != true {
			t.Errorf("expected NOT flag to be true")
		}
		if len(filter.Sets) != 1 {
			t.Errorf("expected 1 set after update, got %d", len(filter.Sets))
		}

		// Old filter should not exist
		_, err = repo.Get(ctx, "TestFilter1")
		if err != sql.ErrNoRows {
			t.Errorf("expected old filter to not exist, got %v", err)
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		err := repo.Delete(ctx, "TestFilter1Updated")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify deletion
		_, err = repo.Get(ctx, "TestFilter1Updated")
		if err != sql.ErrNoRows {
			t.Errorf("expected filter to be deleted, got %v", err)
		}
	})

	// Test Delete non-existent
	t.Run("DeleteNonExistent", func(t *testing.T) {
		err := repo.Delete(ctx, "NonExistentFilter")
		if err != sql.ErrNoRows {
			t.Errorf("expected ErrNoRows for non-existent filter, got %v", err)
		}
	})

	// Test Get non-existent
	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := repo.Get(ctx, "NonExistentFilter")
		if err != sql.ErrNoRows {
			t.Errorf("expected ErrNoRows for non-existent filter, got %v", err)
		}
	})
}

func TestPostmasterFilterRepository_CreateValidation(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test db: %v", err)
	}
	defer db.Close()

	repo := NewPostmasterFilterRepository(db)
	ctx := context.Background()

	t.Run("NilFilter", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		if err == nil {
			t.Error("expected error for nil filter")
		}
	})

	t.Run("EmptyName", func(t *testing.T) {
		filter := &PostmasterFilter{
			Name: "",
			Matches: []FilterMatch{
				{Key: "From", Value: ".*"},
			},
		}
		err := repo.Create(ctx, filter)
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestPostmasterFilterRepository_FilterGrouping(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test db: %v", err)
	}
	defer db.Close()

	repo := NewPostmasterFilterRepository(db)
	ctx := context.Background()

	cleanupTestFilters(t, db)
	defer cleanupTestFilters(t, db)

	// Create a filter with multiple matches and sets
	filter := &PostmasterFilter{
		Name: "GroupedFilter",
		Stop: true,
		Matches: []FilterMatch{
			{Key: "From", Value: ".*@example\\.com", Not: false},
			{Key: "To", Value: "support@", Not: false},
			{Key: "Subject", Value: "\\[URGENT\\]", Not: false},
		},
		Sets: []FilterSet{
			{Key: "X-GOTRS-Queue", Value: "Priority"},
			{Key: "X-GOTRS-PriorityID", Value: "5"},
			{Key: "X-GOTRS-Title", Value: "Urgent Request"},
		},
	}

	err = repo.Create(ctx, filter)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify grouping
	retrieved, err := repo.Get(ctx, "GroupedFilter")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(retrieved.Matches) != 3 {
		t.Errorf("expected 3 matches, got %d", len(retrieved.Matches))
	}
	if len(retrieved.Sets) != 3 {
		t.Errorf("expected 3 sets, got %d", len(retrieved.Sets))
	}
	if !retrieved.Stop {
		t.Error("expected stop flag to be preserved")
	}
}

func cleanupTestFilters(t *testing.T, db *sql.DB) {
	t.Helper()
	query := database.ConvertPlaceholders(`
		DELETE FROM postmaster_filter WHERE f_name LIKE ?`)
	_, err := db.Exec(query, "Test%")
	if err != nil {
		t.Logf("cleanup warning: %v", err)
	}

	query2 := database.ConvertPlaceholders(`
		DELETE FROM postmaster_filter WHERE f_name = ?`)
	_, _ = db.Exec(query2, "GroupedFilter")
}
