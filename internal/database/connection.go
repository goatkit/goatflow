package database

import (
	"database/sql"
	
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// GetDB returns the database connection singleton from the service registry
// Service registry is the single source of truth for database connections
func GetDB() (*sql.DB, error) {
    if testDB != nil {
        return testDB, nil
    }
    return adapter.GetDB()
}