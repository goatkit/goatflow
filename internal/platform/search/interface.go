// Package search implements GoatFlow's search backends: the application
// database (default) and an Elasticsearch-compatible index (Zinc or
// Elasticsearch) kept in sync with the database.
package search

import (
	"context"
	"errors"
)

// SearchBackend searches tickets, articles and customers.
type SearchBackend interface {
	// Search performs a search across the requested entity types.
	Search(ctx context.Context, query SearchQuery) (*SearchResults, error)

	// HealthCheck reports whether the backend can serve searches.
	HealthCheck(ctx context.Context) error

	// GetBackendName returns the name of the search backend.
	GetBackendName() string
}

// SearchQuery represents a search request.
type SearchQuery struct {
	Query     string            `json:"query"`     // The search query string
	Types     []string          `json:"types"`     // Entity types to search (ticket, article, customer)
	Filters   map[string]string `json:"filters"`   // Ticket filters: queue_id, state_id
	Offset    int               `json:"offset"`    // Pagination offset
	Limit     int               `json:"limit"`     // Results per page
	Highlight bool              `json:"highlight"` // Enable result highlighting

	// RestrictQueues limits ticket and article hits to tickets in QueueIDs (an
	// empty QueueIDs then matches none). Set by the server from the caller's
	// permissions, never from the request body.
	RestrictQueues bool  `json:"-"`
	QueueIDs       []int `json:"-"`
}

// SearchResults contains search results.
type SearchResults struct {
	Query     string      `json:"query"`
	TotalHits int         `json:"total_hits"`
	Took      int64       `json:"took_ms"` // Time taken in milliseconds
	Hits      []SearchHit `json:"hits"`
}

// SearchHit represents a single search result.
type SearchHit struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Score      float64                `json:"score"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Highlights map[string][]string    `json:"highlights,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
}

var (
	// ErrInvalidQuery: the query or one of its filters is malformed.
	ErrInvalidQuery = errors.New("invalid search query")
	// ErrIndexNotReady: the external index has not been built yet (or was
	// removed); searching it would return incomplete results.
	ErrIndexNotReady = errors.New("search index has not been built yet")
)

// ServiceError reports that the external search service could not serve a
// request. Reason is safe to show to users; Err carries the detail.
type ServiceError struct {
	Backend string
	Reason  string
	Err     error
}

func (e *ServiceError) Error() string {
	return e.Backend + ": " + e.Reason + ": " + e.Err.Error()
}

func (e *ServiceError) Unwrap() error { return e.Err }
