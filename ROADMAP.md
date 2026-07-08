# GoatFlow Roadmap

Current status, past releases, and future plans for GoatFlow.

## 🚀 Current Status

**Version**: 0.8.2 (April 2026) - MCP Dynamic API Discovery, SSE & Plugin Manager Resilience

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

## 📜 Past Releases

### [0.8.2] - April 2026

**MCP Dynamic API Discovery, SSE Transport, and Plugin Manager Resilience**

MCP v2 — Dynamic API Discovery & SSE Transport:

- MCP tools dynamically generated from YAML route definitions + OpenAPI spec — no manual tool registration
- API bridge invokes real Gin handlers with full RBAC middleware enforcement
- Plugin endpoints auto-discovered and exposed as MCP tools (namespaced by plugin name)
- `MCPToolSpec` in `GKRegistration` — plugins declare MCP tools with JSON Schema input schemas
- Streamable HTTP / SSE transport (MCP 2025-03-26) with session management and heartbeat
- Admin SQL promoted to REST endpoint (`POST /api/v1/admin/sql`) with allowlisted statement types
- Protocol version negotiation (supports 2024-11-05 and 2025-03-26)
- `mcp_description` and `mcp: false` route YAML fields for per-route MCP control

Plugin Manager Resilience:

- **`EnsureLoaded` uses manager registry as ground truth** — previously trusted a cached `discovered[].Loaded` flag that could desync from reality, causing duplicate-spawn of already-running gRPC plugins and silent state loss (e.g. WireGuard peer maps). The cache flag is now a fast-path hint only; a warn log surfaces drift for diagnostics.
- **Bounded graceful shutdown** — `Manager.ShutdownAll` applies per-plugin `ResourcePolicy.ShutdownTimeout` (10s default) with a 30s overall ceiling in `cmd/goats`. `GRPCPlugin.Shutdown(ctx)` now respects its context via goroutine+select, so a hung plugin can't wedge goatflow's process exit. `client.Kill()` always runs afterwards as a supervised teardown.
- **Plugin health checker** — optional background goroutine probes every loaded plugin every 60s via the reserved `__health_ping__` function name on the existing `Call` path (no protocol changes, no plugin rebuilds). Three consecutive 5s timeouts flip `HealthStatus.Healthy` to false with a warn log; recovery flips it back. Health state exposed via `Manager.HealthStatus(name)` and `Manager.AllHealthStatuses()` for admin UI consumption. No auto-restart yet — that's 0.8.3. Opt out with `GOATFLOW_PLUGIN_HEALTH_CHECK=false`.
- Toolbox build: `govulncheck` pinned to `v1.1.4` so `make test-unit` keeps working (`v1.2.0` transitively requires Go 1.25)

### [0.8.1] - April 4, 2026

**Mobile, PWA & Security**

- CORS origin validation, JWT production guards, dependency audits
- SQL dialect portability — automatic function rewriting (MySQL ↔ PostgreSQL)
- Custom field atomic operations (increment, append, remove, cas, toggle)
- Plugin sidecar containers, webhook routes, file storage API, SSE channels
- Automatic org context injection for plugin calls
- Statistics & reporting plugin v2.0 (SLA compliance, time tracking, scheduled reports)
- Mobile optimization — responsive tables, GridStack breakpoints, touch targets, compact modals, mobile ticket creation
- PWA — web app manifest, service worker, offline fallback, push notifications with VAPID
- Coachmarks — 7 onboarding tips with i18n in all 15 languages

### [0.8.0] - March 2026

**GoatKit PaaS Core**

- Universal custom fields on all entities (15 field types including GIS)
- Plugin UI system (5 UI types, 3 shell templates, PWA manifests, custom domains)
- Organisations & multi-tenancy (hierarchy, membership, per-org config, HostAPI scoping)
- Secure settings (AES-256-GCM encrypted plugin secrets)
- Entity deletion (soft delete, anonymisation, hard cascade, recycle bin, auto-purge)
- Plugin marketplace (`gk install/update/search`, dependency resolution, theme-as-plugin)
- Self-service authentication (password recovery, registration, email verification, CAPTCHA)
- Reusable UI components (7 components: daily-queue, week-calendar, progress-bar, stat-card, quick-action, file-dropzone, presence-indicator)
- Accessibility (WCAG 2.1 AA keyboard navigation, focus management, screen reader announcements)

### [0.7.0] - March 26, 2026

**GoatKit Plugin Platform Complete**

