package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

// InternalNoteService handles business logic for internal notes.
type InternalNoteService struct {
	repo repository.InternalNoteRepository
}

// NewInternalNoteService creates a new internal note service.
func NewInternalNoteService(repo repository.InternalNoteRepository) *InternalNoteService {
	return &InternalNoteService{
		repo: repo,
	}
}

// CreateNote creates a new internal note.
func (s *InternalNoteService) CreateNote(ctx context.Context, note *models.InternalNote) error {
	// Validate content
	if err := s.validateNoteContent(note.Content); err != nil {
		return err
	}

	// Set defaults
	note.IsInternal = true
	if note.ContentType == "" {
		note.ContentType = "text/plain"
	}

	// Detect mentions
	mentions := s.detectMentions(note.Content)
	for range mentions {
		// TODO: Look up user ID from username
		// For now, just track that there are mentions
		note.MentionedUsers = append(note.MentionedUsers, 0)
	}

	// Detect ticket references
	note.RelatedTickets = s.detectTicketReferences(note.Content)

	return s.repo.CreateNote(ctx, note)
}

// CreateNoteFromTemplate creates a note from a template.
func (s *InternalNoteService) CreateNoteFromTemplate(ctx context.Context, ticketID uint, templateID uint, variables map[string]string, authorID uint) (*models.InternalNote, error) {
	// Get template
	template, err := s.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Substitute variables
	content := s.substituteVariables(template.Content, variables)

	// Create note
	note := &models.InternalNote{
		TicketID:    ticketID,
		AuthorID:    authorID,
		Content:     content,
		Category:    template.Category,
		Tags:        template.Tags,
		IsImportant: template.IsImportant,
		ContentType: "text/plain",
	}

	if err := s.CreateNote(ctx, note); err != nil {
		return nil, err
	}

	// Increment template usage
	s.repo.IncrementTemplateUsage(ctx, templateID)

	return note, nil
}

// GetNote retrieves a note by ID.
func (s *InternalNoteService) GetNote(ctx context.Context, id uint) (*models.InternalNote, error) {
	return s.repo.GetNoteByID(ctx, id)
}

// GetNotesByTicket retrieves all notes for a ticket.
func (s *InternalNoteService) GetNotesByTicket(ctx context.Context, ticketID uint) ([]models.InternalNote, error) {
	return s.repo.GetNotesByTicket(ctx, ticketID)
}

// GetPinnedNotes retrieves pinned notes for a ticket.
func (s *InternalNoteService) GetPinnedNotes(ctx context.Context, ticketID uint) ([]models.InternalNote, error) {
	return s.repo.GetPinnedNotes(ctx, ticketID)
}

// GetImportantNotes retrieves important notes for a ticket.
func (s *InternalNoteService) GetImportantNotes(ctx context.Context, ticketID uint) ([]models.InternalNote, error) {
	return s.repo.GetImportantNotes(ctx, ticketID)
}

// UpdateNote updates an existing note.
func (s *InternalNoteService) UpdateNote(ctx context.Context, note *models.InternalNote, editReason string) error {
	// Validate content
	if err := s.validateNoteContent(note.Content); err != nil {
		return err
	}

	// Verify note exists
	_, err := s.repo.GetNoteByID(ctx, note.ID)
	if err != nil {
		return err
	}

	// Note: Edit history is handled by the repository's UpdateNote method
	// We just need to pass the edit reason if needed

	// Update mentions
	mentions := s.detectMentions(note.Content)
	note.MentionedUsers = []uint{}
	for range mentions {
		// TODO: Look up user ID from username
		note.MentionedUsers = append(note.MentionedUsers, 0)
	}

	// Update ticket references
	note.RelatedTickets = s.detectTicketReferences(note.Content)

	// Log activity
	activity := &models.NoteActivity{
		NoteID:       note.ID,
		TicketID:     note.TicketID,
		UserID:       note.EditedBy,
		ActivityType: "edited",
		Details:      editReason,
	}
	s.repo.LogActivity(ctx, activity)

	return s.repo.UpdateNote(ctx, note)
}

