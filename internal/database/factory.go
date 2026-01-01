package database

import (
	"fmt"
	"os"
	"strings"
)

// DatabaseFactory implements IDatabaseFactory.
type DatabaseFactory struct{}

// NewDatabaseFactory creates a new database factory instance.
func NewDatabaseFactory() IDatabaseFactory {
	return &DatabaseFactory{}
}

// Create creates a database instance based on the configuration.
func (f *DatabaseFactory) Create(config DatabaseConfig) (IDatabase, error) {
	if err := f.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	switch config.Type {
	case PostgreSQL:
		return NewPostgreSQLDatabase(config), nil
	case MySQL:
		return NewMySQLDatabase(config), nil
	case Oracle:
		return NewOracleDatabase(config), nil
	case SQLServer:
		return NewSQLServerDatabase(config), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

// GetSupportedTypes returns the list of supported database types.
func (f *DatabaseFactory) GetSupportedTypes() []DatabaseType {
	return []DatabaseType{PostgreSQL, MySQL, Oracle, SQLServer}
}

// ValidateConfig validates the database configuration.
func (f *DatabaseFactory) ValidateConfig(config DatabaseConfig) error {
	if config.Type == "" {
		return fmt.Errorf("database type is required")
	}

	if config.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if config.Port == "" {
		return fmt.Errorf("database port is required")
	}

	if config.Database == "" {
		return fmt.Errorf("database name is required")
	}

	if config.Username == "" {
		return fmt.Errorf("database username is required")
	}

	// Validate database type is supported
	supportedTypes := f.GetSupportedTypes()
	found := false
	for _, supportedType := range supportedTypes {
		if config.Type == supportedType {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("unsupported database type: %s, supported types: %s",
			config.Type, strings.Join(typeStrings(supportedTypes), ", "))
	}

	// Validate connection pool settings
	if config.MaxOpenConns < 0 {
		return fmt.Errorf("max_open_conns cannot be negative")
	}

	if config.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns cannot be negative")
	}

	if config.MaxIdleConns > config.MaxOpenConns && config.MaxOpenConns > 0 {
		return fmt.Errorf("max_idle_conns cannot be greater than max_open_conns")
	}

	return nil
}

// GetDatabaseFeatures returns the features supported by a database type.
func GetDatabaseFeatures(dbType DatabaseType) DatabaseFeatures {
	switch dbType {
	case PostgreSQL:
		return DatabaseFeatures{
			SupportsReturning:       true,
			SupportsUpsert:          true,
			SupportsJSONColumn:      true,
			SupportsArrayColumn:     true,
			SupportsWindowFunctions: true,
			SupportsCTE:             true,
			MaxIdentifierLength:     63,
			MaxIndexNameLength:      63,
		}
	case MySQL:
		return DatabaseFeatures{
			SupportsReturning:       false,
			SupportsUpsert:          true, // ON DUPLICATE KEY UPDATE
			SupportsJSONColumn:      true, // MySQL 5.7+
			SupportsArrayColumn:     false,
			SupportsWindowFunctions: true, // MySQL 8.0+
			SupportsCTE:             true, // MySQL 8.0+
			MaxIdentifierLength:     64,
			MaxIndexNameLength:      64,
		}
	case Oracle:
		return DatabaseFeatures{
			SupportsReturning:       true,
			SupportsUpsert:          true, // MERGE statement
			SupportsJSONColumn:      true, // Oracle 12c+
			SupportsArrayColumn:     false,
			SupportsWindowFunctions: true,
			SupportsCTE:             true,
			MaxIdentifierLength:     128, // Oracle 12c+, 30 for older
			MaxIndexNameLength:      128,
		}
	case SQLServer:
		return DatabaseFeatures{
			SupportsReturning:       true, // OUTPUT clause
			SupportsUpsert:          true, // MERGE statement
			SupportsJSONColumn:      true, // SQL Server 2016+
			SupportsArrayColumn:     false,
			SupportsWindowFunctions: true,
			SupportsCTE:             true,
			MaxIdentifierLength:     128,
			MaxIndexNameLength:      128,
		}
	default:
		return DatabaseFeatures{}
	}
}

// Helper function to convert DatabaseType slice to string slice.
func typeStrings(types []DatabaseType) []string {
	strings := make([]string, len(types))
	for i, t := range types {
		strings[i] = string(t)
	}
	return strings
}

// LoadConfigFromEnv loads database configuration from environment variables.
func LoadConfigFromEnv() DatabaseConfig {
	config := DatabaseConfig{
		Type:     PostgreSQL, // Default to PostgreSQL
		Host:     getEnvWithDefault("DB_HOST", "postgres"),
		Port:     getEnvWithDefault("DB_PORT", "5432"),
		Database: getEnvWithDefault("DB_NAME", "gotrs"),
		Username: getEnvWithDefault("DB_USER", "gotrs"),
		Password: getEnvWithDefault("DB_PASSWORD", "gotrs_password"),
		SSLMode:  getEnvWithDefault("DB_SSLMODE", "disable"),

		// Connection pool defaults
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 0,
		ConnMaxIdleTime: 0,

		Options: make(map[string]string),
	}

	// Override database type if specified
	if dbType := getEnvWithDefault("DB_TYPE", ""); dbType != "" {
		config.Type = DatabaseType(dbType)
	}

	return config
}

// getEnvWithDefault gets environment variable with fallback default.
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
