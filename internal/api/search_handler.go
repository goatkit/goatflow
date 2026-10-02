package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/search"
)

// External search backend selected by SEARCH_BACKEND (nil: database backend).
// externalSearchErr holds a configuration error; searches then fail instead of
// silently falling back to another backend.
var (
	externalSearch    *search.ExternalBackend
	externalSearchErr error
)

func init() {
	cfg, err := search.ConfigFromEnv()
	if err != nil {
		externalSearchErr = err
		log.Printf("search: %v", err)
		return
	}
	if cfg != nil {
		externalSearch = search.NewExternalBackend(*cfg)
	}
}

// primarySearchBackend returns the configured external backend (Zinc or
// Elasticsearch), or a database backend bound to the current connection.
func primarySearchBackend() (search.SearchBackend, error) {
	if externalSearchErr != nil {
		return nil, externalSearchErr
	}
	if externalSearch != nil {
		return externalSearch, nil
	}
	return search.NewDatabaseBackend()
}

// HandleSearchAPI handles POST /api/v1/search.
//
//	@Summary		Search tickets
//	@Description	Full-text search across tickets, articles and customers. Ticket and article hits are limited to queues the caller can read.
//	@Tags			Search
//	@Accept			json
//	@Produce		json
//	@Param			query	body		object	true	"Search query"
//	@Success		200		{object}	map[string]interface{}	"Search results"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Failure		503		{object}	map[string]interface{}	"Search backend unavailable or index not built"
//	@Security		BearerAuth
//	@Router			/search [post]
func HandleSearchAPI(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req search.SearchQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query cannot be empty"})
		return
	}
	if len(req.Types) == 0 {
		req.Types = []string{"ticket", "article", "customer"}
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	// Tickets and articles only from queues the agent can read (admins: all).
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return
	}
	scope, ok := resolveTicketReadScope(c, db, false)
	if !ok {
		return
	}
	if !scope.queues.all {
		req.RestrictQueues = true
		req.QueueIDs = scope.queues.queueIDs
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	backend, err := primarySearchBackend()
	if err != nil {
		log.Printf("HandleSearchAPI: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Search backend is misconfigured: " + err.Error()})
		return
	}

	results, err := backend.Search(ctx, req)
	if err != nil {
		status, body := searchErrorResponse(backend.GetBackendName(), err)
		log.Printf("HandleSearchAPI: %s search failed: %v", backend.GetBackendName(), err)
		c.JSON(status, body)
		return
	}
	c.JSON(http.StatusOK, results)
}

// searchErrorResponse maps a backend error to the HTTP answer.
func searchErrorResponse(backend string, err error) (int, gin.H) {
	var svcErr *search.ServiceError
	switch {
	case errors.Is(err, search.ErrInvalidQuery):
		return http.StatusBadRequest, gin.H{"error": err.Error()}
	case errors.Is(err, search.ErrIndexNotReady):
		return http.StatusServiceUnavailable, gin.H{"backend": backend,
			"error": "The " + backend + " search index has not been built yet. It is built automatically by the runner; an administrator can also start a reindex."}
	case errors.As(err, &svcErr):
		return http.StatusServiceUnavailable, gin.H{"backend": backend,
			"error": "Search backend " + backend + " is unavailable: " + svcErr.Reason}
	}
	return http.StatusServiceUnavailable, gin.H{"backend": backend, "error": "Search backend unavailable"}
}

// HandleReindexAPI handles POST /api/v1/search/reindex.
//
//	@Summary		Reindex search
//	@Description	Start a full rebuild of the Zinc or Elasticsearch index in the background (admin only). Progress is reported by GET /search/health. The database backend needs no index.
//	@Tags			Search
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Database backend: nothing to do"
//	@Success		202	{object}	map[string]interface{}	"Reindex started"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		403	{object}	map[string]interface{}	"Admin access required"
//	@Failure		409	{object}	map[string]interface{}	"A reindex is already running"
//	@Failure		503	{object}	map[string]interface{}	"Search backend misconfigured"
//	@Security		BearerAuth
//	@Router			/search/reindex [post]
func HandleReindexAPI(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !isAdminCaller(c) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin access required"})
		return
	}
	if externalSearchErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Search backend is misconfigured: " + externalSearchErr.Error()})
		return
	}
	if externalSearch == nil {
		// The database backend searches the live tables.
		c.JSON(http.StatusOK, gin.H{
			"message": "Database backend does not require reindexing",
			"backend": "database",
		})
		return
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return
	}
	if !externalSearch.StartRebuild(db) {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "A reindex is already running",
			"backend": externalSearch.GetBackendName(),
			"reindex": externalSearch.RebuildStatus(),
		})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"message": "Reindex started",
		"backend": externalSearch.GetBackendName(),
		"reindex": externalSearch.RebuildStatus(),
	})
}

// HandleSearchHealthAPI handles GET /api/v1/search/health.
//
//	@Summary		Search health
//	@Description	Report whether the search backend can serve searches; for Zinc and Elasticsearch also the index state, document counts and the latest reindex started by this server.
//	@Tags			Search
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Search health status"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		503	{object}	map[string]interface{}	"Backend unreachable, misconfigured or index not built"
//	@Security		BearerAuth
//	@Router			/search/health [get]
func HandleSearchHealthAPI(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	backend, err := primarySearchBackend()
	if err != nil {
		log.Printf("HandleSearchHealthAPI: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "Search backend is misconfigured: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if externalSearch == nil {
		if err := backend.HealthCheck(ctx); err != nil {
			log.Printf("HandleSearchHealthAPI: database: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "backend": "database",
				"error": "Database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "backend": "database"})
		return
	}

	name := externalSearch.GetBackendName()
	reindex := externalSearch.RebuildStatus()
	index, err := externalSearch.Status(ctx)
	if err != nil {
		log.Printf("HandleSearchHealthAPI: %s: %v", name, err)
		_, body := searchErrorResponse(name, err)
		body["status"] = "unhealthy"
		body["reindex"] = reindex
		c.JSON(http.StatusServiceUnavailable, body)
		return
	}
	if !index.Ready {
		_, body := searchErrorResponse(name, search.ErrIndexNotReady)
		body["status"] = "indexing"
		body["index"] = index
		body["reindex"] = reindex
		c.JSON(http.StatusServiceUnavailable, body)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"backend": name,
		"index":   index,
		"reindex": reindex,
	})
}
