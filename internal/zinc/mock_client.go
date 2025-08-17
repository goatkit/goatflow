package zinc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// MockZincClient is a mock implementation of the Client interface for testing
type MockZincClient struct {
	mu        sync.RWMutex
	indices   map[string]map[string]interface{} // index -> document ID -> document
	indexInfo map[string]*models.IndexStats
}

// NewMockZincClient creates a new mock Zinc client
func NewMockZincClient() *MockZincClient {
	return &MockZincClient{
		indices:   make(map[string]map[string]interface{}),
		indexInfo: make(map[string]*models.IndexStats),
	}
}

// CreateIndex creates a new index
func (c *MockZincClient) CreateIndex(ctx context.Context, name string, mapping map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indices[name]; exists {
		return fmt.Errorf("index %s already exists", name)
	}

	c.indices[name] = make(map[string]interface{})
	c.indexInfo[name] = &models.IndexStats{
		Name:          name,
		DocumentCount: 0,
		StorageSize:   0,
		LastUpdated:   time.Now(),
		Status:        "green",
	}

	return nil
}

// DeleteIndex deletes an index
func (c *MockZincClient) DeleteIndex(ctx context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indices[name]; !exists {
		return fmt.Errorf("index %s not found", name)
	}

	delete(c.indices, name)
	delete(c.indexInfo, name)
	return nil
}

// IndexExists checks if an index exists
func (c *MockZincClient) IndexExists(ctx context.Context, name string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.indices[name]
	return exists, nil
}

// GetIndexStats retrieves statistics for an index
func (c *MockZincClient) GetIndexStats(ctx context.Context, name string) (*models.IndexStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats, exists := c.indexInfo[name]
	if !exists {
		return nil, fmt.Errorf("index %s not found", name)
	}

	// Update document count
	if docs, ok := c.indices[name]; ok {
		stats.DocumentCount = int64(len(docs))
		stats.StorageSize = int64(len(docs) * 1024) // Approximate
	}

	return stats, nil
}

// IndexDocument indexes a single document
func (c *MockZincClient) IndexDocument(ctx context.Context, index string, id string, doc interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indices[index]; !exists {
		c.indices[index] = make(map[string]interface{})
		c.indexInfo[index] = &models.IndexStats{
			Name:   index,
			Status: "green",
		}
	}

	c.indices[index][id] = doc
	c.indexInfo[index].DocumentCount++
	c.indexInfo[index].LastUpdated = time.Now()

	return nil
}

// UpdateDocument updates a document
func (c *MockZincClient) UpdateDocument(ctx context.Context, index string, id string, updates map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexDocs, exists := c.indices[index]
	if !exists {
		return fmt.Errorf("index %s not found", index)
	}

	doc, exists := indexDocs[id]
	if !exists {
		return fmt.Errorf("document %s not found", id)
	}

	// Apply updates
	if docMap, ok := doc.(map[string]interface{}); ok {
		for k, v := range updates {
			docMap[k] = v
		}
	} else if searchDoc, ok := doc.(*models.TicketSearchDocument); ok {
		// Handle TicketSearchDocument updates
		for k, v := range updates {
			switch k {
			case "status":
				if s, ok := v.(string); ok {
					searchDoc.Status = s
				}
			case "priority":
				if p, ok := v.(string); ok {
					searchDoc.Priority = p
				}
			case "title":
				if t, ok := v.(string); ok {
					searchDoc.Title = t
				}
			}
		}
	}

	c.indexInfo[index].LastUpdated = time.Now()
	return nil
}

// DeleteDocument deletes a document
func (c *MockZincClient) DeleteDocument(ctx context.Context, index string, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	indexDocs, exists := c.indices[index]
	if !exists {
		return fmt.Errorf("index %s not found", index)
	}

	if _, exists := indexDocs[id]; !exists {
		return fmt.Errorf("document %s not found", id)
	}

	delete(indexDocs, id)
	c.indexInfo[index].DocumentCount--
	c.indexInfo[index].LastUpdated = time.Now()

	return nil
}

// GetDocument retrieves a document
func (c *MockZincClient) GetDocument(ctx context.Context, index string, id string) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	indexDocs, exists := c.indices[index]
	if !exists {
		return nil, fmt.Errorf("index %s not found", index)
	}

	doc, exists := indexDocs[id]
	if !exists {
		return nil, fmt.Errorf("document %s not found", id)
	}

	// Convert to map
	result := make(map[string]interface{})
	
	switch v := doc.(type) {
	case map[string]interface{}:
		result = v
	case models.TicketSearchDocument:
		result["id"] = v.ID
		result["title"] = v.Title
		result["content"] = v.Content
		result["status"] = v.Status
		result["priority"] = v.Priority
		result["queue"] = v.Queue
		result["tags"] = v.Tags
	case *models.TicketSearchDocument:
		result["id"] = v.ID
		result["title"] = v.Title
		result["content"] = v.Content
		result["status"] = v.Status
		result["priority"] = v.Priority
		result["queue"] = v.Queue
		result["tags"] = v.Tags
	}

	return result, nil
}

