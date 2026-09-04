# GoatFlow Roadmap

Current status, upcoming releases, and future plans for GoatFlow. Full release history: [CHANGELOG.md](CHANGELOG.md).

## 🚀 Current Status

**Version**: 0.10.0 (in release preparation — last released: 0.9.0, August 2026) - Plugin Platform Expansion, Ops Hardening, Security Cleanup

GoatFlow is a GoatKit based ITSM system. It is a modern, secure, cloud-native ticketing and service management platform. It is built as a premier standalone solution for all organizations. Written in Go with a modular monolith architecture, GoatFlow provides enterprise-grade support ticketing, ITSM capabilities, and extensive customization options.

- **GoatKit Plugin Platform** — Dual-runtime (WASM + gRPC), HostAPI, admin UI, CLI tooling, hot reload, signed plugins, periodic health checks, bounded graceful shutdown
- **Plugin Sandbox & Security** — Per-plugin isolation, resource policies, SQL whitelisting, namespace isolation, blue-green reload
- **Custom Fields** — Universal EAV on all entities, 15 field types including GIS, plugin registration, admin UI, REST API, MCP tools
- **Plugin UI System** — Independent plugin UIs with 3 shell types, PWA manifests, per-UI branding, auth, and navigation
- **Organisations & Multi-Tenancy** — Org entity with hierarchy, user membership, per-org sysconfig, automatic HostAPI query scoping
- **Secure Settings** — AES-256-GCM encrypted plugin secrets via HostAPI, org-scoped, platform-managed key
- **Entity Deletion** — Soft delete with recycle bin, PII anonymisation, hard delete with cascade, tombstone logging, auto-purge
- **Plugin Marketplace** — `gk install/update/search` CLI, GitHub Releases backend, dependency resolution, theme-as-plugin
- **Self-Service Authentication** — Password recovery, customer registration with approval workflow, email verification, CAPTCHA
- **Reusable UI Components** — 7 components: daily-queue, week-calendar, progress-bar, stat-card, quick-action, file-dropzone, presence-indicator
- **Statistics & Reporting** — Dashboard statistics API, CSV/Excel export, RBAC-filtered endpoints
- **Two-Factor Authentication (TOTP)** — 2FA for agents and customers with QR setup, recovery codes, admin override
- **API Tokens** — Personal access tokens with scoped permissions and configurable expiration
- **REST API v1 Enhanced** — OpenAPI 3.0, Swagger UI, rate limiting, webhooks
- **MCP Server v2** — Dynamic API discovery with SSE transport, auto-generated tools from REST API + plugins, API bridge with full RBAC
- **Granular RBAC** — OTRS-compatible permission service with 1,300+ lines of auth tests
- **Demo Mode** — Restricted mode for public demos (session-only prefs, blocked password/MFA changes)
- **Coachmarks** — Declarative onboarding tooltips (7 tips) with view tracking, dismissal persistence, and i18n
- **Wallpaper Toggle** — Per-theme background wallpaper control with cookie persistence
- **Mobile Optimization** — Responsive tables, GridStack breakpoints, touch targets, mobile ticket creation, compact modals
- **PWA & Push Notifications** — Web app manifest, service worker with offline fallback, VAPID push subscriptions, browser notification delivery
- **Setup Assistant** — first-run wizard + re-runnable task catalog, plugin-extensible (`GKRegistration.SetupTasks`), JSON API at `/api/v1/admin/setup/*`
- **Identity Providers (SAML2 + OIDC)** — SAML2 via `crewjam/saml`, OIDC client (Google + generic), per-org IdP config, login-page IdP buttons
- **Platform/Product Decoupling** — plugin runtime separated from product code into `internal/platform/` (Phases 1–8), boundary linter enforced
- **Customer Knowledge Base Pages** — list, search, and article view wired into the customer portal (backed by the goat-kb plugin)
- **Ops Hardening** — Prometheus `/metrics` (+ optional dedicated `METRICS_PORT` listener), structured JSON logging, real health probes, graceful shutdown with connection draining
- **HostAPI Expansion** — markdown/PDF rendering, article + attachment management, ticket views, ticket-state changes for first-party plugin consumers
- **Security Cleanup** — all open Dependabot alerts cleared (7 high), tooling on bun 1.3

