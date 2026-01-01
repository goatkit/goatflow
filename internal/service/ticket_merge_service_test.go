package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

func TestTicketMergeService(t *testing.T) {
	ctx := context.Background()

	t.Run("MergeTickets", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		request := &models.MergeRequest{
			ParentTicketID: 100,
			ChildTicketIDs: []uint{101, 102, 103},
			Reason:         "Duplicate reports",
			Notes:          "All reporting same issue",
			MergeMessages:  true,
			CloseChildren:  true,
		}

		mergeIDs, err := service.MergeTickets(ctx, request, 1)
		require.NoError(t, err)
		assert.Len(t, mergeIDs, 3)

		// Verify all children are merged
		for _, childID := range request.ChildTicketIDs {
			isMerged, err := service.IsTicketMerged(ctx, childID)
			require.NoError(t, err)
			assert.True(t, isMerged)
		}
	})

	t.Run("ValidateMerge", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		// First merge: 200 <- 201
		firstRequest := &models.MergeRequest{
			ParentTicketID: 200,
			ChildTicketIDs: []uint{201},
			Reason:         "Initial merge",
		}
		service.MergeTickets(ctx, firstRequest, 1)

		// Try to merge already merged ticket
		secondRequest := &models.MergeRequest{
			ParentTicketID: 202,
			ChildTicketIDs: []uint{201}, // Already merged
			Reason:         "Invalid merge",
		}

		validation, err := service.ValidateMerge(ctx, secondRequest)
		require.NoError(t, err)
		assert.False(t, validation.CanMerge)
		assert.NotEmpty(t, validation.Errors)
		assert.Contains(t, validation.Errors[0], "already merged")

		// Try circular merge
		circularRequest := &models.MergeRequest{
			ParentTicketID: 201,
			ChildTicketIDs: []uint{200},
			Reason:         "Circular merge",
		}

		validation, err = service.ValidateMerge(ctx, circularRequest)
		require.NoError(t, err)
		assert.False(t, validation.CanMerge)
		assert.NotEmpty(t, validation.Errors)
	})

	t.Run("UnmergeTicket", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		// Create merge
		request := &models.MergeRequest{
			ParentTicketID: 300,
			ChildTicketIDs: []uint{301},
			Reason:         "Test merge",
		}
		mergeIDs, _ := service.MergeTickets(ctx, request, 1)

		// Unmerge
		unmergeReq := &models.UnmergeRequest{
			TicketID:     301,
			Reason:       "Not actually duplicate",
			ReopenTicket: true,
		}

		err := service.UnmergeTicket(ctx, unmergeReq, 2)
		require.NoError(t, err)

		// Verify unmerged
		isMerged, err := service.IsTicketMerged(ctx, 301)
		require.NoError(t, err)
		assert.False(t, isMerged)

		// Get merge to verify unmerge details
		merge, err := repo.GetMerge(ctx, mergeIDs[0])
		require.NoError(t, err)
		assert.False(t, merge.IsActive)
		assert.NotNil(t, merge.UnmergedBy)
		assert.Equal(t, uint(2), *merge.UnmergedBy)
	})

	t.Run("GetMergeTree", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		// Create a merge tree: 400 <- [401, 402, 403]
		request := &models.MergeRequest{
			ParentTicketID: 400,
			ChildTicketIDs: []uint{401, 402, 403},
			Reason:         "Multiple duplicates",
		}
		service.MergeTickets(ctx, request, 1)

		tree, err := service.GetMergeTree(ctx, 400)
		require.NoError(t, err)
		assert.Equal(t, uint(400), tree.ParentID)
		assert.Len(t, tree.Children, 3)
		assert.Equal(t, 3, tree.TotalChildren)
		assert.Equal(t, 1, tree.Depth)
	})

	t.Run("SplitTicket", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		splitReq := &models.SplitRequest{
			SourceTicketID:  500,
			MessageIDs:      []uint{10, 11, 12},
			NewTicketTitle:  "Split ticket - Different issue",
			NewTicketQueue:  2,
			CopyAttachments: true,
			LinkTickets:     true,
		}

		result := service.SplitTicket(ctx, splitReq, 1)
		assert.True(t, result.Success)
		assert.NotZero(t, result.NewTicketID)
		assert.NotEmpty(t, result.NewTicketNumber)
		assert.Equal(t, 3, result.MovedMessages)

		// Verify relation was created if LinkTickets was true
		if splitReq.LinkTickets {
			relations, err := service.GetTicketRelations(ctx, 500)
			require.NoError(t, err)
			assert.NotEmpty(t, relations)
		}
	})

	t.Run("CreateTicketRelation", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		relation := &models.TicketRelation{
			TicketID:        600,
			RelatedTicketID: 601,
			RelationType:    "blocks",
			CreatedBy:       1,
			Notes:           "This blocks the other ticket",
		}

		err := service.CreateTicketRelation(ctx, relation)
		require.NoError(t, err)

		// Get relations
		relations, err := service.GetTicketRelations(ctx, 600)
		require.NoError(t, err)
		assert.Len(t, relations, 1)
		assert.Equal(t, "blocks", relations[0].RelationType)
	})

	t.Run("GetMergeStatistics", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		// Create various merges
		merges := []struct {
			parent uint
			child  uint
			reason string
		}{
			{700, 701, "Duplicate"},
			{700, 702, "Duplicate"},
			{703, 704, "Related"},
			{705, 706, "Same customer"},
		}

		for _, m := range merges {
			request := &models.MergeRequest{
				ParentTicketID: m.parent,
				ChildTicketIDs: []uint{m.child},
				Reason:         m.reason,
			}
			service.MergeTickets(ctx, request, 1)
		}

		stats, err := service.GetMergeStatistics(ctx, time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 4, stats.TotalMerges)
		assert.Equal(t, 4, stats.ActiveMerges)
		assert.NotEmpty(t, stats.TopMergeReasons)

		// Check that "Duplicate" is the top reason
		if len(stats.TopMergeReasons) > 0 {
			assert.Equal(t, "Duplicate", stats.TopMergeReasons[0].Reason)
			assert.Equal(t, 2, stats.TopMergeReasons[0].Count)
		}
	})

	t.Run("GetMergeHistory", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		// Create merge
		request := &models.MergeRequest{
			ParentTicketID: 800,
			ChildTicketIDs: []uint{801},
			Reason:         "Test history",
		}
		_, err := service.MergeTickets(ctx, request, 1)
		require.NoError(t, err)

		// Unmerge
		unmergeReq := &models.UnmergeRequest{
			TicketID: 801,
			Reason:   "Unmerge test",
		}
		service.UnmergeTicket(ctx, unmergeReq, 2)

		// Create another merge
		request2 := &models.MergeRequest{
			ParentTicketID: 800,
			ChildTicketIDs: []uint{802},
			Reason:         "Another merge",
		}
		service.MergeTickets(ctx, request2, 1)

		// Get history
		history, err := service.GetMergeHistory(ctx, 800)
		require.NoError(t, err)
		assert.Len(t, history, 2)

		// Check that we have one active and one inactive merge
		activeCount := 0
		for _, h := range history {
			if h.IsActive {
				activeCount++
			}
		}
		assert.Equal(t, 1, activeCount)
	})

	t.Run("BulkMerge", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		// Bulk merge multiple tickets
		request := &models.MergeRequest{
			ParentTicketID: 900,
			ChildTicketIDs: []uint{901, 902, 903, 904, 905},
			Reason:         "Bulk duplicate cleanup",
			Notes:          "Monthly cleanup of duplicate tickets",
			CloseChildren:  true,
		}

		mergeIDs, err := service.MergeTickets(ctx, request, 1)
		require.NoError(t, err)
		assert.Len(t, mergeIDs, 5)

		// Verify all merged
		for _, childID := range request.ChildTicketIDs {
			isMerged, err := service.IsTicketMerged(ctx, childID)
			require.NoError(t, err)
			assert.True(t, isMerged, "Ticket %d should be merged", childID)
		}

		// Get parent's children
		children, err := service.GetChildTickets(ctx, 900)
		require.NoError(t, err)
		assert.Len(t, children, 5)
	})

	t.Run("PreventSelfMerge", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		// Try to merge ticket into itself
		request := &models.MergeRequest{
			ParentTicketID: 1000,
			ChildTicketIDs: []uint{1000}, // Same as parent
			Reason:         "Self merge",
		}

		validation, err := service.ValidateMerge(ctx, request)
		require.NoError(t, err)
		assert.False(t, validation.CanMerge)
		assert.NotEmpty(t, validation.Errors)
		assert.Contains(t, validation.Errors[0], "cannot merge into itself")
	})

	t.Run("AutoSplitByCriteria", func(t *testing.T) {
		repo := repository.NewMemoryTicketMergeRepository()
		service := NewTicketMergeService(repo)

		splitReq := &models.SplitRequest{
			SourceTicketID: 1100,
			MessageIDs:     []uint{20, 21, 22, 23, 24},
			NewTicketTitle: "Auto-split ticket",
			NewTicketQueue: 3,
			LinkTickets:    true,
			SplitCriteria: models.SplitCriteria{
				ByCustomer:   true,
				ByTimeGap:    true,
				TimeGapHours: 24,
			},
		}

		result := service.SplitTicket(ctx, splitReq, 1)
		assert.True(t, result.Success)
		assert.NotZero(t, result.NewTicketID)
		assert.Equal(t, 5, result.MovedMessages)
	})
}
