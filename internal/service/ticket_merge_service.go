package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// TicketMergeService handles ticket merging and splitting operations
type TicketMergeService struct {
	repo repository.TicketMergeRepository
}

// NewTicketMergeService creates a new ticket merge service
func NewTicketMergeService(repo repository.TicketMergeRepository) *TicketMergeService {
	return &TicketMergeService{
		repo: repo,
	}
}

// MergeTree represents a hierarchical view of merged tickets
type MergeTree struct {
	ParentID      uint        `json:"parent_id"`
	Children      []MergeNode `json:"children"`
	TotalChildren int         `json:"total_children"`
	Depth         int         `json:"depth"`
}

// MergeNode represents a node in the merge tree
type MergeNode struct {
	TicketID   uint      `json:"ticket_id"`
	MergedAt   time.Time `json:"merged_at"`
	MergedBy   uint      `json:"merged_by"`
	Reason     string    `json:"reason"`
	IsActive   bool      `json:"is_active"`
}

// MergeTickets merges multiple child tickets into a parent ticket
func (s *TicketMergeService) MergeTickets(ctx context.Context, request *models.MergeRequest, userID uint) ([]uint, error) {
	// Validate the merge request first
	validation, err := s.ValidateMerge(ctx, request)
	if err != nil {
		return nil, err
	}
	
	if !validation.CanMerge {
		return nil, fmt.Errorf("merge validation failed: %v", validation.Errors)
	}
	
	var mergeIDs []uint
	
	// Create merge records for each child
	for _, childID := range request.ChildTicketIDs {
		merge := &models.TicketMerge{
			ParentTicketID: request.ParentTicketID,
			ChildTicketID:  childID,
			MergedBy:       userID,
			Reason:         request.Reason,
			Notes:          request.Notes,
		}
		
		if err := s.repo.CreateMerge(ctx, merge); err != nil {
			// Rollback previous merges on error
			for _, id := range mergeIDs {
				s.repo.UnmergeTicket(ctx, id, userID)
			}
			return nil, fmt.Errorf("failed to merge ticket %d: %w", childID, err)
		}
		
		mergeIDs = append(mergeIDs, merge.ID)
		
		// TODO: If MergeMessages is true, copy messages to parent
		// TODO: If MergeAttachments is true, copy attachments to parent
		// TODO: If CloseChildren is true, update ticket status to closed
	}
	
	return mergeIDs, nil
}

// ValidateMerge validates if tickets can be merged
func (s *TicketMergeService) ValidateMerge(ctx context.Context, request *models.MergeRequest) (*models.MergeValidation, error) {
	validation := &models.MergeValidation{
		CanMerge:    true,
		Warnings:    []string{},
		Errors:      []string{},
		Suggestions: []string{},
	}
	
	// Check for self-merge
	for _, childID := range request.ChildTicketIDs {
		if childID == request.ParentTicketID {
			validation.CanMerge = false
			validation.Errors = append(validation.Errors, 
				fmt.Sprintf("Ticket %d cannot merge into itself", childID))
		}
	}
	
	// Check if any child is already merged
	for _, childID := range request.ChildTicketIDs {
		isMerged, err := s.repo.IsMerged(ctx, childID)
		if err != nil {
			return validation, err
		}
		
		if isMerged {
			validation.CanMerge = false
			validation.Errors = append(validation.Errors, 
				fmt.Sprintf("Ticket %d is already merged", childID))
		}
	}
	
	// Check if parent is merged (circular merge prevention)
	parentMerged, err := s.repo.IsMerged(ctx, request.ParentTicketID)
	if err != nil {
		return validation, err
	}
	
	if parentMerged {
		validation.CanMerge = false
		validation.Errors = append(validation.Errors, 
			"Parent ticket is already merged as a child")
	}
	
	// Add warnings for large merges
	if len(request.ChildTicketIDs) > 10 {
		validation.Warnings = append(validation.Warnings, 
			fmt.Sprintf("Merging %d tickets at once may affect performance", 
				len(request.ChildTicketIDs)))
	}
	
	// Add suggestions
	if request.MergeMessages && len(request.ChildTicketIDs) > 5 {
		validation.Suggestions = append(validation.Suggestions, 
			"Consider reviewing messages before merging this many tickets")
	}
	
	return validation, nil
}

