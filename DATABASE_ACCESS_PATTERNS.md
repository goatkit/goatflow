# Database Access Patterns - GOTRS MySQL Compatibility

## 🎯 Critical Achievement: MySQL Compatibility Restored (August 29, 2025)

GOTRS now has **full MySQL compatibility** with zero placeholder conversion errors. This document establishes the **mandatory patterns** for all database access going forward.

## 🚨 MANDATORY: No Direct SQL Without ConvertPlaceholders

**NEVER write direct SQL queries. ALWAYS use the ConvertPlaceholders wrapper.**

### ❌ WRONG - Direct SQL (Breaks MySQL)
```go
// DON'T DO THIS - Will fail on MySQL with "$1" errors
db.QueryRow("SELECT id FROM users WHERE login = $1", login)
db.Exec("UPDATE ticket SET status = $1 WHERE id = $2", status, id)
```

### ✅ CORRECT - ConvertPlaceholders Pattern
```go
// ALWAYS DO THIS - Works on both PostgreSQL and MySQL
db.QueryRow(database.ConvertPlaceholders(`
    SELECT id FROM users WHERE login = $1
`), login)

db.Exec(database.ConvertPlaceholders(`
    UPDATE ticket SET status = $1 WHERE id = $2
`), status, id)
```

## 📝 ConvertPlaceholders Syntax Rules

### 1. Parenthesis Placement - CRITICAL
```go
// ❌ WRONG - Parenthesis after arguments
db.Query(database.ConvertPlaceholders(`SELECT * FROM ticket WHERE id = $1`, ticketID))

// ✅ CORRECT - Parenthesis after backtick, then comma and arguments
db.Query(database.ConvertPlaceholders(`SELECT * FROM ticket WHERE id = $1`), ticketID)
```

### 2. Multi-line Queries
```go
// ✅ CORRECT - Clean formatting with proper parenthesis placement
rows, err := db.Query(database.ConvertPlaceholders(`
    SELECT t.id, t.tn, t.title,
           ts.name as state,
           tp.name as priority
    FROM ticket t
    JOIN ticket_state ts ON t.ticket_state_id = ts.id
    JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
    WHERE t.queue_id = $1 
      AND t.ticket_state_id IN ($2, $3)
    ORDER BY t.create_time DESC
    LIMIT $4
`), queueID, state1, state2, limit)
```

### 3. Complex Queries with Multiple Conditions
```go
// ✅ CORRECT - Complex conditional logic
query := `SELECT * FROM ticket WHERE 1=1`
args := []interface{}{}
argCount := 0

if status != "" {
    argCount++
    query += fmt.Sprintf(" AND status = $%d", argCount)
    args = append(args, status)
}

if queueID > 0 {
    argCount++
    query += fmt.Sprintf(" AND queue_id = $%d", argCount)
    args = append(args, queueID)
}

rows, err := db.Query(database.ConvertPlaceholders(query), args...)
```

## 🔧 How ConvertPlaceholders Works

The function automatically converts PostgreSQL-style placeholders to the target database format:

```go
func ConvertPlaceholders(query string) string {
    if !IsMySQL() {
        return query // PostgreSQL uses $1, $2, etc. directly
    }
    
    // For MySQL: Convert $1, $2, $3 -> ?, ?, ?
    re := regexp.MustCompile(`\$\d+`)
    placeholders := re.FindAllString(query, -1)
    
    result := query
    for _, placeholder := range placeholders {
        result = strings.Replace(result, placeholder, "?", 1)
    }
    
    return result
}
```

**Result**:
- **PostgreSQL**: `SELECT * FROM users WHERE id = $1` → `SELECT * FROM users WHERE id = $1` (unchanged)
- **MySQL**: `SELECT * FROM users WHERE id = $1` → `SELECT * FROM users WHERE id = ?` (converted)

## 📊 Database Driver Detection

The system automatically detects the database driver from environment variables:

```go
func IsMySQL() bool {
    driver := os.Getenv("DB_DRIVER")
    return driver == "mysql"
}
```

**Environment Configuration**:
```bash
# MySQL (OTRS compatibility)
DB_DRIVER=mysql
DB_HOST=mysql
DB_USER=otrs
DB_PASSWORD=LetClaude.1n
DB_NAME=otrs
DB_PORT=3306

# PostgreSQL (development)
DB_DRIVER=postgres
DB_HOST=postgres
DB_USER=gotrs_user
DB_PASSWORD=your_postgres_password_here
DB_NAME=gotrs
DB_PORT=5432
```

## 🛠️ Common Patterns and Examples

### 1. Simple Queries
```go
// User lookup
var user models.User
err := db.QueryRow(database.ConvertPlaceholders(`
    SELECT id, login, first_name, last_name, valid_id
    FROM users
    WHERE id = $1
`), userID).Scan(&user.ID, &user.Login, &user.FirstName, &user.LastName, &user.ValidID)
```

