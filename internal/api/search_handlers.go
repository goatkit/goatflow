package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/zinc"
)

// SearchHandlers handles search-related API endpoints.
type SearchHandlers struct {
	searchService *service.SearchService
}

// NewSearchHandlers creates new search handlers.
func NewSearchHandlers() *SearchHandlers {
	// Initialize with mock client for now
	// TODO: Replace with real Zinc client in production
	client := zinc.NewMockZincClient()
	return &SearchHandlers{
		searchService: service.NewSearchService(client),
	}
}

// SearchTickets performs a ticket search.
func (h *SearchHandlers) SearchTickets(c *gin.Context) {
	var request models.SearchRequest

	// Get query parameters
	request.Query = c.Query("q")
	if request.Query == "" {
		request.Query = "*" // Search all if no query
	}

	// Pagination
	if page, err := strconv.Atoi(c.Query("page")); err == nil && page > 0 {
		request.Page = page
	} else {
		request.Page = 1
	}

	if pageSize, err := strconv.Atoi(c.Query("page_size")); err == nil && pageSize > 0 {
		request.PageSize = pageSize
	} else {
		request.PageSize = 20
	}

	// Filters
	request.Filters = make(map[string]string)
	if status := c.Query("status"); status != "" {
		request.Filters["status"] = status
	}
	if priority := c.Query("priority"); priority != "" {
		request.Filters["priority"] = priority
	}
	if queue := c.Query("queue"); queue != "" {
		request.Filters["queue"] = queue
	}

	// Sorting
	request.SortBy = c.Query("sort_by")
	request.SortOrder = c.Query("sort_order")

	// Highlighting
	request.Highlight = c.Query("highlight") == "true"

	// Perform search
	results, err := h.searchService.SearchTickets(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record search history for authenticated users
	if uid := GetUserIDFromCtxUint(c, 0); uid > 0 {
		_ = h.searchService.RecordSearchHistory(c.Request.Context(), uid, &request, results) //nolint:errcheck // Best-effort history
	}

	c.JSON(http.StatusOK, results)
}

// AdvancedSearch performs an advanced search with filters.
func (h *SearchHandlers) AdvancedSearch(c *gin.Context) {
	var filter models.SearchFilter

	if err := c.ShouldBindJSON(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := h.searchService.SearchWithFilter(c.Request.Context(), &filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record search history
	if uid := GetUserIDFromCtxUint(c, 0); uid > 0 {
		request := &models.SearchRequest{
			Query:    filter.Query,
			PageSize: 20,
		}
		_ = h.searchService.RecordSearchHistory(c.Request.Context(), uid, request, results) //nolint:errcheck // Best-effort history
	}

	c.JSON(http.StatusOK, results)
}

// GetSearchSuggestions provides search suggestions.
func (h *SearchHandlers) GetSearchSuggestions(c *gin.Context) {
	text := c.Query("text")
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text parameter required"})
		return
	}

	field := c.DefaultQuery("field", "title")

	suggestions, err := h.searchService.GetSearchSuggestions(c.Request.Context(), text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
		"field":       field,
	})
}

// SaveSearch saves a search query.
func (h *SearchHandlers) SaveSearch(c *gin.Context) {
	var savedSearch models.SavedSearch

	if err := c.ShouldBindJSON(&savedSearch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set user ID from context
	savedSearch.UserID = GetUserIDFromCtxUint(c, 1)

	if err := h.searchService.SaveSearch(c.Request.Context(), &savedSearch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, savedSearch)
}

// GetSavedSearches retrieves saved searches for a user.
func (h *SearchHandlers) GetSavedSearches(c *gin.Context) {
	userID := GetUserIDFromCtxUint(c, 1)

	searches, err := h.searchService.GetSavedSearches(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, searches)
}

// GetSavedSearch retrieves a specific saved search.
func (h *SearchHandlers) GetSavedSearch(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid saved search ID"})
		return
	}

	search, err := h.searchService.GetSavedSearch(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, search)
}

// ExecuteSavedSearch executes a saved search.
func (h *SearchHandlers) ExecuteSavedSearch(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid saved search ID"})
		return
	}

	results, err := h.searchService.ExecuteSavedSearch(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// GetSearchHistory retrieves search history for a user.
func (h *SearchHandlers) GetSearchHistory(c *gin.Context) {
	userID := GetUserIDFromCtxUint(c, 1)

	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	history, err := h.searchService.GetSearchHistory(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetSearchAnalytics retrieves search analytics.
func (h *SearchHandlers) GetSearchAnalytics(c *gin.Context) {
	// Parse date range
	from := time.Now().AddDate(0, 0, -30) // Default: last 30 days
	to := time.Now()

	if fromStr := c.Query("from"); fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = parsed
		}
	}

	if toStr := c.Query("to"); toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = parsed
		}
	}

	analytics, err := h.searchService.GetSearchAnalytics(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// ReindexTickets triggers reindexing of all tickets.
func (h *SearchHandlers) ReindexTickets(c *gin.Context) {
	// This would typically fetch tickets from the database
	// For now, we'll use a mock fetcher
	fetcher := func() ([]models.Ticket, error) {
		// TODO: Implement actual ticket fetching from database
		return []models.Ticket{}, nil
	}

	stats, err := h.searchService.ReindexAllTickets(c.Request.Context(), fetcher)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Reindexing completed",
		"stats":   stats,
	})
}

// IndexTicket indexes a single ticket.
func (h *SearchHandlers) IndexTicket(c *gin.Context) {
	var ticket models.Ticket

	if err := c.ShouldBindJSON(&ticket); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.searchService.IndexTicket(c.Request.Context(), &ticket); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ticket indexed successfully"})
}

// UpdateTicketIndex updates a ticket in the search index.
func (h *SearchHandlers) UpdateTicketIndex(c *gin.Context) {
	var ticket models.Ticket

	if err := c.ShouldBindJSON(&ticket); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.searchService.UpdateTicketInIndex(c.Request.Context(), &ticket); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ticket index updated successfully"})
}

// DeleteTicketFromIndex removes a ticket from the search index.
func (h *SearchHandlers) DeleteTicketFromIndex(c *gin.Context) {
	ticketNumber := c.Param("ticket_number")
	if ticketNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticket_number required"})
		return
	}

	if err := h.searchService.DeleteTicketFromIndex(c.Request.Context(), ticketNumber); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ticket removed from index"})
}
