# Plugin Author Guide

This guide covers everything you need to know to build plugins for GoatFlow.

## Overview

GoatFlow supports two plugin runtimes:

| Type | Language | Use Case | Performance |
|------|----------|----------|-------------|
| **WASM** | TinyGo, Rust, AssemblyScript | Sandboxed, portable | Good |
| **gRPC** | Go (native) | Full language features, I/O-heavy | Excellent |

**Note:** gRPC plugins use HashiCorp go-plugin with net/rpc — no protoc or proto files needed.

## Quick Start

### Scaffold a New Plugin

```bash
# WASM plugin (TinyGo)
gk init my-plugin --type wasm

# gRPC plugin (Go)
gk init my-plugin --type grpc
```

### Build & Install

```bash
cd my-plugin
./build.sh                          # Compiles to .wasm or binary
cp -r . /path/to/goatflow/plugins/  # Install
```

## Plugin Metadata

### WASM Plugins

WASM plugins self-describe via their `GKRegister()` export function — no separate manifest file is needed. The returned `GKRegistration` struct declares all capabilities.

### gRPC Plugins

gRPC plugins require a `plugin.yaml` file alongside the binary:

```yaml
name: my-plugin
version: "1.0.0"
runtime: grpc
binary: my-plugin
resources:
  memory_mb: 512
  call_timeout: 30s
  init_timeout: 10s
  permissions:
    - type: db
      access: readwrite
    - type: cache
      access: readwrite
    - type: http
      scope: ["api.example.com", "*.cdn.example.com"]
    - type: email
    - type: plugin_call
      scope: ["stats"]
```

The plugin also self-describes via `GKRegister()` for routes, widgets, jobs, menu items, i18n, and error codes.

### Directory Layout

```
plugins/
├── stats.wasm              # WASM plugin (single file)
└── my-plugin/              # gRPC plugin (directory)
    ├── plugin.yaml         # Required manifest
    └── my-plugin           # Executable binary
```

## Registration (GKRegistration)

Both WASM and gRPC plugins return a `GKRegistration` struct from `GKRegister()`:

```go
return &plugin.GKRegistration{
    Name:        "my-plugin",
    Version:     "1.0.0",
    Description: "What this plugin does",
    Author:      "Your Name",
    License:     "Apache-2.0",
    Homepage:    "https://github.com/you/my-plugin",

    Routes: []plugin.RouteSpec{
        {
            Method:      "GET",
            Path:        "/api/plugins/my-plugin/data",
            Handler:     "get_data",
            Middleware:  []string{"auth"},
            Description: "Returns plugin data",
        },
    },

    Widgets: []plugin.WidgetSpec{
        {
            ID:       "my-widget",
            Title:    "My Widget",
            Handler:  "render_widget",
            Location: "agent_home",
            Size:     "medium",
        },
    },

    MenuItems: []plugin.MenuItemSpec{
        {
            ID:       "my-menu",
            Label:    "My Plugin",
            Icon:     "puzzle",
            Path:     "/admin/my-plugin",
            Location: "admin",
        },
    },

    Jobs: []plugin.JobSpec{
        {
            ID:          "daily-sync",
            Schedule:    "0 0 * * *",
            Handler:     "daily_sync",
            Description: "Syncs data daily",
            Enabled:     true,
            Timeout:     "5m",
        },
    },

    I18n: &plugin.I18nSpec{
        Namespace: "my_plugin",
        Languages: []string{"en", "de"},
        Translations: map[string]map[string]string{
            "en": {"title": "My Plugin", "description": "Plugin description"},
            "de": {"title": "Mein Plugin", "description": "Plugin-Beschreibung"},
        },
    },

    ErrorCodes: []plugin.ErrorCodeSpec{
        {Code: "sync_failed", Message: "Sync operation failed", HTTPStatus: 500},
    },

    // Navigation control — hide default menu items and set a custom landing page
    HideMenuItems: []string{"dashboard", "tickets", "queues"},
    LandingPage:   "/my-plugin",

    Resources: &plugin.ResourceRequest{
        MemoryMB:    512,
        CallTimeout: "30s",
        Permissions: []plugin.Permission{
            {Type: "db", Access: "read"},
            {Type: "cache", Access: "readwrite"},
        },
    },
}
```

