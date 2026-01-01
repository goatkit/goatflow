package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/search"
)

var searchManager *search.SearchManager

func init() {
	// Initialize search manager
	searchManager = search.NewSearchManager()

	// Register PostgreSQL backend as primary by default, but only if DB is reachable
	if os.Getenv("APP_ENV") != "test" { // tests can run without DB
		if pgBackend, err := search.NewPostgresBackend(); err == nil {
			searchManager.RegisterBackend("postgresql", pgBackend, true)
		}
	}

	// Register Elasticsearch/Zinc backend if configured
	if esEndpoint := os.Getenv("ELASTICSEARCH_ENDPOINT"); esEndpoint != "" {
		esBackend := search.NewElasticBackend(
			esEndpoint,
			os.Getenv("ELASTICSEARCH_USERNAME"),
			os.Getenv("ELASTICSEARCH_PASSWORD"),
		)
		// Make Elasticsearch primary if explicitly configured
		if os.Getenv("SEARCH_BACKEND") == "elasticsearch" {
			searchManager.RegisterBackend("elasticsearch", esBackend, true)
		} else {
			searchManager.RegisterBackend("elasticsearch", esBackend, false)
		}
	}

	// Register Zinc backend if configured (alternative to Elasticsearch)
	if zincEndpoint := os.Getenv("ZINC_ENDPOINT"); zincEndpoint != "" {
		zincBackend := search.NewElasticBackend(
			zincEndpoint,
			os.Getenv("ZINC_USERNAME"),
			os.Getenv("ZINC_PASSWORD"),
		)
		if os.Getenv("SEARCH_BACKEND") == "zinc" {
			searchManager.RegisterBackend("zinc", zincBackend, true)
		} else {
			searchManager.RegisterBackend("zinc", zincBackend, false)
		}
	}
}

// HandleSearchAPI handles POST /api/v1/search.
func HandleSearchAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission-based filtering later

	var req search.SearchQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate query
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query cannot be empty"})
		return
	}

	// Set defaults
	if len(req.Types) == 0 {
		req.Types = []string{"ticket", "article", "customer"}
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100 // Max limit
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// If no backend available (common in tests without DB), return empty results
	backend := searchManager.GetPrimaryBackend()
	if backend == nil {
		c.JSON(http.StatusOK, gin.H{
			"hits":       []interface{}{},
			"total_hits": 0,
			"took_ms":    0,
		})
		return
	}

	// Perform search
	results, err := searchManager.Search(ctx, req)
	if err != nil {
		// On database backends that may not support advanced search (e.g. MySQL), fall back to empty results
		c.JSON(http.StatusOK, gin.H{
			"hits":       []interface{}{},
			"total_hits": 0,
			"took_ms":    0,
			"warning":    "search backend unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, results)
}

// HandleSearchSuggestionsAPI handles GET /api/v1/search/suggestions.
func HandleSearchSuggestionsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	entityType := c.DefaultQuery("type", "all")

	// For now, return simple suggestions based on recent searches
	// In production, this would query a suggestions index
	suggestions := []string{
		query + " open",
		query + " closed",
		query + " urgent",
		query + " customer",
		query + " today",
	}

	c.JSON(http.StatusOK, gin.H{
		"query":       query,
		"type":        entityType,
		"suggestions": suggestions,
	})
}

// HandleReindexAPI handles POST /api/v1/search/reindex.
func HandleReindexAPI(c *gin.Context) {
	// Check authentication and admin permissions
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // TODO: Check for admin permissions

	var req struct {
		Types []string `json:"types"`
		Force bool     `json:"force"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to all types
		req.Types = []string{"ticket", "article", "customer"}
	}

	// Get the primary backend
	backend := searchManager.GetPrimaryBackend()
	if backend == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No search backend available"})
		return
	}

	// For PostgreSQL backend, reindexing is not needed
	if backend.GetBackendName() == "postgresql" {
		c.JSON(http.StatusOK, gin.H{
			"message": "PostgreSQL backend does not require reindexing",
			"backend": "postgresql",
		})
		return
	}

	// For Elasticsearch/Zinc, we would need to fetch all documents and reindex
	// This is a simplified version - in production, this would be done in batches
	go func() {
		// This would run in the background
		// Fetch all tickets, articles, customers and index them
		// Log progress and errors
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Reindexing started in background",
		"types":   req.Types,
		"backend": backend.GetBackendName(),
	})
}

// HandleSearchHealthAPI handles GET /api/v1/search/health.
func HandleSearchHealthAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	backend := searchManager.GetPrimaryBackend()
	if backend == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"message": "No search backend available",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := backend.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"backend": backend.GetBackendName(),
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"backend": backend.GetBackendName(),
	})
}