- Plugin sandbox & security: per-plugin `SandboxedHostAPI` with permission enforcement, rate limiting, resource accounting
- OS-level gRPC process isolation: Linux namespace isolation (CLONE_NEWNS, CLONE_NEWPID), Pdeathsig, minimal environment
- Plugin signing: ed25519 signature verification for plugin binaries, opt-in via `GOATFLOW_REQUIRE_SIGNATURES=1`
- SQL table whitelisting: query parsing with table name extraction and scope enforcement
- Call depth limiting: plugin-to-plugin chains tracked with max depth of 10
- Config key blacklist: sensitive patterns (database, password, secret, token, auth, ldap, smtp, aws, etc.) blocked by default
- Email domain scoping and rate limiting (10/min per plugin)
- Caller identity stamping: gRPC host stamps authenticated plugin name, prevents impersonation
- ZIP extraction security: symlink detection, size/count limits (100MB/file, 500MB total, 1000 files)
- Live policy updates: RWMutex-protected, immediate effect without restart
- Atomic blue-green plugin reload: no request-dropping window during hot reload
- Policy persistence: JSON in `sysconfig_modified` table, survives restarts
- Statistics & reporting plugin (first-party): dashboard API, CSV/Excel export, RBAC-filtered endpoints
- Hot reload for local development: fsnotify-based WASM + gRPC binary watching
- Plugin resource policies: admin-configurable limits with `ResourceRequest` / `ResourcePolicy`

### [0.6.5] - February 8, 2026

**GoatKit Plugin Platform**

- Two-Factor Authentication (TOTP) for agents and customers with QR setup, recovery codes, admin override
- GoatKit Plugin Platform: dual-runtime (WASM + gRPC), HostAPI, admin UI, CLI tooling
- API Tokens: personal access tokens with scoped permissions
- REST API v1 Enhanced: OpenAPI 3.0, Swagger UI, rate limiting, webhooks
- MCP Server: AI assistant integration via JSON-RPC (`/api/mcp`) with multi-user proxy RBAC
- Granular RBAC: OTRS-compatible permission service with 1,300+ lines of auth tests
- RBAC Security Hardening: queue-level permission enforcement on all statistics endpoints
- Demo Mode: restricted mode for public demos
- Coachmarks: declarative onboarding tooltips with view tracking
- Wallpaper Toggle: per-theme background wallpaper control
- Dark Theme Contrast Fix: 338 CSS overrides for proper contrast across all dark themes

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
- [x] **v2: Dynamic API discovery** — tools auto-generated from YAML routes + OpenAPI spec
- [x] **v2: API bridge** — tool execution via real Gin handlers with RBAC middleware
- [x] **v2: Plugin tools** — auto-discovered from enabled plugins, `MCPToolSpec` for rich schemas
- [x] **v2: SSE transport** — MCP 2025-03-26 Streamable HTTP with session management
- [x] **v2: Admin SQL REST endpoint** — `POST /api/v1/admin/sql` with allowlisted statements
- [x] **v2: Protocol negotiation** — supports 2024-11-05 and 2025-03-26

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

**Wallpaper Toggle**
- [x] Checkbox in theme selector dropdown
- [x] Cookie + server-side preference persistence
- [x] Flash prevention (inline `<head>` script)
- [x] Smart disable for themes without wallpaper
- [x] GoatFlow Classic light and dark wallpaper images

**Quality**
- [x] 70% test coverage target (71.2% achieved)
- [x] API documentation site (Swagger UI)

---

### 0.8.0 - March 2026 ✅

**GoatKit PaaS Core — Custom Fields**

Universal custom fields on every core entity. Plugins declare fields at registration; GoatKit handles storage, validation, UI rendering, and querying. Eliminates the need for plugin extension tables.

