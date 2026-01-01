// Package sqlite provides the SQLite database driver implementation.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// SQLiteDriver implements the DatabaseDriver interface for SQLite.
type SQLiteDriver struct {
	db *sql.DB
}

// NewSQLiteDriver creates a new SQLite driver.
func NewSQLiteDriver() database.DatabaseDriver {
	return &SQLiteDriver{}
}

// Connect establishes a connection to SQLite.
func (d *SQLiteDriver) Connect(ctx context.Context, dsn string) error {
	// For in-memory database, use ":memory:"
	// For file database, use file path
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return err
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return err
	}

	d.db = db
	return nil
}

// Close closes the database connection.
func (d *SQLiteDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping checks if the connection is alive.
func (d *SQLiteDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return sql.ErrConnDone
	}
	return d.db.PingContext(ctx)
}

// CreateTable generates and returns a CREATE TABLE query for SQLite.
func (d *SQLiteDriver) CreateTable(schema database.TableSchema) (database.Query, error) {
	var parts []string

	// Determine primary key field
	pkField := schema.PK
	if pkField == "" {
		pkField = "id"
	}

	// Build column definitions
	for colName, colDef := range schema.Columns {
		colSQL := fmt.Sprintf("%s %s", colName, d.MapType(colDef.Type))

		// Handle primary key with AUTOINCREMENT for serial types
		if colName == pkField {
			if strings.Contains(strings.ToLower(colDef.Type), "serial") {
				colSQL = fmt.Sprintf("%s INTEGER PRIMARY KEY AUTOINCREMENT", colName)
			} else {
				colSQL += " PRIMARY KEY"
			}
		}

		if colDef.Required && colName != pkField {
			colSQL += " NOT NULL"
		}

		if colDef.Unique && colName != pkField {
			colSQL += " UNIQUE"
		}

		if colDef.Default != nil && colName != pkField {
			switch v := colDef.Default.(type) {
			case string:
				if strings.HasPrefix(strings.ToUpper(v), "CURRENT_") {
					colSQL += " DEFAULT CURRENT_TIMESTAMP"
				} else {
					colSQL += fmt.Sprintf(" DEFAULT '%s'", v)
				}
			case int, int64, float64:
				colSQL += fmt.Sprintf(" DEFAULT %v", v)
			case bool:
				if v {
					colSQL += " DEFAULT 1"
				} else {
					colSQL += " DEFAULT 0"
				}
			}
		}

		parts = append(parts, colSQL)
	}

	// Add timestamps if requested
	if schema.Timestamps {
		if _, ok := schema.Columns["create_time"]; !ok {
			parts = append(parts, "create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP")
		}
		if _, ok := schema.Columns["change_time"]; !ok {
			parts = append(parts, "change_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP")
		}
		if _, ok := schema.Columns["create_by"]; !ok {
			parts = append(parts, "create_by INTEGER NOT NULL")
		}
		if _, ok := schema.Columns["change_by"]; !ok {
			parts = append(parts, "change_by INTEGER NOT NULL")
		}
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n    %s\n)",
		schema.Name, strings.Join(parts, ",\n    "))

	return database.Query{SQL: sql, Args: nil}, nil
}

// DropTable generates a DROP TABLE query.
func (d *SQLiteDriver) DropTable(tableName string) (database.Query, error) {
	return database.Query{
		SQL:  fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName),
		Args: nil,
	}, nil
}

