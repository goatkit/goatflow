// Package search provides search backend implementations including Elasticsearch.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ElasticBackend implements SearchBackend using Elasticsearch-compatible APIs (works with Zinc).
type ElasticBackend struct {
	endpoint string
	username string
	password string
	client   *http.Client
}

// NewElasticBackend creates a new Elasticsearch/Zinc search backend.
func NewElasticBackend(endpoint, username, password string) *ElasticBackend {
	if endpoint == "" {
		endpoint = os.Getenv("ELASTIC_ENDPOINT")
		if endpoint == "" {
			endpoint = "http://localhost:4080" // Default Zinc port
		}
	}
	if username == "" {
		username = os.Getenv("ELASTIC_USERNAME")
		if username == "" {
			username = "admin"
		}
	}
	if password == "" {
		password = os.Getenv("ELASTIC_PASSWORD")
		if password == "" {
			password = "admin"
		}
	}

	return &ElasticBackend{
		endpoint: endpoint,
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetBackendName returns the backend name.
func (eb *ElasticBackend) GetBackendName() string {
	return "elasticsearch"
}

// Search performs a search using Elasticsearch/Zinc.
func (eb *ElasticBackend) Search(ctx context.Context, query SearchQuery) (*SearchResults, error) {
	// Build Elasticsearch query
	esQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  query.Query,
				"fields": []string{"title^2", "content", "subject", "body"},
				"type":   "best_fields",
			},
		},
		"size": query.Limit,
		"from": query.Offset,
	}

	// Add type filter if specified
	if len(query.Types) > 0 {
		esQuery["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"multi_match": map[string]interface{}{
							"query":  query.Query,
							"fields": []string{"title^2", "content", "subject", "body"},
						},
					},
				},
				"filter": []interface{}{
					map[string]interface{}{
						"terms": map[string]interface{}{
							"type": query.Types,
						},
					},
				},
			},
		}
	}

	// Add highlighting
	if query.Highlight {
		esQuery["highlight"] = map[string]interface{}{
			"fields": map[string]interface{}{
				"title":   map[string]interface{}{},
				"content": map[string]interface{}{},
				"subject": map[string]interface{}{},
				"body":    map[string]interface{}{},
			},
		}
	}

	// Add sorting
	if query.SortBy != "" {
		order := "desc"
		if query.SortOrder == "asc" {
			order = "asc"
		}
		esQuery["sort"] = []interface{}{
			map[string]interface{}{
				query.SortBy: map[string]interface{}{
					"order": order,
				},
			},
		}
	}

	// Send search request
	body, err := json.Marshal(esQuery)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/_search", eb.endpoint), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(eb.username, eb.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := eb.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", string(bodyBytes))
	}

	// Parse response
	var esResp ElasticSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, err
	}

	// Convert to our format
	results := &SearchResults{
		Query:     query.Query,
		TotalHits: esResp.Hits.Total.Value,
		Took:      esResp.Took,
		Hits:      []SearchHit{},
	}

	for _, hit := range esResp.Hits.Hits {
		searchHit := SearchHit{
			ID:       hit.ID,
			Type:     hit.Source["type"].(string),
			Score:    hit.Score,
			Title:    getStringField(hit.Source, "title"),
			Content:  getStringField(hit.Source, "content"),
			Metadata: hit.Source,
		}

		// Add highlights if present
		if len(hit.Highlight) > 0 {
			searchHit.Highlights = hit.Highlight
		}

		results.Hits = append(results.Hits, searchHit)
	}

	return results, nil
}

// Index adds or updates a document in Elasticsearch/Zinc.
func (eb *ElasticBackend) Index(ctx context.Context, doc Document) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s/_doc/%s", eb.endpoint, doc.Type, doc.ID)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.SetBasicAuth(eb.username, eb.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := eb.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("indexing failed: %s", string(bodyBytes))
	}

	return nil
}

// Delete removes a document from Elasticsearch/Zinc.
func (eb *ElasticBackend) Delete(ctx context.Context, docType string, id string) error {
	url := fmt.Sprintf("%s/%s/_doc/%s", eb.endpoint, docType, id)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(eb.username, eb.password)

	resp, err := eb.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deletion failed: %s", string(bodyBytes))
	}

	return nil
}

// BulkIndex indexes multiple documents at once.
func (eb *ElasticBackend) BulkIndex(ctx context.Context, docs []Document) error {
	var bulkBody bytes.Buffer

	for _, doc := range docs {
		// Index action
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": doc.Type,
				"_id":    doc.ID,
			},
		}
		actionBytes, _ := json.Marshal(action)
		bulkBody.Write(actionBytes)
		bulkBody.WriteByte('\n')

		// Document
		docBytes, _ := json.Marshal(doc)
		bulkBody.Write(docBytes)
		bulkBody.WriteByte('\n')
	}

	url := fmt.Sprintf("%s/_bulk", eb.endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &bulkBody)
	if err != nil {
		return err
	}

	req.SetBasicAuth(eb.username, eb.password)
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := eb.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bulk indexing failed: %s", string(bodyBytes))
	}

	return nil
}

// HealthCheck verifies the Elasticsearch/Zinc connection.
func (eb *ElasticBackend) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/healthz", eb.endpoint), nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(eb.username, eb.password)

	resp, err := eb.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// ElasticSearchResponse represents Elasticsearch API response.
type ElasticSearchResponse struct {
	Took int64 `json:"took"`
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID        string                 `json:"_id"`
			Score     float64                `json:"_score"`
			Source    map[string]interface{} `json:"_source"`
			Highlight map[string][]string    `json:"highlight,omitempty"`
		} `json:"hits"`
	} `json:"hits"`
}

// getStringField safely extracts a string field from a map.
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