### What Works
- Agent Interface: Full ticket management with bulk actions and multi-theme UI (4 themes with wallpaper toggle)
- Customer Portal: Complete self-service with profile management, password changes, **self-registration with approval workflow**
- Email Integration: POP3/IMAP + RFC-compliant threading + auto-responses
- Database: MySQL/MariaDB and PostgreSQL with cross-database compatibility
- Automation: GenericAgent, ACLs, SLA escalations, ticket attribute relations
- Integration: GenericInterface with REST/SOAP transports, webservice dynamic fields
- Security: Group-based queue permissions, session management, auth middleware, **API tokens**, **RBAC-filtered statistics**, **Two-factor authentication (TOTP)**, **CSP headers**, **secure plugin secrets**
- i18n: 15 languages including RTL support (ar, he, fa, ur)
- Deployment: Docker Compose and Kubernetes Helm chart with multi-arch support, **demo mode**, **K8s pod isolation for plugins**
- Admin Modules: 30+ admin interfaces including ticket attribute relations, dynamic fields, templates, **custom fields**, **recycle bin**, **organisation management**
- **Plugins**: Dual-runtime (WASM + gRPC) plugin system with admin UI, sandbox isolation, signed verification, state persistence, **custom fields**, **plugin UIs**, **marketplace**, **dependency resolution**, **theme-as-plugin**
- **PaaS Core**: Universal custom fields, plugin UI system, organisations with multi-tenancy, secure settings, entity deletion with GDPR anonymisation
- **API Documentation**: OpenAPI 3.0 spec with Swagger UI (94 endpoints, 71% coverage)
- **RBAC**: Granular permission service with authorization tests, **entity.hard_delete permission**
- **Accessibility**: WCAG 2.1 AA keyboard navigation, skip-to-content, focus management, screen reader announcements
- **Mobile**: Responsive tables with column hiding, touch-friendly controls (44px targets), mobile ticket creation, compact modals
- **PWA**: Web app manifest, service worker, offline fallback page, push notifications for ticket reminders

---

## 🔮 Future Roadmap

### 0.10.0 - Unreleased (in review)

**Plugin Platform Expansion, Ops Hardening, and Security Cleanup**

First release since the platform/product split, focused on the plugin-hosting
surface (GoatCoach + goat-kb are the first real consumers), ops hardening
(health/metrics/logging/shutdown), and clearing the Dependabot backlog.

**Plugin platform (HostAPI + shared editors)**
- [x] Shared Tiptap editor partial for plugins (`gk-editor.js`
      + `tiptap_editor.pongo2`) — one promise-based `GoatKitEditor` API
      instead of copy-pasted script tags + retry dance; markdown canonical
      round-trip with GFM table support so rich-editor content survives
      save/reload
- [x] `POST /api/v1/markdown/render` — canonical goldmark+bluemonday renderer
      for plugin preview panes (auth, 1 MiB cap)
- [x] Importable `pkg/markdown` renderer (one canonical goldmark+bluemonday
      stack; `api.RenderMarkdown` delegates to it)
- [x] HostAPI `RenderMarkdownToPdf` — branded, sanitised markdown-to-PDF via
      the Browserless headless-Chromium sidecar (`PdfRenderOptions`: page
      size, margins, title, brand name, color, logo)
- [x] HostAPI `CreateArticle` + article attachment trio
      (`CreateArticleAttachment` / `ListArticleAttachments` /
      `DeleteArticleAttachment`) — transcript/deliverable articles and their
      files through the platform, OTRS invariants in one transaction
- [x] HostAPI `ListTicketViews` — plugin-declared ticket views; board-style
      plugin UIs deep-link cards into the declaring plugin's ticket page
      (standard view fallback when the plugin is disabled)
- [x] HostAPI `ChangeTicketStatus` / `ListTicketStates` + shared
      `ticketstate` service (pending states require `until_time`; the core
      agent status handler is refactored onto it)
- [x] Standard plugin shell renders `ui_nav_items` (label, icon, active
      state, `badge_count`); cross-plugin nav links gate on the target
      UI + plugin being enabled
- [x] Plugin UI routes carry the authenticated identity (`_user_id`,
      `_is_admin`, `_user_role`, ...) via session-auth middleware; the
      minimal shell now gets the base theme bootstrap so plugin pages
      resolve light/dark themes

**Attachments & customer portal**
- [x] PDF page-1 thumbnails via the attachment thumbnail routes
      (`pdftoppm` at ≤400px; raw-file redirect fallback when the tool is
      unavailable)
- [x] ICS calendar attachments render as structured event cards in the
      inline viewer