// UnmergeTicket unmerges a previously merged ticket
func (s *TicketMergeService) UnmergeTicket(ctx context.Context, request *models.UnmergeRequest, userID uint) error {
	// Find the merge record for this child ticket
	merge, err := s.repo.GetMergeByChild(ctx, request.TicketID)
	if err != nil {
		return fmt.Errorf("ticket %d is not merged", request.TicketID)
	}
	
	if !merge.IsActive {
		return fmt.Errorf("ticket %d is already unmerged", request.TicketID)
	}
	
	// Unmerge the ticket
	if err := s.repo.UnmergeTicket(ctx, merge.ID, userID); err != nil {
		return err
	}
	
	// TODO: If ReopenTicket is true, update ticket status to open
	
	return nil
}

// SplitTicket splits messages from one ticket into a new ticket
func (s *TicketMergeService) SplitTicket(ctx context.Context, request *models.SplitRequest, userID uint) *models.SplitResult {
	result := &models.SplitResult{
		Success: false,
	}
	
	// Generate new ticket ID and number (mock implementation)
	result.NewTicketID = uint(time.Now().Unix() % 10000)
	result.NewTicketNumber = fmt.Sprintf("SPLIT-%d", result.NewTicketID)
	
	// TODO: Create new ticket with specified title and queue
	// TODO: Move specified messages to new ticket
	// TODO: Copy attachments if requested
	
	result.MovedMessages = len(request.MessageIDs)
	
	// Create relation between tickets if requested
	if request.LinkTickets {
		relation := &models.TicketRelation{
			TicketID:        request.SourceTicketID,
			RelatedTicketID: result.NewTicketID,
			RelationType:    "related",
			CreatedBy:       userID,
			Notes:          "Split from original ticket",
		}
		
		if err := s.repo.CreateTicketRelation(ctx, relation); err != nil {
			result.Error = fmt.Sprintf("Failed to create relation: %v", err)
		}
	}
	
	result.Success = true
	return result
}

// GetMergeTree returns a hierarchical view of merged tickets
func (s *TicketMergeService) GetMergeTree(ctx context.Context, parentID uint) (*MergeTree, error) {
	tree := &MergeTree{
		ParentID: parentID,
		Children: []MergeNode{},
		Depth:    1,
	}
	
	// Get all merges for this parent
	merges, err := s.repo.GetMergesByParent(ctx, parentID)
	if err != nil {
		return tree, err
	}
	
	for _, merge := range merges {
		node := MergeNode{
			TicketID: merge.ChildTicketID,
			MergedAt: merge.MergedAt,
			MergedBy: merge.MergedBy,
			Reason:   merge.Reason,
			IsActive: merge.IsActive,
		}
		tree.Children = append(tree.Children, node)
	}
	
	tree.TotalChildren = len(tree.Children)
	
	return tree, nil
}

// IsTicketMerged checks if a ticket is currently merged
func (s *TicketMergeService) IsTicketMerged(ctx context.Context, ticketID uint) (bool, error) {
	return s.repo.IsMerged(ctx, ticketID)
}

// GetChildTickets returns all child ticket IDs for a parent
func (s *TicketMergeService) GetChildTickets(ctx context.Context, parentID uint) ([]uint, error) {
	return s.repo.GetAllChildren(ctx, parentID)
}

// GetMergeHistory returns the merge history for a ticket
func (s *TicketMergeService) GetMergeHistory(ctx context.Context, ticketID uint) ([]models.TicketMerge, error) {
	return s.repo.GetMergeHistory(ctx, ticketID)
}

