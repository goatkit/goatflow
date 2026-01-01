package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// TicketMergeRepository defines the interface for ticket merge operations.
type TicketMergeRepository interface {
	CreateMerge(ctx context.Context, merge *models.TicketMerge) error
	GetMerge(ctx context.Context, id uint) (*models.TicketMerge, error)
	GetMergesByParent(ctx context.Context, parentID uint) ([]models.TicketMerge, error)
	GetMergeByChild(ctx context.Context, childID uint) (*models.TicketMerge, error)
	UnmergeTicket(ctx context.Context, mergeID uint, unmergedBy uint) error
	GetMergeHistory(ctx context.Context, ticketID uint) ([]models.TicketMerge, error)
	IsMerged(ctx context.Context, ticketID uint) (bool, error)
	GetAllChildren(ctx context.Context, parentID uint) ([]uint, error)
	GetMergeStatistics(ctx context.Context, from, to time.Time) (*models.MergeStatistics, error)

	// Ticket relations
	CreateTicketRelation(ctx context.Context, relation *models.TicketRelation) error
	GetTicketRelations(ctx context.Context, ticketID uint) ([]models.TicketRelation, error)
	DeleteTicketRelation(ctx context.Context, relationID uint) error
}

// MemoryTicketMergeRepository is an in-memory implementation of TicketMergeRepository.
type MemoryTicketMergeRepository struct {
	merges    map[uint]*models.TicketMerge
	relations map[uint]*models.TicketRelation
	mu        sync.RWMutex
	nextID    uint
	nextRelID uint
}

// NewMemoryTicketMergeRepository creates a new in-memory ticket merge repository.
func NewMemoryTicketMergeRepository() *MemoryTicketMergeRepository {
	return &MemoryTicketMergeRepository{
		merges:    make(map[uint]*models.TicketMerge),
		relations: make(map[uint]*models.TicketRelation),
		nextID:    1,
		nextRelID: 1,
	}
}

// CreateMerge creates a new merge record.
func (r *MemoryTicketMergeRepository) CreateMerge(ctx context.Context, merge *models.TicketMerge) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if child is already merged
	for _, existing := range r.merges {
		if existing.ChildTicketID == merge.ChildTicketID && existing.IsActive {
			return fmt.Errorf("ticket %d is already merged into ticket %d",
				merge.ChildTicketID, existing.ParentTicketID)
		}
		// Prevent circular merges
		if existing.ChildTicketID == merge.ParentTicketID && existing.IsActive {
			return fmt.Errorf("ticket %d is already merged as a child, cannot be a parent",
				merge.ParentTicketID)
		}
	}

	merge.ID = r.nextID
	r.nextID++
	merge.MergedAt = time.Now()
	merge.IsActive = true

	// Create a copy to store
	stored := *merge
	r.merges[merge.ID] = &stored

	return nil
}

// GetMerge retrieves a merge by ID.
func (r *MemoryTicketMergeRepository) GetMerge(ctx context.Context, id uint) (*models.TicketMerge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	merge, exists := r.merges[id]
	if !exists {
		return nil, fmt.Errorf("merge %d not found", id)
	}

	// Return a copy
	result := *merge
	return &result, nil
}

// GetMergesByParent retrieves all active merges for a parent ticket.
func (r *MemoryTicketMergeRepository) GetMergesByParent(ctx context.Context, parentID uint) ([]models.TicketMerge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var merges []models.TicketMerge
	for _, merge := range r.merges {
		if merge.ParentTicketID == parentID && merge.IsActive {
			merges = append(merges, *merge)
		}
	}

	return merges, nil
}

// GetMergeByChild retrieves the merge record for a child ticket.
func (r *MemoryTicketMergeRepository) GetMergeByChild(ctx context.Context, childID uint) (*models.TicketMerge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, merge := range r.merges {
		if merge.ChildTicketID == childID && merge.IsActive {
			result := *merge
			return &result, nil
		}
	}

	return nil, fmt.Errorf("no active merge found for child ticket %d", childID)
}

// UnmergeTicket marks a merge as inactive.
func (r *MemoryTicketMergeRepository) UnmergeTicket(ctx context.Context, mergeID uint, unmergedBy uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	merge, exists := r.merges[mergeID]
	if !exists {
		return fmt.Errorf("merge %d not found", mergeID)
	}

	if !merge.IsActive {
		return fmt.Errorf("merge %d is already inactive", mergeID)
	}

	now := time.Now()
	merge.IsActive = false
	merge.UnmergedBy = &unmergedBy
	merge.UnmergedAt = &now

	return nil
}

// GetMergeHistory retrieves all merge history for a ticket.
func (r *MemoryTicketMergeRepository) GetMergeHistory(ctx context.Context, ticketID uint) ([]models.TicketMerge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var history []models.TicketMerge
	for _, merge := range r.merges {
		if merge.ParentTicketID == ticketID || merge.ChildTicketID == ticketID {
			history = append(history, *merge)
		}
	}

	return history, nil
}

