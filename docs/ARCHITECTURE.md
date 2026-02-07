# GoatFlow Architecture

## Overview

GoatFlow is a modern, open-source ITSM solution built on the **GoatKit platform**. It employs a modular monolith architecture with server-side rendering and hypermedia-driven design using HTMX.

## GoatKit Platform

GoatKit is the **platform layer** that powers GoatFlow—and can power other products. Think of GoatKit as the engine and GoatFlow as the first vehicle built on it.

### Platform vs Product

| Layer | What It Is | Examples |
|-------|------------|----------|
| **GoatKit Core** | Shared infrastructure | Auth, DB, Email, Scheduler, Admin UI, Plugin Runtime |
| **Modules** | Pluggable features | Stats, FAQ, Calendar, Process Management |
| **Products** | Complete applications | GoatFlow (ITSM), future products |

This separation enables:
- **Third-party developers** to build plugins without touching core
- **New products** to leverage the same infrastructure
- **Core stability** while features evolve independently

### Platform Capabilities

- **YAML-First Configuration**: All routing in YAML files
- **Dynamic Route Loading**: Hot-reload with manifest governance
- **Unified Binary**: Single `goats` binary runs everything
- **Template Engine**: Pongo2 (Django-like syntax)
- **Plugin Runtime**: WASM (wazero) + gRPC (go-plugin)
- **Container Optimization**: Multi-stage builds (~45MB images)

## System Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                       Proxy (Port 80)                        │
└─────────────────────────────┬────────────────────────────────┘
                              │
┌─────────────────────────────▼────────────────────────────────┐
│                    goats binary                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  GoatFlow Product Layer                                │  │
│  │  • Ticket management, queues, SLAs                     │  │
│  │  • Agent & customer interfaces                         │  │
│  │  • ITSM-specific business logic                        │  │
│  └────────────────────────┬───────────────────────────────┘  │
│  ┌────────────────────────▼───────────────────────────────┐  │
│  │  Modules (Plugins)                                     │  │
│  │  • Stats, FAQ, Calendar, Process Management            │  │
│  │  • Third-party plugins (WASM + gRPC)                   │  │
│  └────────────────────────┬───────────────────────────────┘  │
│  ┌────────────────────────▼───────────────────────────────┐  │
│  │  GoatKit Platform Core                                 │  │
│  │  ┌──────────────┐ ┌──────────────┐ ┌────────────────┐  │  │
│  │  │ Web Server   │ │ Auth System  │ │ Plugin Runtime │  │  │
│  │  │ (Gin + HTMX) │ │ (JWT, LDAP)  │ │ (WASM, gRPC)   │  │  │
│  │  └──────────────┘ └──────────────┘ └────────────────┘  │  │
│  │  ┌──────────────┐ ┌──────────────┐ ┌────────────────┐  │  │
│  │  │ YAML Router  │ │ Templates    │ │ Email System   │  │  │
│  │  │              │ │ (Pongo2)     │ │ (SMTP/IMAP)    │  │  │
│  │  └──────────────┘ └──────────────┘ └────────────────┘  │  │
│  │  ┌──────────────┐ ┌──────────────┐ ┌────────────────┐  │  │
│  │  │ DB Abstraction│ │ Scheduler   │ │ Cache Layer    │  │  │
│  │  │ (PG + MySQL) │ │              │ │ (Valkey)       │  │  │
│  │  └──────────────┘ └──────────────┘ └────────────────┘  │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────┬─────────────────────────────────┬─────────────────┘
           │                                 │
    ┌──────▼────────┐                 ┌──────▼──────┐
    │ PgSQL/MariaDB │                 │   Valkey    │
    │  (5432/3306)  │                 │   (6388)    │
    └───────────────┘                 └─────────────┘
```

## Core Components

### Application Server (goats binary)

- **Language**: Go 1.24
- **Framework**: Gin
- **Template Engine**: Pongo2
- **Image Size**: ~45MB
- **Build**: Multi-stage Docker build with Go compiler

### Database Access Pattern (CRITICAL)

All database access MUST use `ConvertPlaceholders` for MySQL compatibility:

```go
// ✅ CORRECT - Works on PostgreSQL and MySQL
rows, err := db.Query(database.ConvertPlaceholders(
    "SELECT * FROM ticket WHERE queue_id = $1", queueID))