// DeleteNote deletes a note.
func (s *InternalNoteService) DeleteNote(ctx context.Context, id uint, deletedBy uint) error {
	// Get note for activity logging
	note, err := s.repo.GetNoteByID(ctx, id)
	if err != nil {
		return err
	}

	// Log activity
	activity := &models.NoteActivity{
		NoteID:       id,
		TicketID:     note.TicketID,
		UserID:       deletedBy,
		ActivityType: "deleted",
		Details:      "Note deleted",
	}
	s.repo.LogActivity(ctx, activity)

	return s.repo.DeleteNote(ctx, id)
}

// PinNote pins or unpins a note.
func (s *InternalNoteService) PinNote(ctx context.Context, noteID uint, userID uint) error {
	note, err := s.repo.GetNoteByID(ctx, noteID)
	if err != nil {
		return err
	}

	note.IsPinned = !note.IsPinned
	note.EditedBy = userID

	// Log activity
	activityType := "pinned"
	if !note.IsPinned {
		activityType = "unpinned"
	}

	activity := &models.NoteActivity{
		NoteID:       noteID,
		TicketID:     note.TicketID,
		UserID:       userID,
		ActivityType: activityType,
	}
	s.repo.LogActivity(ctx, activity)

	return s.repo.UpdateNote(ctx, note)
}

// MarkImportant marks or unmarks a note as important.
func (s *InternalNoteService) MarkImportant(ctx context.Context, noteID uint, important bool, userID uint) error {
	note, err := s.repo.GetNoteByID(ctx, noteID)
	if err != nil {
		return err
	}

	note.IsImportant = important
	note.EditedBy = userID

	// Log activity
	activityType := "marked_important"
	if !important {
		activityType = "unmarked_important"
	}

	activity := &models.NoteActivity{
		NoteID:       noteID,
		TicketID:     note.TicketID,
		UserID:       userID,
		ActivityType: activityType,
	}
	s.repo.LogActivity(ctx, activity)

	return s.repo.UpdateNote(ctx, note)
}

// SearchNotes searches for notes.
func (s *InternalNoteService) SearchNotes(ctx context.Context, filter *models.NoteFilter) ([]models.InternalNote, error) {
	return s.repo.SearchNotes(ctx, filter)
}

// GetEditHistory retrieves edit history for a note.
func (s *InternalNoteService) GetEditHistory(ctx context.Context, noteID uint) ([]models.NoteEdit, error) {
	return s.repo.GetEditHistory(ctx, noteID)
}

// GetTicketNoteSummary gets statistics for a ticket's notes.
func (s *InternalNoteService) GetTicketNoteSummary(ctx context.Context, ticketID uint) (*models.NoteStatistics, error) {
	return s.repo.GetNoteStatistics(ctx, ticketID)
}

// GetRecentActivity gets recent activity for a ticket.
func (s *InternalNoteService) GetRecentActivity(ctx context.Context, ticketID uint, limit int) ([]models.NoteActivity, error) {
	return s.repo.GetActivityLog(ctx, ticketID, limit)
}

// GetUserMentions gets mentions for a user.
func (s *InternalNoteService) GetUserMentions(ctx context.Context, userID uint) ([]models.NoteMention, error) {
	return s.repo.GetMentionsByUser(ctx, userID)
}

// MarkMentionRead marks a mention as read.
func (s *InternalNoteService) MarkMentionRead(ctx context.Context, mentionID uint) error {
	return s.repo.MarkMentionAsRead(ctx, mentionID)
}

// CreateTemplate creates a note template.
func (s *InternalNoteService) CreateTemplate(ctx context.Context, template *models.NoteTemplate) error {
	// Extract variables from content
	template.Variables = s.extractTemplateVariables(template.Content)
	return s.repo.CreateTemplate(ctx, template)
}

// GetTemplates retrieves all templates.
func (s *InternalNoteService) GetTemplates(ctx context.Context) ([]models.NoteTemplate, error) {
	return s.repo.GetTemplates(ctx)
}

