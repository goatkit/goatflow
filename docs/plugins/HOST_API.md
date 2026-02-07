# Host API Reference

The Host API is how plugins interact with GoatFlow. All functions are available to both WASM and gRPC plugins.

## Database

### DBQuery

Execute a read-only SQL query.

```go
DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error)
```

**Parameters:**
- `query` - SQL SELECT statement with `?` placeholders
- `args` - Values for placeholders

**Returns:**
- Array of row objects (column name → value)
- Error if query fails

**SQL Portability:**
Queries are automatically processed through `ConvertPlaceholders()` for MySQL/PostgreSQL compatibility. Always use `?` placeholders — never use PostgreSQL-style `$1` or MySQL-specific syntax.

**Example:**
```go
rows, err := host.DBQuery(ctx, 
    "SELECT id, title, state_id FROM tickets WHERE queue_id = ? AND state_id IN (?, ?)",
    5, 1, 4)
// rows = [{"id": 1, "title": "Issue", "state_id": 1}, ...]
```

**Permissions:** `db:read`

---

### DBExec

Execute a write SQL statement (INSERT, UPDATE, DELETE).

```go
DBExec(ctx context.Context, query string, args ...any) (DBResult, error)
```

**Parameters:**
- `query` - SQL statement with `?` placeholders
- `args` - Values for placeholders

**Returns:**
- `DBResult` with `LastInsertId` and `RowsAffected`
- Error if execution fails

**SQL Portability:**
Queries are automatically processed through `ConvertPlaceholders()` for MySQL/PostgreSQL compatibility. Always use `?` placeholders — never use PostgreSQL-style `$1` or MySQL-specific syntax.

**Example:**
```go
result, err := host.DBExec(ctx,
    "UPDATE tickets SET priority_id = ? WHERE id = ?",
    3, 123)
// result.RowsAffected = 1
```

**Permissions:** `db:write`

---

## HTTP

### HTTPRequest

Make an HTTP request to an external service.

```go
HTTPRequest(ctx context.Context, method, url string, body []byte, headers map[string]string) (HTTPResponse, error)
```

**Parameters:**
- `method` - HTTP method (GET, POST, PUT, DELETE, etc.)
- `url` - Full URL including protocol
- `body` - Request body (nil for GET)
- `headers` - Request headers

**Returns:**
- `HTTPResponse` with `StatusCode`, `Body`, `Headers`
- Error if request fails

**Example:**
```go
resp, err := host.HTTPRequest(ctx, "POST", "https://api.example.com/webhook",
    []byte(`{"event": "ticket.created"}`),
    map[string]string{
        "Content-Type": "application/json",
        "Authorization": "Bearer secret",
    })
if resp.StatusCode == 200 {
    // Success
}
```

**Permissions:** `http:external`

**Notes:**
- Timeout is enforced by host (default 30s)
- Only HTTPS URLs allowed in production
- Response body limited to 10MB

---

## Key-Value Store

Plugin-scoped persistent storage.

### KVSet

Store a value.

```go
KVSet(ctx context.Context, key string, value string) error
```

**Example:**
```go
host.KVSet(ctx, "config.api_key", "sk-123456")
host.KVSet(ctx, "last_sync", time.Now().Format(time.RFC3339))
```

---

### KVGet

Retrieve a value.

```go
KVGet(ctx context.Context, key string) (string, error)
```

**Returns:**
- Value if found
- Empty string and error if not found

**Example:**
```go
apiKey, err := host.KVGet(ctx, "config.api_key")
if err != nil {
    // Key not found
}
```

---

### KVDelete

Remove a value.

```go
KVDelete(ctx context.Context, key string) error
```

---

### KVList

List keys with optional prefix.

```go
KVList(ctx context.Context, prefix string) ([]string, error)
```

**Example:**
```go
keys, _ := host.KVList(ctx, "config.")
// ["config.api_key", "config.endpoint", ...]
```

**Notes:**
- Keys are namespaced per plugin automatically
- Max key length: 255 bytes
- Max value length: 1MB
- Persisted to database

---

## Cache

Fast in-memory caching with TTL.

### CacheSet

Store a value with expiration.

```go
CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error
```

**Example:**
```go
stats, _ := json.Marshal(computeStats())
host.CacheSet(ctx, "dashboard_stats", stats, 300) // 5 minutes
```

---

### CacheGet

Retrieve a cached value.

```go
CacheGet(ctx context.Context, key string) ([]byte, bool)
```

**Returns:**
- Value and `true` if found and not expired
- `nil` and `false` if not found or expired

**Example:**
```go
if data, found := host.CacheGet(ctx, "dashboard_stats"); found {
    return data, nil
}
// Cache miss - compute and cache
```

---

### CacheDelete

Remove a cached value.

```go
CacheDelete(ctx context.Context, key string) error
```