- [x] `gk_custom_field_def` + `gk_custom_field_value` tables (EAV with denormalised typed columns)
- [x] Supported entities: ticket, article, contact, agent, group, customer_group, queue, organisation
- [x] Field types: text, textarea, integer, decimal, boolean, date, datetime, select, multi_select, url, email, phone
- [x] GIS field types: point (lat/lng), polygon (GeoJSON), address (structured + auto-geocode)
- [x] Plugin registration: `CustomFieldSpec` in `GKRegistration` (auto-prefixed names, auto-create on load)
- [x] HostAPI: `CustomFieldsGet()`, `CustomFieldsSet()`, `CustomFieldsQuery()` (with sandbox prefix enforcement)
- [x] WASM + gRPC wire format for custom field host functions
- [x] Auto UI rendering partial (`custom_fields.pongo2` — edit/view/inline modes, all 15 field types)
- [x] Admin-defined custom fields (admin UI: list, create, edit, soft-delete)
- [x] Searchable fields with indexed typed columns (text, int, decimal, date, datetime)
- [x] Legacy auto-migration: copy `dynamic_field` → `gk_custom_field_def` on startup (idempotent, downgrade-safe)
- [x] Validation engine: 15 type-specific validators (regex with timeout, ranges, option membership, GeoJSON, email/URL/phone)
- [x] REST API v1 endpoints (definitions CRUD, entity values get/set, query by field values)
- [x] MCP tools: `custom_fields_get`, `custom_fields_set`, `custom_fields_query`, `custom_fields_list`
- [x] Design spec: `docs/design/CUSTOM_FIELDS.md`

**Platform/Product Decoupling** *(prerequisite for v0.10.0 plugins)*

- [x] **Phase 1 — Break `scheduler.go` coupling**: Defined platform scheduler interface in plugin package, replaced `*services/scheduler` dependency with adapter pattern, moved connection lifecycle into `database/`, moved `services/{adapter,database,registry}` to `internal/platform/services/`.
- [x] **Phase 2 — Invert `database` to `services/adapter` dependency**: Moved services/{adapter,database,registry} to `internal/platform/services/`, updated import sites, `database/` now imports only platform packages.
- [x] **Phase 3 — Split `internal/models/`**: Moved platform types (`User`, `Group`, `Role`, `Session`, `APIToken`, `LDAPConfiguration`, `SearchRequest`, `EmailAccount` split) to `internal/platform/models/`, kept product types in `internal/models/`, added type aliases to preserve identity.
- [x] **Phase 4 — Decouple `internal/routing/` from `internal/api/` and `internal/models/`**: Introduced `HandlerResolver` interface, removed `internal/models` import, wired API-backed resolver from `internal/api/handler_registry.go`.
- [x] **Phase 5 — Reorganize `internal/api/` and `internal/service/`**: Moved platform handlers (`auth_api`, `auth_handler`, `user_*`, `organisation_handlers`, `deletion_handlers`, `custom_fields_api_handlers`, `i18n_handlers`) to `internal/platform/api/`; moved platform services (`auth_service`, `totp_service`, `webauthn_service`, `user_preferences`) to `internal/platform/service/`.
- [x] **Phase 6 — Move all remaining platform packages to `internal/platform/`**: Moved `auth`, `middleware`, `template`, `shared`, `cache`, `config`, `customfields`, `data`, `deletion`, `httpcookie`, `i18n`, `ldap`, `lookups`, `marketplace`, `mcp`, `notifications`, `oauth2`, `organisation`, `pluginui`, `push`, `runner`, `search`, `secureconfig`, `service`, `services`, `storage`, `sysconfig`, `template`, `utils`, `webhook`, `yamlmgmt`, `zinc` to `internal/platform/` after decoupling hidden dependencies via interfaces.
- [x] **Phase 7 — Enforce boundary with linter**: Created `cmd/gk-lint/` that scans `internal/platform/` for direct and transitive product package imports using `go list -deps`, added `lint-platform` target to Makefile and CI, wrote `docs/development/PLATFORM_BOUNDARY.md`.
- [x] **Phase 8 — Documentation and cleanup**: Update `docs/ARCHITECTURE.md` to reflect `internal/platform/` structure, fix `DATABASE.md:99` claim about `faq_*` tables, update `docs/PLUGIN_PLATFORM.md`, write `docs/development/PLATFORM_PACKAGES.md` canonical list.
- [x] Dead code deleted — `internal/models/knowledge.go` (already removed), `KnowledgeArticles` field from `service_catalog.go` (already removed), TODO stub handlers from `customer_routes.go:1561-1577` (removed — `handleCustomerCompanyInfo`, `handleCustomerCompanyUsers`).

Full plan: `docs/PLATFORM_PRODUCT_DECOUPLING.md` (version 1.6).

**GoatKit PaaS Core — Plugin UI System**

