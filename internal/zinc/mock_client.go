package zinc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"time"
)

// MockZincClient is a mock implementation of the Client interface for testing.
type MockZincClient struct {
	mu        sync.RWMutex
	indices   map[string]map[string]interface{} // index -> document ID -> document
	indexInfo map[string]*platformmodels.IndexStats
}

// NewMockZincClient creates a new mock Zinc client.
func NewMockZincClient() *MockZincClient {
	return &MockZincClient{
		indices:   make(map[string]map[string]interface{}),
		indexInfo: make(map[string]*platformmodels.IndexStats),
	}
}

// CreateIndex creates a new index.
func (c *MockZincClient) CreateIndex(ctx context.Context, name string, mapping map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indices[name]; exists {
		return fmt.Errorf("index %s already exists", name)
	}

	c.indices[name] = make(map[string]interface{})
	c.indexInfo[name] = &platformmodels.IndexStats{
		Name:          name,
		DocumentCount: 0,
		StorageSize:   0,
		LastUpdated:   time.Now(),
		Status:        "green",
	}

	return nil
}

// DeleteIndex deletes an index.
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

// IndexExists checks if an index exists.
func (c *MockZincClient) IndexExists(ctx context.Context, name string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.indices[name]
	return exists, nil
}

// GetIndexStats retrieves statistics for an index.
func (c *MockZincClient) GetIndexStats(ctx context.Context, name string) (*platformmodels.IndexStats, error) {
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

// IndexDocument indexes a single document.
func (c *MockZincClient) IndexDocument(ctx context.Context, index string, id string, doc interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indices[index]; !exists {
		c.indices[index] = make(map[string]interface{})
		c.indexInfo[index] = &platformmodels.IndexStats{
			Name:   index,
			Status: "green",
		}
	}

	c.indices[index][id] = doc
	c.indexInfo[index].DocumentCount++
	c.indexInfo[index].LastUpdated = time.Now()

	return nil
}

// UpdateDocument updates a document.
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

	// Apply updates via JSON roundtrip — supports any serializable type
	docMap, err := docToMap(doc)
	if err == nil && docMap != nil {
		for k, v := range updates {
			docMap[k] = v
		}
		c.indices[index][id] = docMap
	}

	c.indexInfo[index].LastUpdated = time.Now()
	return nil
}

// DeleteDocument deletes a document.
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

// GetDocument retrieves a document.
func (c *MockZincClient) GetDocument(ctx context.Context, index string, id string) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	indexDocs, exists := c.indices[index]
	if !exists {
		return nil, nil
	}

	doc, exists := indexDocs[id]
	if !exists {
		return nil, fmt.Errorf("document %s not found", id)
	}

	// Convert to map via JSON roundtrip
	result, _ := docToMap(doc)
	if result == nil {
		result = make(map[string]interface{})
	}

	return result, nil
}

// BulkIndex indexes multiple documents.
func (c *MockZincClient) BulkIndex(ctx context.Context, index string, docs []interface{}) error {
	for _, doc := range docs {
		// Extract ID from document
		id := extractDocID(doc)
		if id == "" {
			id = fmt.Sprintf("doc_%d", time.Now().UnixNano())
		}

		if err := c.IndexDocument(ctx, index, id, doc); err != nil {
			return err
		}
	}

	return nil
}

// Search performs a search query.
func (c *MockZincClient) Search(ctx context.Context, index string, query *platformmodels.SearchRequest) (*platformmodels.SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	indexDocs, exists := c.indices[index]
	if !exists {
		return nil, nil
	}

	var hits []platformmodels.SearchHit
	queryLower := strings.ToLower(query.Query)

	// Simple text search
	for id, doc := range indexDocs {
		match := false
		score := 0.0

		// Convert document to map via JSON roundtrip for uniform field access
		docMap, _ := docToMap(doc)

		var title, content, status, priority string
		var tags []string

		if docMap != nil {
			if t, ok := docMap["title"].(string); ok {
				title = t
			}
			if c, ok := docMap["content"].(string); ok {
				content = c
			}
			if s, ok := docMap["status"].(string); ok {
				status = s
			}
			if p, ok := docMap["priority"].(string); ok {
				priority = p
			}
			if t, ok := docMap["tags"]; ok {
				switch tv := t.(type) {
				case []string:
					tags = tv
				case []interface{}:
					for _, tag := range tv {
						if s, ok := tag.(string); ok {
							tags = append(tags, s)
						}
					}
				}
			}
		}

		// Check query match
		if query.Query == "" || query.Query == "*" {
			match = true
			score = 1.0
		} else {
			// Search in title
			if strings.Contains(strings.ToLower(title), queryLower) {
				match = true
				score += 1.0
			}
			// Search in content
			if strings.Contains(strings.ToLower(content), queryLower) {
				match = true
				score += 0.5
			}
			// Search in tags
			for _, tag := range tags {
				if strings.Contains(strings.ToLower(tag), queryLower) {
					match = true
					score += 0.3
				}
			}
		}

		// Check filters
		if match && len(query.Filters) > 0 {
			for key, value := range query.Filters {
				fieldValue := ""
				if docMap != nil {
					if v, ok := docMap[key]; ok {
						fieldValue = fmt.Sprintf("%v", v)
					}
				}
				if fieldValue != value {
					match = false
					break
				}
			}
		}

		if match {
			highlights := make(map[string][]string)
			if query.Highlight {
				excerpts := []string{"<em>" + title + "</em>"}
				if title != "" {
					highlights["title"] = excerpts
				}
				if content != "" {
					highlights["content"] = []string{"<em>" + content + "</em>"}
				}
			}
			hit := platformmodels.SearchHit{
				ID:    id,
				Type:  "ticket",
				Score: score,
				Source: map[string]interface{}{
					"title":    title,
					"content":  content,
					"status":   status,
					"priority": priority,
					"tags":     tags,
				},
				Timestamp:  time.Now(),
				Highlights: highlights,
			}
			hits = append(hits, hit)
		}
	}

	result := &platformmodels.SearchResult{
		Query:    query.Query,
		Page:     query.Page,
		PageSize: query.PageSize,
		Hits:     hits,
		Took:     0,
	}

	if result.Page == 0 {
		result.Page = 1
	}
	if result.PageSize == 0 {
		result.PageSize = 20
	}

	result.TotalHits = int64(len(hits))
	result.TotalPages = int(result.TotalHits / int64(result.PageSize))
	if result.TotalHits%int64(result.PageSize) > 0 {
		result.TotalPages++
	}

	// Apply pagination
	if result.PageSize > 0 && len(result.Hits) > result.PageSize {
		start := (result.Page - 1) * result.PageSize
		if start < 0 {
			start = 0
		}
		if start >= len(result.Hits) {
			result.Hits = nil
		} else {
			end := start + result.PageSize
			if end > len(result.Hits) {
				end = len(result.Hits)
			}
			result.Hits = result.Hits[start:end]
		}
	}

	return result, nil
}

// Suggest provides search suggestions.
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

// docToMap converts any JSON-serializable value to a map.
func docToMap(doc interface{}) (map[string]interface{}, error) {
	if docMap, ok := doc.(map[string]interface{}); ok {
		return docMap, nil
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
