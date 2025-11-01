# GOTRS Architecture

## Overview

GOTRS is built on the GoatKit platform, employing a modular monolith architecture with server-side rendering and hypermedia-driven design. The system uses container-first development with optimized Docker images and YAML-based configuration for maximum flexibility and minimal complexity.

## GoatKit Platform
GoatKit is the underlying platform that powers GOTRS, providing:

- **YAML-First Configuration**: All routing and configuration in YAML files
- **Dynamic Route Loading**: Hot-reload capable route system
- **Route Manifest Governance**: YAML → generated manifest with drift detection & structured diff tooling
- **Unified Binary**: Single `goats` binary (44.7MB) runs everything
- **Template Flexibility**: Support for multiple template engines
- **Container Optimization**: Multi-stage builds with caching
- **Development Tooling**: Comprehensive toolbox containers

The main application binary is called `goats` (GOatkit Application Ticketing System), emphasizing the platform nature of the architecture.

## Current Architecture (Modular Monolith)

### System Architecture
```
┌─────────────────────────────────────────────┐
│             Nginx (Port 80)                  │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│      GOTRS Application (goats binary)        │
│  ┌────────────────────────────────────┐    │
│  │    Web Server (Gin Framework)      │    │
│  │    - HTMX endpoints (HTML)         │    │
│  │    - REST API (JSON)               │    │
│  │    - SSE for real-time updates     │    │
│  └────────────────┬───────────────────┘    │
│  ┌────────────────▼───────────────────┐    │
│  │    YAML Route Configuration        │    │
│  │    - Dynamic route loading         │    │
│  │    - Hot reload capability         │    │
│  └────────────────┬───────────────────┘    │
│  ┌────────────────▼───────────────────┐    │
│  │    Template Engine (Pongo2)        │    │
│  │    - Server-side rendering         │    │
│  │    - Django-like syntax            │    │
│  └────────────────┬───────────────────┘    │
│  ┌────────────────▼───────────────────┐    │
│  │         Service Layer              │    │
│  │  ┌──────┐ ┌──────┐ ┌──────┐      │    │
│  │  │Auth  │ │Ticket│ │Admin │ ...   │    │
│  │  └──────┘ └──────┘ └──────┘      │    │
│  └────────────────┬───────────────────┘    │
│  ┌────────────────▼───────────────────┐    │
│  │      Repository Layer              │    │
│  └────────────────────────────────────┘    │
└─────────────────────────────────────────────┘
           │              │
    ┌──────▼──────┐ ┌─────▼─────┐
    │ PostgreSQL  │ │   Valkey   │
    │   (Port     │ │  (Port     │
    │    5432)    │ │   6388)    │
    └─────────────┘ └───────────┘
```

### Container Architecture
```
┌─────────────────────────────────────────────────────┐
│                 Docker/Podman Host                   │
├─────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐                │
│  │ gotrs:latest │  │gotrs-toolbox │                │
│  │   (44.7MB)   │  │   (136MB)    │                │
│  │  Production  │  │ Development  │                │
│  │    Binary    │  │    Tools     │                │
│  └──────────────┘  └──────────────┘                │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐                │
│  │ gotrs-tests  │  │gotrs-route-  │                │
│  │   (29MB)     │  │tools (47MB)  │                │
│  │ Test Runner  │  │Route Manager │                │
│  └──────────────┘  └──────────────┘                │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐                │
│  │gotrs-config- │  │gotrs-goatkit │                │
│  │manager (32MB)│  │  Config UI   │                │
│  └──────────────┘  └──────────────┘                │
└─────────────────────────────────────────────────────┘
```

## Core Components

### 1. Web Server (Nginx)
- **Technology**: Nginx Alpine
- **Responsibilities**:
  - Reverse proxy to backend (port 8080)
  - Static file serving
  - SSL termination (production)
  - Request buffering
  - Gzip compression

### 2. Application Server (goats binary)
- **Technology**: Go 1.23 binary (44.7MB)
- **Framework**: Gin web framework
- **Template Engine**: Pongo2 (Django-like syntax)
- **Responsibilities**:
  - HTMX endpoint handling
  - REST API serving
  - Session management
  - YAML route loading
  - Server-side rendering

### 3. Core Services

#### Auth Service
```go
// Handles all authentication and authorization
type AuthService interface {
    Login(credentials) (*Token, error)
    Validate(token) (*Claims, error)
    Refresh(refreshToken) (*Token, error)
    Logout(token) error
    // OAuth2, SAML, LDAP integrations
}
```

