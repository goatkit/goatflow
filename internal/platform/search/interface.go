package search

import (
	"context"
	"time"
)

// SearchBackend defines the interface for pluggable search implementations.
type SearchBackend interface {
	// Search performs a search across specified entities
	Search(ctx context.Context, query SearchQuery) (*SearchResults, error)

	// Index adds or updates a document in the search index
	Index(ctx context.Context, doc Document) error

	// Delete removes a document from the search index
	Delete(ctx context.Context, docType string, id string) error

	// BulkIndex indexes multiple documents at once
	BulkIndex(ctx context.Context, docs []Document) error

	// HealthCheck verifies the search backend is operational
	HealthCheck(ctx context.Context) error

	// GetBackendName returns the name of the search backend
	GetBackendName() string
}

// SearchQuery represents a search request.
type SearchQuery struct {
	Query     string            `json:"query"`      // The search query string
	Types     []string          `json:"types"`      // Entity types to search (ticket, article, customer)
	Filters   map[string]string `json:"filters"`    // Additional filters
	Offset    int               `json:"offset"`     // Pagination offset
	Limit     int               `json:"limit"`      // Results per page
	SortBy    string            `json:"sort_by"`    // Sort field
	SortOrder string            `json:"sort_order"` // asc or desc
	Highlight bool              `json:"highlight"`  // Enable result highlighting
	Facets    []string          `json:"facets"`     // Fields to generate facets for
}

// Document represents a searchable document.
type Document struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // ticket, article, customer, etc.
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
	ModifiedAt time.Time              `json:"modified_at"`
}

// SearchResults contains search results.
type SearchResults struct {
	Query       string             `json:"query"`
	TotalHits   int                `json:"total_hits"`
	Took        int64              `json:"took_ms"` // Time taken in milliseconds
	Hits        []SearchHit        `json:"hits"`
	Facets      map[string][]Facet `json:"facets,omitempty"`
	Suggestions []string           `json:"suggestions,omitempty"`
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

// Facet represents a search facet.
type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// SearchManager manages different search backend implementations.
type SearchManager struct {
	backends map[string]SearchBackend
	primary  SearchBackend
}

// NewSearchManager creates a new search manager.
func NewSearchManager() *SearchManager {
	return &SearchManager{
		backends: make(map[string]SearchBackend),
	}
}

// RegisterBackend registers a search backend.
func (sm *SearchManager) RegisterBackend(name string, backend SearchBackend, isPrimary bool) {
	sm.backends[name] = backend
	if isPrimary {
		sm.primary = backend
	}
}

// GetBackend returns a specific backend by name.
func (sm *SearchManager) GetBackend(name string) (SearchBackend, bool) {
	backend, exists := sm.backends[name]
	return backend, exists
}

// GetPrimaryBackend returns the primary search backend.
func (sm *SearchManager) GetPrimaryBackend() SearchBackend {
	return sm.primary
}

// Search performs a search using the primary backend.
func (sm *SearchManager) Search(ctx context.Context, query SearchQuery) (*SearchResults, error) {
	if sm.primary == nil {
		// Fallback to first available backend
		for _, backend := range sm.backends {
			return backend.Search(ctx, query)
		}
		return nil, ErrNoBackendAvailable
	}
	return sm.primary.Search(ctx, query)
}

// Common errors.
var (
	ErrNoBackendAvailable = &SearchError{Code: "NO_BACKEND", Message: "No search backend available"}
	ErrInvalidQuery       = &SearchError{Code: "INVALID_QUERY", Message: "Invalid search query"}
	ErrIndexingFailed     = &SearchError{Code: "INDEXING_FAILED", Message: "Failed to index document"}
)

// SearchError represents a search-related error.
type SearchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SearchError) Error() string {
	return e.Message
}
