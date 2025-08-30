package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// DBAdapter provides database-specific query adaptations
type DBAdapter interface {
	// InsertWithReturning handles INSERT ... RETURNING for different databases
	InsertWithReturning(db *sql.DB, query string, args ...interface{}) (int64, error)
	
	// InsertWithReturningTx handles INSERT ... RETURNING within a transaction
	InsertWithReturningTx(tx *sql.Tx, query string, args ...interface{}) (int64, error)
	
	// CaseInsensitiveLike returns the appropriate case-insensitive LIKE operator
	CaseInsensitiveLike(column, pattern string) string
	
	// TypeCast handles type casting for different databases
	TypeCast(value, targetType string) string
}

// PostgreSQLAdapter implements DBAdapter for PostgreSQL
type PostgreSQLAdapter struct{}

func (p *PostgreSQLAdapter) InsertWithReturning(db *sql.DB, query string, args ...interface{}) (int64, error) {
	// PostgreSQL supports RETURNING directly
	var id int64
	err := db.QueryRow(query, args...).Scan(&id)
	return id, err
}

func (p *PostgreSQLAdapter) InsertWithReturningTx(tx *sql.Tx, query string, args ...interface{}) (int64, error) {
	// PostgreSQL supports RETURNING directly
	var id int64
	err := tx.QueryRow(query, args...).Scan(&id)
	return id, err
}

func (p *PostgreSQLAdapter) CaseInsensitiveLike(column, pattern string) string {
	// PostgreSQL uses ILIKE for case-insensitive
	return fmt.Sprintf("%s ILIKE %s", column, pattern)
}

func (p *PostgreSQLAdapter) TypeCast(value, targetType string) string {
	// PostgreSQL uses :: for type casting
	return fmt.Sprintf("%s::%s", value, targetType)
}

// MySQLAdapter implements DBAdapter for MySQL/MariaDB
type MySQLAdapter struct{}

func (m *MySQLAdapter) InsertWithReturning(db *sql.DB, query string, args ...interface{}) (int64, error) {
	// MySQL doesn't support RETURNING, remove it and use LastInsertId
	query = removeReturningClause(query)
	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (m *MySQLAdapter) InsertWithReturningTx(tx *sql.Tx, query string, args ...interface{}) (int64, error) {
	// MySQL doesn't support RETURNING, remove it and use LastInsertId
	query = removeReturningClause(query)
	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (m *MySQLAdapter) CaseInsensitiveLike(column, pattern string) string {
	// MySQL is case-insensitive by default, but we can be explicit
	return fmt.Sprintf("LOWER(%s) LIKE LOWER(%s)", column, pattern)
}

func (m *MySQLAdapter) TypeCast(value, targetType string) string {
	// MySQL uses CAST(value AS type)
	mysqlType := targetType
	switch targetType {
	case "text", "varchar":
		mysqlType = "CHAR"
	case "integer", "int":
		mysqlType = "SIGNED"
	case "timestamp":
		mysqlType = "DATETIME"
	}
	return fmt.Sprintf("CAST(%s AS %s)", value, mysqlType)
}

// Helper function to remove RETURNING clause from query
func removeReturningClause(query string) string {
	// Remove RETURNING clause and everything after it
	if idx := strings.Index(strings.ToUpper(query), "RETURNING"); idx != -1 {
		query = strings.TrimSpace(query[:idx])
	}
	return query
}

// Global adapter instance
var dbAdapter DBAdapter

// GetAdapter returns the appropriate database adapter based on configuration
func GetAdapter() DBAdapter {
	if dbAdapter == nil {
		dbDriver := os.Getenv("DB_DRIVER")
		if dbDriver == "" {
			dbDriver = "postgres" // default
		}
		
		switch dbDriver {
		case "mysql", "mariadb":
			dbAdapter = &MySQLAdapter{}
		default:
			dbAdapter = &PostgreSQLAdapter{}
		}
	}
	return dbAdapter
}

// ConvertQuery adapts a query for the current database
// This extends the existing ConvertPlaceholders functionality
func ConvertQuery(query string) string {
	// First convert placeholders
	query = ConvertPlaceholders(query)
	
	// Then handle ILIKE conversion for MySQL
	if os.Getenv("DB_DRIVER") == "mysql" || os.Getenv("DB_DRIVER") == "mariadb" {
		query = convertILIKE(query)
		query = convertTypeCasting(query)
	}
	
	return query
}

// convertILIKE converts ILIKE to MySQL-compatible syntax
func convertILIKE(query string) string {
	// Simple replacement - in production, use proper SQL parser
	query = strings.ReplaceAll(query, " ILIKE ", " LIKE ")
	query = strings.ReplaceAll(query, " ilike ", " LIKE ")
	
	// For more complex cases where we need case-insensitive in MySQL
	// This would need more sophisticated parsing in production
	return query
}

// convertTypeCasting converts PostgreSQL :: casting to MySQL CAST()
func convertTypeCasting(query string) string {
	// Simple patterns - in production, use proper SQL parser
	query = strings.ReplaceAll(query, "::text", "")  // MySQL doesn't need explicit text cast
	query = strings.ReplaceAll(query, "::integer", "")  // MySQL handles this automatically
	query = strings.ReplaceAll(query, "::date", "")  // MySQL handles this automatically
	query = strings.ReplaceAll(query, "::timestamp", "")  // MySQL handles this automatically
	
	return query
}