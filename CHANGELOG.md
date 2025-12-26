# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project (currently) does not yet use semantic versioning; versions will be tagged once the ticket vertical slice lands.

## [Unreleased]

### Added
- **Self-Registering Handler Architecture**: Handlers now register via `init()` calls to `routing.RegisterHandler()`, eliminating manual registration in main.go. Test validates all YAML handlers are registered.
- **SLA Admin UX Improvements**: Time fields now use unit dropdowns (Minutes/Hours/Days) instead of raw minutes input, with automatic conversion.
- YAML handler wiring test (`internal/routing/yaml_handler_wiring_test.go`) that verifies all YAML-referenced handlers are registered.
- Handler registration init files (`internal/api/*_init.go`) for self-registering handlers.
- **Customer Portal**: Full customer-facing ticket management with login, ticket creation, viewing, replies, and ticket closure.
- **Customer Portal i18n**: Full internationalization for all 12 customer portal templates with English and German translations.
- Customer portal rich text editor (Tiptap) for ticket creation and replies.
- Customer close ticket functionality with proper article/article_data_mime insertion.
- Inbound email pipeline: POP3 connector factory, postmaster processor, ticket token filters, external ticket rules example, and mail account metadata/tests.
- IMAP connector support (go-imap/v2) with IMAPTLS alias, folder metadata propagation, and factory registration.
- Admin mail account poll status API/routes backed by Valkey cache.
- SMTP4Dev integration suite covering POP/SMTP roundtrips (attachments, threading, TLS/STARTTLS/SMTPS, concurrency) with minimal smtp4dev test client.
- SMTP4Dev IMAP integration flow to verify folder retention and account metadata on fetch without delete.
- POP3 fetcher resilience + mail queue task delivery/backoff cleanup coverage for SMTP sink flows.
- Notifications render context helper to populate agent/customer names for templates.
- Unit tests for filter chain, postmaster service, mail queue repository ordering, and email queue cleanup.
- Scheduler jobs CLI (`cmd/goats/scheduler_jobs`) with metrics publishing.
- Admin customer company create POST route at `/customer/companies/new`.
- Queue meta partial for ticket list/queue UI and updated templates.
- Dynamic module handler wiring with expanded acceptance coverage.
- **Email Threading Support**: RFC-compliant Message-ID, In-Reply-To, and References headers for conversation tracking in customer notifications.
- `BuildEmailMessageWithThreading()` function in mailqueue repository for generating threaded email messages.
- `GenerateMessageID()` function for creating unique RFC-compliant message identifiers.
- Database schema support for storing email threading headers in article records.
- Integration with agent ticket routes to include threading headers in customer notifications.
- Outbound customer notifications now send threaded emails on ticket creation and public replies, persisting Message-ID/In-Reply-To/References for future responses.
- Unit coverage for mailqueue threading helpers (Message-ID generation, threading headers, extraction) to guard regressions.
- Completed ticket creation vertical slice: `/api/tickets` service handler, HTMX agent form, attachment/time accounting support, and history recorder coverage.
- Ticket zoom (`pages/ticket_detail.pongo2`) now renders live articles, history, and customer context for newly created tickets.
- Status transitions, agent assignment, and queue transfer endpoints wired for both HTMX and JSON flows with history logging.
- Agent Ticket Zoom tabs now render ticket history and linked tickets via Pongo2 HTMX fragments, providing empty-state messaging until data exists.
- MySQL test container now applies the same integration fixtures as PostgreSQL, so API suites run identically across drivers.
- Regression coverage for `/admin/users` and YAML fallback routes when `GOTRS_DISABLE_TEST_AUTH_BYPASS` is disabled.
- **Admin Services Module**: Full CRUD functionality with 31 unit tests covering page rendering, create (form+JSON), update, delete, validation, DB integration, JSON responses, HTMX responses, and content-type handling.

### Changed
- Routes manifest regenerated (including admin dynamic aliases) and config defaults refreshed.
- Ticket creation validation tightened; queue UI updated with meta component.
- Dynamic module templates and handler registration aligned with tests.
- Scheduler email poller covers IMAPTLS alias predicate and factory registration.
- E2E/Playwright and schema discovery scripts refreshed.
- Agent ticket creation path issues `HX-Redirect` to the canonical zoom view and shares queue/state validation with the API handler.
- API test harness now defaults to Postgres to align history assertions with integration coverage.
- Documentation updated for inbound email IMAP aliases, folder metadata, and integration coverage notes.

### Fixed
- SLA admin update handler now converts PostgreSQL placeholders to MySQL (`ConvertPlaceholders`).
- SLA admin create handler properly handles NOT NULL columns by converting nil to 0.
- Admin customer company create now returns validation (400) instead of 404 for POST to `/customer/companies/new`.
- Database connectivity issues in test environments with proper network configuration for test containers.
- Auth middleware, YAML fallback guards, and legacy route middleware now respect `GOTRS_DISABLE_TEST_AUTH_BYPASS`, preventing unauthenticated access to admin surfaces during regression runs.
- SQL placeholder conversion issues for MySQL compatibility in user and group repositories.
- User title field length validation to prevent varchar(50) constraint violations.
- Admin groups overview now renders the `comments` column so descriptions entered in Znuny/OTRS appear in the group list UI.
- Admin groups membership links now launch the modal and load data through `/members`, restoring the key icon and member count actions.
- Queue-centric group permissions view with HTML + JSON endpoints for `/admin/groups/:id/permissions`.

