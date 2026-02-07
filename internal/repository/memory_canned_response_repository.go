package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

// MemoryCannedResponseRepository is an in-memory implementation of CannedResponseRepository.
type MemoryCannedResponseRepository struct {
	mu           sync.RWMutex
	responses    map[uint]*models.CannedResponse
	usageHistory map[uint][]models.CannedResponseUsage
	nextID       uint
	nextUsageID  uint
}

// NewMemoryCannedResponseRepository creates a new in-memory canned response repository.
func NewMemoryCannedResponseRepository() *MemoryCannedResponseRepository {
	return &MemoryCannedResponseRepository{
		responses:    make(map[uint]*models.CannedResponse),
		usageHistory: make(map[uint][]models.CannedResponseUsage),
		nextID:       1,
		nextUsageID:  1,
	}
}

// CreateResponse creates a new canned response.
func (r *MemoryCannedResponseRepository) CreateResponse(ctx context.Context, response *models.CannedResponse) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	response.ID = r.nextID
	r.nextID++
	response.CreatedAt = time.Now()
	response.UpdatedAt = response.CreatedAt

	// Create a copy to store
	stored := *response
	r.responses[response.ID] = &stored

	return nil
}

// GetResponseByID retrieves a response by ID.
func (r *MemoryCannedResponseRepository) GetResponseByID(ctx context.Context, id uint) (*models.CannedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	response, exists := r.responses[id]
	if !exists {
		return nil, fmt.Errorf("response with ID %d not found", id)
	}

	// Return a copy
	result := *response
	return &result, nil
}

// GetResponseByShortcut retrieves a response by its shortcut.
func (r *MemoryCannedResponseRepository) GetResponseByShortcut(ctx context.Context, shortcut string) (*models.CannedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, resp := range r.responses {
		if resp.Shortcut == shortcut && resp.IsActive {
			// Return a copy
			result := *resp
			return &result, nil
		}
	}

	return nil, fmt.Errorf("response with shortcut %s not found or inactive", shortcut)
}

// GetActiveResponses retrieves all active responses.
func (r *MemoryCannedResponseRepository) GetActiveResponses(ctx context.Context) ([]models.CannedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var responses []models.CannedResponse
	for _, resp := range r.responses {
		if resp.IsActive {
			responses = append(responses, *resp)
		}
	}

	// Sort by name for consistent ordering
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Name < responses[j].Name
	})

	return responses, nil
}

// GetResponsesByCategory retrieves responses by category.
func (r *MemoryCannedResponseRepository) GetResponsesByCategory(ctx context.Context, category string) ([]models.CannedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var responses []models.CannedResponse
	for _, resp := range r.responses {
		if resp.Category == category && resp.IsActive {
			responses = append(responses, *resp)
		}
	}

	// Sort by name
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Name < responses[j].Name
	})

	return responses, nil
}

// GetResponsesForUser retrieves responses accessible to a specific user.
func (r *MemoryCannedResponseRepository) GetResponsesForUser(ctx context.Context, userID uint) ([]models.CannedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var responses []models.CannedResponse
	for _, resp := range r.responses {
		if !resp.IsActive {
			continue
		}

		// Include if public
		if resp.IsPublic {
			responses = append(responses, *resp)
			continue
		}

		// Include if user is owner
		if resp.OwnerID == userID {
			responses = append(responses, *resp)
			continue
		}

		// Include if shared with user
		for _, sharedID := range resp.SharedWith {
			if sharedID == userID {
				responses = append(responses, *resp)
				break
			}
		}
	}

	// Sort by usage count (most used first), then by name
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].UsageCount != responses[j].UsageCount {
			return responses[i].UsageCount > responses[j].UsageCount
		}
		return responses[i].Name < responses[j].Name
	})

	return responses, nil
}

// UpdateResponse updates an existing response.
func (r *MemoryCannedResponseRepository) UpdateResponse(ctx context.Context, response *models.CannedResponse) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.responses[response.ID]
	if !exists {
		return fmt.Errorf("response with ID %d not found", response.ID)
	}

	// Preserve creation time and usage count
	response.CreatedAt = existing.CreatedAt
	response.UsageCount = existing.UsageCount
	response.UpdatedAt = time.Now()

	// Update with a copy
	updated := *response
	r.responses[response.ID] = &updated

	return nil
}

// DeleteResponse deletes a response.
func (r *MemoryCannedResponseRepository) DeleteResponse(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.responses[id]; !exists {
		return fmt.Errorf("response with ID %d not found", id)
	}

	delete(r.responses, id)
	// Also delete usage history
	delete(r.usageHistory, id)
	return nil
}

