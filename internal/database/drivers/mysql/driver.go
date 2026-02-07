// Package mysql provides the MySQL database driver implementation.
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/goatkit/goatflow/internal/database"
)

// MySQLDriver implements the DatabaseDriver interface for MySQL/MariaDB.
type MySQLDriver struct {
	db *sql.DB
}

// NewMySQLDriver creates a new MySQL driver.
func NewMySQLDriver() database.DatabaseDriver {
	return &MySQLDriver{}
}

// Connect establishes a connection to MySQL.
func (d *MySQLDriver) Connect(ctx context.Context, dsn string) error {
	db, err := sql.Open("mysql", dsn)
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
func (d *MySQLDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping checks if the connection is alive.
func (d *MySQLDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return sql.ErrConnDone
	}
	return d.db.PingContext(ctx)
}

// CreateTable generates and returns a CREATE TABLE query for MySQL.
func (d *MySQLDriver) CreateTable(schema database.TableSchema) (database.Query, error) {
	parts := make([]string, 0, len(schema.Columns))

	// Determine primary key field
	pkField := schema.PK
	if pkField == "" {
		pkField = "id"
	}

	// Build column definitions
	for colName, colDef := range schema.Columns {
		colSQL := fmt.Sprintf("`%s` %s", colName, d.MapType(colDef.Type))

		// Handle AUTO_INCREMENT for primary key
		if colName == pkField {
			if strings.Contains(strings.ToLower(colDef.Type), "serial") {
				colSQL = fmt.Sprintf("`%s` %s AUTO_INCREMENT", colName, d.MapType(colDef.Type))
			}
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
			parts = append(parts, "`create_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP")
		}
		if _, ok := schema.Columns["change_time"]; !ok {
			parts = append(parts, "`change_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP")
		}
		if _, ok := schema.Columns["create_by"]; !ok {
			parts = append(parts, "`create_by` INT NOT NULL")
		}
		if _, ok := schema.Columns["change_by"]; !ok {
			parts = append(parts, "`change_by` INT NOT NULL")
		}
	}

	// Add indexes
	for _, idx := range schema.Indexes {
		parts = append(parts, fmt.Sprintf("INDEX `idx_%s_%s` (`%s`)", schema.Name, idx, idx))
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (\n    %s\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		schema.Name, strings.Join(parts, ",\n    "))

	return database.Query{SQL: sql, Args: nil}, nil
}

// DropTable generates a DROP TABLE query.
func (d *MySQLDriver) DropTable(tableName string) (database.Query, error) {
	return database.Query{
		SQL:  fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName),
		Args: nil,
	}, nil
}

// TableExists checks if a table exists.
func (d *MySQLDriver) TableExists(tableName string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM information_schema.tables 
		WHERE table_schema = DATABASE()
		AND table_name = ?`

	var exists bool
	err := d.db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// Insert generates an INSERT query (no RETURNING support in MySQL).
func (d *MySQLDriver) Insert(table string, data map[string]interface{}) (database.Query, error) {
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	for col, val := range data {
		columns = append(columns, fmt.Sprintf("`%s`", col))
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	return database.Query{SQL: sql, Args: values}, nil
}

// Update generates an UPDATE query.
func (d *MySQLDriver) Update(table string, data map[string]interface{}, where string, whereArgs ...interface{}) (database.Query, error) {
	setClauses := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data)+len(whereArgs))

	for col, val := range data {
		setClauses = append(setClauses, fmt.Sprintf("`%s` = ?", col))
		values = append(values, val)
	}

	// Add where arguments
	values = append(values, whereArgs...)

	sql := fmt.Sprintf("UPDATE `%s` SET %s WHERE %s",
		table,
		strings.Join(setClauses, ", "),
		where)

	return database.Query{SQL: sql, Args: values}, nil
}

// Delete generates a DELETE query.
func (d *MySQLDriver) Delete(table string, where string, whereArgs ...interface{}) (database.Query, error) {
	sql := fmt.Sprintf("DELETE FROM `%s` WHERE %s", table, where)
	return database.Query{SQL: sql, Args: whereArgs}, nil
}

// Select generates a SELECT query.
func (d *MySQLDriver) Select(table string, columns []string, where string, whereArgs ...interface{}) (database.Query, error) {
	cols := "*"
	if len(columns) > 0 {
		quotedCols := make([]string, len(columns))
		for i, col := range columns {
			quotedCols[i] = fmt.Sprintf("`%s`", col)
		}
		cols = strings.Join(quotedCols, ", ")
	}

	sql := fmt.Sprintf("SELECT %s FROM `%s`", cols, table)
	if where != "" {
		sql += " WHERE " + where
	}

	return database.Query{SQL: sql, Args: whereArgs}, nil
}

// MapType maps schema types to MySQL types.
func (d *MySQLDriver) MapType(schemaType string) string {
	// Handle parameterized types
	if strings.HasPrefix(strings.ToLower(schemaType), "varchar") {
		return strings.ToUpper(schemaType)
	}

	switch strings.ToLower(schemaType) {
	case "serial":
		return "INT"
	case "bigserial":
		return "BIGINT"
	case "int", "integer":
		return "INT"
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
		return "TINYINT(1)"
	case "json", "jsonb":
		return "JSON"
	case "uuid":
		return "VARCHAR(36)"
	case "bytea", "blob":
		return "BLOB"
	case "float", "real":
		return "FLOAT"
	case "double":
		return "DOUBLE"
	default:
		return strings.ToUpper(schemaType)
	}
}

// SupportsReturning returns false for MySQL.
func (d *MySQLDriver) SupportsReturning() bool {
	return false
}

// SupportsLastInsertId returns true for MySQL.
func (d *MySQLDriver) SupportsLastInsertId() bool {
	return true
}

// SupportsArrays returns false for MySQL.
func (d *MySQLDriver) SupportsArrays() bool {
	return false
}

// BeginTx starts a new transaction.
func (d *MySQLDriver) BeginTx(ctx context.Context) (database.Transaction, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &mysqlTransaction{tx: tx}, nil
}

// Exec executes a query without returning rows.
func (d *MySQLDriver) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (d *MySQLDriver) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if d.db == nil {
		return nil, sql.ErrConnDone
	}
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *MySQLDriver) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if d.db == nil {
		return nil
	}
	return d.db.QueryRowContext(ctx, query, args...)
}

// mysqlTransaction wraps sql.Tx to implement Transaction interface.
type mysqlTransaction struct {
	tx *sql.Tx
}

func (t *mysqlTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *mysqlTransaction) Rollback() error {
	return t.tx.Rollback()
}

func (t *mysqlTransaction) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

func (t *mysqlTransaction) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

func (t *mysqlTransaction) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(query, args...)
}

func init() {
	database.RegisterDriver("mysql", NewMySQLDriver)
	database.RegisterDriver("mariadb", NewMySQLDriver)
}
