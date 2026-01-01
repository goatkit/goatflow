//go:build integration

package lambda

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

type sqlmockDatabaseAdapter struct {
	db *sql.DB
}

type sqlmockTransaction struct {
	tx *sql.Tx
}

func newSQLMockAdapter(t *testing.T) (*sql.DB, sqlmock.Sqlmock, database.IDatabase) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock, &sqlmockDatabaseAdapter{db: db}
}

func (m *sqlmockDatabaseAdapter) Connect() error {
	return nil
}

func (m *sqlmockDatabaseAdapter) Close() error {
	return m.db.Close()
}

func (m *sqlmockDatabaseAdapter) Ping() error {
	return m.db.Ping()
}

func (m *sqlmockDatabaseAdapter) GetType() database.DatabaseType {
	return database.PostgreSQL
}

func (m *sqlmockDatabaseAdapter) GetConfig() database.DatabaseConfig {
	return database.DatabaseConfig{Type: database.PostgreSQL}
}

func (m *sqlmockDatabaseAdapter) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

func (m *sqlmockDatabaseAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

func (m *sqlmockDatabaseAdapter) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return m.db.ExecContext(ctx, query, args...)
}

func (m *sqlmockDatabaseAdapter) Begin(ctx context.Context) (database.ITransaction, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqlmockTransaction{tx: tx}, nil
}

func (m *sqlmockDatabaseAdapter) BeginTx(ctx context.Context, opts *sql.TxOptions) (database.ITransaction, error) {
	tx, err := m.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlmockTransaction{tx: tx}, nil
}

func (m *sqlmockDatabaseAdapter) TableExists(ctx context.Context, tableName string) (bool, error) {
	return false, nil
}

func (m *sqlmockDatabaseAdapter) GetTableColumns(ctx context.Context, tableName string) ([]database.ColumnInfo, error) {
	return nil, nil
}

func (m *sqlmockDatabaseAdapter) CreateTable(ctx context.Context, definition *database.TableDefinition) error {
	return nil
}

func (m *sqlmockDatabaseAdapter) DropTable(ctx context.Context, tableName string) error {
	return nil
}

func (m *sqlmockDatabaseAdapter) CreateIndex(ctx context.Context, tableName, indexName string, columns []string, unique bool) error {
	return nil
}

func (m *sqlmockDatabaseAdapter) DropIndex(ctx context.Context, tableName, indexName string) error {
	return nil
}

func (m *sqlmockDatabaseAdapter) Quote(identifier string) string {
	return fmt.Sprintf("\"%s\"", identifier)
}

func (m *sqlmockDatabaseAdapter) QuoteValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

func (m *sqlmockDatabaseAdapter) BuildInsert(tableName string, data map[string]interface{}) (string, []interface{}) {
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	args := make([]interface{}, 0, len(data))
	i := 1
	for col, val := range data {
		columns = append(columns, m.Quote(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		args = append(args, val)
		i++
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", m.Quote(tableName), strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	return query, args
}

func (m *sqlmockDatabaseAdapter) BuildUpdate(tableName string, data map[string]interface{}, where string, whereArgs []interface{}) (string, []interface{}) {
	setParts := make([]string, 0, len(data))
	args := make([]interface{}, 0, len(data)+len(whereArgs))
	i := 1
	for col, val := range data {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", m.Quote(col), i))
		args = append(args, val)
		i++
	}
	query := fmt.Sprintf("UPDATE %s SET %s", m.Quote(tableName), strings.Join(setParts, ", "))
	if where != "" {
		query += " WHERE " + where
		args = append(args, whereArgs...)
	}
	return query, args
}

func (m *sqlmockDatabaseAdapter) BuildSelect(tableName string, columns []string, where string, orderBy string, limit int) string {
	quoted := make([]string, len(columns))
	for i, col := range columns {
		quoted[i] = m.Quote(col)
	}
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(quoted, ", "), m.Quote(tableName))
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

func (m *sqlmockDatabaseAdapter) GetLimitClause(limit, offset int) string {
	switch {
	case limit > 0 && offset > 0:
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	case limit > 0:
		return fmt.Sprintf("LIMIT %d", limit)
	case offset > 0:
		return fmt.Sprintf("OFFSET %d", offset)
	default:
		return ""
	}
}

func (m *sqlmockDatabaseAdapter) GetDateFunction() string {
	return "NOW()"
}

func (m *sqlmockDatabaseAdapter) GetConcatFunction(fields []string) string {
	return fmt.Sprintf("CONCAT(%s)", strings.Join(fields, ", "))
}

func (m *sqlmockDatabaseAdapter) SupportsReturning() bool {
	return true
}

func (m *sqlmockDatabaseAdapter) Stats() sql.DBStats {
	return m.db.Stats()
}

func (m *sqlmockDatabaseAdapter) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return m.db.PingContext(ctx) == nil
}

func (t *sqlmockTransaction) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *sqlmockTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *sqlmockTransaction) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *sqlmockTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *sqlmockTransaction) Rollback() error {
	return t.tx.Rollback()
}

