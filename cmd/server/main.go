package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
)

func main() {
	// Set Gin mode
	if os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create a simple router with HTMX routes (includes health check)
	r := api.NewSimpleRouter()

	// Start server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting GOTRS HTMX server on port %s\n", port)
	fmt.Println("Available routes:")
	fmt.Println("  GET  /          -> Redirect to /login")
	fmt.Println("  GET  /login     -> Login page") 
	fmt.Println("  GET  /dashboard -> Dashboard (demo)")
	fmt.Println("  GET  /tickets   -> Tickets list (demo)")
	fmt.Println("  POST /api/auth/login -> HTMX login")
	fmt.Println("")
	fmt.Println("LDAP API routes:")
	fmt.Println("  POST /api/v1/ldap/configure -> Configure LDAP")
	fmt.Println("  POST /api/v1/ldap/test -> Test LDAP connection")
	fmt.Println("  POST /api/v1/ldap/authenticate -> Authenticate user")
	fmt.Println("  GET  /api/v1/ldap/users/:username -> Get user info")
	fmt.Println("  POST /api/v1/ldap/sync/users -> Sync users")
	fmt.Println("  GET  /api/v1/ldap/config -> Get LDAP config")
	
	log.Fatal(r.Run(":" + port))
}