package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SearchResult represents a search result item.
type SearchResult struct {
	ID           int               `json:"id"`
	Subject      string            `json:"subject"`
	Body         string            `json:"body"`
	Status       string            `json:"status"`
	Priority     string            `json:"priority"`
	QueueID      int               `json:"queue_id"`
	QueueName    string            `json:"queue_name"`
	AssigneeID   int               `json:"assignee_id"`
	AssigneeName string            `json:"assignee_name"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Score        float64           `json:"score"`
	Highlights   map[string]string `json:"highlights,omitempty"`
}

// SavedSearch represents a user's saved search.
type SavedSearch struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Name      string    `json:"name"`
	Query     string    `json:"query"`
	Filters   string    `json:"filters"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchHistory represents a user's search history entry.
type SearchHistory struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Query     string    `json:"query"`
	Name      string    `json:"name,omitempty"`
	Results   int       `json:"results"`
	CreatedAt time.Time `json:"created_at"`
}

// Mock data for searches.
var savedSearches = map[int]*SavedSearch{
	1: {
		ID:        1,
		UserID:    1,
		Name:      "Critical Open",
		Query:     "status:open priority:critical",
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-24 * time.Hour),
	},
}

var searchHistory = []SearchHistory{}
var nextSavedSearchID = 2
var nextHistoryID = 1

// Mock tickets for searching.
var searchableTickets = []SearchResult{
	{
		ID:           1,
		Subject:      "Network issue with server",
		Body:         "The server is experiencing network connectivity problems",
		Status:       "open",
		Priority:     "high",
		QueueID:      1,
		QueueName:    "IT Support",
		AssigneeID:   1,
		AssigneeName: "John Doe",
		CreatedAt:    time.Now().Add(-2 * time.Hour),
		UpdatedAt:    time.Now().Add(-1 * time.Hour),
	},
	{
		ID:           2,
		Subject:      "Password reset request",
		Body:         "User needs urgent password reset for email account",
		Status:       "pending",
		Priority:     "medium",
		QueueID:      1,
		QueueName:    "IT Support",
		AssigneeID:   2,
		AssigneeName: "Jane Smith",
		CreatedAt:    time.Now().Add(-5 * time.Hour),
		UpdatedAt:    time.Now().Add(-3 * time.Hour),
	},
	{
		ID:           3,
		Subject:      "Email server error",
		Body:         "Critical server error preventing email delivery",
		Status:       "open",
		Priority:     "critical",
		QueueID:      2,
		QueueName:    "Email Admin",
		AssigneeID:   1,
		AssigneeName: "John Doe",
		CreatedAt:    time.Now().Add(-1 * time.Hour),
		UpdatedAt:    time.Now().Add(-30 * time.Minute),
	},
}