#### Ticket Service
```go
// Core ticketing functionality
type TicketService interface {
    Create(ticket) (*Ticket, error)
    Update(id, updates) (*Ticket, error)
    Get(id) (*Ticket, error)
    List(filters) ([]*Ticket, error)
    AddComment(ticketId, comment) error
    ChangeStatus(ticketId, status) error
}
```

#### User Service
```go
// User and organization management
type UserService interface {
    CreateUser(user) (*User, error)
    GetUser(id) (*User, error)
    UpdateUser(id, updates) (*User, error)
    AssignRole(userId, role) error
    ManageOrganization(org) error
}
```

#### Pending Reminder Notifications

GOTRS mirrors the classic OTRS pending reminder flow with three pieces that can evolve independently:

- **Detector (Scheduler job)** – a cron-backed job (`pending-reminder`) runs every minute and asks the ticket repository for pending tickets whose `until_time` has passed. The job only works with repository interfaces so it can be mocked in tests. It emits `PendingReminder` events for downstream handling.
- **Dispatcher (pluggable backends)** – reminder events flow through a dispatcher abstraction. The default implementation fan-outs to the in-process toast hub, but the interface allows wiring future channels (email, Slack, etc.) without touching the scheduler core.
- **Presentation (UI toast feed)** – agents poll a lightweight `/api/notifications/pending` endpoint. The feed returns reminders targeted at the current user (responsible user first, owner as fallback). Each toast includes a snooze action posting to the legacy-compatible alias `POST /api/tickets/:id/status`, which resolves to the HTMX handler and updates the ticket’s `until_time` without leaving the ticket context.

The first release ships with the toast backend enabled; follow-up work can register additional dispatchers beside it. All reminder logic continues to rely on the existing `ticket.until_time` column so auto-close and reminder flows remain consistent.

### 4. Database Access Patterns (Thin Wrapper + Repositories)

**Achievement: Full MySQL compatibility restored (August 29, 2025)**

#### Database Access Pattern (MANDATORY)
All database access MUST use the ConvertPlaceholders wrapper for MySQL compatibility:

```go
// ✅ CORRECT - Works on both PostgreSQL and MySQL
rows, err := db.Query(database.ConvertPlaceholders(`
    SELECT t.id, t.tn, t.title, ts.name as state
    FROM ticket t
    JOIN ticket_state ts ON t.ticket_state_id = ts.id
    WHERE t.queue_id = $1 AND t.status IN ($2, $3)
    ORDER BY t.create_time DESC
    LIMIT $4
`), queueID, status1, status2, limit)

// ❌ WRONG - Direct SQL breaks MySQL with "$1" errors  
rows, err := db.Query("SELECT * FROM ticket WHERE id = $1", ticketID)
```

#### Multi-Database Support
```go
type DatabaseDriver interface {
    ConvertPlaceholders(query string) string
    ExecuteQuery(query string, args ...interface{}) (*sql.Rows, error)
    SupportsReturning() bool
    GetDialect() string
}

// Automatic driver detection
func IsMySQL() bool {
    return os.Getenv("DB_DRIVER") == "mysql"
}
```

#### Configuration Examples
```bash
# OTRS MySQL (Production)
DB_DRIVER=mysql
DB_HOST=mysql
DB_USER=otrs  
DB_NAME=otrs
DB_PORT=3306

# PostgreSQL (Development)
DB_DRIVER=postgres  
DB_HOST=postgres
DB_USER=gotrs_user
DB_NAME=gotrs
DB_PORT=5432
```

## Routing (YAML-Driven UI + Governance)

The application now sources most UI/HTMX and several transitional API & alias routes from declarative YAML files in `routes/`:

* Each file: `apiVersion: v1`, `kind: RouteGroup`, with `metadata` (name, description, enabled) and `spec` (`prefix`, `middleware`, `routes`).
* Route fields: `path`, `method`, `handler`, optional `template`, `middleware` (tokens), extended: `redirectTo` (+ optional `status`), `websocket` (boolean upgrade hint).
* Loader: `internal/api/yaml_router_loader.go` scans `./routes` on startup (called inside `setupHTMXRoutesWithAuth`) and registers groups in lexicographic filename order. Redirects and websocket endpoints receive special handling; others resolve handler names via a small registry map.
* Hard-coded HTMX routes in `htmx_routes.go` have been reduced to only those needing dynamic or pre-initialization logic. Governance script `scripts/validate_routes.sh` flags newly added static route registrations under protected groups.
* API reference mapping: `scripts/api_map.sh` generates `runtime/api-map.json|dot|mmd|svg` linking templates & JS assets to `/api/...` endpoints for drift & dead endpoint detection.
* Manifest drift: `scripts/check_routes_manifest.sh` compares current `runtime/routes-manifest.json` to baseline `runtime/routes-manifest.baseline.json` to detect unintended changes.

