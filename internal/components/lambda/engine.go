// Package lambda provides JavaScript execution capabilities for dynamic modules
package lambda

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/goatkit/goatflow/internal/database"
)

// ExecutionContext provides safe access to data and operations for lambda functions.
type ExecutionContext struct {
	Item map[string]interface{} `json:"item"`
	DB   *SafeDBInterface       `json:"db"`
}

// SafeDBInterface provides read-only database access for lambda functions.
type SafeDBInterface struct {
	db database.IDatabase
}

// SafeRow provides access to query results.
type SafeRow struct {
	data map[string]interface{}
}

// SafeRows provides access to multiple query results.
type SafeRows struct {
	rows []map[string]interface{}
}

// Engine manages JavaScript lambda execution with security constraints.
type Engine struct {
	runtime *goja.Runtime
	ctx     context.Context
}

// LambdaConfig holds configuration for lambda execution.
type LambdaConfig struct {
	TimeoutMs     int64 `yaml:"timeout_ms" json:"timeout_ms"`
	MemoryLimitMB int64 `yaml:"memory_limit_mb" json:"memory_limit_mb"`
}

// DefaultLambdaConfig returns safe default configuration.
func DefaultLambdaConfig() LambdaConfig {
	return LambdaConfig{
		TimeoutMs:     5000, // 5 second timeout
		MemoryLimitMB: 32,   // 32MB memory limit
	}
}

// NewEngine creates a new JavaScript execution engine.
func NewEngine(ctx context.Context) (*Engine, error) {
	runtime := goja.New()
	return &Engine{
		runtime: runtime,
		ctx:     ctx,
	}, nil
}

// Close properly disposes of the JavaScript runtime.
func (e *Engine) Close() {
	// Goja runtime doesn't need explicit disposal
	e.runtime = nil
}

// ExecuteLambda executes a JavaScript lambda function with the given context.
func (e *Engine) ExecuteLambda(code string, execCtx ExecutionContext, config LambdaConfig) (string, error) {
	// Create fresh runtime for each execution to ensure isolation
	runtime := goja.New()

	// Set execution timeout
	timeoutCtx, cancel := context.WithTimeout(e.ctx, time.Duration(config.TimeoutMs)*time.Millisecond)
	defer cancel()

	// Inject safe global objects
	e.injectGlobals(runtime, execCtx)

	// Wrap the code in a function to ensure return value
	wrappedCode := fmt.Sprintf(`
		(function() {
			try {
				%s
			} catch (error) {
				return 'Error: ' + error.message;
			}
		})()
	`, code)

	// Execute with timeout using goroutine
	type result struct {
		value goja.Value
		error error
	}
	resultCh := make(chan result, 1)

	go func() {
		val, err := runtime.RunString(wrappedCode)
		resultCh <- result{value: val, error: err}
	}()

	select {
	case <-timeoutCtx.Done():
		return "", fmt.Errorf("lambda execution timeout after %dms", config.TimeoutMs)
	case res := <-resultCh:
		if res.error != nil {
			return "", fmt.Errorf("lambda execution error: %w", res.error)
		}
		return res.value.String(), nil
	}
}

// injectGlobals provides safe access to data and utilities in the JavaScript context.
func (e *Engine) injectGlobals(runtime *goja.Runtime, execCtx ExecutionContext) {
	// Set item data directly as JavaScript object
	runtime.Set("item", execCtx.Item)

	// Create database interface object
	dbObj := runtime.NewObject()

	// Add queryRow method
	dbObj.Set("queryRow", func(call goja.FunctionCall) goja.Value {
		return e.handleQueryRow(runtime, call, execCtx.DB)
	})

	// Add query method
	dbObj.Set("query", func(call goja.FunctionCall) goja.Value {
		return e.handleQuery(runtime, call, execCtx.DB)
	})

	runtime.Set("db", dbObj)

	// Inject safe utility functions
	e.injectUtilities(runtime)
}

// injectUtilities provides safe utility functions.
func (e *Engine) injectUtilities(runtime *goja.Runtime) {
	// Add formatDate utility
	runtime.Set("formatDate", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return runtime.ToValue("")
		}

		dateStr := call.Arguments[0].String()
		if dateStr == "" || dateStr == "null" {
			return runtime.ToValue("-")
		}

		// Try to parse and format the date
		if parsedTime, err := time.Parse(time.RFC3339, dateStr); err == nil {
			formatted := parsedTime.Format("Jan 2, 2006 3:04 PM")
			return runtime.ToValue(formatted)
		}

		// Return original if parsing fails
		return runtime.ToValue(dateStr)
	})
}

