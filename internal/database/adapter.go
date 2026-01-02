// Package database provides database connection and adapter management.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// DBAdapter provides database-specific query adaptations.
type DBAdapter interface {
	// InsertWithReturning handles INSERT ... RETURNING for different databases.
	// Accepts PostgreSQL-style queries with $N placeholders (handles repeated placeholders).
	InsertWithReturning(db *sql.DB, query string, args ...interface{}) (int64, error)

	// InsertWithReturningTx handles INSERT ... RETURNING within a transaction.
	// Accepts PostgreSQL-style queries with $N placeholders (handles repeated placeholders).
	InsertWithReturningTx(tx *sql.Tx, query string, args ...interface{}) (int64, error)

	// Exec executes a query with PostgreSQL-style $N placeholders.
	// Handles repeated placeholders and placeholder conversion for MySQL.
	Exec(db *sql.DB, query string, args ...interface{}) (sql.Result, error)

	// ExecTx executes a query within a transaction with PostgreSQL-style $N placeholders.
	// Handles repeated placeholders and placeholder conversion for MySQL.
	ExecTx(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error)

	// Query executes a query with PostgreSQL-style $N placeholders and returns rows.
	// Handles repeated placeholders and placeholder conversion for MySQL.
	Query(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error)

	// QueryTx executes a query within a transaction and returns rows.
	// Handles repeated placeholders and placeholder conversion for MySQL.
	QueryTx(tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error)

	// QueryRow executes a query expected to return at most one row.
	// Handles repeated placeholders and placeholder conversion for MySQL.
	QueryRow(db *sql.DB, query string, args ...interface{}) *sql.Row

	// QueryRowTx executes a query within a transaction expected to return at most one row.
	// Handles repeated placeholders and placeholder conversion for MySQL.
	QueryRowTx(tx *sql.Tx, query string, args ...interface{}) *sql.Row

	// CaseInsensitiveLike returns the appropriate case-insensitive LIKE operator
	CaseInsensitiveLike(column, pattern string) string

	// TypeCast handles type casting for different databases
	TypeCast(value, targetType string) string

	// IntervalAdd returns SQL expression for adding an interval to a timestamp.
	// unit: "SECOND", "MINUTE", "HOUR", "DAY"
	// Example: IntervalAdd("NOW()", 1, "MINUTE") returns database-specific SQL
	IntervalAdd(timestamp string, amount int, unit string) string
}

// PostgreSQLAdapter implements DBAdapter for PostgreSQL.
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

func (p *PostgreSQLAdapter) Exec(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	// PostgreSQL handles $N placeholders and repeated references natively
	return db.Exec(query, args...)
}

func (p *PostgreSQLAdapter) ExecTx(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	// PostgreSQL handles $N placeholders and repeated references natively
	return tx.Exec(query, args...)
}

func (p *PostgreSQLAdapter) Query(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(query, args...)
}

func (p *PostgreSQLAdapter) QueryTx(tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	return tx.Query(query, args...)
}

func (p *PostgreSQLAdapter) QueryRow(db *sql.DB, query string, args ...interface{}) *sql.Row {
	return db.QueryRow(query, args...)
}

func (p *PostgreSQLAdapter) QueryRowTx(tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	return tx.QueryRow(query, args...)
}

func (p *PostgreSQLAdapter) CaseInsensitiveLike(column, pattern string) string {
	// PostgreSQL uses ILIKE for case-insensitive
	return fmt.Sprintf("%s ILIKE %s", column, pattern)
}

func (p *PostgreSQLAdapter) TypeCast(value, targetType string) string {
	// PostgreSQL uses :: for type casting
	return fmt.Sprintf("%s::%s", value, targetType)
}

func (p *PostgreSQLAdapter) IntervalAdd(timestamp string, amount int, unit string) string {
	// PostgreSQL: NOW() + INTERVAL '1 minute'
	return fmt.Sprintf("%s + INTERVAL '%d %s'", timestamp, amount, strings.ToLower(unit))
}

