# Database Abstraction Layer Usage Guide

## CRITICAL: Always Use the Database Abstraction Layer

**As of August 28, 2025, all database operations MUST use the abstraction layer to maintain compatibility with multiple database backends (PostgreSQL, MySQL, Oracle, SQL Server) just like OTRS.**

## Why This Matters

GoatFlow supports the same database backends as OTRS to ensure seamless migration and deployment flexibility. Direct SQL queries bypass this abstraction and break compatibility.

## The Abstraction Layer

Located in `internal/database/`, the abstraction layer provides:

1. **IDatabase Interface**: Database-agnostic operations
2. **Backend Implementations**: PostgreSQL, MySQL, Oracle, SQL Server
3. **Query Builders**: Database-specific SQL generation
4. **Transaction Support**: Consistent transaction handling across backends

## How to Use It

### Getting the Database Instance

```go
import "github.com/goatkit/goatflow/internal/database"

// Get the abstracted database instance
db := database.GetInstance()
if db == nil {
    // Handle error - database not initialized
}
```

### Building Queries

Instead of raw SQL, use the abstraction layer's query builders:

```go
// BAD - Direct SQL (breaks compatibility)
query := `INSERT INTO ticket (tn, title, queue_id) VALUES ($1, $2, $3)`

// GOOD - Using abstraction layer
query, args := db.BuildInsert("ticket", map[string]interface{}{
    "tn":         ticketNumber,
    "title":      title,
    "queue_id":   queueID,
})
result, err := db.Exec(ctx, query, args...)
```

### Handling Database Differences

The abstraction layer handles database-specific differences:

```go
// Getting current timestamp (database-specific)
dateFunc := db.GetDateFunction() // Returns NOW() for PostgreSQL, GETDATE() for SQL Server, etc.

// Building LIMIT clauses (varies by database)
limitClause := db.GetLimitClause(10, 0) // Handles LIMIT/TOP/ROWNUM differences

// Checking for RETURNING support
if db.SupportsReturning() {
    query += " RETURNING id"
}
```

### Transactions

Use the abstraction layer's transaction interface:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

// Perform operations
_, err = tx.Exec(ctx, query, args...)
if err != nil {
    return err
}

return tx.Commit()
```

## Examples

### Insert with RETURNING (PostgreSQL) or Last Insert ID (MySQL)

```go
func CreateTicket(ctx context.Context, ticket *models.Ticket) (int64, error) {
    db := database.GetInstance()
    
    // Build insert query
    query, args := db.BuildInsert("ticket", map[string]interface{}{
        "tn":                ticket.TicketNumber,
        "title":             ticket.Title,
        "queue_id":          ticket.QueueID,
        "ticket_state_id":   ticket.StateID,
        "ticket_priority_id": ticket.PriorityID,
        "create_time":       db.GetDateFunction(),
        "create_by":         ticket.CreateBy,
    })
    
    var ticketID int64
    
    if db.SupportsReturning() {
        // PostgreSQL, newer SQL Server
        query += " RETURNING id"
        err := db.QueryRow(ctx, query, args...).Scan(&ticketID)
        return ticketID, err
    } else {
        // MySQL, Oracle, older SQL Server
        result, err := db.Exec(ctx, query, args...)
        if err != nil {
            return 0, err
        }
        ticketID, err = result.LastInsertId()
        return ticketID, err
    }
}
```

### Portable SELECT with Joins

```go
func GetTicketsWithQueue(ctx context.Context, limit int) ([]TicketInfo, error) {
    db := database.GetInstance()
    
    query := db.BuildSelect("ticket t", 
        []string{"t.id", "t.tn", "t.title", "q.name as queue_name"},
        "t.valid_id = 1",
        "t.create_time DESC",
        limit)
    
    // Add join (syntax varies by database)
    query = strings.Replace(query, "FROM ticket t", 
        "FROM ticket t INNER JOIN queue q ON t.queue_id = q.id", 1)
    
    rows, err := db.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    // Process results...
}
```

## Repository Pattern Best Practice

Create repositories that encapsulate database operations:

```go
type TicketRepository struct {
    db database.IDatabase
}

func NewTicketRepository() *TicketRepository {
    return &TicketRepository{
        db: database.GetInstance(),
    }
}

func (r *TicketRepository) Create(ctx context.Context, ticket *models.Ticket) error {
    // Use abstraction layer for all operations
}
```

## Migration from Direct SQL

When refactoring existing code:

1. Replace `*sql.DB` with `database.IDatabase`
2. Use `BuildInsert`, `BuildUpdate`, `BuildSelect` for query generation
3. Replace database-specific functions with abstraction methods
4. Test with multiple database backends

## Testing

Always test with at least two database backends:

```bash
# Test with PostgreSQL (default)
make test

# Test with MySQL
DATABASE_TYPE=mysql make test

# Test with SQLite (for unit tests)
DATABASE_TYPE=sqlite make test-unit
```

## Common Pitfalls to Avoid

1. **Never use `$1, $2` placeholders directly** - Use the abstraction layer which handles placeholder differences (`?` for MySQL, `:1` for Oracle)
2. **Never hardcode `NOW()`** - Use `db.GetDateFunction()`
3. **Never assume RETURNING support** - Check with `db.SupportsReturning()`
4. **Never use LIMIT directly** - Use `db.GetLimitClause()`
5. **Never bypass the abstraction** - Even for "simple" queries

## Database-Specific Features

If you need database-specific features, check the database type first:

```go
if db.GetType() == database.PostgreSQL {
    // PostgreSQL-specific feature
} else if db.GetType() == database.MySQL {
    // MySQL-specific feature
}
```

## Performance Considerations

The abstraction layer adds minimal overhead:
- Query building is done once at startup for static queries
- Runtime overhead is negligible (< 1% in benchmarks)
- Connection pooling is handled efficiently

## Getting Help

- Check existing repositories for examples: `internal/repository/`
- Read the interface documentation: `internal/database/interfaces.go`
- Test with multiple backends before committing

Remember: **Every direct SQL query is a compatibility bug waiting to happen!**