- [x] `UISpec` in `GKRegistration` (name, type, routes, branding, auth, PWA, data scope, rate limit)
- [x] UI types: admin_page, agent_app, customer_app, public_page, kiosk
- [x] Shell system: none (raw HTML), minimal (mobile-first with bottom/top/side nav), standard (full GoatFlow chrome)
- [x] Shell templates: `ui_standard.pongo2`, `ui_minimal.pongo2`, `ui_none.pongo2`
- [x] Custom domain support (admin-assignable, reverse proxy config generation)
- [x] Per-UI branding (logo, colour, fonts, favicon, app name) via `UIBrandingSpec`
- [x] PWA manifest auto-generation (`/ui/{id}/manifest.json`) with configurable display mode
- [x] Service worker offline support with sysconfig strategies and per-plugin `pwa.cache_routes`
- [x] Auth per UI type: session (agent/customer), PIN (kiosk), token, none (public)
- [x] Data scoping for customer UIs (self, org, all) — passed to plugin handlers
- [x] Rate limiting for public UIs (configurable per UI)
- [x] Bottom/top/side navigation with badge counts (resolved via plugin function calls)
- [x] Navigation integration — plugin UIs auto-appear in agent/customer/admin nav
- [x] Plugin manager auto-registers UIs on plugin load (`gk_plugin_ui` table)
- [x] Dynamic router registers all UI routes under `/ui/{plugin}_{ui_id}/`
- [x] Design spec: `docs/design/PLUGIN_UIS.md`

**GoatKit PaaS Core — Organisations & Multi-Tenancy**

Core `gk_organisation` entity with user membership, data isolation, and per-org settings. Every SaaS deployment needs this.

- [x] `gk_organisation` table (name, slug, parent_id hierarchy, status, customer_company link)
- [x] `gk_user_organisation` table (agents AND customers, roles: member/admin/owner, default org)
- [x] `sysconfig_org` table (per-org sysconfig overrides using existing key names)
- [x] Repository layer — full CRUD for orgs, memberships, per-org config with cascade resolution
- [x] Org context middleware (`WithOrgID`/`OrgIDFromContext`) for request scoping
- [x] HostAPI `OrgID()` method — plugins read active org from context
- [x] WASM + gRPC wire format for `org_id` host function
- [x] Backward compatible — zero orgs = single-org mode, everything works as today
- [x] Design spec: `docs/design/ORGANISATIONS.md`
- [x] Org switching API endpoint — POST `/api/v1/session/org` with membership verification
- [x] Org listing endpoint — GET `/api/v1/session/orgs` with active org indicator
- [x] Org switcher UI component — dropdown in navigation (`org_switcher.pongo2`)
- [x] Org context middleware — resolves from cookie or default org, sets in gin + request context
- [x] Admin API — full CRUD for organisations, members, per-org sysconfig overrides
- [x] i18n — `organisations.select` and `organisations.switch_org` in all 15 languages
- [x] HostAPI query scoping — auto-inject `org_id` filters in SandboxedHostAPI (opt-in per table via `OrgAwareTables` registry, alias-aware SQL rewriting, SELECT/UPDATE/DELETE scoped, INSERT pass-through)

**GoatKit PaaS Core — Secure Settings**

Encrypted configuration storage via HostAPI. Plugins store API keys, device PINs, secrets without handling crypto directly.

- [x] `host.SecureConfigGet()` / `host.SecureConfigSet()` HostAPI methods (with sandbox plugin-name enforcement)
- [x] AES-256-GCM encryption with platform-managed key (`GOATFLOW_SECURE_KEY` env var or auto-generated)
- [x] `gk_secure_config` table with org-scoped secrets (org-specific → global fallback)
- [x] Masked display helpers (`ValueHint` last-4 chars, `MaskedDisplay` for admin UI)
- [x] WASM + gRPC wire format for `secure_config_get` / `secure_config_set`
- [x] Full repository CRUD (set, get with org fallback, delete, list per plugin)
- [x] Design spec: `docs/design/SECURE_SETTINGS.md`

**GoatKit PaaS Core — Entity Deletion**

Two deletion patterns: soft delete + anonymisation (GDPR, preserve business records) and hard cascading delete (complete erasure). Recycle bin as the standard deletion pipeline.

