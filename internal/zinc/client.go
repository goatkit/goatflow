// Package zinc provides the ZincSearch client for full-text search.
package zinc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// Client interface for Zinc search operations.
type Client interface {
	// Index operations
	CreateIndex(ctx context.Context, name string, mapping map[string]interface{}) error
	DeleteIndex(ctx context.Context, name string) error
	IndexExists(ctx context.Context, name string) (bool, error)
	GetIndexStats(ctx context.Context, name string) (*models.IndexStats, error)

	// Document operations
	IndexDocument(ctx context.Context, index string, id string, doc interface{}) error
	UpdateDocument(ctx context.Context, index string, id string, updates map[string]interface{}) error
	DeleteDocument(ctx context.Context, index string, id string) error
	GetDocument(ctx context.Context, index string, id string) (map[string]interface{}, error)
	BulkIndex(ctx context.Context, index string, docs []interface{}) error

	// Search operations
	Search(ctx context.Context, index string, query *models.SearchRequest) (*models.SearchResult, error)
	Suggest(ctx context.Context, index string, text string, field string) ([]string, error)
}

// ZincClient implements the Client interface for Zinc.
type ZincClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

// NewZincClient creates a new Zinc client.
func NewZincClient(baseURL, username, password string) *ZincClient {
	return &ZincClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateIndex creates a new index with optional mapping.
func (c *ZincClient) CreateIndex(ctx context.Context, name string, mapping map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/%s", c.baseURL, name)

	body := map[string]interface{}{
		"name": name,
	}
	if mapping != nil {
		body["mappings"] = mapping
	}

	return c.doRequest(ctx, "PUT", url, body, nil)
}

// DeleteIndex deletes an index.
func (c *ZincClient) DeleteIndex(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/api/index/%s", c.baseURL, name)
	return c.doRequest(ctx, "DELETE", url, nil, nil)
}

// IndexExists checks if an index exists.
func (c *ZincClient) IndexExists(ctx context.Context, name string) (bool, error) {
	url := fmt.Sprintf("%s/api/index", c.baseURL)

	var response struct {
		List []struct {
			Name string `json:"name"`
		} `json:"list"`
	}

	if err := c.doRequest(ctx, "GET", url, nil, &response); err != nil {
		return false, err
	}

	for _, index := range response.List {
		if index.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// GetIndexStats retrieves statistics for an index.
func (c *ZincClient) GetIndexStats(ctx context.Context, name string) (*models.IndexStats, error) {
	url := fmt.Sprintf("%s/api/index/%s/stats", c.baseURL, name)

	var response struct {
		DocNum      int64  `json:"doc_num"`
		StorageSize int64  `json:"storage_size"`
		Status      string `json:"status"`
	}

	if err := c.doRequest(ctx, "GET", url, nil, &response); err != nil {
		return nil, err
	}

	return &models.IndexStats{
		Name:          name,
		DocumentCount: response.DocNum,
		StorageSize:   response.StorageSize,
		LastUpdated:   time.Now(),
		Status:        response.Status,
	}, nil
}

// IndexDocument indexes a single document.
func (c *ZincClient) IndexDocument(ctx context.Context, index string, id string, doc interface{}) error {
	url := fmt.Sprintf("%s/api/%s/_doc/%s", c.baseURL, index, id)
	return c.doRequest(ctx, "PUT", url, doc, nil)
}

// UpdateDocument updates a document.
func (c *ZincClient) UpdateDocument(ctx context.Context, index string, id string, updates map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/%s/_update/%s", c.baseURL, index, id)

	body := map[string]interface{}{
		"doc": updates,
	}

	return c.doRequest(ctx, "POST", url, body, nil)
}

// DeleteDocument deletes a document.
func (c *ZincClient) DeleteDocument(ctx context.Context, index string, id string) error {
	url := fmt.Sprintf("%s/api/%s/_doc/%s", c.baseURL, index, id)
	return c.doRequest(ctx, "DELETE", url, nil, nil)
}

// GetDocument retrieves a document.
func (c *ZincClient) GetDocument(ctx context.Context, index string, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/%s/_doc/%s", c.baseURL, index, id)

	var response struct {
		Source map[string]interface{} `json:"_source"`
	}

	if err := c.doRequest(ctx, "GET", url, nil, &response); err != nil {
		return nil, err
	}

	return response.Source, nil
}

// BulkIndex indexes multiple documents.
func (c *ZincClient) BulkIndex(ctx context.Context, index string, docs []interface{}) error {
	url := fmt.Sprintf("%s/api/%s/_bulk", c.baseURL, index)

	// Build bulk request body
	var buffer bytes.Buffer
	for _, doc := range docs {
		// Get document ID if available
		docID := ""
		if docMap, ok := doc.(map[string]interface{}); ok {
			if id, exists := docMap["id"]; exists {
				docID = fmt.Sprintf("%v", id)
			}
		} else if searchDoc, ok := doc.(models.TicketSearchDocument); ok {
			docID = searchDoc.ID
		}

		// Index action
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": index,
			},
		}
		if docID != "" {
			action["index"].(map[string]interface{})["_id"] = docID
		}

		actionJSON, _ := json.Marshal(action)
		buffer.Write(actionJSON)
		buffer.WriteByte('\n')

		// Document
		docJSON, _ := json.Marshal(doc)
		buffer.Write(docJSON)
		buffer.WriteByte('\n')
	}

	return c.doRequest(ctx, "POST", url, buffer.Bytes(), nil)
}

// Search performs a search query.
func (c *ZincClient) Search(ctx context.Context, index string, query *models.SearchRequest) (*models.SearchResult, error) {
	url := fmt.Sprintf("%s/api/%s/_search", c.baseURL, index)

	// Build Zinc query
	zincQuery := c.buildZincQuery(query)

	var response struct {
		Took int64 `json:"took"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID        string                 `json:"_id"`
				Score     float64                `json:"_score"`
				Source    map[string]interface{} `json:"_source"`
				Highlight map[string][]string    `json:"highlight,omitempty"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations map[string]interface{} `json:"aggregations,omitempty"`
	}

	if err := c.doRequest(ctx, "POST", url, zincQuery, &response); err != nil {
		return nil, err
	}

	// Convert to SearchResult
	result := &models.SearchResult{
		Query:     query.Query,
		TotalHits: response.Hits.Total.Value,
		Page:      query.Page,
		PageSize:  query.PageSize,
		Took:      response.Took,
		Hits:      make([]models.SearchHit, 0, len(response.Hits.Hits)),
	}

	if result.Page == 0 {
		result.Page = 1
	}
	if result.PageSize == 0 {
		result.PageSize = 20
	}

	result.TotalPages = int(result.TotalHits / int64(result.PageSize))
	if result.TotalHits%int64(result.PageSize) > 0 {
		result.TotalPages++
	}

	// Convert hits
	for _, hit := range response.Hits.Hits {
		searchHit := models.SearchHit{
			ID:         hit.ID,
			Type:       "ticket",
			Score:      hit.Score,
			Source:     hit.Source,
			Highlights: hit.Highlight,
			Timestamp:  time.Now(),
		}
		result.Hits = append(result.Hits, searchHit)
	}

	return result, nil
}

// Suggest provides search suggestions.
func (c *ZincClient) Suggest(ctx context.Context, index string, text string, field string) ([]string, error) {
	url := fmt.Sprintf("%s/api/%s/_search", c.baseURL, index)

	query := map[string]interface{}{
		"suggest": map[string]interface{}{
			"text": text,
			"term": map[string]interface{}{
				"field": field,
			},
		},
	}

	var response struct {
		Suggest map[string][]struct {
			Options []struct {
				Text string `json:"text"`
			} `json:"options"`
		} `json:"suggest"`
	}

	if err := c.doRequest(ctx, "POST", url, query, &response); err != nil {
		return nil, err
	}

	var suggestions []string
	for _, suggest := range response.Suggest {
		for _, s := range suggest {
			for _, option := range s.Options {
				suggestions = append(suggestions, option.Text)
			}
		}
	}

	return suggestions, nil
}

// buildZincQuery builds a Zinc query from SearchRequest.
func (c *ZincClient) buildZincQuery(req *models.SearchRequest) map[string]interface{} {
	query := map[string]interface{}{
		"from": (req.Page - 1) * req.PageSize,
		"size": req.PageSize,
	}

	// Build query
	if req.Query != "" {
		query["query"] = map[string]interface{}{
			"query_string": map[string]interface{}{
				"query": req.Query,
			},
		}
	} else {
		query["query"] = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	// Add filters
	if len(req.Filters) > 0 {
		must := []map[string]interface{}{}
		for field, value := range req.Filters {
			must = append(must, map[string]interface{}{
				"term": map[string]interface{}{
					field: value,
				},
			})
		}

		query["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		}
	}

	// Add sorting
	if req.SortBy != "" {
		order := "asc"
		if req.SortOrder == "desc" {
			order = "desc"
		}
		query["sort"] = []map[string]interface{}{
			{
				req.SortBy: map[string]interface{}{
					"order": order,
				},
			},
		}
	}

	// Add highlighting
	if req.Highlight {
		query["highlight"] = map[string]interface{}{
			"fields": map[string]interface{}{
				"title":   map[string]interface{}{},
				"content": map[string]interface{}{},
			},
		}
	}

	return query
}

// doRequest performs an HTTP request.
func (c *ZincClient) doRequest(ctx context.Context, method, url string, body interface{}, response interface{}) error {
	var reqBody io.Reader

	if body != nil {
		switch v := body.(type) {
		case []byte:
			reqBody = bytes.NewReader(v)
		default:
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("failed to marshal request body: %w", err)
			}
			reqBody = bytes.NewReader(jsonBody)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