- [x] Customer portal renders markdown articles as HTML (same sniff rule as
      the agent ticket view); article tables themed via the platform prose
      CSS stack
- [x] Static assets stay fresh across deploys: `no-cache` on `/static/`,
      network-first service-worker policy (offline cache kept), and the
      image build copies the whole `static/js` tree instead of a fixed file
      list
- [x] Dead pre-plugin customer KB handlers removed (the customer KB is
      served entirely by the goat-kb plugin)

**FAQ / Knowledge Base Plugin** *(shipped in separate repo `github.com/goatkit/goat-kb`)*
- [x] Public and internal article categories with permissions
- [x] Rich text articles with attachments and images
- [x] Search with relevance ranking and filters
- [x] Article ratings, feedback, and usage analytics
- [x] Link articles to tickets for quick reference
- [x] Customer portal FAQ integration with search (customer-facing KB pages wired into 0.9.0)
- [x] Article approval workflow

**Ops hardening**
- [x] Prometheus metrics endpoint with custom metrics — real exposition format on `/metrics` (default registerer; cache metrics via `promauto`) + `goatflow_up` / `goatflow_process_start_time_seconds` gauges; optional dedicated listener on `METRICS_PORT` when `METRICS_ENABLED=true` (0.9.x → Unreleased).
- [x] Structured JSON logging (configurable levels) — `internal/platform/logging` drives both `slog` and legacy stdlib `log` from `LOG_FORMAT` / `LOG_LEVEL` / `LOG_OUTPUT` (`LOG_FILE_PATH` legacy alias) (Unreleased).
- [x] Health check endpoints (liveness, readiness, startup) — `GET /health` does a real short-timeout DB ping (503 when unreachable); `GET /health/detailed` adds cache check + version/uptime; wired to the Dockerfile `HEALTHCHECK` and TrueNAS app probes (Unreleased).
- [x] Graceful shutdown handling with connection draining — `http.Server` + SIGTERM/SIGINT: stop accepting, drain in-flight requests up to `DRAIN_TIMEOUT` (default 10 s), then the existing bounded plugin shutdown (Unreleased).
- [x] First-boot admin bootstrap honours `GOATFLOW_ADMIN_PASSWORD` (one-shot,
      race-safe; a later password change makes it a permanent no-op)
- [x] Health/metrics endpoints de-fingerprinted and admin-gated on the app
      port; Prometheus scrapers use the unauthenticated `METRICS_PORT`
      listener

**Security & dependency cleanup**
- [x] All 11 open Dependabot alerts cleared (7 high): `postcss-selector-parser`
      pinned to 6.1.4, `grpc` v1.83.2, `moby/go-archive` v0.3.3,
      `golang.org/x/net` v0.58.0, `postcss` 8.5.26, `js-yaml` 4.3.2,
      `@tiptap/*` 3.31.0, `nanoid` 3.3.18
- [x] Tooling: text `bun.lock` (bun 1.3) adopted across `Dockerfile`,
      `Makefile` and gitleaks; toolbox image pinned to bun 1.3.14 +
      staticcheck v0.7.0 so the unit-test stage can no longer fail silently

**Packaging & housekeeping**
- [x] API license metadata aligned to Apache-2.0 (OpenAPI specs, Swagger
      docs, swagger annotation)
- [x] TrueNAS SCALE added to the supported platforms list; app template
      pinned to 0.10.0
- [x] Helm chart: `appVersion` 0.9.0 + `kubeVersion` gate, real
      `ghcr.io/goatkit/goatflow` image reference, and the `v`-prefix
      stripped from the CI-published appVersion

---

### 1.0.0 - Target: November 2026

**Production Release**

*Feature Complete*
- GoatKit PaaS platform GA (WASM + gRPC runtimes, custom fields, plugin UIs, multi-tenancy)
- All OTRS core modules operational
- First-party open source plugins shipped:
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
- Plugin development guide (custom fields, UIs, enterprise plugin patterns)
- Troubleshooting guide with common issues
- Video tutorials and screencasts

*Quality*
- 85% test coverage (unit + integration)
- Comprehensive Playwright E2E test suite
- Chaos engineering tests for resilience
- Performance regression testing in CI
- Automated smoke tests on production deployments

*Calendar & Appointments Plugin*
- Agent calendar view (day/week/month)
- Ticket-linked appointments with reminders
- Recurring events (daily, weekly, monthly)
- Calendar sharing between agents and teams
- iCal export/subscription
- Integration with ticket escalations
- Resource scheduling (meeting rooms, equipment)

