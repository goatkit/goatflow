package repository

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

// MemoryInternalNoteRepository is an in-memory implementation of InternalNoteRepository.
type MemoryInternalNoteRepository struct {
	mu             sync.RWMutex
	notes          map[uint]*models.InternalNote
	editHistory    map[uint][]models.NoteEdit
	categories     map[uint]*models.NoteCategory
	templates      map[uint]*models.NoteTemplate
	mentions       map[uint]*models.NoteMention
	activities     []models.NoteActivity
	nextID         uint
	nextEditID     uint
	nextCatID      uint
	nextTemplID    uint
	nextMentionID  uint
	nextActivityID uint
}

// NewMemoryInternalNoteRepository creates a new in-memory internal note repository.
func NewMemoryInternalNoteRepository() *MemoryInternalNoteRepository {
	return &MemoryInternalNoteRepository{
		notes:          make(map[uint]*models.InternalNote),
		editHistory:    make(map[uint][]models.NoteEdit),
		categories:     make(map[uint]*models.NoteCategory),
		templates:      make(map[uint]*models.NoteTemplate),
		mentions:       make(map[uint]*models.NoteMention),
		activities:     []models.NoteActivity{},
		nextID:         1,
		nextEditID:     1,
		nextCatID:      1,
		nextTemplID:    1,
		nextMentionID:  1,
		nextActivityID: 1,
	}
}

// CreateNote creates a new internal note.
func (r *MemoryInternalNoteRepository) CreateNote(ctx context.Context, note *models.InternalNote) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	note.ID = r.nextID
	r.nextID++
	note.CreatedAt = time.Now()
	note.UpdatedAt = note.CreatedAt
	note.IsInternal = true // Always true for internal notes

	// Create a copy to store
	stored := *note
	r.notes[note.ID] = &stored

	// Log activity
	r.logActivityInternal("created", note.ID, note.TicketID, note.AuthorID, note.AuthorName)

	return nil
}

// GetNoteByID retrieves a note by ID.
func (r *MemoryInternalNoteRepository) GetNoteByID(ctx context.Context, id uint) (*models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	note, exists := r.notes[id]
	if !exists {
		return nil, fmt.Errorf("note with ID %d not found", id)
	}

	// Return a copy
	result := *note
	return &result, nil
}

// GetNotesByTicket retrieves all notes for a ticket.
func (r *MemoryInternalNoteRepository) GetNotesByTicket(ctx context.Context, ticketID uint) ([]models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var notes []models.InternalNote
	for _, note := range r.notes {
		if note.TicketID == ticketID {
			notes = append(notes, *note)
		}
	}

	// Sort by creation date (newest first)
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	return notes, nil
}

// GetPinnedNotes retrieves pinned notes for a ticket.
func (r *MemoryInternalNoteRepository) GetPinnedNotes(ctx context.Context, ticketID uint) ([]models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var notes []models.InternalNote
	for _, note := range r.notes {
		if note.TicketID == ticketID && note.IsPinned {
			notes = append(notes, *note)
		}
	}

	// Sort by creation date
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	return notes, nil
}

// GetImportantNotes retrieves important notes for a ticket.
func (r *MemoryInternalNoteRepository) GetImportantNotes(ctx context.Context, ticketID uint) ([]models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var notes []models.InternalNote
	for _, note := range r.notes {
		if note.TicketID == ticketID && note.IsImportant {
			notes = append(notes, *note)
		}
	}

	// Sort by creation date
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	return notes, nil
}

// UpdateNote updates an existing note.
func (r *MemoryInternalNoteRepository) UpdateNote(ctx context.Context, note *models.InternalNote) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.notes[note.ID]
	if !exists {
		return fmt.Errorf("note with ID %d not found", note.ID)
	}

	// Create edit history entry if content changed
	if existing.Content != note.Content {
		edit := models.NoteEdit{
			ID:         r.nextEditID,
			NoteID:     note.ID,
			EditorID:   note.EditedBy,
			OldContent: existing.Content,
			NewContent: note.Content,
			EditedAt:   time.Now(),
		}
		r.nextEditID++

		if r.editHistory[note.ID] == nil {
			r.editHistory[note.ID] = []models.NoteEdit{}
		}
		r.editHistory[note.ID] = append(r.editHistory[note.ID], edit)

		// Add to note's edit history
		note.EditHistory = r.editHistory[note.ID]
	}

	// Preserve creation time
	note.CreatedAt = existing.CreatedAt
	note.UpdatedAt = time.Now()

	// Update with a copy
	updated := *note
	r.notes[note.ID] = &updated

	// Log activity
	r.logActivityInternal("edited", note.ID, note.TicketID, note.EditedBy, "")

	return nil
}

