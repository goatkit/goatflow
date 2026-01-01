// Package postgres provides the PostgreSQL database driver implementation.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// PostgreSQLDriver implements the DatabaseDriver interface for PostgreSQL.
type PostgreSQLDriver struct {
	db *sql.DB
}

// NewPostgreSQLDriver creates a new PostgreSQL driver.
func NewPostgreSQLDriver() database.DatabaseDriver {
	return &PostgreSQLDriver{}
}

// Connect establishes a connection to PostgreSQL.
func (d *PostgreSQLDriver) Connect(ctx context.Context, dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}

	d.db = db
	return nil
}

// Close closes the database connection.
func (d *PostgreSQLDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping checks if the connection is alive.
func (d *PostgreSQLDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return sql.ErrConnDone
	}
	return d.db.PingContext(ctx)
}

// CreateTable generates and returns a CREATE TABLE query for PostgreSQL.
func (d *PostgreSQLDriver) CreateTable(schema database.TableSchema) (database.Query, error) {
	var parts []string

	// Determine primary key field
	pkField := schema.PK
	if pkField == "" {
		pkField = "id"
	}

	// Build column definitions
	for colName, colDef := range schema.Columns {
		colSQL := fmt.Sprintf("%s %s", colName, d.MapType(colDef.Type))

		if colName == pkField {
			colSQL += " PRIMARY KEY"
		}

		if colDef.Required {
			colSQL += " NOT NULL"
		}

		if colDef.Unique && colName != pkField {
			colSQL += " UNIQUE"
		}

		if colDef.Default != nil {
			switch v := colDef.Default.(type) {
			case string:
				if strings.HasPrefix(v, "CURRENT_") {
					colSQL += fmt.Sprintf(" DEFAULT %s", v)
				} else {
					colSQL += fmt.Sprintf(" DEFAULT '%s'", v)
				}
			case int, int64, float64:
				colSQL += fmt.Sprintf(" DEFAULT %v", v)
			case bool:
				if v {
					colSQL += " DEFAULT TRUE"
				} else {
					colSQL += " DEFAULT FALSE"
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
func (d *PostgreSQLDriver) DropTable(tableName string) (database.Query, error) {
	return database.Query{
		SQL:  fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", tableName),
		Args: nil,
	}, nil
}

// TableExists checks if a table exists.
func (d *PostgreSQLDriver) TableExists(tableName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)`

	var exists bool
	err := d.db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// Insert generates an INSERT query with RETURNING support.
func (d *PostgreSQLDriver) Insert(table string, data map[string]interface{}) (database.Query, error) {
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	i := 1
	for col, val := range data {
		columns = append(columns, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	return database.Query{SQL: sql, Args: values}, nil
}

// Update generates an UPDATE query.
func (d *PostgreSQLDriver) Update(table string, data map[string]interface{}, where string, whereArgs ...interface{}) (database.Query, error) {
	setClauses := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data)+len(whereArgs))

	i := 1
	for col, val := range data {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		values = append(values, val)
		i++
	}

	// Adjust where clause placeholders
	adjustedWhere := where
	for j := range whereArgs {
		old := fmt.Sprintf("$%d", j+1)
		new := fmt.Sprintf("$%d", i)
		adjustedWhere = strings.Replace(adjustedWhere, old, new, 1)
		values = append(values, whereArgs[j])
		i++
	}

	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s RETURNING *",
		table,
		strings.Join(setClauses, ", "),
		adjustedWhere)

	return database.Query{SQL: sql, Args: values}, nil
}

// Delete generates a DELETE query.
func (d *PostgreSQLDriver) Delete(table string, where string, whereArgs ...interface{}) (database.Query, error) {
	sql := fmt.Sprintf("DELETE FROM %s WHERE %s", table, where)
	return database.Query{SQL: sql, Args: whereArgs}, nil
}

// Select generates a SELECT query.
func (d *PostgreSQLDriver) Select(table string, columns []string, where string, whereArgs ...interface{}) (database.Query, error) {
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

// MapType maps schema types to PostgreSQL types.
func (d *PostgreSQLDriver) MapType(schemaType string) string {
	// Handle parameterized types
	if strings.HasPrefix(schemaType, "varchar") {
		return strings.ToUpper(schemaType)
	}

	switch strings.ToLower(schemaType) {
	case "serial":
		return "SERIAL"
	case "bigserial":
		return "BIGSERIAL"
	case "int", "integer":
		return "INTEGER"
	case "bigint":
		return "BIGINT"
	case "smallint":
		return "SMALLINT"
	case "text":
		return "TEXT"
	case "timestamp":
		return "TIMESTAMP"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case "boolean", "bool":
		return "BOOLEAN"
	case "json":
		return "JSONB"
	case "uuid":
		return "UUID"
	case "bytea", "blob":
		return "BYTEA"
	case "float", "real":
		return "REAL"
	case "double":
		return "DOUBLE PRECISION"
	default:
		return strings.ToUpper(schemaType)
	}
}

// SupportsReturning returns true for PostgreSQL.
func (d *PostgreSQLDriver) SupportsReturning() bool {
	return true
}

// SupportsLastInsertId returns false for PostgreSQL.
func (d *PostgreSQLDriver) SupportsLastInsertId() bool {
	return false
}

// SupportsArrays returns true for PostgreSQL.
func (d *PostgreSQLDriver) SupportsArrays() bool {
	return true
}

// BeginTx starts a new transaction.
func (d *PostgreSQLDriver) BeginTx(ctx context.Context) (database.Transaction, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &pgTransaction{tx: tx}, nil
}

// Exec executes a query without returning rows.
func (d *PostgreSQLDriver) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (d *PostgreSQLDriver) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *PostgreSQLDriver) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if d.db == nil {
		return nil
	}
	return d.db.QueryRowContext(ctx, query, args...)
}

// pgTransaction wraps sql.Tx to implement Transaction interface.
type pgTransaction struct {
	tx *sql.Tx
}

func (t *pgTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *pgTransaction) Rollback() error {
	return t.tx.Rollback()
}

func (t *pgTransaction) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

func (t *pgTransaction) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

func (t *pgTransaction) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(query, args...)
}

func init() {
	database.RegisterDriver("postgres", NewPostgreSQLDriver)
	database.RegisterDriver("postgresql", NewPostgreSQLDriver)
}