### Changed
- Handler registration architecture: YAML routes now resolve handlers from `GlobalHandlerMap` populated via `init()` functions.
- SLA admin routes added to `routes/admin.yaml` for YAML-driven routing consistency.
- User repository Create and Update methods now include title length validation and proper SQL placeholder conversion.
- Group repository queries now use database.ConvertPlaceholders for cross-database compatibility.

### Removed
- _Nothing yet._

### Breaking Changes
- _None._

### Internal / Developer Notes
- Track follow-up work for status/assignment transitions and SMTP mail-sink container integration.

---

## [0.4.0] - 2025-10-20
### Added
- Generic GoatKit Typeahead enhancement (`goatkit-typeahead.js`): Enter/Tab auto-selects first suggestion, prevents accidental form submission, advances focus.
- GoatKit Autocomplete module (`goatkit-autocomplete.js`): declarative data-attribute driven autocomplete (seed JSON + future remote source), ARIA roles (combobox/listbox/option), keyboard navigation, first-item auto-highlight.
- Visual commit feedback (flash outline) on auto-complete commit.
- Global guards to prevent duplicate script initialization.
- Data seed loader with tolerant JSON parsing (trailing comma removal) and inline `<script type="application/json" data-gk-seed>` support.
- Hidden input synchronization via `data-hidden-target` for canonical value submission.
- Blur + click-outside handling to close suggestion lists.
- Configurable min character threshold (`data-min-chars`, default 1).
- Debug gating via `window.GK_DEBUG` flag (suppressed logs by default).
- Ticket zoom page base template.
- Per-queue ticket stats table (dashboard) and admin dashboard deduplication.
- Redis (Valkey-compatible) caching layer abstraction.
- Article storage backend (DB + filesystem) integration.
- Evidence diff utility for TDD enforcement.
- Unified ticket number generator framework + counter migration.
- Pluggable auth provider registry (database, ldap, static) with tests.
- Dockerfile/dev compose improvements for caching & user customization.
- Comprehensive ticket creation & validation test suite.
- Agent ticket creation auto-selects preferred queues pulled from customer and customer-user group permissions, with info panel surfacing the resolved queue name.
- Playwright acceptance harness (`test-acceptance-playwright`) with queue preference coverage, configurable artifact directories, and resilient base URL resolution.
- Consolidated schema alignment with Znuny: added `ticket_number_counter`, surrogate primary key for `acl_sync`, `acl_ticket_attribute_relations`, `activity`, `article_color`, `permission_groups`, `translation`, `calendar_appointment_plugin`, `pm_process_preferences`, `smime_keys`, `oauth2_token_config`/`oauth2_token`, and `mention` tables via migration `000001_schema_alignment`.

### Changed
- Refactored customer user inline autocomplete logic on ticket creation form to generic GoatKit modules (removal of large inline JS block in `templates/pages/tickets/new.pongo2`).
- Display template placeholder format switched to single-brace form `{firstName}` to avoid template engine collision; template compiler now supports both `{{key}}` and `{key}`.
- Auth handlers adapted to new provider registry API.
- Ticket creation now relies on repository ticket number generator (post framework introduction).
- Dockerfile optimized for builds (layer caching / user customization notes).
- Activity stream handling cleaned (duplicate handlers removed).
- Added surrogate primary key to `acl_sync` as part of consolidated migration `000001_schema_alignment` to stay aligned with Znuny upstream schema.
- Ticket list + queue detail defaults to `not_closed`, populating status dropdowns from live state tables and excluding closed types when requested.
- Login screen auto-focuses and selects the username field on load for quicker keyboard entry.
- Coverage targets (`make test-coverage*`) now run through the toolbox inside containers, spin up DB/cache services, and delegate execution to `scripts/run_coverage.sh` for filtered package selection.

### Fixed
- Trailing comma in generated seed JSON causing parse error (replaced incorrect loop variable usage and added tolerant parser).
- Auto-commit path previously populating hidden field with display string instead of login (added `data-login` / `data-value` attributes to suggestion options).
- MutationObserver early attachment errors (guarded until `document.body` present in both typeahead and autocomplete scripts).
- Empty dropdown lingering after selection (added blur close + explicit hide on commit).
- Initial absence of suggestions due to seed load ordering (added pre-load of all seed scripts before initialization).
- Ticket number StartFrom honored via proper counter initialization.
- Premature return in activity stream handler.
- Build handler duplication causing symbol redeclaration.
- Toolbox build/test hanging issues (interactive shell hang & GOFLAGS parsing) resolved.

