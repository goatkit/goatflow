package storage

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// Config represents storage configuration.
type Config struct {
	// Backend type: "DB" or "FS"
	Backend string

	// Filesystem backend settings
	FSBasePath string

	// Mixed mode settings
	CheckAllBackends bool

	// Migration settings
	MigrationBatchSize int
	MigrationSleepMs   int

	// Database connection (for both backends)
	DB *sql.DB
}

// NewConfigFromEnv creates configuration from environment variables.
func NewConfigFromEnv(db *sql.DB) *Config {
	config := &Config{
		Backend:            getEnv("ARTICLE_STORAGE_BACKEND", "DB"),
		FSBasePath:         getEnv("ARTICLE_STORAGE_FS_PATH", "/opt/goatflow/var/article"),
		CheckAllBackends:   getEnvBool("ARTICLE_STORAGE_CHECK_ALL", false),
		MigrationBatchSize: getEnvInt("ARTICLE_STORAGE_MIGRATION_BATCH", 100),
		MigrationSleepMs:   getEnvInt("ARTICLE_STORAGE_MIGRATION_SLEEP_MS", 10),
		DB:                 db,
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("Invalid storage configuration: %v", err))
	}

	return config
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate backend type
	backend := strings.ToUpper(c.Backend)
	if backend != "DB" && backend != "FS" {
		return fmt.Errorf("invalid backend type: %s (must be DB or FS)", c.Backend)
	}
	c.Backend = backend

	// Validate filesystem path if using FS backend
	if c.Backend == "FS" || c.CheckAllBackends {
		if c.FSBasePath == "" {
			return fmt.Errorf("filesystem base path is required for FS backend")
		}

		// Ensure path exists or can be created
		if err := os.MkdirAll(c.FSBasePath, 0750); err != nil {
			return fmt.Errorf("cannot create filesystem base path: %w", err)
		}

		// Check write permissions
		testFile := fmt.Sprintf("%s/.write_test", c.FSBasePath)
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			return fmt.Errorf("filesystem base path is not writable: %w", err)
		}
		os.Remove(testFile)
	}

	// Validate database connection
	if c.DB == nil {
		return fmt.Errorf("database connection is required")
	}

	return nil
}

// CreateBackend creates a storage backend based on configuration.
func (c *Config) CreateBackend() (Backend, error) {
	// Create primary backend
	var primary Backend
	var err error

	switch c.Backend {
	case "DB":
		primary = NewDatabaseBackend(c.DB)
	case "FS":
		primary, err = NewFilesystemBackend(c.FSBasePath, c.DB)
		if err != nil {
			return nil, fmt.Errorf("failed to create filesystem backend: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown backend type: %s", c.Backend)
	}

	// If mixed mode is enabled, create fallback backends
	if c.CheckAllBackends {
		var fallbacks []Backend

		// Add the opposite backend as fallback
		if c.Backend == "DB" {
			fs, err := NewFilesystemBackend(c.FSBasePath, c.DB)
			if err == nil {
				fallbacks = append(fallbacks, fs)
			}
		} else {
			fallbacks = append(fallbacks, NewDatabaseBackend(c.DB))
		}

		if len(fallbacks) > 0 {
			return NewMixedModeBackend(primary, fallbacks...), nil
		}
	}

	return primary, nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	value = strings.ToLower(value)
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	var intValue int
	if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
		return intValue
	}

	return defaultValue
}
