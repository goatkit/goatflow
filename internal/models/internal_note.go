package models

import (
	"time"
)

// InternalNote represents an internal note/comment on a ticket
type InternalNote struct {
	ID             uint      `json:"id"`
	TicketID       uint      `json:"ticket_id" binding:"required"`
	AuthorID       uint      `json:"author_id"`
	AuthorName     string    `json:"author_name"`
	AuthorEmail    string    `json:"author_email"`
	Content        string    `json:"content" binding:"required"`
	ContentType    string    `json:"content_type"` // text/plain or text/html
	IsInternal     bool      `json:"is_internal"`  // Always true for internal notes
	IsImportant    bool      `json:"is_important"` // Flag for important notes
	IsPinned       bool      `json:"is_pinned"`    // Pin to top of note list
	Category       string    `json:"category"`     // Optional categorization
	Tags           []string  `json:"tags"`         // Tags for filtering
	Attachments    []string  `json:"attachments"`  // Attachment URLs
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	EditedBy       uint      `json:"edited_by,omitempty"`
	EditHistory    []NoteEdit `json:"edit_history,omitempty"`
	
	// Related entities
	MentionedUsers []uint    `json:"mentioned_users,omitempty"` // @mentions
	RelatedTickets []uint    `json:"related_tickets,omitempty"` // Referenced ticket IDs
	
	// Visibility control
	VisibleToRoles []string  `json:"visible_to_roles,omitempty"` // Role-based visibility
	VisibleToUsers []uint    `json:"visible_to_users,omitempty"` // User-specific visibility
}

// NoteEdit represents an edit history entry
type NoteEdit struct {
	ID           uint      `json:"id"`
	NoteID       uint      `json:"note_id"`
	EditorID     uint      `json:"editor_id"`
	EditorName   string    `json:"editor_name"`
	OldContent   string    `json:"old_content"`
	NewContent   string    `json:"new_content"`
	EditedAt     time.Time `json:"edited_at"`
	EditReason   string    `json:"edit_reason,omitempty"`
}

// NoteCategory represents categories for organizing notes
type NoteCategory struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"` // For UI display
	Icon        string `json:"icon"`  // Icon identifier
	Order       int    `json:"order"`
	Active      bool   `json:"active"`
}

// NoteFilter for searching/filtering notes
type NoteFilter struct {
	TicketID      uint     `json:"ticket_id,omitempty"`
	AuthorID      uint     `json:"author_id,omitempty"`
	Category      string   `json:"category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	IsImportant   *bool    `json:"is_important,omitempty"`
	IsPinned      *bool    `json:"is_pinned,omitempty"`
	DateFrom      *time.Time `json:"date_from,omitempty"`
	DateTo        *time.Time `json:"date_to,omitempty"`
	SearchQuery   string   `json:"search_query,omitempty"`
	IncludeEdited bool     `json:"include_edited,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Offset        int      `json:"offset,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"` // created_at, updated_at
	SortOrder     string   `json:"sort_order,omitempty"` // asc, desc
}

// NoteStatistics represents statistics about notes
type NoteStatistics struct {
	TotalNotes      int            `json:"total_notes"`
	ImportantNotes  int            `json:"important_notes"`
	PinnedNotes     int            `json:"pinned_notes"`
	NotesByAuthor   map[string]int `json:"notes_by_author"`
	NotesByCategory map[string]int `json:"notes_by_category"`
	AverageLength   int            `json:"average_length"`
	LastNoteDate    *time.Time     `json:"last_note_date,omitempty"`
}

// NoteTemplate represents a template for quick note creation
type NoteTemplate struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name" binding:"required"`
	Content     string    `json:"content" binding:"required"`
	Category    string    `json:"category"`
	Tags        []string  `json:"tags"`
	IsImportant bool      `json:"is_important"`
	Variables   []string  `json:"variables"` // Template variables like {{reason}}
	CreatedBy   uint      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UsageCount  int       `json:"usage_count"`
}

// NoteMention represents a user mention in a note
type NoteMention struct {
	ID           uint      `json:"id"`
	NoteID       uint      `json:"note_id"`
	MentionedID  uint      `json:"mentioned_id"`
	MentionedBy  uint      `json:"mentioned_by"`
	MentionType  string    `json:"mention_type"` // @user, @team, @role
	IsRead       bool      `json:"is_read"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	NotifiedAt   *time.Time `json:"notified_at,omitempty"`
}

// NoteActivity represents activity on internal notes
type NoteActivity struct {
	ID           uint      `json:"id"`
	NoteID       uint      `json:"note_id"`
	TicketID     uint      `json:"ticket_id"`
	UserID       uint      `json:"user_id"`
	UserName     string    `json:"user_name"`
	ActivityType string    `json:"activity_type"` // created, edited, deleted, pinned, unpinned
	Details      string    `json:"details"`
	Timestamp    time.Time `json:"timestamp"`
}

// NoteExport represents exported note data
type NoteExport struct {
	TicketNumber string         `json:"ticket_number"`
	TicketTitle  string         `json:"ticket_title"`
	Notes        []InternalNote `json:"notes"`
	ExportedAt   time.Time      `json:"exported_at"`
	ExportedBy   string         `json:"exported_by"`
	Format       string         `json:"format"` // json, csv, pdf
}