## Host API

Plugins interact with GoatFlow through the HostAPI interface. See [HOST_API.md](./HOST_API.md) for the full reference.

### Database

```go
// Query (read)
rows, err := host.DBQuery(ctx, "SELECT id, name FROM queues WHERE valid_id = ?", 1)

// Execute (write) — returns rows affected
affected, err := host.DBExec(ctx, "UPDATE tickets SET title = ? WHERE id = ?", "New Title", 123)
```

### Cache

```go
// Cache with TTL (keys are auto-namespaced per plugin)
host.CacheSet(ctx, "stats", statsJSON, 300) // 5 min TTL
data, found, err := host.CacheGet(ctx, "stats")
host.CacheDelete(ctx, "stats")
```

### HTTP Requests

```go
statusCode, body, err := host.HTTPRequest(ctx, "POST", "https://api.example.com/webhook",
    map[string]string{"Content-Type": "application/json", "Authorization": "Bearer token"},
    []byte(`{"event": "ticket.created"}`))
```

### Email

```go
err := host.SendEmail(ctx, "user@example.com", "Subject", "<h1>Hello</h1>", true)
```

### Logging

```go
host.Log(ctx, "info", "Processing ticket", map[string]any{
    "ticket_id": 123,
    "action":    "update",
})
```

### Configuration

```go
appName, err := host.ConfigGet(ctx, "app.name")
```

### i18n

```go
title := host.Translate(ctx, "my_plugin.title")
```

### Plugin-to-Plugin Calls

```go
result, err := host.CallPlugin(ctx, "other-plugin", "get_stats", args)
```

## Resources & Permissions

### Declaring Resources

Plugins declare their resource needs in `ResourceRequest` (via `GKRegistration.Resources` or `plugin.yaml`):

```go
Resources: &plugin.ResourceRequest{
    MemoryMB:    512,
    CallTimeout: "30s",
    Permissions: []plugin.Permission{
        {Type: "db", Access: "read"},
        {Type: "cache", Access: "readwrite"},
        {Type: "http", Scope: []string{"*.example.com"}},
        {Type: "email"},
        {Type: "plugin_call", Scope: []string{"stats"}},
    },
}
```

### Permission Types

| Type | Access | Scope |
|------|--------|-------|
| `db` | `read`, `write`, `readwrite` | Table patterns (e.g. `["tickets", "queue"]`) |
| `cache` | `read`, `write`, `readwrite` | Auto-namespaced |
| `http` | — | URL patterns (`*.example.com`) |
| `email` | — | Domain patterns (e.g. `["@example.com"]`) |
| `config` | `read` | — |
| `plugin_call` | — | Plugin names (`["stats", "analytics"]`) |

### What Happens at Registration

1. Plugin registers → gets `DefaultResourcePolicy` (restrictive: DB read-only, cache RW, rate limited)
2. Status is `pending_review` until an admin approves
3. Admin can grant more (or fewer) permissions via `ResourcePolicy`
4. The `SandboxedHostAPI` enforces the granted policy on every call

### Rate Limits

Default rate limits (configurable by admin per plugin):

- 100 calls/second
- 600 DB queries/minute
- 60 HTTP requests/minute

Exceeding limits returns an error — the call is not executed.

## Security & Sandbox

Every plugin gets a `SandboxedHostAPI` wrapper that enforces:

