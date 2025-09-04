//go:build integration
// +build integration

package lambda

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_ExecuteLambda_BasicFunctionality(t *testing.T) {
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close()

	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

    // Wrap the raw *sql.DB with a trivial adapter for tests
    // We only need the SafeDB interface methods used by the engine
    type rawDB struct{ *sql.DB }
    safeDB := NewSafeDBInterface(rawDB{db})

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

	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

    type rawDB struct{ *sql.DB }
    safeDB := NewSafeDBInterface(rawDB{db})

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

	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

    type rawDB struct{ *sql.DB }
    safeDB := NewSafeDBInterface(rawDB{db})

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

	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

    type rawDB struct{ *sql.DB }
    safeDB := NewSafeDBInterface(rawDB{db})

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

	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := database.NewDatabase(db)
	safeDB := NewSafeDBInterface(mockDB)

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

	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := database.NewDatabase(db)
	safeDB := NewSafeDBInterface(mockDB)

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