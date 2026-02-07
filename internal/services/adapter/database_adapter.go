// Package adapter provides service adapters for database operations.
package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/goatkit/goatflow/internal/services/database"
	"github.com/goatkit/goatflow/internal/services/registry"
)

var (
	globalRegistry *registry.ServiceRegistry
	globalDB       database.DatabaseService
	once           sync.Once
	initErr        error
)

// InitializeServiceRegistry initializes the global service registry.
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
// Deprecated: kept for compatibility; callers should invoke AutoConfigureDatabase directly.
// func shouldAutoConfig() bool {
//     return os.Getenv("DB_HOST") != "" || os.Getenv("DATABASE_URL") != ""
// }

// AutoConfigureDatabase configures database from environment variables.
func AutoConfigureDatabase() error {
	// In tests with no DB configured, treat as no-op (allow DB-less tests)
	if os.Getenv("APP_ENV") == "test" && os.Getenv("TEST_DB_HOST") == "" && os.Getenv("TEST_DB_NAME") == "" && os.Getenv("DATABASE_URL") == "" {
		return nil
	}
	// Initialize registry if not already done
	reg, err := InitializeServiceRegistry()
	if err != nil {
		return err
	}

	// Build configuration from environment
	config := buildDatabaseConfig()

	// Register the database service
	service, err := reg.GetService(config.ID)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}

		if err := reg.RegisterService(config); err != nil {
			if !strings.Contains(err.Error(), "already registered") {
				return fmt.Errorf("failed to register database service: %w", err)
			}
		}

		service, err = reg.GetService(config.ID)
		if err != nil {
			return err
		}
	} else {
		if updateErr := service.UpdateConfig(config); updateErr != nil {
			return fmt.Errorf("failed to update database service: %w", updateErr)
		}
	}

	dbService, ok := service.(database.DatabaseService)
	if !ok {
		return fmt.Errorf("service is not a database service")
	}

	// Ensure the underlying connection is established before returning success.
	if err := ensureDatabaseConnection(dbService); err != nil {
		return err
	}

	globalDB = dbService

	binding := &registry.ServiceBinding{
		ID:        "default-db-binding",
		AppID:     "goatflow",
		ServiceID: config.ID,
		Name:      "Primary Database",
		Purpose:   "primary",
		Priority:  100,
	}

	if err := reg.CreateBinding(binding); err != nil {
		if !strings.Contains(err.Error(), "already") {
			return err
		}
	}

	return nil
}

// ensureDatabaseConnection guarantees the provided service has an active
// connection that responds to Ping. Retries once after forcing a reconnect to
// avoid lingering handles from an earlier failure.
func ensureDatabaseConnection(dbSvc database.DatabaseService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	pingErr := dbSvc.Ping(ctx)
	cancel()

	if pingErr == nil && dbSvc.GetDB() != nil {
		return nil
	}

	// Attempt a forced reconnect with a longer timeout.
	reconnectCtx, reconnectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer reconnectCancel()

	if err := dbSvc.Disconnect(reconnectCtx); err != nil {
		// Ignore disconnect errors but surface them in case reconnect fails too.
	}

	if err := dbSvc.Connect(reconnectCtx); err != nil {
		return fmt.Errorf("failed to connect to database service: %w", err)
	}

	// Final ping confirmation.
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer verifyCancel()

	if err := dbSvc.Ping(verifyCtx); err != nil {
		return fmt.Errorf("database service ping failed after reconnect: %w", err)
	}

	if dbSvc.GetDB() == nil {
		return fmt.Errorf("database service connected but returned nil *sql.DB")
	}

	return nil
}