- **Permission checks** on every HostAPI call
- **DDL blocking** — no DROP/ALTER/TRUNCATE/CREATE without DB write permission
- **SQL table whitelisting** — if your `db` permission has a scope (e.g. `["tickets", "queue"]`), queries touching other tables are rejected. Table names are parsed from the SQL automatically
- **HTTP URL filtering** — outbound requests checked against allowed patterns
- **Cache key namespacing** — keys prefixed with `plugin:<name>:` automatically
- **Plugin call scoping** — can only call plugins listed in scope
- **Call depth limiting** — plugin-to-plugin call chains are limited to depth 10 (prevents infinite recursion)
- **Config key restrictions** — sensitive keys are blocked by default (database credentials, passwords, secrets, tokens, auth config, cloud provider keys). If your `config` permission has a scope, only matching keys are accessible
- **Email domain scoping** — if your `email` permission has a scope (e.g. `["@mycompany.com"]`), recipients are validated against allowed domains. Rate limited to 10 emails/minute
- **Caller identity** — the host stamps your plugin name on all calls; you cannot impersonate other plugins
- **Blocked status** — admin can block a plugin, killing all HostAPI access
- **Rate limiting** — sliding window limiters per resource type
- **Resource accounting** — atomic counters for all operations, visible to admin

### Plugin Signing

Plugin binaries can be signed with ed25519 keys for tamper detection:

1. **Generate a key pair** — use `signing.GenerateKeyPair()` or the CLI
2. **Sign your binary** — `signing.SignBinary("my-plugin", "my-plugin.sig", privateKey)` creates a `.sig` file containing the hex-encoded signature of the binary's SHA-256 hash
3. **Ship both files** — place `my-plugin` and `my-plugin.sig` in your plugin directory
4. **Verification** — the host checks the signature against its list of trusted public keys

Signing is currently opt-in. Set `GOATFLOW_REQUIRE_SIGNATURES=1` on the host to enforce signature verification. Unsigned plugins will still load without this flag but generate a warning.

### gRPC Process Isolation (Linux)

On Linux, gRPC plugins run with OS-level restrictions:

- **Namespace isolation** — separate PID and mount namespaces
- **Parent death signal** — plugin is killed if the host process dies
- **Minimal environment** — only `PATH`, `HOME`, `TMPDIR`, and `TZ` are set. **No database credentials or secrets are passed to plugin processes**
- **Network hint** — `GOATFLOW_NO_NETWORK=1` is set for plugins without HTTP permission

## Navigation Control

Plugins can customize the GoatFlow navigation to create focused, single-purpose experiences — for example, a plugin that replaces the helpdesk UI entirely with its own interface.

### Hiding Default Menu Items

Use `HideMenuItems` to remove built-in navigation entries when your plugin is enabled:

```go
HideMenuItems: []string{"dashboard", "tickets", "queues", "phone_ticket", "email_ticket", "admin"},
```

Available menu item IDs:

| ID | Menu Entry |
|----|------------|
| `dashboard` | Agent dashboard |
| `tickets` | Ticket list |
| `queues` | Queue view |
| `phone_ticket` | New phone ticket |
| `email_ticket` | New email ticket |
| `admin` | Admin panel |

Hidden items are removed from both desktop and mobile navigation. Multiple plugins can hide items — the union of all `HideMenuItems` from enabled plugins is applied.

### Custom Landing Page

Use `LandingPage` to redirect users to your plugin's page after login instead of the default dashboard:

```go
LandingPage: "/my-plugin",
```

When set, non-customer users are redirected to this path after successful login. If multiple plugins define a landing page, the first one registered takes effect.

### Example: Plugin-as-App

To make your plugin the primary application (hiding the helpdesk entirely):

```go
return &plugin.GKRegistration{
    Name:          "my-app",
    HideMenuItems: []string{"dashboard", "tickets", "queues", "phone_ticket", "email_ticket"},
    LandingPage:   "/my-app",
    MenuItems: []plugin.MenuItemSpec{
        {ID: "my-app-home", Label: "Home", Icon: "home", Path: "/my-app", Location: "agent"},
    },
    Routes: []plugin.RouteSpec{
        {Method: "GET", Path: "/my-app", Handler: "render_home", Middleware: []string{"auth"}},
    },
}
```

This pattern is ideal for standalone products built on the GoatKit platform — the plugin becomes the entire user experience while still leveraging GoatKit's auth, database, and plugin infrastructure.

## Routes

Register HTTP endpoints your plugin handles:

