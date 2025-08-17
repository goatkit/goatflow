package models

import (
	"time"
)

// SearchRequest represents a search query
type SearchRequest struct {
	Query       string            `json:"query" binding:"required"`
	Type        string            `json:"type,omitempty"`        // tickets, notes, customers
	Filters     map[string]string `json:"filters,omitempty"`
	DateFrom    *time.Time        `json:"date_from,omitempty"`
	DateTo      *time.Time        `json:"date_to,omitempty"`
	Page        int               `json:"page,omitempty"`
	PageSize    int               `json:"page_size,omitempty"`
	SortBy      string            `json:"sort_by,omitempty"`
	SortOrder   string            `json:"sort_order,omitempty"`
	Highlight   bool              `json:"highlight,omitempty"`
	Facets      []string          `json:"facets,omitempty"`
}

// SearchResult represents search results
type SearchResult struct {
	Query       string           `json:"query"`
	TotalHits   int64            `json:"total_hits"`
	Page        int              `json:"page"`
	PageSize    int              `json:"page_size"`
	TotalPages  int              `json:"total_pages"`
	Took        int64            `json:"took_ms"`
	Hits        []SearchHit      `json:"hits"`
	Facets      map[string][]Facet `json:"facets,omitempty"`
	Suggestions []string         `json:"suggestions,omitempty"`
}

// SearchHit represents a single search result
type SearchHit struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Score       float64                `json:"score"`
	Source      map[string]interface{} `json:"source"`
	Highlights  map[string][]string    `json:"highlights,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Facet represents a search facet
type Facet struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// TicketSearchDocument represents a ticket in the search index
type TicketSearchDocument struct {
	ID            string    `json:"id"`
	TicketNumber  string    `json:"ticket_number"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Status        string    `json:"status"`
	Priority      string    `json:"priority"`
	Queue         string    `json:"queue"`
	CustomerName  string    `json:"customer_name"`
	CustomerEmail string    `json:"customer_email"`
	AgentName     string    `json:"agent_name"`
	AgentEmail    string    `json:"agent_email"`
	Tags          []string  `json:"tags"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	
	// Additional searchable content
	Messages      []string  `json:"messages"`
	InternalNotes []string  `json:"internal_notes"`
	Attachments   []string  `json:"attachment_names"`
}

// SearchFilter represents advanced search filters
type SearchFilter struct {
	// Text search
	Query           string   `json:"query,omitempty"`
	MustContain     []string `json:"must_contain,omitempty"`
	MustNotContain  []string `json:"must_not_contain,omitempty"`
	
	// Field-specific filters
	Statuses        []string `json:"statuses,omitempty"`
	Priorities      []string `json:"priorities,omitempty"`
	Queues          []string `json:"queues,omitempty"`
	Agents          []string `json:"agents,omitempty"`
	Customers       []string `json:"customers,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	
	// Date ranges
	CreatedAfter    *time.Time `json:"created_after,omitempty"`
	CreatedBefore   *time.Time `json:"created_before,omitempty"`
	UpdatedAfter    *time.Time `json:"updated_after,omitempty"`
	UpdatedBefore   *time.Time `json:"updated_before,omitempty"`
	ResolvedAfter   *time.Time `json:"resolved_after,omitempty"`
	ResolvedBefore  *time.Time `json:"resolved_before,omitempty"`
	
	// Numeric filters
	MinResponseTime int `json:"min_response_time,omitempty"`
	MaxResponseTime int `json:"max_response_time,omitempty"`
	MinMessages     int `json:"min_messages,omitempty"`
	MaxMessages     int `json:"max_messages,omitempty"`
	
	// Special filters
	HasAttachments  *bool `json:"has_attachments,omitempty"`
	HasInternalNotes *bool `json:"has_internal_notes,omitempty"`
	IsEscalated     *bool `json:"is_escalated,omitempty"`
	IsOverdue       *bool `json:"is_overdue,omitempty"`
}

// SavedSearch represents a saved search query
type SavedSearch struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"user_id"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	Filter      SearchFilter `json:"filter"`
	IsPublic    bool      `json:"is_public"`
	IsDefault   bool      `json:"is_default"`
	UsageCount  int       `json:"usage_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SearchHistory represents a user's search history
type SearchHistory struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Query     string    `json:"query"`
	Filter    SearchFilter `json:"filter"`
	Results   int       `json:"results"`
	SearchedAt time.Time `json:"searched_at"`
}

// SearchSuggestion represents a search suggestion
type SearchSuggestion struct {
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
	Frequency  int     `json:"frequency"`
	Type       string  `json:"type"` // query, field, value
}

// IndexStats represents search index statistics
type IndexStats struct {
	Name          string    `json:"name"`
	DocumentCount int64     `json:"document_count"`
	StorageSize   int64     `json:"storage_size"`
	LastUpdated   time.Time `json:"last_updated"`
	Status        string    `json:"status"`
}

// SearchAnalytics represents search usage analytics
type SearchAnalytics struct {
	TotalSearches     int64              `json:"total_searches"`
	UniqueUsers       int                `json:"unique_users"`
	AverageResultSize float64            `json:"average_result_size"`
	TopQueries        []QueryStats       `json:"top_queries"`
	TopFilters        map[string]int     `json:"top_filters"`
	SearchTrends      []TrendPoint       `json:"search_trends"`
	ZeroResultQueries []string           `json:"zero_result_queries"`
}

// QueryStats represents statistics for a specific query
type QueryStats struct {
	Query     string  `json:"query"`
	Count     int     `json:"count"`
	AvgResults float64 `json:"avg_results"`
	ClickRate float64 `json:"click_rate"`
}

// TrendPoint represents a point in a trend graph
type TrendPoint struct {
	Time  time.Time `json:"time"`
	Value int       `json:"value"`
}