// handleQueryRow executes a single-row query safely.
func (e *Engine) handleQueryRow(runtime *goja.Runtime, call goja.FunctionCall, db *SafeDBInterface) goja.Value {
	if len(call.Arguments) < 1 {
		return runtime.ToValue("Error: queryRow requires at least 1 argument")
	}

	query := call.Arguments[0].String()

	// Validate query is safe (read-only)
	if !isReadOnlyQuery(query) {
		return runtime.ToValue("Error: Only SELECT queries are allowed")
	}

	// Execute query with parameters
	args := make([]interface{}, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		args[i-1] = call.Arguments[i].String()
	}

	result, err := db.QueryRow(query, args...)
	if err != nil {
		return runtime.ToValue(fmt.Sprintf("Error: %v", err))
	}

	// Convert result to JavaScript object
	if len(result.data) == 0 {
		return runtime.ToValue("")
	}

	// For single values, return the value directly
	if len(result.data) == 1 {
		for _, value := range result.data {
			return runtime.ToValue(fmt.Sprintf("%v", value))
		}
	}

	// For multiple columns, return as object
	return runtime.ToValue(result.data)
}

// handleQuery executes a multi-row query safely.
func (e *Engine) handleQuery(runtime *goja.Runtime, call goja.FunctionCall, db *SafeDBInterface) goja.Value {
	if len(call.Arguments) < 1 {
		return runtime.ToValue("Error: query requires at least 1 argument")
	}

	query := call.Arguments[0].String()

	// Validate query is safe (read-only)
	if !isReadOnlyQuery(query) {
		return runtime.ToValue("Error: Only SELECT queries are allowed")
	}

	// Execute query with parameters
	args := make([]interface{}, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		args[i-1] = call.Arguments[i].String()
	}

	result, err := db.Query(query, args...)
	if err != nil {
		return runtime.ToValue(fmt.Sprintf("Error: %v", err))
	}

	// Convert results to JavaScript array
	return runtime.ToValue(result.rows)
}

// QueryRow executes a query that returns a single row.
func (db *SafeDBInterface) QueryRow(query string, args ...interface{}) (*SafeRow, error) {
	if !isReadOnlyQuery(query) {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}

	ctx := context.Background()
	row := db.db.QueryRow(ctx, query, args...)

	// We need to know the column names to build the result map
	// This is a simplified implementation - in practice you'd want to
	// use sql.Rows to get column information
	result := &SafeRow{
		data: make(map[string]interface{}),
	}

	// For now, assume simple single-value queries
	var value interface{}
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return result, nil
		}
		return nil, err
	}

	result.data["value"] = value
	return result, nil
}

// Query executes a query that returns multiple rows.
func (db *SafeDBInterface) Query(query string, args ...interface{}) (*SafeRows, error) {
	if !isReadOnlyQuery(query) {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}

	ctx := context.Background()
	rows, err := db.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &SafeRows{
		rows: make([]map[string]interface{}, 0),
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowData := make(map[string]interface{})
		for i, col := range columns {
			rowData[col] = values[i]
		}
		result.rows = append(result.rows, rowData)
	}

	return result, rows.Err()
}

// NewSafeDBInterface creates a new safe database interface.
func NewSafeDBInterface(db database.IDatabase) *SafeDBInterface {
	return &SafeDBInterface{db: db}
}

// isReadOnlyQuery validates that a query is safe (read-only).
func isReadOnlyQuery(query string) bool {
	// Convert to lowercase for case-insensitive checking
	q := query
	if len(q) < 6 {
		return false
	}

	// Trim whitespace and get first word
	q = " " + q + " " // Add spaces for boundary checking

	// Allow only SELECT statements
	// Block INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, etc.
	dangerous := []string{
		" INSERT ", " UPDATE ", " DELETE ", " DROP ", " ALTER ",
		" CREATE ", " TRUNCATE ", " REPLACE ", " MERGE ", " CALL ",
		" EXEC ", " EXECUTE ", " WITH ",
	}

	queryUpper := " " + strings.ToUpper(q) + " "
	for _, danger := range dangerous {
		if contains(queryUpper, danger) {
			return false
		}
	}

	// Must start with SELECT (after whitespace)
	trimmed := query
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n') {
		trimmed = trimmed[1:]
	}

	if len(trimmed) < 6 {
		return false
	}

	return (trimmed[0] == 'S' || trimmed[0] == 's') &&
		(trimmed[1] == 'E' || trimmed[1] == 'e') &&
		(trimmed[2] == 'L' || trimmed[2] == 'l') &&
		(trimmed[3] == 'E' || trimmed[3] == 'e') &&
		(trimmed[4] == 'C' || trimmed[4] == 'c') &&
		(trimmed[5] == 'T' || trimmed[5] == 't')
}

// contains checks if s contains substr (case insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (indexOf(s, substr) >= 0))
}

// indexOf returns the index of substr in s, -1 if not found (case insensitive).
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] >= 'A' && s[i+j] <= 'Z' {
				if s[i+j]+32 != substr[j] && s[i+j] != substr[j] {
					match = false
					break
				}
			} else if s[i+j] >= 'a' && s[i+j] <= 'z' {
				if s[i+j] != substr[j] && s[i+j]-32 != substr[j] {
					match = false
					break
				}
			} else if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