*Process Management Plugin*
- Visual process designer with drag-and-drop
- Multi-step ticket workflows with validation
- Conditional transitions based on ticket data
- Custom activity dialogs with dynamic forms
- Process ticket templates with pre-filled data
- SLA integration with process steps and deadlines
- Process analytics and bottleneck identification

*Theme & UX Enhancements*
- Sound event support (notifications, alerts, ticket actions)
- Custom CSS injection per theme
- Theme preview in admin

*Observability & Resilience*
- Distributed tracing (OpenTelemetry)
- Circuit breakers for external dependencies

---

## 🏢 Enterprise Plugins

Enterprise plugins are paid, reusable horizontal capabilities built on GoatKit core. They extend the platform with configurable business logic that any vertical plugin can consume.

| Plugin | Description |
|--------|-------------|
| goatkit-subscriptions | Recurring service agreements, auto-ticket generation |
| goatkit-invoicing | Invoice generation, numbering, lifecycle, PDF delivery |
| goatkit-payments | Payment recording, reconciliation, Stripe, GoCardless |
| goatkit-billing | Usage-based metering, credit/token balance, Stripe top-ups |
| goatkit-media | Universal media management — file storage, GIF search, thumbnails |
| goatkit-llm | LLM provider management, prompt templates, completion API |
| goatkit-devices | Physical device fleet management, provisioning pipeline |
| goatkit-workflows | Multi-stage job orchestration, DAG pipelines, progress tracking |
| goatkit-audit | Immutable audit logging, compliance, tamper-evident records |
| goatkit-content-feeds | RSS/scraping/API content ingestion, caching, RAG feeds |
| goatkit-maps | Geocoding, route optimisation, area/territory management |
| goatkit-notify | Templated SMS/WhatsApp/email notifications |
| goatkit-chat | Realtime 1:1 chat between customers and agents over SSE, with a pluggable virtual-agent seam |
| goatkit-memory | Org-scoped long-term memory — ingest/query/reflect, documents, mental models |
| goatkit-rag | RAG pipeline — document extraction (Tika sidecar), Hindsight indexing, bank routing |
| goatkit-tts | Text-to-speech with voice cloning (OmniVoice) and speech-to-text |

**Status (September 2026):**

| Status | Plugins |
|--------|---------|
| Released (v1.0.0) | goatkit-llm, goatkit-chat, goatkit-rag |
| In development | goatkit-media, goatkit-billing, goatkit-devices, goatkit-workflows, goatkit-audit, goatkit-content-feeds, goatkit-notify, goatkit-memory, goatkit-tts |
| In planning | goatkit-subscriptions, goatkit-invoicing, goatkit-payments, goatkit-maps |

**Enterprise enquiries:** Enterprise plugins are available as paid add-ons. Contact us at [hello@goatflow.io](mailto:hello@goatflow.io) for enterprise enquiries, licensing, and support.

---

## 📊 Version Summary

| Version | Date | Status | Theme |
|---------|------|--------|-------|
| 1.0.0 | Nov 2026 | 🔮 Future | Production Release |
| 0.10.0 | Unreleased | 🚧 In review | Plugin platform, ops hardening, security cleanup |
| 0.9.0 | Aug 2026 | 🚀 Current | Setup Assistant, SAML + OIDC, Platform Decoupl...[truncated]
| 0.8.4 | Aug 2026 | ✅ Released (with 0.9.0) | External Identity Provider Integration (OIDC Client) |
| 0.8.3 | May 2026 | ✅ Complete | Plugin Auto-Restart, Plugin UI Offline, WebAuthn, Quality |
| 0.8.2 | Apr 2026 | ✅ Released | **MCP v2** + Plugin Manager Resilience (health checks, bounded shutdown) |
| 0.8.1 | Apr 2026 | ✅ Released | Mobile, PWA & Security |
| 0.8.0 | Mar 2026 | ✅ Released | **PaaS Core** — Custom Fields, Plugin UIs, Multi-Tenancy, Deletion |
| 0.7.0 | Mar 2026 | ✅ Released | Plugin Platform Complete, Sandbox & Security, Statistics API |
| 0.6.5 | Feb 2026 | ✅ Released | 2FA, API Tokens, RBAC, Demo Mode, Plugin Platform, MCP Server |
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

Want to influence the roadmap? Open a [GitHub Discussion](https://github.com/goatkit/goatflow/discussions).