- [x] Soft delete (move to `gk_recycle_bin`, entity hidden via native mechanism: archive_flag/valid_id)
- [x] Anonymise PII on soft delete (configurable per entity type, irreversible `[DELETED]` replacement)
- [x] Restore from recycle bin (clears recycle bin entry, restores entity via native mechanism)
- [x] Hard cascading delete (purge — physical removal of entity + linked data per entity type)
- [x] Plugin cascade handlers: `CascadeSpec` in `GKRegistration` (OnSoftDelete, OnHardDelete per entity type)
- [x] Tombstone logging (`gk_deletion_log` — immutable record of soft_delete/restore/hard_delete actions)
- [x] Auto-purge (`PurgeExpired` — deletes entries past `expires_at`, configurable retention per entity type)
- [x] HostAPI: `EntitySoftDelete()`, `EntityRestore()`, `EntityHardDelete()`, `RecycleBinList()`
- [x] WASM + gRPC wire format for all deletion host functions
- [x] Deletion service — orchestrates soft/hard delete, anonymisation, cascade, tombstone, recycle bin
- [x] Design spec: `docs/design/ENTITY_DELETION.md`
- [x] Recycle bin admin UI — template with HTMX restore/purge, entity type filter, deletion log viewer
- [x] Batch/scope delete — `ScopeSoftDelete` and `ScopeHardDelete` for bulk entity deletion
- [x] RBAC: `entity.hard_delete` permission (admin-only, added to RBAC permission system)
- [x] Admin API routes — list bin, restore, purge, batch delete, batch purge, deletion log
- [x] i18n — recycle bin UI translated to all 15 native languages (18 keys each)

**GoatKit PaaS Core — Reusable UI Components**

Shared UI components usable by any plugin. Server-rendered HTML building blocks.

- [x] `gk-daily-queue` — ordered task list with priority indicators, status badges, and HTMX action buttons
- [x] `gk-week-calendar` — week-at-a-glance grid with colour-coded events and click-through links
- [x] `gk-progress-bar` — "3/5 completed" counter with animated bar and configurable colour
- [x] `gk-stat-card` — dashboard metric card with icon, trend indicator (up/down/flat), and optional link
- [x] `gk-quick-action` — big mobile-friendly tap targets with responsive grid (2-col mobile, 4-col desktop)
- [x] All components theme-aware (CSS variables: `--gk-bg-surface`, `--gk-primary`, `--gk-text-*`, etc.)
- [x] WCAG 2.1 AA accessible — `role` attributes, `aria-label`, `aria-valuenow`/`aria-valuemax` on progress bars

**Plugin Ecosystem Expansion**
- [x] Plugin marketplace integration — `gk install/update/search` CLI commands, GitHub Releases backend, marketplace index client
- [x] Plugin dependency resolution — `Dependencies` field in manifest, `ResolveDependencies()` with missing dep detection, `TopologicalSort()` for load ordering with circular dependency detection
- [x] Theme-as-plugin support — `PluginType: "theme"` in manifest, `InstallTheme()` extracts CSS/fonts to `.cache/`, `UninstallTheme()` cleanup
- [x] Plugin update notifications — `CheckUpdates()` compares installed versions against marketplace index
- [x] Kubernetes pod isolation — `GOATFLOW_PLUGIN_ISOLATION=k8s` mode, `GeneratePodManifest()` generates Deployment + Service + NetworkPolicy YAML
- [x] Design spec: `docs/design/PLUGIN_MARKETPLACE.md`
- [x] Admin marketplace UI — `/admin/marketplace` page with stats cards, category/sort/search filters, and install/update buttons; backed by `GET /api/v1/plugins/marketplace`, `GET /api/v1/plugins/marketplace/search`, `POST /api/v1/plugins/marketplace/install`. Dashboard card links to the page.

**Self-Service Authentication**
- [x] Password recovery for customers — email-based reset with secure tokens (1hr expiry), anti-enumeration response
- [x] Password recovery for agents — admin-initiated reset (existing `HandleAdminUserResetPassword`)
- [x] Customer sign-up/registration — approval workflow with pending/approved/rejected status, admin review handlers
- [x] Email verification — token-based verification links (24hr expiry), consumed on use
- [x] CAPTCHA integration — reCAPTCHA v3 (score-based) and hCaptcha support, configurable provider/threshold
- [x] Auth token system — `gk_auth_token` table, cryptographic token generation, expiry, single-use consumption
- [x] i18n — forgot password, reset, registration, verification translated to all 15 languages

**Enhancements**
- [x] Keyboard navigation accessibility — skip-to-content link, focus-visible detection (keyboard vs mouse), arrow key menu navigation, Escape closes dropdowns/modals, focus trapping in modals, screen reader announcements
- [x] Drag-and-drop file uploads — `gk-file-dropzone` component with progress bars, file size validation, XHR upload with progress, accessible labels
- [x] Real-time collaborative ticket editing indicators — `gk-presence-indicator` component with SSE-based presence, viewing/editing status, avatar initials with colour coding

