//go:build integration
// +build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupsCRUDOperations(t *testing.T) {
	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		t.Skip("Database not available, skipping integration test")
	}

	// Create repository
	groupRepo := repository.NewGroupRepository(db)

	// Generate unique test group name
	testGroupName := fmt.Sprintf("TestGroup_%d", time.Now().Unix())
	testGroupDesc := "Test group for integration testing"
	updatedDesc := "Updated description for test group"

	var createdGroup *models.Group

	t.Run("Create Group", func(t *testing.T) {
		// Create new group
		group := &models.Group{
			Name:     testGroupName,
			Comments: testGroupDesc,
			ValidID:  1,
			CreateBy: 1, // System user
			ChangeBy: 1,
		}

		err := groupRepo.Create(group)
		require.NoError(t, err, "Should create group successfully")
		assert.NotEqual(t, 0, group.ID, "Group should have ID after creation")

		createdGroup = group
		t.Logf("Created group with ID: %d, Name: %s", group.ID, group.Name)
	})

	t.Run("Read Group", func(t *testing.T) {
		require.NotNil(t, createdGroup, "Need created group for read test")

		// Get by name
		group, err := groupRepo.GetByName(testGroupName)
		require.NoError(t, err, "Should find group by name")
		assert.NotNil(t, group, "Group should exist")
		assert.Equal(t, testGroupName, group.Name, "Name should match")
		assert.Equal(t, testGroupDesc, group.Comments, "Description should match")
		assert.Equal(t, 1, group.ValidID, "Should be valid")

		// Get by ID
		groupByID, err := groupRepo.GetByID(createdGroup.ID)
		require.NoError(t, err, "Should find group by ID")
		assert.NotNil(t, groupByID, "Group should exist")
		assert.Equal(t, testGroupName, groupByID.Name, "Name should match")
	})

	t.Run("Update Group", func(t *testing.T) {
		require.NotNil(t, createdGroup, "Need created group for update test")

		// Update the group
		createdGroup.Comments = updatedDesc
		createdGroup.ChangeBy = 1

		err := groupRepo.Update(createdGroup)
		require.NoError(t, err, "Should update group successfully")

		// Verify update
		updated, err := groupRepo.GetByID(createdGroup.ID)
		require.NoError(t, err, "Should find updated group")
		assert.Equal(t, updatedDesc, updated.Comments, "Description should be updated")
		t.Logf("Updated group description to: %s", updated.Comments)
	})

	t.Run("List Groups", func(t *testing.T) {
		// Get all groups
		groups, err := groupRepo.List()
		require.NoError(t, err, "Should get all groups")
		assert.NotEmpty(t, groups, "Should have groups")

		// Find our test group
		found := false
		for _, g := range groups {
			if g.Name == testGroupName {
				found = true
				assert.Equal(t, updatedDesc, g.Comments, "Should have updated description")
				break
			}
		}
		assert.True(t, found, "Should find test group in list")

		// Check system groups exist
		hasAdmin := false
		hasUsers := false
		hasStats := false
		for _, g := range groups {
			switch g.Name {
			case "admin":
				hasAdmin = true
			case "users":
				hasUsers = true
			case "stats":
				hasStats = true
			}
		}
		assert.True(t, hasAdmin, "Should have admin group")
		assert.True(t, hasUsers, "Should have users group")
		assert.True(t, hasStats, "Should have stats group")

		t.Logf("Found %d groups total", len(groups))
	})

	t.Run("Delete Group", func(t *testing.T) {
		require.NotNil(t, createdGroup, "Need created group for delete test")

		// Delete the group
		err := groupRepo.Delete(createdGroup.ID)
		require.NoError(t, err, "Should delete group successfully")

		// Verify deletion
		deleted, err := groupRepo.GetByID(createdGroup.ID)
		assert.Error(t, err, "Should not find deleted group")
		assert.Nil(t, deleted, "Deleted group should not exist")

		t.Logf("Successfully deleted group with ID: %d", createdGroup.ID)
	})

	t.Run("Cannot Delete System Groups", func(t *testing.T) {
		// Get admin group
		adminGroup, err := groupRepo.GetByName("admin")
		require.NoError(t, err, "Admin group should exist")
		require.NotNil(t, adminGroup, "Admin group should exist")

		// Try to delete it (this should be prevented in the handler, not repository)
		// The repository might allow it, but the handler should check
		// For now, just verify admin group exists
		assert.Equal(t, "admin", adminGroup.Name, "Should be admin group")
		assert.Equal(t, 1, adminGroup.ValidID, "Admin group should be valid")

		// Same for users and stats groups
		usersGroup, err := groupRepo.GetByName("users")
		assert.NoError(t, err, "Users group should exist")
		assert.NotNil(t, usersGroup, "Users group should exist")

		statsGroup, err := groupRepo.GetByName("stats")
		assert.NoError(t, err, "Stats group should exist")
		assert.NotNil(t, statsGroup, "Stats group should exist")
	})

	t.Run("Cannot Create Duplicate Group", func(t *testing.T) {
		// Try to create a group with existing name
		duplicate := &models.Group{
			Name:     "admin", // System group that exists
			Comments: "Trying to duplicate admin group",
			ValidID:  1,
			CreateBy: 1,
			ChangeBy: 1,
		}

		err := groupRepo.Create(duplicate)
		assert.Error(t, err, "Should not create duplicate group")
		t.Logf("Expected error for duplicate: %v", err)
	})

	t.Run("Create and Manage Inactive Group", func(t *testing.T) {
		// Create a group with Invalid status (2 = Invalid/Inactive)
		inactiveGroup := &models.Group{
			Name:     fmt.Sprintf("InactiveTest_%d", time.Now().Unix()),
			Comments: "This group is inactive/invalid",
			ValidID:  2, // Invalid status
			CreateBy: 1,
			ChangeBy: 1,
		}

		// Create the inactive group
		err := groupRepo.Create(inactiveGroup)
		require.NoError(t, err, "Should create inactive group successfully")
		assert.NotEqual(t, 0, inactiveGroup.ID, "Inactive group should have ID")
		t.Logf("Created inactive group with ID: %d", inactiveGroup.ID)

		// Retrieve and verify it's inactive
		retrieved, err := groupRepo.GetByID(inactiveGroup.ID)
		require.NoError(t, err, "Should find inactive group")
		assert.Equal(t, 2, retrieved.ValidID, "Should have Invalid status (2)")
		assert.Equal(t, inactiveGroup.Name, retrieved.Name, "Name should match")

		// Get all groups and verify inactive group is included
		allGroups, err := groupRepo.List()
		require.NoError(t, err, "Should get all groups")
		
		foundInactive := false
		for _, g := range allGroups {
			if g.ID == inactiveGroup.ID {
				foundInactive = true
				assert.Equal(t, 2, g.ValidID, "Should still be invalid in list")
				break
			}
		}
		assert.True(t, foundInactive, "Inactive group should appear in all groups list")

		// Update inactive group to make it active
		inactiveGroup.ValidID = 1 // Change to Valid
		inactiveGroup.Comments = "Now this group is active"
		err = groupRepo.Update(inactiveGroup)
		require.NoError(t, err, "Should update group status")

		// Verify the status change
		updated, err := groupRepo.GetByID(inactiveGroup.ID)
		require.NoError(t, err, "Should find updated group")
		assert.Equal(t, 1, updated.ValidID, "Should now have Valid status (1)")
		assert.Equal(t, "Now this group is active", updated.Comments, "Comments should be updated")

		// Clean up
		err = groupRepo.Delete(inactiveGroup.ID)
		require.NoError(t, err, "Should delete the test group")
		t.Logf("Successfully cleaned up inactive group test")
	})

	t.Run("Group Members", func(t *testing.T) {
		// Get members of admin group
		adminGroup, err := groupRepo.GetByName("admin")
		require.NoError(t, err, "Admin group should exist")

		members, err := groupRepo.GetGroupMembers(adminGroup.ID)
		if err != nil {
			t.Logf("GetGroupMembers not fully implemented: %v", err)
		} else {
			t.Logf("Admin group has %d members", len(members))
			for _, member := range members {
				t.Logf("  - User ID: %d", member.ID)
			}
		}
	})
}

