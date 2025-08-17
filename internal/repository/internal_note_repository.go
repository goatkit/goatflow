package repository

import (
	"context"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// InternalNoteRepository defines the interface for internal note persistence
type InternalNoteRepository interface {
	// Note CRUD operations
	CreateNote(ctx context.Context, note *models.InternalNote) error
	GetNoteByID(ctx context.Context, id uint) (*models.InternalNote, error)
	GetNotesByTicket(ctx context.Context, ticketID uint) ([]models.InternalNote, error)
	GetPinnedNotes(ctx context.Context, ticketID uint) ([]models.InternalNote, error)
	GetImportantNotes(ctx context.Context, ticketID uint) ([]models.InternalNote, error)
	UpdateNote(ctx context.Context, note *models.InternalNote) error
	DeleteNote(ctx context.Context, id uint) error

	// Search and filtering
	SearchNotes(ctx context.Context, filter *models.NoteFilter) ([]models.InternalNote, error)
	GetNotesByAuthor(ctx context.Context, authorID uint) ([]models.InternalNote, error)
	GetNotesByCategory(ctx context.Context, category string) ([]models.InternalNote, error)
	GetNotesByTags(ctx context.Context, tags []string) ([]models.InternalNote, error)

	// Edit history
	AddEditHistory(ctx context.Context, edit *models.NoteEdit) error
	GetEditHistory(ctx context.Context, noteID uint) ([]models.NoteEdit, error)

	// Statistics
	GetNoteStatistics(ctx context.Context, ticketID uint) (*models.NoteStatistics, error)
	
	// Categories
	GetCategories(ctx context.Context) ([]models.NoteCategory, error)
	CreateCategory(ctx context.Context, category *models.NoteCategory) error
	UpdateCategory(ctx context.Context, category *models.NoteCategory) error
	DeleteCategory(ctx context.Context, id uint) error

	// Templates
	CreateTemplate(ctx context.Context, template *models.NoteTemplate) error
	GetTemplates(ctx context.Context) ([]models.NoteTemplate, error)
	GetTemplateByID(ctx context.Context, id uint) (*models.NoteTemplate, error)
	UpdateTemplate(ctx context.Context, template *models.NoteTemplate) error
	DeleteTemplate(ctx context.Context, id uint) error
	IncrementTemplateUsage(ctx context.Context, id uint) error

	// Mentions
	CreateMention(ctx context.Context, mention *models.NoteMention) error
	GetMentionsByUser(ctx context.Context, userID uint) ([]models.NoteMention, error)
	MarkMentionAsRead(ctx context.Context, mentionID uint) error

	// Activity tracking
	LogActivity(ctx context.Context, activity *models.NoteActivity) error
	GetActivityLog(ctx context.Context, ticketID uint, limit int) ([]models.NoteActivity, error)
}