// handleAdvancedTicketSearch handles advanced ticket search with filters.
func handleAdvancedTicketSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	// Parse filters
	status := c.Query("status")
	priority := c.Query("priority")
	queueStr := c.Query("queue")
	assigneeStr := c.Query("assignee")
	createdFrom := c.Query("created_from")
	createdTo := c.Query("created_to")

	// Pagination
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	}

	// Sorting
	sortBy := c.DefaultQuery("sort", "relevance")
	order := c.DefaultQuery("order", "desc")

	// Highlighting
	highlight := c.Query("highlight") == "true"

	// Filter results
	results := []SearchResult{}
	startTime := time.Now()

	for _, ticket := range searchableTickets {
		// Basic text search (simplified - in production use full-text search)
		if query != "*" {
			searchText := strings.ToLower(ticket.Subject + " " + ticket.Body)

			// Handle field-specific queries
			if strings.Contains(query, ":") {
				parts := strings.SplitN(query, ":", 2)
				field := strings.ToLower(parts[0])
				value := strings.Trim(parts[1], "\"")

				switch field {
				case "subject":
					if !strings.Contains(strings.ToLower(ticket.Subject), strings.ToLower(value)) {
						continue
					}
				case "body":
					if !strings.Contains(strings.ToLower(ticket.Body), strings.ToLower(value)) {
						continue
					}
				default:
					if !strings.Contains(searchText, strings.ToLower(query)) {
						continue
					}
				}
			} else {
				// Regular search
				if !strings.Contains(searchText, strings.ToLower(query)) {
					continue
				}
			}
		}

		// Apply filters
		if status != "" {
			statuses := strings.Split(status, ",")
			found := false
			for _, s := range statuses {
				if ticket.Status == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if priority != "" {
			priorities := strings.Split(priority, ",")
			found := false
			for _, p := range priorities {
				if ticket.Priority == p {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if queueStr != "" {
			queueID, err := strconv.Atoi(queueStr)
			if err != nil || ticket.QueueID != queueID {
				continue
			}
		}

		if assigneeStr != "" {
			assigneeID, err := strconv.Atoi(assigneeStr)
			if err != nil || ticket.AssigneeID != assigneeID {
				continue
			}
		}

		// Date range filter
		if createdFrom != "" {
			fromDate, err := time.Parse("2006-01-02", createdFrom)
			if err == nil && ticket.CreatedAt.Before(fromDate) {
				continue
			}
		}

		if createdTo != "" {
			toDate, err := time.Parse("2006-01-02", createdTo)
			if err == nil {
				toDate = toDate.Add(24 * time.Hour) // Include the entire day
				if ticket.CreatedAt.After(toDate) {
					continue
				}
			}
		}

		// Add highlighting if requested
		if highlight && query != "*" {
			ticket.Highlights = make(map[string]string)
			if strings.Contains(strings.ToLower(ticket.Subject), strings.ToLower(query)) {
				highlighted := strings.ReplaceAll(ticket.Subject, query, fmt.Sprintf("<mark>%s</mark>", query))
				ticket.Highlights["subject"] = highlighted
			}
			if strings.Contains(strings.ToLower(ticket.Body), strings.ToLower(query)) {
				// Show snippet with highlight
				idx := strings.Index(strings.ToLower(ticket.Body), strings.ToLower(query))
				start := idx - 20
				if start < 0 {
					start = 0
				}
				end := idx + len(query) + 20
				if end > len(ticket.Body) {
					end = len(ticket.Body)
				}
				snippet := ticket.Body[start:end]
				highlighted := strings.ReplaceAll(snippet, query, fmt.Sprintf("<mark>%s</mark>", query))
				ticket.Highlights["body"] = "..." + highlighted + "..."
			}
		}

		// Calculate relevance score (simplified)
		ticket.Score = 1.0
		if strings.Contains(strings.ToLower(ticket.Subject), strings.ToLower(query)) {
			ticket.Score += 2.0
		}
		if strings.Contains(strings.ToLower(ticket.Body), strings.ToLower(query)) {
			ticket.Score += 1.0
		}

		results = append(results, ticket)
	}

	// Sort results
	if sortBy == "created_at" && order == "desc" && len(results) > 1 {
		// Simple bubble sort for demo (use proper sorting in production)
		for i := 0; i < len(results)-1; i++ {
			for j := 0; j < len(results)-i-1; j++ {
				if results[j].CreatedAt.Before(results[j+1].CreatedAt) {
					results[j], results[j+1] = results[j+1], results[j]
				}
			}
		}
	}

	// Apply pagination
	total := len(results)
	start := (page - 1) * limit
	end := start + limit
	if start > len(results) {
		results = []SearchResult{}
	} else {
		if end > len(results) {
			end = len(results)
		}
		results = results[start:end]
	}

	// Calculate search time
	took := time.Since(startTime).Milliseconds()

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"took_ms": took,
	})
}

// handleSearchSuggestions returns search suggestions based on partial query.
func handleSearchSuggestions(c *gin.Context) {
	query := c.Query("q")

	var suggestions []string

	// Only suggest for queries with 2+ characters
	if len(query) >= 2 {
		// Mock suggestions (in production, use actual search index)
		allSuggestions := []string{
			"password", "password reset", "password change",
			"email", "email server", "email delivery",
			"network", "network issue", "network error",
			"server", "server error", "server down",
			"login", "login failed", "login issue",
		}

		for _, suggestion := range allSuggestions {
			if strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(query)) {
				suggestions = append(suggestions, suggestion)
			}
		}

		// Limit to 10 suggestions
		if len(suggestions) > 10 {
			suggestions = suggestions[:10]
		}
	}

	if suggestions == nil {
		suggestions = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
	})
}

// handleSaveSearchHistory saves a search to user's history.
func handleSaveSearchHistory(c *gin.Context) {
	userID := getUserID(c)
	query := c.Query("q")
	name := c.Query("name")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	history := SearchHistory{
		ID:        nextHistoryID,
		UserID:    userID,
		Query:     query,
		Name:      name,
		Results:   10, // Mock result count
		CreatedAt: time.Now(),
	}

	searchHistory = append(searchHistory, history)
	nextHistoryID++

	c.JSON(http.StatusCreated, gin.H{
		"message": "Search saved to history",
		"id":      history.ID,
	})
}