Implemented additions:
* Middleware tokens (`auth`, `admin`) enforced with real `AuthMiddleware` (fallback guard only in tests/dev without middleware context).
* Consolidated manifest emitted at `runtime/routes-manifest.json` (timestamp + route metadata) for tooling.
* Handler registry (`internal/api/handler_registry.go`) replaces static map; core handlers registered via `ensureCoreHandlers()`.
* Poll watcher (`ROUTES_WATCH=1`) regenerates manifest; optional hot reload (`ROUTES_HOT_RELOAD=1`) atomically swaps a YAML-only engine for rapid iteration.
* Governance Make targets: `make routes-verify` (drift check) and `make routes-baseline-update` (accept changes) maintaining baseline file.
 * Fast bootstrap: `make routes-generate` runs a lightweight generator (no DB, no full server) that loads YAML and writes the manifest. `make routes-verify` will auto-run this if the manifest is missing, so a fresh clone can immediately verify route drift without first starting the full stack or running all tests.
 * Selective reload mode (`ROUTES_SELECTIVE=1` + `ROUTES_WATCH=1`) mounts a dynamic sub-engine for YAML routes only; changes rebuild just that engine without touching static/legacy routes, offering lower-risk iterative updates.

Planned improvements:
1. Selective hot reload preserving full middleware stack & legacy routes.
2. Optional reflection-based auto-registration to remove manual list in `ensureCoreHandlers()`.
3. Route coverage metrics (hit counts) for unused/dead route detection.
4. Structured manifest diff (added/removed/changed middleware/status/template) with severity.
 5. Merge selective + full hot reload strategies (fallback to selective when only YAML changed, escalate to full when core code changes detected via checksum).

Migration Guidance:
* New UI endpoints: add to an existing logical YAML file or create a new `*.yaml` with `enabled: true`.
* Avoid reintroducing hard-coded `protected.GET/POST` lines—tests & validation will fail CI if detected.
* Use `redirects.yaml` for simple aliases instead of inline handlers.


**Critical Success**: System now connects to live OTRS MySQL databases with zero placeholder errors.

### 4. YAML-Based Routing System
```yaml
# Example route configuration (routes/admin/admin-users.yaml)
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

### 5. Data Layer

#### Primary Database (PostgreSQL)
```sql
-- OTRS-Compatible Schema (100% compatibility maintained)
-- Uses INTEGER primary keys, NOT UUIDs
ticket
├── id (BIGSERIAL)           -- BIGINT auto-increment
├── tn (VARCHAR(50))         -- Ticket number, unique
├── title (VARCHAR(255))
├── ticket_state_id (SMALLINT, FK)
├── ticket_priority_id (SMALLINT, FK)
├── queue_id (INTEGER, FK)
├── customer_id (VARCHAR(150))      -- Company ID
├── customer_user_id (VARCHAR(250)) -- Customer email/login
├── user_id (INTEGER, FK)           -- Owner
├── responsible_user_id (INTEGER)   -- Assigned agent
├── create_time (TIMESTAMP)
├── change_time (TIMESTAMP)
└── (116 total OTRS tables)

