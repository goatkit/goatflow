# GoatKit Plugin Platform

## Overview

GoatKit provides a full plugin platform that enables third-party developers to extend GoatFlow without modifying core code. The platform supports two runtimes — WASM for portable, sandboxed plugins and gRPC for native, I/O-heavy workloads — managed uniformly through a single `Plugin` interface.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        GoatKit Core                              │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────────┐  │
│  │   Router    │  │  Template   │  │     Plugin Runtime       │  │
│  │   (Gin)     │  │  (Pongo2)   │  │  ┌──────┐  ┌──────────┐  │  │
│  │             │  │             │  │  │ WASM │  │   gRPC   │  │  │
│  └─────────────┘  └─────────────┘  │  │wazero│  │go-plugin │  │  │
│                                    │  └──────┘  └──────────┘  │  │
│                                    └──────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │              Plugin Manager                              │    │
│  │  register │ unregister │ enable/disable │ lazy loading   │    │
│  │  policy CRUD │ per-plugin stats │ sandboxed HostAPI      │    │
│  └──────────────────────────────────────────────────────────┘    │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │         SandboxedHostAPI (per-plugin isolation)          │    │
│  │  permission enforcement │ rate limiting │ accounting     │    │
│  └──────────────────────────────────────────────────────────┘    │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │              Host Function API (ProdHostAPI)             │    │
│  │  DBQuery │ DBExec │ HTTPRequest │ SendEmail │ Cache      │    │
│  │  Log │ ConfigGet │ Translate │ CallPlugin                │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ Stats Plugin  │   │  Hello gRPC   │   │ 3rd Party     │
│   (WASM)      │   │  (gRPC/RPC)   │   │   (either)    │
└───────────────┘   └───────────────┘   └───────────────┘
```

## Dual Runtime Support

### WASM Plugins (Default)

Portable, sandboxed plugins using [wazero](https://wazero.io/) (pure Go, no CGO):

- **Single binary distribution** — one `.wasm` file runs everywhere
- **Sandboxed execution** — memory limits, call timeouts, no direct I/O
- **Cross-platform** — no OS/arch-specific builds
- **Best for**: Most plugins, especially UI extensions and business logic

Implementation: `internal/plugin/wasm/`

### gRPC Plugins (Power Users)

Native Go plugins running as separate processes via [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin) with **net/rpc** (not protobuf/gRPC wire protocol):

- **Full Go stdlib** — all packages, goroutines, channels
- **Process isolation** — plugin crashes don't affect core
- **Bidirectional RPC** — plugins can call back to the HostAPI via MuxBroker
- **Hot reload** — fsnotify watches binaries and auto-reloads on change
- **Best for**: Heavy integrations, native dependencies, I/O-heavy workloads

Implementation: `internal/plugin/grpc/`, `pkg/plugin/grpcutil/`

**Note:** Despite the package name "grpc", the actual wire protocol is HashiCorp's net/rpc — no protoc or proto files are needed.

## Plugin Interface

Both runtimes implement a unified interface (`pkg/plugin/plugin.go`):

```go
type Plugin interface {
    GKRegister() GKRegistration            // Self-describe capabilities
    Init(ctx context.Context, host HostAPI) error  // Initialize with host services
    Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error)
    Shutdown(ctx context.Context) error
}
```

## Self-Describing Registration

Plugins return a `GKRegistration` from `GKRegister()` declaring their identity and capabilities:

- **Identity**: Name, version, description, author, license, homepage
- **Routes**: HTTP endpoints the plugin handles (method, path, handler, middleware)
- **Widgets**: Dashboard widgets (location, size, handler, refresh settings)
- **Menu Items**: Navigation entries (admin, agent, customer locations)
- **Jobs**: Scheduled cron tasks (schedule, handler, timeout)
- **Templates**: Template overrides/additions
- **I18n**: Translations with namespace support
- **Error Codes**: API error codes (auto-prefixed with plugin name)
- **Resources**: Requested resource limits and permissions (`ResourceRequest`)

## Plugin Manager

The Manager (`internal/plugin/manager.go`) handles the full plugin lifecycle:

- **Register/Unregister** — loads plugins, creates sandboxed HostAPI, initializes
- **Enable/Disable** — state persisted to sysconfig tables (not separate files)
- **Lazy Loading** — plugins discovered on startup but loaded on first use
- **Policy CRUD** — admin can set/get resource policies per plugin
- **Stats** — per-plugin resource usage counters (DB queries, HTTP requests, cache ops, errors)
- **Plugin-to-plugin calls** — `CallPlugin()` and `CallFrom()` with caller tracking

## Loader & Discovery

The Loader (`internal/plugin/loader/loader.go`) handles filesystem discovery:

- Scans `plugins/` directory for `.wasm` files and subdirectories with `plugin.yaml`
- Supports lazy loading (discover without loading) or eager loading
- Hot reload via fsnotify file watcher with 500ms debounce:
  - WASM: watches for `.wasm` file create/modify/delete
  - gRPC: watches `plugin.yaml` and binary files for changes
  - New `plugin.yaml` files trigger discovery and loading
  - Binary changes trigger unload → reload
  - Removed files trigger unload

### gRPC Plugin Layout

gRPC plugins are deployed as directories with a `plugin.yaml` manifest:

```
plugins/
├── stats.wasm              # WASM plugin (single file)
└── hello-grpc/             # gRPC plugin (directory)
    ├── plugin.yaml         # Required manifest
    └── hello-grpc          # Executable binary