// DeleteNote deletes a note.
func (r *MemoryInternalNoteRepository) DeleteNote(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	note, exists := r.notes[id]
	if !exists {
		return fmt.Errorf("note with ID %d not found", id)
	}

	// Log activity before deletion
	r.logActivityInternal("deleted", id, note.TicketID, 0, "")

	delete(r.notes, id)
	delete(r.editHistory, id)
	return nil
}

// SearchNotes searches notes based on filter criteria.
func (r *MemoryInternalNoteRepository) SearchNotes(ctx context.Context, filter *models.NoteFilter) ([]models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]models.InternalNote, 0, len(r.notes))
	query := strings.ToLower(filter.SearchQuery)

	for _, note := range r.notes {
		// Apply filters
		if filter.TicketID != 0 && note.TicketID != filter.TicketID {
			continue
		}

		if filter.AuthorID != 0 && note.AuthorID != filter.AuthorID {
			continue
		}

		if filter.Category != "" && note.Category != filter.Category {
			continue
		}

		if filter.IsImportant != nil && note.IsImportant != *filter.IsImportant {
			continue
		}

		if filter.IsPinned != nil && note.IsPinned != *filter.IsPinned {
			continue
		}

		// Date range filter
		if filter.DateFrom != nil && note.CreatedAt.Before(*filter.DateFrom) {
			continue
		}

		if filter.DateTo != nil && note.CreatedAt.After(*filter.DateTo) {
			continue
		}

		// Tags filter
		if len(filter.Tags) > 0 {
			hasTag := false
			for _, filterTag := range filter.Tags {
				for _, noteTag := range note.Tags {
					if noteTag == filterTag {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// Text search
		if query != "" {
			if !strings.Contains(strings.ToLower(note.Content), query) &&
				!strings.Contains(strings.ToLower(note.Category), query) {
				// Check in tags
				tagMatch := false
				for _, tag := range note.Tags {
					if strings.Contains(strings.ToLower(tag), query) {
						tagMatch = true
						break
					}
				}
				if !tagMatch {
					continue
				}
			}
		}

		results = append(results, *note)
	}

	// Sort
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}

	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(results, func(i, j int) bool {
		switch sortBy {
		case "updated_at":
			if sortOrder == "asc" {
				return results[i].UpdatedAt.Before(results[j].UpdatedAt)
			}
			return results[i].UpdatedAt.After(results[j].UpdatedAt)
		default: // created_at
			if sortOrder == "asc" {
				return results[i].CreatedAt.Before(results[j].CreatedAt)
			}
			return results[i].CreatedAt.After(results[j].CreatedAt)
		}
	})

	// Apply pagination
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// GetNotesByAuthor retrieves notes by author.
func (r *MemoryInternalNoteRepository) GetNotesByAuthor(ctx context.Context, authorID uint) ([]models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var notes []models.InternalNote
	for _, note := range r.notes {
		if note.AuthorID == authorID {
			notes = append(notes, *note)
		}
	}

	// Sort by creation date
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	return notes, nil
}

// GetNotesByCategory retrieves notes by category.
func (r *MemoryInternalNoteRepository) GetNotesByCategory(ctx context.Context, category string) ([]models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var notes []models.InternalNote
	for _, note := range r.notes {
		if note.Category == category {
			notes = append(notes, *note)
		}
	}

	// Sort by creation date
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	return notes, nil
}

// GetNotesByTags retrieves notes by tags.
func (r *MemoryInternalNoteRepository) GetNotesByTags(ctx context.Context, tags []string) ([]models.InternalNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var notes []models.InternalNote
	for _, note := range r.notes {
		for _, tag := range tags {
			for _, noteTag := range note.Tags {
				if noteTag == tag {
					notes = append(notes, *note)
					goto nextNote
				}
			}
		}
	nextNote:
	}

	// Sort by creation date
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	return notes, nil
}

// AddEditHistory adds an edit history entry.
func (r *MemoryInternalNoteRepository) AddEditHistory(ctx context.Context, edit *models.NoteEdit) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	edit.ID = r.nextEditID
	r.nextEditID++
	edit.EditedAt = time.Now()

	if r.editHistory[edit.NoteID] == nil {
		r.editHistory[edit.NoteID] = []models.NoteEdit{}
	}
	r.editHistory[edit.NoteID] = append(r.editHistory[edit.NoteID], *edit)

	return nil
}

// GetEditHistory retrieves edit history for a note.
func (r *MemoryInternalNoteRepository) GetEditHistory(ctx context.Context, noteID uint) ([]models.NoteEdit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	history := r.editHistory[noteID]
	if history == nil {
		return []models.NoteEdit{}, nil
	}

	// Return a copy
	result := make([]models.NoteEdit, len(history))
	copy(result, history)

	// Sort by edit time (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].EditedAt.After(result[j].EditedAt)
	})

	return result, nil
}

