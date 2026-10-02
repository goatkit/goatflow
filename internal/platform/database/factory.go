package database

import (
	"fmt"
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
	default: // Oracle, SQLServer: declared, not implemented
		return newUnimplementedDatabase(config), nil
	}
}

// supportedTypes lists the database types with a working implementation.
var supportedTypes = []DatabaseType{PostgreSQL, MySQL}

// plannedTypes lists declared database types without an implementation yet.
// The factory hands out a stub for them whose Connect fails with
// ErrDatabaseNotImplemented.
var plannedTypes = []DatabaseType{Oracle, SQLServer}

// GetSupportedTypes returns the database types with a working implementation.
func (f *DatabaseFactory) GetSupportedTypes() []DatabaseType {
	return append([]DatabaseType(nil), supportedTypes...)
}

func supportedTypesList() string {
	names := make([]string, len(supportedTypes))
	for i, t := range supportedTypes {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
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

	// Validate database type is known (implemented or planned)
	known := false
	for _, t := range append(f.GetSupportedTypes(), plannedTypes...) {
		if config.Type == t {
			known = true
			break
		}
	}

	if !known {
		return fmt.Errorf("unsupported database type: %s, supported types: %s",
			config.Type, supportedTypesList())
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
	default:
		return DatabaseFeatures{}
	}
}