func TestEngine_ExecuteLambda_BasicFunctionality(t *testing.T) {
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close()

	db, mock, adapter := newSQLMockAdapter(t)
	defer db.Close()

	safeDB := NewSafeDBInterface(adapter)

	execCtx := ExecutionContext{
		Item: map[string]interface{}{
			"id":       1,
			"name":     "Test Item",
			"priority": 5,
			"status":   "active",
		},
		DB: safeDB,
	}

	config := DefaultLambdaConfig()

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "Simple return value",
			code:     `return "Hello, World!";`,
			expected: "Hello, World!",
		},
		{
			name:     "Access item properties",
			code:     `return "Item: " + item.name;`,
			expected: "Item: Test Item",
		},
		{
			name:     "Conditional logic",
			code:     `if (item.priority >= 5) { return "High Priority"; } else { return "Low Priority"; }`,
			expected: "High Priority",
		},
		{
			name:     "String concatenation",
			code:     `return item.name + " (" + item.status + ")";`,
			expected: "Test Item (active)",
		},
		{
			name:     "HTML generation",
			code:     `return '<span class="priority-' + item.priority + '">' + item.priority + '</span>';`,
			expected: `<span class="priority-5">5</span>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ExecuteLambda(tt.code, execCtx, config)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEngine_ExecuteLambda_DatabaseQueries(t *testing.T) {
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close()

	db, mock, adapter := newSQLMockAdapter(t)
	defer db.Close()

	safeDB := NewSafeDBInterface(adapter)

	execCtx := ExecutionContext{
		Item: map[string]interface{}{
			"id":       1,
			"queue_id": 2,
		},
		DB: safeDB,
	}

	config := DefaultLambdaConfig()

	// Test safe SELECT query
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM ticket WHERE queue_id = \\$1").
		WithArgs("2").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	code := `
		var count = db.queryRow("SELECT COUNT(*) FROM ticket WHERE queue_id = $1", item.queue_id.toString());
		return "Tickets: " + count;
	`

	result, err := engine.ExecuteLambda(code, execCtx, config)
	require.NoError(t, err)
	assert.Contains(t, result, "5") // Should contain the count

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEngine_ExecuteLambda_SecurityValidation(t *testing.T) {
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close()

	db, mock, adapter := newSQLMockAdapter(t)
	defer db.Close()

	safeDB := NewSafeDBInterface(adapter)

	execCtx := ExecutionContext{
		Item: map[string]interface{}{"id": 1},
		DB:   safeDB,
	}

	config := DefaultLambdaConfig()

	// Test that dangerous queries are blocked
	dangerousCode := `
		return db.queryRow("DELETE FROM users WHERE id = 1");
	`

	result, err := engine.ExecuteLambda(dangerousCode, execCtx, config)
	require.NoError(t, err)
	assert.Contains(t, result, "Error: Only SELECT queries are allowed")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEngine_ExecuteLambda_Timeout(t *testing.T) {
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close()

	db, mock, adapter := newSQLMockAdapter(t)
	defer db.Close()

	safeDB := NewSafeDBInterface(adapter)

	execCtx := ExecutionContext{
		Item: map[string]interface{}{"id": 1},
		DB:   safeDB,
	}

	// Use very short timeout
	config := LambdaConfig{
		TimeoutMs:     100, // 100ms
		MemoryLimitMB: 32,
	}

	// Infinite loop should timeout
	infiniteLoopCode := `
		while (true) {
			// This should timeout
		}
		return "Should not reach here";
	`

	result, err := engine.ExecuteLambda(infiniteLoopCode, execCtx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.Empty(t, result)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEngine_ExecuteLambda_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close()

	db, mock, adapter := newSQLMockAdapter(t)
	defer db.Close()

	safeDB := NewSafeDBInterface(adapter)

	execCtx := ExecutionContext{
		Item: map[string]interface{}{"id": 1},
		DB:   safeDB,
	}

	config := DefaultLambdaConfig()

	tests := []struct {
		name     string
		code     string
		hasError bool
	}{
		{
			name:     "Syntax error",
			code:     `return "unclosed string;`,
			hasError: true,
		},
		{
			name:     "Runtime error (handled by wrapper)",
			code:     `throw new Error("Runtime error");`,
			hasError: false, // Should be caught and returned as string
		},
		{
			name:     "Valid code",
			code:     `return "All good";`,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ExecuteLambda(tt.code, execCtx, config)

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.name == "Runtime error (handled by wrapper)" {
					assert.Contains(t, result, "Error: Runtime error")
				} else {
					assert.NotEmpty(t, result)
				}
			}
		})
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIsReadOnlyQuery(t *testing.T) {
	tests := []struct {
		query    string
		expected bool
	}{
		{"SELECT * FROM users", true},
		{"select count(*) from tickets", true},
		{"  SELECT name FROM queue  ", true},
		{"SELECT id FROM users WHERE id = 1", true},

		{"INSERT INTO users VALUES (1)", false},
		{"UPDATE users SET name = 'test'", false},
		{"DELETE FROM users", false},
		{"DROP TABLE users", false},
		{"CREATE TABLE test (id INT)", false},
		{"ALTER TABLE users ADD COLUMN test VARCHAR(50)", false},
		{"TRUNCATE TABLE users", false},
		{"CALL procedure()", false},
		{"EXEC sp_test", false},
		{"WITH cte AS (SELECT 1) INSERT INTO test SELECT * FROM cte", false},

		{"", false},
		{"SELEC", false},
		{"NOT A QUERY", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := isReadOnlyQuery(tt.query)
			assert.Equal(t, tt.expected, result, "Query: %s", tt.query)
		})
	}
}

func TestEngine_UtilityFunctions(t *testing.T) {
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close()

	db, mock, adapter := newSQLMockAdapter(t)
	defer db.Close()

	safeDB := NewSafeDBInterface(adapter)

	execCtx := ExecutionContext{
		Item: map[string]interface{}{
			"create_time": time.Now().Format(time.RFC3339),
		},
		DB: safeDB,
	}

	config := DefaultLambdaConfig()

	// Test formatDate utility
	code := `return formatDate(item.create_time);`

	result, err := engine.ExecuteLambda(code, execCtx, config)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.NotEqual(t, "-", result) // Should format the date

	require.NoError(t, mock.ExpectationsWereMet())
}