// TableExists checks if a table exists.
func (d *SQLiteDriver) TableExists(tableName string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM sqlite_master 
		WHERE type='table' 
		AND name = ?`

	var exists bool
	err := d.db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// Insert generates an INSERT query with RETURNING support (SQLite 3.35+).
func (d *SQLiteDriver) Insert(table string, data map[string]interface{}) (database.Query, error) {
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	for col, val := range data {
		columns = append(columns, col)
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	// SQLite 3.35+ supports RETURNING
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	return database.Query{SQL: sql, Args: values}, nil
}

// Update generates an UPDATE query.
func (d *SQLiteDriver) Update(table string, data map[string]interface{}, where string, whereArgs ...interface{}) (database.Query, error) {
	setClauses := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data)+len(whereArgs))

	for col, val := range data {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", col))
		values = append(values, val)
	}

	// Add where arguments
	values = append(values, whereArgs...)

	// SQLite 3.35+ supports RETURNING
	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s RETURNING *",
		table,
		strings.Join(setClauses, ", "),
		where)

	return database.Query{SQL: sql, Args: values}, nil
}

// Delete generates a DELETE query.
func (d *SQLiteDriver) Delete(table string, where string, whereArgs ...interface{}) (database.Query, error) {
	sql := fmt.Sprintf("DELETE FROM %s WHERE %s", table, where)
	return database.Query{SQL: sql, Args: whereArgs}, nil
}

// Select generates a SELECT query.
func (d *SQLiteDriver) Select(table string, columns []string, where string, whereArgs ...interface{}) (database.Query, error) {
	cols := "*"
	if len(columns) > 0 {
		cols = strings.Join(columns, ", ")
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", cols, table)
	if where != "" {
		sql += " WHERE " + where
	}

	return database.Query{SQL: sql, Args: whereArgs}, nil
}

// MapType maps schema types to SQLite types.
func (d *SQLiteDriver) MapType(schemaType string) string {
	// SQLite has a very simple type system
	schemaType = strings.ToLower(schemaType)

	// Handle parameterized types
	if strings.HasPrefix(schemaType, "varchar") {
		return "TEXT"
	}

	switch schemaType {
	case "serial", "bigserial", "int", "integer", "bigint", "smallint":
		return "INTEGER"
	case "text", "char", "character":
		return "TEXT"
	case "timestamp", "date", "time", "datetime":
		return "TIMESTAMP"
	case "boolean", "bool":
		return "INTEGER" // 0 or 1
	case "json", "jsonb":
		return "TEXT" // Store as JSON text
	case "uuid":
		return "TEXT"
	case "bytea", "blob":
		return "BLOB"
	case "float", "real", "double":
		return "REAL"
	default:
		return "TEXT" // SQLite's default fallback
	}
}

// SupportsReturning returns true for SQLite 3.35+.
func (d *SQLiteDriver) SupportsReturning() bool {
	// Check SQLite version to determine RETURNING support
	var version string
	err := d.db.QueryRow("SELECT sqlite_version()").Scan(&version)
	if err != nil {
		return false
	}

	// Parse version and check if >= 3.35.0
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		major := parts[0]
		minor := parts[1]
		if major == "3" && len(minor) >= 2 {
			minorNum := 0
			fmt.Sscanf(minor, "%d", &minorNum)
			return minorNum >= 35
		}
	}
	return false
}

// SupportsLastInsertId returns true for SQLite.
func (d *SQLiteDriver) SupportsLastInsertId() bool {
	return true
}

// SupportsArrays returns false for SQLite.
func (d *SQLiteDriver) SupportsArrays() bool {
	return false
}

// BeginTx starts a new transaction.
func (d *SQLiteDriver) BeginTx(ctx context.Context) (database.Transaction, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &sqliteTransaction{tx: tx}, nil
}

// Exec executes a query without returning rows.
func (d *SQLiteDriver) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (d *SQLiteDriver) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *SQLiteDriver) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if d.db == nil {
		return nil
	}
	return d.db.QueryRowContext(ctx, query, args...)
}

// sqliteTransaction wraps sql.Tx to implement Transaction interface.
type sqliteTransaction struct {
	tx *sql.Tx
}

func (t *sqliteTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *sqliteTransaction) Rollback() error {
	return t.tx.Rollback()
}

func (t *sqliteTransaction) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

func (t *sqliteTransaction) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

func (t *sqliteTransaction) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(query, args...)
}

func init() {
	database.RegisterDriver("sqlite", NewSQLiteDriver)
	database.RegisterDriver("sqlite3", NewSQLiteDriver)
}
