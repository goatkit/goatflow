package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/zinc"
)

func TestSearchService(t *testing.T) {
	ctx := context.Background()

	t.Run("IndexTicket", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		ticket := &models.Ticket{
			ID:               123,
			TicketNumber:     "TICKET-123",
			Title:            "Email delivery issues",
			QueueID:          1,
			TicketStateID:    2,
			TicketPriorityID: 3,
		}

		err := service.IndexTicket(ctx, ticket)
		require.NoError(t, err)

		// Verify ticket was indexed
		doc, err := client.GetDocument(ctx, "tickets", "TICKET-123")
		require.NoError(t, err)
		assert.Equal(t, "TICKET-123", doc["ticket_number"])
		assert.Equal(t, "Email delivery issues", doc["title"])
	})

	t.Run("SearchTickets", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Index test tickets
		tickets := []models.Ticket{
			{
				ID:               101,
				TicketNumber:     "TICKET-101",
				Title:            "Cannot send emails",
				TicketStateID:    1, // new
				TicketPriorityID: 3, // high
			},
			{
				ID:               102,
				TicketNumber:     "TICKET-102",
				Title:            "Password reset needed",
				TicketStateID:    4, // closed
				TicketPriorityID: 2, // normal
			},
			{
				ID:               103,
				TicketNumber:     "TICKET-103",
				Title:            "Email attachments broken",
				TicketStateID:    1, // new
				TicketPriorityID: 2, // normal
			},
		}

		for _, ticket := range tickets {
			service.IndexTicket(ctx, &ticket)
		}

		// Search for email-related tickets
		request := &models.SearchRequest{
			Query:    "email",
			PageSize: 10,
		}

		results, err := service.SearchTickets(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, int64(2), results.TotalHits)
		assert.Equal(t, 2, len(results.Hits))
	})

	t.Run("SearchWithFilters", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Index test data
		tickets := []models.Ticket{
			{ID: 201, TicketNumber: "T-201", Title: "Urgent issue", TicketStateID: 1, TicketPriorityID: 4},
			{ID: 202, TicketNumber: "T-202", Title: "Normal request", TicketStateID: 1, TicketPriorityID: 2},
			{ID: 203, TicketNumber: "T-203", Title: "Urgent problem", TicketStateID: 4, TicketPriorityID: 4},
		}

		for _, ticket := range tickets {
			service.IndexTicket(ctx, &ticket)
		}

		// Search for urgent open tickets
		filter := &models.SearchFilter{
			Query:      "urgent",
			Statuses:   []string{"new", "open"},
			Priorities: []string{"high"},
		}

		results, err := service.SearchWithFilter(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, int64(1), results.TotalHits) // Only T-201 matches
	})

	t.Run("UpdateTicketInIndex", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Index initial ticket
		ticket := &models.Ticket{
			ID:            301,
			TicketNumber:  "TICKET-301",
			Title:         "Initial title",
			TicketStateID: 1,
		}
		service.IndexTicket(ctx, ticket)

		// Update ticket
		ticket.Title = "Updated title"
		ticket.TicketStateID = 4 // closed

		err := service.UpdateTicketInIndex(ctx, ticket)
		require.NoError(t, err)

		// Verify update
		doc, err := client.GetDocument(ctx, "tickets", "TICKET-301")
		require.NoError(t, err)
		assert.Equal(t, "Updated title", doc["title"])
		assert.Equal(t, "closed", doc["status"])
	})

	t.Run("DeleteTicketFromIndex", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Index ticket
		ticket := &models.Ticket{
			ID:           401,
			TicketNumber: "TICKET-401",
			Title:        "To be deleted",
		}
		service.IndexTicket(ctx, ticket)

		// Delete from index
		err := service.DeleteTicketFromIndex(ctx, "TICKET-401")
		require.NoError(t, err)

		// Verify deletion
		_, err = client.GetDocument(ctx, "tickets", "TICKET-401")
		assert.Error(t, err)
	})

	t.Run("BulkIndexTickets", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		tickets := []models.Ticket{
			{ID: 501, TicketNumber: "BULK-501", Title: "Bulk ticket 1"},
			{ID: 502, TicketNumber: "BULK-502", Title: "Bulk ticket 2"},
			{ID: 503, TicketNumber: "BULK-503", Title: "Bulk ticket 3"},
		}

		err := service.BulkIndexTickets(ctx, tickets)
		require.NoError(t, err)

		// Verify all were indexed
		for _, ticket := range tickets {
			doc, err := client.GetDocument(ctx, "tickets", ticket.TicketNumber)
			require.NoError(t, err)
			assert.Equal(t, ticket.Title, doc["title"])
		}
	})

	t.Run("ReindexAllTickets", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Mock ticket fetcher
		tickets := []models.Ticket{
			{ID: 601, TicketNumber: "ALL-601", Title: "Ticket 1"},
			{ID: 602, TicketNumber: "ALL-602", Title: "Ticket 2"},
		}

		fetcher := func() ([]models.Ticket, error) {
			return tickets, nil
		}

		stats, err := service.ReindexAllTickets(ctx, fetcher)
		require.NoError(t, err)
		assert.Equal(t, 2, stats.Indexed)
		assert.Equal(t, 0, stats.Failed)
		assert.Equal(t, 2, stats.Total)
	})

	t.Run("SaveSearch", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		savedSearch := &models.SavedSearch{
			UserID:      1,
			Name:        "My urgent tickets",
			Description: "All urgent tickets assigned to me",
			Filter: models.SearchFilter{
				Query:      "urgent",
				Priorities: []string{"high", "very high"},
			},
			IsPublic: false,
		}

		err := service.SaveSearch(ctx, savedSearch)
		require.NoError(t, err)
		assert.NotZero(t, savedSearch.ID)

		// Retrieve saved search
		retrieved, err := service.GetSavedSearch(ctx, savedSearch.ID)
		require.NoError(t, err)
		assert.Equal(t, "My urgent tickets", retrieved.Name)
	})

	t.Run("GetSearchSuggestions", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		suggestions, err := service.GetSearchSuggestions(ctx, "emai")
		require.NoError(t, err)
		assert.Contains(t, suggestions, "email")
	})

	t.Run("RecordSearchHistory", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Perform a search
		request := &models.SearchRequest{
			Query:    "network issue",
			PageSize: 10,
		}

		results, err := service.SearchTickets(ctx, request)
		require.NoError(t, err)

		// Record in history
		err = service.RecordSearchHistory(ctx, 1, request, results)
		require.NoError(t, err)

		// Get history
		history, err := service.GetSearchHistory(ctx, 1, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, history)
		assert.Equal(t, "network issue", history[0].Query)
	})

	t.Run("GetSearchAnalytics", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Perform several searches and record them in history
		searches := []string{"email", "password", "email", "network", "email"}
		for _, query := range searches {
			req := &models.SearchRequest{Query: query, PageSize: 10}
			results, _ := service.SearchTickets(ctx, req)
			// Record each search in history
			service.RecordSearchHistory(ctx, 1, req, results)
		}

		analytics, err := service.GetSearchAnalytics(ctx, time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))
		require.NoError(t, err)
		assert.Greater(t, analytics.TotalSearches, int64(0))
		assert.NotEmpty(t, analytics.TopQueries)
	})

	t.Run("SearchWithHighlighting", func(t *testing.T) {
		client := zinc.NewMockZincClient()
		service := NewSearchService(client)

		// Index ticket
		ticket := &models.Ticket{
			ID:           701,
			TicketNumber: "TICKET-701",
			Title:        "Email server not responding",
		}
		service.IndexTicket(ctx, ticket)

		// Search with highlighting
		request := &models.SearchRequest{
			Query:     "email",
			Highlight: true,
			PageSize:  10,
		}

		results, err := service.SearchTickets(ctx, request)
		require.NoError(t, err)
		assert.NotEmpty(t, results.Hits)

		// Check for highlights
		if len(results.Hits) > 0 {
			assert.NotNil(t, results.Hits[0].Highlights)
		}
	})

	t.Run("MapTicketToSearchDocument", func(t *testing.T) {
		service := &SearchService{}

		ticket := &models.Ticket{
			ID:               123,
			TicketNumber:     "TN-2025-123",
			Title:            "Test ticket",
			QueueID:          1,
			TicketStateID:    2,
			TicketPriorityID: 3,
			CustomerID:       stringPtr("CUST-001"),
			UserID:           intPtr(10),
			CreateTime:       time.Now().Add(-24 * time.Hour),
			ChangeTime:       time.Now(),
		}

		doc := service.mapTicketToSearchDocument(ticket)

		assert.Equal(t, "TN-2025-123", doc.ID)
		assert.Equal(t, "TN-2025-123", doc.TicketNumber)
		assert.Equal(t, "Test ticket", doc.Title)
		assert.NotEmpty(t, doc.Status)
		assert.NotEmpty(t, doc.Priority)
		assert.NotEmpty(t, doc.CreatedAt)
		assert.NotEmpty(t, doc.UpdatedAt)
	})
}

// Helper functions for tests.
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
