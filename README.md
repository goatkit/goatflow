# GOTRS - Modern Open Source Ticketing System

[![Security & Code Quality](https://github.com/gotrs-io/gotrs-ce/actions/workflows/security.yml/badge.svg)](https://github.com/gotrs-io/gotrs-ce/actions/workflows/security.yml)
[![Tests](https://github.com/gotrs-io/gotrs-ce/actions/workflows/test.yml/badge.svg)](https://github.com/gotrs-io/gotrs-ce/actions/workflows/test.yml)
[![Build & Release](https://github.com/gotrs-io/gotrs-ce/actions/workflows/build.yml/badge.svg)](https://github.com/gotrs-io/gotrs-ce/actions/workflows/build.yml)
[![codecov](https://codecov.io/github/gotrs-io/gotrs-ce/graph/badge.svg?token=P2ID45BMU4)](https://codecov.io/github/gotrs-io/gotrs-ce)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![SLSA 2](https://slsa.dev/images/gh-badge-level2.svg)](https://slsa.dev)

GOTRS (Go Open Ticket Request System) is a modern, secure, cloud-native ticketing and service management platform built as a next-generation replacement for OTRS. Written in Go with a modular monolith architecture, GOTRS provides enterprise-grade support ticketing, ITSM capabilities, and extensive customization options.

## Key Features

- 🔒 **Security-First Design** - Built with zero-trust principles, comprehensive audit logging, and enterprise security standards
- 🚀 **High Performance** - Go-based backend with optimized database queries and caching
- 🌐 **Cloud Native** - Containerized deployment supporting Docker, Podman, and Kubernetes
- 📱 **Responsive UI** - Modern HTMX-powered interface with progressive enhancement
- 🔄 **OTRS Compatible** - Database schema superset enables seamless migration from OTRS 5.x and 6.x
- 🌍 **Multi-Language** - Full i18n with 15 languages at 100% coverage including RTL support, even supports Klingon! 🖖
- 🎨 **Theme Engine** - 4 distinct themes (Synthwave, Classic, 70s Vibes, 90s Vibe) with dark/light modes and custom fonts
- 🔌 **Extensible** - REST/SOAP APIs, webhooks, and theme customization

## Screenshot

![Screenshot 1](/docs/images/ticket_dark_1.png?raw=true "Ticket")

## Quick Start

### Prerequisites
- Docker with Compose plugin OR Docker Compose standalone OR Podman with Compose
- Git
- 4GB RAM minimum
- Modern web browser with JavaScript enabled

### Container Runtime Support

GOTRS is designed for container-first workflows and supports the following runtimes:

- **Docker Engine**
  - Docker Compose v1 (`docker-compose`) and v2 (Compose plugin: `docker compose`)
- **Podman**
  - Podman with Docker-compatible Compose (e.g. `podman compose` / `podman-docker`)
- **Rootless containers**
  - Supported for both Docker and Podman; may require additional host configuration for ports and volumes
- **SELinux-enabled hosts**
  - Volumes and bind mounts are compatible; use standard SELinux options/labels as required by your distribution

All `make` targets (for example, `make up`, `make down`, `make restart`) automatically detect whether Docker or Podman is available and choose the appropriate Compose command.

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
# - smtp4dev (email sandbox): http://localhost:8025
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

*Demo data resets daily*

### Testing

GOTRS has comprehensive test coverage across multiple layers:

```bash
# Fast unit/integration tests (Alpine toolbox, ~1 minute)
make toolbox-test

# Full browser E2E tests (Ubuntu + Chromium, ~3-5 minutes)
make test-e2e-playwright-go

# Run all 1,200+ tests (5-8 minutes)
make test
```

Browser tests use Go + Playwright with the `//go:build playwright` tag and run in a dedicated Ubuntu container with Chromium. This separation keeps developer feedback fast while maintaining thorough browser coverage.

## Architecture

GOTRS uses a modern, hypermedia-driven architecture that scales from single-server deployments to large enterprise clusters:

- **Core Services**: Authentication, Tickets, Users, Notifications, Workflow Engine
- **Data Layer**: MariaDB/MySQL (default) or PostgreSQL, Valkey (cache), Zinc (search), S3-compatible storage (attachments)
- **API**: RESTful JSON APIs with HTMX hypermedia endpoints
- **Frontend**: HTMX + Alpine.js for progressive enhancement with Tailwind CSS
- **Workflow Engine**: Temporal for complex business processes and automation
- **Real-time**: Server-Sent Events (SSE) for live updates
- **Search**: Zinc with Elasticsearch compatibility for full-text search

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed technical documentation.

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

- Database access: This project uses `database/sql` with a thin `database.ConvertPlaceholders` wrapper to support PostgreSQL and MySQL. All SQL must be wrapped. See [docs/development/DATABASE_ACCESS_PATTERNS.md](docs/development/DATABASE_ACCESS_PATTERNS.md).
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
- **Database**: Full schema validation (MariaDB/PostgreSQL)

### 🚀 Build Pipeline
- **Multi-arch**: AMD64 and ARM64 container builds
- **Supply Chain Security**: SLSA Level 2 attestations, container signing
- **Automated Releases**: Tagged releases with comprehensive release notes
- **Manual Builds**: On-demand builds without registry pushing

### 📊 Quality Metrics
- Comprehensive security scanning (8+ tools)
- SLSA Level 2 compliant build process
- Automated test coverage reporting via Codecov

## Installation

### System Requirements

**Minimum (Development/Small Business)**
- 2 CPU cores
- 4 GB RAM
- 20 GB storage
- MariaDB 11+ or PostgreSQL 14+
- Docker 20+ or Podman 3+

**Recommended (Enterprise)**
- 8+ CPU cores
- 16+ GB RAM
- 100+ GB SSD storage
- MariaDB 11+ or PostgreSQL 14+ cluster
- Kubernetes 1.24+

### Production Deployment

For production deployments, see our comprehensive guides:
- [Docker Deployment Guide](docs/deployment/docker.md)
- [Kubernetes Deployment Guide](docs/deployment/kubernetes.md)

> 💡 **Tip**: Start with Docker deployment for single-server setups, then scale to Kubernetes as your needs grow.

## Documentation

### User Guides
- [Getting Started Guide](docs/getting-started/quickstart.md)
- [Administrator Manual](docs/admin-guide/README.md)
- [Agent Manual](docs/agent-manual/README.md)

### Technical Documentation
- [Architecture Overview](docs/ARCHITECTURE.md)
- [API Reference](docs/api/README.md)
- [Developer Guide](docs/developer-guide/README.md)
- [Configuration System](docs/configuration.md)
- [YAML Platform](docs/YAML_PLATFORM.md)
- [Ticket Number Generators](docs/ticket_number_generators.md)

### Additional Resources
- [Roadmap](ROADMAP.md)
- [Testing Guide](docs/development/TESTING.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [i18n Contributing Guide](docs/i18n/CONTRIBUTING.md)

## Migration from OTRS

GOTRS maintains database schema compatibility with OTRS, enabling migration from existing installations. The `gotrs-migrate` tool handles database import from OTRS 5.x and 6.x SQL dumps.

```bash
# Analyze your OTRS dump
make migrate-analyze SQL=/path/to/otrs_dump.sql

# Test import (dry run)
make migrate-import SQL=/path/to/otrs_dump.sql DRY_RUN=true

# Execute import
make migrate-import SQL=/path/to/otrs_dump.sql DRY_RUN=false

# Validate migrated data
make migrate-validate
```

The migration tool imports tickets, articles, users, customers, queues, and configuration data. Article attachments require a separate filesystem copy.

See [docs/MIGRATION.md](docs/MIGRATION.md) for the complete migration guide.

## Internationalization (i18n)

GOTRS provides comprehensive multi-language support:

### Language Support

| Language | Code | Direction | Status |
|----------|------|-----------|--------|
| English | en | LTR | ✅ Base Language |
| Arabic | ar | RTL | ✅ Complete |
| Chinese | zh | LTR | ✅ Complete |
| French | fr | LTR | ✅ Complete |
| German | de | LTR | ✅ Complete |
| Hebrew | he | RTL | ✅ Complete |
| Japanese | ja | LTR | ✅ Complete |
| Persian | fa | RTL | ✅ Complete |
| Polish | pl | LTR | ✅ Complete |
| Portuguese | pt | LTR | ✅ Complete |
| Russian | ru | LTR | ✅ Complete |
| Spanish | es | LTR | ✅ Complete |
| Ukrainian | uk | LTR | ✅ Complete |
| Urdu | ur | RTL | ✅ Complete |
| Klingon | tlh | LTR | ✅ Complete (Qapla'! 🖖) |

### i18n Features
- **Embedded translations** - JSON files compiled into the binary for zero-config deployment
- **RTL support** - Full right-to-left language support (Arabic, Hebrew, Persian, Urdu)
- **User preferences** - Language selection persisted per-user in profile settings
- **Locale formatting** - Date, time, number, and currency formatting per language
- **Validation tests** - Automated coverage testing ensures 100% translation completeness

### Adding New Languages

1. Create translation file: `internal/i18n/translations/xx.json`
2. Add language config to `internal/i18n/rtl.go` (single source of truth)
3. Run tests: `make test-i18n`
4. Rebuild: `make build`

See [i18n Contributing Guide](docs/i18n/CONTRIBUTING.md) for detailed instructions.

## Features Comparison

See [FEATURES.md](docs/FEATURES.md) for a comprehensive comparison matrix of GOTRS vs OTRS, Zendesk, and ServiceNow across 19 feature categories including:

- ✅ Core ticketing, email integration, knowledge base
- ✅ Theme engine with 4 built-in themes and dark mode
- ✅ Cloud native, air-gapped deployment, 15 languages
- ✅ REST API, source code access, self-hosted

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the development timeline and planned features.

## Contributing

See our [Contributing Guide](CONTRIBUTING.md) for details on:
- Code of Conduct
- Development setup
- Coding standards
- Pull request process
- Issue reporting

**For AI/Agent Contributors**: See [docs/development/AGENT_GUIDE.md](docs/development/AGENT_GUIDE.md) for the canonical operating manual.

## Community

- 💬 [Discord Community](https://discord.gg/kE4XVyX9gF)
-  [Issue Tracker](https://github.com/gotrs-io/gotrs-ce/issues)

## License

GOTRS Community Edition is licensed under the [Apache License 2.0](LICENSE).

For enterprise licensing options, see [LICENSING.md](docs/LICENSING.md).

## Support

### Community Support
See the [Community](#community) section above for forums, chat, and issue tracking.

### Commercial Support
- Professional support contracts
- Implementation services
- Custom development
- Training and certification

Contact: support@gotrs.io

## Security

Security is our top priority. Please report security vulnerabilities to security@gotrs.io.

See [SECURITY.md](docs/SECURITY.md) for our security policies and practices.


## Legal and Compatibility Notice

GOTRS-CE is an **independent, original implementation** of a ticket management system. While we maintain database compatibility for interoperability purposes, all code is originally written. We are not affiliated with OTRS AG. See [LEGAL.md](LEGAL.md) for important legal information.

## Acknowledgments

GOTRS builds upon decades of open source ticketing system innovation. We acknowledge the contributions of the OTRS community and other open source projects that have paved the way.

---

**GOTRS** - Enterprise Ticketing, Community Driven

Copyright © 2025-2026 Gibbsoft Ltd and Contributors