-- Indexes for performance
CREATE INDEX idx_ticket_state_id ON ticket(ticket_state_id);
CREATE INDEX idx_ticket_queue_id ON ticket(queue_id);
CREATE INDEX idx_ticket_customer_id ON ticket(customer_id);
CREATE INDEX idx_ticket_create_time ON ticket(create_time DESC);
```

#### Cache Layer (Valkey)
```
Key Patterns:
- session:{session_id} - User sessions
- cache:ticket:{id} - Ticket cache
- rate:{ip} - Rate limiting
- queue:{name} - Job queues
- pubsub:{channel} - Real-time events
```

#### Search Layer (Zinc)
```
Elasticsearch-compatible API:
- Full-text search across tickets
- Faceted search and filtering
- Auto-complete suggestions
- Index: gotrs_tickets
- Compatible with ES clients
```

#### Workflow Engine (Temporal)
```
Workflow Patterns:
- Ticket lifecycle management
- SLA enforcement
- Escalation rules
- Automated notifications
- Approval chains
```

### 6. Communication Patterns

#### Synchronous Communication
- REST API for JSON data exchange
- HTMX endpoints returning HTML fragments
- gRPC for internal service-to-service calls

#### Asynchronous Communication
- Temporal for workflow orchestration
- Server-Sent Events (SSE) for real-time updates
- Background job processing via Temporal activities

### 7. Security Architecture

```yaml
Security Layers:
  Network:
    - TLS 1.3 for all communications
    - Network segmentation
    - Firewall rules
    
  Application:
    - JWT with short TTL
    - API key management
    - Rate limiting per endpoint
    - Input validation
    
  Data:
    - Encryption at rest
    - Field-level encryption for PII
    - Audit logging
    
  Access Control:
    - RBAC with fine-grained permissions
    - Multi-factor authentication
    - Session management
```

## Docker Build Architecture

### Multi-Stage Build Strategy
```dockerfile
# Optimized Dockerfile with BuildKit
# syntax=docker/dockerfile:1

# Stage 1: Dependencies (cached)
FROM golang:1.23-alpine AS deps
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Stage 2: Build tools (parallel)
FROM deps AS tools
RUN --mount=type=cache,target=/go/pkg/mod \
    go install -tags 'postgres' migrate@latest

# Stage 3: Application build
FROM deps AS builder
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 go build -ldflags="-w -s" -o goats

# Stage 4: Minimal runtime (Alpine 3.19)
FROM alpine:3.19 AS runtime
# Final image: 44.7MB
```

### Container Images
- **gotrs:latest** (44.7MB) - Main application
- **gotrs-toolbox** (136MB) - Development tools
- **gotrs-tests** (29MB) - Test runner
- **gotrs-route-tools** (47MB) - Route management
- **gotrs-config-manager** (32MB) - Configuration UI
- **gotrs-goatkit** - GoatKit platform tools

## Deployment Architecture

### Development Environment
```yaml
# docker-compose.yml (Podman-compatible, no version specified)
services:
  backend:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: gotrs-backend
    ports:
      - "8080:8080"
    environment:
      ENABLE_YAML_ROUTING: true
      ROUTES_DIR: /app/routes
    depends_on:
      postgres:
        condition: service_healthy
      
  postgres:
    image: postgres:15-alpine
    container_name: gotrs-postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      
  valkey:
    image: valkey/valkey:7-alpine
    container_name: gotrs-valkey
    ports:
      - "6388:6379"
    volumes:
      - valkey_data:/data
      
  nginx:
    image: nginx:alpine
    container_name: gotrs-nginx
    ports:
      - "80:80"
    volumes:
      - ./docker/nginx/nginx.conf:/etc/nginx/nginx.conf:ro,Z
      - ./static:/var/www/static:ro,Z
      
  mailhog:
    image: mailhog/mailhog:latest
    container_name: gotrs-mailhog
    ports:
      - "8025:8025"
      
  # Future services (currently disabled):
  # temporal, zinc, ldap - commented out until needed
```

### Production Kubernetes
```yaml
# Kubernetes deployment structure
namespaces/
├── gotrs-prod/
│   ├── deployments/
│   │   ├── auth-service
│   │   ├── ticket-service
│   │   └── user-service
│   ├── services/
│   ├── configmaps/
│   └── secrets/
└── gotrs-monitoring/
    ├── prometheus
    ├── grafana
    └── elasticsearch
```

## Scalability Patterns

### Horizontal Scaling
```
┌──────────────┐
│Load Balancer │
└──────┬───────┘
       │
┌──────▼───────────────────────┐
│   Service Instances (N)      │
│ ┌────┐ ┌────┐ ┌────┐ ┌────┐│
│ │Pod1│ │Pod2│ │Pod3│ │PodN││
│ └────┘ └────┘ └────┘ └────┘│
└──────────────────────────────┘
```

### Database Scaling
- Read replicas for query distribution
- Connection pooling (pgBouncer)
- Partitioning for large tables
- Archival strategy for old tickets

### Caching Strategy
1. **Application Cache**: In-memory caching
2. **Valkey Cache**: Shared cache across instances
3. **CDN**: Static assets and attachments
4. **Database Query Cache**: PostgreSQL query optimization

## Monitoring and Observability

### Metrics (Prometheus)
```go
// Key metrics to track
var (
    RequestDuration = prometheus.NewHistogramVec(...)
    RequestCount = prometheus.NewCounterVec(...)
    ErrorRate = prometheus.NewGaugeVec(...)
    QueueDepth = prometheus.NewGaugeVec(...)
)
```

### Logging (ELK Stack)
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "service": "ticket-service",
  "trace_id": "abc123",
  "user_id": "user456",
  "action": "ticket.create",
  "duration_ms": 45
}
```