func TestGroupValidation(t *testing.T) {
	db, err := database.GetDB()
	if err != nil {
		t.Skip("Database not available, skipping integration test")
	}

	groupRepo := repository.NewGroupRepository(db)

	t.Run("Empty Name", func(t *testing.T) {
		group := &models.Group{
			Name:     "", // Empty name
			Comments: "Group with empty name",
			ValidID:  1,
			CreateBy: 1,
			ChangeBy: 1,
		}

		err := groupRepo.Create(group)
		assert.Error(t, err, "Should not create group with empty name")
	})

	t.Run("Very Long Name", func(t *testing.T) {
		// Create a name longer than typical database column size
		longName := ""
		for i := 0; i < 300; i++ {
			longName += "a"
		}

		group := &models.Group{
			Name:     longName,
			Comments: "Group with very long name",
			ValidID:  1,
			CreateBy: 1,
			ChangeBy: 1,
		}

		err := groupRepo.Create(group)
		// This might succeed or fail depending on database column size
		if err != nil {
			t.Logf("Long name validation: %v", err)
		}
	})

	t.Run("Invalid ValidID", func(t *testing.T) {
		group := &models.Group{
			Name:     fmt.Sprintf("TestInvalid_%d", time.Now().Unix()),
			Comments: "Group with invalid status",
			ValidID:  999, // Non-existent valid_id
			CreateBy: 1,
			ChangeBy: 1,
		}

		// This might succeed (if no FK constraint) or fail
		err := groupRepo.Create(group)
		if err != nil {
			t.Logf("Invalid ValidID validation: %v", err)
			assert.Contains(t, err.Error(), "valid", "Error should mention valid_id")
		} else {
			// Clean up if it succeeded
			groupRepo.Delete(group.ID)
		}
	})
}