// handleGetSearchHistory returns user's search history.
func handleGetSearchHistory(c *gin.Context) {
	userID := getUserID(c)

	var userHistory []SearchHistory
	for _, h := range searchHistory {
		if h.UserID == userID {
			userHistory = append(userHistory, h)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"history": userHistory,
	})
}

// handleDeleteSearchHistory removes a search from history.
func handleDeleteSearchHistory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	userID := getUserID(c)

	// Remove from history
	newHistory := make([]SearchHistory, 0, len(searchHistory))
	found := false
	for _, h := range searchHistory {
		if h.ID == id && h.UserID == userID {
			found = true
			continue
		}
		newHistory = append(newHistory, h)
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Search not found in history"})
		return
	}

	searchHistory = newHistory

	c.JSON(http.StatusOK, gin.H{
		"message": "Search removed from history",
	})
}

// handleCreateSavedSearch creates a new saved search.
func handleCreateSavedSearch(c *gin.Context) {
	userID := getUserID(c)
	name := c.Query("name")
	query := c.Query("q")

	if name == "" || query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and query are required"})
		return
	}

	search := &SavedSearch{
		ID:        nextSavedSearchID,
		UserID:    userID,
		Name:      name,
		Query:     query,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	savedSearches[nextSavedSearchID] = search
	nextSavedSearchID++

	c.JSON(http.StatusCreated, gin.H{
		"message": "Search saved successfully",
		"id":      search.ID,
	})
}

// handleGetSavedSearches returns user's saved searches.
func handleGetSavedSearches(c *gin.Context) {
	userID := getUserID(c)

	var userSearches []SavedSearch
	for _, s := range savedSearches {
		if s.UserID == userID {
			userSearches = append(userSearches, *s)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"searches": userSearches,
	})
}

// handleExecuteSavedSearch executes a saved search.
func handleExecuteSavedSearch(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	userID := getUserID(c)

	search, exists := savedSearches[id]
	if !exists || search.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
		return
	}

	// Execute the search (simplified - reuse the search logic)
	var results []SearchResult
	for _, ticket := range searchableTickets {
		// Simple match for demo
		if strings.Contains(search.Query, "open") && ticket.Status == "open" {
			results = append(results, ticket)
			continue
		}
		if strings.Contains(search.Query, "critical") && ticket.Priority == "critical" {
			results = append(results, ticket)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
		"query":   search.Query,
	})
}

// handleUpdateSavedSearch updates a saved search.
func handleUpdateSavedSearch(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	userID := getUserID(c)
	name := c.Query("name")
	query := c.Query("q")

	search, exists := savedSearches[id]
	if !exists || search.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
		return
	}

	if name != "" {
		search.Name = name
	}
	if query != "" {
		search.Query = query
	}
	search.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"message": "Saved search updated",
	})
}

// handleDeleteSavedSearch deletes a saved search.
func handleDeleteSavedSearch(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	userID := getUserID(c)

	search, exists := savedSearches[id]
	if !exists || search.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Saved search not found"})
		return
	}

	delete(savedSearches, id)

	c.JSON(http.StatusOK, gin.H{
		"message": "Saved search deleted",
	})
}

// handleExportSearchResults exports search results in various formats.
func handleExportSearchResults(c *gin.Context) {
	query := c.Query("q")
	format := c.Query("format")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	// Get search results (simplified)
	var results []SearchResult
	results = append(results, searchableTickets...)

	timestamp := time.Now().Format("20060102-150405")

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"tickets-%s.csv\"", timestamp))

		writer := csv.NewWriter(c.Writer)
		// Write headers
		_ = writer.Write([]string{"ID", "Subject", "Status", "Priority", "Queue", "Assignee", "Created"}) //nolint:errcheck // Best effort CSV write

		// Write data
		for _, ticket := range results {
			_ = writer.Write([]string{ //nolint:errcheck // Best effort CSV write
				strconv.Itoa(ticket.ID),
				ticket.Subject,
				ticket.Status,
				ticket.Priority,
				ticket.QueueName,
				ticket.AssigneeName,
				ticket.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		writer.Flush()

	case "json":
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"tickets-%s.json\"", timestamp))

		c.JSON(http.StatusOK, gin.H{
			"tickets":     results,
			"exported_at": time.Now(),
			"total":       len(results),
		})

	case "xlsx":
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"tickets-%s.xlsx\"", timestamp))

		// For demo, just send empty Excel file marker
		// In production, use a library like excelize to generate actual Excel file
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("Excel file content"))

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid export format. Supported: csv, json, xlsx"})
	}
}
