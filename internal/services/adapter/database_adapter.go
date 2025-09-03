package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gotrs-io/gotrs-ce/internal/services/database"
	"github.com/gotrs-io/gotrs-ce/internal/services/registry"
)

var (
	globalRegistry *registry.ServiceRegistry
	globalDB       database.DatabaseService
	once           sync.Once
	initErr        error
)

// InitializeServiceRegistry initializes the global service registry
func InitializeServiceRegistry() (*registry.ServiceRegistry, error) {
	once.Do(func() {
		globalRegistry = registry.NewServiceRegistry()

		// Register database factory
		dbFactory := database.NewDatabaseFactory()

		// Register PostgreSQL provider
		dbFactory.RegisterProvider(registry.ProviderPostgres, database.NewPostgresService)

		// Register MySQL provider
		dbFactory.RegisterProvider(registry.ProviderMySQL, database.NewMySQLService)

		// Register the factory with the registry
		initErr = globalRegistry.RegisterFactory(registry.ServiceTypeDatabase, dbFactory)
		if initErr != nil {
			return
		}

		// Don't auto-configure here - let the caller do it explicitly
		// This prevents initialization loops
	})

	return globalRegistry, initErr
}

// shouldAutoConfig checks if we should auto-configure from environment
func shouldAutoConfig() bool {
	// Check if we have database environment variables
	return os.Getenv("DB_HOST") != "" || os.Getenv("DATABASE_URL") != ""
}

// AutoConfigureDatabase configures database from environment variables
func AutoConfigureDatabase() error {
	// Initialize registry if not already done
	reg, err := InitializeServiceRegistry()
	if err != nil {
		return err
	}

	// Build configuration from environment
	config := buildDatabaseConfig()

	// Register the database service
	if err := reg.RegisterService(config); err != nil {
		return fmt.Errorf("failed to register database service: %w", err)
	}

	// Get the registered service
	service, err := reg.GetService(config.ID)
	if err != nil {
		return err
	}

	// Cast to DatabaseService
	dbService, ok := service.(database.DatabaseService)
	if !ok {
		return fmt.Errorf("service is not a database service")
	}

	globalDB = dbService

	// Create default binding for the application
	binding := &registry.ServiceBinding{
		ID:        "default-db-binding",
		AppID:     "gotrs",
		ServiceID: config.ID,
		Name:      "Primary Database",
		Purpose:   "primary",
		Priority:  100,
	}

	return reg.CreateBinding(binding)
}

// buildDatabaseConfig builds database configuration from environment
func buildDatabaseConfig() *registry.ServiceConfig {
	// Determine database provider from DB_DRIVER
	driver := os.Getenv("DB_DRIVER")
	provider := registry.ProviderPostgres // default
	if driver == "mysql" || driver == "mariadb" {
		provider = registry.ProviderMySQL
	}

	config := &registry.ServiceConfig{
		ID:       "primary-db",
		Name:     "Primary Database",
		Type:     registry.ServiceTypeDatabase,
		Provider: provider,
		Options:  make(map[string]interface{}),
	}

	// Check for DATABASE_URL first
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// Parse DATABASE_URL
		// Format: mysql://user:password@host:port/database or postgres://...
		// This is simplified - use a proper URL parser in production
		config.Options["connection_url"] = dbURL
	} else {
		// Use individual environment variables
		config.Host = getEnvOrDefault("DB_HOST", "localhost")
		config.Port = getEnvAsIntOrDefault("DB_PORT", 5432)
		config.Username = getEnvOrDefault("DB_USER", "gotrs_user")
		config.Password = getEnvOrDefault("DB_PASSWORD", "gotrs_password")
		config.Database = getEnvOrDefault("DB_NAME", "gotrs")

		// SSL mode
		if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
			config.Options["sslmode"] = sslMode
		}
	}

	// Connection pool settings
	config.MaxConns = getEnvAsIntOrDefault("DB_MAX_CONNS", 25)
	config.MinConns = getEnvAsIntOrDefault("DB_MIN_CONNS", 5)

	return config
}

// GetDatabase returns the primary database service
func GetDatabase() (database.DatabaseService, error) {
	if globalDB == nil {
		// Try to initialize
		if err := AutoConfigureDatabase(); err != nil {
			return nil, fmt.Errorf("database not initialized: %w", err)
		}
	}

	return globalDB, nil
}

// GetDatabaseForApp returns a database service bound to a specific application
func GetDatabaseForApp(appID string, purpose string) (database.DatabaseService, error) {
	reg, err := InitializeServiceRegistry()
	if err != nil {
		return nil, err
	}

	service, err := reg.GetBoundService(appID, purpose)
	if err != nil {
		return nil, err
	}

	dbService, ok := service.(database.DatabaseService)
	if !ok {
		return nil, fmt.Errorf("bound service is not a database service")
	}

	return dbService, nil
}

// GetDB returns a *sql.DB for compatibility with existing code
func GetDB() (*sql.DB, error) {
	// Quick check if already initialized
	if globalDB != nil {
		return globalDB.GetDB(), nil
	}

	// Try direct connection first (bypass service registry)
	if db := GetDirectDB(); db != nil {
		return db, nil
	}

    // Fallback to service registry
    dbService, err := GetDatabase()
    if err != nil {
        return nil, err
    }

    db := dbService.GetDB()
    if db == nil {
        if os.Getenv("APP_ENV") == "test" {
            return nil, fmt.Errorf("database unreachable in test: no db instance")
        }
        return nil, fmt.Errorf("database not initialized: no db instance")
    }
    // In tests, proactively verify connectivity with a short timeout to avoid blocking queries
    if os.Getenv("APP_ENV") == "test" {
        ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
        defer cancel()
        if pingErr := db.PingContext(ctx); pingErr != nil {
            return nil, fmt.Errorf("database unreachable in test: %w", pingErr)
        }
    }

    return db, nil
}

// GetDirectDB creates a direct database connection using environment variables
func GetDirectDB() *sql.DB {
	// Check for DATABASE_URL first
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, err := sql.Open("mysql", dbURL)
		if err == nil {
			// Test the connection
			if err := db.Ping(); err == nil {
				return db
			}
			db.Close()
		}
	}

	// Use individual environment variables
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvAsIntOrDefault("DB_PORT", 3306)
	user := getEnvOrDefault("DB_USER", "otrs")
	password := getEnvOrDefault("DB_PASSWORD", "LetClaude.1n")
	database := getEnvOrDefault("DB_NAME", "otrs")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		user, password, host, port, database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil
	}

	return db
}

// RegisterDatabaseService registers a custom database service
func RegisterDatabaseService(config *registry.ServiceConfig) error {
	reg, err := InitializeServiceRegistry()
	if err != nil {
		return err
	}

	return reg.RegisterService(config)
}

// MigrateDatabase migrates from one database to another
func MigrateDatabase(fromServiceID, toServiceID string, strategy registry.MigrationStrategy) error {
	reg, err := InitializeServiceRegistry()
	if err != nil {
		return err
	}

	migration, err := reg.StartMigration(fromServiceID, toServiceID, strategy)
	if err != nil {
		return err
	}

	// Wait for migration to complete (simplified)
	ctx := context.Background()
	for {
		m, err := reg.GetMigration(migration.ID)
		if err != nil {
			return err
		}

		if m.Status == "completed" {
			return nil
		}

		if m.Status == "failed" {
			return fmt.Errorf("migration failed: %s", m.Error)
		}

		// Wait and check again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			continue
		}
	}
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
