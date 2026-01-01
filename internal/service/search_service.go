package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/zinc"
)

// SearchService handles search operations for tickets.
type SearchService struct {
	client        zinc.Client
	savedSearches map[uint]*models.SavedSearch
	searchHistory []models.SearchHistory
	mu            sync.RWMutex
	nextSearchID  uint
	nextHistoryID uint
}

// NewSearchService creates a new search service.
func NewSearchService(client zinc.Client) *SearchService {
	service := &SearchService{
		client:        client,
		savedSearches: make(map[uint]*models.SavedSearch),
		searchHistory: []models.SearchHistory{},
		nextSearchID:  1,
		nextHistoryID: 1,
	}

	// Initialize tickets index
	service.initializeIndex(context.Background())

	return service
}

// initializeIndex creates the tickets index if it doesn't exist.
func (s *SearchService) initializeIndex(ctx context.Context) error {
	exists, err := s.client.IndexExists(ctx, "tickets")
	if err != nil {
		return err
	}

	if !exists {
		mapping := map[string]interface{}{
			"properties": map[string]interface{}{
				"ticket_number": map[string]interface{}{
					"type": "keyword",
				},
				"title": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"content": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"priority": map[string]interface{}{
					"type": "keyword",
				},
				"queue": map[string]interface{}{
					"type": "keyword",
				},
				"customer_name": map[string]interface{}{
					"type": "text",
				},
				"customer_email": map[string]interface{}{
					"type": "keyword",
				},
				"agent_name": map[string]interface{}{
					"type": "text",
				},
				"tags": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		}

		return s.client.CreateIndex(ctx, "tickets", mapping)
	}

	return nil
}

// IndexTicket indexes a single ticket.
func (s *SearchService) IndexTicket(ctx context.Context, ticket *models.Ticket) error {
	doc := s.mapTicketToSearchDocument(ticket)
	return s.client.IndexDocument(ctx, "tickets", doc.ID, doc)
}

// UpdateTicketInIndex updates a ticket in the search index.
func (s *SearchService) UpdateTicketInIndex(ctx context.Context, ticket *models.Ticket) error {
	doc := s.mapTicketToSearchDocument(ticket)

	updates := map[string]interface{}{
		"title":      doc.Title,
		"content":    doc.Content,
		"status":     doc.Status,
		"priority":   doc.Priority,
		"queue":      doc.Queue,
		"updated_at": doc.UpdatedAt,
	}

	return s.client.UpdateDocument(ctx, "tickets", doc.ID, updates)
}

// DeleteTicketFromIndex removes a ticket from the search index.
func (s *SearchService) DeleteTicketFromIndex(ctx context.Context, ticketNumber string) error {
	return s.client.DeleteDocument(ctx, "tickets", ticketNumber)
}

// BulkIndexTickets indexes multiple tickets at once.
func (s *SearchService) BulkIndexTickets(ctx context.Context, tickets []models.Ticket) error {
	docs := make([]interface{}, len(tickets))
	for i, ticket := range tickets {
		docs[i] = s.mapTicketToSearchDocument(&ticket)
	}
	return s.client.BulkIndex(ctx, "tickets", docs)
}

// SearchTickets performs a ticket search.
func (s *SearchService) SearchTickets(ctx context.Context, request *models.SearchRequest) (*models.SearchResult, error) {
	// Set defaults
	if request.Page == 0 {
		request.Page = 1
	}
	if request.PageSize == 0 {
		request.PageSize = 20
	}

	return s.client.Search(ctx, "tickets", request)
}

// SearchWithFilter performs an advanced search with filters.
func (s *SearchService) SearchWithFilter(ctx context.Context, filter *models.SearchFilter) (*models.SearchResult, error) {
	// Build search request from filter
	request := &models.SearchRequest{
		Query:     filter.Query,
		Page:      1,
		PageSize:  20,
		Filters:   make(map[string]string),
		Highlight: true,
	}

	// Apply status filter
	if len(filter.Statuses) > 0 {
		// For simplicity, use the first status
		request.Filters["status"] = s.mapStateToStatus(filter.Statuses[0])
	}

	// Apply priority filter
	if len(filter.Priorities) > 0 {
		request.Filters["priority"] = filter.Priorities[0]
	}

	// Apply queue filter
	if len(filter.Queues) > 0 {
		request.Filters["queue"] = filter.Queues[0]
	}

	// Apply date filters
	if filter.CreatedAfter != nil {
		request.DateFrom = filter.CreatedAfter
	}
	if filter.CreatedBefore != nil {
		request.DateTo = filter.CreatedBefore
	}

	return s.client.Search(ctx, "tickets", request)
}

// GetSearchSuggestions provides search suggestions.
func (s *SearchService) GetSearchSuggestions(ctx context.Context, text string) ([]string, error) {
	return s.client.Suggest(ctx, "tickets", text, "title")
}

// SaveSearch saves a search query for later use.
func (s *SearchService) SaveSearch(ctx context.Context, search *models.SavedSearch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	search.ID = s.nextSearchID
	s.nextSearchID++
	search.CreatedAt = time.Now()
	search.UpdatedAt = search.CreatedAt

	// Create a copy to store
	stored := *search
	s.savedSearches[search.ID] = &stored

	return nil
}

// GetSavedSearch retrieves a saved search.
func (s *SearchService) GetSavedSearch(ctx context.Context, id uint) (*models.SavedSearch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	search, exists := s.savedSearches[id]
	if !exists {
		return nil, fmt.Errorf("saved search %d not found", id)
	}

	// Return a copy
	result := *search
	return &result, nil
}

// GetSavedSearches retrieves all saved searches for a user.
func (s *SearchService) GetSavedSearches(ctx context.Context, userID uint) ([]models.SavedSearch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var searches []models.SavedSearch
	for _, search := range s.savedSearches {
		if search.UserID == userID || search.IsPublic {
			searches = append(searches, *search)
		}
	}

	return searches, nil
}

// ExecuteSavedSearch runs a saved search.
func (s *SearchService) ExecuteSavedSearch(ctx context.Context, id uint) (*models.SearchResult, error) {
	search, err := s.GetSavedSearch(ctx, id)
	if err != nil {
		return nil, err
	}

	// Increment usage count
	s.mu.Lock()
	s.savedSearches[id].UsageCount++
	s.mu.Unlock()

	return s.SearchWithFilter(ctx, &search.Filter)
}

// RecordSearchHistory records a search in history.
func (s *SearchService) RecordSearchHistory(ctx context.Context, userID uint, request *models.SearchRequest, results *models.SearchResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	history := models.SearchHistory{
		ID:         s.nextHistoryID,
		UserID:     userID,
		Query:      request.Query,
		Filter:     s.requestToFilter(request),
		Results:    int(results.TotalHits),
		SearchedAt: time.Now(),
	}
	s.nextHistoryID++

	s.searchHistory = append(s.searchHistory, history)

	// Keep only last 1000 entries
	if len(s.searchHistory) > 1000 {
		s.searchHistory = s.searchHistory[len(s.searchHistory)-1000:]
	}

	return nil
}

// GetSearchHistory retrieves search history for a user.
func (s *SearchService) GetSearchHistory(ctx context.Context, userID uint, limit int) ([]models.SearchHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var history []models.SearchHistory
	count := 0

	// Iterate in reverse for most recent first
	for i := len(s.searchHistory) - 1; i >= 0 && count < limit; i-- {
		if s.searchHistory[i].UserID == userID {
			history = append(history, s.searchHistory[i])
			count++
		}
	}

	return history, nil
}

// GetSearchAnalytics generates search analytics.
func (s *SearchService) GetSearchAnalytics(ctx context.Context, from, to time.Time) (*models.SearchAnalytics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	analytics := &models.SearchAnalytics{
		TopFilters: make(map[string]int),
	}

	queryCount := make(map[string]int)
	queryResults := make(map[string][]int)
	userSet := make(map[uint]bool)

	for _, hist := range s.searchHistory {
		if hist.SearchedAt.After(from) && hist.SearchedAt.Before(to) {
			analytics.TotalSearches++
			userSet[hist.UserID] = true

			queryCount[hist.Query]++
			queryResults[hist.Query] = append(queryResults[hist.Query], hist.Results)

			// Count filters
			if len(hist.Filter.Statuses) > 0 {
				analytics.TopFilters["status"]++
			}
			if len(hist.Filter.Priorities) > 0 {
				analytics.TopFilters["priority"]++
			}
			if len(hist.Filter.Tags) > 0 {
				analytics.TopFilters["tags"]++
			}
		}
	}

	analytics.UniqueUsers = len(userSet)

	// Calculate top queries
	for query, count := range queryCount {
		avgResults := 0.0
		if len(queryResults[query]) > 0 {
			sum := 0
			for _, r := range queryResults[query] {
				sum += r
			}
			avgResults = float64(sum) / float64(len(queryResults[query]))
		}

		analytics.TopQueries = append(analytics.TopQueries, models.QueryStats{
			Query:      query,
			Count:      count,
			AvgResults: avgResults,
		})
	}

	// Sort top queries by count
	for i := 0; i < len(analytics.TopQueries); i++ {
		for j := i + 1; j < len(analytics.TopQueries); j++ {
			if analytics.TopQueries[j].Count > analytics.TopQueries[i].Count {
				analytics.TopQueries[i], analytics.TopQueries[j] = analytics.TopQueries[j], analytics.TopQueries[i]
			}
		}
	}

	// Keep only top 10
	if len(analytics.TopQueries) > 10 {
		analytics.TopQueries = analytics.TopQueries[:10]
	}

	// Calculate average result size
	if analytics.TotalSearches > 0 {
		totalResults := 0
		for _, hist := range s.searchHistory {
			if hist.SearchedAt.After(from) && hist.SearchedAt.Before(to) {
				totalResults += hist.Results
			}
		}
		analytics.AverageResultSize = float64(totalResults) / float64(analytics.TotalSearches)
	}

	return analytics, nil
}

// ReindexAllTickets reindexes all tickets.
func (s *SearchService) ReindexAllTickets(ctx context.Context, fetcher func() ([]models.Ticket, error)) (*ReindexStats, error) {
	stats := &ReindexStats{
		StartTime: time.Now(),
	}

	// Fetch all tickets
	tickets, err := fetcher()
	if err != nil {
		return stats, fmt.Errorf("failed to fetch tickets: %w", err)
	}

	stats.Total = len(tickets)

	// Delete and recreate index
	s.client.DeleteIndex(ctx, "tickets")
	s.initializeIndex(ctx)

	// Bulk index in batches
	batchSize := 100
	for i := 0; i < len(tickets); i += batchSize {
		end := i + batchSize
		if end > len(tickets) {
			end = len(tickets)
		}

		batch := tickets[i:end]
		if err := s.BulkIndexTickets(ctx, batch); err != nil {
			stats.Failed += len(batch)
			fmt.Printf("Failed to index batch %d-%d: %v\n", i, end, err)
		} else {
			stats.Indexed += len(batch)
		}
	}

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	return stats, nil
}

// mapTicketToSearchDocument converts a ticket to a search document.
func (s *SearchService) mapTicketToSearchDocument(ticket *models.Ticket) *models.TicketSearchDocument {
	doc := &models.TicketSearchDocument{
		ID:           ticket.TicketNumber,
		TicketNumber: ticket.TicketNumber,
		Title:        ticket.Title,
		Status:       s.mapTicketStateToStatus(ticket.TicketStateID),
		Priority:     s.mapTicketPriorityToString(ticket.TicketPriorityID),
		Queue:        s.mapQueueIDToName(ticket.QueueID),
		CreatedAt:    ticket.CreateTime,
		UpdatedAt:    ticket.ChangeTime,
	}

	// Add customer info if available
	if ticket.CustomerID != nil {
		doc.CustomerEmail = *ticket.CustomerID
	}

	// Add agent info if available
	if ticket.UserID != nil {
		doc.AgentName = fmt.Sprintf("Agent %d", *ticket.UserID)
	}

	// TODO: Add messages, internal notes, and attachments when available

	return doc
}

// mapTicketStateToStatus maps ticket state ID to status string.
func (s *SearchService) mapTicketStateToStatus(stateID int) string {
	// Based on OTRS default states
	statusMap := map[int]string{
		1:  "new",
		2:  "open",
		3:  "pending reminder",
		4:  "closed",
		5:  "removed",
		6:  "pending auto close+",
		7:  "pending auto close-",
		8:  "merged",
		9:  "closed successful",
		10: "closed unsuccessful",
	}

	if status, ok := statusMap[stateID]; ok {
		return status
	}
	return "unknown"
}

// mapStateToStatus maps state name to status.
func (s *SearchService) mapStateToStatus(state string) string {
	// Simple mapping for common states
	switch state {
	case "new", "open", "closed":
		return state
	default:
		return "open"
	}
}

// mapTicketPriorityToString maps priority ID to string.
func (s *SearchService) mapTicketPriorityToString(priorityID int) string {
	priorityMap := map[int]string{
		1: "very low",
		2: "low",
		3: "normal",
		4: "high",
		5: "very high",
	}

	if priority, ok := priorityMap[priorityID]; ok {
		return priority
	}
	return "normal"
}

// mapQueueIDToName maps queue ID to name.
func (s *SearchService) mapQueueIDToName(queueID int) string {
	// TODO: Fetch actual queue names from database
	queueMap := map[int]string{
		1: "General",
		2: "Support",
		3: "Sales",
		4: "Technical",
	}

	if name, ok := queueMap[queueID]; ok {
		return name
	}
	return fmt.Sprintf("Queue %d", queueID)
}

// requestToFilter converts a search request to a filter.
func (s *SearchService) requestToFilter(request *models.SearchRequest) models.SearchFilter {
	filter := models.SearchFilter{
		Query: request.Query,
	}

	// Extract filters
	if status, ok := request.Filters["status"]; ok {
		filter.Statuses = []string{status}
	}
	if priority, ok := request.Filters["priority"]; ok {
		filter.Priorities = []string{priority}
	}
	if queue, ok := request.Filters["queue"]; ok {
		filter.Queues = []string{queue}
	}

	filter.CreatedAfter = request.DateFrom
	filter.CreatedBefore = request.DateTo

	return filter
}

// ReindexStats contains statistics for reindexing operation.
type ReindexStats struct {
	Total     int
	Indexed   int
	Failed    int
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}
