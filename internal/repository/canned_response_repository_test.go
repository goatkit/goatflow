package repository

import (
	"context"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCannedResponseRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateResponse", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		response := &models.CannedResponse{
			Name:        "Welcome Message",
			Shortcut:    "/welcome",
			Category:    "Greetings",
			Subject:     "Welcome to our support",
			Content:     "Hello {{customer_name}},\n\nWelcome to our support system. We're here to help!",
			ContentType: "text/plain",
			Tags:        []string{"greeting", "welcome", "new"},
			IsPublic:    true,
			IsActive:    true,
			Variables: []models.ResponseVariable{
				{
					Name:        "{{customer_name}}",
					Description: "Customer's name",
					Type:        "text",
					AutoFill:    "customer_name",
				},
			},
		}

		err := repo.CreateResponse(ctx, response)
		require.NoError(t, err)
		assert.NotZero(t, response.ID)
		assert.NotZero(t, response.CreatedAt)
	})

	t.Run("GetResponseByID", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create a response
		response := &models.CannedResponse{
			Name:     "Test Response",
			Category: "Test",
			Content:  "Test content",
			IsActive: true,
		}
		repo.CreateResponse(ctx, response)

		// Retrieve it
		retrieved, err := repo.GetResponseByID(ctx, response.ID)
		require.NoError(t, err)
		assert.Equal(t, response.Name, retrieved.Name)
		assert.Equal(t, response.Content, retrieved.Content)
	})

	t.Run("GetResponseByID_NotFound", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		_, err := repo.GetResponseByID(ctx, 9999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetResponseByShortcut", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create responses with shortcuts
		responses := []models.CannedResponse{
			{Name: "Greeting", Shortcut: "/hello", Category: "General", Content: "Hello!", IsActive: true},
			{Name: "Goodbye", Shortcut: "/bye", Category: "General", Content: "Goodbye!", IsActive: true},
			{Name: "Thanks", Shortcut: "/thanks", Category: "General", Content: "Thank you!", IsActive: false},
		}

		for i := range responses {
			repo.CreateResponse(ctx, &responses[i])
		}

		// Test finding by shortcut
		response, err := repo.GetResponseByShortcut(ctx, "/hello")
		require.NoError(t, err)
		assert.Equal(t, "Greeting", response.Name)

		// Test with inactive response
		_, err = repo.GetResponseByShortcut(ctx, "/thanks")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Test with non-existent shortcut
		_, err = repo.GetResponseByShortcut(ctx, "/nonexistent")
		assert.Error(t, err)
	})

	t.Run("GetActiveResponses", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create multiple responses
		for i := 0; i < 5; i++ {
			response := &models.CannedResponse{
				Name:     "Response " + string(rune('A'+i)),
				Category: "Test",
				Content:  "Content",
				IsActive: i%2 == 0, // Only even ones are active
			}
			repo.CreateResponse(ctx, response)
		}

		// Get active responses
		responses, err := repo.GetActiveResponses(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, len(responses))

		// Verify all returned responses are active
		for _, resp := range responses {
			assert.True(t, resp.IsActive)
		}
	})

	t.Run("GetResponsesByCategory", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create responses in different categories
		categories := []string{"Technical", "Billing", "Technical", "General"}
		for i, cat := range categories {
			response := &models.CannedResponse{
				Name:     "Response " + string(rune('A'+i)),
				Category: cat,
				Content:  "Content",
				IsActive: true,
			}
			repo.CreateResponse(ctx, response)
		}

		// Get responses by category
		techResponses, err := repo.GetResponsesByCategory(ctx, "Technical")
		require.NoError(t, err)
		assert.Equal(t, 2, len(techResponses))

		billingResponses, err := repo.GetResponsesByCategory(ctx, "Billing")
		require.NoError(t, err)
		assert.Equal(t, 1, len(billingResponses))
	})

	t.Run("GetResponsesForUser", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create responses with different access controls
		responses := []models.CannedResponse{
			{Name: "Public", IsPublic: true, IsActive: true, Category: "Test", Content: "Public content"},
			{Name: "Private Owner", OwnerID: 1, IsPublic: false, IsActive: true, Category: "Test", Content: "Private content"},
			{Name: "Shared", OwnerID: 2, SharedWith: []uint{1, 3}, IsPublic: false, IsActive: true, Category: "Test", Content: "Shared content"},
			{Name: "Other Private", OwnerID: 2, IsPublic: false, IsActive: true, Category: "Test", Content: "Other private"},
		}

		for i := range responses {
			repo.CreateResponse(ctx, &responses[i])
		}

		// Get responses for user 1
		userResponses, err := repo.GetResponsesForUser(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, 3, len(userResponses)) // Public + Private Owner + Shared

		// Verify correct responses returned
		names := make(map[string]bool)
		for _, resp := range userResponses {
			names[resp.Name] = true
		}
		assert.True(t, names["Public"])
		assert.True(t, names["Private Owner"])
		assert.True(t, names["Shared"])
		assert.False(t, names["Other Private"])
	})

	t.Run("UpdateResponse", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create a response
		response := &models.CannedResponse{
			Name:     "Original Name",
			Category: "Test",
			Content:  "Original Content",
			IsActive: true,
		}
		repo.CreateResponse(ctx, response)
		originalID := response.ID

		// Update it
		response.Name = "Updated Name"
		response.Content = "Updated Content"
		response.IsActive = false

		err := repo.UpdateResponse(ctx, response)
		require.NoError(t, err)

		// Verify update
		updated, err := repo.GetResponseByID(ctx, originalID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "Updated Content", updated.Content)
		assert.False(t, updated.IsActive)
		assert.True(t, updated.UpdatedAt.After(updated.CreatedAt))
	})

	t.Run("DeleteResponse", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create a response
		response := &models.CannedResponse{
			Name:     "To Delete",
			Category: "Test",
			Content:  "Content",
			IsActive: true,
		}
		repo.CreateResponse(ctx, response)
		id := response.ID

		// Delete it
		err := repo.DeleteResponse(ctx, id)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetResponseByID(ctx, id)
		assert.Error(t, err)
	})

	t.Run("IncrementUsageCount", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create a response
		response := &models.CannedResponse{
			Name:       "Popular Response",
			Category:   "Test",
			Content:    "Content",
			IsActive:   true,
			UsageCount: 0,
		}
		repo.CreateResponse(ctx, response)

		// Increment usage count multiple times
		for i := 0; i < 5; i++ {
			err := repo.IncrementUsageCount(ctx, response.ID)
			require.NoError(t, err)
		}

		// Verify count
		updated, err := repo.GetResponseByID(ctx, response.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, updated.UsageCount)
	})

	t.Run("RecordUsage", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create a response
		response := &models.CannedResponse{
			Name:     "Test Response",
			Category: "Test",
			Content:  "Content",
			IsActive: true,
		}
		repo.CreateResponse(ctx, response)

		// Record usage
		usage := &models.CannedResponseUsage{
			ResponseID:        response.ID,
			TicketID:          123,
			UserID:            456,
			ModifiedBefore:    false,
		}

		err := repo.RecordUsage(ctx, usage)
		require.NoError(t, err)
		assert.NotZero(t, usage.ID)
		assert.NotZero(t, usage.UsedAt)

		// Get usage history
		history, err := repo.GetUsageHistory(ctx, response.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, 1, len(history))
		assert.Equal(t, uint(123), history[0].TicketID)
	})

	t.Run("SearchResponses", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create responses with different content
		responses := []struct {
			name string
			content string
			tags []string
		}{
			{"Password Reset", "Reset your password instructions", []string{"password", "security"}},
			{"Network Issue", "Network connectivity troubleshooting", []string{"network", "technical"}},
			{"Billing Question", "Billing and payment information", []string{"billing", "payment"}},
			{"Password Change", "Change your password guide", []string{"password", "account"}},
		}

		for _, resp := range responses {
			response := &models.CannedResponse{
				Name:     resp.name,
				Category: "Test",
				Content:  resp.content,
				Tags:     resp.tags,
				IsActive: true,
			}
			repo.CreateResponse(ctx, response)
		}

		// Search with filter
		filter := &models.CannedResponseFilter{
			Query: "password",
			Limit: 10,
		}

		results, err := repo.SearchResponses(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 2, len(results))

		// Search by tag
		filter = &models.CannedResponseFilter{
			Tags:  []string{"billing"},
			Limit: 10,
		}

		results, err = repo.SearchResponses(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("GetCategories", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create responses in various categories
		categories := []string{"Technical", "Billing", "Technical", "General", "Billing", "Support"}
		for i, cat := range categories {
			response := &models.CannedResponse{
				Name:     "Response " + string(rune('A'+i)),
				Category: cat,
				Content:  "Content",
				IsActive: true,
			}
			repo.CreateResponse(ctx, response)
		}

		// Get unique categories
		cats, err := repo.GetCategories(ctx)
		require.NoError(t, err)
		assert.Equal(t, 4, len(cats)) // Technical, Billing, General, Support

		// Verify categories are unique
		catMap := make(map[string]bool)
		for _, cat := range cats {
			assert.False(t, catMap[cat.Name], "Duplicate category found")
			catMap[cat.Name] = true
		}
	})

	t.Run("GetMostUsedResponses", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()

		// Create responses with different usage counts
		responses := []struct {
			name  string
			count int
		}{
			{"Low Usage", 2},
			{"High Usage", 10},
			{"Medium Usage", 5},
			{"No Usage", 0},
		}

		for _, resp := range responses {
			response := &models.CannedResponse{
				Name:       resp.name,
				Category:   "Test",
				Content:    "Content",
				IsActive:   true,
				UsageCount: resp.count,
			}
			repo.CreateResponse(ctx, response)
		}

		// Get most used responses
		mostUsed, err := repo.GetMostUsedResponses(ctx, 3)
		require.NoError(t, err)
		assert.Equal(t, 3, len(mostUsed))

		// Verify ordering
		assert.Equal(t, "High Usage", mostUsed[0].Name)
		assert.Equal(t, "Medium Usage", mostUsed[1].Name)
		assert.Equal(t, "Low Usage", mostUsed[2].Name)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		repo := NewMemoryCannedResponseRepository()
		done := make(chan bool, 100)

		// Concurrent creates
		for i := 0; i < 20; i++ {
			go func(idx int) {
				response := &models.CannedResponse{
					Name:     "Concurrent " + string(rune('A'+idx)),
					Category: "Test",
					Content:  "Content",
					IsActive: true,
				}
				err := repo.CreateResponse(ctx, response)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 30; i++ {
			go func() {
				_, err := repo.GetActiveResponses(ctx)
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all operations
		for i := 0; i < 50; i++ {
			<-done
		}

		// Verify all responses were created
		responses, err := repo.GetActiveResponses(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(responses), 20)
	})
}