// IsMerged checks if a ticket is currently merged.
func (r *MemoryTicketMergeRepository) IsMerged(ctx context.Context, ticketID uint) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, merge := range r.merges {
		if merge.ChildTicketID == ticketID && merge.IsActive {
			return true, nil
		}
	}

	return false, nil
}

// GetAllChildren retrieves all child ticket IDs for a parent.
func (r *MemoryTicketMergeRepository) GetAllChildren(ctx context.Context, parentID uint) ([]uint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var children []uint
	for _, merge := range r.merges {
		if merge.ParentTicketID == parentID && merge.IsActive {
			children = append(children, merge.ChildTicketID)
		}
	}

	return children, nil
}

// GetMergeStatistics generates merge statistics.
func (r *MemoryTicketMergeRepository) GetMergeStatistics(ctx context.Context, from, to time.Time) (*models.MergeStatistics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &models.MergeStatistics{
		TopMergeReasons: []models.MergeReasonStat{},
		MergesByMonth:   []models.MergeMonthStat{},
	}

	reasonCount := make(map[string]int)
	monthCount := make(map[string]*models.MergeMonthStat)
	childCounts := []int{}

	// Process merges
	for _, merge := range r.merges {
		if merge.MergedAt.After(from) && merge.MergedAt.Before(to) {
			stats.TotalMerges++
			if merge.IsActive {
				stats.ActiveMerges++
			} else {
				stats.UnmergedCount++
			}

			// Count reasons
			reasonCount[merge.Reason]++

			// Count by month
			monthKey := merge.MergedAt.Format("2006-01")
			if _, exists := monthCount[monthKey]; !exists {
				monthCount[monthKey] = &models.MergeMonthStat{
					Month: merge.MergedAt.Format("January"),
					Year:  merge.MergedAt.Year(),
				}
			}
			monthCount[monthKey].Merges++

			if !merge.IsActive && merge.UnmergedAt != nil {
				unmergeKey := merge.UnmergedAt.Format("2006-01")
				if _, exists := monthCount[unmergeKey]; !exists {
					monthCount[unmergeKey] = &models.MergeMonthStat{
						Month: merge.UnmergedAt.Format("January"),
						Year:  merge.UnmergedAt.Year(),
					}
				}
				monthCount[unmergeKey].Unmerges++
			}
		}
	}

	// Count children per parent
	parentChildCount := make(map[uint]int)
	for _, merge := range r.merges {
		if merge.IsActive {
			parentChildCount[merge.ParentTicketID]++
		}
	}

	for _, count := range parentChildCount {
		childCounts = append(childCounts, count)
	}

	// Calculate average child count
	if len(childCounts) > 0 {
		sum := 0
		for _, count := range childCounts {
			sum += count
		}
		stats.AverageChildCount = float64(sum) / float64(len(childCounts))
	}

	// Convert reason counts to sorted list
	for reason, count := range reasonCount {
		stats.TopMergeReasons = append(stats.TopMergeReasons, models.MergeReasonStat{
			Reason: reason,
			Count:  count,
		})
	}

	// Sort by count descending
	for i := 0; i < len(stats.TopMergeReasons); i++ {
		for j := i + 1; j < len(stats.TopMergeReasons); j++ {
			if stats.TopMergeReasons[j].Count > stats.TopMergeReasons[i].Count {
				stats.TopMergeReasons[i], stats.TopMergeReasons[j] =
					stats.TopMergeReasons[j], stats.TopMergeReasons[i]
			}
		}
	}

	// Convert month stats to list
	for _, monthStat := range monthCount {
		stats.MergesByMonth = append(stats.MergesByMonth, *monthStat)
	}

	return stats, nil
}

// CreateTicketRelation creates a new ticket relation.
func (r *MemoryTicketMergeRepository) CreateTicketRelation(ctx context.Context, relation *models.TicketRelation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate relations
	for _, existing := range r.relations {
		if existing.TicketID == relation.TicketID &&
			existing.RelatedTicketID == relation.RelatedTicketID &&
			existing.RelationType == relation.RelationType {
			return fmt.Errorf("relation already exists between tickets %d and %d",
				relation.TicketID, relation.RelatedTicketID)
		}
	}

	relation.ID = r.nextRelID
	r.nextRelID++
	relation.CreatedAt = time.Now()

	// Create a copy to store
	stored := *relation
	r.relations[relation.ID] = &stored

	return nil
}

// GetTicketRelations retrieves all relations for a ticket.
func (r *MemoryTicketMergeRepository) GetTicketRelations(ctx context.Context, ticketID uint) ([]models.TicketRelation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var relations []models.TicketRelation
	for _, relation := range r.relations {
		if relation.TicketID == ticketID || relation.RelatedTicketID == ticketID {
			relations = append(relations, *relation)
		}
	}

	return relations, nil
}

// DeleteTicketRelation deletes a ticket relation.
func (r *MemoryTicketMergeRepository) DeleteTicketRelation(ctx context.Context, relationID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.relations[relationID]; !exists {
		return fmt.Errorf("relation %d not found", relationID)
	}

	delete(r.relations, relationID)
	return nil
}
