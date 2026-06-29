package models

import "time"

// SearchRequest represents a search query.
type SearchRequest struct {
	Query     string            `json:"query" binding:"required"`
	Type      string            `json:"type,omitempty"` // tickets, notes, customers
	Filters   map[string]string `json:"filters,omitempty"`
	DateFrom  *time.Time        `json:"date_from,omitempty"`
	DateTo    *time.Time        `json:"date_to,omitempty"`
	Page      int               `json:"page,omitempty"`
	PageSize  int               `json:"page_size,omitempty"`
	SortBy    string            `json:"sort_by,omitempty"`
	SortOrder string            `json:"sort_order,omitempty"`
	Highlight bool              `json:"highlight,omitempty"`
	Facets    []string          `json:"facets,omitempty"`
}

// SearchResult represents search results.
type SearchResult struct {
	Query       string             `json:"query"`
	TotalHits   int64              `json:"total_hits"`
	Page        int                `json:"page"`
	PageSize    int                `json:"page_size"`
	TotalPages  int                `json:"total_pages"`
	Took        int64              `json:"took_ms"`
	Hits        []SearchHit        `json:"hits"`
	Facets      map[string][]Facet `json:"facets,omitempty"`
	Suggestions []string           `json:"suggestions,omitempty"`
}

// SearchHit represents a single search result.
type SearchHit struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Score      float64                `json:"score"`
	Source     map[string]interface{} `json:"source"`
	Highlights map[string][]string    `json:"highlights,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Facet represents a search facet.
type Facet struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// IndexStats represents search index statistics (Zinc-specific).
type IndexStats struct {
	Name          string    `json:"name"`
	DocumentCount int64     `json:"document_count"`
	StorageSize   int64     `json:"storage_size"`
	LastUpdated   time.Time `json:"last_updated"`
	Status        string    `json:"status"`
}