**Notes:**
- Cache is shared across plugin instances
- Keys are namespaced per plugin
- Not persisted across restarts
- Max value size: 1MB

---

## Logging

### Log

Write a structured log entry.

```go
Log(ctx context.Context, level string, message string, fields map[string]any)
```

**Levels:**
- `debug` - Detailed debugging info
- `info` - General information
- `warn` - Warning conditions
- `error` - Error conditions

**Example:**
```go
host.Log(ctx, "info", "Processing webhook", map[string]any{
    "webhook_id": "wh_123",
    "event_type": "ticket.created",
})

host.Log(ctx, "error", "External API failed", map[string]any{
    "url": "https://api.example.com",
    "status": 500,
    "error": err.Error(),
})
```

**Notes:**
- Logs visible in Admin → Plugins → View Logs
- Plugin name automatically added
- Fields are JSON-serialized

---

## Configuration

### ConfigGet

Read host configuration values.

```go
ConfigGet(ctx context.Context, key string) string
```

**Available Keys:**
- `app.name` - Application name
- `app.url` - Base URL
- `database.host` - Database hostname
- `database.name` - Database name

**Example:**
```go
appName := host.ConfigGet(ctx, "app.name")
baseURL := host.ConfigGet(ctx, "app.url")
```

**Notes:**
- Read-only access
- Sensitive values (passwords) are redacted

---

## Events

### EmitEvent

Emit an event for other plugins or the host to handle.

```go
EmitEvent(ctx context.Context, eventType string, payload map[string]any) error
```

**Example:**
```go
host.EmitEvent(ctx, "my_plugin.sync_completed", map[string]any{
    "records_synced": 150,
    "duration_ms": 2340,
})
```

**Standard Events:**
- `ticket.created` - New ticket
- `ticket.updated` - Ticket modified
- `ticket.state_changed` - State transition
- `article.created` - New article
- `user.login` - User logged in

---

## Plugin Interop

### CallPlugin

Call a function in another plugin.

```go
CallPlugin(ctx context.Context, pluginName string, function string, args []byte) ([]byte, error)
```

**Example:**
```go
// Call the stats plugin
args, _ := json.Marshal(map[string]any{"period": "day"})
result, err := host.CallPlugin(ctx, "stats", "get_ticket_stats", args)
if err != nil {
    return nil, err
}
var stats TicketStats
json.Unmarshal(result, &stats)
```

**Notes:**
- Target plugin must be loaded and enabled
- Function must be exported by target plugin
- Args and result are JSON-encoded

---

## Response Helpers

### Request Object

Passed to route/widget handlers:

```go
type Request struct {
    Method  string            // HTTP method
    Path    string            // Request path
    Headers map[string]string // Request headers
    Query   map[string]string // Query parameters
    Body    []byte            // Request body
}
```

### Response Format

Handlers return `([]byte, error)`:

```go
// JSON response
func handleGetData(ctx context.Context, req Request) ([]byte, error) {
    data := getData()
    return json.Marshal(data)
}

// HTML response (widgets)
func renderWidget(ctx context.Context, req Request) ([]byte, error) {
    html := `<div class="widget">...</div>`
    return []byte(html), nil
}

// Error response
func handleError(ctx context.Context, req Request) ([]byte, error) {
    return nil, fmt.Errorf("something went wrong")
}
```

---

## Permissions

Plugins must declare required permissions in `manifest.json`:

| Permission | Description |
|------------|-------------|
| `db:read` | Read database |
| `db:write` | Write database |
| `http:external` | External HTTP requests |
| `cache:read` | Read cache |
| `cache:write` | Write cache |
| `kv:read` | Read key-value store |
| `kv:write` | Write key-value store |
| `events:emit` | Emit events |
| `plugins:call` | Call other plugins |

**Example:**
```json
{
  "permissions": ["db:read", "cache:read", "cache:write"]
}
```

---

## Error Handling

All Host API functions can return errors. Always check:

```go
result, err := host.DBQuery(ctx, query)
if err != nil {
    host.Log(ctx, "error", "Query failed", map[string]any{
        "query": query,
        "error": err.Error(),
    })
    return nil, fmt.Errorf("database error: %w", err)
}
```

### Common Errors

| Error | Cause |
|-------|-------|
| `permission denied` | Missing permission in manifest |
| `timeout` | Operation exceeded time limit |
| `not found` | Resource doesn't exist |
| `invalid argument` | Bad parameter value |

---

## Context

The `context.Context` passed to handlers contains:

- Request timeout/deadline
- User information (if authenticated route)
- Language preference (for i18n)
- Trace ID (for logging correlation)

```go
// Get current user (authenticated routes only)
userID := ctx.Value("user_id")

// Get language
lang := ctx.Value("language") // "en", "de", etc.
```