// UpdateTemplate updates a template.
func (s *InternalNoteService) UpdateTemplate(ctx context.Context, template *models.NoteTemplate) error {
	// Re-extract variables
	template.Variables = s.extractTemplateVariables(template.Content)
	return s.repo.UpdateTemplate(ctx, template)
}

// DeleteTemplate deletes a template.
func (s *InternalNoteService) DeleteTemplate(ctx context.Context, id uint) error {
	return s.repo.DeleteTemplate(ctx, id)
}

// GetCategories retrieves all note categories.
func (s *InternalNoteService) GetCategories(ctx context.Context) ([]models.NoteCategory, error) {
	return s.repo.GetCategories(ctx)
}

// ExportNotes exports notes in various formats.
func (s *InternalNoteService) ExportNotes(ctx context.Context, ticketID uint, format string, exportedBy string) ([]byte, error) {
	notes, err := s.repo.GetNotesByTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return s.exportAsJSON(notes, ticketID, exportedBy)
	case "csv":
		return s.exportAsCSV(notes)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportAsJSON exports notes as JSON.
func (s *InternalNoteService) exportAsJSON(notes []models.InternalNote, ticketID uint, exportedBy string) ([]byte, error) {
	export := models.NoteExport{
		TicketNumber: fmt.Sprintf("TICKET-%d", ticketID),
		TicketTitle:  "Ticket Title", // TODO: Get from ticket service
		Notes:        notes,
		ExportedAt:   time.Now(),
		ExportedBy:   exportedBy,
		Format:       "json",
	}

	return json.MarshalIndent(export, "", "  ")
}

// exportAsCSV exports notes as CSV.
func (s *InternalNoteService) exportAsCSV(notes []models.InternalNote) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"ID", "Created At", "Author", "Category", "Content", "Important", "Pinned", "Tags"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write notes
	for _, note := range notes {
		record := []string{
			fmt.Sprintf("%d", note.ID),
			note.CreatedAt.Format("2006-01-02 15:04:05"),
			note.AuthorName,
			note.Category,
			note.Content,
			fmt.Sprintf("%t", note.IsImportant),
			fmt.Sprintf("%t", note.IsPinned),
			strings.Join(note.Tags, ";"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return []byte(buf.String()), nil
}

// validateNoteContent validates note content.
func (s *InternalNoteService) validateNoteContent(content string) error {
	if content == "" {
		return fmt.Errorf("note content cannot be empty")
	}
	if len(content) > 10000 {
		return fmt.Errorf("note content too long (max 10000 characters)")
	}
	return nil
}

// detectMentions detects @mentions in content.
func (s *InternalNoteService) detectMentions(content string) []string {
	re := regexp.MustCompile(`@(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	mentionMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			mentionMap[match[1]] = true
		}
	}

	mentions := make([]string, 0, len(mentionMap))
	for mention := range mentionMap {
		mentions = append(mentions, mention)
	}

	return mentions
}

// detectTicketReferences detects ticket references in content.
func (s *InternalNoteService) detectTicketReferences(content string) []uint {
	re := regexp.MustCompile(`(?:TICKET-|#)(\d+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	ticketMap := make(map[uint]bool)
	for _, match := range matches {
		if len(match) > 1 {
			var ticketID uint
			fmt.Sscanf(match[1], "%d", &ticketID)
			if ticketID > 0 {
				ticketMap[ticketID] = true
			}
		}
	}

	tickets := make([]uint, 0, len(ticketMap))
	for ticketID := range ticketMap {
		tickets = append(tickets, ticketID)
	}

	return tickets
}

// substituteVariables replaces template variables with values.
func (s *InternalNoteService) substituteVariables(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

// extractTemplateVariables extracts variables from template content.
func (s *InternalNoteService) extractTemplateVariables(content string) []string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	varMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 0 {
			varMap[match[0]] = true
		}
	}

	variables := make([]string, 0, len(varMap))
	for variable := range varMap {
		variables = append(variables, variable)
	}

	return variables
}