// buildDatabaseConfig builds database configuration from environment.
func buildDatabaseConfig() *registry.ServiceConfig {
	// In test mode, prefer TEST_ prefixed environment variables
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if driver == "" {
		driver = "mysql"
	}
	driver = strings.ToLower(driver)

	provider := registry.ProviderMySQL
	switch driver {
	case "postgres", "postgresql":
		provider = registry.ProviderPostgres
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
		// Use individual environment variables (prefer TEST_ prefixed in test mode)
		config.Host = getEnvOrDefault("TEST_DB_HOST", getEnvOrDefault("DB_HOST", "localhost"))
		defaultPort := 3306
		if provider == registry.ProviderPostgres {
			defaultPort = 5432
		}
		config.Port = getEnvAsIntOrDefault("TEST_DB_PORT", getEnvAsIntOrDefault("DB_PORT", defaultPort))
		config.Username = getEnvOrDefault("TEST_DB_USER", getEnvOrDefault("DB_USER", "goatflow_user"))
		config.Password = getEnvOrDefault("TEST_DB_PASSWORD", getEnvOrDefault("DB_PASSWORD", "goatflow_password"))
		config.Database = getEnvOrDefault("TEST_DB_NAME", getEnvOrDefault("DB_NAME", "goatflow"))

		// SSL mode
		if sslMode := os.Getenv("TEST_DB_SSLMODE"); sslMode != "" {
			config.Options["sslmode"] = sslMode
		} else if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
			config.Options["sslmode"] = sslMode
		}

		// MySQL charset for Unicode support (only when explicitly enabled)
		if provider == registry.ProviderMySQL {
			unicodeSupport := os.Getenv("UNICODE_SUPPORT")
			if unicodeSupport == "true" || unicodeSupport == "1" || unicodeSupport == "enabled" {
				config.Options["charset"] = "utf8mb4"
			}
			// Default: no charset specified, uses database default (utf8mb3 for OTRS compatibility)
		}
	}

	// Connection pool settings
	config.MaxConns = getEnvAsIntOrDefault("DB_MAX_CONNS", 25)
	config.MinConns = getEnvAsIntOrDefault("DB_MIN_CONNS", 5)

	return config
}

// GetDatabase returns the primary database service.
func GetDatabase() (database.DatabaseService, error) {
	if globalDB == nil {
		// Try to initialize; in tests without DB, return explicit error quickly
		if err := AutoConfigureDatabase(); err != nil {
			if os.Getenv("APP_ENV") == "test" {
				return nil, fmt.Errorf("database not initialized in test: %w", err)
			}
			return nil, fmt.Errorf("database not initialized: %w", err)
		}
	}

	return globalDB, nil
}

// GetDatabaseForApp returns a database service bound to a specific application.
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

// GetDB returns a *sql.DB for compatibility with existing code.
func GetDB() (*sql.DB, error) {
	// Quick check if already initialized
	if globalDB != nil {
		db := globalDB.GetDB()
		if db == nil {
			if os.Getenv("APP_ENV") == "test" {
				return nil, fmt.Errorf("database not initialized in test: no db instance")
			}
			return nil, fmt.Errorf("database not initialized: no db instance")
		}
		// In tests, proactively verify connectivity with a short timeout
		if os.Getenv("APP_ENV") == "test" {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			if pingErr := db.PingContext(ctx); pingErr != nil {
				return nil, fmt.Errorf("database not initialized in test: %w", pingErr)
			}
		}
		return db, nil
	}

	// Try direct connection first (bypass service registry)
	if db := GetDirectDB(); db != nil {
		return db, nil
	}

	// Fallback to service registry
	dbService, err := GetDatabase()
	if err != nil {
		if os.Getenv("APP_ENV") == "test" {
			return nil, fmt.Errorf("database not initialized in test: %w", err)
		}
		return nil, err
	}

	if dbService == nil {
		if os.Getenv("APP_ENV") == "test" {
			return nil, fmt.Errorf("database not initialized in test: no service")
		}
		return nil, fmt.Errorf("database not initialized: no service")
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

// GetDirectDB creates a direct database connection using environment variables.
func GetDirectDB() *sql.DB {
	if os.Getenv("APP_ENV") == "test" {
		// Respect a very short timeout by ping, but don't attempt if no host
		if os.Getenv("DB_HOST") == "" && os.Getenv("DATABASE_URL") == "" {
			return nil
		}
	}
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
	password := getEnvOrDefault("DB_PASSWORD", "CHANGEME")
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

// RegisterDatabaseService registers a custom database service.
func RegisterDatabaseService(config *registry.ServiceConfig) error {
	reg, err := InitializeServiceRegistry()
	if err != nil {
		return err
	}

	return reg.RegisterService(config)
}

// MigrateDatabase migrates from one database to another.
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