// GetMergeStatistics returns merge statistics for a time period
func (s *TicketMergeService) GetMergeStatistics(ctx context.Context, from, to time.Time) (*models.MergeStatistics, error) {
	return s.repo.GetMergeStatistics(ctx, from, to)
}

// CreateTicketRelation creates a relation between two tickets
func (s *TicketMergeService) CreateTicketRelation(ctx context.Context, relation *models.TicketRelation) error {
	// Validate relation type
	validTypes := map[string]bool{
		"parent":     true,
		"child":      true,
		"duplicate":  true,
		"related":    true,
		"blocks":     true,
		"blocked_by": true,
	}
	
	if !validTypes[relation.RelationType] {
		return fmt.Errorf("invalid relation type: %s", relation.RelationType)
	}
	
	// Prevent self-relation
	if relation.TicketID == relation.RelatedTicketID {
		return fmt.Errorf("ticket cannot be related to itself")
	}
	
	return s.repo.CreateTicketRelation(ctx, relation)
}

// GetTicketRelations returns all relations for a ticket
func (s *TicketMergeService) GetTicketRelations(ctx context.Context, ticketID uint) ([]models.TicketRelation, error) {
	return s.repo.GetTicketRelations(ctx, ticketID)
}

// DeleteTicketRelation removes a ticket relation
func (s *TicketMergeService) DeleteTicketRelation(ctx context.Context, relationID uint) error {
	return s.repo.DeleteTicketRelation(ctx, relationID)
}

// GetRelatedTickets returns all tickets related to a given ticket
func (s *TicketMergeService) GetRelatedTickets(ctx context.Context, ticketID uint) (map[string][]uint, error) {
	relations, err := s.repo.GetTicketRelations(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	
	related := make(map[string][]uint)
	for _, rel := range relations {
		var relatedID uint
		if rel.TicketID == ticketID {
			relatedID = rel.RelatedTicketID
		} else {
			relatedID = rel.TicketID
		}
		
		related[rel.RelationType] = append(related[rel.RelationType], relatedID)
	}
	
	return related, nil
}

// BulkUnmerge unmerges multiple tickets at once
func (s *TicketMergeService) BulkUnmerge(ctx context.Context, ticketIDs []uint, reason string, userID uint) (int, error) {
	unmerged := 0
	
	for _, ticketID := range ticketIDs {
		request := &models.UnmergeRequest{
			TicketID:     ticketID,
			Reason:       reason,
			ReopenTicket: true,
		}
		
		if err := s.UnmergeTicket(ctx, request, userID); err == nil {
			unmerged++
		}
	}
	
	return unmerged, nil
}

// FindDuplicates suggests potential duplicate tickets
func (s *TicketMergeService) FindDuplicates(ctx context.Context, ticketID uint) ([]uint, error) {
	// This is a placeholder for duplicate detection logic
	// In a real implementation, this would:
	// 1. Analyze ticket title, content, and metadata
	// 2. Use search service to find similar tickets
	// 3. Apply ML models for similarity scoring
	// 4. Return ranked list of potential duplicates
	
	return []uint{}, nil
}

// AutoMergeByRules applies automatic merge rules
func (s *TicketMergeService) AutoMergeByRules(ctx context.Context, rules []MergeRule) (int, error) {
	// This would implement automatic merging based on predefined rules
	// For example: merge all tickets from same customer within 1 hour with same subject
	
	merged := 0
	// TODO: Implement rule-based auto-merging
	
	return merged, nil
}

// MergeRule defines a rule for automatic merging
type MergeRule struct {
	Name        string    `json:"name"`
	Condition   string    `json:"condition"`
	TimeWindow  int       `json:"time_window_hours"`
	MaxMerges   int       `json:"max_merges"`
	IsActive    bool      `json:"is_active"`
}