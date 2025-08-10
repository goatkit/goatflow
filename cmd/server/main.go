package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

func main() {
	// Set Gin mode
	if os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	r := gin.Default()

	// Add OpenAPI contract validation middleware
	r.Use(middleware.LoadOpenAPIMiddleware())

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"service": "gotrs-backend",
		})
	})

	// API routes
	api := r.Group("/api/v1")
	{
		api.GET("/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"message": "GOTRS API is running",
					"version": "0.1.0",
				},
			})
		})
	}

	// Start server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting GOTRS server on port %s\n", port)
	log.Fatal(r.Run(":" + port))
}