### Tracing (OpenTelemetry)
- Distributed tracing across services
- Performance bottleneck identification
- Request flow visualization

## Technology Stack

### Backend
- **Language**: Go 1.23
- **Web Framework**: Gin
- **Template Engine**: Pongo2 (Django-like syntax)
- **Data Access**: database/sql with ConvertPlaceholders (no ORM)
- **Authentication**: JWT with refresh tokens
- **Validation**: go-playground/validator
- **Binary Name**: goats (GoatKit platform)

### Frontend
- **Architecture**: Server-side rendering (SSR)
- **Framework**: HTMX for hypermedia-driven architecture
- **JavaScript**: Alpine.js for lightweight interactivity
- **CSS**: Tailwind CSS with Makefile build targets
- **Templates**: Pongo2 templates with base layouts
- **Real-time**: Server-Sent Events (SSE)
- **Build**: `make css-build` for production CSS

### Infrastructure
- **Container Runtime**: Docker or Podman (auto-detected)
- **Build System**: Multi-stage Dockerfile with BuildKit
- **Orchestration**: Kubernetes/OpenShift ready
- **Development**: Container-first with Makefile
- **Security**: Non-root containers (UID 1000)

## Development Principles

1. **Container-First**: All development in containers, no host dependencies
2. **Production-Like Always**: Single Dockerfile, no separate dev environment
3. **YAML-Driven Configuration**: Routes defined in YAML for flexibility
4. **Server-Side Rendering**: HTMX + Pongo2 for simplicity
5. **Test-Driven Development**: Comprehensive test coverage
6. **Security by Design**: Non-root containers, security scanning
7. **DRY Principle**: Shared base images, reusable components
8. **Makefile as Entry Point**: All operations through make targets

## Performance Characteristics

### Current Performance (MVP)
- **API Response Time**: < 200ms (p95)
- **Page Load Time**: < 2 seconds
- **Docker Image Size**: 44.7MB (main application)
- **Build Time**: 20 seconds (cached), 44 seconds (clean)
- **Container Startup**: < 5 seconds
- **Memory Usage**: ~50MB idle, ~200MB under load
- **Concurrent Users**: 100+ (MVP target)

### Build Performance (with BuildKit)
- **Cached Rebuild**: 20 seconds (70% faster)
- **Clean Build**: 44 seconds
- **Parallel Stages**: Tools and app build concurrently
- **Cache Hit Rate**: >90% for unchanged dependencies

## Extension Architecture

### Plugin System
```go
type Plugin interface {
    Name() string
    Version() string
    Init(container *Container) error
    RegisterRoutes(router *gin.RouterGroup)
    RegisterHooks(hooks *HookRegistry)
    Shutdown() error
}
```

### Extension Points
- Custom ticket fields
- Workflow actions
- Notification channels
- Report generators
- Authentication providers
- Storage backends

## Current State and Roadmap

### Implemented Features
- ✅ HTMX-driven UI with server-side rendering
- ✅ Complete admin interface (users, groups, queues, tickets)
- ✅ YAML-based dynamic routing system
- ✅ Optimized Docker build system (44.7MB images)
- ✅ Container-first development workflow
- ✅ PostgreSQL with OTRS-compatible schema
- ✅ Pongo2 template engine with dark mode
- ✅ Session-based authentication
- ✅ Makefile-driven operations

### In Progress
- 🚧 JWT authentication implementation
- 🚧 Customer portal interface
- ✅ Email threading with Mailhog (RFC-compliant Message-ID, In-Reply-To, References)
- 🚧 Advanced search capabilities

### Future Considerations
- **Temporal Workflows**: Workflow engine integration (currently disabled)
- **Zinc Search**: Full-text search engine (currently disabled)
- **LDAP Integration**: Enterprise authentication (scaffold exists)
- **Multi-tenancy**: Isolated customer environments
- **Microservices Split**: When scaling requires it
- **AI Integration**: Ticket classification and routing