```go
Routes: []plugin.RouteSpec{
    {Method: "GET", Path: "/api/plugins/stats/overview", Handler: "stats_overview", Middleware: []string{"auth", "admin"}},
    {Method: "POST", Path: "/api/plugins/stats/refresh", Handler: "stats_refresh", Middleware: []string{"auth", "admin"}},
}
```

### Middleware Options

- `auth` — Requires authenticated user
- `admin` — Requires admin role
- `customer` — Requires customer portal access
- `api` — API-only (no session)

## Widgets

Dashboard widgets return HTML from their handler:

```go
Widgets: []plugin.WidgetSpec{
    {ID: "ticket-stats", Title: "Ticket Statistics", Handler: "render_ticket_stats", Location: "agent_home", Size: "large"},
}
```

### Widget Locations

- `agent_home` — Agent dashboard
- `admin_home` — Admin dashboard
- `customer_home` — Customer portal dashboard

### Widget Sizes

- `small` — 1/4 width
- `medium` — 1/2 width
- `large` — 3/4 width
- `full` — Full width

## Scheduled Jobs

```go
Jobs: []plugin.JobSpec{
    {ID: "hourly-cleanup", Schedule: "0 * * * *", Handler: "cleanup_old_data", Description: "Removes stale cache entries", Enabled: true, Timeout: "5m"},
}
```

Schedule format is standard cron. Jobs are registered with GoatFlow's scheduler service automatically.

## Internationalization (i18n)

Provide translations inline or via files. Translations are namespaced per plugin:

```go
I18n: &plugin.I18nSpec{
    Namespace: "my_plugin",
    Translations: map[string]map[string]string{
        "en": {"widget_title": "My Widget", "no_data": "No data available"},
        "de": {"widget_title": "Mein Widget", "no_data": "Keine Daten verfügbar"},
    },
}
```

Access in handlers:

```go
title := host.Translate(ctx, "my_plugin.widget_title")
```

## Packaging

### ZIP Package Structure

```
my-plugin.zip
├── manifest.yaml     # Plugin metadata
├── plugin.wasm       # WASM binary (or binary + plugin.yaml for gRPC)
├── assets/           # Optional static files
└── i18n/             # Optional translation files
```

### Install

Upload via Admin → Plugins, or copy to the `plugins/` directory. Hot reload will pick up changes automatically.

## Hot Reload (Development)

The plugin loader watches the `plugins/` directory via fsnotify:

- **WASM**: Drop a new `.wasm` file → auto-loaded. Modify → auto-reloaded. Delete → auto-unloaded.
- **gRPC**: Add a directory with `plugin.yaml` → auto-discovered and loaded. Rebuild the binary → auto-reloaded (500ms debounce for build tools).

No environment variable or restart needed.

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
cached, found, _ := host.CacheGet(ctx, "expensive_stats")
if found {
    return cached, nil
}
stats := computeExpensiveStats()
host.CacheSet(ctx, "expensive_stats", stats, 300)
return stats, nil
```

### 3. Request Minimal Permissions

Only request what you actually need. Plugins start with restrictive defaults — requesting less means faster admin approval.

### 4. Version Your API

```go
Routes: []plugin.RouteSpec{
    {Path: "/api/plugins/my-plugin/v1/data", Handler: "get_data_v1"},
    {Path: "/api/plugins/my-plugin/v2/data", Handler: "get_data_v2"},
}
```

### 5. Document Your Plugin

Include a README.md with what the plugin does, configuration options, API endpoints, and changelog.

## Debugging

### View Logs

Admin → Plugins → View Logs. Plugin name is automatically added to all log entries.

### Hot Reload

Modify your plugin file/binary and the platform reloads it automatically with 500ms debounce.

## Example Plugins

See the source tree for examples:

- `plugins/stats/` — WASM plugin with dashboard widgets and i18n
- `internal/plugin/grpc/example/` — gRPC plugin with routes and widgets

## Getting Help

- [Host API Reference](./HOST_API.md)
- [WASM Plugin Tutorial](./WASM_TUTORIAL.md)
- [gRPC Plugin Tutorial](./GRPC_TUTORIAL.md)
- [Plugin Platform Overview](../PLUGIN_PLATFORM.md)