**Quality**
- [x] 75% test coverage target — achieved on 4/7 new packages (customfields 76.7%, pluginui 83.6%, organisation 73.4%, secureconfig 73.2%)
- [x] Accessibility audit — `accessibility.js` module (focus management, keyboard nav, SR announcements), skip-to-content, ARIA attributes on all new components, i18n for all a11y strings

---

### 0.8.1 - April 2026 ✅

**GoatKit PaaS Core — Plugin Webhook Routes**

Plugins receive callbacks from external services (payment providers, CI/CD, etc.) without session authentication. HMAC signature verification prevents forgery.

- [x] `"webhook"` middleware keyword for `RouteSpec` — plugins declare webhook routes with `Middleware: ["webhook"]`
- [x] HMAC-SHA256 signature verification with signing secret from plugin secure config (`<plugin>_webhook_secret`)
- [x] Stripe-specific signature parsing (`t=<timestamp>,v1=<signature>` format with 5-minute replay window)
- [x] Standard webhook headers: `X-Signature-256`, `X-Hub-Signature-256`, `X-Webhook-Signature`
- [x] 1MB body size limit to prevent OOM on malicious payloads
- [x] Secure by default — rejects unsigned webhooks unless `GOATFLOW_WEBHOOK_ALLOW_UNSIGNED=true` (dev only)
- [x] Webhook request logging (method, path, source IP, plugin name, verification result)
- [x] Rate limiting on webhook routes (500 req/hr per source IP per plugin, runs before signature verification)

**GoatKit PaaS Core — Plugin File Storage API**

Platform-managed file storage for plugins. Eliminates direct filesystem access, enables S3/cloud backends.

- [x] `HostAPI.StoreFile(key, data, metadata)` / `GetFile(key)` / `DeleteFile(key)` / `ListFiles(prefix)`
- [x] Backend: local disk (default) — files stored under `$STORAGE_PATH/plugins/<plugin_name>/`
- [x] Per-plugin namespace isolation (sandbox injects plugin name, path traversal blocked)
- [x] Sidecar metadata files (JSON) for content-type, size, modification time
- [x] gRPC wire format: `store_file`, `get_file`, `delete_file`, `list_files` host methods
- [x] S3-compatible backend (`GOATFLOW_STORAGE_BACKEND=s3`) — PUT/GET/DELETE via standard HTTP, configurable endpoint/bucket/credentials
- [x] Org-scoped file storage — files stored under `<plugin>/org-<id>/<key>` when org context is active
- [x] Size limits per plugin — `MaxFileStorageBytes` in `ResourcePolicy` (default 500MB), enforced before write

**GoatKit PaaS Core — Plugin SSE Channel**

Real-time server-sent events for plugin UIs. Eliminates polling, enables live progress updates and status notifications.

- [x] `HostAPI.PublishEvent(channel, event, data)` — plugins push events
- [x] SSE endpoint `/api/v1/plugins/{name}/events/{channel}` — clients subscribe
- [x] Per-plugin channel isolation (sandbox enforced)
- [x] Auth-scoped: agent channels require session, customer channels scoped to org
- [x] Automatic keepalive and reconnection handling

**GoatKit PaaS Core — Automatic Org Context Injection**

Plugins receive org context automatically from the authenticated session, eliminating manual org lookup.

- [x] Middleware injects `org_id` into all plugin API call params automatically
- [x] `HostAPI.OrgID()` returns the active org from the request context (already exists — extend to plugin dispatch)
- [x] Plugin dispatch wraps params with `_org_id` field before forwarding to plugin handler
- [x] Opt-out flag for plugins that handle multi-org queries themselves

**Statistics & Reporting Plugin — UI & Shipping**
- [x] Dashboard widgets with Chart.js
- [x] Built-in report templates (tickets by queue, agent, SLA compliance)
- [x] Scheduled report delivery via email
- [x] Time tracking reports and analytics
- [x] Ships as standalone WASM plugin