// BulkIndex indexes multiple documents
func (c *MockZincClient) BulkIndex(ctx context.Context, index string, docs []interface{}) error {
	for _, doc := range docs {
		// Extract ID from document
		var id string
		switch v := doc.(type) {
		case models.TicketSearchDocument:
			id = v.ID
		case *models.TicketSearchDocument:
			id = v.ID
		case map[string]interface{}:
			if idVal, ok := v["id"]; ok {
				id = fmt.Sprintf("%v", idVal)
			}
		}

		if id == "" {
			id = fmt.Sprintf("doc_%d", time.Now().UnixNano())
		}

		if err := c.IndexDocument(ctx, index, id, doc); err != nil {
			return err
		}
	}

	return nil
}

// Search performs a search query
func (c *MockZincClient) Search(ctx context.Context, index string, query *models.SearchRequest) (*models.SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	indexDocs, exists := c.indices[index]
	if !exists {
		return nil, fmt.Errorf("index %s not found", index)
	}

	var hits []models.SearchHit
	queryLower := strings.ToLower(query.Query)

	// Simple text search
	for id, doc := range indexDocs {
		match := false
		score := 0.0

		// Convert document to searchable format
		var title, content, status, priority string
		var tags []string

		switch v := doc.(type) {
		case models.TicketSearchDocument:
			title = v.Title
			content = v.Content
			status = v.Status
			priority = v.Priority
			tags = v.Tags
		case *models.TicketSearchDocument:
			title = v.Title
			content = v.Content
			status = v.Status
			priority = v.Priority
			tags = v.Tags
		case map[string]interface{}:
			if t, ok := v["title"].(string); ok {
				title = t
			}
			if c, ok := v["content"].(string); ok {
				content = c
			}
			if s, ok := v["status"].(string); ok {
				status = s
			}
			if p, ok := v["priority"].(string); ok {
				priority = p
			}
		}

		// Check query match
		if query.Query == "*" || query.Query == "" {
			match = true
			score = 1.0
		} else if strings.Contains(strings.ToLower(title), queryLower) {
			match = true
			score = 2.0
		} else if strings.Contains(strings.ToLower(content), queryLower) {
			match = true
			score = 1.0
		} else {
			for _, tag := range tags {
				if strings.Contains(strings.ToLower(tag), queryLower) {
					match = true
					score = 0.5
					break
				}
			}
		}

		// Apply filters
		if match && len(query.Filters) > 0 {
			for field, value := range query.Filters {
				switch field {
				case "status":
					if status != value {
						match = false
					}
				case "priority":
					if priority != value {
						match = false
					}
				}
			}
		}

		if match {
			source := make(map[string]interface{})
			source["id"] = id
			source["title"] = title
			source["content"] = content
			source["status"] = status
			source["priority"] = priority
			source["tags"] = tags

			hit := models.SearchHit{
				ID:        id,
				Type:      "ticket",
				Score:     score,
				Source:    source,
				Timestamp: time.Now(),
			}

			// Add highlights if requested
			if query.Highlight && queryLower != "" && queryLower != "*" {
				hit.Highlights = make(map[string][]string)
				if strings.Contains(strings.ToLower(title), queryLower) {
					// Case-insensitive replacement
					highlighted := strings.ReplaceAll(strings.ToLower(title), queryLower, "<em>"+queryLower+"</em>")
					hit.Highlights["title"] = []string{highlighted}
				}
				if strings.Contains(strings.ToLower(content), queryLower) {
					highlighted := strings.ReplaceAll(strings.ToLower(content), queryLower, "<em>"+queryLower+"</em>")
					hit.Highlights["content"] = []string{highlighted}
				}
			}

			hits = append(hits, hit)
		}
	}

	// Apply pagination
	page := query.Page
	if page == 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize == 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	totalHits := len(hits)
	if start < len(hits) {
		if end > len(hits) {
			end = len(hits)
		}
		hits = hits[start:end]
	} else {
		hits = []models.SearchHit{}
	}

	result := &models.SearchResult{
		Query:      query.Query,
		TotalHits:  int64(totalHits),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (totalHits + pageSize - 1) / pageSize,
		Took:       10, // Mock timing
		Hits:       hits,
	}

	return result, nil
}

// Suggest provides search suggestions
func (c *MockZincClient) Suggest(ctx context.Context, index string, text string, field string) ([]string, error) {
	// Simple mock implementation
	suggestions := []string{}
	textLower := strings.ToLower(text)
	
	// Common terms that might be suggested
	terms := []string{"email", "password", "network", "error", "login", "ticket"}
	
	for _, term := range terms {
		if strings.HasPrefix(term, textLower) {
			suggestions = append(suggestions, term)
		}
	}
	
	// Always suggest "email" for "emal" (typo correction)
	if textLower == "emal" {
		suggestions = append(suggestions, "email")
	}
	
	return suggestions, nil
}