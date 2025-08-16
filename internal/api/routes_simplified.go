package api

import (
	"github.com/gin-gonic/gin"
)

// Simplified router for HTMX demo
func NewSimpleRouter() *gin.Engine {
	// Create router without default middleware
	r := gin.New()
	
	// Add logging and recovery middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	
	// Setup HTMX routes with dynamic template loading
	SetupHTMXRoutes(r)
	
	return r
}