### Removed
- Unnecessary `console.debug` noise (now gated behind `window.GK_DEBUG`).

### Breaking Changes
- Auth initialization now requires explicit provider registration (auth provider registry).
- New DB migration `000001_schema_alignment` required before further ticket creation.

### Internal / Developer Notes
- Autocomplete registry kept in-memory (`REGISTRY`) for potential future API exposure.
- Future enhancements (not yet implemented): remote data source (`data-source`), match substring highlighting, customizable "No results" template, hot reload of seeds.

---

## [0.3.0] - 2025-09-23
### Added
- Queue detail view with real-time statistics and enhanced ticket display (`feat(queue)`).
- Agent queues handler & template (agent queue list).
- Dark mode + custom Tailwind color palette, dark form element theming.
- Actions dropdown on ticket detail page.
- Rich text editor (Tiptap) integration for ticket/article content.
- Unicode support configuration & filtering.
- Markdown rendering switched to Goldmark with enhanced styling.
- Authentication middleware enhancements (logging, permission service improvements).
- Ticket creation page (HTMX form + error handling) and supporting templates.
- PATH and migration tooling updates for dual Postgres/MySQL dev support.

### Changed
- Refactored authentication middleware & API routes for consistency.
- Updated documentation and Makefile for toolbox workflow & container-first lessons.
- Standardized YAML routing & route loader tooling (static baseline + validation script).

### Fixed
- Permissions issues in admin modules (admin permissions functionality fix).
- SQL placeholder compatibility for MariaDB (PostgreSQL-style placeholders replaced).
- Various authentication, routing, ticket functionality issues (multi-fix commit 4a897cb).

### Internal
- Copilot instructions updated with container-first lessons.
- HTMX/JS refactors for API calls and utilities consolidation.

## [0.2.0] - 2025-09-03
### Added
- DB-less fallbacks for lookups, dashboard, tickets, admin pages to keep pages rendering under test / missing DB.
- Deterministic HTMX login path for tests; DB-less ticket creation in `APP_ENV=test`.
- Toolbox targets: staticcheck, curated integration test suites, test harness utilities.
- Storage path env expansion (`STORAGE_PATH`), host network mapping for toolbox, template directory overrides.
- CLI support: auto-create minimal users table & seed (DB-agnostic reset-user), user/admin helpers.
- API routing migration to YAML system completed.

### Changed
- Extensive test hardening & gating (skip when DB unavailable, deterministic outputs).
- Simplified toolbox execution (dropping UID mapping, caching modules/build, SELinux-friendly binds).
- Static analysis integration (staticcheck suppressions + fixes; normalized error strings & context keys).
- Build/runtime Docker & compose improvements (toolchain pinning Go 1.24.6, caching).

### Fixed
- Numerous nil DB panics across handlers/services (graceful fallbacks & guards).
- MariaDB-safe tests & placeholder corrections.
- Lookup handlers defensive defaults (queues/priorities/statuses) when DB absent.
- Test flakiness (shortened DB pings, guarded migrations, removal of unstable skips).
- Integration test compilation errors & unused symbol issues.

### Internal
- Separation of archived/ignored handlers via `//go:build ignore`.
- Normalization of Make targets (whitespace/tab fixes, GOFLAGS enforcement).
- Added curated test tags (integration, debug-only).

## [0.1.0] - 2025-08-17
### Added
- Foundational authentication (JWT, RBAC), session management, secret management system.
- OTRS-compatible database schema import (116 tables) and migration tooling.
- Ticket, article, internal notes, canned responses, SLA, search (Zinc), workflow automation, ticket templates, file storage service.
- LDAP / Active Directory integration & comprehensive LDAP testing infra (OpenLDAP).
- Internationalization (babelfish) and multi-language admin modules.
- Admin modules: roles, priorities, queues, states, types, services; dynamic lookup system.
- Customer portal, agent dashboard (SSE), queue management, ticket workflow state management.
- GraphQL API (initial) and REST API v1 Phase 2/3 progression.
- Comprehensive test suites (unit, integration, pact/contract tests) and TDD ticket creation with persistence.
- Security: automated secret scanning, removal of hardcoded credentials, secure test data generation.
- Multi-stage optimized Dockerfiles and build pipeline basics.

### Changed
- Pivot to HTMX frontend architecture (from prior approach) with Temporal & Zinc references.
- Consolidated documentation (architecture, roadmap progress reports, velocity/burndown charts).

### Fixed
- Numerous early stabilization fixes: authentication compile errors, database integration for tickets/queues/priorities, test panics, route duplication, credential corrections.
- Password generation switched to base64; placeholder/token format corrections.

### Security
- Removal of all hardcoded credentials; environment variable driven secrets; clean-room schema design for interoperability.

### Internal
- Early refactors improving security posture and documentation consolidation.

[Unreleased]: https://github.com/gotrs-io/gotrs-ce/compare/0.4.0...HEAD
[0.4.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.4.0
[0.3.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.3.0
[0.2.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.2.0
[0.1.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.1.0