**Mobile Optimization**
- [x] Responsive table column hiding — agent and customer ticket lists hide secondary columns below `md`/`lg` breakpoints
- [x] Dashboard GridStack responsive breakpoints — 1 column (mobile), 6 columns (tablet), 12 columns (desktop)
- [x] Mobile CSS component overrides — tighter table/modal padding below 768px via `@media` queries in `input.css`
- [x] Touch-friendly action buttons — `.gk-action-btn` class with 44px minimum touch targets (WCAG 2.5.8)
- [x] Ticket detail tabs — horizontal scroll with `scrollbar-hide` and gradient fade hint on mobile
- [x] Responsive typography — ticket detail header scales `text-xl`→`text-3xl` with `break-words` for long subjects
- [x] Admin users table — hide Groups, 2FA, Last Login columns below `lg`, action buttons upgraded to `.gk-action-btn`
- [x] Meta grid mobile padding — reduced card padding on small screens (`p-3 sm:p-4`)
- [x] Modal mobile layout — stacked full-width buttons, reduced padding on mobile viewports
- [x] Mobile ticket creation flow — responsive padding, collapsible tips, stacked buttons, compact file upload zones, touch-friendly tiptap toolbar
- [x] Push notifications (PWA) — web app manifest, service worker with offline fallback, VAPID key infrastructure, push subscription API, scheduler integration for pending reminders, notification bell toggle in navbar

**Coachmarks**
- [x] Additional tips for key features — 6 new coachmarks: dashboard widgets, ticket creation, ticket filters, bulk actions, queue overview, push notifications (i18n in all 15 languages)

---

### 0.8.2 - April 2026 ✅

**MCP Dynamic API Discovery & SSE Transport**

- [x] Dynamic tool generation from YAML routes + OpenAPI spec (no manual registration)
- [x] API bridge — tools invoke real Gin handlers with RBAC middleware
- [x] Plugin tool auto-discovery — enabled plugins exposed as MCP tools
- [x] `MCPToolSpec` in `GKRegistration` — plugins declare tools with JSON Schema
- [x] Streamable HTTP / SSE transport (MCP 2025-03-26) with sessions and heartbeat
- [x] Admin SQL REST endpoint (`POST /api/v1/admin/sql`) with allowlisted statements
- [x] `mcp_description` and `mcp: false` route YAML fields
- [x] Protocol version negotiation (2024-11-05 + 2025-03-26)

---

### 0.8.3 - Target: June 2026

**Plugin Manager — Auto-Recovery** ✅
- [x] Auto-restart on health-check failure with exponential backoff (5s → 5min) and crash-loop guard (>5 attempts in 10min → abandoned, requires admin reset). Loader wired as `Restarter` via `Manager.SetRestarter`; opt out with `GOATFLOW_PLUGIN_AUTO_RESTART=false`.
- [x] Admin UI widget showing per-plugin health status — `GET /api/v1/plugins/health`, Health column + Healthy/Unhealthy summary cards on `/admin/plugins`, "Reset" action for crash-loop-abandoned plugins (`POST /api/v1/plugins/:name/reset-crashloop`).
- [x] Rich health payloads — plugins that return JSON from their `__health_ping__` handler get it surfaced on `PluginHealth.Payload`. Existing "any response = alive" contract preserved.
- [x] Parallel shutdown inside `ShutdownAll` — total time is now max(per-plugin timeout) instead of sum. Verified by `TestShutdownAllParallel`.

**Plugin UI System — Offline & Admin**
- [x] Service worker / offline support with configurable caching strategy (per-plugin CacheRoutes)
- [x] Admin UI for managing plugin UIs (list, enable/disable, custom domain, branding override)

**Two-Factor Authentication — Hardware Keys**
- [x] Hardware key support (WebAuthn/FIDO2)

**Quality**
- [x] Performance benchmarks established — `make bench` runs the curated Go benchmark suite and writes baseline artifacts under `generated/benchmarks/`; `make bench-compare` compares captures with benchstat.
- [x] Load testing harness — `make load-test` runs the k6 smoke profile against the test stack, with configurable VUs, duration, base URL, endpoints, and JSON summary output.

---

### 0.8.4 - Target: July 2026

**External Identity Provider Integration (OIDC Client)**
- [x] Identity provider registry — `OIDCProvider` interface (type-asserted in handlers), `StateStore` with PKCE verifier + 5min TTL, `MemoryStateStore` implementation (`internal/platform/auth/`)
- [x] Admin UI for identity providers — per-org management (`/admin/identity-providers`): list, create/edit, enable/disable, client credentials (client ID, client secret), issuer URL, discovery endpoint configuration (`internal/api/admin_identity_providers_handlers.go`)
- [x] OIDC discovery-based authentication — support for Google and generic OIDC providers via `golang.org/x/oauth2` + `coreos/go-oidc/v3`; automatic discovery of `/.well-known/openid-configuration`; PKCE (S256) mandatory
- [x] User provisioning — auto-provision on first login (create local user record from IdP claims), local user lookup fallback, email-based user resolution
- [x] Post-auth TOTP — 2FA challenge still applies after IdP login; recovery codes and hardware keys apply uniformly
- [x] Login page IdP buttons — dynamically rendered based on org config; falls back to local login when no IdPs configured for the org (`templates/pages/login.pongo2`)
- [x] Integration tests — 5 OIDC integration tests with Keycloak 26.x testcontainer (`make test-oidc-integration`); unit tests for PKCE generation, provider name/priority, state store
---

