package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
)

func TestTicketMergeRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateMerge", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		merge := &models.TicketMerge{
			ParentTicketID: 100,
			ChildTicketID:  101,
			MergedBy:       1,
			Reason:         "Duplicate issue",
			Notes:          "Same customer reporting same problem",
		}

		err := repo.CreateMerge(ctx, merge)
		require.NoError(t, err)
		assert.NotZero(t, merge.ID)
		assert.True(t, merge.IsActive)
		assert.NotZero(t, merge.MergedAt)
	})

	t.Run("GetMerge", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		merge := &models.TicketMerge{
			ParentTicketID: 200,
			ChildTicketID:  201,
			MergedBy:       1,
			Reason:         "Related issues",
		}
		repo.CreateMerge(ctx, merge)

		retrieved, err := repo.GetMerge(ctx, merge.ID)
		require.NoError(t, err)
		assert.Equal(t, merge.ParentTicketID, retrieved.ParentTicketID)
		assert.Equal(t, merge.ChildTicketID, retrieved.ChildTicketID)
	})

	t.Run("GetMergesByParent", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		// Create multiple merges for same parent
		parentID := uint(300)
		for i := uint(1); i <= 3; i++ {
			merge := &models.TicketMerge{
				ParentTicketID: parentID,
				ChildTicketID:  300 + i,
				MergedBy:       1,
				Reason:         "Duplicate",
			}
			repo.CreateMerge(ctx, merge)
		}

		merges, err := repo.GetMergesByParent(ctx, parentID)
		require.NoError(t, err)
		assert.Len(t, merges, 3)

		for _, merge := range merges {
			assert.Equal(t, parentID, merge.ParentTicketID)
			assert.True(t, merge.IsActive)
		}
	})

	t.Run("GetMergeByChild", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		merge := &models.TicketMerge{
			ParentTicketID: 400,
			ChildTicketID:  401,
			MergedBy:       1,
			Reason:         "Same issue",
		}
		repo.CreateMerge(ctx, merge)

		retrieved, err := repo.GetMergeByChild(ctx, 401)
		require.NoError(t, err)
		assert.Equal(t, uint(400), retrieved.ParentTicketID)
		assert.Equal(t, uint(401), retrieved.ChildTicketID)
	})

	t.Run("UnmergeTicket", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		merge := &models.TicketMerge{
			ParentTicketID: 500,
			ChildTicketID:  501,
			MergedBy:       1,
			Reason:         "Duplicate",
		}
		repo.CreateMerge(ctx, merge)

		unmergedBy := uint(2)
		err := repo.UnmergeTicket(ctx, merge.ID, unmergedBy)
		require.NoError(t, err)

		retrieved, err := repo.GetMerge(ctx, merge.ID)
		require.NoError(t, err)
		assert.False(t, retrieved.IsActive)
		assert.NotNil(t, retrieved.UnmergedBy)
		assert.Equal(t, unmergedBy, *retrieved.UnmergedBy)
		assert.NotNil(t, retrieved.UnmergedAt)
	})

	t.Run("GetMergeHistory", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		// Create and unmerge
		merge := &models.TicketMerge{
			ParentTicketID: 600,
			ChildTicketID:  601,
			MergedBy:       1,
			Reason:         "Test merge",
		}
		repo.CreateMerge(ctx, merge)
		repo.UnmergeTicket(ctx, merge.ID, 2)

		// Create another merge
		merge2 := &models.TicketMerge{
			ParentTicketID: 600,
			ChildTicketID:  602,
			MergedBy:       1,
			Reason:         "Another merge",
		}
		repo.CreateMerge(ctx, merge2)

		history, err := repo.GetMergeHistory(ctx, 600)
		require.NoError(t, err)
		assert.Len(t, history, 2)
	})

	t.Run("IsMerged", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		merge := &models.TicketMerge{
			ParentTicketID: 700,
			ChildTicketID:  701,
			MergedBy:       1,
			Reason:         "Check merge status",
		}
		repo.CreateMerge(ctx, merge)

		// Check child ticket is merged
		isMerged, err := repo.IsMerged(ctx, 701)
		require.NoError(t, err)
		assert.True(t, isMerged)

		// Check non-merged ticket
		isMerged, err = repo.IsMerged(ctx, 999)
		require.NoError(t, err)
		assert.False(t, isMerged)

		// Unmerge and check again
		repo.UnmergeTicket(ctx, merge.ID, 2)
		isMerged, err = repo.IsMerged(ctx, 701)
		require.NoError(t, err)
		assert.False(t, isMerged)
	})

	t.Run("GetAllChildren", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		parentID := uint(800)
		childIDs := []uint{801, 802, 803}

		for _, childID := range childIDs {
			merge := &models.TicketMerge{
				ParentTicketID: parentID,
				ChildTicketID:  childID,
				MergedBy:       1,
				Reason:         "Bulk merge",
			}
			repo.CreateMerge(ctx, merge)
		}

		children, err := repo.GetAllChildren(ctx, parentID)
		require.NoError(t, err)
		assert.Len(t, children, 3)

		// Order is not guaranteed; compare as sets
		expected := map[uint]bool{childIDs[0]: true, childIDs[1]: true, childIDs[2]: true}
		for _, childID := range children {
			assert.True(t, expected[childID], "unexpected child id: %v", childID)
			delete(expected, childID)
		}
		assert.Empty(t, expected)
	})

	t.Run("GetMergeStatistics", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		// Create various merges
		reasons := []string{"Duplicate", "Related", "Duplicate", "Same customer"}
		for i, reason := range reasons {
			merge := &models.TicketMerge{
				ParentTicketID: uint(900 + i*10),
				ChildTicketID:  uint(901 + i*10),
				MergedBy:       1,
				Reason:         reason,
			}
			repo.CreateMerge(ctx, merge)
		}

		// Unmerge one
		repo.UnmergeTicket(ctx, 1, 2)

		stats, err := repo.GetMergeStatistics(ctx, time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 4, stats.TotalMerges)
		assert.Equal(t, 3, stats.ActiveMerges)
		assert.Equal(t, 1, stats.UnmergedCount)
		assert.NotEmpty(t, stats.TopMergeReasons)

		// Check top reason is "Duplicate"
		if len(stats.TopMergeReasons) > 0 {
			assert.Equal(t, "Duplicate", stats.TopMergeReasons[0].Reason)
			assert.Equal(t, 2, stats.TopMergeReasons[0].Count)
		}
	})

	t.Run("CreateTicketRelation", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		relation := &models.TicketRelation{
			TicketID:        1000,
			RelatedTicketID: 1001,
			RelationType:    "blocks",
			CreatedBy:       1,
			Notes:           "This ticket blocks the other",
		}

		err := repo.CreateTicketRelation(ctx, relation)
		require.NoError(t, err)
		assert.NotZero(t, relation.ID)
		assert.NotZero(t, relation.CreatedAt)
	})

	t.Run("GetTicketRelations", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		ticketID := uint(1100)
		relationTypes := []string{"blocks", "related", "duplicate"}

		for i, relType := range relationTypes {
			relation := &models.TicketRelation{
				TicketID:        ticketID,
				RelatedTicketID: uint(1101 + i),
				RelationType:    relType,
				CreatedBy:       1,
			}
			repo.CreateTicketRelation(ctx, relation)
		}

		relations, err := repo.GetTicketRelations(ctx, ticketID)
		require.NoError(t, err)
		assert.Len(t, relations, 3)

		for _, rel := range relations {
			assert.Equal(t, ticketID, rel.TicketID)
		}
	})

	t.Run("DeleteTicketRelation", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		relation := &models.TicketRelation{
			TicketID:        1200,
			RelatedTicketID: 1201,
			RelationType:    "related",
			CreatedBy:       1,
		}
		repo.CreateTicketRelation(ctx, relation)

		err := repo.DeleteTicketRelation(ctx, relation.ID)
		require.NoError(t, err)

		relations, err := repo.GetTicketRelations(ctx, 1200)
		require.NoError(t, err)
		assert.Empty(t, relations)
	})

	t.Run("PreventCircularMerge", func(t *testing.T) {
		repo := NewMemoryTicketMergeRepository()

		// Create initial merge: 1300 <- 1301
		merge1 := &models.TicketMerge{
			ParentTicketID: 1300,
			ChildTicketID:  1301,
			MergedBy:       1,
			Reason:         "Initial merge",
		}
		err := repo.CreateMerge(ctx, merge1)
		require.NoError(t, err)

		// Try to create circular merge: 1301 <- 1300 (should fail)
		merge2 := &models.TicketMerge{
			ParentTicketID: 1301,
			ChildTicketID:  1300,
			MergedBy:       1,
			Reason:         "Circular merge",
		}
		err = repo.CreateMerge(ctx, merge2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already merged")
	})
}