### 2. Insert with RETURNING (PostgreSQL) vs Last Insert ID (MySQL)
```go
// This pattern works on both databases
var ticketID int64
err = db.QueryRow(database.ConvertPlaceholders(`
    INSERT INTO ticket (tn, title, queue_id, create_time, create_by)
    VALUES ($1, $2, $3, NOW(), $4)
    RETURNING id
`), ticketNumber, title, queueID, userID).Scan(&ticketID)

if err != nil && IsMySQL() {
    // MySQL fallback: Use LastInsertId() if RETURNING not supported
    result, execErr := db.Exec(database.ConvertPlaceholders(`
        INSERT INTO ticket (tn, title, queue_id, create_time, create_by)
        VALUES ($1, $2, $3, NOW(), $4)
    `), ticketNumber, title, queueID, userID)
    
    if execErr == nil {
        ticketID, _ = result.LastInsertId()
    }
}
```

### 3. Transactions
```go
tx, err := db.Begin()
if err != nil {
    return err
}
defer tx.Rollback()

// All queries in transaction must use ConvertPlaceholders
_, err = tx.Exec(database.ConvertPlaceholders(`
    UPDATE ticket SET status = $1 WHERE id = $2
`), newStatus, ticketID)

if err != nil {
    return err
}

_, err = tx.Exec(database.ConvertPlaceholders(`
    INSERT INTO article (ticket_id, subject, body, create_time, create_by)
    VALUES ($1, $2, $3, NOW(), $4)
`), ticketID, subject, body, userID)

if err != nil {
    return err
}

return tx.Commit()
```

### 4. Dynamic Query Building
```go
// Build query dynamically while maintaining placeholder pattern
func buildTicketQuery(filters map[string]interface{}) (string, []interface{}) {
    query := `
        SELECT t.id, t.tn, t.title, ts.name as state
        FROM ticket t
        JOIN ticket_state ts ON t.ticket_state_id = ts.id
        WHERE 1=1
    `
    
    args := []interface{}{}
    argCount := 0
    
    if status, exists := filters["status"]; exists {
        argCount++
        query += fmt.Sprintf(" AND t.ticket_state_id = $%d", argCount)
        args = append(args, status)
    }
    
    if queueID, exists := filters["queue_id"]; exists {
        argCount++
        query += fmt.Sprintf(" AND t.queue_id = $%d", argCount)
        args = append(args, queueID)
    }
    
    if search, exists := filters["search"]; exists {
        argCount++
        query += fmt.Sprintf(" AND (t.tn ILIKE $%d OR t.title ILIKE $%d)", argCount, argCount)
        args = append(args, "%"+search.(string)+"%")
    }
    
    query += " ORDER BY t.create_time DESC LIMIT 50"
    
    return query, args
}

// Usage
query, args := buildTicketQuery(filters)
rows, err := db.Query(database.ConvertPlaceholders(query), args...)
```

## ⚠️ Common Mistakes to Avoid

### 1. Wrong Parenthesis Placement
```go
// ❌ This breaks compilation
db.Query(database.ConvertPlaceholders(`SELECT * FROM ticket`, args...))
//                                                          ^
//                                               Wrong placement

// ✅ Correct placement
db.Query(database.ConvertPlaceholders(`SELECT * FROM ticket`), args...)
//                                                            ^
//                                                    Correct placement
```

### 2. Forgetting ConvertPlaceholders
```go
// ❌ Direct SQL - will fail on MySQL
rows, err := db.Query("SELECT * FROM ticket WHERE queue_id = $1", queueID)

// ✅ Always wrap with ConvertPlaceholders
rows, err := db.Query(database.ConvertPlaceholders(
    "SELECT * FROM ticket WHERE queue_id = $1"), queueID)
```

### 3. Mixing Direct SQL with ConvertPlaceholders
```go
// ❌ Inconsistent - some queries wrapped, others not
db.QueryRow("SELECT COUNT(*) FROM ticket")  // Direct SQL - inconsistent
db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket WHERE tn = $1`), ticketNumber)

// ✅ Consistent - all queries use ConvertPlaceholders
db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket`))
db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket WHERE tn = $1`), ticketNumber)
```

## 🏆 Success Metrics

**Before (August 28, 2025)**:
- ❌ 500+ compilation errors with "$1" placeholder syntax
- ❌ "Error 1054 (42S22): Unknown column '$1'" on MySQL
- ❌ Complete inability to connect to OTRS MySQL databases

**After (August 29, 2025)**:
- ✅ Zero compilation errors
- ✅ Zero MySQL placeholder errors
- ✅ Full compatibility with OTRS MySQL databases
- ✅ Agent/tickets endpoint working without database errors
- ✅ Established mandatory database access patterns

## 🚀 Future Considerations

### 1. Migration to Repository Pattern
Once the immediate MySQL compatibility is stable, consider moving to a repository pattern:

```go
type TicketRepository interface {
    GetByID(id int) (*models.Ticket, error)
    GetByStatus(status string) ([]*models.Ticket, error)
    Create(ticket *models.Ticket) error
    Update(ticket *models.Ticket) error
}

type ticketRepository struct {
    db database.IDatabase
}