// GetNoteStatistics retrieves statistics for notes.
func (r *MemoryInternalNoteRepository) GetNoteStatistics(ctx context.Context, ticketID uint) (*models.NoteStatistics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &models.NoteStatistics{
		NotesByAuthor:   make(map[string]int),
		NotesByCategory: make(map[string]int),
	}

	totalLength := 0
	var lastDate *time.Time

	for _, note := range r.notes {
		if note.TicketID != ticketID {
			continue
		}

		stats.TotalNotes++

		if note.IsImportant {
			stats.ImportantNotes++
		}

		if note.IsPinned {
			stats.PinnedNotes++
		}

		// Count by author
		authorKey := strconv.Itoa(int(note.AuthorID))
		stats.NotesByAuthor[authorKey]++

		// Count by category
		if note.Category != "" {
			stats.NotesByCategory[note.Category]++
		}

		// Track average length
		totalLength += len(note.Content)

		// Track last note date
		if lastDate == nil || note.CreatedAt.After(*lastDate) {
			lastDate = &note.CreatedAt
		}
	}

	if stats.TotalNotes > 0 {
		stats.AverageLength = totalLength / stats.TotalNotes
	}
	stats.LastNoteDate = lastDate

	return stats, nil
}

// GetCategories retrieves all unique categories.
func (r *MemoryInternalNoteRepository) GetCategories(ctx context.Context) ([]models.NoteCategory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// First check if we have predefined categories
	if len(r.categories) > 0 {
		var categories []models.NoteCategory
		for _, cat := range r.categories {
			if cat.Active {
				categories = append(categories, *cat)
			}
		}

		// Sort by order
		sort.Slice(categories, func(i, j int) bool {
			return categories[i].Order < categories[j].Order
		})

		return categories, nil
	}

	// Otherwise, collect unique categories from notes
	categoryMap := make(map[string]int)
	for _, note := range r.notes {
		if note.Category != "" {
			categoryMap[note.Category]++
		}
	}

	// Convert to slice of NoteCategory
	categories := make([]models.NoteCategory, 0, len(categoryMap))
	order := 1
	for name, count := range categoryMap {
		categories = append(categories, models.NoteCategory{
			ID:          uint(order),
			Name:        name,
			Description: fmt.Sprintf("%d notes", count),
			Order:       order,
			Active:      true,
		})
		order++
	}

	// Sort by name
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	// Re-assign IDs after sorting
	for i := range categories {
		categories[i].ID = uint(i + 1)
		categories[i].Order = i + 1
	}

	return categories, nil
}

// CreateCategory creates a new category.
func (r *MemoryInternalNoteRepository) CreateCategory(ctx context.Context, category *models.NoteCategory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	category.ID = r.nextCatID
	r.nextCatID++

	stored := *category
	r.categories[category.ID] = &stored

	return nil
}

// UpdateCategory updates a category.
func (r *MemoryInternalNoteRepository) UpdateCategory(ctx context.Context, category *models.NoteCategory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.categories[category.ID]; !exists {
		return fmt.Errorf("category with ID %d not found", category.ID)
	}

	updated := *category
	r.categories[category.ID] = &updated

	return nil
}

// DeleteCategory deletes a category.
func (r *MemoryInternalNoteRepository) DeleteCategory(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.categories[id]; !exists {
		return fmt.Errorf("category with ID %d not found", id)
	}

	delete(r.categories, id)
	return nil
}

