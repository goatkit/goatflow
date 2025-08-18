# CLAUDE.md - AI Assistant Project Context

## Project Overview
GOTRS is a modern ticketing system replacing OTRS, built with Go backend and HTMX frontend. Hypermedia-driven architecture with server-side rendering for simplicity and performance. Security-first design with rootless containers.

## Tech Stack
- Backend: Go 1.22+, Gin, PostgreSQL, Valkey, Temporal, Zinc
- Frontend: HTMX, Alpine.js, Tailwind CSS (server-rendered)
- Real-time: Server-Sent Events (SSE)
- Containers: Docker/Podman (first-class support for both)
- Deploy: Kubernetes, OpenShift-ready
- Current Phase: MVP Development (Started Aug 10, 2025)

## Key Principles
- **Docker/Podman from Day 1**: All development in containers
- **Security First**: Non-root containers (UID 1000), Alpine base
- **Podman Priority**: Full rootless and SELinux support
- **OpenShift Ready**: SCC-compatible containers

## Project Structure
```
/cmd/server/main.go         - Entry point
/internal/                  - Core business logic
  /api/                    - HTTP handlers
  /core/                   - Domain logic
  /data/                   - Database layer
/web/                      - React frontend
/migrations/               - SQL migrations
/docker/                    - Container configs
/scripts/                   - Init scripts
```

## Key Commands
```bash
make up                     # Start all services (auto-detects docker/podman)
make down                   # Stop services
make logs                   # View logs
make db-shell              # PostgreSQL shell
make clean                 # Reset everything
```

## Container Services
- **postgres**: PostgreSQL 15 Alpine
- **valkey**: Valkey 7 Alpine  
- **backend**: Go with Air hot reload (non-root)
- **nginx**: Reverse proxy with static assets
- **mailhog**: Email testing
- **temporal**: Workflow orchestration engine
- **zinc**: Elasticsearch-compatible search engine

## Database
- PostgreSQL with OTRS-compatible schema
- Main tables: users, tickets, ticket_messages, attachments
- Use migrations for schema changes

## API Patterns
- REST endpoints: /api/v1/*
- Auth: JWT in Authorization header
- Response: JSON with {success, data, error}
- Status codes: 200 OK, 201 Created, 400 Bad Request, 401 Unauthorized, 500 Error

## Common Tasks
1. Add endpoint: handler → route → service → repository
2. Add UI: template → HTMX endpoint → Alpine.js for interactivity
3. Add migration: Create numbered SQL file in /migrations
4. Add workflow: Define Temporal workflow → activities → worker

## Current Status (Aug 10, 2025)
✅ **Phase 0 Complete**: Foundation established
✅ Project structure created with Go backend and HTMX frontend
✅ Docker/Podman containers working with rootless support
✅ Frontend (HTMX + Alpine.js + Tailwind) with server-side rendering
✅ Backend (Go + Gin + Air) with hot reload
✅ PostgreSQL database with OTRS-compatible schema (14 tables)
✅ Database migration system with make commands
✅ Development environment fully functional
✅ Comprehensive documentation and troubleshooting guides

**Ready for Phase 1**: Backend development (JWT auth, RBAC, ticket CRUD)

## Current Focus
Building MVP with core ticketing functionality. See ROADMAP.md for timeline.

## Troubleshooting

### Permission Issues (Rare)
If you see `EACCES: permission denied` errors:
```bash
sudo chown -R $USER:$USER ./web/
make down && make up
```
Note: Usually not needed as files are created with correct ownership.

### Docker Hub Auth Issues
If pulls fail with `unauthorized`:
```bash
podman pull docker.io/library/postgres:15-alpine
podman pull docker.io/library/valkey:7-alpine
make up
```

## Important Files
- `.env.example` - Environment configuration template
- `docker-compose.yml` - Container orchestration (Podman compatible)
- `Makefile` - Development commands (auto-detects docker/podman)
- `ROADMAP.md` - Development phases and timeline
- `docs/PODMAN.md` - Podman/rootless documentation
- `docs/development/MVP.md` - MVP implementation details
- `docs/development/DATABASE.md` - Schema documentation

## Coding Standards
- Go: Standard formatting (gofmt)
- Templates: Go HTML templates with layouts
- JavaScript: Minimal Alpine.js for interactivity
- CSS: Tailwind utility classes
- SQL: Lowercase with underscores
- Git: Conventional commits (feat:, fix:, docs:)

## Development Environment
- **Primary Platform**: Fedora Kinoite with Podman
- **Also Supports**: Any OS with Docker/Podman
- **No Local Dependencies**: Everything runs in containers
- **Hot Reload**: Both backend (Air) and frontend (Vite)
- **Email Testing**: Mailhog catches all emails locally

## Security Requirements
- All containers run as non-root (UID 1000)
- Alpine Linux base images only
- SELinux labels on volume mounts (:Z)
- Rootless container support
- OpenShift SCC compatible

## Testing Requirements
- Unit tests for critical business logic
- API endpoint tests
- Run tests in containers: `make test`
- Skipping UI tests in MVP phase

## Performance Targets
- API response < 200ms (p95)
- Page load < 2 seconds
- Support 100 concurrent users (MVP)
- Container images < 100MB each

## Lessons Learned

### Testing Before Declaring Ready (Aug 18, 2025)
- **ALWAYS test commands/features before claiming they work**
- Don't ask users to be QA - test it yourself first
- When creating containerized commands, verify:
  - Correct tool versions (e.g., Go 1.22 not 1.21)
  - Proper cache/permission handling (GOCACHE, GOMODCACHE)
  - All dependencies are available
- Better to say "Let me test this" than "It's ready!" without verification

### Containers First Philosophy
- Every tool command should run in containers
- Use environment variables for cache directories to avoid permission issues
- Always provide containerized alternatives for host commands
- Test with `make test-containerized` to verify compliance

### Secret Management (Updated Aug 18, 2025)
- All secrets exclusively in environment variables
- Use `make synthesize` to generate secure .env files
- Never hardcode even test secrets - use obvious prefixes (test-, mock-, dummy-)
- Pre-commit hooks essential for preventing accidental secret commits
- **CI/CD should use synthesize**: Run synthesize in CI to generate secrets rather than hardcoding test values
- **This is the way**: Let the application generate its own secure configuration

### Test Stabilization Approach (Aug 18, 2025)
- **Fix compilation errors first** - Can't test if it doesn't build
- **Address panics before logic errors** - Runtime failures block everything
- **Make tests flexible for environments** - Tests should handle DB vs fallback data
- **Use proper test data setup** - Don't assume global state exists
- **Mock expectations must be flexible** - SQL regex patterns shouldn't be too strict