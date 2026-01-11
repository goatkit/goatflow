
## TESTING INFRASTRUCTURE - MEMORIZE THIS (Jan 11, 2026)

**We have a FULL test stack with a dedicated database.**

### Test Database Setup
- Dedicated test database container available
- Tests run WITH a real database, not mocks
- Seed stage populates baseline data before tests
- After each test, database resets to baseline for next test
- Run tests with: `make test`

### How to Write Tests
1. Use the real database connection - DO NOT mock the database
2. Seed data is available - use it
3. Database resets between tests - each test starts clean
4. Integration tests should use the actual DB, not be skipped

### Makefile Targets for Testing
- `make test` - brings up test stack and runs all tests
- `make toolbox-test` - runs tests in toolbox container with DB access
- `make db-shell-test` - access the database directly

### NEVER DO THIS
- Don't write tests that skip because "no DB connection"
- Don't mock database calls when real DB is available. Spoiler: REAL test db is available.
- Don't claim low coverage is acceptable because "DB required"
- Don't use `// +build integration` tags to skip DB tests

**The test database EXISTS. Use it.**

## DATABASE WRAPPER PATTERNS - ALWAYS USE THESE (Jan 11, 2026)

**Use `database.ConvertPlaceholders()` for all SQL queries. This allows future sqlx migration.**

### The Correct Pattern
```go
import "github.com/gotrs-io/gotrs-ce/internal/database"

// Write SQL with ? placeholders, convert before execution
query := database.ConvertPlaceholders(`
    SELECT id, name FROM users WHERE id = ? AND valid_id = ?
`)
row := db.QueryRowContext(ctx, query, userID, 1)

// For INSERT with RETURNING (handles MySQL vs PostgreSQL)
query := database.ConvertPlaceholders(`
    INSERT INTO users (name, email) VALUES (?, ?) RETURNING id
`)
query, useLastInsert := database.ConvertReturning(query)
if useLastInsert {
    result, err := db.ExecContext(ctx, query, name, email)
    id, _ = result.LastInsertId()
} else {
    err = db.QueryRowContext(ctx, query, name, email).Scan(&id)
}
```

### For Complex Operations Use GetAdapter()
```go
// GetAdapter() is for complex cases like InsertWithReturning
adapter := database.GetAdapter()
id, err := adapter.InsertWithReturning(db, query, args...)
```

### Test Code Uses Same Patterns
```go
func TestSomething(t *testing.T) {
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available")
    }
    defer database.CloseTestDB()

    db, err := database.GetDB()
    require.NoError(t, err)

    // Use ConvertPlaceholders for queries
    query := database.ConvertPlaceholders(`SELECT id FROM users WHERE id = ?`)
    row := db.QueryRowContext(ctx, query, 1)
}
```

### Why This Pattern
- `ConvertPlaceholders()` handles MySQL vs PostgreSQL placeholder differences
- Designed so sqlx can be swapped in later
- `ConvertReturning()` handles RETURNING clause differences
- `GetAdapter()` for complex operations like InsertWithReturning
