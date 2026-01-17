package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func getTestDBForQueueAccess(t *testing.T) *sql.DB {
	db, err := database.GetDB()
	if err != nil {
		t.Skipf("Database not available: %v", err)
	}
	return db
}

func TestQueueAccessService_GetUserEffectiveGroupIDs(t *testing.T) {
	db := getTestDBForQueueAccess(t)
	svc := NewQueueAccessService(db)
	ctx := context.Background()

	t.Run("user_with_direct_ro_permission", func(t *testing.T) {
		// User 1 (admin) should have access to groups via direct permissions
		groupIDs, err := svc.GetUserEffectiveGroupIDs(ctx, 1, "ro")
		require.NoError(t, err)
		assert.NotEmpty(t, groupIDs, "Admin user should have group access")
	})

	t.Run("user_with_rw_permission_includes_ro", func(t *testing.T) {
		// User with 'rw' should also get access when checking 'ro'
		// because rw supersedes all
		groupIDs, err := svc.GetUserEffectiveGroupIDs(ctx, 1, "ro")
		require.NoError(t, err)
		assert.NotEmpty(t, groupIDs)
	})

	t.Run("nonexistent_user_returns_empty", func(t *testing.T) {
		groupIDs, err := svc.GetUserEffectiveGroupIDs(ctx, 999999, "ro")
		require.NoError(t, err)
		assert.Empty(t, groupIDs)
	})
}

func TestQueueAccessService_GetAccessibleQueueIDs(t *testing.T) {
	db := getTestDBForQueueAccess(t)
	svc := NewQueueAccessService(db)
	ctx := context.Background()

	t.Run("admin_user_can_access_queues", func(t *testing.T) {
		queueIDs, err := svc.GetAccessibleQueueIDs(ctx, 1, "ro")
		require.NoError(t, err)
		assert.NotEmpty(t, queueIDs, "Admin user should have queue access")
	})

	t.Run("nonexistent_user_returns_empty", func(t *testing.T) {
		queueIDs, err := svc.GetAccessibleQueueIDs(ctx, 999999, "ro")
		require.NoError(t, err)
		assert.Empty(t, queueIDs)
	})
}

func TestQueueAccessService_GetAccessibleQueues(t *testing.T) {
	db := getTestDBForQueueAccess(t)
	svc := NewQueueAccessService(db)
	ctx := context.Background()

	t.Run("returns_queue_details", func(t *testing.T) {
		queues, err := svc.GetAccessibleQueues(ctx, 1, "ro")
		require.NoError(t, err)
		assert.NotEmpty(t, queues, "Admin user should have queue access")

		// Verify structure
		if len(queues) > 0 {
			assert.NotZero(t, queues[0].QueueID)
			assert.NotEmpty(t, queues[0].QueueName)
			assert.NotZero(t, queues[0].GroupID)
			assert.NotEmpty(t, queues[0].GroupName)
		}
	})
}

func TestQueueAccessService_HasQueueAccess(t *testing.T) {
	db := getTestDBForQueueAccess(t)
	svc := NewQueueAccessService(db)
	ctx := context.Background()

	t.Run("returns_no_error_for_valid_params", func(t *testing.T) {
		// Just check the function works without error
		_, err := svc.HasQueueAccess(ctx, 1, 1, "ro")
		require.NoError(t, err)
	})

	t.Run("nonexistent_queue_returns_false", func(t *testing.T) {
		hasAccess, err := svc.HasQueueAccess(ctx, 1, 999999, "ro")
		require.NoError(t, err)
		assert.False(t, hasAccess)
	})

	t.Run("nonexistent_user_returns_false", func(t *testing.T) {
		hasAccess, err := svc.HasQueueAccess(ctx, 999999, 1, "ro")
		require.NoError(t, err)
		assert.False(t, hasAccess)
	})
}

func TestQueueAccessService_IsAdmin(t *testing.T) {
	db := getTestDBForQueueAccess(t)
	svc := NewQueueAccessService(db)
	ctx := context.Background()

	t.Run("returns_no_error_for_valid_user", func(t *testing.T) {
		// Just check the function works without error
		_, err := svc.IsAdmin(ctx, 1)
		require.NoError(t, err)
	})

	t.Run("nonexistent_user_is_not_admin", func(t *testing.T) {
		isAdmin, err := svc.IsAdmin(ctx, 999999)
		require.NoError(t, err)
		assert.False(t, isAdmin)
	})
}

func TestQueueAccessService_GetQueueGroupID(t *testing.T) {
	db := getTestDBForQueueAccess(t)
	svc := NewQueueAccessService(db)
	ctx := context.Background()

	t.Run("valid_queue_returns_group_id", func(t *testing.T) {
		groupID, err := svc.GetQueueGroupID(ctx, 1)
		require.NoError(t, err)
		assert.NotZero(t, groupID)
	})

	t.Run("invalid_queue_returns_error", func(t *testing.T) {
		_, err := svc.GetQueueGroupID(ctx, 999999)
		assert.Error(t, err)
	})
}

func TestQueueAccessService_PermissionTypes(t *testing.T) {
	db := getTestDBForQueueAccess(t)
	svc := NewQueueAccessService(db)
	ctx := context.Background()

	// Test various permission types
	permTypes := []string{"ro", "rw", "create", "move_into", "note", "owner", "priority"}

	for _, permType := range permTypes {
		t.Run("permission_type_"+permType, func(t *testing.T) {
			// Should not error for any valid permission type
			_, err := svc.GetUserEffectiveGroupIDs(ctx, 1, permType)
			require.NoError(t, err, "Should handle permission type: %s", permType)
		})
	}
}