func (r *ticketRepository) GetByID(id int) (*models.Ticket, error) {
    // ConvertPlaceholders usage encapsulated in repository
    // Business logic never sees SQL
}
```

### 2. Query Builder Integration
For complex dynamic queries, consider a query builder that automatically uses ConvertPlaceholders:

```go
tickets, err := queryBuilder.
    Select("t.id, t.tn, t.title").
    From("ticket t").
    Join("ticket_state ts", "t.ticket_state_id = ts.id").
    Where("t.queue_id", "=", queueID).
    Where("t.status", "IN", []int{1, 2}).
    OrderBy("t.create_time", "DESC").
    Limit(50).
    Execute()
```

## 📚 Additional Resources

- **OTRS Database Schema**: Complete table definitions in `internal/database/schema/`
- **Migration Guide**: `docs/OTRS_MIGRATION_GUIDE.md`
- **Testing Patterns**: `internal/database/sql_compat_test.go`
- **Environment Setup**: `.env.example` with database configurations

---

**⚠️ CRITICAL REMINDER**: Every single database query MUST use `database.ConvertPlaceholders()`. No exceptions. This ensures MySQL compatibility and prevents the "$1" placeholder errors that took significant effort to fix.

*This pattern is now mandatory for all GOTRS development going forward.*

## Guardrails

- Pre-commit: scan for direct `$[0-9]+` usage not wrapped by `ConvertPlaceholders`.
- CI: run tests against PostgreSQL and MySQL matrices.
- Code review: reject PRs with unwrapped SQL or DAL/ORM introductions.


## 🔎 Case-Insensitive Search (ILIKE)

There are three safe ways to implement case-insensitive LIKE across PostgreSQL and MySQL:

- Use ILIKE in the SQL and wrap with `database.ConvertPlaceholders` or `database.ConvertQuery`.
  - `ConvertPlaceholders` already converts `ILIKE` to `LIKE` on MySQL.
  - `ConvertQuery` extends this with additional adaptations (e.g., type cast conversions).
- Use `database.GetAdapter().CaseInsensitiveLike(column, patternParam)` to generate a DB-specific expression.
  - PostgreSQL: `column ILIKE $n`
  - MySQL: `LOWER(column) LIKE LOWER($n)`
- Avoid manual LOWER(...) on values only; prefer adapter-based or ILIKE-based patterns for portability.

### Recommended patterns

```go
// Simple: write ILIKE and wrap
rows, err := db.Query(database.ConvertPlaceholders(`
    SELECT id, title FROM ticket
    WHERE title ILIKE $1 OR tn ILIKE $1
    ORDER BY create_time DESC
    LIMIT 50
`), "%"+search+"%")
```

```go
// Explicit: build with adapter for complex/dynamic queries
argCount++
p := fmt.Sprintf("$%d", argCount)
conds = append(conds, fmt.Sprintf("(%s OR %s)",
    database.GetAdapter().CaseInsensitiveLike("t.tn", p),
    database.GetAdapter().CaseInsensitiveLike("t.title", p),
))
args = append(args, "%"+search+"%")
```

```go
// ConvertQuery: when your SQL also uses PG-specific casts or you prefer one call
query := `SELECT * FROM users WHERE login ILIKE $1::text`
rows, err := db.Query(database.ConvertQuery(query), "%"+login+"%")
```

### Dynamic query building tips

- Keep placeholder indexing with `fmt.Sprintf("$%d", ...)` while appending conditions.
- Always pass the final SQL through `database.ConvertPlaceholders(...)` or `database.ConvertQuery(...)` at execution time.
- For multi-column search, prefer the adapter helper to avoid sprinkling `ILIKE` across fragments.

```go
query := "SELECT t.id, t.tn, t.title FROM ticket t WHERE 1=1"
var args []interface{}
arg := 0
if search != "" {
    arg++
    p := fmt.Sprintf("$%d", arg)
    query += " AND (" +
        database.GetAdapter().CaseInsensitiveLike("t.tn", p) +
        " OR " + database.GetAdapter().CaseInsensitiveLike("t.title", p) + ")"
    args = append(args, "%"+search+"%")
}
query += " ORDER BY t.create_time DESC LIMIT 50"
rows, err := db.Query(database.ConvertPlaceholders(query), args...)
```

### When to choose which

- `ConvertPlaceholders`: default choice; handles placeholders and `ILIKE`→`LIKE` for MySQL.
- `ConvertQuery`: use when your SQL also needs cross-DB tweaks beyond placeholders (casts, etc.).
- `GetAdapter().CaseInsensitiveLike`: best for dynamic builders or when you want explicit per-DB expressions.

### Anti-patterns

- Writing raw SQL without conversion wrappers.
- Applying `LOWER(value)` without applying it to the column on MySQL.
- Mixing different patterns within the same module — be consistent.

### Verification

- Run the guard: `scripts/tools/check-sql.sh --all` and ensure no critical issues.
- Optional: prefer adapter-based construction to reduce ILIKE warnings in dynamic builders.
