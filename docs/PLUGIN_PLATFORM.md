# GoatKit Plugin Platform

> **Status**: Design Document (planned for v0.7.0, May 2026)
>
> This document describes the **planned** plugin architecture for GoatKit. 
> The features described here are **not yet implemented** — this is a design specification
> that will guide development of the 0.7.0 release.

## Overview

GoatKit will evolve from a modular monolith to a true plugin platform, enabling third-party developers to extend GoatFlow without modifying core code.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     GoatKit Core                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Router    │  │  Template   │  │   Plugin Runtime    │  │
│  │   (Gin)     │  │  (Pongo2)   │  │  ┌──────┐ ┌──────┐  │  │
│  │             │  │             │  │  │ WASM │ │ gRPC │  │  │
│  └─────────────┘  └─────────────┘  │  └──────┘ └──────┘  │  │
│                                    └─────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Host Function API                       │   │
│  │  db_query │ http_request │ send_email │ cache │ log  │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ Stats Plugin  │   │  FAQ Plugin   │   │ 3rd Party     │
│   (WASM)      │   │   (WASM)      │   │   (gRPC)      │
└───────────────┘   └───────────────┘   └───────────────┘
```

## Planned Dual Runtime Support

### WASM Plugins (Default)

Portable, sandboxed plugins will use [wazero](https://wazero.io/) (pure Go, no CGO):

- **Single binary distribution** — one `.wasm` file will run everywhere
- **Sandboxed execution** — memory limits, timeouts, no direct I/O
- **Cross-platform** — no OS/arch-specific builds required
- **Best for**: Most plugins, especially UI extensions and business logic

### gRPC Plugins (Power Users)

Native integrations will use [go-plugin](https://github.com/hashicorp/go-plugin) (HashiCorp pattern):

- **Full I/O access** — native libraries, hardware, network
- **Language agnostic** — write in any language with gRPC support
- **Process isolation** — plugin crashes won't affect core
- **Best for**: Heavy integrations, existing gRPC services, native dependencies

## Planned Plugin Package Format

Plugins will be distributed as ZIP files:

```
my-plugin.zip
├── manifest.yaml          # Plugin metadata and exports
├── plugin.wasm            # WASM binary (or plugin binary for gRPC)
├── templates/             # Pongo2 templates
│   └── my-feature/
│       └── index.html
├── static/                # CSS, JS, images
│   └── my-plugin.css
└── i18n/                  # Translations
    ├── en.yaml
    └── de.yaml
```

## Planned Self-Describing Registration

Plugins will export a `gk_register()` function that returns their capabilities:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "runtime": "wasm",
  "functions": [
    {
      "name": "calculate_something",
      "args": [{"name": "input", "type": "string"}],
      "returns": "json",
      "description": "Calculates something useful"
    }
  ],
  "hooks": ["before_render", "after_save"],
  "menu_items": [
    {"label": "My Feature", "path": "/admin/my-feature", "icon": "star"}
  ],
  "permissions": ["my_plugin.view", "my_plugin.edit"]
}
```

## Planned Host Function API

Plugins will access core capabilities through host functions:

| Function | Description |
|----------|-------------|
| `db_query(sql, params)` | Execute SELECT queries, returns rows |
| `db_exec(sql, params)` | Execute INSERT/UPDATE/DELETE, returns affected |
| `http_request(method, url, headers, body)` | Outbound HTTP calls |
| `send_email(to, subject, body, attachments)` | SMTP integration |
| `cache_get(key)` / `cache_set(key, val, ttl)` | Shared cache access |
| `schedule_job(cron, callback)` | Register scheduled tasks |
| `log(level, message)` | Structured logging |

## Planned Template Integration

Plugin functions will be callable from templates using the `use` directive:

```html
{% use my_plugin %}

<div class="stats-widget">
  {% with stats=calculate_something("input") %}
    <p>Result: {{ stats.value }}</p>
  {% endwith %}
</div>
```

The `use` directive will be idempotent: first encounter loads the plugin; subsequent `use` calls will be no-ops. Plugins will be lazy-loaded on demand, not at startup.

## Planned Lifecycle

1. **Discover**: Core will scan `plugins/` directory on startup, reading manifests
2. **Lazy Load**: Plugin loaded on first `{% use %}` in a template
3. **Register**: Plugin's `gk_register()` called, routes/menus/permissions activated
4. **Run**: Plugin functions become available to templates and handlers
5. **Hot Reload**: File watcher will trigger reload on plugin changes
6. **Unload**: Managed by platform, not templates:
   - Admin UI: manual unload/reload controls
   - Idle timeout: auto-unload after configurable inactivity
   - Memory pressure: LRU eviction when nearing limits
   - Graceful shutdown: clean release on process exit

## Planned Security Model

- **Sandboxing**: WASM plugins will run in isolated memory space
- **Timeouts**: Maximum execution time per function call
- **Memory limits**: Configurable per-plugin memory cap
- **Signed plugins**: Optional verification for marketplace plugins
- **Permission system**: Plugins will declare required permissions

## First-Party Plugins (Roadmap)

| Plugin | Version | Runtime | Description |
|--------|---------|---------|-------------|
| Statistics & Reporting | 0.7.0 | WASM | Dashboards, charts, scheduled reports |
| FAQ / Knowledge Base | 0.8.0 | WASM | Articles, search, customer portal |
| Calendar & Appointments | 0.8.0 | WASM | Scheduling, iCal, reminders |
| Process Management | 0.9.0 | WASM | Visual workflow designer |

## Planned Developer Experience

- **Admin UI**: Enable/disable/inspect plugins, view logs
- **SDK**: Example plugins for both WASM and gRPC
- **CLI**: `gk plugin init` will scaffold new plugins
- **Hot reload**: Changes will apply without restart
- **Local dev mode**: Test plugins against running instance

## Current Foundation

The plugin platform builds on existing GoatKit capabilities:

- **Dynamic Modules** (today): YAML-based CRUD generation
- **Lambda Functions** (today): V8 JavaScript for computed fields
- **Plugin Platform** (0.7.0): Full WASM + gRPC runtime

See also:
- [Dynamic Modules](DYNAMIC_MODULES.md) — Current YAML module system
- [Lambda Functions](LAMBDA_FUNCTIONS.md) — Embedded JavaScript
- [ROADMAP](../ROADMAP.md) — Release timeline
