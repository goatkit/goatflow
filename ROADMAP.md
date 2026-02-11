# GoatFlow Roadmap

Current status, past releases, and future plans for GoatFlow.

## 🚀 Current Status

**Version**: 0.6.5 (February 8, 2026) - GoatKit Plugin Platform

GoatFlow is a GoatKit based ITSM system. It is a modern, secure, cloud-native ticketing and service management platform. It is built as a premier standalone solution for all organizations. Written in Go with a modular monolith architecture, GoatFlow provides enterprise-grade support ticketing, ITSM capabilities, and extensive customization options.

### What's New in 0.6.5
See the [0.7.0 checklist](#070---target-may-2026) for detailed progress. Highlights:
- **Two-Factor Authentication (TOTP)** — 2FA for agents and customers with QR setup, recovery codes, admin override
- **GoatKit Plugin Platform** — Dual-runtime (WASM + gRPC), HostAPI, admin UI, CLI tooling
- **API Tokens** — Personal access tokens for agents and customers
- **REST API v1 Enhanced** — OpenAPI 3.0, Swagger UI, rate limiting, webhooks
- **MCP Server** — AI assistant integration via JSON-RPC (`/api/mcp`) with multi-user proxy RBAC
- **Granular RBAC** — OTRS-compatible permission service with 1,300+ lines of auth tests
- **RBAC Security Hardening** — All statistics and queue endpoints now enforce queue-level permissions (prevents data leakage)
- **Demo Mode** — Restricted mode for public demos (session-only prefs, blocked password/MFA changes)
- **Coachmarks** — Declarative onboarding tooltips with view tracking and dismissal persistence
- **Wallpaper Toggle** — Per-theme background wallpaper control with cookie persistence
- **Dark Theme Contrast Fix** — 338 CSS overrides for proper contrast across all dark themes

### What Works
- Agent Interface: Full ticket management with bulk actions and multi-theme UI (4 themes with wallpaper toggle)
- Customer Portal: Complete self-service with profile management, password changes
- Email Integration: POP3/IMAP + RFC-compliant threading + auto-responses
- Database: MySQL/MariaDB and PostgreSQL with cross-database compatibility
- Automation: GenericAgent, ACLs, SLA escalations, ticket attribute relations
- Integration: GenericInterface with REST/SOAP transports, webservice dynamic fields
- Security: Group-based queue permissions, session management, auth middleware, **API tokens**, **RBAC-filtered statistics**, **Two-factor authentication (TOTP)**
- i18n: 15 languages including RTL support (ar, he, fa, ur)
- Deployment: Docker Compose and Kubernetes Helm chart with multi-arch support, **demo mode**
- Admin Modules: 30+ admin interfaces including ticket attribute relations, dynamic fields, templates
- **Plugins**: Dual-runtime (WASM + gRPC) plugin system with admin UI and state persistence
- **API Documentation**: OpenAPI 3.0 spec with Swagger UI (94 endpoints, 71% coverage)
- **RBAC**: Granular permission service with authorization tests

---

## 📜 Past Releases

### [0.6.4] - February 1, 2026

**GoatKit Plugin Platform Roadmap**

- GoatKit Plugin Platform documentation (`docs/PLUGIN_PLATFORM.md`)
- Roadmap updated: 0.7.0 focused on WASM + gRPC plugin system
- Architecture docs aligned with plugin platform vision
- Handler registry dual registration fix
- 90s theme button contrast fix

### [0.6.3] - January 31, 2026

**Stability & Testing**

- Multi-arch Playwright E2E tests (amd64, arm64)
- Type conversion package (`internal/convert`) breaking circular dependencies
- Single YAML route loader consolidation
- Customer user lookup by login or email
- Handler registry dual registration fixes
- 90s theme button contrast improvements
- Test database setup with OTRS-compatible permissions
- UI test fixes (navigation, accessibility, error pages)

### [0.6.2] - January 25, 2026

**Theming & UX**

- Multi-theme system: Synthwave (default), GoatFlow Classic, Seventies Vibes, Nineties Vibe
- Vendored fonts for offline/air-gapped deployments
- Ticket detail page refactoring (17 modular partials)
- Bulk ticket actions (assign, merge, priority, queue, status)
- Language selector partial component
- Customer password change functionality
- Ticket list pagination
- Customer profile page with preferences
- Admin ticket attribute relations (CSV/Excel import)
- Separate cookie names for agent/customer sessions

### [0.6.1] - January 17, 2026

**Automation & Access Control**

- GenericAgent execution engine for scheduled ticket processing
- ACL evaluation engine for dynamic form filtering
- GenericInterface framework (REST + SOAP transports)
- Group-based queue permission enforcement
- 15 languages with RTL support (ar, he, fa, ur)
- 12 new admin modules (sessions, maintenance, postmaster filters, etc.)
- Webservice dynamic field types (dropdown, multiselect)

### [0.5.1] - January 9, 2026

**Polish & Portability**

- PostgreSQL support alongside MySQL/MariaDB
- Enhanced customer portal
- Improved test coverage

### [0.5.0] - January 3, 2026

**MVP Release** - Core ticketing system complete

- Templates, Roles, Dynamic Fields, Services modules
- Customer portal with full i18n (EN, DE, ES, FR, AR)
- Email threading (Message-ID, In-Reply-To, References)
- 1000+ unit tests, Codecov integration

### [0.4.0] - October 20, 2025

- Preferred queue auto-selection
- Playwright acceptance harness
- Ticket filters (not_closed option)
- GoatKit typeahead/autocomplete modules

### [0.3.0] - September 23, 2025

- Queue detail view with statistics
- Tiptap rich text editor
- Dark mode + Tailwind palette
- Unicode support

### [0.2.0] - September 3, 2025

- DB-less fallbacks for testing
- Toolbox targets
- YAML API routing
- Auth provider registry

### [0.1.0] - August 17, 2025

- OTRS-compatible schema (116 tables)
- Docker/Podman containerization
- JWT authentication, RBAC
- LDAP integration, i18n

---

## 🔮 Future Roadmap

### 0.7.0 - Target: May 2026

**GoatKit Plugin Platform**
- [x] WASM runtime via wazero (pure Go, no CGO)
- [x] gRPC runtime via go-plugin (HashiCorp pattern)
- [x] Unified `Plugin` interface
- [x] Self-describing plugins via `gk_register()` protocol
- [x] `db_query` / `db_exec` — database access (ConvertPlaceholders enforced)
- [x] `http_request` — outbound HTTP calls
- [x] `send_email` — SMTP integration
- [x] `cache_get` / `cache_set` — shared cache
- [x] `schedule_job` — cron/timer registration
- [x] `log` — structured logging
- [x] ZIP distribution: manifest.yaml + wasm/binary + templates + assets + i18n
- [x] Plugin lifecycle: load, register, unload
- [x] Admin UI for plugin management (enable/disable/inspect/logs)
- [x] Example plugins (WASM + gRPC)
- [x] `gk init` scaffolding CLI
- [x] Plugin SDK documentation (AUTHOR_GUIDE, HOST_API, tutorials)
- [x] Hot reload for local development (fsnotify-based, WASM + gRPC binary watching)
- [x] Plugin isolation: per-plugin SandboxedHostAPI with permission enforcement, rate limiting, resource accounting
- [x] Resource policies: plugins declare ResourceRequest, platform enforces ResourcePolicy (admin-configurable)
- [x] Signed plugin verification (optional, ed25519 signatures with `.sig` files)
- [x] OS-level gRPC process isolation (Linux namespace isolation, Pdeathsig, minimal environment)
- [x] SQL table whitelisting (query parsing, scope enforcement)
- [x] Live policy updates (RWMutex-protected, immediate effect without restart)
- [x] Call depth limiting (max 10 for plugin-to-plugin chains)
- [x] Config key blacklist (sensitive patterns blocked by default)
- [x] Email domain scoping and rate limiting (10/min per plugin)
- [x] Caller identity stamping (host-side, prevents impersonation)
- [x] ZIP extraction security (symlink detection, size/count limits)
- [x] Atomic blue-green plugin reload (no request-dropping window)
- [x] Policy persistence (JSON in sysconfig_modified table)

**Statistics & Reporting Plugin** *(first-party, dogfooding)*
- [x] Dashboard statistics API endpoints
- [x] CSV/Excel export
- [x] RBAC filtering on all statistics endpoints (queue-level permission enforcement)
- [ ] Dashboard widgets with Chart.js
- [ ] Built-in report templates (tickets by queue, agent, SLA compliance)
- [ ] Scheduled report delivery via email
- [ ] Time tracking reports and analytics
- [ ] Ships as standalone WASM plugin

**Two-Factor Authentication (TOTP)**
- [x] Agent 2FA setup/disable via Settings page
- [x] Customer 2FA setup/disable via Profile page
- [x] QR code generation for authenticator app enrollment
- [x] Recovery codes (8 codes, 128-bit entropy, single-use)
- [x] 2FA verification during login flow
- [x] Password re-verification before 2FA setup/disable
- [x] Admin override to disable 2FA for locked-out users
- [x] Audit logging for all 2FA events
- [x] Session security: 256-bit tokens, IP binding, rate limiting
- [x] i18n: All 15 languages translated
- [x] Security documentation: `docs/security/TOTP_THREAT_MODEL.md`
- [x] Test coverage: 75 tests (unit, security, E2E, Playwright)
- [ ] Hardware key support (WebAuthn/FIDO2) — Deferred, see threat model

**API Tokens (Personal Access Tokens)**
- [x] Token management UI for agents AND customers
- [x] Scoped permissions (`tickets:read`, `tickets:write`, `admin:*`)
- [x] Configurable expiration (30d, 90d, 1yr, never)
- [x] Rate limiting per token
- [x] RBAC-inherited permissions with scope filtering
- [x] Design spec: `docs/design/API_TOKENS.md`

**REST API v1 (Enhanced)**
- [x] OpenAPI 3.0 specification (4,845 lines, 94 endpoints)
- [x] Swagger UI at `/swagger/`
- [x] Structured error responses (`internal/apierrors/`)
- [x] Rate limiting per endpoint/user
- [x] Webhook subscriptions (HMAC-signed)

**MCP Server (AI Assistant Integration)**
- [x] JSON-RPC 2.0 endpoint at `/api/mcp`
- [x] Bearer token authentication (via API tokens)
- [x] `list_tickets` — with filters (queue, state, owner, customer) + RBAC
- [x] `get_ticket` — by ID or ticket number, with articles + RBAC
- [x] `search_tickets` — full-text search + RBAC
- [x] `list_queues` — user's accessible queues only (RBAC filtered)
- [x] `list_users` — agent users
- [x] `get_statistics` — dashboard stats (RBAC filtered)
- [x] `execute_sql` — SELECT only, admin group required, read-only tables
- [x] Documentation: `docs/api/MCP.md`
- [x] `create_ticket` — create new tickets
- [x] `update_ticket` — update ticket attributes
- [x] `add_article` — add notes to tickets

**Mobile Optimization**
- [ ] Responsive mobile layouts for all pages
- [ ] Touch-optimized controls
- [ ] Mobile ticket creation flow
- [ ] Push notifications (PWA)

**Demo Mode**
- [x] `DemoMode` middleware (sets `is_demo` flag on all requests)
- [x] `DemoGuard` middleware (blocks password/MFA changes, returns 403)
- [x] Session-only preferences (cookie-based, no DB writes)
- [x] Profile page restrictions (hides 2FA/password sections in demo)
- [x] Config: `app.demo_mode` / `GOATFLOW_APP_DEMO_MODE=true`

**Coachmarks (Onboarding Feature Spotlight)**
- [x] Declarative tip registration (`GoatFlow.coachmarks.register()`)
- [x] Auto-positioned balloons with arrow pointers
- [x] View tracking (localStorage) + server-side dismissal persistence
- [x] Theme-aware CSS variable styling
- [x] "Reset feature highlights" on profile pages
- [x] First tip: theme switcher introduction
- [ ] Additional tips for key features (ticket creation, queue management)

**Wallpaper Toggle**
- [x] Checkbox in theme selector dropdown
- [x] Cookie + server-side preference persistence
- [x] Flash prevention (inline `<head>` script)
- [x] Smart disable for themes without wallpaper
- [x] GoatFlow Classic light and dark wallpaper images

**Quality**
- [x] 70% test coverage target (71.2% achieved)
- [x] API documentation site (Swagger UI)
- [ ] Performance benchmarks established
- [ ] Load testing harness

---

### 0.8.0 - September 2026

**Plugin Ecosystem Expansion**
- Plugin marketplace integration (browse, install, update)
- Plugin dependency resolution
- Theme-as-plugin support (themes distributed via plugin system)
- Plugin update notifications and auto-update
- Kubernetes pod isolation for plugins (plugin-per-pod, network policies, resource quotas via K8s-native primitives — replaces namespace/Docker socket approaches)

**FAQ / Knowledge Base Plugin** *(first-party plugin)*
- Public and internal article categories with permissions
- Rich text articles with attachments and images
- Search with relevance ranking and filters
- Article ratings, feedback, and usage analytics
- Link articles to tickets for quick reference
- Customer portal FAQ integration with search
- Article approval workflow

**Calendar & Appointments Plugin** *(first-party plugin)*
- Agent calendar view (day/week/month)
- Ticket-linked appointments with reminders
- Recurring events (daily, weekly, monthly)
- Calendar sharing between agents and teams
- iCal export/subscription
- Integration with ticket escalations
- Resource scheduling (meeting rooms, equipment)

**Self-Service Authentication**
- Password recovery UI for customers (email-based reset)
- Password recovery UI for agents (admin-initiated reset)
- Customer sign-up/registration UI with approval workflow
- Email verification for new accounts
- CAPTCHA integration (reCAPTCHA v3, hCaptcha)
- ~~Two-factor authentication (TOTP)~~ → Moved to 0.6.5 ✅

**Enhancements**
- Keyboard navigation accessibility (WCAG 2.1 AA compliance)
- Drag-and-drop file uploads
- Real-time collaborative ticket editing indicators

**Quality**
- 75% test coverage target
- Accessibility audit and fixes

---

### 0.9.0 - January 2027

**Process Management Plugin** *(first-party plugin)*
- Visual process designer with drag-and-drop
- Multi-step ticket workflows with validation
- Conditional transitions based on ticket data
- Custom activity dialogs with dynamic forms
- Process ticket templates with pre-filled data
- SLA integration with process steps and deadlines
- Process analytics and bottleneck identification

**Theme & UX Enhancements**
- Sound event support (notifications, alerts, ticket actions)
- Custom CSS injection per theme
- Theme preview in admin

**Production Preparation**
- Prometheus metrics endpoint with custom metrics
- Structured JSON logging (configurable levels)
- Health check endpoints (liveness, readiness, startup)
- Graceful shutdown handling with connection draining
- Distributed tracing (OpenTelemetry)
- Circuit breakers for external dependencies

**Quality**
- 80% test coverage target
- Load testing harness (Gatling/k6)
- Chaos engineering tests

---

### 1.0.0 - April 2027

**Production Release**

*Feature Complete*
- GoatKit plugin platform GA (WASM + gRPC runtimes)
- All OTRS core modules operational
- First-party plugins shipped:
  - Statistics & Reporting (dashboards, reports, charts)
  - FAQ/Knowledge Base (articles, search, portal)
  - Calendar & Appointments (scheduling, iCal)
  - Process Management (workflows, designer)

*Security*
- Third-party security audit completed
- Automated dependency vulnerability scanning (Dependabot, Snyk)
- Security hardening guide and best practices
- OWASP Top 10 compliance verification
- Rate limiting and DDoS protection
- Security response policy and CVE process

*Performance*
- 1000+ concurrent users verified under load
- Sub-100ms response times (p95) for all endpoints
- Database query optimization with indexes
- Caching layer (Redis/Valkey) with configurable TTLs
- Connection pooling tuning
- CDN integration for static assets

*Documentation*
- Administrator guide with best practices
- API reference (OpenAPI 3.0) with interactive docs
- Deployment guides (Docker, Kubernetes, cloud providers)
- Migration guide from OTRS 6.x with automation scripts
- Troubleshooting guide with common issues
- Video tutorials and screencasts

*Quality*
- 85% test coverage (unit + integration)
- Comprehensive Playwright E2E test suite
- Chaos engineering tests for resilience
- Performance regression testing in CI
- Automated smoke tests on production deployments

*Enterprise Features*
- High availability setup documentation
- Backup and disaster recovery procedures
- Multi-tenancy support
- LDAP/AD/SAML/OAuth integration guides
- Audit logging and compliance reporting

---

## 📊 Version Summary

| Version | Date | Status | Theme |
|---------|------|--------|-------|
| 1.0.0 | Apr 2027 | 🔮 Future | Production Release |
| 0.9.0 | Jan 2027 | 🔮 Future | Process Management Plugin |
| 0.8.0 | Sep 2026 | 🔮 Future | Plugin Marketplace, FAQ & Calendar Plugins |
| 0.7.0 | May 2026 | 🔮 Future | GoatKit Plugin Platform, Stats Plugin, API v2, Mobile |
| 0.6.4 | Feb 2026 | ✅ Released | Plugin Platform Roadmap |
| 0.6.3 | Jan 2026 | ✅ Released | Stability & Testing |
| 0.6.2 | Jan 2026 | ✅ Released | Multi-Theme System |
| 0.6.1 | Jan 2026 | ✅ Released | Automation & ACLs |
| 0.5.1 | Jan 2026 | ✅ Released | Polish & Portability |
| 0.5.0 | Jan 2026 | ✅ Released | MVP Release |
| 0.4.0 | Oct 2025 | ✅ Released | Filters & Typeahead |
| 0.3.0 | Sep 2025 | ✅ Released | Rich Text & Dark Mode |
| 0.2.0 | Sep 2025 | ✅ Released | YAML Routing |
| 0.1.0 | Aug 2025 | ✅ Released | Foundation |

---

## Get Involved

Want to influence the roadmap? [Join our Discord](https://discord.gg/goatflow) or open a [GitHub Discussion](https://github.com/goatkit/goatflow/discussions).
