package models

import (
	"time"
)

// TicketMerge represents a merge operation between tickets
type TicketMerge struct {
	ID             uint      `json:"id"`
	ParentTicketID uint      `json:"parent_ticket_id" binding:"required"`
	ChildTicketID  uint      `json:"child_ticket_id" binding:"required"`
	MergedBy       uint      `json:"merged_by" binding:"required"`
	MergedAt       time.Time `json:"merged_at"`
	UnmergedBy     *uint     `json:"unmerged_by,omitempty"`
	UnmergedAt     *time.Time `json:"unmerged_at,omitempty"`
	Reason         string    `json:"reason"`
	Notes          string    `json:"notes,omitempty"`
	IsActive       bool      `json:"is_active"`
}

// MergeRequest represents a request to merge tickets
type MergeRequest struct {
	ParentTicketID uint     `json:"parent_ticket_id" binding:"required"`
	ChildTicketIDs []uint   `json:"child_ticket_ids" binding:"required,min=1"`
	Reason         string   `json:"reason" binding:"required"`
	Notes          string   `json:"notes,omitempty"`
	MergeMessages  bool     `json:"merge_messages"`
	MergeAttachments bool   `json:"merge_attachments"`
	CloseChildren  bool     `json:"close_children"`
}

// UnmergeRequest represents a request to unmerge tickets
type UnmergeRequest struct {
	TicketID uint   `json:"ticket_id" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
	ReopenTicket bool `json:"reopen_ticket"`
}

// SplitRequest represents a request to split a ticket
type SplitRequest struct {
	SourceTicketID uint           `json:"source_ticket_id" binding:"required"`
	MessageIDs     []uint         `json:"message_ids" binding:"required,min=1"`
	NewTicketTitle string         `json:"new_ticket_title" binding:"required"`
	NewTicketQueue uint           `json:"new_ticket_queue" binding:"required"`
	CopyAttachments bool          `json:"copy_attachments"`
	LinkTickets    bool           `json:"link_tickets"`
	SplitCriteria  SplitCriteria  `json:"split_criteria,omitempty"`
}

// SplitCriteria defines criteria for automatic ticket splitting
type SplitCriteria struct {
	ByCustomer    bool      `json:"by_customer"`
	ByTimeGap     bool      `json:"by_time_gap"`
	TimeGapHours  int       `json:"time_gap_hours,omitempty"`
	BySubject     bool      `json:"by_subject"`
	ByKeywords    []string  `json:"by_keywords,omitempty"`
}

// MergeHistory represents the history of merge operations
type MergeHistory struct {
	ID          uint      `json:"id"`
	TicketID    uint      `json:"ticket_id"`
	Operation   string    `json:"operation"` // "merge", "unmerge", "split"
	RelatedTickets []uint `json:"related_tickets"`
	PerformedBy uint      `json:"performed_by"`
	PerformedAt time.Time `json:"performed_at"`
	Details     string    `json:"details"`
}

// MergeStatistics provides statistics about merge operations
type MergeStatistics struct {
	TotalMerges      int     `json:"total_merges"`
	ActiveMerges     int     `json:"active_merges"`
	UnmergedCount    int     `json:"unmerged_count"`
	AverageChildCount float64 `json:"average_child_count"`
	TopMergeReasons  []MergeReasonStat `json:"top_merge_reasons"`
	MergesByMonth    []MergeMonthStat  `json:"merges_by_month"`
}

// MergeReasonStat represents statistics for a merge reason
type MergeReasonStat struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// MergeMonthStat represents monthly merge statistics
type MergeMonthStat struct {
	Month  string `json:"month"`
	Year   int    `json:"year"`
	Merges int    `json:"merges"`
	Unmerges int  `json:"unmerges"`
}

// TicketRelation represents a relationship between tickets
type TicketRelation struct {
	ID           uint      `json:"id"`
	TicketID     uint      `json:"ticket_id" binding:"required"`
	RelatedTicketID uint   `json:"related_ticket_id" binding:"required"`
	RelationType string    `json:"relation_type" binding:"required"` // "parent", "child", "duplicate", "related", "blocks", "blocked_by"
	CreatedBy    uint      `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	Notes        string    `json:"notes,omitempty"`
}

// MergeValidation represents validation results for a merge operation
type MergeValidation struct {
	CanMerge bool     `json:"can_merge"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// SplitResult represents the result of a split operation
type SplitResult struct {
	Success        bool   `json:"success"`
	NewTicketID    uint   `json:"new_ticket_id,omitempty"`
	NewTicketNumber string `json:"new_ticket_number,omitempty"`
	MovedMessages  int    `json:"moved_messages"`
	CopiedAttachments int `json:"copied_attachments"`
	Error          string `json:"error,omitempty"`
}