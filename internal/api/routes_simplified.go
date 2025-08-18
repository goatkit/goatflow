package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
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
		c.JSON(200, gin.H{
			"success": true,
			"message": "Endpoint /api/v1/users/me is under construction", 
			"data": gin.H{
				"id":    1,
				"email": "test-user@example.com",
				"name":  "Demo User",
				"role":  "Admin",
			},
		})
	})
	apiV1.GET("/queues", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"message": "Endpoint /api/v1/queues is under construction",
			"data":    []interface{}{},
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