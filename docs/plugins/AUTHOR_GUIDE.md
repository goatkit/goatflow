# Plugin Author Guide

This guide covers everything you need to know to build plugins for GoatFlow.

## Overview

GoatFlow supports two plugin types:

| Type | Language | Use Case | Performance |
|------|----------|----------|-------------|
| **WASM** | TinyGo, Rust, AssemblyScript | Sandboxed, portable | Good |
| **gRPC** | Any (Go, Python, Node.js) | Full language features | Excellent |

## Quick Start

### Scaffold a New Plugin

```bash
# WASM plugin (TinyGo)
gk plugin init my-plugin --type wasm

# gRPC plugin (Go)
gk plugin init my-plugin --type grpc
```

This creates:
```
my-plugin/
├── manifest.json      # Plugin metadata
├── main.go            # Entry point
├── build.sh           # Build script
└── README.md          # Documentation
```

### Build & Install

```bash
cd my-plugin
./build.sh                          # Compiles to .wasm or binary
cp my-plugin.wasm /path/to/plugins/ # Install
```

## Plugin Manifest

Every plugin needs a `manifest.json`:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "What this plugin does",
  "author": "Your Name",
  "license": "Apache-2.0",
  "homepage": "https://github.com/you/my-plugin",
  
  "routes": [
    {
      "method": "GET",
      "path": "/api/plugins/my-plugin/data",
      "handler": "get_data",
      "middleware": ["auth"],
      "description": "Returns plugin data"
    }
  ],
  
  "widgets": [
    {
      "id": "my-widget",
      "title": "My Widget",
      "handler": "render_widget",
      "location": "agent_home",
      "size": "medium"
    }
  ],
  
  "menu_items": [
    {
      "id": "my-menu",
      "label": "My Plugin",
      "icon": "puzzle",
      "path": "/admin/my-plugin",
      "location": "admin"
    }
  ],
  
  "jobs": [
    {
      "id": "daily-sync",
      "schedule": "0 0 * * *",
      "handler": "daily_sync",
      "description": "Syncs data daily"
    }
  ],
  
  "i18n": {
    "namespace": "my_plugin",
    "languages": ["en", "de", "es"],
    "translations": {
      "en": {
        "title": "My Plugin",
        "description": "Plugin description"
      },
      "de": {
        "title": "Mein Plugin",
        "description": "Plugin-Beschreibung"
      }
    }
  },
  
  "permissions": ["db:read", "http:external"],
  "min_host_version": "1.0.0"
}
```

## Host API

Plugins interact with GoatFlow through the Host API:

### Database

```go
// Query (read-only)
result, err := host.DBQuery(ctx, "SELECT id, name FROM queues WHERE valid_id = ?", 1)

