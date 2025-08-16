package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	_ "github.com/lib/pq"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	
	if err := config.Load(configPath); err != nil {
		log.Printf("Failed to load configuration from file: %v", err)
		// Continue with environment variables
	}
	
	cfg := config.Get()
	
	// If config is nil, use defaults
	if cfg == nil {
		// Use environment variable for Gin mode
		if os.Getenv("APP_ENV") == "production" {
			gin.SetMode(gin.ReleaseMode)
		} else {
			gin.SetMode(gin.DebugMode)
		}
	} else {
		// Set Gin mode from config
		if cfg.App.Env == "production" {
			gin.SetMode(gin.ReleaseMode)
		} else {
			gin.SetMode(gin.DebugMode)
		}
	}

	// Connect to database
	var dsn string
	if cfg != nil {
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
			cfg.Database.Password, cfg.Database.Name)
	} else {
		// Use environment variables directly
		dbHost := os.Getenv("DB_HOST")
		if dbHost == "" {
			dbHost = "postgres"
		}
		dbPort := os.Getenv("DB_PORT")
		if dbPort == "" {
			dbPort = "5432"
		}
		dbUser := os.Getenv("DB_USER")
		if dbUser == "" {
			dbUser = "gotrs"
		}
		dbPass := os.Getenv("DB_PASSWORD")
		if dbPass == "" {
			dbPass = "gotrs_password"
		}
		dbName := os.Getenv("DB_NAME")
		if dbName == "" {
			dbName = "gotrs"
		}
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPass, dbName)
	}
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Successfully connected to database")

	// Get JWT secret from environment or config
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-secret-change-in-production" // TODO: Load from config
		log.Println("WARNING: Using default JWT secret. Change this in production!")
	}

	// Email configuration (using Mailhog in development)
	emailConfig := api.EmailConfig{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),
		UseTLS:       os.Getenv("SMTP_USE_TLS") == "true",
	}

	// Default to Mailhog settings for development
	if emailConfig.SMTPHost == "" {
		emailConfig.SMTPHost = "mailhog"
		emailConfig.SMTPPort = "1025"
		emailConfig.SMTPFrom = "noreply@gotrs.local"
		emailConfig.UseTLS = false
		log.Println("Using Mailhog for email (development mode)")
	}

	// Create API router with all routes
	router := api.NewRouter(db, jwtSecret, emailConfig)
	router.SetupRoutes()
	
	// Get the Gin engine
	r := router.GetEngine()
	
	// Add OpenAPI contract validation middleware
	r.Use(middleware.LoadOpenAPIMiddleware())

	// Start server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080" // Default port
	}

	fmt.Printf("Starting GOTRS server on port %s\n", port)
	log.Fatal(r.Run(":" + port))
}