// CreateTemplate creates a new template.
func (r *MemoryInternalNoteRepository) CreateTemplate(ctx context.Context, template *models.NoteTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	template.ID = r.nextTemplID
	r.nextTemplID++
	template.CreatedAt = time.Now()

	stored := *template
	r.templates[template.ID] = &stored

	return nil
}

// GetTemplates retrieves all templates.
func (r *MemoryInternalNoteRepository) GetTemplates(ctx context.Context) ([]models.NoteTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	templates := make([]models.NoteTemplate, 0, len(r.templates))
	for _, tmpl := range r.templates {
		templates = append(templates, *tmpl)
	}

	// Sort by usage count
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].UsageCount != templates[j].UsageCount {
			return templates[i].UsageCount > templates[j].UsageCount
		}
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

// GetTemplateByID retrieves a template by ID.
func (r *MemoryInternalNoteRepository) GetTemplateByID(ctx context.Context, id uint) (*models.NoteTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	template, exists := r.templates[id]
	if !exists {
		return nil, fmt.Errorf("template with ID %d not found", id)
	}

	result := *template
	return &result, nil
}

// UpdateTemplate updates a template.
func (r *MemoryInternalNoteRepository) UpdateTemplate(ctx context.Context, template *models.NoteTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[template.ID]; !exists {
		return fmt.Errorf("template with ID %d not found", template.ID)
	}

	updated := *template
	r.templates[template.ID] = &updated

	return nil
}

// DeleteTemplate deletes a template.
func (r *MemoryInternalNoteRepository) DeleteTemplate(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[id]; !exists {
		return fmt.Errorf("template with ID %d not found", id)
	}

	delete(r.templates, id)
	return nil
}

// IncrementTemplateUsage increments the usage count of a template.
func (r *MemoryInternalNoteRepository) IncrementTemplateUsage(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	template, exists := r.templates[id]
	if !exists {
		return fmt.Errorf("template with ID %d not found", id)
	}

	template.UsageCount++
	return nil
}

// CreateMention creates a new mention.
func (r *MemoryInternalNoteRepository) CreateMention(ctx context.Context, mention *models.NoteMention) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mention.ID = r.nextMentionID
	r.nextMentionID++

	stored := *mention
	r.mentions[mention.ID] = &stored

	return nil
}

// GetMentionsByUser retrieves mentions for a user.
func (r *MemoryInternalNoteRepository) GetMentionsByUser(ctx context.Context, userID uint) ([]models.NoteMention, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var mentions []models.NoteMention
	for _, mention := range r.mentions {
		if mention.MentionedID == userID {
			mentions = append(mentions, *mention)
		}
	}

	return mentions, nil
}

// MarkMentionAsRead marks a mention as read.
func (r *MemoryInternalNoteRepository) MarkMentionAsRead(ctx context.Context, mentionID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mention, exists := r.mentions[mentionID]
	if !exists {
		return fmt.Errorf("mention with ID %d not found", mentionID)
	}

	mention.IsRead = true
	now := time.Now()
	mention.ReadAt = &now

	return nil
}

// LogActivity logs an activity.
func (r *MemoryInternalNoteRepository) LogActivity(ctx context.Context, activity *models.NoteActivity) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	activity.ID = r.nextActivityID
	r.nextActivityID++
	activity.Timestamp = time.Now()

	r.activities = append(r.activities, *activity)

	return nil
}

// GetActivityLog retrieves activity log for a ticket.
func (r *MemoryInternalNoteRepository) GetActivityLog(ctx context.Context, ticketID uint, limit int) ([]models.NoteActivity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var activities []models.NoteActivity
	for _, activity := range r.activities {
		if activity.TicketID == ticketID {
			activities = append(activities, activity)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.After(activities[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && limit < len(activities) {
		activities = activities[:limit]
	}

	return activities, nil
}

// logActivityInternal is a helper to log activity internally.
func (r *MemoryInternalNoteRepository) logActivityInternal(activityType string, noteID, ticketID, userID uint, userName string) {
	activity := models.NoteActivity{
		ID:           r.nextActivityID,
		NoteID:       noteID,
		TicketID:     ticketID,
		UserID:       userID,
		UserName:     userName,
		ActivityType: activityType,
		Timestamp:    time.Now(),
	}
	r.nextActivityID++
	r.activities = append(r.activities, activity)
}