// Execute (write)
result, err := host.DBExec(ctx, "UPDATE tickets SET title = ? WHERE id = ?", "New Title", 123)
```

### HTTP Requests

```go
// External API calls
resp, err := host.HTTPRequest(ctx, "GET", "https://api.example.com/data", nil, map[string]string{
    "Authorization": "Bearer token",
})
```

### Key-Value Store

```go
// Plugin-scoped storage
host.KVSet(ctx, "last_sync", time.Now().Format(time.RFC3339))
value, _ := host.KVGet(ctx, "last_sync")
host.KVDelete(ctx, "old_key")
```

### Cache

```go
// Fast caching with TTL
host.CacheSet(ctx, "stats", statsJSON, 300) // 5 min TTL
data, found := host.CacheGet(ctx, "stats")
```

### Logging

```go
host.Log(ctx, "info", "Processing ticket", map[string]any{
    "ticket_id": 123,
    "action": "update",
})
```

### Configuration

```go
// Read host config values
appName := host.ConfigGet(ctx, "app.name")
dbHost := host.ConfigGet(ctx, "database.host")
```

### Events

```go
// Emit events for other plugins/host
host.EmitEvent(ctx, "my_plugin.data_updated", map[string]any{
    "record_id": 456,
})
```

### Plugin-to-Plugin Calls

```go
// Call another plugin's function
result, err := host.CallPlugin(ctx, "other-plugin", "get_stats", args)
```

## Routes

Register HTTP endpoints your plugin handles:

```json
{
  "routes": [
    {
      "method": "GET",
      "path": "/api/plugins/stats/overview",
      "handler": "stats_overview",
      "middleware": ["auth", "admin"]
    },
    {
      "method": "POST",
      "path": "/api/plugins/stats/refresh",
      "handler": "stats_refresh",
      "middleware": ["auth", "admin"]
    }
  ]
}
```

Handler receives request context:
```go
func handleStatsOverview(ctx context.Context, req Request) ([]byte, error) {
    // req.Method, req.Path, req.Headers, req.Body, req.Query
    
    stats := getStats()
    return json.Marshal(stats)
}
```

### Middleware Options

- `auth` - Requires authenticated user
- `admin` - Requires admin role
- `customer` - Requires customer portal access
- `api` - API-only (no session)

## Widgets

Dashboard widgets appear on agent/admin home:

```json
{
  "widgets": [
    {
      "id": "ticket-stats",
      "title": "Ticket Statistics",
      "handler": "render_ticket_stats",
      "location": "agent_home",
      "size": "large"
    }
  ]
}
```

Handler returns HTML:
```go
func renderTicketStats(ctx context.Context, req Request) ([]byte, error) {
    stats := getStats()
    html := fmt.Sprintf(`
        <div class="stats">
            <div class="stat">
                <div class="stat-title">Open Tickets</div>
                <div class="stat-value">%d</div>
            </div>
        </div>
    `, stats.OpenCount)
    return []byte(html), nil
}
```

### Widget Locations

- `agent_home` - Agent dashboard
- `admin_home` - Admin dashboard  
- `customer_home` - Customer portal dashboard
- `ticket_sidebar` - Ticket detail sidebar

### Widget Sizes

- `small` - 1/4 width
- `medium` - 1/2 width
- `large` - 3/4 width
- `full` - Full width

## Scheduled Jobs

Run tasks on a schedule:

```json
{
  "jobs": [
    {
      "id": "hourly-cleanup",
      "schedule": "0 * * * *",
      "handler": "cleanup_old_data",
      "description": "Removes stale cache entries"
    }
  ]
}
```

Schedule format is standard cron:
```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sun=0)
│ │ │ │ │
* * * * *
```

## Internationalization (i18n)

Provide translations for your plugin:

```json
{
  "i18n": {
    "namespace": "my_plugin",
    "translations": {
      "en": {
        "widget_title": "My Widget",
        "no_data": "No data available"
      },
      "de": {
        "widget_title": "Mein Widget",
        "no_data": "Keine Daten verfügbar"
      }
    }
  }
}
```

Access in handlers:
```go
func renderWidget(ctx context.Context, req Request) ([]byte, error) {
    lang := req.Headers["Accept-Language"]
    title := host.Translate(ctx, "my_plugin.widget_title", lang)
    // ...
}
```

## Packaging

### ZIP Package Structure

```
my-plugin.zip
├── manifest.json     # Required
├── plugin.wasm       # Required (WASM plugins)
├── assets/           # Optional static files
│   ├── styles.css
│   └── script.js
└── i18n/             # Optional translation files
    ├── en.json
    └── de.json
```

### Create Package

```bash
# From plugin directory
zip -r my-plugin.zip manifest.json plugin.wasm assets/ i18n/
```

### Install Package

Upload via Admin → Plugins → Upload Plugin, or:

```bash
cp my-plugin.zip /path/to/goatflow/plugins/
# GoatFlow auto-extracts on next load (or immediately with hot reload)
```

## Best Practices

### 1. Handle Errors Gracefully

```go
result, err := host.DBQuery(ctx, query, args...)
if err != nil {
    host.Log(ctx, "error", "Query failed", map[string]any{"error": err.Error()})
    return nil, fmt.Errorf("database error: %w", err)
}
```

### 2. Use Caching

```go
// Cache expensive operations
cached, found := host.CacheGet(ctx, "expensive_stats")
if found {
    return cached, nil
}

stats := computeExpensiveStats()
host.CacheSet(ctx, "expensive_stats", stats, 300)
return stats, nil
```

### 3. Respect Permissions

Only request permissions you actually need:
```json
{
  "permissions": ["db:read"]  // Don't request db:write if you only read
}
```

### 4. Version Your API

```json
{
  "routes": [
    {"path": "/api/plugins/my-plugin/v1/data", "handler": "get_data_v1"},
    {"path": "/api/plugins/my-plugin/v2/data", "handler": "get_data_v2"}
  ]
}
```

### 5. Document Your Plugin

Include a README.md with:
- What the plugin does
- Configuration options
- API endpoints
- Widget descriptions
- Changelog

## Debugging

### View Logs

Admin → Plugins → View Logs

Filter by plugin name to see only your plugin's logs.

### Enable Debug Logging

```go
host.Log(ctx, "debug", "Processing item", map[string]any{
    "item_id": id,
    "step": "validation",
})
```

### Hot Reload (Development)

Set `GOATFLOW_PLUGIN_HOT_RELOAD=true` to auto-reload plugins when files change.

## Example Plugins

See the `plugins/` directory for examples:

- `plugins/hello/` - Simple WASM plugin
- `plugins/stats/` - Dashboard widgets with i18n
- `plugins/example-grpc/` - gRPC plugin template

## Getting Help

- [Host API Reference](./HOST_API.md)
- [WASM Plugin Tutorial](./WASM_TUTORIAL.md)
- [gRPC Plugin Tutorial](./GRPC_TUTORIAL.md)
- [GitHub Issues](https://github.com/goatkit/goatflow/issues)
