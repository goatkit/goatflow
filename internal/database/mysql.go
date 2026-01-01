package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	// _ "github.com/go-sql-driver/mysql" // TODO: Add when implementing MySQL support.
)

// MySQLDatabase implements IDatabase for MySQL/MariaDB.
type MySQLDatabase struct {
	config DatabaseConfig
	db     *sql.DB
}

// NewMySQLDatabase creates a new MySQL database instance.
func NewMySQLDatabase(config DatabaseConfig) *MySQLDatabase {
	return &MySQLDatabase{
		config: config,
	}
}

// Connect establishes connection to MySQL database.
func (m *MySQLDatabase) Connect() error {
	dsn := m.buildDSN()

	var err error
	m.db, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Configure connection pool
	if m.config.MaxOpenConns > 0 {
		m.db.SetMaxOpenConns(m.config.MaxOpenConns)
	}
	if m.config.MaxIdleConns > 0 {
		m.db.SetMaxIdleConns(m.config.MaxIdleConns)
	}
	if m.config.ConnMaxLifetime > 0 {
		m.db.SetConnMaxLifetime(m.config.ConnMaxLifetime)
	}
	if m.config.ConnMaxIdleTime > 0 {
		m.db.SetConnMaxIdleTime(m.config.ConnMaxIdleTime)
	}

	return m.Ping()
}

// Close closes the database connection.
func (m *MySQLDatabase) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Ping tests the database connection.
func (m *MySQLDatabase) Ping() error {
	if m.db == nil {
		return fmt.Errorf("database connection not established")
	}
	return m.db.Ping()
}

// GetType returns the database type.
func (m *MySQLDatabase) GetType() DatabaseType {
	return MySQL
}

// GetConfig returns the database configuration.
func (m *MySQLDatabase) GetConfig() DatabaseConfig {
	return m.config
}

// Query executes a query and returns rows.
func (m *MySQLDatabase) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query and returns a single row.
func (m *MySQLDatabase) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query and returns the result.
func (m *MySQLDatabase) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return m.db.ExecContext(ctx, query, args...)
}

// Begin starts a transaction.
func (m *MySQLDatabase) Begin(ctx context.Context) (ITransaction, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &MySQLTransaction{tx: tx}, nil
}

// BeginTx starts a transaction with options.
func (m *MySQLDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (ITransaction, error) {
	tx, err := m.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &MySQLTransaction{tx: tx}, nil
}

// TableExists checks if a table exists.
func (m *MySQLDatabase) TableExists(ctx context.Context, tableName string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0 
		FROM information_schema.tables 
		WHERE table_schema = DATABASE() 
		AND table_name = ?`

	var exists bool
	err := m.db.QueryRowContext(ctx, query, tableName).Scan(&exists)
	return exists, err
}

// GetTableColumns returns column information for a table (stub implementation).
func (m *MySQLDatabase) GetTableColumns(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	// TODO: Implement MySQL-specific column introspection
	return []ColumnInfo{}, fmt.Errorf("GetTableColumns not yet implemented for MySQL")
}

// CreateTable creates a table from definition (stub implementation).
func (m *MySQLDatabase) CreateTable(ctx context.Context, definition *TableDefinition) error {
	// TODO: Implement MySQL-specific CREATE TABLE
	return fmt.Errorf("CreateTable not yet implemented for MySQL")
}

// DropTable drops a table.
func (m *MySQLDatabase) DropTable(ctx context.Context, tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", m.Quote(tableName))
	_, err := m.db.ExecContext(ctx, query)
	return err
}

// CreateIndex creates an index.
func (m *MySQLDatabase) CreateIndex(ctx context.Context, tableName, indexName string, columns []string, unique bool) error {
	uniqueClause := ""
	if unique {
		uniqueClause = "UNIQUE "
	}

	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = m.Quote(col)
	}

	query := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
		uniqueClause,
		m.Quote(indexName),
		m.Quote(tableName),
		strings.Join(quotedColumns, ", "))

	_, err := m.db.ExecContext(ctx, query)
	return err
}

// DropIndex drops an index.
func (m *MySQLDatabase) DropIndex(ctx context.Context, tableName, indexName string) error {
	query := fmt.Sprintf("DROP INDEX %s ON %s", m.Quote(indexName), m.Quote(tableName))
	_, err := m.db.ExecContext(ctx, query)
	return err
}

// Quote quotes an identifier (MySQL uses backticks).
func (m *MySQLDatabase) Quote(identifier string) string {
	return fmt.Sprintf("`%s`", identifier)
}

// QuoteValue quotes a value.
func (m *MySQLDatabase) QuoteValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

// BuildInsert builds an INSERT statement (stub implementation).
func (m *MySQLDatabase) BuildInsert(tableName string, data map[string]interface{}) (string, []interface{}) {
	// TODO: Implement MySQL-specific INSERT with ? placeholders
	return "", nil
}

// BuildUpdate builds an UPDATE statement (stub implementation).
func (m *MySQLDatabase) BuildUpdate(tableName string, data map[string]interface{}, where string, whereArgs []interface{}) (string, []interface{}) {
	// TODO: Implement MySQL-specific UPDATE with ? placeholders
	return "", nil
}

// BuildSelect builds a SELECT statement.
func (m *MySQLDatabase) BuildSelect(tableName string, columns []string, where string, orderBy string, limit int) string {
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = m.Quote(col)
	}

	query := fmt.Sprintf("SELECT %s FROM %s",
		strings.Join(quotedColumns, ", "),
		m.Quote(tableName))

	if where != "" {
		query += " WHERE " + where
	}

	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	return query
}

// GetLimitClause returns MySQL-specific LIMIT clause.
func (m *MySQLDatabase) GetLimitClause(limit, offset int) string {
	if limit > 0 && offset > 0 {
		return fmt.Sprintf("LIMIT %d, %d", offset, limit)
	} else if limit > 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	}
	return ""
}

// GetDateFunction returns current date function.
func (m *MySQLDatabase) GetDateFunction() string {
	return "NOW()"
}

// GetConcatFunction returns concatenation function.
func (m *MySQLDatabase) GetConcatFunction(fields []string) string {
	return fmt.Sprintf("CONCAT(%s)", strings.Join(fields, ", "))
}

// SupportsReturning returns false for MySQL.
func (m *MySQLDatabase) SupportsReturning() bool {
	return false
}

// Stats returns database connection statistics.
func (m *MySQLDatabase) Stats() sql.DBStats {
	return m.db.Stats()
}

// IsHealthy checks if database is healthy.
func (m *MySQLDatabase) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return m.db.PingContext(ctx) == nil
}

// buildDSN builds the MySQL connection string.
func (m *MySQLDatabase) buildDSN() string {
	// MySQL DSN format: user:password@tcp(host:port)/dbname
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		m.config.Username, m.config.Password, m.config.Host, m.config.Port, m.config.Database)

	// Add parameters
	params := []string{"parseTime=true"} // Always parse time

	// Add additional options
	for key, value := range m.config.Options {
		params = append(params, fmt.Sprintf("%s=%s", key, value))
	}

	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}

	return dsn
}

// MySQLTransaction implements ITransaction for MySQL.
type MySQLTransaction struct {
	tx *sql.Tx
}

func (t *MySQLTransaction) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *MySQLTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *MySQLTransaction) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *MySQLTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *MySQLTransaction) Rollback() error {
	return t.tx.Rollback()
}