// ❌ WRONG - "$1" placeholders fail on MySQL
rows, err := db.Query("SELECT * FROM ticket WHERE id = $1", id)
```

### Container Images

| Image | Size | Purpose |
|-------|------|---------|
| goatflow | ~45MB | Main application |
| goatflow-toolbox | ~136MB | Development tools |
| goatflow-tests | ~29MB | Test runner |

## Implemented Features (0.5.0 MVP)

### Agent Interface
- Ticket creation with Tiptap editor
- Ticket zoom (view, reply, history)
- Status transitions, assignment, queue transfer
- Queue detail pages with statistics
- Article system (internal/external notes)
- SLA management with time dropdowns

### Customer Portal
- Login with stateless JWT
- Ticket submission with rich text
- View own tickets with filtering
- Reply to and close tickets
- Full i18n (EN, DE, ES, FR, AR)

### Admin Modules
| Module | Status |
|--------|--------|
| Users | ✅ Complete |
| Groups | ✅ Complete |
| Customer Users | ✅ Complete |
| Customer Companies | ✅ Complete |
| Queues | ✅ Complete |
| Priorities | ✅ Complete |
| States | ✅ Complete |
| Types | ✅ Complete |
| SLA | ✅ Complete |
| Services | ✅ Complete |
| Roles | ✅ Complete |
| Dynamic Fields (7 types) | ✅ Complete |
| Templates (8 types) | ✅ Complete |

### Email Integration
- RFC-compliant threading (Message-ID, In-Reply-To, References)
- Outbound notifications on ticket create/reply
- POP3 connector with postmaster processor
- IMAP connector with folder metadata

### Authentication
- JWT with refresh tokens
- LDAP integration (scaffolded)
- Database auth provider

### CI/CD
- Security scanning (gosec, govulncheck, Semgrep, GitLeaks)
- Containerized tests via `make test`
- Codecov integration
- SLSA Level 2 supply chain security

## Technology Stack

### Backend
| Component | Version | Notes |
|-----------|---------|-------|
| Go | 1.24 | Latest stable with generics support |
| Gin | 1.9.1 | HTTP web framework |
| Pongo2 | 6.0.0 | Django-style templates |
| JWT | 5.3.0 | Authentication tokens |

**Key Patterns:**
- No ORM - raw SQL with `ConvertPlaceholders` for cross-database compatibility
- Generics used in utility functions where applicable
- Context propagation for request-scoped data

### Frontend
- HTMX for hypermedia
- Alpine.js for interactivity
- Tailwind CSS
- Tiptap rich text editor
- Server-side rendering

### Infrastructure
- Docker/Podman with multi-stage builds
- PostgreSQL 15+ / MySQL 8+
- Valkey 7+ (Redis-compatible cache)
- GitHub Actions CI/CD

## Go Best Practices

### Code Organization
```
internal/
├── api/           # API handlers (HTMX + REST)
├── auth/          # Authentication providers
├── cache/         # Valkey client wrapper
├── database/      # Connection pool & ConvertPlaceholders
├── models/        # Domain entities with sqlx scan
├── repository/    # Data access layer
├── service/       # Business logic
└── template/      # Pongo2 helpers & functions
```

### Error Handling
```go
// Standard pattern - wrap errors with context
return fmt.Errorf("get ticket: %w", err)

// Repository errors - wrap at service boundary
if errors.Is(err, sql.ErrNoRows) {
    return ErrTicketNotFound
}
```

### Database Access
```go
// Use sqlx or sql.Rows with ConvertPlaceholders
rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(
    "SELECT id, title FROM ticket WHERE queue_id = $1", queueID))

// Scan into structs via sqlx or manual mapping
var tickets []model.Ticket
for rows.Next() {
    var t model.Ticket
    if err := rows.Scan(&t.ID, &t.Title); err != nil {
        return nil, err
    }
    tickets = append(tickets, t)
}
```

### Performance
- Connection pool: `SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`
- Prepared statements for frequently queried columns
- Context cancellation for long-running operations
- Cache-aside pattern with Valkey for session/data caching

## Routing System

Routes defined in YAML files under `routes/`:

```yaml
route_group:
  prefix: /admin
  name: admin-users
  middleware:
    - auth
    - admin
  routes:
    - path: /users
      method: GET
      handler: handleAdminUsers
      template: pages/admin/users.pongo2
```

Governance:
- `make routes-verify` - Check manifest drift
- `make routes-baseline-update` - Accept changes
- `ROUTES_WATCH=1` - Regenerate manifest on changes

## Security

- Non-root containers (UID 1000)
- JWT with short TTL
- Rate limiting per endpoint
- Input validation
- Security scanning in CI

## Development

```bash
make up              # Start services
make test            # Run tests
make test-coverage   # Coverage report
make down            # Stop services
make clean           # Reset everything
```

## Platform Roadmap

### Current: Modular Monolith (v0.5-0.6)

- Single `goats` binary with all features
- YAML-based dynamic modules
- Lambda functions (V8 JavaScript) for computed fields
- Theme engine with hot reload

### Next: GoatKit Plugin Platform (v0.7.0+)

GoatKit evolves into a true platform with plugin extensibility:

- **Dual Runtime**: WASM (wazero) + gRPC (go-plugin)
- **Plugin Packaging**: ZIP with templates, assets, i18n
- **Host Function API**: Database, HTTP, email, cache, scheduler
- **First-Party Plugins**: Statistics, FAQ, Calendar, Process Management

Core becomes platform infrastructure; features become plugins. Third-party developers can extend the platform without modifying core code.

### Future: Multi-Product Platform (v1.0+)

The GoatKit architecture enables multiple products on the same foundation:

- **Shared infrastructure**: Auth, DB, email, scheduling—built once, used everywhere
- **Product-specific logic**: Each product owns its domain (ITSM, asset management, etc.)
- **Unified deployment**: Single stack, multiple capabilities
- **Third-party products**: External developers can build entire products on GoatKit

This isn't just an application—it's a platform for building service management products.

See [Plugin Platform](PLUGIN_PLATFORM.md) for detailed design.
