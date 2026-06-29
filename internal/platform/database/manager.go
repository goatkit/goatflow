package database

import (
	"database/sql"
	"fmt"
	"sync"
)

var (
	abstractDB IDatabase
	initOnce   sync.Once
)

// Manager provides a singleton database instance using the abstraction layer.
type Manager struct {
	database IDatabase
	factory  IDatabaseFactory
}

// NewManager creates a new database manager.
func NewManager() *Manager {
	return &Manager{
		factory: NewDatabaseFactory(),
	}
}

// Initialize sets up the database connection with the given configuration.
func (m *Manager) Initialize(config DatabaseConfig) error {
	var err error
	initOnce.Do(func() {
		abstractDB, err = m.factory.Create(config)
		if err != nil {
			return
		}

		err = abstractDB.Connect()
		if err != nil {
			return
		}

		m.database = abstractDB
	})

	return err
}

// GetAbstractDB returns the global database instance (for backward compatibility).
func GetAbstractDB() (*sql.DB, error) {
	if abstractDB == nil {
		// Fallback to legacy connection method
		return getLegacyDB()
	}

	// If we have the abstraction layer, we need to extract the underlying sql.DB
	// This is a temporary bridge for backward compatibility
	switch d := abstractDB.(type) {
	case *PostgreSQLDatabase:
		return d.db, nil
	case *MySQLDatabase:
		return d.db, nil
	case *OracleDatabase:
		return d.db, nil
	case *SQLServerDatabase:
		return d.db, nil
	default:
		return nil, fmt.Errorf("unsupported database type for legacy access")
	}
}

// GetDatabase returns the database abstraction instance.
func GetDatabase() IDatabase {
	return abstractDB
}

// InitializeDefault initializes the database with default configuration from environment.
func InitializeDefault() error {
	config := LoadConfigFromEnv()
	manager := NewManager()
	return manager.Initialize(config)
}

// getLegacyDB is the original connection logic for backward compatibility.
func getLegacyDB() (*sql.DB, error) {
	// This is the original code from connection.go for fallback
	config := LoadConfigFromEnv()

	if config.Type != PostgreSQL {
		return nil, fmt.Errorf("legacy mode only supports PostgreSQL")
	}

	postgres := NewPostgreSQLDatabase(config)
	err := postgres.Connect()
	if err != nil {
		return nil, err
	}

	// Set the global database instance
	abstractDB = postgres

	return postgres.db, nil
}
