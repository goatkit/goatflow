package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
)

func TestMemoryInternalNoteRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateNote", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		note := &models.InternalNote{
			TicketID:    123,
			AuthorID:    1,
			AuthorName:  "John Doe",
			AuthorEmail: "john@example.com",
			Content:     "This is an internal note about the ticket investigation.",
			ContentType: "text/plain",
			IsInternal:  true,
			Category:    "Investigation",
			Tags:        []string{"technical", "priority"},
		}

		err := repo.CreateNote(ctx, note)
		require.NoError(t, err)
		assert.NotZero(t, note.ID)
		assert.NotZero(t, note.CreatedAt)
	})

	t.Run("GetNoteByID", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create a note
		note := &models.InternalNote{
			TicketID: 123,
			AuthorID: 1,
			Content:  "Test note",
		}
		repo.CreateNote(ctx, note)

		// Retrieve it
		retrieved, err := repo.GetNoteByID(ctx, note.ID)
		require.NoError(t, err)
		assert.Equal(t, note.Content, retrieved.Content)
		assert.Equal(t, note.TicketID, retrieved.TicketID)
	})

	t.Run("GetNoteByID_NotFound", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		_, err := repo.GetNoteByID(ctx, 9999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetNotesByTicket", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create notes for different tickets
		for i := 0; i < 5; i++ {
			note := &models.InternalNote{
				TicketID: uint(123),
				AuthorID: 1,
				Content:  "Note " + string(rune('A'+i)),
			}
			repo.CreateNote(ctx, note)
		}

		// Create notes for another ticket
		for i := 0; i < 3; i++ {
			note := &models.InternalNote{
				TicketID: uint(456),
				AuthorID: 1,
				Content:  "Other note " + string(rune('A'+i)),
			}
			repo.CreateNote(ctx, note)
		}

		// Get notes for ticket 123
		notes, err := repo.GetNotesByTicket(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 5, len(notes))

		// Get notes for ticket 456
		notes, err = repo.GetNotesByTicket(ctx, 456)
		require.NoError(t, err)
		assert.Equal(t, 3, len(notes))
	})

	t.Run("GetPinnedNotes", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create mixed notes
		notes := []models.InternalNote{
			{TicketID: 123, Content: "Pinned 1", IsPinned: true},
			{TicketID: 123, Content: "Not pinned", IsPinned: false},
			{TicketID: 123, Content: "Pinned 2", IsPinned: true},
			{TicketID: 456, Content: "Other pinned", IsPinned: true},
		}

		for i := range notes {
			repo.CreateNote(ctx, &notes[i])
		}

		// Get pinned notes for ticket 123
		pinned, err := repo.GetPinnedNotes(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 2, len(pinned))
		for _, note := range pinned {
			assert.True(t, note.IsPinned)
			assert.Equal(t, uint(123), note.TicketID)
		}
	})

	t.Run("GetImportantNotes", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create mixed notes
		notes := []models.InternalNote{
			{TicketID: 123, Content: "Important 1", IsImportant: true},
			{TicketID: 123, Content: "Regular", IsImportant: false},
			{TicketID: 123, Content: "Important 2", IsImportant: true},
		}

		for i := range notes {
			repo.CreateNote(ctx, &notes[i])
		}

		// Get important notes
		important, err := repo.GetImportantNotes(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 2, len(important))
		for _, note := range important {
			assert.True(t, note.IsImportant)
		}
	})

	t.Run("UpdateNote", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create a note
		note := &models.InternalNote{
			TicketID: 123,
			Content:  "Original content",
			Category: "Original",
		}
		repo.CreateNote(ctx, note)
		originalID := note.ID

		// Update it
		note.Content = "Updated content"
		note.Category = "Updated"
		note.IsImportant = true
		note.EditedBy = 2

		err := repo.UpdateNote(ctx, note)
		require.NoError(t, err)

		// Verify update
		updated, err := repo.GetNoteByID(ctx, originalID)
		require.NoError(t, err)
		assert.Equal(t, "Updated content", updated.Content)
		assert.Equal(t, "Updated", updated.Category)
		assert.True(t, updated.IsImportant)
		assert.True(t, updated.UpdatedAt.After(updated.CreatedAt))
	})

	t.Run("DeleteNote", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create a note
		note := &models.InternalNote{
			TicketID: 123,
			Content:  "To delete",
		}
		repo.CreateNote(ctx, note)
		id := note.ID

		// Delete it
		err := repo.DeleteNote(ctx, id)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetNoteByID(ctx, id)
		assert.Error(t, err)
	})

	t.Run("SearchNotes", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create various notes
		notes := []models.InternalNote{
			{TicketID: 123, Content: "Customer complained about slow response", Category: "Customer"},
			{TicketID: 123, Content: "Technical investigation shows database issue", Category: "Technical"},
			{TicketID: 456, Content: "Customer happy with resolution", Category: "Customer"},
			{TicketID: 123, Content: "Database optimized", Category: "Technical"},
		}

		for i := range notes {
			repo.CreateNote(ctx, &notes[i])
		}

		// Search with filter
		filter := &models.NoteFilter{
			TicketID:    123,
			SearchQuery: "database",
		}

		results, err := repo.SearchNotes(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 2, len(results))

		// Search by category
		filter = &models.NoteFilter{
			Category: "Customer",
		}

		results, err = repo.SearchNotes(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
	})

	t.Run("GetNotesByAuthor", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create notes by different authors
		authors := []uint{1, 2, 1, 3, 1}
		for i, authorID := range authors {
			note := &models.InternalNote{
				TicketID: 123,
				AuthorID: authorID,
				Content:  "Note " + string(rune('A'+i)),
			}
			repo.CreateNote(ctx, note)
		}

		// Get notes by author 1
		notes, err := repo.GetNotesByAuthor(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, 3, len(notes))

		// Get notes by author 2
		notes, err = repo.GetNotesByAuthor(ctx, 2)
		require.NoError(t, err)
		assert.Equal(t, 1, len(notes))
	})

	t.Run("GetNoteStatistics", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create various notes
		notes := []models.InternalNote{
			{TicketID: 123, AuthorID: 1, Category: "Technical", IsImportant: true, IsPinned: true},
			{TicketID: 123, AuthorID: 1, Category: "Customer", IsImportant: false},
			{TicketID: 123, AuthorID: 2, Category: "Technical", IsImportant: true},
			{TicketID: 123, AuthorID: 2, Category: "Technical", IsPinned: true},
		}

		for i := range notes {
			repo.CreateNote(ctx, &notes[i])
		}

		// Get statistics
		stats, err := repo.GetNoteStatistics(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 4, stats.TotalNotes)
		assert.Equal(t, 2, stats.ImportantNotes)
		assert.Equal(t, 2, stats.PinnedNotes)
		assert.Equal(t, 2, stats.NotesByAuthor["1"])
		assert.Equal(t, 2, stats.NotesByAuthor["2"])
		assert.Equal(t, 3, stats.NotesByCategory["Technical"])
		assert.Equal(t, 1, stats.NotesByCategory["Customer"])
	})

	t.Run("AddEditHistory", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create a note
		note := &models.InternalNote{
			TicketID: 123,
			Content:  "Original content",
		}
		repo.CreateNote(ctx, note)

		// Add edit history
		edit := &models.NoteEdit{
			NoteID:     note.ID,
			EditorID:   2,
			EditorName: "Jane Doe",
			OldContent: "Original content",
			NewContent: "Updated content",
			EditReason: "Correcting information",
		}

		err := repo.AddEditHistory(ctx, edit)
		require.NoError(t, err)
		assert.NotZero(t, edit.ID)

		// Get edit history
		history, err := repo.GetEditHistory(ctx, note.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, len(history))
		assert.Equal(t, "Correcting information", history[0].EditReason)
	})

	t.Run("GetCategories", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create notes in various categories
		categories := []string{"Technical", "Customer", "Technical", "Billing", "Customer", "Security"}
		for i, cat := range categories {
			note := &models.InternalNote{
				TicketID: 123,
				Content:  "Note " + string(rune('A'+i)),
				Category: cat,
			}
			repo.CreateNote(ctx, note)
		}

		// Get unique categories
		cats, err := repo.GetCategories(ctx)
		require.NoError(t, err)
		assert.Equal(t, 4, len(cats)) // Technical, Customer, Billing, Security

		// Verify categories are unique
		catMap := make(map[string]bool)
		for _, cat := range cats {
			assert.False(t, catMap[cat.Name], "Duplicate category found")
			catMap[cat.Name] = true
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()
		done := make(chan bool, 100)

		// Concurrent creates
		for i := 0; i < 20; i++ {
			go func(idx int) {
				note := &models.InternalNote{
					TicketID: 123,
					AuthorID: uint(idx%3 + 1),
					Content:  "Concurrent note " + string(rune('A'+idx)),
				}
				err := repo.CreateNote(ctx, note)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 30; i++ {
			go func() {
				_, err := repo.GetNotesByTicket(ctx, 123)
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all operations
		for i := 0; i < 50; i++ {
			<-done
		}

		// Verify all notes were created
		notes, err := repo.GetNotesByTicket(ctx, 123)
		assert.NoError(t, err)
		assert.Equal(t, 20, len(notes))
	})

	t.Run("FilterNotes_Complex", func(t *testing.T) {
		repo := NewMemoryInternalNoteRepository()

		// Create notes with various attributes
		baseTime := time.Now()
		notes := []models.InternalNote{
			{TicketID: 123, Content: "Important technical note", Category: "Technical", IsImportant: true, Tags: []string{"bug", "database"}},
			{TicketID: 123, Content: "Regular customer note", Category: "Customer", IsImportant: false, Tags: []string{"feedback"}},
			{TicketID: 456, Content: "Another important note", IsImportant: true, Tags: []string{"urgent"}},
			{TicketID: 123, Content: "Pinned resolution note", IsPinned: true, Category: "Resolution", Tags: []string{"solved"}},
		}

		for i := range notes {
			notes[i].CreatedAt = baseTime.Add(time.Duration(i) * time.Hour)
			repo.CreateNote(ctx, &notes[i])
		}

		// Complex filter
		isImportant := true
		filter := &models.NoteFilter{
			TicketID:    123,
			IsImportant: &isImportant,
			Tags:        []string{"bug"},
		}

		results, err := repo.SearchNotes(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Contains(t, results[0].Content, "Important technical")
	})
}