```

### plugin.yaml Format

```yaml
name: hello-grpc
version: "1.0.0"
runtime: grpc
binary: hello-grpc
resources:
  memory_mb: 512
  call_timeout: 30s
  permissions:
    - type: db
      access: readwrite
    - type: http
      scope: ["*.tenor.com"]
```

## Host API

The HostAPI interface (`pkg/plugin/plugin.go`) provides access to host services:

| Function | Signature | Description |
|----------|-----------|-------------|
| `DBQuery` | `(ctx, query, args...) → ([]map[string]any, error)` | SELECT queries with `?` placeholders |
| `DBExec` | `(ctx, query, args...) → (int64, error)` | INSERT/UPDATE/DELETE, returns rows affected |
| `CacheGet` | `(ctx, key) → ([]byte, bool, error)` | Retrieve cached value |
| `CacheSet` | `(ctx, key, value, ttlSeconds) → error` | Store with TTL |
| `CacheDelete` | `(ctx, key) → error` | Remove cached value |
| `HTTPRequest` | `(ctx, method, url, headers, body) → (int, []byte, error)` | Outbound HTTP calls |
| `SendEmail` | `(ctx, to, subject, body, html) → error` | Email via configured provider |
| `Log` | `(ctx, level, message, fields)` | Structured logging (debug/info/warn/error) |
| `ConfigGet` | `(ctx, key) → (string, error)` | Read host config values |
| `Translate` | `(ctx, key, args...) → string` | i18n translation |
| `CallPlugin` | `(ctx, pluginName, fn, args) → (json.RawMessage, error)` | Plugin-to-plugin calls |

The production implementation (`ProdHostAPI` in `internal/plugin/hostapi_prod.go`) wires these to real database, cache (Redis/Valkey), email, and other services. It supports multiple named databases with `@dbname:` query prefix syntax.

## Sandbox & Security Model

Every plugin receives a **SandboxedHostAPI** (`internal/plugin/sandbox.go`) that wraps the real HostAPI with per-plugin enforcement:

### OS-Level Process Isolation (gRPC)

On Linux, gRPC plugin processes run with OS-level restrictions (`internal/plugin/grpc/sandbox_linux.go`):

- **Namespace isolation** — `CLONE_NEWNS` and `CLONE_NEWPID` separate the plugin's mount and PID namespaces from the host
- **Pdeathsig** — `SIGKILL` ensures plugin processes die when the host dies (no orphans)
- **Minimal environment** — plugins receive a stripped-down environment: `PATH`, `HOME`, `TMPDIR` (plugin-specific under `/tmp/goatflow-plugin-<name>`), and `TZ`. No database credentials, secrets, or host environment variables are passed through
- **Network hint** — plugins without `http` permission get `GOATFLOW_NO_NETWORK=1` set in their environment

On non-Linux platforms, process sandboxing is not available and a warning is logged. Use containers or limit gRPC plugins to trusted code on these platforms.

### Plugin Signing

Optional ed25519 signature verification for plugin binaries (`internal/plugin/signing/signing.go`):

- **Key generation** — `GenerateKeyPair()` creates ed25519 key pairs for signing
- **Signing** — `SignBinary()` computes SHA-256 hash of the binary and signs with ed25519, writing hex-encoded signature to `<binary>.sig`
- **Verification** — `VerifyBinary()` checks binary against its `.sig` file using a list of trusted public keys
- **Opt-in enforcement** — set `GOATFLOW_REQUIRE_SIGNATURES=1` to require valid signatures; without it, unsigned plugins load with a warning
- **Tamper detection** — any modification to the binary after signing invalidates the signature

### Permission System

Plugins declare what they need via `ResourceRequest` with `Permission` entries. The platform enforces what they get via `ResourcePolicy` (set by admin).

Permission types:

| Type | Access Levels | Scope |
|------|--------------|-------|
| `db` | `read`, `write`, `readwrite` | Table allowlist patterns |
| `cache` | `read`, `write`, `readwrite` | Auto-namespaced keys |
| `http` | (any) | URL patterns, e.g. `["*.tenor.com"]` |
| `email` | (any) | Domain allowlist, e.g. `["@example.com"]` |
| `config` | `read` | Key patterns (sensitive keys blocked) |
| `plugin_call` | (any) | Plugin name allowlist |

### Enforcement

- **Permission checks** — every HostAPI call checks the plugin's granted permissions
- **DDL blocking** — plugins without write access cannot execute DROP, ALTER, TRUNCATE, CREATE, GRANT, REVOKE
- **SQL table whitelisting** — `extractTableNames()` parses SQL queries and validates each table against the `db` permission scope. Queries touching unallowed tables are rejected
- **HTTP URL filtering** — outbound requests checked against allowed URL patterns (wildcard subdomain matching)
- **Cache namespacing** — keys auto-prefixed with `plugin:<name>:` to prevent cross-plugin collisions
- **Plugin call scoping** — CallPlugin checks which target plugins are in the caller's allowed scope
- **Call depth limiting** — plugin-to-plugin calls are tracked via context; maximum depth of 10 prevents infinite recursion
- **Config key blacklist** — sensitive configuration keys are blocked by default (patterns: `database.*`, `password`, `secret`, `token`, `key`, `auth`, `ldap.*`, `smtp.*`, `aws.*`, `gcp.*`, `azure.*`, etc.)
- **Email domain scoping** — if the `email` permission has a scope (e.g. `["@example.com"]`), recipients are validated against the allowed domains
- **Email rate limiting** — 10 emails per minute per plugin (hardcoded sliding window)
- **Caller identity stamping** — the host sets the authenticated caller name on all gRPC HostAPI calls; plugins cannot impersonate other plugins
- **Blocked status** — a "blocked" policy kills all HostAPI access

### Rate Limiting

Sliding window rate limiters per plugin:

- **DB queries/min** — limits DBQuery + DBExec combined
- **HTTP requests/min** — limits outbound HTTP calls
- **Calls/sec** — limits overall call rate
- **Emails/min** — 10 per minute per plugin

Default policy (`DefaultResourcePolicy`):
- Status: `pending_review`
- Memory: 256 MB
- Call timeout: 30s
- Permissions: DB read-only + cache read/write
- Rate limits: 100 calls/sec, 600 DB queries/min, 60 HTTP requests/min

### Resource Accounting

Atomic counters track per-plugin usage:
- DB queries, DB execs, cache operations, HTTP requests, plugin calls, errors
- Last call timestamp
- Accessible via `Manager.PluginStats()` and `Manager.AllPluginStats()`

### Atomic Plugin Reload

`ReplacePlugin()` in the Manager performs blue-green replacement: the new plugin is fully initialized before the old one is shut down, with the swap happening under a single mutex lock. This eliminates request-dropping windows during hot reload.

### Live Policy Updates

Policies can be updated at runtime via `SandboxedHostAPI.UpdatePolicy()`. The sandbox uses a `sync.RWMutex` to protect the policy pointer — reads acquire RLock, updates acquire full Lock. Changes take effect immediately on the next HostAPI call without requiring plugin restart.

### Policy Persistence

Policies are serialized as JSON and stored in the `sysconfig_modified` table (key: `Plugin::<name>::Policy`). Policies survive restarts and are loaded when the plugin is registered.

### Policy Lifecycle

1. Plugin registers → gets `DefaultResourcePolicy` (restrictive)
2. Admin reviews plugin's `ResourceRequest`
3. Admin sets `ResourcePolicy` via `Manager.SetPolicy()` — approving, restricting, or blocking
4. Policy persisted to database and sandbox updated immediately via `UpdatePolicy()`

### ZIP Package Security

Plugin ZIP extraction (`internal/plugin/packaging/`) enforces strict limits:

- **Symlink rejection** — symlinks in archives are rejected (prevents path traversal)
- **File size limit** — 100 MB per file maximum
- **Total size limit** — 500 MB total extracted content
- **File count limit** — maximum 1,000 files per archive

## Scheduler Integration

Plugin-defined cron jobs are registered with the scheduler (`internal/plugin/scheduler.go`). Jobs declared in `GKRegistration.Jobs` are automatically wired to the scheduler service with configurable timeouts.

## Packaging

Plugin packages (`internal/plugin/packaging/`) support ZIP distribution:

```
my-plugin.zip
├── manifest.yaml
├── plugin.wasm (or binary for gRPC)
├── templates/
├── static/
└── i18n/
```

## Template Integration

Plugin functions are callable from Pongo2 templates via the `{% use %}` directive:

```html
{% use "my_plugin" %}
```

The directive is idempotent — first encounter triggers lazy loading; subsequent calls are no-ops.

## Admin UI

Plugin management is available at `/admin/plugins`:

- Enable/disable plugins with state persisted to sysconfig
- View plugin logs (in-memory ring buffer per plugin)
- Inspect registered routes, widgets, jobs, and menu items
- JWT-authenticated API endpoints for programmatic management

## Example Plugins

- **Stats** (`plugins/stats/`) — WASM plugin providing dashboard widgets
- **Hello gRPC** (`internal/plugin/grpc/example/`) — gRPC plugin demonstrating routes and widgets

## Developer Experience

- **CLI scaffolding**: `gk init my-plugin --type wasm|grpc`
- **Hot reload**: File watcher auto-reloads on plugin changes (no env var needed)
- **Public SDK types**: `pkg/plugin/` for the Plugin interface and types; `pkg/plugin/grpcutil/` for gRPC plugin serving
- **Documentation**: Author Guide, Host API Reference, WASM Tutorial, gRPC Tutorial

## See Also

- [Plugin Author Guide](plugins/AUTHOR_GUIDE.md) — How to build plugins
- [Host API Reference](plugins/HOST_API.md) — Full API documentation
- [gRPC Tutorial](plugins/GRPC_TUTORIAL.md) — Step-by-step gRPC plugin guide
- [WASM Tutorial](plugins/WASM_TUTORIAL.md) — Step-by-step WASM plugin guide
- [ROADMAP](../ROADMAP.md) — Release timeline
