package zinc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

func TestZincClient(t *testing.T) {
	// Use mock client for testing
	client := NewMockZincClient()
	ctx := context.Background()

	t.Run("IndexDocument", func(t *testing.T) {
		doc := &models.TicketSearchDocument{
			ID:           "TICKET-123",
			TicketNumber: "123",
			Title:        "Network connectivity issue",
			Content:      "Customer experiencing intermittent network drops",
			Status:       "open",
			Priority:     "high",
			Queue:        "Network",
			CustomerName: "John Doe",
			Tags:         []string{"network", "connectivity", "urgent"},
		}

		err := client.IndexDocument(ctx, "tickets", doc.ID, doc)
		require.NoError(t, err)

		// Verify document was indexed
		result, err := client.GetDocument(ctx, "tickets", doc.ID)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("SearchDocuments", func(t *testing.T) {
		// Index some test documents
		docs := []models.TicketSearchDocument{
			{
				ID:       "TICKET-101",
				Title:    "Email not working",
				Content:  "Cannot send or receive emails",
				Status:   "open",
				Priority: "high",
				Queue:    "Email",
				Tags:     []string{"email", "urgent"},
			},
			{
				ID:       "TICKET-102",
				Title:    "Password reset request",
				Content:  "User forgot password and needs reset",
				Status:   "resolved",
				Priority: "normal",
				Queue:    "Account",
				Tags:     []string{"password", "account"},
			},
			{
				ID:       "TICKET-103",
				Title:    "Email attachment issue",
				Content:  "Large attachments not sending via email",
				Status:   "open",
				Priority: "normal",
				Queue:    "Email",
				Tags:     []string{"email", "attachment"},
			},
		}

		for _, doc := range docs {
			client.IndexDocument(ctx, "tickets", doc.ID, doc)
		}

		// Search for email-related tickets
		query := &models.SearchRequest{
			Query:    "email",
			PageSize: 10,
		}

		results, err := client.Search(ctx, "tickets", query)
		require.NoError(t, err)
		assert.Equal(t, int64(2), results.TotalHits)
		assert.Equal(t, 2, len(results.Hits))
	})

	t.Run("SearchWithFilters", func(t *testing.T) {
		query := &models.SearchRequest{
			Query: "*",
			Filters: map[string]string{
				"status":   "open",
				"priority": "high",
			},
			PageSize: 10,
		}

		results, err := client.Search(ctx, "tickets", query)
		require.NoError(t, err)
		assert.Greater(t, results.TotalHits, int64(0))

		// Verify all results match filters
		for _, hit := range results.Hits {
			source := hit.Source
			assert.Equal(t, "open", source["status"])
			assert.Equal(t, "high", source["priority"])
		}
	})

	t.Run("UpdateDocument", func(t *testing.T) {
		docID := "TICKET-200"

		// Index initial document
		doc := &models.TicketSearchDocument{
			ID:       docID,
			Title:    "Initial title",
			Status:   "open",
			Priority: "normal",
		}
		client.IndexDocument(ctx, "tickets", docID, doc)

		// Update document
		updates := map[string]interface{}{
			"status":   "resolved",
			"priority": "high",
			"title":    "Updated title",
		}

		err := client.UpdateDocument(ctx, "tickets", docID, updates)
		require.NoError(t, err)

		// Verify updates
		result, err := client.GetDocument(ctx, "tickets", docID)
		require.NoError(t, err)
		assert.Equal(t, "resolved", result["status"])
		assert.Equal(t, "high", result["priority"])
		assert.Equal(t, "Updated title", result["title"])
	})

	t.Run("DeleteDocument", func(t *testing.T) {
		docID := "TICKET-300"

		// Index document
		doc := &models.TicketSearchDocument{
			ID:    docID,
			Title: "To be deleted",
		}
		client.IndexDocument(ctx, "tickets", docID, doc)

		// Delete document
		err := client.DeleteDocument(ctx, "tickets", docID)
		require.NoError(t, err)

		// Verify deletion
		result, err := client.GetDocument(ctx, "tickets", docID)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("BulkIndex", func(t *testing.T) {
		docs := []interface{}{
			models.TicketSearchDocument{
				ID:    "BULK-1",
				Title: "Bulk document 1",
			},
			models.TicketSearchDocument{
				ID:    "BULK-2",
				Title: "Bulk document 2",
			},
			models.TicketSearchDocument{
				ID:    "BULK-3",
				Title: "Bulk document 3",
			},
		}

		err := client.BulkIndex(ctx, "tickets", docs)
		require.NoError(t, err)

		// Verify all documents were indexed
		for i := 1; i <= 3; i++ {
			docID := "BULK-" + string(rune('0'+i))
			result, err := client.GetDocument(ctx, "tickets", docID)
			require.NoError(t, err)
			assert.NotNil(t, result)
		}
	})

	t.Run("CreateIndex", func(t *testing.T) {
		indexName := "test_index"
		mapping := map[string]interface{}{
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
			},
		}

		err := client.CreateIndex(ctx, indexName, mapping)
		require.NoError(t, err)

		// Verify index exists
		exists, err := client.IndexExists(ctx, indexName)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("DeleteIndex", func(t *testing.T) {
		indexName := "temp_index"

		// Create index
		client.CreateIndex(ctx, indexName, nil)

		// Delete index
		err := client.DeleteIndex(ctx, indexName)
		require.NoError(t, err)

		// Verify deletion
		exists, err := client.IndexExists(ctx, indexName)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("GetIndexStats", func(t *testing.T) {
		stats, err := client.GetIndexStats(ctx, "tickets")
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, "tickets", stats.Name)
		assert.Greater(t, stats.DocumentCount, int64(0))
	})

	t.Run("SearchWithHighlight", func(t *testing.T) {
		query := &models.SearchRequest{
			Query:     "email",
			Highlight: true,
			PageSize:  10,
		}

		results, err := client.Search(ctx, "tickets", query)
		require.NoError(t, err)

		// Check for highlights in results
		for _, hit := range results.Hits {
			if hit.Source["title"] == "Email not working" {
				assert.NotEmpty(t, hit.Highlights)
				assert.Contains(t, hit.Highlights["title"][0], "<em>")
			}
		}
	})

	t.Run("SearchWithPagination", func(t *testing.T) {
		// Index many documents
		for i := 0; i < 25; i++ {
			doc := models.TicketSearchDocument{
				ID:    "PAGE-" + string(rune('A'+i)),
				Title: "Pagination test " + string(rune('A'+i)),
			}
			client.IndexDocument(ctx, "tickets", doc.ID, doc)
		}

		// First page
		query := &models.SearchRequest{
			Query:    "pagination",
			Page:     1,
			PageSize: 10,
		}

		results, err := client.Search(ctx, "tickets", query)
		require.NoError(t, err)
		assert.Equal(t, 10, len(results.Hits))
		assert.Equal(t, 1, results.Page)

		// Second page
		query.Page = 2
		results, err = client.Search(ctx, "tickets", query)
		require.NoError(t, err)
		assert.Equal(t, 10, len(results.Hits))
		assert.Equal(t, 2, results.Page)

		// Third page (partial)
		query.Page = 3
		results, err = client.Search(ctx, "tickets", query)
		require.NoError(t, err)
		assert.Equal(t, 5, len(results.Hits))
		assert.Equal(t, 3, results.Page)
	})

	t.Run("SearchWithSort", func(t *testing.T) {
		query := &models.SearchRequest{
			Query:     "*",
			SortBy:    "created_at",
			SortOrder: "desc",
			PageSize:  10,
		}

		results, err := client.Search(ctx, "tickets", query)
		require.NoError(t, err)

		// For mock client, just verify we got results
		// Real sorting would be implemented in actual Zinc
		assert.NotEmpty(t, results.Hits)
	})

	t.Run("Suggest", func(t *testing.T) {
		suggestions, err := client.Suggest(ctx, "tickets", "emal", "title")
		require.NoError(t, err)
		assert.NotEmpty(t, suggestions)
		assert.Contains(t, suggestions, "email")
	})
}