### 0.9.0 - Target: August 2026

**FAQ / Knowledge Base Plugin** *(first-party, open source)*
- [ ] Public and internal article categories with permissions
- [ ] Rich text articles with attachments and images
- [ ] Search with relevance ranking and filters
- [ ] Article ratings, feedback, and usage analytics
- [ ] Link articles to tickets for quick reference
- [ ] Customer portal FAQ integration with search
- [ ] Article approval workflow

**Calendar & Appointments Plugin** *(first-party, open source)*
- [ ] Agent calendar view (day/week/month)
- [ ] Ticket-linked appointments with reminders
- [ ] Recurring events (daily, weekly, monthly)
- [ ] Calendar sharing between agents and teams
- [ ] iCal export/subscription
- [ ] Integration with ticket escalations
- [ ] Resource scheduling (meeting rooms, equipment)

**Process Management Plugin** *(first-party, open source)*
- [ ] Visual process designer with drag-and-drop
- [ ] Multi-step ticket workflows with validation
- [ ] Conditional transitions based on ticket data
- [ ] Custom activity dialogs with dynamic forms
- [ ] Process ticket templates with pre-filled data
- [ ] SLA integration with process steps and deadlines
- [ ] Process analytics and bottleneck identification

**Theme & UX Enhancements**
- [ ] Sound event support (notifications, alerts, ticket actions)
- [ ] Custom CSS injection per theme
- [ ] Theme preview in admin

**Production Preparation**
- [ ] Prometheus metrics endpoint with custom metrics
- [ ] Structured JSON logging (configurable levels)
- [ ] Health check endpoints (liveness, readiness, startup)
- [ ] Graceful shutdown handling with connection draining
- [ ] Distributed tracing (OpenTelemetry)
- [ ] Circuit breakers for external dependencies

**Quality**
- [ ] 80% test coverage target
- [x] Load testing harness (k6) — pulled forward into 0.8.3 with `tests/load/k6/goatflow_smoke.js` and Makefile targets.
- [ ] Chaos engineering tests

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

**Status (May 2026):** 8 of 12 enterprise plugins have initial gRPC implementations with manifests, build packaging, HostAPI handlers, schema migrations, and plugin-health support. The remaining 4 plugins are still in planning/design.

| Status | Plugins |
|--------|---------|
| Initial API implementation | goatkit-media, goatkit-llm, goatkit-billing, goatkit-devices, goatkit-workflows, goatkit-audit, goatkit-content-feeds, goatkit-notify |
| Planning/design | goatkit-subscriptions, goatkit-invoicing, goatkit-payments, goatkit-maps |

**Maturity assessment (May 2026):**

| Maturity | Plugins | Evidence | Gaps |
|----------|---------|----------|------|
| Functional API scaffold | goatkit-media, goatkit-llm, goatkit-billing, goatkit-devices, goatkit-content-feeds | Manifests, build packaging, HostAPI route/handler dispatch, schema migrations, health ping; devices also declares a sidecar capability | Needs API smoke/contract coverage and public usage docs |
| Thin API scaffold | goatkit-workflows, goatkit-audit, goatkit-notify | Manifests, build packaging, schema migrations, route/handler dispatch, health ping | Needs deeper integration coverage and production workflow validation |
| Planning only | goatkit-subscriptions, goatkit-invoicing, goatkit-payments, goatkit-maps | Product scope and design notes exist | Needs implementation, schema, handlers, packaging, and tests |

---

## 📊 Version Summary

| Version | Date | Status | Theme |
|---------|------|--------|-------|
| 1.0.0 | Nov 2026 | 🔮 Future | Production Release |
| 0.9.0 | Aug 2026 | 🔮 Future | FAQ, Calendar, Process Management Plugins |
| 0.8.4 | Jul 2026 | 🔧 In Progress | External Identity Provider Integration (OIDC Client) |
| 0.8.3 | Jun 2026 | ✅ Complete | Plugin Auto-Restart, Plugin UI Offline, WebAuthn, Quality |
| 0.8.2 | Apr 2026 | 🚀 Current | **MCP v2** + Plugin Manager Resilience (health checks, bounded shutdown) |
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
