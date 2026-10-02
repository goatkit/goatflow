# GoatFlow - Modern Open Source Ticketing System

[![Tests](https://github.com/goatkit/goatflow/actions/workflows/test.yml/badge.svg)](https://github.com/goatkit/goatflow/actions/workflows/test.yml)
[![Build & Release](https://github.com/goatkit/goatflow/actions/workflows/build.yml/badge.svg)](https://github.com/goatkit/goatflow/actions/workflows/build.yml)
[![codecov](https://codecov.io/github/goatkit/goatflow/graph/badge.svg?token=P2ID45BMU4)](https://codecov.io/github/goatkit/goatflow)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![SLSA 2](https://slsa.dev/images/gh-badge-level2.svg)](https://slsa.dev)

GoatFlow is a GoatKit based ITSM system. It is a modern, secure, cloud-native ticketing and service management platform. Although it offers a seamless upgrade path for OTRS installations, it is built as a premier standalone solution for all organizations. Written in Go with a modular monolith architecture, GoatFlow provides enterprise-grade support ticketing, ITSM capabilities, and extensive customization options.

## Key Features

- 🔒 **Security** - Every route is checked by an authorization test in CI, admin rights come only from admin-group membership, two-factor login (TOTP, passkeys/security keys, recovery codes), and a hardened plugin sandbox (OS-level isolation, ed25519 signing, SQL table whitelisting)
- 🔑 **Login Options** - Database passwords, LDAP / Active Directory, single sign-on through OIDC / OAuth2 (any OIDC provider such as Keycloak or Azure AD / Entra ID, plus Google and GitHub) and SAML 2.0, and passkey login; OIDC uses PKCE (S256) and JWKS verification, with auto-provisioning and post-login 2FA
- 🚀 **High Performance** - Go-based backend with optimized database queries and caching
- 🌐 **Cloud Native** - Containerized deployment supporting Docker, Podman, and Kubernetes
- 📱 **Responsive UI** - Modern HTMX-powered interface with progressive enhancement
- 🔄 **OTRS Compatible** - Database schema superset; `goatflow-migrate` imports OTRS 6 and Znuny 6.x databases, and the filesystem attachment store reads an OTRS `var/article` tree as-is
- 🧭 **Setup Assistant** — First-run wizard auto-launches on clean installs and guides admins through teams, queues, agents, customers, and SLAs; re-runnable task catalog for ongoing operations with plugin-extensible tasks
- 🌍 **Multi-Language** - Full i18n with 15 languages at 100% coverage including RTL support, even supports Klingon! 🖖
- 🎨 **Theme Engine** - 4 distinct themes (Synthwave, Classic, 70s Vibes, 90s Vibe) with dark/light modes and custom fonts
- 🔌 **Plugin Platform** - Dual-runtime (WASM + gRPC) plugin system with sandboxed execution, hot reload, admin UI, ed25519 plugin signing, OS-level process isolation, SQL table whitelisting, live policy updates, periodic health monitoring, and bounded graceful shutdown
- 🔗 **Extensible** - REST API, signed outbound webhooks (Admin -> Webhooks), outbound REST/SOAP web-service calls (Admin -> Web Services), and theme customization

## Screenshot

![Screenshot 1](/docs/images/ticket_dark_1.png?raw=true "Ticket")

## Quick Start

### Prerequisites
- Docker with Compose plugin OR Docker Compose standalone OR Podman with Compose
- Git
- 4GB RAM minimum
- Modern web browser with JavaScript enabled

### Container Runtime Support

GoatFlow is designed for container-first workflows and supports the following runtimes:

- **Docker Engine**
  - Docker Compose v1 (`docker-compose`) and v2 (Compose plugin: `docker compose`)
- **Podman**
  - Podman with Docker-compatible Compose (e.g. `podman compose` / `podman-docker`)
- **Rootless containers**
  - Supported for both Docker and Podman; may require additional host configuration for ports and volumes
- **SELinux-enabled hosts**
  - Volumes and bind mounts are compatible; use standard SELinux options/labels as required by your distribution
- **Kubernetes**
  - Via Helm chart (`charts/goatflow`), K8s 1.25+; see [Kubernetes Deployment Guide](docs/deployment/kubernetes.md)
- **TrueNAS SCALE (24.10+)**
  - Docker Compose-based app catalog; the official app store package is in submission
    (`docs/truenas-app/`, pins `ghcr.io/goatkit/goatflow:0.10.0`), so TrueNAS support
    is from GoatFlow 0.10.0 onwards; until it's merged, install via YAML with the
    standard Compose stack

All `make` targets (for example, `make up`, `make down`, `make restart`) automatically detect whether Docker or Podman is available and choose the appropriate Compose command.

### Using Containers (Auto-detected)

```bash
# Clone the repository
git clone https://github.com/goatkit/goatflow.git
cd goatflow

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

Try GoatFlow without installation at [https://try.goatflow.io](https://try.goatflow.io)

*Demo data resets daily*

### Testing

GoatFlow has comprehensive test coverage across multiple layers:

```bash
# Fast unit/integration tests (Alpine toolbox, ~1 minute)
make toolbox-test

# Full browser E2E tests (Ubuntu + Chromium, ~3-5 minutes)
make test-e2e-playwright-go

# Run the full Go test suite (2,000+ test functions)
make test
```

Browser tests use Go + Playwright with the `//go:build playwright` tag and run in a dedicated Ubuntu container with Chromium. This separation keeps developer feedback fast while maintaining thorough browser coverage.

## Architecture

GoatFlow uses a modern, hypermedia-driven architecture that scales from single-server deployments to large enterprise clusters:

- **Core Services**: Authentication, Tickets, Users, Notifications
- **Data Layer**: MariaDB/MySQL (default) or PostgreSQL, Valkey (cache); attachments in the database or an OTRS-compatible filesystem tree ([Article Storage](docs/ARTICLE_STORAGE.md))
- **API**: RESTful JSON APIs with HTMX hypermedia endpoints
- **Frontend**: HTMX + Alpine.js for progressive enhancement with Tailwind CSS
- **Real-time**: Server-Sent Events (SSE) for live updates

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed technical documentation.

### Authentication

GoatFlow has these ways to sign in:

| Method | Who | Where to set it up |
|--------|-----|--------------------|
| Database password | Agents and customers | Default; always available |
| LDAP / Active Directory | Agents | `AUTH_PROVIDERS` + `LDAP_*` env vars, see [docs/LDAP.md](docs/LDAP.md) |
| OIDC / OAuth2 (generic OIDC, Google, GitHub) | Agents and customers | Admin -> Identity Providers (`/admin/identity-providers`) |
| SAML 2.0 | Agents and customers | Admin -> Identity Providers |
| Passkey (WebAuthn) | Agents and customers | Each user adds passkeys in their profile |
| Second factor: TOTP app, passkey/security key, recovery codes | Agents and customers | Each user turns on 2FA in their profile |

#### Password provider order (`AUTH_PROVIDERS`)

Password logins try an ordered list of providers. The first one that accepts the login wins.

- `database` - agents and customer users stored in the database
- `ldap` - agents from an LDAP / Active Directory server (also set `LDAP_ENABLED=true`, see [docs/LDAP.md](docs/LDAP.md))
- `static` - in-memory users for demos and tests (see below)

The list is read once at startup, in this order:

1. The `AUTH_PROVIDERS` environment variable, comma separated, e.g. `AUTH_PROVIDERS=ldap,database`.
2. The `Auth::Providers` setting in `config/Config.yaml`.
3. If neither is set: `database` only.

A provider that cannot be created is skipped with a log line. If none can be created, `database` is used. Restart GoatFlow after changing the list.

#### Static users (demos and tests only)

Add `static` to the provider list and set `GOATFLOW_STATIC_USERS` at runtime. Format:

```
GOATFLOW_STATIC_USERS="alice:password:Agent,bob:secret:Customer,carol:adminpass:Admin"
```

- Do not put this variable (or sample secrets) in committed `.env` files. GitLeaks will flag them.
- Passwords may be plain text or pre-hashed (bcrypt, or the OTRS SHA formats). The format is detected automatically.
- Without the variable, the static provider is skipped.

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

GoatFlow maintains high code quality and security standards through comprehensive automated testing:

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
- Kubernetes 1.25+

### Production Deployment

For production deployments, see our comprehensive guides:
- [Docker Deployment Guide](docs/deployment/docker.md)
- [Kubernetes Deployment Guide](docs/deployment/kubernetes.md)
- [TrueNAS SCALE](docs/truenas-app-catalog-requirements.md) — official app catalog
  submission in progress; in the meantime use Apps → Discover → "Install via YAML"
  with the standard Compose stack on SCALE 24.10+

> 💡 **Tip**: Start with Docker deployment for single-server setups, then scale to Kubernetes as your needs grow.

## Documentation

### User Guides
- [Getting Started Guide](docs/getting-started/quickstart.md)
- [Administrator Manual](docs/admin-guide/README.md)
- [Agent Manual](docs/agent-manual/README.md)
- [Customer Portal](docs/CUSTOMER_PORTAL.md)
- [Reports & Analytics](docs/REPORTS.md)
- [Webhooks](docs/WEBHOOKS.md)

### Technical Documentation
- [Architecture Overview](docs/ARCHITECTURE.md)
- [API Reference](docs/api/README.md)
- [Developer Guide](docs/developer-guide/README.md)
- [Configuration](docs/configuration.md)
- [LDAP / Active Directory](docs/LDAP.md)
- [Article Storage](docs/ARTICLE_STORAGE.md)
- [Observability (health, metrics, logging, shutdown)](docs/OBSERVABILITY.md)
- [High Availability](docs/HIGH_AVAILABILITY.md)
- [YAML Platform](docs/YAML_PLATFORM.md)
- [Ticket Number Generators](docs/ticket_number_generators.md)

### Additional Resources
- [Roadmap](ROADMAP.md)
- [Testing Guide](docs/development/TESTING.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [i18n Contributing Guide](docs/i18n/CONTRIBUTING.md)

## Migration from OTRS

GoatFlow's database schema is a superset of the OTRS schema. The `goatflow-migrate` tool imports an **OTRS 6 or Znuny 6.x** database. Older OTRS versions (for example 5.x) are refused: upgrade them to OTRS 6 first.

The OTRS source can be either:

- `SQL=` a mysqldump / mariadb-dump file, or
- `SOURCE=` a live OTRS database: MySQL/MariaDB (`user:pass@tcp(host:3306)/otrs`) or PostgreSQL (`postgres://user:pass@host:5432/otrs?sslmode=disable`).

```bash
# List the source tables, row counts and what the import does with each
make migrate-analyze SQL=/path/to/otrs_dump.sql

# Dry run: print the import plan without writing (DRY_RUN=true is the default)
make migrate-import SQL=/path/to/otrs_dump.sql

# Import for real
make migrate-import SQL=/path/to/otrs_dump.sql DRY_RUN=false

# Validate migrated data
make migrate-validate
```

The import covers tickets, articles, agents, customers, queues, permissions, preferences, dynamic fields and changed settings, and runs in one transaction. It also resets id sequences and the ticket number counters.

Attachments:

- Attachments stored in the OTRS database (ArticleStorageDB) are imported with the rest of the data.
- Attachments stored on disk (ArticleStorageFS) are not copied by the import. Mount or copy the OTRS `var/article` tree to `<STORAGE_PATH>/var/article` and set `STORAGE_TYPE=fs`. See [docs/ARTICLE_STORAGE.md](docs/ARTICLE_STORAGE.md).

See [docs/MIGRATION.md](docs/MIGRATION.md) for the complete migration guide.

## Internationalization (i18n)

GoatFlow provides comprehensive multi-language support:

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
3. Run tests: `make check-i18n`
4. Rebuild: `make build`

See [i18n Contributing Guide](docs/i18n/CONTRIBUTING.md) for detailed instructions.

## Features Comparison

See [FEATURES.md](docs/FEATURES.md) for the status of every feature in 0.10.0 and a comparison matrix of GoatFlow vs OTRS, Zendesk, and ServiceNow across 22 feature categories, including:

- ✅ Core ticketing, email integration, reports
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

- [GitHub Discussions](https://github.com/goatkit/goatflow/discussions)
- [Issue Tracker](https://github.com/goatkit/goatflow/issues)

## License

GoatFlow Community Edition is licensed under the [Apache License 2.0](LICENSE).

For enterprise licensing options, see [LICENSING.md](docs/LICENSING.md).

## Support

### Community Support
See the [Community](#community) section above for forums, chat, and issue tracking.

### Commercial Support
- Professional support contracts
- Implementation services
- Custom development
- Training and certification

Contact: hello@goatflow.io

## Security

Security is our top priority. Please report security vulnerabilities to security@goatflow.io.

See [SECURITY.md](docs/SECURITY.md) for our security policies and practices.


## Legal and Compatibility Notice

GoatFlow-CE is an **independent, original implementation** of a ticket management system. While we maintain database compatibility for interoperability purposes, all code is originally written. We are not affiliated with OTRS AG. See [LEGAL.md](LEGAL.md) for important legal information.

## Acknowledgments

GoatFlow builds upon decades of open source ticketing system innovation. We acknowledge the contributions of the OTRS community and other open source projects that have paved the way.

---

**GoatFlow** - Enterprise Ticketing, Community Driven

Copyright © 2025-2026 Gibbsoft Ltd and Contributors
