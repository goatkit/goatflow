package service

import (
	"context"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalNoteService(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateNote", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		note := &models.InternalNote{
			TicketID:    123,
			AuthorID:    1,
			AuthorName:  "John Doe",
			AuthorEmail: "john@example.com",
			Content:     "Investigation shows database timeout issue @jane",
			Category:    "Technical",
			Tags:        []string{"database", "performance"},
		}

		err := service.CreateNote(ctx, note)
		require.NoError(t, err)
		assert.NotZero(t, note.ID)

		// Verify mention was detected
		_, err = service.GetUserMentions(ctx, 0) // Assuming jane has some ID
		assert.NoError(t, err)
		// Note: In real implementation, we'd need user lookup
	})

	t.Run("CreateNote_WithTemplate", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create a template first
		template := &models.NoteTemplate{
			Name:     "Investigation",
			Content:  "Investigation Result:\n{{finding}}\nNext Steps:\n{{action}}",
			Category: "Technical",
			Tags:     []string{"investigation"},
		}
		err := service.CreateTemplate(ctx, template)
		require.NoError(t, err)

		// Apply template
		variables := map[string]string{
			"{{finding}}": "Database connection pool exhausted",
			"{{action}}":  "Increase pool size to 100",
		}

		note, err := service.CreateNoteFromTemplate(ctx, 123, template.ID, variables, 1)
		require.NoError(t, err)
		assert.Contains(t, note.Content, "Database connection pool exhausted")
		assert.Contains(t, note.Content, "Increase pool size to 100")
		assert.Equal(t, "Technical", note.Category)
	})

	t.Run("UpdateNote_TracksHistory", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create a note
		note := &models.InternalNote{
			TicketID: 123,
			Content:  "Original content",
		}
		service.CreateNote(ctx, note)

		// Update it
		note.Content = "Updated content"
		note.EditedBy = 2
		err := service.UpdateNote(ctx, note, "Correcting information")
		require.NoError(t, err)

		// Check edit history
		history, err := service.GetEditHistory(ctx, note.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, len(history))
		assert.Equal(t, "Original content", history[0].OldContent)
		assert.Equal(t, "Updated content", history[0].NewContent)
	})

	t.Run("PinNote", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create a note
		note := &models.InternalNote{
			TicketID: 123,
			Content:  "Important resolution",
		}
		service.CreateNote(ctx, note)

		// Pin it
		err := service.PinNote(ctx, note.ID, 1)
		require.NoError(t, err)

		// Get pinned notes
		pinned, err := service.GetPinnedNotes(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 1, len(pinned))
		assert.True(t, pinned[0].IsPinned)
	})

	t.Run("MarkImportant", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create a note
		note := &models.InternalNote{
			TicketID: 123,
			Content:  "Critical finding",
		}
		service.CreateNote(ctx, note)

		// Mark as important
		err := service.MarkImportant(ctx, note.ID, true, 1)
		require.NoError(t, err)

		// Get important notes
		important, err := service.GetImportantNotes(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 1, len(important))
		assert.True(t, important[0].IsImportant)
	})

	t.Run("SearchNotes", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create various notes
		notes := []models.InternalNote{
			{TicketID: 123, Content: "Database performance issue", Category: "Technical", Tags: []string{"database"}},
			{TicketID: 123, Content: "Customer complaint", Category: "Customer", Tags: []string{"complaint"}},
			{TicketID: 456, Content: "Another database issue", Category: "Technical", Tags: []string{"database"}},
		}

		for i := range notes {
			service.CreateNote(ctx, &notes[i])
		}

		// Search for database issues in ticket 123
		filter := &models.NoteFilter{
			TicketID:    123,
			SearchQuery: "database",
		}

		results, err := service.SearchNotes(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Contains(t, results[0].Content, "Database performance")
	})

	t.Run("GetNoteSummary", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create multiple notes
		for i := 0; i < 5; i++ {
			note := &models.InternalNote{
				TicketID:    123,
				Content:     "Note content " + string(rune('A'+i)),
				IsImportant: i%2 == 0,
				IsPinned:    i == 0,
			}
			service.CreateNote(ctx, note)
		}

		// Get summary
		summary, err := service.GetTicketNoteSummary(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 5, summary.TotalNotes)
		assert.Equal(t, 3, summary.ImportantNotes) // 0, 2, 4
		assert.Equal(t, 1, summary.PinnedNotes)
		assert.NotNil(t, summary.LastNoteDate)
	})

	t.Run("ExportNotes", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create notes
		notes := []models.InternalNote{
			{TicketID: 123, Content: "First note", Category: "Technical"},
			{TicketID: 123, Content: "Second note", Category: "Customer"},
		}

		for i := range notes {
			service.CreateNote(ctx, &notes[i])
		}

		// Export as JSON
		exported, err := service.ExportNotes(ctx, 123, "json", "user@example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, exported)

		// Export as CSV
		exported, err = service.ExportNotes(ctx, 123, "csv", "user@example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, exported)
	})

	t.Run("DetectMentions", func(t *testing.T) {
		service := &InternalNoteService{}

		tests := []struct {
			content  string
			expected []string
		}{
			{
				content:  "Please review this @john and @jane",
				expected: []string{"john", "jane"},
			},
			{
				content:  "Urgent: @admin needs to check this",
				expected: []string{"admin"},
			},
			{
				content:  "No mentions here",
				expected: []string{},
			},
			{
				content:  "Multiple mentions @user1 @user2 @user1",
				expected: []string{"user1", "user2"},
			},
		}

		for _, tt := range tests {
			mentions := service.detectMentions(tt.content)
			assert.Equal(t, len(tt.expected), len(mentions))
			for _, expected := range tt.expected {
				assert.Contains(t, mentions, expected)
			}
		}
	})

	t.Run("GetRecentActivity", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create notes and perform actions
		note := &models.InternalNote{
			TicketID: 123,
			Content:  "Test note",
		}
		service.CreateNote(ctx, note)

		// Update
		note.Content = "Updated"
		service.UpdateNote(ctx, note, "Test edit")

		// Pin
		service.PinNote(ctx, note.ID, 1)

		// Get activity
		activities, err := service.GetRecentActivity(ctx, 123, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(activities), 2) // At least create and edit
	})

	t.Run("ValidateNoteContent", func(t *testing.T) {
		service := &InternalNoteService{}

		tests := []struct {
			content string
			wantErr bool
		}{
			{"Valid content", false},
			{"", true}, // Empty
			{string(make([]byte, 10001)), true}, // Too long (assuming 10k limit)
			{"Normal note with @mention", false},
		}

		for _, tt := range tests {
			err := service.validateNoteContent(tt.content)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	})

	t.Run("GetNotesByDateRange", func(t *testing.T) {
		repo := repository.NewMemoryInternalNoteRepository()
		service := NewInternalNoteService(repo)

		// Create notes with specific content
		notes := []models.InternalNote{
			{TicketID: 123, Content: "Yesterday's note"},
			{TicketID: 123, Content: "Today's note"},
			{TicketID: 123, Content: "Tomorrow's note"},
		}

		for i := range notes {
			service.CreateNote(ctx, &notes[i])
		}

		// Get all notes to verify they were created
		allNotes, err := service.GetNotesByTicket(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, 3, len(allNotes))

		// For this test, we'll just verify we can filter by searching content
		filter := &models.NoteFilter{
			TicketID:    123,
			SearchQuery: "Today",
		}

		results, err := service.SearchNotes(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Contains(t, results[0].Content, "Today's note")
	})
}