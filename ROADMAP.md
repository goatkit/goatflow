# GoatFlow Roadmap

Current status, upcoming releases, and future plans for GoatFlow. Full release history: [CHANGELOG.md](CHANGELOG.md).

## 🚀 Current Status

**Version**: 0.10.0 — release candidate, not yet tagged. Last release: 0.9.0 (2026-08-06).

0.10.0 theme: sign-in and self-service (LDAP, password reset, customer sign-up, passkey management), full PostgreSQL support, OTRS article storage and import, outbound webhooks, admin reports, route-level authorization, plugin platform expansion, ops hardening.

GoatFlow is an ITSM and helpdesk system built on the GoatKit platform. It is written in Go as a modular monolith. It covers support ticketing, ITSM features and customization through plugins.

- **GoatKit Plugin Platform** — Dual-runtime (WASM + gRPC), HostAPI, admin UI, CLI tooling, hot reload, signed plugins, periodic health checks, bounded graceful shutdown
- **Plugin Sandbox & Security** — Per-plugin isolation, resource policies, SQL whitelisting, namespace isolation, blue-green reload
- **Custom Fields** — Universal EAV on all entities, 15 field types including GIS, plugin registration, admin UI, REST API, MCP tools
- **Plugin UI System** — Independent plugin UIs with 3 shell types, PWA manifests, per-UI branding, auth, and navigation
- **Organisations & Multi-Tenancy** — Org entity with hierarchy, user membership, per-org sysconfig, automatic HostAPI query scoping
- **Secure Settings** — AES-256-GCM encrypted plugin secrets via HostAPI, org-scoped, platform-managed key
- **Entity Deletion** — Soft delete with recycle bin, PII anonymisation, hard delete with cascade, tombstone logging, auto-purge
- **Plugin Marketplace** — `gk install/update/search` CLI, GitHub Releases backend, dependency resolution, theme-as-plugin
- **LDAP / Active Directory Login** — agent login against a directory, optional auto-create and name/email sync, group-based access and admin mapping, provider order via `AUTH_PROVIDERS` (see [docs/LDAP.md](docs/LDAP.md))
- **Self-Service Authentication** — forgotten-password reset for agents and customers (one-hour, single-use links), customer self-registration with email confirmation (`features.registration`, off by default), per-IP and per-recipient rate limits
- **Reusable UI Components** — 7 components: daily-queue, week-calendar, progress-bar, stat-card, quick-action, file-dropzone, presence-indicator
- **Statistics & Reporting** — Admin → Reports & Analytics (`/admin/reports`), statistics API (`/api/v1/statistics/*`), CSV and JSON export, results limited to queues the viewer can read
- **Two-Factor Authentication** — TOTP and passkeys (WebAuthn) for agents and customers, per-key management, recovery codes, admin override
- **API Tokens** — Personal access tokens with scoped permissions and configurable expiration; admin API access needs the `admin:*` scope
- **REST API v1** — OpenAPI 3.0, Swagger UI, refresh tokens
- **Outbound Webhooks** — Admin → Webhooks, 10 ticket and article events, HMAC-signed deliveries, retries with backoff, delivery log with redeliver
- **MCP Server v2** — Dynamic API discovery with SSE transport, auto-generated tools from REST API + plugins, API bridge with full RBAC
- **Granular RBAC** — OTRS-compatible permission service; every route's access is checked by a route-table test (`TestRouteAuthorizationMatrix`)
- **Databases** — MySQL/MariaDB and PostgreSQL, same migrations and the same test suite on both
- **OTRS Compatibility** — attachments stored in the database or in an OTRS ArticleStorageFS tree (`goatflow-storage` CLI), OTRS 6 / Znuny 6 importer (`goatflow-migrate`)
- **Demo Mode** — Restricted mode for public demos (session-only prefs, blocked password/MFA changes)
- **Coachmarks** — Declarative onboarding tooltips (7 tips) with view tracking, dismissal persistence, and i18n
- **Wallpaper Toggle** — Per-theme background wallpaper control with cookie persistence
- **Mobile Optimization** — Responsive tables, GridStack breakpoints, touch targets, mobile ticket creation, compact modals
- **PWA & Push Notifications** — Web app manifest, service worker with offline fallback, VAPID push subscriptions, browser notification delivery
- **Setup Assistant** — first-run wizard + re-runnable task catalog, plugin-extensible (`GKRegistration.SetupTasks`), JSON API at `/api/v1/admin/setup/*`
- **Identity Providers (SAML2 + OIDC)** — SAML2 via `crewjam/saml`, OIDC client (Google + generic), per-org IdP config, login-page IdP buttons
- **Platform/Product Decoupling** — plugin runtime separated from product code into `internal/platform/` (Phases 1–8), boundary linter enforced
- **Customer Knowledge Base Pages** — list, search, and article view in the customer portal (served by the goat-kb plugin)
- **Customer Company Pages** — customers see their own company and its users; admins see each company's users, tickets and services
- **Ops Hardening** — Prometheus `/metrics` (+ optional dedicated `METRICS_PORT` listener), structured JSON logging, `/health` checks the database, graceful shutdown with connection draining
- **HostAPI Expansion** — markdown/PDF rendering, article + attachment management, ticket views, ticket-state changes for first-party plugin consumers
- **Security Cleanup** — all 11 open Dependabot alerts cleared (7 high), tooling on bun 1.3

