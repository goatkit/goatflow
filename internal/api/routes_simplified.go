package api

import (
	"os"
	
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/config"
)

// Simplified router for HTMX demo
func NewSimpleRouter() *gin.Engine {
	// Create router without default middleware
	r := gin.New()
	
	// Add logging and recovery middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	
	// Add i18n middleware for language detection
	i18nMiddleware := middleware.NewI18nMiddleware()
	r.Use(i18nMiddleware.Handle())
	
	// Initialize dashboard manager
	config.InitializeDashboardManager("./config")
	
	// Setup HTMX routes with dynamic template loading
	SetupHTMXRoutes(r)
	
	// Setup LDAP API routes
	SetupLDAPRoutes(r)
	
	// Setup i18n API routes
	i18nHandlers := NewI18nHandlers()
	apiV1 := r.Group("/api/v1")
	i18nHandlers.RegisterRoutes(apiV1)
	
	// Add v1 API stub endpoints (under construction)
	// These are expected by the SDK but not yet implemented
	apiV1.GET("/tickets", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"message": "Endpoint /api/v1/tickets is under construction",
			"data":    []interface{}{},
		})
	})
	apiV1.GET("/users/me", func(c *gin.Context) {
		demoEmail := os.Getenv("DEMO_USER_EMAIL")
		if demoEmail == "" {
			demoEmail = "test-user@example.com"
		}
		c.JSON(200, gin.H{
			"success": true,
			"message": "Endpoint /api/v1/users/me is under construction", 
			"data": gin.H{
				"id":    1,
				"email": demoEmail,
				"name":  "Demo User",
				"role":  "Admin",
			},
		})
	})
	apiV1.GET("/queues", func(c *gin.Context) {
		queueRepo := GetQueueRepository()
		if queueRepo == nil {
			c.JSON(500, gin.H{
				"success": false,
				"error":   "Queue repository not initialized",
			})
			return
		}
		
		queues, err := queueRepo.List()
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		
		// Convert to simpler format for API response
		queueList := make([]gin.H, 0, len(queues))
		for _, q := range queues {
			queueList = append(queueList, gin.H{
				"id":   q.ID,
				"name": q.Name,
			})
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    queueList,
		})
	})
	apiV1.GET("/priorities", func(c *gin.Context) {
		priorityRepo := GetPriorityRepository()
		if priorityRepo == nil {
			c.JSON(500, gin.H{
				"success": false,
				"error":   "Priority repository not initialized",
			})
			return
		}
		
		priorities, err := priorityRepo.List()
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		
		// Convert to simpler format for API response
		priorityList := make([]gin.H, 0, len(priorities))
		for _, p := range priorities {
			priorityList = append(priorityList, gin.H{
				"id":   p.ID,
				"name": p.Name,
			})
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    priorityList,
		})
	})
	apiV1.GET("/search", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"message": "Endpoint /api/v1/search is under construction",
			"data": gin.H{
				"results": []interface{}{},
				"total":   0,
			},
		})
	})
	
	return r
}