// MySQLAdapter implements DBAdapter for MySQL/MariaDB.
type MySQLAdapter struct{}

func (m *MySQLAdapter) InsertWithReturning(db *sql.DB, query string, args ...interface{}) (int64, error) {
	// Expand args for repeated placeholders and convert query if needed
	query, expandedArgs := prepareQueryForMySQL(query, args)
	result, err := db.Exec(query, expandedArgs...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (m *MySQLAdapter) InsertWithReturningTx(tx *sql.Tx, query string, args ...interface{}) (int64, error) {
	// Expand args for repeated placeholders and convert query if needed
	query, expandedArgs := prepareQueryForMySQL(query, args)
	result, err := tx.Exec(query, expandedArgs...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (m *MySQLAdapter) Exec(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	// Expand args for repeated placeholders and convert $N to ?
	expandedArgs := remapArgsForRepeatedPlaceholders(query, args)
	convertedQuery := ConvertPlaceholders(query)
	return db.Exec(convertedQuery, expandedArgs...)
}

func (m *MySQLAdapter) ExecTx(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	// Expand args for repeated placeholders and convert $N to ?
	expandedArgs := remapArgsForRepeatedPlaceholders(query, args)
	query = ConvertPlaceholders(query)
	return tx.Exec(query, expandedArgs...)
}

func (m *MySQLAdapter) Query(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	expandedArgs := remapArgsForRepeatedPlaceholders(query, args)
	query = ConvertPlaceholders(query)
	return db.Query(query, expandedArgs...)
}

func (m *MySQLAdapter) QueryTx(tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	expandedArgs := remapArgsForRepeatedPlaceholders(query, args)
	query = ConvertPlaceholders(query)
	return tx.Query(query, expandedArgs...)
}

func (m *MySQLAdapter) QueryRow(db *sql.DB, query string, args ...interface{}) *sql.Row {
	expandedArgs := remapArgsForRepeatedPlaceholders(query, args)
	query = ConvertPlaceholders(query)
	return db.QueryRow(query, expandedArgs...)
}

func (m *MySQLAdapter) QueryRowTx(tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	expandedArgs := remapArgsForRepeatedPlaceholders(query, args)
	query = ConvertPlaceholders(query)
	return tx.QueryRow(query, expandedArgs...)
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

func (m *MySQLAdapter) IntervalAdd(timestamp string, amount int, unit string) string {
	// MySQL: DATE_ADD(NOW(), INTERVAL 1 MINUTE)
	return fmt.Sprintf("DATE_ADD(%s, INTERVAL %d %s)", timestamp, amount, strings.ToUpper(unit))
}

// Helper function to remove RETURNING clause from query.
func removeReturningClause(query string) string {
	// Remove RETURNING clause and everything after it
	if idx := strings.Index(strings.ToUpper(query), "RETURNING"); idx != -1 {
		query = strings.TrimSpace(query[:idx])
	}
	return query
}

// prepareQueryForMySQL handles placeholder expansion and conversion for MySQL.
// It accepts either PostgreSQL-style ($N) or pre-converted (?) queries.
// For PostgreSQL-style queries, it expands args for repeated placeholders and converts to ?.
// For pre-converted queries, args are passed through unchanged.
func prepareQueryForMySQL(query string, args []interface{}) (string, []interface{}) {
	// Check if query has PostgreSQL-style placeholders
	re := regexp.MustCompile(`\$(\d+)`)
	matches := re.FindAllStringSubmatch(query, -1)

	if len(matches) == 0 {
		// Already converted to ?, just remove RETURNING
		return removeReturningClause(query), args
	}

	// Has $N placeholders - expand args for repeats, then convert
	expandedArgs := remapArgsForRepeatedPlaceholders(query, args)
	convertedQuery := ConvertPlaceholders(query)
	return removeReturningClause(convertedQuery), expandedArgs
}

// remapArgsForRepeatedPlaceholders expands args for queries with repeated $N placeholders
// and handles out-of-order placeholders.
// PostgreSQL allows $1 to appear multiple times, sharing the same arg, and any order.
// MySQL uses positional ? markers, so each placeholder needs its own arg copy in the order they appear.
// This is called internally by MySQLAdapter methods - callers don't need to use it.
func remapArgsForRepeatedPlaceholders(query string, args []interface{}) []interface{} {
	re := regexp.MustCompile(`\$(\d+)`)
	matches := re.FindAllStringSubmatch(query, -1)
	if len(matches) == 0 {
		return args
	}

	// Always expand to handle both repeated and out-of-order placeholders
	expanded := make([]interface{}, 0, len(matches))
	for _, match := range matches {
		idx, err := strconv.Atoi(match[1])
		if err != nil || idx < 1 || idx > len(args) {
			return args // Fall back to original args on parse error
		}
		expanded = append(expanded, args[idx-1])
	}

	return expanded
}

// Global adapter instance protected for concurrent access.
var (
	adapterMu sync.RWMutex
	dbAdapter DBAdapter
)

// GetAdapter returns the appropriate database adapter based on configuration.
func GetAdapter() DBAdapter {
	adapterMu.RLock()
	if dbAdapter != nil {
		defer adapterMu.RUnlock()
		return dbAdapter
	}
	adapterMu.RUnlock()

	adapterMu.Lock()
	defer adapterMu.Unlock()
	if dbAdapter == nil {
		dbAdapter = buildAdapterFromEnv()
	}
	return dbAdapter
}

// SetAdapter overrides the global adapter, primarily for tests.
func SetAdapter(adapter DBAdapter) {
	adapterMu.Lock()
	dbAdapter = adapter
	adapterMu.Unlock()
}

// ResetAdapterForTest clears the cached adapter so tests can rebuild state.
func ResetAdapterForTest() {
	adapterMu.Lock()
	dbAdapter = nil
	adapterMu.Unlock()
}

func buildAdapterFromEnv() DBAdapter {
	// In test mode, prefer TEST_ prefixed environment variables
	dbDriver := os.Getenv("TEST_DB_DRIVER")
	if dbDriver == "" {
		dbDriver = os.Getenv("DB_DRIVER")
	}
	if dbDriver == "" {
		dbDriver = "postgres"
	}

	switch dbDriver {
	case "mysql", "mariadb":
		return &MySQLAdapter{}
	default:
		return &PostgreSQLAdapter{}
	}
}

// This extends the existing ConvertPlaceholders functionality.
func ConvertQuery(query string) string {
	// First convert placeholders
	query = ConvertPlaceholders(query)

	// Then handle ILIKE conversion for MySQL
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if driver == "mysql" || driver == "mariadb" {
		query = convertILIKE(query)
		query = convertTypeCasting(query)
	}

	return query
}

// convertILIKE converts ILIKE to MySQL-compatible syntax.
func convertILIKE(query string) string {
	// Simple replacement - in production, use proper SQL parser
	query = strings.ReplaceAll(query, " ILIKE ", " LIKE ")
	query = strings.ReplaceAll(query, " ilike ", " LIKE ")

	// For more complex cases where we need case-insensitive in MySQL
	// This would need more sophisticated parsing in production
	return query
}

// convertTypeCasting converts PostgreSQL :: casting to MySQL CAST().
func convertTypeCasting(query string) string {
	// Simple patterns - in production, use proper SQL parser
	query = strings.ReplaceAll(query, "::text", "")      // MySQL doesn't need explicit text cast
	query = strings.ReplaceAll(query, "::integer", "")   // MySQL handles this automatically
	query = strings.ReplaceAll(query, "::date", "")      // MySQL handles this automatically
	query = strings.ReplaceAll(query, "::timestamp", "") // MySQL handles this automatically

	return query
}