### What Works
- Agent Interface: Full ticket management with bulk actions and multi-theme UI (4 themes with wallpaper toggle)
- Customer Portal: self-service with profile management, password changes, **forgotten-password reset**, **self-registration with email confirmation (opt-in)**, **company info pages**
- Email Integration: POP3/IMAP + RFC-compliant threading + auto-responses
- Database: MySQL/MariaDB and PostgreSQL (full PostgreSQL support from 0.10.0). Oracle and SQL Server are not implemented.
- Automation: GenericAgent, ACLs, SLA escalations, ticket attribute relations
- Integration: GenericInterface with outbound REST/SOAP invokers, webservice dynamic fields, **outbound webhooks**
- Security: Group-based queue permissions, session management, auth middleware, **LDAP login**, **API tokens**, **RBAC-filtered statistics**, **TOTP and passkey 2FA**, **CSP headers**, **secure plugin secrets**
- i18n: 15 languages including RTL support (ar, he, fa, ur)
- Deployment: Docker Compose and TrueNAS SCALE app; a Kubernetes Helm chart exists but has known limitations in 0.10.0 (see [charts/goatflow/README.md](charts/goatflow/README.md#known-limitations-0100-chart)); multi-arch images, **demo mode**, **K8s pod isolation for plugins**
- Admin Modules: 30+ admin interfaces including ticket attribute relations, dynamic fields, templates, **custom fields**, **recycle bin**, **organisation management**
- **Plugins**: Dual-runtime (WASM + gRPC) plugin system with admin UI, sandbox isolation, signed verification, state persistence, **custom fields**, **plugin UIs**, **marketplace**, **dependency resolution**, **theme-as-plugin**
- **PaaS Core**: Universal custom fields, plugin UI system, organisations with multi-tenancy, secure settings, entity deletion with GDPR anonymisation
- **API Documentation**: OpenAPI 3.0 spec with Swagger UI, regenerated from `routes/*.yaml` with `make api-docs`
- **RBAC**: Granular permission service, route authorization matrix test, **entity.hard_delete permission**
- **Accessibility**: WCAG 2.1 AA keyboard navigation, skip-to-content, focus management, screen reader announcements
- **Mobile**: Responsive tables with column hiding, touch-friendly controls (44px targets), mobile ticket creation, compact modals
- **PWA**: Web app manifest, service worker, offline fallback page, push notifications for ticket reminders

---

## 🔮 Future Roadmap

### 0.10.0 - Release candidate (not yet tagged)

**Sign-in and self-service, PostgreSQL, OTRS storage and import, webhooks, reports, route-level authorization, plugin platform, ops hardening**

First release since the platform/product split. Full list of changes:
[CHANGELOG.md](CHANGELOG.md) section `[0.10.0]`.

**Sign-in and self-service**
- [x] LDAP / Active Directory agent login (`LDAP_*` variables, Helm
      `config.ldap.*`); provider order with `AUTH_PROVIDERS=ldap,database`;
      invalid LDAP settings stop the server at startup
- [x] Forgotten-password reset for agents (`/forgot-password`) and customers
      (`/customer/forgot-password`): one-hour, single-use links; links are
      built from `BASE_URL` and are not sent while it is unset
- [x] Customer self-registration (`/customer/register`) with a 24-hour email
      confirmation link; off by default (`features.registration`)
- [x] The agent "Sign up" page and the `POST /api/auth/register` stub are
      removed: administrators create agents
- [x] Passkey management in Profile (list, last used, remove), recovery
      codes for passkey-only users, "Other ways to sign in" on the 2FA step
- [x] "Disable 2FA" removes the authenticator app, every passkey and the
      recovery codes; recovery codes work without an authenticator app

**Admin and reporting**
- [x] Admin → Reports & Analytics (`/admin/reports`): totals, trend chart,
      per-queue counts, agent activity, top customers, CSV and JSON export
- [x] Admin → Webhooks (`/admin/webhooks`) and the `/api/v1/webhooks` API:
      10 ticket and article events, HMAC-signed deliveries
      (`X-Webhook-Signature`), retries with backoff, delivery log and
      redeliver. Sent by the runner's `webhook-dispatch` task
- [x] Customer company pages: `/customer/company` and
      `/customer/company/users` in the portal;
      `/admin/customer/companies/:id/users`, `/tickets` and `/services` for
      admins
- [x] Canned responses read and write the `canned_response` table (create,
      update, delete, share, import, export)
- [x] Admin pages fixed: Clone Permissions, Groups search/filter/sort,
      agent editing with the agent password policy, web service dynamic
      fields, disabled queues listed, admin writes record the signed-in admin

**Databases and OTRS compatibility**
- [x] PostgreSQL fully supported: about 470 queries moved onto the
      conversion layer, 70 `LastInsertId()` calls removed, Go test suite
      passes on MySQL/MariaDB and PostgreSQL
- [x] MySQL and PostgreSQL migration sets aligned (29 versions each);
      migration `000027` adds the 267 missing PostgreSQL foreign keys
- [x] `database.ConvertUpsert` for portable upserts
- [x] SQL lint in `cmd/gk-lint` (`make lint-platform`, pre-commit):
      `sql-unconverted`, `sql-last-insert-id`, `sql-mysql-only`,
      `sql-postgres-only`, `sql-unknown-table`, `sql-unknown-column`
- [x] OTRS-compatible article storage: database (`storage.type: db`,
      default) or an OTRS ArticleStorageFS tree (`fs`);
      `goatflow-storage status|migrate|verify` copies between them (see
      [docs/ARTICLE_STORAGE.md](docs/ARTICLE_STORAGE.md))
- [x] OTRS importer (`goatflow-migrate`) covers the whole OTRS 6 / Znuny 6
      schema, reads a database directly (`-source`) or a mysqldump (`-sql`),
      runs in one transaction and sets the ticket number counters
- [x] `POST /api/v1/search` uses a portable `database` backend that works on
      both databases
- [x] Oracle and SQL Server are declared but not implemented: they return
      `ErrDatabaseNotImplemented`

**Authorization and security**
- [x] Every route's access is checked by `TestRouteAuthorizationMatrix`; a
      new `agent` middleware keeps customer logins off agent routes
- [x] An API token is admin only with the `admin:*` scope and a current
      admin owner
- [x] Admin status comes only from admin-group membership
- [x] Plugin routes, plugin UIs, the plugin API and MCP enforce the caller's
      permissions; public plugin UIs are rate limited (`UISpec.RateLimit`,
      default 60 per minute); plugins can authorize event subscriptions
      (`EventAuthorizer`)
- [x] Internal notes are no longer shown to customers
- [x] Script injection fixed in Admin → Groups and the dashboard
      recent-tickets fragment
- [x] Test and demo login shortcuts deleted

**Real data or a clear error**
- [x] Ticket write APIs save what they report (`/api/tickets/:id/reply`,
      messages, internal notes, `PUT /api/tickets/:id`); ticket history,
      search and filter read real data
- [x] Dashboard widgets show "currently unavailable" instead of fake zeros
- [x] Placeholder pages and stub endpoints removed (`/admin/settings`,
      `/admin/backup`, canned dashboard endpoints, in-memory merge
      endpoints); unused GraphQL scaffold deleted

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
- [x] Plugin menu location `profile` (Profile → Connected accounts) for
      per-user settings pages, such as connecting your own calendar
- [x] Plugins receive the user's language as `_lang` on route and plugin
      UI calls; plugin pages and YAML routes now see the user's language
- [x] Plugin menu items sort by `Order` and translate their labels
- [x] WASM plugins get the real system clock and a `time_now` host function

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

**Ops hardening**
- [x] Prometheus metrics endpoint with custom metrics — real exposition format on `/metrics` (default registerer; cache metrics via `promauto`) + `goatflow_up` / `goatflow_process_start_time_seconds` gauges; optional dedicated listener on `METRICS_PORT` when `METRICS_ENABLED=true`
- [x] Structured JSON logging (configurable levels) — `internal/platform/logging` drives both `slog` and legacy stdlib `log` from `LOG_FORMAT` / `LOG_LEVEL` / `LOG_OUTPUT` (`LOG_FILE_PATH` legacy alias)
- [x] Health probes — `GET /health` (and `/healthz`, same handler) does a short-timeout database ping and answers 503 when the database is unreachable; used for liveness and readiness. Admin-only `GET /health/detailed` adds the cache check, version and uptime. Wired to the Dockerfile `HEALTHCHECK`, the TrueNAS app and the Helm probes
- [x] Graceful shutdown handling with connection draining — `http.Server` + SIGTERM/SIGINT: stop accepting, drain in-flight requests up to `DRAIN_TIMEOUT` (default 10 s), then the existing bounded plugin shutdown
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

**Packaging, docs and tests**
- [x] API license metadata aligned to Apache-2.0 (OpenAPI specs, Swagger
      docs, swagger annotation)
- [x] Route YAML schema normalised (`apiVersion: goatflow.io/v1`);
      `make api-docs` regenerates the OpenAPI and Swagger docs from
      `routes/*.yaml`
- [x] TrueNAS SCALE added to the supported platforms list; app template
      pinned to 0.10.0
- [x] Helm chart: `appVersion` 0.10.0 + `kubeVersion` gate, real
      `ghcr.io/goatkit/goatflow` image reference, the `v`-prefix stripped
      from the CI-published appVersion, `config.baseUrl`, `config.ldap.*`
      and `config.authProviders` values
- [x] E2E make targets `test-e2e-go` and `test-e2e-playwright-go` (own
      tmpfs for `/tmp`); MariaDB and PostgreSQL test DB init scripts apply
      every migration

**Not in 0.10.0 (known gaps)**
- [ ] Helm chart does not give a working install yet: no background runner
      (no outgoing email, webhook delivery or session cleanup), the Ingress
      only reaches the backend for `/api/` and `/ws`, `GOATFLOW_SECURE_KEY`
      is not set, and several environment names are ones GoatFlow does not
      read. See [Known limitations (0.10.0 chart)](charts/goatflow/README.md#known-limitations-0100-chart)

---

### 1.0.0 - Target: November 2026

**Production Release**

*Feature Complete*
- GoatKit PaaS platform GA (WASM + gRPC runtimes, custom fields, plugin UIs, multi-tenancy)
- All OTRS core modules operational
- First-party open source plugins shipped:
  - FAQ/Knowledge Base (articles, search, portal) — *done: goat-kb (`github.com/goatkit/goat-kb`, Apache-2.0) serves the customer portal KB pages*
  - Calendar & Appointments (scheduling, iCal) — *not started. The enterprise goatkit-calendar plugin covers one-way push to Google and Microsoft 365 calendars only*
  - Process Management (workflows, designer)
- Statistics & Reporting — *done in core, not as a plugin: Admin → Reports & Analytics (`/admin/reports`) and `/api/v1/statistics/*` (0.10.0)*

*Security*
- Third-party security audit completed
- Automated dependency vulnerability scanning — *partial: Dependabot is active (all open alerts cleared in 0.10.0); Snyk is not set up*
- Security hardening guide and best practices
- OWASP Top 10 compliance verification
- Rate limiting and DDoS protection — *partial: login, passkey, self-service forms, plugin webhooks and public plugin UIs are rate limited; there is no general API rate limit*
- Security response policy and CVE process

*Performance*
- 1000+ concurrent users verified under load
- Sub-100ms response times (p95) for all endpoints
- Database query optimization with indexes
- Caching layer tuning (Valkey cache already in place)
- Connection pooling tuning
- CDN integration for static assets

*Documentation*
- Administrator guide with best practices
- API reference (OpenAPI 3.0) with interactive docs — *partial: Swagger UI and `make api-docs` exist; the hand-written `api/openapi.yaml` still lists some routes that do not exist*
- Deployment guides (Docker, Kubernetes, cloud providers) — *partial: Docker Compose, Helm and TrueNAS guides exist; no cloud provider guides*
- Migration guide from OTRS 6.x with automation scripts — *done in 0.10.0: [docs/OTRS_MIGRATION_GUIDE.md](docs/OTRS_MIGRATION_GUIDE.md), `goatflow-migrate`, `make migrate-analyze` / `migrate-import` / `otrs-import`*
- Plugin development guide (custom fields, UIs, enterprise plugin patterns)
- Troubleshooting guide with common issues
- Video tutorials and screencasts

*Quality*
- 85% test coverage (unit + integration)
- Comprehensive Playwright E2E test suite — *partial: Go Playwright suites in `tests/e2e/` run with `make test-e2e-go` / `test-e2e-playwright-go`*
- Chaos engineering tests for resilience
- Performance regression testing in CI
- Automated smoke tests on production deployments

*Calendar & Appointments Plugin* (not started)
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
| goatkit-calendar | One-way push of events into each user's own Google Calendar or Microsoft 365 calendar (per-user OAuth2, set up under Profile → Connected accounts); nothing is read back |

**Status (October 2026):**

| Status | Plugins |
|--------|---------|
| Released (v1.0.0) | goatkit-llm, goatkit-chat, goatkit-rag |
| In development | goatkit-media, goatkit-billing, goatkit-devices, goatkit-workflows, goatkit-audit, goatkit-content-feeds, goatkit-notify, goatkit-memory, goatkit-tts, goatkit-calendar |
| In planning | goatkit-subscriptions, goatkit-invoicing, goatkit-payments, goatkit-maps |

**Enterprise enquiries:** Enterprise plugins are available as paid add-ons. Contact us at [hello@goatflow.io](mailto:hello@goatflow.io) for enterprise enquiries, licensing, and support.

---

## 📊 Version Summary

| Version | Date | Status | Theme |
|---------|------|--------|-------|
| 1.0.0 | Nov 2026 | 🔮 Future | Production Release |
| 0.10.0 | Unreleased | 🚧 Release candidate | Sign-in and self-service, PostgreSQL, OTRS storage and import, webhooks, reports, route-level authorization |
| 0.9.0 | Aug 2026 | 🚀 Current | Setup Assistant, SAML2 + OIDC identity providers, platform/product decoupling, customer KB pages |
| 0.8.3 | May 2026 | ✅ Released | Plugin Auto-Restart, Plugin UI Offline, WebAuthn, Quality |
| 0.8.2 | Apr 2026 | ✅ Released | **MCP v2** + Plugin Manager Resilience (health checks, bounded shutdown) |
| 0.8.1 | Apr 2026 | ✅ Released | Mobile, PWA & Security |
| 0.8.0 | Mar 2026 | ✅ Released | **PaaS Core** — Custom Fields, Plugin UIs, Multi-Tenancy, Deletion |
| 0.7.0 | Mar 2026 | ✅ Released | Plugin Platform Complete, Sandbox & Security, Statistics API |
| 0.6.5 | Feb 2026 | ✅ Released | 2FA, API Tokens, RBAC, Demo Mode, Plugin Platform, MCP Server |
| 0.6.4 | Feb 2026 | ✅ Released | Plugin Platform Roadmap |
| 0.6.3 | Jan 2026 | ✅ Released | Stability & Testing |
| 0.6.2 | Jan 2026 | ✅ Released | Multi-Theme System |
| 0.6.1 | Jan 2026 | ✅ Released | Automation & ACLs |
| 0.6.0 | Jan 2026 | ✅ Released | Admin modules (system maintenance, sessions), GenericAgent engine |
| 0.5.1 | Jan 2026 | ✅ Released | Polish & Portability |
| 0.5.0 | Jan 2026 | ✅ Released | MVP Release |
| 0.4.0 | Oct 2025 | ✅ Released | Filters & Typeahead |
| 0.3.0 | Sep 2025 | ✅ Released | Rich Text & Dark Mode |
| 0.2.0 | Sep 2025 | ✅ Released | YAML Routing |
| 0.1.0 | Aug 2025 | ✅ Released | Foundation |

0.8.4 was planned but never tagged. Its OIDC client work shipped in 0.9.0.

---

## Get Involved

Want to influence the roadmap? Open a [GitHub Discussion](https://github.com/goatkit/goatflow/discussions).