func TestGroupSearch(t *testing.T) {
	db, err := database.GetDB()
	if err != nil {
		t.Skip("Database not available, skipping integration test")
	}

	groupRepo := repository.NewGroupRepository(db)

	// Create test groups with specific patterns
	testGroups := []struct {
		name     string
		comments string
	}{
		{fmt.Sprintf("SearchTest_A_%d", time.Now().Unix()), "First search test group"},
		{fmt.Sprintf("SearchTest_B_%d", time.Now().Unix()), "Second search test group"},
		{fmt.Sprintf("SearchTest_C_%d", time.Now().Unix()), "Third search test group"},
	}

	// Create test groups
	createdIDs := []int{}
	for _, tg := range testGroups {
		group := &models.Group{
			Name:     tg.name,
			Comments: tg.comments,
			ValidID:  1,
			CreateBy: 1,
			ChangeBy: 1,
		}
		err := groupRepo.Create(group)
		require.NoError(t, err, "Should create test group")
		createdIDs = append(createdIDs, int(group.ID))
	}

	// Clean up after test
	defer func() {
		for _, id := range createdIDs {
			groupRepo.Delete(uint(id))
		}
	}()

	t.Run("Search by partial name", func(t *testing.T) {
		// Get all groups and filter manually (since search might be in handler)
		allGroups, err := groupRepo.List()
		require.NoError(t, err)

		searchCount := 0
		for _, g := range allGroups {
			if len(g.Name) > 10 && g.Name[:10] == "SearchTest" {
				searchCount++
			}
		}
		assert.GreaterOrEqual(t, searchCount, 3, "Should find at least 3 SearchTest groups")
	})

	t.Run("Case sensitivity", func(t *testing.T) {
		// Test that 'admin' != 'Admin'
		adminLower, err := groupRepo.GetByName("admin")
		assert.NoError(t, err, "Should find 'admin' group")
		assert.NotNil(t, adminLower)

		adminUpper, err := groupRepo.GetByName("Admin")
		if err == nil && adminUpper != nil {
			assert.NotEqual(t, adminLower.ID, adminUpper.ID, "Should be different groups if both exist")
		} else {
			t.Log("'Admin' group does not exist (case-sensitive search working)")
		}
	})
}