// IncrementUsageCount increments the usage count of a response.
func (r *MemoryCannedResponseRepository) IncrementUsageCount(ctx context.Context, responseID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	response, exists := r.responses[responseID]
	if !exists {
		return fmt.Errorf("response with ID %d not found", responseID)
	}

	response.UsageCount++
	response.UpdatedAt = time.Now()

	return nil
}

// RecordUsage records when a response was used.
func (r *MemoryCannedResponseRepository) RecordUsage(ctx context.Context, usage *models.CannedResponseUsage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Verify response exists
	if _, exists := r.responses[usage.ResponseID]; !exists {
		return fmt.Errorf("response with ID %d not found", usage.ResponseID)
	}

	usage.ID = r.nextUsageID
	r.nextUsageID++
	usage.UsedAt = time.Now()

	// Create a copy to store
	stored := *usage
	r.usageHistory[usage.ResponseID] = append(r.usageHistory[usage.ResponseID], stored)

	return nil
}

// GetUsageHistory retrieves usage history for a response.
func (r *MemoryCannedResponseRepository) GetUsageHistory(ctx context.Context, responseID uint, limit int) ([]models.CannedResponseUsage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	history, exists := r.usageHistory[responseID]
	if !exists {
		return []models.CannedResponseUsage{}, nil
	}

	// Sort by most recent first
	result := make([]models.CannedResponseUsage, len(history))
	copy(result, history)
	sort.Slice(result, func(i, j int) bool {
		return result[i].UsedAt.After(result[j].UsedAt)
	})

	// Apply limit
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result, nil
}

// SearchResponses searches responses based on filter criteria.
func (r *MemoryCannedResponseRepository) SearchResponses(ctx context.Context, filter *models.CannedResponseFilter) ([]models.CannedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]models.CannedResponse, 0, len(r.responses))
	query := strings.ToLower(filter.Query)

	for _, resp := range r.responses {
		if !resp.IsActive {
			continue
		}

		// Apply filters
		if filter.OnlyPublic && !resp.IsPublic {
			continue
		}

		if filter.OnlyOwned && resp.OwnerID != filter.UserID {
			continue
		}

		if filter.Category != "" && resp.Category != filter.Category {
			continue
		}

		if filter.QueueID != 0 {
			found := false
			for _, qid := range resp.QueueIDs {
				if qid == filter.QueueID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check tags filter
		if len(filter.Tags) > 0 {
			hasTag := false
			for _, filterTag := range filter.Tags {
				for _, respTag := range resp.Tags {
					if respTag == filterTag {
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
			if !strings.Contains(strings.ToLower(resp.Name), query) &&
				!strings.Contains(strings.ToLower(resp.Content), query) &&
				!strings.Contains(strings.ToLower(resp.Subject), query) &&
				!strings.Contains(strings.ToLower(resp.Shortcut), query) {
				// Check in tags
				tagMatch := false
				for _, tag := range resp.Tags {
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

		results = append(results, *resp)
	}

	// Sort by usage count (most used first), then by name
	sort.Slice(results, func(i, j int) bool {
		if results[i].UsageCount != results[j].UsageCount {
			return results[i].UsageCount > results[j].UsageCount
		}
		return results[i].Name < results[j].Name
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

// GetMostUsedResponses returns the most frequently used responses.
func (r *MemoryCannedResponseRepository) GetMostUsedResponses(ctx context.Context, limit int) ([]models.CannedResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var responses []models.CannedResponse
	for _, resp := range r.responses {
		if resp.IsActive {
			responses = append(responses, *resp)
		}
	}

	// Sort by usage count descending
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].UsageCount != responses[j].UsageCount {
			return responses[i].UsageCount > responses[j].UsageCount
		}
		return responses[i].Name < responses[j].Name
	})

	// Apply limit
	if limit > 0 && limit < len(responses) {
		responses = responses[:limit]
	}

	return responses, nil
}

// GetCategories retrieves all unique categories.
func (r *MemoryCannedResponseRepository) GetCategories(ctx context.Context) ([]models.CannedResponseCategory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect unique categories
	categoryMap := make(map[string]int) // category name -> count
	for _, resp := range r.responses {
		if resp.IsActive && resp.Category != "" {
			categoryMap[resp.Category]++
		}
	}

	// Convert to slice of CannedResponseCategory
	categories := make([]models.CannedResponseCategory, 0, len(categoryMap))
	order := 1
	for name, count := range categoryMap {
		categories = append(categories, models.CannedResponseCategory{
			ID:          uint(order),
			Name:        name,
			Description: fmt.Sprintf("%d responses", count),
			Order:       order,
			Active:      true,
		})
		order++
	}

	// Sort by name for consistent ordering
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
