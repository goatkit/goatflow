# GOTRS - Modern Open Source Ticketing System

[![Security & Code Quality](https://github.com/gotrs-io/gotrs-ce/actions/workflows/security.yml/badge.svg)](https://github.com/gotrs-io/gotrs-ce/actions/workflows/security.yml)
[![Tests](https://github.com/gotrs-io/gotrs-ce/actions/workflows/test.yml/badge.svg)](https://github.com/gotrs-io/gotrs-ce/actions/workflows/test.yml)
[![Build & Release](https://github.com/gotrs-io/gotrs-ce/actions/workflows/build.yml/badge.svg)](https://github.com/gotrs-io/gotrs-ce/actions/workflows/build.yml)
[![codecov](https://codecov.io/gh/gotrs-io/gotrs-ce/branch/main/graph/badge.svg)](https://codecov.io/gh/gotrs-io/gotrs-ce)
[![Go Report Card](https://goreportcard.com/badge/github.com/gotrs-io/gotrs-ce)](https://goreportcard.com/report/github.com/gotrs-io/gotrs-ce)
[![License](https://img.shields.io/github/license/gotrs-io/gotrs-ce)](LICENSE)
[![SLSA 2](https://slsa.dev/images/gh-badge-level2.svg)](https://slsa.dev)

GOTRS (Go Open Ticket Request System) is a modern, secure, cloud-native ticketing and service management platform built as a next-generation replacement for OTRS. Written in Go with a microservices architecture, GOTRS provides enterprise-grade support ticketing, ITSM capabilities, and extensive customization options.

## Legal and Compatibility Notice

GOTRS-CE is an **independent, original implementation** of a ticket management system. While we maintain database compatibility for interoperability purposes, all code is originally written. We are not affiliated with OTRS AG. See [LEGAL.md](LEGAL.md) for important legal information.

## Key Features

- 🔒 **Security-First Design** - Built with zero-trust principles, comprehensive audit logging, and enterprise security standards
- 🚀 **High Performance** - Go-based backend with optimized database queries and caching
- 🌐 **Cloud Native** - Containerized microservices supporting Docker, Podman, and Kubernetes
- 📱 **Responsive UI** - Modern HTMX-powered interface with progressive enhancement
- 🔄 **OTRS Compatible** - Database schema superset enables seamless migration
  - ⚠️ **Unicode Support**: Configure with `UNICODE_SUPPORT=true` for full Unicode support (requires utf8mb4 migration)
- 🌍 **Multi-Language** - Full i18n with German 100% complete, even supports Klingon! 🖖
- 🎨 **Themeable** - Customizable UI with dark/light modes and branding options
- 🔌 **Extensible** - Plugin framework for custom modules and integrations

## Quick Start

### Prerequisites
- Docker with Compose plugin OR Docker Compose standalone OR Podman with Compose
- Git
- 4GB RAM minimum
- Modern web browser with JavaScript enabled

**Container Runtime Support:**
- ✅ Docker with `docker compose` plugin (v2) - Recommended
- ✅ `docker-compose` standalone (v1) - Legacy support
- ✅ Podman with `podman compose` plugin
- ✅ `podman-compose` standalone
- ✅ Full rootless container support
- ✅ SELinux labels configured for Fedora/RHEL systems

### Using Containers (Auto-detected)

```bash
# Clone the repository
git clone https://github.com/gotrs-io/gotrs-ce.git
cd gotrs-ce

# Set up environment variables (REQUIRED - containers won't start without this!)
cp .env.development .env    # For local development (includes safe demo credentials)
# OR for production:
cp .env.example .env        # Then edit ALL values before use

# Start all services (auto-detects docker/podman compose command)
make up

# Alternative methods:
./scripts/compose.sh up          # Auto-detect wrapper script
docker compose up        # Modern Docker
docker-compose up        # Legacy Docker
podman compose up        # Podman plugin
podman-compose up        # Podman standalone

# Check which commands are available on your system
make debug-env

# Services will be available at:
# - Frontend: http://localhost
# - Backend API: http://localhost/api
# - Mailhog (email testing): http://localhost:8025
# - Adminer (database UI): http://localhost:8090 (optional)

```

### Development Workflow

```bash
# Start services in background
make up-d

# View logs
make backend-logs

# Run database migrations
make db-migrate

# Stop services
make down

# Reset everything (including database)
make clean
```

### Podman on Fedora Kinoite/Silverblue

```bash
# Install podman-compose if needed
sudo rpm-ostree install podman-compose

# The Makefile auto-detects podman
make up

# Generate systemd units (Podman only)
make podman-systemd
```

### Demo Instance

Try GOTRS without installation at [https://try.gotrs.io](https://try.gotrs.io)

Demo credentials are shown on the demo instance login page.

*Note: Demo data resets daily at 2 AM UTC*

### Browser E2E (Go + Playwright)

We separate backend/API tests from full browser automation:

- Toolbox targets (`make toolbox-test`, `make toolbox-test-api`) intentionally skip heavy browser tests.
- Browser-driven Go tests are tagged with `playwright` (see `//go:build playwright`).
- Run them in a dedicated Playwright image (Ubuntu base) to avoid glibc/musl issues:

```bash
make test-e2e-playwright-go        # Builds Dockerfile.playwright-go and runs go test -tags playwright ./tests/e2e
```

Standard JavaScript-based Playwright tests (if present) continue to use `make test-e2e-playwright` which wraps `docker-compose.playwright.yml`.

Rationale: the lightweight Alpine toolbox keeps feedback fast; heavyweight Chromium dependencies stay isolated.

## Architecture

GOTRS uses a modern, hypermedia-driven architecture that scales from single-server deployments to large enterprise clusters:

- **Core Services**: Authentication, Tickets, Users, Notifications, Workflow Engine
- **Data Layer**: PostgreSQL (primary), Valkey (cache), Zinc (search), S3-compatible storage (attachments)
- **API**: RESTful JSON APIs with HTMX hypermedia endpoints
- **Frontend**: HTMX + Alpine.js for progressive enhancement with Tailwind CSS
- **Workflow Engine**: Temporal for complex business processes and automation
- **Real-time**: Server-Sent Events (SSE) for live updates
- **Search**: Zinc with Elasticsearch compatibility for full-text search

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed technical documentation.

### Pluggable Authentication

Authentication supports an ordered provider list configured via the `Auth::Providers` setting in `Config.yaml` (default: `[database]`). Implemented providers:

- `database` (agents + customer users from the database)
- `ldap` (optional; enable with environment variables `LDAP_ENABLED=true` and related LDAP settings)
- `static` (in-memory users for demos/tests)

Static users are enabled by setting the environment variable `GOTRS_STATIC_USERS` at runtime (NOT committed). Format:

```
GOTRS_STATIC_USERS="alice:password:Agent,bob:secret:Customer,carol:adminpass:Admin"
```

Notes:
- Do not add this variable (or sample secrets) to committed `.env` files to avoid GitLeaks false positives.
- Passwords may be plain or pre-hashed (bcrypt / legacy SHA from OTRS). The verifier auto-detects.
- Omit the variable entirely to disable the static provider silently.

Provider resolution order: the system attempts each provider in the configured list until one authenticates or all fail.

Implementation note: the `Auth::Providers` list is read at startup via the unified configuration adapter; the main process wires this adapter into the auth service so changes to the list (after a restart) alter provider selection order without code changes.

### Development Policies

- Database access: This project uses `database/sql` with a thin `database.ConvertPlaceholders` wrapper to support PostgreSQL and MySQL. All SQL must be wrapped. See `DATABASE_ACCESS_PATTERNS.md`.
- Templating: Use Pongo2 templates exclusively. Do not use Go's `html/template`. Render user-facing views via Pongo2 with `layouts/base.pongo2` and proper context.
- Routing: Define all HTTP routes in YAML under `routes/*.yaml` using the YAML router. Do not register routes directly in Go code.
  - YAML is the single source of truth for routes; hardcoded Gin registrations are prohibited.
  - Health endpoints (including `/healthz`) are declared in YAML. Static files are served via YAML using `handleStaticFiles`.
  - In test mode (`APP_ENV=test`), legacy admin hardcoded routes are skipped so SSR/YAML tests don’t double-register paths.
  - YAML route loader is idempotent in dev/tests: it skips registering a route if the same method+path already exists and logs a dedupe message to avoid Gin panics.

### SSR Smoke Tests

- The SSR smoke test discovers GET routes with templates from YAML and ensures they render without 5xx errors.
- Non-strict by default: logs 5xx; to fail on 5xx and on invalid/missing YAML-referenced templates, set `SSR_SMOKE_STRICT=1`.

## CI/CD & Quality

GOTRS maintains high code quality and security standards through comprehensive automated testing:

### 🔒 Security Pipeline
- **Vulnerability Scanning**: Go (`govulncheck`), NPM dependencies (`npm audit`), container images (Trivy)
- **Static Analysis**: Security (gosec, Semgrep), code quality (golangci-lint, ESLint)
- **Secret Detection**: GitLeaks scans for accidentally committed secrets
- **License Compliance**: Automated license checking for all dependencies
- **SAST**: GitHub CodeQL for comprehensive static application security testing

### 🧪 Testing Pipeline  
- **Unit Tests**: Go backend with race detection, HTMX frontend
- **Integration Tests**: End-to-end API testing with test database
- **Coverage**: Automated coverage reporting via Codecov
- **Database**: Full PostgreSQL schema validation

### 🚀 Build Pipeline
- **Multi-arch**: AMD64 and ARM64 container builds
- **Supply Chain Security**: SLSA Level 2 attestations, container signing
- **Automated Releases**: Tagged releases with comprehensive release notes
- **Manual Builds**: On-demand builds without registry pushing

### 📊 Quality Metrics
- All pipelines complete in under 5 minutes
- Zero-cost (GitHub Actions free tier + open source tools)
- Comprehensive security scanning (10+ tools)
- SLSA Level 2 compliant build process

## Installation

### System Requirements

**Minimum (Development/Small Business)**
- 2 CPU cores
- 4 GB RAM
- 20 GB storage
- PostgreSQL 14+
- Docker 20+ or Podman 3+

**Recommended (Enterprise)**
- 8+ CPU cores
- 16+ GB RAM
- 100+ GB SSD storage
- PostgreSQL 14+ cluster
- Kubernetes 1.24+

### Production Deployment

For production deployments, see our comprehensive guides:
- [Docker Deployment](docs/deployment/docker.md)
- [Kubernetes Deployment](docs/deployment/kubernetes.md)
- [Bare Metal Installation](docs/deployment/bare-metal.md)

## Documentation

- Configuration System: `docs/configuration.md`
- Ticket Number Generators: `docs/ticket_number_generators.md`
- YAML Platform: `docs/YAML_PLATFORM.md`
- Architecture: `ARCHITECTURE.md`
- Roadmap: `ROADMAP.md`

- [Getting Started Guide](docs/getting-started/quickstart.md)
- [Administrator Manual](docs/admin-guide/README.md)
- [Agent Manual](docs/agent-manual/README.md)
- [Developer Guide](docs/developer-guide/README.md)
- [API Reference](docs/api/README.md)
- [i18n Contributing Guide](docs/i18n/CONTRIBUTING.md)

## Migration from OTRS

GOTRS provides comprehensive migration tools for OTRS users:

```bash
# Run the migration tool in a container
docker-compose exec backend /app/tools/otrs-migration/migrate \
  --source-db "postgres://otrs_user:pass@old-server/otrs" \
  --target-db "postgres://postgres:5432/gotrs" \
  --validate

# Or using a dedicated migration container
docker run --rm \
  --network gotrs-network \
  -v ./data:/data:Z \
  gotrs/migration-tool:latest \
  --source-db "postgres://otrs_user:pass@old-server/otrs" \
  --target-db "postgres://postgres:5432/gotrs" \
  --validate
```

See [Migration Guide](docs/MIGRATION.md) for detailed instructions.

## Internationalization (i18n)

GOTRS provides comprehensive multi-language support with developer-friendly tools:

### Language Support
- **English** (en) - 100% complete (base language)
- **German** (de) - 100% complete
- **Klingon** (tlh) - 39% complete (Yes, really! 🖖)
- **Spanish** (es) - 47% complete
- **French** (fr) - In progress
- **Portuguese** (pt) - In progress
- **Japanese** (ja) - In progress
- **Chinese** (zh) - In progress
- More languages coming soon!

### i18n Features
- **API-driven translation management** - RESTful endpoints for coverage, validation, import/export
- **CLI tools** - Command-line utilities for translation workflows
- **Live language switching** - Change language without page reload using `?lang=xx`
- **Translation validation** - Automatic completeness checking and key validation
- **CSV/JSON export** - Easy integration with translation services
- **TDD approach** - All i18n features developed with test-driven development

### For Contributors - Using gotrs-babelfish 🐠
```bash
# Check translation coverage (with Hitchhiker's Guide style!)
make babelfish-coverage

# Find missing translations (even for Klingon!)
make babelfish-missing LANG=tlh

# Validate translations
make babelfish-validate LANG=de

# Run with custom options (Don't Panic!)
docker exec gotrs-backend go run cmd/gotrs-babelfish/main.go -help

# Use API for coverage stats
curl http://localhost:8080/api/v1/i18n/coverage

# Test the UI in Klingon (Qapla'!)
# http://localhost:8080/dashboard?lang=tlh
```

> **gotrs-babelfish**: Named after the Babel fish from The Hitchhiker's Guide to the Galaxy - stick it in your ear and instantly understand any language!

See [i18n Contributing Guide](docs/i18n/CONTRIBUTING.md) for detailed instructions on adding new languages.

## Features Comparison

| Feature | GOTRS | OTRS | Zendesk | ServiceNow |
|---------|-------|------|---------|------------|
| Open Source | ✅ (Apache 2.0) | ✅ (GPL) | ❌ | ❌ |
| Self-Hosted | ✅ | ✅ | ❌ | ✅ |
| Cloud Native | ✅ | ❌ | ✅ | ✅ |
| Modern UI | ✅ | ❌ | ✅ | ✅ |
| REST API | ✅ | ✅ | ✅ | ✅ |
| GraphQL API | ✅ | ❌ | ❌ | ✅ |
| Microservices | ✅ | ❌ | ✅ | ✅ |
| Plugin System | ✅ | ✅ | ✅ | ✅ |
| ITSM Modules | ✅ | ✅ | ❌ | ✅ |
| Multi-Language | ✅ (100% DE) | ✅ | ✅ | ✅ |

## Roadmap

### Current Phase: MVP Development (Starting August 2025)
- 🚧 Core ticketing functionality
- 🚧 User authentication and authorization
- 🚧 Email integration
- 📋 Basic reporting
- 📋 Docker deployment

### Upcoming Phases
- **Q4 2025**: Essential features, Production-ready deployment
- **Q1 2026**: Advanced workflows, API v1, Plugin framework
- **Q2 2026**: ITSM modules, Advanced reporting, Mobile apps
- **Q3 2026**: AI/ML features, Enterprise features
- **Q4 2026**: Platform maturity, Cloud SaaS launch

See [ROADMAP.md](ROADMAP.md) for detailed development timeline.

## Contributing

Engineering assistants: See [AGENT.md](AGENT.md) for the canonical operating manual.
Developers: See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution process and standards.

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details on:
- Code of Conduct
- Development setup
- Coding standards
- Pull request process
- Issue reporting

## Community

- 💬 [Discord Community](https://discord.gg/gotrs)
- 📧 [Mailing List](https://groups.google.com/g/gotrs-users)
- 🐛 [Issue Tracker](https://github.com/gotrs-io/gotrs-ce/issues)
- 📖 [Wiki](https://github.com/gotrs-io/gotrs-ce/wiki)

## License

GOTRS is dual-licensed:

- **Community Edition**: [Apache License 2.0](LICENSE)
- **Enterprise Edition**: [Commercial License](LICENSE-ENTERPRISE)

See [LICENSING.md](docs/LICENSING.md) for details on our dual licensing model.

## Support

### Community Support
- GitHub Issues
- Discord Community
- Community Forums

### Commercial Support
- Professional support contracts
- Implementation services
- Custom development
- Training and certification

Contact: support@gotrs.io

## Security

Security is our top priority. Please report security vulnerabilities to security@gotrs.io.

See [SECURITY.md](docs/SECURITY.md) for our security policies and practices.

## Acknowledgments

GOTRS builds upon decades of open source ticketing system innovation. We acknowledge the contributions of the OTRS community and other open source projects that have paved the way.

---

**GOTRS** - Enterprise Ticketing, Community Driven

Copyright © 2025 Gibbsoft Ltd and Contributors# Test comment
