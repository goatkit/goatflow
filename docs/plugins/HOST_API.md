# Host API Reference

The Host API is how plugins interact with GoatFlow. All functions are available to both WASM and gRPC plugins. The interface is defined in `pkg/plugin/plugin.go`.

Every plugin receives a **SandboxedHostAPI** that enforces per-plugin permissions and rate limits. See the [Sandboxing & Permissions](#sandboxing--permissions) section for details.

## Database

### DBQuery

Execute a read-only SQL query.

```go
DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error)
```

**Parameters:**
- `query` — SQL SELECT statement with `?` placeholders
- `args` — Values for placeholders

**Returns:**
- Array of row maps (column name → value). `[]byte` values are converted to strings.
- Error if query fails or permission denied

**SQL Portability:**
Queries are automatically processed through `ConvertPlaceholders()` for MySQL/PostgreSQL compatibility. Always use `?` placeholders — never use PostgreSQL-style `$1`.

**Multi-Database:**
Prefix query with `@dbname:` to target a named database (e.g., `@analytics:SELECT...`). Default database is used if no prefix.

**Example:**
```go
rows, err := host.DBQuery(ctx,
    "SELECT id, title, state_id FROM tickets WHERE queue_id = ? AND state_id IN (?, ?)",
    5, 1, 4)
// rows = [{"id": 1, "title": "Issue", "state_id": 1}, ...]
```

**Permission required:** `db` with `read` (or `readwrite`) access

---

### DBExec

Execute a write SQL statement (INSERT, UPDATE, DELETE).

```go
DBExec(ctx context.Context, query string, args ...any) (int64, error)
```

**Parameters:**
- `query` — SQL statement with `?` placeholders
- `args` — Values for placeholders

**Returns:**
- `int64` — Number of rows affected
- Error if execution fails or permission denied

**DDL Protection:** Plugins without `write` access are blocked from executing DDL statements (DROP, ALTER, TRUNCATE, CREATE, GRANT, REVOKE).

**Table Whitelisting:** If the plugin's `db` permission has a scope (e.g. `["tickets", "queue"]`), table names are extracted from the query and validated against the allowlist. Queries touching tables outside the scope are rejected with an error.

**Example:**
```go
affected, err := host.DBExec(ctx,
    "UPDATE tickets SET priority_id = ? WHERE id = ?",
    3, 123)
// affected = 1
```

**Permission required:** `db` with `write` (or `readwrite`) access

---

## Cache

Fast in-memory caching via Redis/Valkey with TTL.

**Important:** Cache keys are **automatically namespaced** per plugin. When you set key `"stats"`, it's stored as `"plugin:my-plugin:stats"`. This prevents cross-plugin key collisions — you don't need to prefix keys yourself.

### CacheGet

Retrieve a cached value.

```go
CacheGet(ctx context.Context, key string) ([]byte, bool, error)
```

**Returns:**
- `[]byte` — Value if found
- `bool` — `true` if found and not expired
- `error` — Error if permission denied

**Example:**
```go
if data, found, err := host.CacheGet(ctx, "dashboard_stats"); found {
    return data, nil
}
```

**Permission required:** `cache` with `read` (or `readwrite`) access

---

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

**Permission required:** `cache` with `write` (or `readwrite`) access

---

### CacheDelete

Remove a cached value.

```go
CacheDelete(ctx context.Context, key string) error
```

**Permission required:** `cache` with `write` (or `readwrite`) access

---

## HTTP

### HTTPRequest

Make an outbound HTTP request.

```go
HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error)
```

**Parameters:**
- `method` — HTTP method (GET, POST, PUT, DELETE, etc.)
- `url` — Full URL including protocol
- `headers` — Request headers (may be nil)
- `body` — Request body (nil for GET)

**Returns:**
- `int` — HTTP status code
- `[]byte` — Response body
- `error` — Error if request fails or permission denied

**Example:**
```go
statusCode, body, err := host.HTTPRequest(ctx, "POST", "https://api.example.com/webhook",
    map[string]string{
        "Content-Type":  "application/json",
        "Authorization": "Bearer secret",
    },
    []byte(`{"event": "ticket.created"}`))
```

**Permission required:** `http`

**URL Filtering:** If the plugin's policy specifies HTTP scope patterns (e.g., `["*.example.com"]`), only matching URLs are allowed. Wildcard `*.example.com` matches any subdomain.

**Notes:**
- Default timeout: 30 seconds (enforced by host)

---

## Email

### SendEmail

Send an email using the host's configured email provider.

```go
SendEmail(ctx context.Context, to, subject, body string, html bool) error
```

**Parameters:**
- `to` — Recipient email address
- `subject` — Email subject
- `body` — Email body (plain text or HTML)
- `html` — `true` for HTML body

**Example:**
```go
err := host.SendEmail(ctx, "user@example.com", "Report Ready",
    "<h1>Your report is ready</h1>", true)
```

**Permission required:** `email`

**Domain Scoping:** If the plugin's `email` permission has a scope (e.g. `["@example.com", "@mycompany.com"]`), only recipients at those domains are allowed. Domain patterns match the email suffix — `@example.com` allows `user@example.com`, `admin@example.com`, etc.

**Rate Limiting:** Email sending is rate-limited to 10 emails per minute per plugin (sliding window), independent of the general rate limits.

---

## Logging

### Log

Write a structured log entry. **Always allowed** — no permission check.

```go
Log(ctx context.Context, level string, message string, fields map[string]any)
```

**Levels:** `debug`, `info`, `warn`, `error`

The plugin name is automatically added to the `fields` map. Logs are written to both the structured logger and the in-memory log buffer (visible in Admin → Plugins → View Logs).

**Example:**
```go
host.Log(ctx, "info", "Processing webhook", map[string]any{
    "webhook_id": "wh_123",
    "event_type": "ticket.created",
})
```

---

## Configuration

### ConfigGet

Read host configuration values.

```go
ConfigGet(ctx context.Context, key string) (string, error)
```

**Available Keys:**
- `app.name` — Application name
- `app.timezone` — Timezone
- `app.env` — Environment (development, production, etc.)

**Permission required:** `config` with `read` access

**Sensitive Key Blocking:** The following key patterns are blocked by default and return an error, even with `config` permission: `database.*`, `db.*`, `mysql.*`, `postgres.*`, `smtp.*`, `mail.*`, `secret`, `password`, `credential`, `token`, `key`, `private`, `auth`, `session`, `cookie`, `ldap.*`, `oauth.*`, `saml.*`, `aws.*`, `gcp.*`, `azure.*`, `cloud.*`. If the `config` permission has a scope, only keys matching the scope patterns are allowed (overrides the default blacklist).

---

## i18n

### Translate

Translate a key to the current locale. **Always allowed** — no permission check.

```go
Translate(ctx context.Context, key string, args ...any) string
```

The language is determined from the request context (set via `PluginLanguageKey`). Falls back to `"en"` if not set.

**Example:**
```go
title := host.Translate(ctx, "my_plugin.widget_title")
```

---

## Plugin Interop

### CallPlugin

Call a function in another plugin.

```go
CallPlugin(ctx context.Context, pluginName string, fn string, args json.RawMessage) (json.RawMessage, error)
```

**Example:**
```go
args, _ := json.Marshal(map[string]any{"period": "day"})
result, err := host.CallPlugin(ctx, "stats", "get_ticket_stats", args)
```

**Permission required:** `plugin_call`

**Scope:** If the plugin's policy specifies a `plugin_call` scope (e.g., `["stats"]`), only listed plugins can be called. Without scope, all plugins are callable.

**Call Depth Limit:** Plugin-to-plugin call chains are limited to a maximum depth of 10. This prevents infinite recursion when Plugin A calls Plugin B which calls Plugin A. The depth is tracked via context and incremented on each cross-plugin call. Exceeding the limit returns: `plugin call depth exceeded (max 10): pluginA -> pluginB`

**Notes:**
- Target plugin must be loaded and enabled
- Lazy loading is attempted if the target isn't loaded yet
- Caller plugin name is tracked for better error messages and stamped by the host (plugins can't impersonate each other)

---

## Sandboxing & Permissions

Every plugin receives a `SandboxedHostAPI` that wraps the real HostAPI with enforcement:

### Permission Enforcement

Each HostAPI call checks the plugin's `ResourcePolicy` before executing. If the permission isn't granted, the call returns an error like:

```
plugin "my-plugin": database write access not granted
```

### Rate Limiting

Three sliding-window rate limiters (configurable per plugin via policy):

| Limiter | Default | Scope |
|---------|---------|-------|
| DB queries/min | 600 | DBQuery + DBExec |
| HTTP requests/min | 60 | HTTPRequest |
| Calls/sec | 100 | All Call() invocations |

Exceeding a limit returns an error:
```
plugin "my-plugin": DB query rate limit exceeded
```

### Resource Accounting

All operations are counted via atomic counters:
- `DBQueries`, `DBExecs`, `CacheOps`, `HTTPRequests`, `Calls`, `Errors`
- `LastCallAt` timestamp

Admins can view these stats via the plugin management API.

### Policy Status

| Status | Effect |
|--------|--------|
| `pending_review` | Default for new plugins — restrictive permissions |
| `approved` | Admin-granted permissions |
| `restricted` | Limited by admin |
| `blocked` | All HostAPI calls denied |

### Default Policy

New plugins receive `DefaultResourcePolicy`:
- DB read-only + cache read/write
- 256 MB memory, 30s call timeout
- 100 calls/sec, 600 DB queries/min, 60 HTTP requests/min
- Status: `pending_review`

---

## Error Handling

All Host API functions can return errors. Always check:

```go
result, err := host.DBQuery(ctx, query, args...)
if err != nil {
    host.Log(ctx, "error", "Query failed", map[string]any{"error": err.Error()})
    return nil, fmt.Errorf("database error: %w", err)
}
```

### Common Errors

| Error | Cause |
|-------|-------|
| `permission denied` / `access not granted` | Missing permission in policy |
| `rate limit exceeded` | Too many calls in the time window |
| `DDL statements not permitted` | Write SQL without write permission |
| `HTTP access to "..." not permitted` | URL not in allowed scope |
| `not permitted to call plugin "..."` | Target not in plugin_call scope |
| `plugin "..." is disabled` | Target plugin disabled by admin |
| `access to table "..." not permitted` | Table not in DB permission scope |
| `plugin call depth exceeded (max 10)` | Too many nested plugin-to-plugin calls |
| `email recipient "..." not allowed` | Recipient domain not in email scope |
| `email rate limit exceeded` | More than 10 emails/minute |
| `config access not granted` / blocked key | Key matches sensitive pattern or not in scope |

---

## Context

The `context.Context` passed to handlers carries:

- Request timeout/deadline
- Language preference (via `PluginLanguageKey`)
- Caller plugin name (via `PluginCallerKey`, for plugin-to-plugin calls)
