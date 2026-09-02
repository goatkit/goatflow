# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/) and this
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- **Shared Tiptap editor partial for plugins (`static/js/gk-editor.js` + `templates/partials/tiptap_editor.pongo2`).**
  Plugins previously copy-pasted the two Tiptap script tags plus a manual "retry until `TiptapEditor`
  is defined" dance into every page. A single `<script src="/static/js/gk-editor.js"></script>`
  now exposes the promise-based `GoatKitEditor` API (`init`, `content`, `set`, `setMode`, `destroy`,
  `insertText`, `ready`), which lazily loads the platform's `tiptap.min.js` + `tiptap-editor.js` pair,
  re-initialises a known id by pushing new content through the underlying cache-hit instance, and
  queues early `set()` calls (draft restores, file drops) until the editor exists so they win over
  init-time content. The eager pair stays in `templates/partials/tiptap_editor.pongo2` for platform
  templates whose inline scripts call `TiptapEditor.*` synchronously; 10 platform templates
  (ticket/agent/customer detail, ticket/new, admin template/signature/email-identity/dynamic-module
  forms) now include the partial instead of repeating the script tags. First consumers: GoatCoach
  (markdown-mode transcript + article editors with preview) and goat-kb (article editor).
- **`POST /api/v1/markdown/render`.** Renders markdown to sanitized HTML via the canonical
  `pkg/markdown` stack (goldmark GFM + bluemonday) for editor preview panes. Returns the bare
  sanitized output — NOT the Tailwind-classed `RenderMarkdown` wrapper — so consumers style it
  with their own theme (e.g. a plugin's prose variables). Protected by `unified_auth` (session
  cookie or API token), body capped at 1 MiB (413), invalid JSON → 400. Generic renderer for
  plugin preview panes; GoatCoach's modal uses its own composition of this renderer + the
  transcript speaker-chip pass for display parity (platform never imports plugin-only code).
- **PDF page thumbnails via the attachment thumbnail routes.** The attachment thumbnail
  endpoints (`/api/tickets/:id/attachments/:id/thumbnail` and the customer portal equivalent)
  now serve a PNG of a PDF's first page instead of redirecting to the raw file — the same URL
  contract as image attachments, so any surface (agent ticket grid, customer list, plugins) can
  render document previews with a plain `<img src="…/thumbnail">`. Rendering uses poppler's
  `pdftoppm` (`poppler-utils` added to the runtime image) via `internal/pdfthumb.RenderPage1`
  (page 1 at ≤400 px wide, aspect-preserved); agent-side thumbnails share the existing
  `storage/thumbs/<ticketID>/<attID>.png` cache. When `pdftoppm` is unavailable or a PDF can't
  be rasterized, the endpoint keeps its previous redirect-to-raw fallback (customer portal:
  400). First consumer: GoatCoach E5 deliverable exports.
- **HostAPI `RenderMarkdownToPdf`.** Plugins render a markdown document to PDF bytes
  (`pkg/plugin/plugin.go`). The platform converts markdown to styled, sanitised HTML (goldmark +
  bluemonday) and prints it via a Browserless headless-Chromium sidecar (`internal/platform/plugin/
  pdf_renderer.go`, default `BROWSERLESS_URL`/`BROWSERLESS_TOKEN`, added to the dev compose).
  Page size, margin and title are controllable via `PdfRenderOptions`. Generic: any plugin that
  exports a formatted deliverable (first consumer: GoatCoach E5 PDF export).
- **PDF branding in `RenderMarkdownToPdf`.** `PdfRenderOptions` gains optional `BrandName`,
  `BrandColor` (`#RRGGBB` accent on headings/links + 12%-alpha table-header wash) and `BrandLogoURL`
  (https-only header logo). The running header becomes `logo + name — title` when any branding is
  set; zero values print exactly as before. Values are strictly validated (regexp-checked colour,
  https + attribute-safe logo URL, HTML-escaped name) so a plugin can never inject arbitrary CSS or
  markup into the printed page (`internal/platform/plugin/pdf_renderer.go`). First consumer:
  GoatCoach E5 branded deliverable export (branding sourced from the coach/client style profile).
- **HostAPI `CreateArticle`.** Plugins create a transcript/deliverable article on a ticket through
  the platform (`pkg/plugin/plugin.go`), replacing hand-written article rows via raw SQL. The
  `ProdHostAPI` implementation (`internal/platform/plugin/article_create.go`) enforces the OTRS
  invariants in one transaction: ticket existence, `article` row (agent sender, internal channel,
  `is_visible_for_customer`), `article_data_mime` row with utf8mb3-safe subject/body, and
  driver-agnostic id retrieval (DB adapter `InsertWithReturningTx`/`ExecTx`). Generic: any
  document/transcript-producing plugin (first consumer: GoatCoach session transcripts + deliverables).
- **Importable `pkg/markdown` renderer.** `github.com/goatkit/goatflow/pkg/markdown` exposes
  `Render(s string) string` (goldmark GFM + bluemonday sanitization: GFM tables/strikethrough/
  autolinks, `class` attrs preserved so an outer layer can add Tailwind classes, safe to insert
  into plugin UI / server-rendered pages). The core ticket-note path (`api.RenderMarkdown`) now
  delegates to it — one canonical renderer instead of per-consumer goldmark copies; a side effect
  is that raw/unsafe HTML in ticket notes/messages is now stripped (previously that path emitted
  raw HTML un-sanitized). First plugin consumer: GoatCoach replaced its `internal/md` goldmark
  shim with this package (shim deleted).
- **Standard plugin shell renders `ui_nav_items`.** `layouts/ui_standard.pongo2` (the default
  shell for plugin UIs) now renders a plugin's page nav (side or top position) with label, icon,
  active state and `badge_count` — previously only the minimal shell did, so a standard-shell
  plugin (e.g. GoatCoach) got no in-plugin navigation or badge counts and had to fall back to a
  count card. Badge counts come from the same `buildNavItems` resolution as the minimal shell
  (plugin badge fn returning `{"count": N}`). First consumer: GoatCoach side nav (Dashboard/
  Clients/Prompt Specs/Capture) with the Dashboard "awaiting review" badge. Also fixes a latent
  bug: the shell template context passed `ui_nav` as the `*UINavConfig` struct, but pongo2
  resolves Go field names not JSON tags, so `ui_nav.position` never matched and no shell nav
  rendered — `ui_nav` is now a map (`{position, items}`), which also un-breaks the minimal shell.
  Whole-number badge counts (JSON `8.0`) are coerced to ints (`buildNavItems.wholeNumber`) so the
  badge shows "8" not "8.000000".
- **Plugin UI routes now carry the authenticated identity.** `buildUIHandler`'s `_user_id` /
  `_is_admin` / `_user_email` / `_user_login` / `_user_role` / `_org_id` args were previously a
  no-op because UI routes got no session middleware — plugins could only guess who was calling (a
  plugin like GoatCoach fell back to the first valid agent, and admin checks required a DB group
  query). `RegisterUIRoutes` now takes a session-auth middleware (supplied by the API layer as
  `SessionOrJWTAuth`, avoiding an import cycle) applied to session-authenticated UI groups, so the
  identity reaches the plugin on UI page and mutation routes. UI routes are now authenticated (they
  previously weren't). First consumer: GoatCoach per-coach attribution + admin gating on its UI.
- **HostAPI article attachments.** `CreateArticleAttachment` / `ListArticleAttachments` /
  `DeleteArticleAttachment` on the plugin HostAPI (`pkg/plugin/plugin.go`) let any document-producing
  plugin attach files to articles (first consumer: GoatCoach E2/E4/E5 deliverable attachments).
  Backed by a `ProdHostAPI` implementation (`internal/platform/plugin/article_attachment.go`) that
  writes the real OTRS-legacy `article_data_mime_attachment` table (article-existence + 10 MiB
  size-limit from storage config, `ConvertPlaceholders` for both mysql/postgres, `created_by` FK),
  plus grpc dispatch, plugin-client methods, and an `ArticleAttachment` struct.
- **First-boot admin bootstrap honours `GOATFLOW_ADMIN_PASSWORD`.** A fresh
  install previously left `root@localhost` in its factory-disabled state with a
  random password, and the setup wizard that could set a password was itself
  behind an admin login — so a clean install (TrueNAS, Docker, bare metal)
  locked operators out. On first boot the backend now checks the
  factory-disabled marker plus a `admin.bootstrap.applied` flag in
  `sysconfig_modified`, and if still pristine it applies the wizard-supplied
  password (bcrypt, matching both login paths) and enables the account.
  One-shot and race-safe: the `UPDATE … WHERE valid_id <> 1` guard means any
  later password change (via the UI or `goatflow reset-user`) makes the
  wizard value a permanent no-op, and a disabled-again admin is treated as
  intentional. Failures are logged and never block startup
  (`cmd/goats/admin_bootstrap.go`). Verified end-to-end in a live Docker
  stack: fresh MariaDB → migrations → bootstrap → successful login with the
  wizard password, and a second boot is a no-op. Note: requires the next
  image build; the pinned `0.9.0` image predates it.
- **Real health probes.** `GET /health` now performs a 500 ms-timeout
  database ping and returns 503 when the DB is unreachable, instead of the
  previous static 200 that ignored the database. The Dockerfile `HEALTHCHECK`,
  the TrueNAS app template and Kubernetes probes all point at this endpoint,
  so a backend with a dead DB now reports unhealthy. `GET /health/detailed`
  adds the Valkey cache check (`ok`/`error`/`disabled`), build version and
  process uptime (`internal/api/health.go`).
- **Prometheus metrics on `/metrics` and optional standalone listener.** The
  `/metrics` route now serves the real Prometheus exposition format from the
  default registerer (previously a hardcoded three-line stub), which brings
  the Valkey cache metrics already recorded via `promauto`
  (`cache_hits_total`, `cache_misses_total`, `cache_errors_total`,
  `cache_set_total`, `cache_delete_total`, `cache_latency_seconds`,
  `cache_size`) into the scrape for the first time. New process gauges:
  `goatflow_up` and `goatflow_process_start_time_seconds`
  (`internal/api/metrics.go`). When `METRICS_ENABLED=true` the server also
  exposes a dedicated listener on `METRICS_PORT` (default 9090) — the
  variable that was documented in `.env.example` but never wired — drained on
  shutdown alongside the main server.
- **Structured logging controlled by the environment.** New
  `internal/platform/logging` package configures both `slog` and the legacy
  stdlib `log` package from `LOG_FORMAT` (`json`|`text`, default text),
  `LOG_LEVEL` (`debug`|`info`|`warn`|`error`, applied to `slog` lines) and
  `LOG_OUTPUT` (`stdout` or a file path; directories are created, `stdout`
  fallback on failure). `LOG_FILE_PATH` is honoured as a legacy alias for the
  destination so existing deployments keep their log file. In JSON mode,
  stdlib `log` lines are emitted as JSON records (`time`/`level`/`msg`) so a
  log stream parses uniformly for log aggregators.
- **Graceful shutdown with connection draining.** The server now runs under
  `http.Server` with SIGTERM/SIGINT handling: stop accepting new connections,
  drain in-flight requests up to `DRAIN_TIMEOUT` (default 10 s, overridable),
  then run the existing bounded plugin shutdown. Port-bind conflicts still
  fail fast exactly as before, and the runner path is untouched
  (`cmd/goats/main.go`).

### Fixed
- **Last remaining Dependabot vulnerability: `postcss-selector-parser`
  pinned to 6.1.4.** The transitive instances (via `tailwindcss ^6.1.2` and
  `postcss-nested ^6.1.1`) resolved to 6.1.2, inside the vulnerable
  `< 6.1.3` range flagged by Dependabot. A `package.json` override now forces
  6.1.4 everywhere; all other open alerts were already covered by the
  dependency bumps below. Together these clear all 11 open alerts
  (7 high) on `dev` — they close on `main` once the merge lands.
- **Go dependency security bumps.** `grpc v1.82.1 → v1.83.2`,
  `moby/go-archive v0.2.0 → v0.3.3`, `golang.org/x/net v0.57.0 → v0.58.0`
  (the high-severity Dependabot findings on the Go side). Build + vet clean.
- **JavaScript dependency security bumps.** `postcss → 8.5.26`,
  `js-yaml → 4.3.2`, `@tiptap/* → 3.31.0` (17 packages), `nanoid → 3.3.18`
  (lockfile), resolving the remaining high/medium Dependabot findings on the
  JS side.
- **Tooling migrated from `bun.lockb` to text `bun.lock` (bun 1.3).** The
  app `Dockerfile` (`COPY` line), three `Makefile` references and the
  `.gitleaks.toml` lockfile-detection regex now match the text lockfile bun
  1.3 writes; the regex still detects the legacy binary name.
- **Toolbox build image no longer silently breaks.** `Dockerfile.toolbox`
  pinned `BUN_VERSION=1.1.42`, which cannot read the text lockfile (its
  failure was swallowed by `|| true`, masking the unit-test stage), and
  `STATICCHECK_VERSION=latest`, which resolved to staticcheck 0.8+ requiring
  Go 1.26. Pinned to bun 1.3.14 and staticcheck v0.7.0 (last release building
  under Go 1.25.12).

### Changed
- **API license metadata aligned to Apache-2.0.** The OpenAPI specs
  (`api/openapi.yaml`, `api/openapi.bundle.yaml`), the generated Swagger docs
  (`docs/api/swagger.{json,yaml}`, `docs/api/docs.go`) and the swagger
  annotation in `cmd/goats/main.go` all declared `AGPL-3.0` — stale metadata
  flagged by the packaging audit. They now match the actual project license
  (Apache-2.0 in `LICENSE`, `README.md`, `docs/LICENSING.md`, `package.json`).
  No functional change; license-only metadata.
- **TrueNAS SCALE added to the supported platforms list.** `README.md`
  (Container Runtime Support + Production Deployment) and `docs/FEATURES.md`
  (Deployment Options) now mention TrueNAS SCALE 24.10+ — the official app
  catalog package is ready in `docs/truenas-app/` (PR in submission); until it
  merges, "Install via YAML" in the Apps Market works with the standard Compose stack.
- **Helm chart version metadata fixed.** `charts/goatflow/Chart.yaml` declared
  `appVersion: "1.0.0"` (no such app release exists — the real version is 0.9.0)
  and had no `kubeVersion` gate; it now declares `appVersion: "0.9.0"` and
  `kubeVersion: ">=1.25.0"`, matching the declared support floor in
  `charts/goatflow/README.md` and the system requirements. The install-from-release
  example in the chart README now points at `v0.9.0`. Chart `version` stays
  `0.1.0` (bump when the first chart release ships). Verified with `helm lint`
  (0 failures) and `helm template` render.
- **Helm chart now references the real container image.** `charts/goatflow/values.yaml`
  defaulted `backend.image.repository` to `goatflow/backend` — an image that never existed
  (Docker Hub 404 / ghcr 403) and a leftover from the pre-rename era — so any out-of-the-box
  `helm install` of the published charts ImagePullBackOff'd. The default is now
  `ghcr.io/goatkit/goatflow`, matching the app's published image. Also, the CI chart-publish
  step (`.github/workflows/build.yml`) wrote `appVersion` with a `v` prefix (`v0.9.0`) while the
  container image tags have no `v` (`ghcr.io/goatkit/goatflow:0.9.0`), so the rendered image tag
  never matched a real tag; it now strips the `v`. Verified: `helm template` renders
  `image: ghcr.io/goatkit/goatflow:0.9.0`, which is pullable from ghcr.

## [0.9.0] - 2026-08-06

### Added
- **Setup Assistant: Response templates, business hours, and email transport onboarding.** Three new
  skippable wizard steps (6: canned responses, 7: business hours calendar, 8: outbound SMTP config)
  with i18n support across all 15 languages. New tasks registered in `GetCoreTasks()`:
  `create_response_template`, `configure_business_hours`, `configure_email_transport`
  (`internal/service/setup_assistant_service.go`). Admin handlers accept JSON payloads via
  `ShouldBindJSON` (`internal/api/admin_setup_handlers.go`). Translation keys added to all
  `internal/platform/i18n/translations/*.json` files under `setup_assistant_tasks` section.
- **Existing customer review mode.** Company name field on `create_customer` task is now a type-ahead
  combo box. Selecting an existing customer redirects to the wizard in review mode
  (`/admin/setup/wizard?existing_customer_id=ID`), loading the full customer configuration for
  review/editing via `LoadExistingCustomer()`. New API endpoint
  `GET /api/v1/admin/setup/customers/search/:query` for dropdown auto-completion.
- **SAML2 identity provider support.** Enterprise SSO via SAML2 using the `crewjam/saml` library,
  with separate flow from existing OIDC/OAuth2. Admin UI for managing identity provider
  configurations including signing certificates and private keys.
- **`HostAPI.GenerateThumbnail` platform interface method.** Plugins can request server-side
  thumbnail generation from image bytes (`pkg/plugin/plugin.go`), backed by libvips via govips.
- **Customer-facing Knowledge Base pages.** List, search, and article view pages wired into the
  customer portal, backed by KB plugin routes.
- **Plugin raw body passthrough and error status codes.** `buildPluginArgs` now forwards non-JSON
  request bodies (XML, CSV, plain text) to plugins as `_body` / `_content_type`. Plugin JSON
  responses of the form `{"error": "<msg>", "status": <N>}` are now mapped to the matching HTTP
  status code instead of always serving 200-OK.
- **Customer onboarding wizard.** "Onboard a customer" task now provisions an entire customer setup in one
  shot, all OTRS-compatible: company record (auto-suggested, editable Customer ID + ISO country dropdown),
  managing agent teams (`group_customer`), portal users with generated temporary passwords, Service/SLA
  mapping (`service` → `service_sla` → `service_customer_user`), and an inbound mailbox (`mail_account`)
  wired to a queue that can be picked or created on the fly (owned by a chosen team). 5-step UI
  (company → users → teams & SLA → email → review) and a programmatic API at
  `POST /api/v1/admin/setup/onboard-customer` (`internal/service/setup_assistant_service.go::OnboardCustomer`).
- **Setup Assistant (first-run wizard + re-runnable task catalog).** Auto-redirect on fresh installs
  guides admins through org type → teams → queues → agents → customers → SLAs → create; re-runnable
  assistant page shows entity snapshot and mini-wizards for common operations. Plugin-extensible via
  `GKRegistration.SetupTasks`. JSON API at `/api/v1/admin/setup/*` enables programmatic/LLM-driven
  configuration (`internal/service/setup_assistant_service.go`, HTML/API handlers in `internal/api`).

- **Admin Users: reusable multi-select group control with search + chips.** The overwhelming group
  checkbox list in the add/edit-user modal is replaced by the platform `data-gk-autocomplete` control
  in a new opt-in `data-multiple` mode: type-ahead search, selected groups rendered as removable tag
  chips, and a comma-separated hidden value. The enhancement lives in the shared
  `static/js/goatkit-autocomplete.js` (new `setMultiple`/`clearMultiple` API), so any admin form can
  reuse it with plain template attributes (`templates/pages/admin/users.pongo2`).
- **Password policy + confirmation in the admin add/edit-user modal.** Setting a new password in the
  add/edit-user modal now shows the live password-policy checklist (length / match / character-class)
  and requires a confirmation field before submit — previously only the separate reset-password modal
  had this. The validation engine is shared: `validatePassword()`, `updateValidationIcon()` and the
  policy rendering now act on a small `pwBind` binding, so both modals reuse one implementation
  (`templates/pages/admin/users.pongo2`).
- **Static-asset cache-busting.** Every `script`/`link` include now carries `?v={{ assetVersion }}`,
  where `assetVersion` is the build date, falling back to a per-process nonce so dev stacks running
  prebuilt images still bust the browser cache on every restart
  (`internal/platform/shared/template_renderer.go`, `templates/layouts/base.pongo2`).
- **RBAC-scoped core dashboard widgets.** The recent-tickets and queue-status dashboard widgets
  (and the stats plugin's by-status / chart / SLA / time-tracking widgets) now scope results to the
  acting user's groups via `args` instead of returning all rows, and the queue system-address deep
  link is admin-only (`internal/platform/plugin/core/dashboard.go`, `plugins/stats/main.go`,
  `templates/components/queue_meta.pongo2`).
- **Plugin UI page handlers now receive caller identity.** `pluginui.buildUIHandler` forwards
  `_user_id` / `_user_login` / `_user_email` / `_is_admin` / `_user_role` / `_org_id` from the request
  context so plugins can resolve which user is acting on a UI page render
  (`internal/platform/pluginui/router.go`).

### Changed
- **Platform/product decoupling (Phases 1–8).** Moved platform code out of product packages into
  `internal/platform/` across 8 phases: plugin runtime, database/services, models split, routing,
  API/service layers, constants/logging/middleware, boundary linter enforcement, and documentation.
  The plugin runtime no longer transitively depends on ~16,000 lines of product code. Full plan:
  `docs/PLATFORM_PRODUCT_DECOUPLING.md`.
- **Docker frontend builder upgraded to bun v1.3.** The previous bun v1.1 image couldn't parse the
  v0.03 lockfile format written by newer bun versions.

### Fixed
- **`RenderMarkdownToPdf` emitted markdown tables as literal pipe text.** The renderer used a bare
  goldmark parser, so GFM tables (e.g. deliverables' action-item tables) passed through unconverted
  and printed as `| A | B | :--- …`. It now uses the same goldmark GFM stack as `pkg/markdown`
  (tables, strikethrough, autolinks; raw HTML still sanitized by bluemonday before printing).

- **gRPC plugins failed to load in CI containers** ("Failed to read any lines from plugin's
  stdout"). `/tmp` is `0755 root:root` in alpine (not world-writable), so go-plugin could not
  create its unix socket. Fixed by pointing plugin `TMPDIR`/`HOME` at `/app/tmp` via
  `buildPluginEnv` and making `/app/tmp` world-writable with sticky bit (`chmod 1777`).
- **CI cache permission errors for non-root containers.** Workspace-local cache paths
  (`/workspace/.go-build`, `/workspace/.gomodcache`) replace volume-mounted `/cache/*` dirs, with
  Docker-based `chmod` as root to fix foreign-UID file permissions before test containers start.
- **`SessionOrJWTAuth` now handles `gf_` API tokens.** Previously API tokens with the `gf_` prefix
  were not recognised by the session/JWT auth middleware.
- **SAML identity provider creation now requires user-supplied signing certificate and private key**
  instead of auto-generating them. Also fixes YAML handler wiring, OIDC empty param check, and
  attachment temp path creation.
- **Test factory wiring and nil checks** for decoupled platform packages.
- **Bun lockfiles restored** and storage path / test isolation fixed for non-root containers.
- **Admin Users group search in the edit modal returned no suggestions.** `data-display-template`
  used pongo2's `{{name}}` (double braces), which the template engine consumed — the display template
  rendered empty. Switched to the single-brace `{name}` convention used by the other
  `data-gk-autocomplete` controls.
- **Duplicate groups in user/group/queue listings.** Group-membership queries in
  `admin_users_handlers.go`, `admin_users_handlers_improved.go`, `admin_crud_handlers.go`, and
  `agent_routes.go` joined `group_user` without `DISTINCT`, so a user holding several permission rows
  for one group (e.g. `grp_atlasconstruction` ×7) surfaced the same group/queue repeatedly. All four
  queries now `SELECT DISTINCT`. No data was remediated — `group_user` `(user, group, permission)`
  rows were already unique.
- **Admin user-create group INSERT passed 2 args for 4 placeholders.** `HandleAdminUserCreate`'s
  group loop INSERT used `userID, groupID` twice but only passed each once, so the `NOT EXISTS`
  subquery had unbound args and new groups were not granted. Now passes all four
  (`userID, groupID, userID, groupID`).
- **Removing a group chip popped the dropdown open.** The chip remove handler refocused the input,
  which — for the `data-min-chars="0"` group control — reopened the suggestion list. The refocus is
  now marked so the list stays closed after removing a chip (`static/js/goatkit-autocomplete.js`).
- **Logged-out pages served a stale cached favicon/logo.** The auth/login layouts referenced the
  favicon and logo without the cache-bust token, so logged-out surfaces kept showing the previous
  artwork while the logged-in UI (versioned) showed the current one. All favicon/logo references now
  carry `?v={{ assetVersion }}` (`templates/layouts/auth.pongo2`, `login.pongo2`,
  `login_2fa.pongo2`, `customer/login.pongo2`, `customer/login_2fa.pongo2`).
- **2FA login loop on Firefox (agent + customer).** The OTP code's 6-digit auto-submit dispatched a
  synthetic `new Event('submit')` on the form. That event is not `cancelable`, so in Firefox the
  browser performs a native form submission (reloading the 2FA page) in addition to the fetch-based
  verify, leaving the user stuck entering their code; Chrome ignores the synthetic event. The form's
  fetch+redirect logic was extracted into a shared `verifyCode()` called directly by both the submit
  handler and the auto-submit, removing the synthetic `submit` event entirely
  (`templates/pages/login_2fa.pongo2`, `templates/pages/customer/login_2fa.pongo2`).

### Security
- **22 Dependabot vulnerabilities remediated** (7 critical, 5 high, 10 moderate). Go:
  `golang.org/x/crypto` v0.51.0 → v0.54.0, `golang.org/x/net` v0.53.0 → v0.57.0,
  `golang.org/x/image` v0.39.0 → v0.44.0, `github.com/quic-go/quic-go` v0.59.0 → v0.60.0,
  `github.com/Azure/go-ntlmssp` → v0.1.1. npm: `js-yaml`, `linkify-it`, `markdown-it` updated via
  overrides. Frontend: `postcss` override bumped to ^8.5.18.
- **Go toolchain bumped to 1.25.12 and reachable advisories cleared.** `govulncheck` reported 9
  reachable vulnerabilities; all fixed: `google.golang.org/grpc` v1.79.3 → v1.82.1,
  `github.com/yuin/goldmark` v1.7.4 → v1.7.17, `github.com/cloudflare/circl` v1.3.3 → v1.6.3,
  `github.com/russellhaering/goxmldsig` v1.4.0 → v1.6.0 (SAML chain), and the Go image / build refs
  bumped from `1.25.10` → `1.25.12` to ship the stdlib fixes (`crypto/tls`, `crypto/x509`, `mime`,
  `net/textproto`) across all Dockerfiles, the Makefile, CI, `.env` files, and helper scripts.
  `govulncheck` is now clean (0 reachable).

## [0.8.3] - 2026-05-13

### Added
- **Passkey login for agents and customers.** Registered WebAuthn credentials are now created as resident, user-verified credentials suitable for passwordless passkey login. The agent and customer login pages expose passkey sign-in buttons backed by short-lived server-side WebAuthn ceremonies, HttpOnly pending cookies, active-account checks, and existing session-cookie issuance. Ceremony state is persisted in the new `gk_webauthn_ceremony` table and consumed once on finish, so begin/finish requests can land on different app instances. Discoverable credentials are resolved by their stored account type, so the selected passkey opens the correct agent or customer session regardless of which login surface started the ceremony, and passkey begin/finish endpoints are allowed through the global auth gate before session checks run.
- **Hardware-key MFA with WebAuthn/FIDO2.** Agents and customers can register browser-backed security keys from their profile 2FA area, then use those keys on the existing pending-2FA login screen as an alternative second factor to TOTP/recovery codes. Credentials live in the new `gk_webauthn_credential` table with public-key material, counters, friendly names, and last-used metadata; admin 2FA override now clears both TOTP state and registered security keys. WebAuthn registration and MFA verification share the persisted ceremony store used by passkeys, and profile security-key setup now uses a password modal with a visibility toggle instead of a visible browser prompt. Security-key-only accounts get a security-key-first challenge instead of a misleading authenticator-code form, while profile security status distinguishes authenticator apps from security keys, hides recovery-code counts when TOTP is not enabled, and avoids offering the TOTP disable flow for security-key-only accounts. Runtime relying-party config can be set with `GOATFLOW_WEBAUTHN_RP_ID`, `GOATFLOW_WEBAUTHN_RP_NAME`, and `GOATFLOW_WEBAUTHN_ORIGINS`, with request-derived localhost-friendly defaults for development.
- **Performance benchmark and load-test harness.** `make bench` now runs a curated Go benchmark suite across routing, middleware, API setup, config, model, sanitizer, and LDAP helper hot paths, writing benchmem captures under `generated/benchmarks/`; `make bench-compare` compares two captures with benchstat. `make load-test` runs the new k6 smoke profile in `tests/load/k6/goatflow_smoke.js` against the test stack or a supplied base URL, with configurable VUs, duration, endpoints, thresholds, and JSON summaries under `generated/load-tests/`. Usage lives in `docs/performance/BENCHMARKS.md`.
- **Plugin auto-recovery (0.8.3).** The health checker now drives auto-restart for plugins it observes wedge: once a plugin trips the failure threshold (`__health_ping__` deadline-exceeded, three consecutive probes), the manager asks the loader (wired in as `Restarter` via `Manager.SetRestarter`) to `Reload` it. Backoff is exponential (5s → 10s → 20s → … capped at 5min) and resets to zero once a probe confirms recovery. A crash-loop guard counts restart attempts in a 10-minute rolling window — more than five within that window flips `PluginHealth.CrashLoopAbandoned` true and stops further auto-restarts until an admin clears the flag. Toggle the whole behaviour with `GOATFLOW_PLUGIN_AUTO_RESTART=false` (health checker still observes; just no restarts). Replaces the placeholder note on the 0.8.2 health-checker entry below that promised restart logic for this release.
- **Plugin health rich payloads.** Plugins can opt into surfacing custom health data by returning a JSON object from their `__health_ping__` `Call` handler — the manager validates and stores it on `PluginHealth.Payload`, exposed alongside the boolean `Healthy` flag via `Manager.HealthStatus` / `AllHealthStatuses`. The "any response = alive" contract is preserved: plugins that don't implement the handler still register as healthy as before, payload is just empty. Non-JSON bodies are silently dropped so a chatty plugin can't break the dashboard.
- **Bundled plugin examples now use the new plugin-resilience features.** `hello-wasm`, `stats`, `test-hostapi`, and the `hello-grpc` example all implement `__health_ping__` with compact JSON payloads so the admin plugin UI can show useful runtime/version detail instead of an empty healthy state. The examples also declare tighter resource requests: lightweight memory/call-timeout hints for demo plugins, scoped HostAPI permissions for the stats and host-api smoke-test plugins, and gRPC init/shutdown timeout hints in both `GKRegister()` and `plugin.yaml`.
- **Configurable service-worker offline support for core and plugin UIs.** `/sw-config.json` now exposes a versioned cache config built from global `ServiceWorker::*` sysconfig values and enabled plugin UI `pwa.cache_routes`. The root `/sw.js` consumes that config, supports `network-first`, `cache-first`, `stale-while-revalidate`, and `network-only` route strategies, pre-caches same-origin configured routes, bypasses SSE/EventSource streams, and keeps push notification handling intact. Plugin UI shells register the root service worker when PWA support is enabled, so direct visits to standalone plugin UIs can install offline support too.
- **Admin management for plugin UIs.** `/admin/plugin-uis` now lists registered plugin UI surfaces with filters, deep links, PWA/manifest visibility, enable/disable controls, custom domain editing, and branding overrides. The backing admin API (`/api/v1/plugin-uis`) updates `gk_plugin_ui`, preserves plugin-owned UI config while merging admin branding changes, and rebuilds the dynamic engine after status/settings updates so routes reflect admin changes immediately.
- **Plugin-health admin widget on `/admin/plugins`.** New Healthy / Unhealthy summary cards next to the existing Total / Enabled / Disabled tiles, plus a Health column per plugin showing one of: pending (no probe yet), healthy, unhealthy with restart attempt count, or crash-loop with an inline "Reset" button. Reset hits the new admin endpoint `POST /api/v1/plugins/:name/reset-crashloop` (calls `Manager.ResetCrashLoop`, which clears the abandoned flag and resets restart bookkeeping so auto-recovery can resume after the operator has fixed the underlying problem). The page polls `/api/v1/plugins/health` alongside `/api/v1/plugins` on the existing 10s refresh.
- **Parallel plugin shutdown.** `Manager.ShutdownAll` now shuts plugins down concurrently — total time is the max per-plugin `ResourcePolicy.ShutdownTimeout`, not the sum. Previously a process exit with N plugins each on a 10s budget could spend 10×N seconds in `ShutdownAll`; now it's bounded at 10s regardless of N. Per-plugin timeouts and the outer-context cap from `cmd/goats` (30s overall ceiling) still apply.
- **Customer-company portal/capture UI strings are now translated in all 15 supported languages.** Added `customer_company_form.save_portal_settings`, `customer_company_form.plugin_capture`, and `customer_company_form.save_capture_setting` across the translation set (`ar`, `de`, `en`, `es`, `fa`, `fr`, `he`, `ja`, `pl`, `pt`, `ru`, `tlh`, `uk`, `ur`, `zh`) so `make check-i18n` no longer flags the form template.
- **MFA and WebAuthn login/profile UI strings are now translated in all 15 supported languages.** Security-key-only prompts, passkey/WebAuthn error fallbacks, MFA profile status labels, security-key registration copy, and password-visibility labels now use i18n keys with English fallbacks instead of raw template/JavaScript strings.
- **Plugin cascade dispatch** — `manifest.Cascades` entries are now wired into `deletion.Service` at plugin load time, closing the previously inert path where manifests could declare cascades but no handlers were registered. When an entity is soft or hard deleted via `HostAPI.EntitySoftDelete` / `EntityHardDelete` (or via the admin recycle-bin API), every plugin that declared an `OnSoftDelete` / `OnHardDelete` handler for that entity type has its handler invoked, receiving `{"id": entityID}` as the JSON arg. Cascades are dispatched via `Manager.Call`, so they inherit the plugin's normal sandboxing and error handling. Registrations are per-plugin keyed (not per-instance), so hot-reload via `ReplacePlugin` re-registers the new manifest's handlers cleanly and `Unregister` clears its entries. The plugin-side handlers have also been audited to scope deletes by the deleted entity's id.
- **Cascade pre-dispatch hook ensures lazy-loaded plugins still get to clean up.** `deletion.Service.runCascades` now calls a plugin-manager-registered hook before iterating the cascade registry. The hook walks every discovered-but-unloaded plugin and `EnsureLoaded`s it, so a plugin that was uploaded mid-session (or would otherwise only load on first API call) still has its cascade closures in the registry by the time dispatch begins. A single log line (`pre-cascade loaded plugins count=N entity_type=…`) fires when the hook actually loaded anything.
- **Lazy mode now eager-loads every discovered plugin at boot**, not just gRPC plugins. Previously WASM plugins stayed deferred, so a WASM plugin declaring `Cascades` wouldn't have its handler registered until its first API call — and the first entity delete after boot would pay the load cost inside the delete request. gRPC plugins were already eager-loaded for route registration; extending the same behaviour to WASM costs little extra boot time and fully removes the first-delete latency penalty for cascade handlers.
- **`customer-fe` and `runner` services now mount the `goatflow_plugins` volume** in the stock `docker-compose.yml`. Previously only `backend` had the mount, so compose-deployed installs that ran `docker compose up --force-recreate` on `customer-fe` or `runner` would see uploaded gRPC plugins vanish. Both services now also depend on `plugin-init`, mirroring the backend wiring.
- **`GOATFLOW_PLUGIN_LOG_ECHO` env var mirrors plugin `host.Log()` calls to the host's stderr**, making plugin diagnostics visible in `docker logs`/`journalctl`. Default off; set to `true`/`1`/`yes`/`on` to enable. Complements the in-memory ring buffer (served at `/api/v1/plugins/logs`) — the buffer is great for the admin UI but fills fast during heavy generation, and low-level media-decision / cascade-dispatch traces rotate out before you can query them. Mirror path bypasses the slog level filter so info/debug lines get through regardless of `LOG_LEVEL`. Evaluated once per process via `sync.Once` — flipping the env mid-run needs a container restart.

### Fixed
- **TOTP login sessions now survive split frontend/backend handling.** Password login now stores pending agent/customer 2FA sessions in the new `gk_totp_pending_session` table using a hash of the pending cookie token, while retaining the in-memory map as a fast local cache. This fixes valid authenticator-app codes being rejected after restarts or when password login and TOTP verification land on different app containers.
- **Captive customer organisations can still reach account self-service.** `/customer/profile`, `/customer/profile/update`, and `/customer/password/*` now bypass the captive-plugin landing redirect so customers can manage profile details, passwords, 2FA, and passkeys even when their organisation is pinned to a plugin landing page.
- **Test stack startup now recovers from stale Docker network references without disrupting the dev stack.** `test-stack-teardown` removes only stopped/dead test infrastructure containers before `test-up` recreates them, while leaving running test DB/cache containers and shared dev services alone.
- **MCP Streamable HTTP startup now handles JSON-RPC notifications correctly.** `Server.HandleMessage` now detects requests with no JSON-RPC `id` before normal dispatch and suppresses all responses for notifications, including `notifications/initialized`, `initialized`, cancellation, and unknown notification methods. This fixes clients such as Codex reporting MCP startup/tool discovery failures because the server returned JSON-RPC errors for notification frames that must be fire-and-forget. The Streamable HTTP transport now returns `202 Accepted` for notification POSTs with no response body, and the SSE GET path explicitly writes `200 OK` before emitting the initial `open` event, matching MCP client expectations more closely.
- **`GET /api/v1/users/me` no longer 500s when auth stores `user_id` as a `uint`.** The handler now resolves the current user via the shared `GetUserIDFromCtx` helper instead of assuming Gin's raw context value is an `int`, and its user query no longer selects a non-existent `users.email` column. The response keeps the existing `email` field for client compatibility by mirroring `login`.
- **Customer-company portal settings remain backwards compatible with direct form posts.** The per-company portal-settings handler now treats directly posted fields (`enabled`, `login_required`, `title`, `footer_text`, `landing_page`) as overrides when no `override_*` controls are present, preserving older tests/callers while the richer override UI remains supported.
- **Plugin argument construction now sees the organisation ID from request context.** `_org_id` / `org_id` injection checks `organisation.OrgIDFromContext(c.Request.Context())` before falling back to Gin keys and cookies, so middleware paths that only populate the standard request context behave correctly.
- **Playwright Go test containers can create their browser cache as non-root host users.** `Dockerfile.playwright-go` now leaves `/opt/playwright-cache` writable after browser installation, fixing lock/cache directory creation failures when the Makefile runs E2E tests under the caller's UID/GID.
- **Self-service API token creation at `/profile` API Tokens was broken four ways.** (1) The Create / Token Created / Revoke modals were rendered inline at the bottom of the document instead of overlaying the viewport — their outer wrapper carried `class="gk-modal hidden"` but `.gk-modal` is styled `position: relative` (it's the content-box class, not the overlay class), and the intermediate `.gk-modal-container` / `.gk-modal-content` classes had no CSS at all. Rewrote each outer wrapper with `fixed inset-0 z-50 overflow-y-auto flex items-center justify-center p-4`, promoted the backdrop to `fixed inset-0`, applied `.gk-modal` to the actual content box, and switched the close-handler selector from `.gk-modal` to `[role="dialog"]` so it still targets the outer wrapper after the class swap. (2) The scope checkbox list read `scope.name` but `GET /api/v1/tokens/scopes` returns items with field `scope` (`internal/api/api_token_handlers.go:247`), so every checkbox had `value="undefined"` and the POST to `/api/v1/tokens` sent `scopes: ["undefined"]`, which the server rejected as invalid. (3) `apiFetch` (`static/js/common.js`) resolves `response.json()` unconditionally regardless of HTTP status, so the caller saw the error body as success data and read `data.token`, which was absent — the input displayed the literal string "undefined". Fixed the field name to `scope.scope` and added a defensive guard that throws with the server's error message when `data.token` is missing. (4) The `*` scope rendered with just a lone asterisk as its label above its description — easy to miss as "the all-permissions row has no name". Swapped the label order so the human description is the primary text and the scope code is a secondary muted monospace line below. Also added `max-h-[70vh] overflow-y-auto` to the Create Token modal body so the form is scrollable on shorter viewports.
- **`.gk-modal-backdrop` used as an outer overlay wrapper had no positioning, so modals in plugins that followed the `<div class="gk-modal-backdrop"><div class="gk-modal">…</div></div>` pattern (e.g. goatfictus LLM-provider add/edit, settings LLM picker) rendered inline at the bottom of the page instead of covering the viewport.** `.gk-modal-backdrop` CSS was only `background` + `backdrop-filter` with no `position`/`z-index`/flex. Added a fallback rule — `position: fixed; inset: 0; z-index: 50; display: flex; …` — guarded by `:not(.absolute):not(.fixed):not(.relative):not(.sticky)` so every existing Tailwind-inline call site (`class="gk-modal-backdrop fixed inset-0"`, ~30 places) keeps its own stacking context and the scoped `absolute inset-0 gk-modal-backdrop` usages on dynamic-module pages are untouched. Fixes any plugin modal using the outer-backdrop pattern without requiring per-plugin markup churn.
- **Scheduler is disabled entirely on the customer-facing frontend container.** `cmd/goats/main.go` now skips scheduler startup when `CUSTOMER_FE_ONLY=true` — no built-in jobs (`escalation-check`, `metrics-ticket-activity`, `generic-agent`, `email-ingest`) and no plugin-registered jobs fire there. Previously both the main `backend` container and `customer-fe` started the scheduler, so cron entries fired concurrently on two separate subprocesses and raced on the same pending work — any handler using `INSERT IGNORE` to dedupe on a composite key would silently drop the losing worker's richer row. The `backend` container remains the sole owner of all scheduler work.
- **Default business-calendar seed added for the escalation service.** Fresh installs had no `TimeWorkingHours` row in `sysconfig_default`, so the scheduler logged `scheduler: failed to initialize escalation service: failed to load default calendar: failed to get TimeWorkingHours: sql: no rows in result set` every minute and SLA escalation calculations never ran. Migration `000016_escalation_calendar_defaults` (both MySQL + Postgres) seeds a sensible default — 08:00–18:00 Monday–Friday, weekends off — in OTRS-style YAML (`{Mon:[8..17], …, Sat:[], Sun:[]}`). Admin edits via the sysconfig UI are preserved on re-run (`ON DUPLICATE KEY UPDATE` / `ON CONFLICT DO UPDATE` skip the `effective_value` column).
- **`demo-guard` middleware protection was silently unwired**, so `App.DemoMode=true` would no longer prevent non-admin users from changing passwords/MFA on shared demo instances. The middleware lived in the orphaned `IntegrateWithExistingSystem` code path (no callers) rather than `RegisterExistingHandlers`, which is what the live YAML route loader uses. Routes in `settings.yaml`/`agent.yaml`/`customer.yaml` that reference `demo-guard` were logging `Warning: Middleware 'demo-guard' not found for route …` on every boot and silently skipping the middleware instead of applying it. Registration moved into the live path; the orphaned `IntegrateWithExistingSystem` function and its helpers are flagged for a follow-up cleanup. `DemoGuard` / `DemoMode` are now nil-safe for test harnesses that wire handlers without initialising `config` — previously the first request through those middlewares panicked with a nil pointer dereference on `config.Get().App.DemoMode`.
- **Customer API-token routes referenced an unregistered `customer_auth` middleware.** The third YAML doc in `routes/api-tokens.yaml` (customer-facing `/customer/api/v1/tokens/*`) declared `middleware: [customer_auth]`, but the YAML loader has no `customer_auth` registered — the loader logged `Warning: Middleware 'customer_auth' not found, skipping` on every boot and the routes ran with no gate. The handlers are aliases to `HandleListTokens` / `HandleCreateToken`, which fail closed via `getUserContext` (401 when no identity is set), so customer token management wasn't exposed — but the declared gate was a lie. Switched the YAML to `unified_auth`, matching the agent and admin docs in the same file (customer handlers already detect role from context).
- **Scheduled plugin jobs were firing twice per interval.** `scheduler.Service.Run()` calls `scheduleAllJobs()` which iterates `s.jobs` and calls `addJobLocked` for every entry — including the jobs that `plugin.RegisterPluginJobs` had already scheduled via `AddJob` between `NewService` and `Run`. `addJobLocked` didn't check for an existing entry before creating a new cron registration, so every plugin job ended up with two cron entries firing on the same schedule. `s.entries[slug]` only tracked the newer entryID, but both fired forever. A plugin that used `INSERT IGNORE` on a composite-key table to dedupe concurrent writes would silently drop the losing worker's rows, which could strip any enrichment the winner's run hadn't produced. `addJobLocked` is now idempotent: if `s.entries[slug]` already exists it short-circuits and returns nil without scheduling a duplicate.
- **Plugin binary upload no longer fails with `text file busy` on overlay2.** `packaging.ExtractPlugin` opened every destination file with `O_TRUNC`, which triggers `ETXTBSY` on overlay2 when the destination is a recently-executed binary — even after the exec'd subprocess has exited, the filesystem keeps the text segment pinned to the original inode for long enough that the upload handler's 2-second post-unload wait is never sufficient. `extractZipFileWithLimits` now `os.Remove`s the destination before opening (ignoring `ENOENT` for first-time installs), which unlinks the old inode so `OpenFile` creates a fresh one and the kernel's stale text-segment reference becomes harmless. Operators previously had to work around this by `docker exec … rm -f <binary>` before each upload; that's no longer needed.
- **Plugin custom-field registration ordering race.** `Manager.Register` used to call `Plugin.Init(ctx, sandbox)` *before* `registerCustomFields`, so any plugin whose `InitWithHost` ran schema migrations or init-time queries against its own custom fields saw zero rows (the `gk_custom_field_def` entries hadn't been inserted yet). Every plugin with that pattern needed a deferred-backfill workaround. Registration order is now: sandbox → custom fields → `Init` — plugins see their CF defs on first run and the workaround can be removed.
- **`ReplacePlugin` only re-registered cascade handlers on hot reload.** Every other manifest side effect — custom-field definitions, translations, error codes, template overrides, UIs — was installed in `Register` but silently skipped when a plugin was replaced via upload, so reloaded plugins that had added or modified any of those would need a full backend restart to take effect. `Register` and `ReplacePlugin` now share `applyManifestSideEffectsPreInit` / `applyManifestSideEffectsPostInit` helpers so every manifest field is re-applied on each register or replace.

### Security
- **Go toolchain and vulnerable support libraries patched for release.** Container build defaults, CI, toolbox fallbacks, and helper scripts now pin Go 1.25.10 instead of the vulnerable 1.25.9 image stream, and `golang.org/x/net` / `golang.org/x/image` are bumped to v0.53.0 / v0.39.0 so `govulncheck` no longer reports the reachable Go standard-library, HTTP/2, or image-decoder advisories found during the pre-release scan.
- **Web push no longer pulls the vulnerable legacy JWT module.** Upgraded `github.com/SherClockHolmes/webpush-go` from v1.3.0 to v1.4.0, removing the transitive `github.com/golang-jwt/jwt` v3 dependency that `govulncheck` and Trivy reported as reachable through push notification delivery.
- **Query-string auth tokens are no longer accepted on ordinary routes.** `middleware.ExtractToken` now only honors `?token=` for WebSocket upgrades and known same-origin SSE endpoints, preventing bearer/API tokens in URLs from authenticating normal page or API requests while preserving browser streaming transports that cannot reliably set headers.
- **Dormant OAuth2 provider routes now fail closed.** `SetupOAuth2Routes` refuses to register endpoints unless real auth and admin middleware are supplied, the mock current-user fallback is gone, and PKCE validation now supports `plain` and `S256` challenge checks.
- **Authentication cookies now share one production-safe policy.** Login, logout, 2FA, WebAuthn, session-refresh, and auth middleware paths now set sensitive auth/session cookies through a shared helper that preserves local development defaults but forces `Secure` in production and applies the configured SameSite policy.

### Removed
- **`internal/routing/integration.go` orphaned code** (~430 lines). `IntegrateWithExistingSystem` and its helpers (`registerExistingHandlers`, `registerDevHandlers`, `registerCoreHandlers`, `registerAdminHandlers`, `registerAgentHandlers`, `registerCustomerHandlers`, `registerExistingMiddleware`, `wrapHandler`, `MigrateExistingRoutes`, `corsAllowedOrigins`, `originAllowed`) had zero external callers — remnants of an earlier routing design superseded by `LoadYAMLRoutesFromGlobalMap`. The file now contains only the live pieces: `globalRegistry`, `GetGlobalRegistry`, `SetGlobalRegistryForTest`, `GlobalHandlerMap`, `RegisterHandler`. This is also what made `demo-guard` registration invisible to the YAML loader (that registration lived inside the orphaned `IntegrateWithExistingSystem`).
- **`deletion.Service.cascadeHandlers` (instance-local) and `Service.RegisterCascade`.** Now that plugin cascade handlers live in the package-level registry (populated by `RegisterPluginCascade`), the instance-local handler map served no production callers and was only used by a few tests — which have been updated to register into the package-level registry with `t.Cleanup` unregistration. Less code, one source of truth for cascade dispatch.

## [0.8.2] - April 2026

**MCP v2, Plugin Manager Resilience, Go 1.25, and Security Upgrades**

### Added

**MCP Server v2 — Dynamic API Discovery**
- All MCP tools are now dynamically generated from YAML route definitions and the OpenAPI spec — no manual tool registration needed
- Every `/api/v1/` endpoint is automatically available as an MCP tool with input schema derived from OpenAPI
- Plugin endpoints are auto-discovered and exposed as MCP tools, namespaced by plugin name (e.g. `myplugin_run_task`)
- `MCPToolSpec` field added to `GKRegistration` — plugins can declare MCP tools with full JSON Schema input schemas
- API bridge executes tools by invoking real Gin handlers with synthetic context — RBAC enforced by the same middleware stack as the REST API
- `mcp_description` field on route YAML specs — override tool description with LLM-friendly text without changing API docs
- `mcp: false` field on route YAML specs — opt individual routes out of MCP tool generation
- Tool list refreshed automatically when plugins are enabled, disabled, or uploaded

**MCP Streamable HTTP Transport (SSE)**
- `POST /api/mcp/sse` — MCP 2025-03-26 Streamable HTTP endpoint with session management
- `GET /api/mcp/sse` — server-to-client SSE notification stream with 30s heartbeat keepalive
- `DELETE /api/mcp/sse` — session termination
- Session manager with configurable inactivity timeout (default 30 minutes) and background cleanup
- Protocol version negotiation — supports both `2024-11-05` and `2025-03-26`
- `unified_auth` middleware on SSE endpoints — supports both JWT and API tokens
- `.mcp.json` configuration now uses native SSE transport (no more stdio proxy)

**Admin SQL REST Endpoint**
- `POST /api/v1/admin/sql` — read-only SQL execution promoted from MCP-only to a proper REST endpoint
- Allowlisted statement types: SELECT, DESCRIBE, EXPLAIN, SHOW TABLES, SHOW COLUMNS (replaces SELECT-only)
- Admin middleware enforced — requires admin group membership
- Dialect-portable via `database.ConvertPlaceholders()`
- OpenAPI spec updated with request/response schemas

**Plugin Manager Resilience**
- **Plugin health checker.** Manager now runs an optional background goroutine that probes every loaded plugin every 60s via the reserved `__health_ping__` function name on the existing `Call` path. Any response within 5s (even an "unknown function" error from plugins that don't handle the name) is treated as healthy — only a context-deadline-exceeded indicates the plugin is wedged. Three consecutive failures flip `HealthStatus.Healthy` to false and log a warn-level transition; a subsequent successful probe flips it back and logs the recovery. Health state is exposed via `Manager.HealthStatus(name)` and `Manager.AllHealthStatuses()` for admin UI / dashboard rendering. No auto-restart yet — surfacing bad state is the goal for this release; restart-policy design lands in 0.8.3. Enabled by default; disable with `GOATFLOW_PLUGIN_HEALTH_CHECK=false`.

### Changed

**MCP Server**
- MCP server version bumped to `0.7.0`
- MCP server rewritten from ~1050 to ~130 lines — all tool implementations removed in favour of API bridge
- `tools.go` and `tools_custom_fields.go` deleted — 14 hardcoded tool definitions replaced by dynamic generation
- `NewServer()` signature changed — no longer takes `*sql.DB`, takes `userRole` and `*APIBridge` instead
- API token role resolution — MCP handlers now resolve actual admin role from database (fixes admin middleware for API tokens)
- `ListChanged: true` capability advertised — SSE clients can be notified when the tool list changes

**Plugin Manager Resilience**
- **Plugin shutdown now respects per-plugin timeouts.** `Manager.ShutdownAll` reads `ResourcePolicy.ShutdownTimeout` (or a 10s default) per plugin and applies it as a deadline on the RPC call. `GRPCPlugin.Shutdown(ctx)` no longer ignores its context — the Shutdown RPC runs in a goroutine guarded by the deadline, and `client.Kill()` runs unconditionally afterwards as a supervised teardown. The outer `cmd/goats` shutdown paths wrap the call in a 30s overall ceiling (`gracefulShutdownTimeout`) so no plugin — or combination of plugins — can wedge goatflow's process exit. Previously a hung plugin's Shutdown RPC would block every subsequent plugin's turn.

**Build & Toolchain**
- **Project Go toolchain bumped to 1.25.** Every reference across the build pipeline updated:
  - `go.mod` directive (`go 1.24.0` → `go 1.25.0`); SDK `sdk/go/go.mod` toolchain directive
  - All Go-using Dockerfiles: `Dockerfile`, `Dockerfile.config-manager`, `Dockerfile.goatkit`, `Dockerfile.route-tools`, `Dockerfile.tests`, `Dockerfile.toolbox`, `Dockerfile.playwright-go` (the last via its `GO_VERSION` arg controlling a manual curl install)
  - The manual Go-binary fallback download URL inside `Dockerfile.toolbox`
  - `Makefile` `GO_IMAGE` default; `.env.development` and `.env.example` `GO_IMAGE` values (the Makefile loads `GO_IMAGE` from `.env` as the single source of truth — env files had to move too)
  - Helper script defaults: `scripts/schema-discovery.sh`, `scripts/test-api-report.sh`, `scripts/test-all-apis.sh`
  - CI: `actions/setup-go` version in `.github/workflows/test.yml` (now `'1.25'`, auto-resolves latest 1.25.x)
  - Docs: README Go badge, design-doc workflow example
- **Toolbox dev-tool pins bumped wholesale** because the previous pins (`goimports v0.24.0`, `gosec v2.21.4`, `staticcheck 2024.1.1`, `golangci-lint v1.64.8`) all transitively depended on `golang.org/x/tools@v0.25.x` or older, which contains constant-arithmetic source that won't compile under Go 1.25 (`invalid array length -delta * delta` in `tokeninternal.go`). New pins: `goimports v0.42.0` (verified), `golangci-lint v2.5.0` (note: v2 changed import path to `/v2/cmd/...`), and `gosec` / `staticcheck` set to `latest` for now (refine when known-good tags are confirmed). The toolbox `govulncheck` pin (added earlier in this release as a 1.24 workaround) is also reverted to `latest`.
- Known follow-up: `Dockerfile`'s WASM-builder stage uses `tinygo/tinygo:0.32.0`, which only supports Go source up to ~1.22. WASM plugins that declare `go 1.25` will fail there until TinyGo is bumped (0.34+ for current Go support).

### Removed
- 14 hardcoded MCP tool implementations (replaced by dynamic generation via API bridge)
- `ToolRegistry` global variable (replaced by `GetDynamicTools()`)
- Direct SQL queries in MCP server (all tool execution now goes through the API bridge)

### Fixed
- **Plugin loader: `EnsureLoaded` no longer spawns duplicate instances of already-loaded plugins.** The previous implementation trusted a cached `discovered[].Loaded` boolean flag, which several reload/replace/unregister code paths fail to keep in sync with the manager's actual registry. When `AllWidgets()` (or any other caller) invoked `EnsureLoaded` on a plugin whose flag had desynced, the loader would spawn a second instance of a running gRPC plugin. The duplicate would exit within ~300ms with `acceptAndServe error: timeout waiting for accept` (usually a socket/port collision with the original), and the manager's routing would be left pointing at a broken-state instance. For stateful plugins holding long-lived network state — e.g. WireGuard peer maps — this manifested as silent peer loss requiring a full plugin redeploy to recover. `EnsureLoaded` now uses `manager.Get(name)` as the ground truth and refreshes the cache flag accordingly; the flag is a fast-path hint rather than the source of truth. A warn-level log is emitted when the flag is detected to have desynced.

### Security
- **`github.com/go-jose/go-jose/v3` upgraded to v3.0.5** (Dependabot **high**-severity GHSA — JWE decryption panic on certain malformed inputs). Indirect via the JWT/auth chain; upgrade is purely safety-driven, no API changes on our side.
- **`golang.org/x/image` upgraded to v0.38.0** (Dependabot **medium**-severity GHSA — out-of-memory error in TIFF decoder). v0.38.0 requires Go 1.25, enabled by the toolchain bump above. Pulled in transitively via `excelize/v2`; goatflow itself doesn't decode TIFFs so practical exploit surface was minimal, but the upgrade clears the alert and keeps the dep graph current.

---

## [0.8.1] - April 2026

**Mobile, PWA & Security**

### Security

- **CORS**: replaced wildcard `Access-Control-Allow-Origin: *` with origin validation against `CORS_ALLOWED_ORIGINS` env var; defaults to same-origin when unset
- **JWT**: production guard rejects weak or placeholder secrets (`APP_ENV=production` + secret < 32 chars or containing dev keywords → fatal on startup)
- **Dependencies**: `make check-deps` target runs `bun pm audit` / `npm audit` as part of `make test` pipeline

### Added

**Coachmarks — Additional Feature Tips**
- 6 new onboarding coachmark tips: dashboard widgets (`⚙️`), ticket creation (`🎫`), ticket filters (`🔍`), bulk actions (`☑️`), queue overview (`📋`), push notifications (`🔔`)
- Existing theme-switcher tip updated to use i18n keys instead of hardcoded English
- All 7 coachmarks fully translated in all 15 languages via `coachmarks.*` i18n keys
- Tips are page-aware (only show on relevant pages) with staggered delays and max view limits

**Mobile Optimization**
- Responsive table column hiding for agent ticket list — Customer, Queue, Assigned, Article count hidden below `md`; Age hidden below `lg`
- Responsive table column hiding for customer ticket list — Queue, Customer, Agent hidden below `md`; Updated hidden below `lg`
- Dashboard GridStack responsive breakpoints via `columnOpts` — 1 column (mobile), 6 columns (tablet), 12 columns (desktop); lock toggle hidden on mobile
- Mobile CSS component overrides in `input.css` — `@media (max-width: 767px)` block reduces `.gk-table` cell padding, `.gk-modal` header/body/footer padding, and stacks modal footer buttons vertically
- `.gk-action-btn` CSS class — 44px minimum touch target for admin action buttons (WCAG 2.5.8 compliance)
- Ticket detail tabs horizontal scroll — `overflow-x-auto` with `.scrollbar-hide` utility and `.tab-scroll-hint` gradient fade on mobile
- Responsive ticket detail header — scales from `text-xl` to `text-3xl` across breakpoints with `break-words` for long subjects
- Admin users table mobile optimization — Groups, 2FA, Last Login columns hidden below `lg`; action buttons use `.gk-action-btn`
- Meta grid card padding reduced on mobile (`p-3 sm:p-4`)
- `.scrollbar-hide` CSS utility — cross-browser scrollbar hiding for horizontal scroll areas

**Mobile Ticket Creation Flow**
- Customer ticket form mobile optimization — responsive heading (`text-2xl sm:text-3xl`), reduced card padding (`p-4 sm:p-6`), stacked form action buttons on mobile, collapsible tips section with Alpine.js toggle
- Agent ticket form mobile optimization — reduced container padding, compact file upload drop zone (`py-5 sm:py-10`, smaller icon), stacked action buttons on mobile
- Attachment upload partial — reduced drop zone height (`h-24 sm:h-32`), smaller icon and padding on mobile
- `.gk-card-body` mobile padding reduced from `p-6` to `p-4` below 768px
- Tiptap rich text editor toolbar buttons enlarged to `2.25rem` on mobile for better touch targets

**PWA Push Notifications**
- Web app manifest (`/manifest.json`) with app name, icons, standalone display mode, and theme color
- PWA meta tags in base layout — `<link rel="manifest">`, `<meta name="theme-color">`, Apple mobile web app tags, `<link rel="apple-touch-icon">`
- PWA icon assets — `icon-192.png` and `icon-512.png` generated from `favicon.svg`
- Service worker (`/sw.js`) — cache-first for static assets, network-first for navigation with offline fallback, push notification handler with notification click support
- Offline fallback page (`/static/offline.html`) — standalone page with GoatFlow branding for network failures
- Service worker registration in base layout `<head>`
- VAPID key infrastructure (`internal/push/vapid.go`) — P-256 ECDSA key generation, base64url encoding, auto-generate with warning when not configured
- Push notification sending (`internal/push/send.go`) — webpush-go integration with subscription expiry detection
- Push subscription database table (`gk_push_subscription`) — migration 000015 for MySQL and PostgreSQL
- Push subscription repository (`internal/push/repository.go`) — CRUD operations with multi-user lookup support
- Push notification API endpoints — `GET /api/push/vapid-key`, `POST /api/push/subscribe`, `DELETE /api/push/unsubscribe`
- Client-side push manager (`static/js/push-manager.js`) — subscribe/unsubscribe/isSubscribed with VAPID key fetch and PushManager integration
- Notification bell toggle in navbar — Alpine.js powered bell icon that enables/disables push subscriptions
- Push dispatch integration with scheduler — `DispatchPushReminder` sends push notifications alongside in-memory pending reminders, auto-removes stale subscriptions on 404/410
- Push configuration — `PushConfig` in config with `GOATFLOW_PUSH_ENABLED`, `GOATFLOW_PUSH_VAPID_PUBLIC_KEY`, `GOATFLOW_PUSH_VAPID_PRIVATE_KEY`, `GOATFLOW_PUSH_VAPID_CONTACT` env vars
- i18n keys for push notifications (`push.enable`, `push.disable`, `push.not_supported`, `push.permission_denied`) and offline page (`offline.title`, `offline.message`) in all 15 languages
- Static route support for `/manifest.json` and `/sw.js` with `Service-Worker-Allowed: /` header
- New Go dependency: `github.com/SherClockHolmes/webpush-go` v1.3.0

**Custom Fields — Atomic Operations**
- `FieldOp` type for atomic custom field updates via `CustomFieldsSet()`
- `increment` operation for integer/decimal fields with optional `Floor`/`Ceiling` bounds
- `append` and `remove` operations for multi_select fields (duplicate-safe)
- `cas` (compare-and-swap) operation for optimistic concurrency on any field type
- `toggle` operation for boolean fields
- Works transparently across gRPC and WASM plugins (detected as JSON map with `"op"` key)
- Full validation: type checking, option membership for multi_select, bounds for numeric
- Backward compatible — plain values continue to work as before

**Plugin Sidecar Containers**
- `SidecarSpec` in `plugin.yaml` — gRPC plugins can declare sidecar containers they require
- K8s mode: sidecars injected into the plugin pod spec (shared pod network, localhost communication)
- Docker Compose mode: `GenerateComposeFragment()` produces service definitions for sidecars
- Supports image, ports, env vars, volumes, privileged mode, memory/CPU limits, and health checks
- First consumer: goatkit-devices declares an ADB server sidecar for physical device fleet management

**Enterprise Plugin Ecosystem**
- 8 enterprise plugins scaffolded with schema, handlers, and private Gitea repos
- goatkit-media, goatkit-llm, goatkit-billing, goatkit-devices, goatkit-workflows, goatkit-audit, goatkit-content-feeds, goatkit-notify
- ROADMAP updated with enterprise plugin status

**Plugin Webhook Routes**
- `"webhook"` middleware keyword for `RouteSpec` — plugins declare unauthenticated endpoints for external callbacks
- HMAC-SHA256 signature verification with per-plugin signing secret (stored in secure config)
- Stripe-specific signature parsing (`t=<timestamp>,v1=<signature>`) with 5-minute replay protection
- Standard webhook headers supported: `X-Signature-256`, `X-Hub-Signature-256`, `X-Webhook-Signature`
- 1MB body size limit to prevent OOM on oversized payloads
- Secure by default — unsigned webhooks rejected unless `GOATFLOW_WEBHOOK_ALLOW_UNSIGNED=true`
- Request logging with method, path, source IP, plugin name, and verification result
- IP-based rate limiting per plugin (500 req/hr default, applied before signature verification)

**SQL Dialect Portability — Automatic Function Rewriting**
- `ConvertPlaceholders()` now transparently rewrites MySQL-specific SQL functions for PostgreSQL
- `DATE_SUB(expr, INTERVAL n UNIT)` → `(expr - INTERVAL 'n unit')` on PostgreSQL
- `DATE_ADD(expr, INTERVAL n UNIT)` → `(expr + INTERVAL 'n unit')` on PostgreSQL
- `UNIX_TIMESTAMP()` → `EXTRACT(EPOCH FROM NOW())::bigint` on PostgreSQL
- `UNIX_TIMESTAMP(expr)` → `EXTRACT(EPOCH FROM expr)::bigint` on PostgreSQL
- `CURDATE()` → `CURRENT_DATE` on PostgreSQL
- Reverse direction: `EXTRACT(EPOCH FROM expr)::bigint` → `UNIX_TIMESTAMP(expr)` on MySQL
- All plugin SQL queries automatically benefit (routed through `ProdHostAPI.DBQuery`)
- Core internal queries also benefit — no manual dialect branching needed

**Statistics & Reporting Plugin v2.0**
- SLA compliance report endpoint `/api/plugins/stats/sla-compliance` — adherence rates by queue with met/breached/rate breakdown
- Time tracking analytics endpoint `/api/plugins/stats/time-tracking` — hours logged by agent and queue, total hours
- Scheduled weekly report delivery via email — `report_email` job runs Monday 08:00, sends HTML report to admin users
- HTML email report template with overview, top queues, SLA compliance, and time tracking sections
- `stats_sla` dashboard widget — per-queue SLA compliance with colour-coded rate badges (green/amber/red)
- `stats_time_tracking` dashboard widget — total hours and top agents for the last 30 days
- Plugin version bumped to 2.0.0; i18n extended with SLA, time tracking, and report labels (en + de)
- Parameterized `LIMIT` in `recent_activity` query for consistent DB abstraction

**Automatic Org Context Injection**
- `_org_id` automatically injected into all plugin call params from the authenticated session
- Works for both YAML-routed plugin calls (`buildPluginArgs`) and direct API calls (`POST /api/v1/plugins/:name/call/:fn`)
- `SkipOrgInjection` opt-out flag in `GKRegistration` for plugins that handle multi-org queries themselves
- `Manager.SkipsOrgInjection(name)` accessor checks the flag before injection
- Only injected when org context is active (orgID > 0); single-org deployments unaffected
- `HostAPI.OrgID(ctx)` continues to work via Go context for plugins that prefer the programmatic API

**Plugin SSE Channel**
- `HostAPI.PublishEvent(channel, eventType, data)` — plugins push events to named channels
- Per-plugin SSE endpoint `/api/v1/plugins/:name/events/:channel` — clients subscribe to isolated streams
- Per-plugin channel isolation — `SSEBroker` filters by plugin name and channel; sandbox auto-injects plugin identity
- Auth-scoped — channel endpoint requires valid session or JWT (via `SessionOrJWTAuth` middleware)
- 30-second keepalive comments prevent proxy/browser idle connection timeouts
- Legacy `/api/v1/sse` endpoint preserved for backward compatibility (presence indicator, unscoped listeners)
- gRPC and WASM plugin runtimes updated — `publish_event` wire format now includes `channel` field

**Plugin File Storage API**
- `HostAPI.StoreFile()` / `GetFile()` / `DeleteFile()` / `ListFiles()` — platform-managed file storage for plugins
- Local disk backend — files stored under `$STORAGE_PATH/plugins/<plugin_name>/` with per-plugin namespace isolation
- Sidecar JSON metadata files (content-type, size, modification time, custom key-value pairs)
- Path traversal protection — sanitised keys, `..` and absolute paths rejected
- gRPC wire format: `store_file`, `get_file`, `delete_file`, `list_files` host methods
- SandboxedHostAPI enforcement — plugin name auto-injected from context
- S3-compatible backend (`GOATFLOW_STORAGE_BACKEND=s3`) — supports MinIO, R2, AWS S3 via configurable endpoint
- Org-scoped file storage — files namespaced under `<plugin>/org-<id>/` when org context is active
- `MaxFileStorageBytes` in `ResourcePolicy` — per-plugin storage quota (default 500MB), enforced before write
- Pluggable `FileStorageBackend` interface for custom storage implementations

**Deployment — Custom Caddy with DNS-01 TLS**
- `deploy/Dockerfile.caddy` — custom Caddy image with `caddy-dns/route53` module for DNS-01 ACME challenges
- Enables Let's Encrypt TLS certificates on VPN-only deployments where ports 80/443 are not publicly accessible
- DNS-01 validation via Route53 API — no inbound HTTP required for certificate issuance/renewal

## [0.8.0] - March 2026

**GoatKit PaaS Core** — Universal custom fields, plugin UI system, multi-tenancy, secure settings, entity deletion, reusable components, plugin marketplace, self-service authentication, and accessibility.

### Added

**Custom Fields (GoatKit PaaS Core)**
- Universal EAV custom fields on all entity types (ticket, article, contact, agent, group, queue, organisation)
- 15 field types including GIS: text, textarea, integer, decimal, boolean, date, datetime, select, multi_select, url, email, phone, point (lat/lng), polygon (GeoJSON), address (structured + auto-geocode)
- Plugin registration via `CustomFieldSpec` in `GKRegistration` with auto-prefixed names
- HostAPI methods: `CustomFieldsGet()`, `CustomFieldsSet()`, `CustomFieldsQuery()` with sandbox prefix enforcement
- Auto UI rendering partial (`custom_fields.pongo2`) with edit/view/inline modes
- Admin-defined custom fields via admin UI
- Legacy `dynamic_field` auto-migration on startup (copy, not move — downgrade-safe)
- Validation engine: 15 type-specific validators with regex timeout, GeoJSON validation
- REST API v1 endpoints for definitions CRUD, entity values, and field queries
- MCP tools: `custom_fields_get`, `custom_fields_set`, `custom_fields_query`, `custom_fields_list`
- Database migrations: `gk_custom_field_def` + `gk_custom_field_value` (MySQL + PostgreSQL)

**Plugin UI System (GoatKit PaaS Core)**
- Independent plugin UIs with dedicated routing under `/ui/{plugin}_{ui_id}/`
- 5 UI types: admin_page, agent_app, customer_app, public_page, kiosk
- 3 shell templates: standard (full GoatFlow chrome), minimal (mobile-first with bottom/top/side nav), none (raw HTML)
- Per-UI branding (logo, colour, favicon, app name) via `UIBrandingSpec`
- PWA manifest auto-generation at `/ui/{id}/manifest.json`
- Auth per UI type: session, PIN, token, none
- Navigation integration — plugin UIs auto-appear in agent/customer/admin nav bars
- Badge counts on nav items resolved via plugin function calls
- Data scoping for customer UIs (self, org, all)
- Database migration: `gk_plugin_ui`

**Organisations & Multi-Tenancy (GoatKit PaaS Core)**
- `gk_organisation` table with hierarchy (parent_id), status, slug-based routing
- `gk_user_organisation` membership for agents AND customers with roles (member, admin, owner)
- Per-org sysconfig overrides via `sysconfig_org` table — extends existing sysconfig cascade
- Config resolution: User Preference → Org Override → System Override → System Default
- Org context middleware — resolves active org from cookie or default membership
- Org switcher UI component in navigation bar (HTMX-powered)
- HostAPI `OrgID()` method for plugins to read active org
- Automatic org-scoped query rewriting in `SandboxedHostAPI` — `DBQuery`/`DBExec` auto-inject `org_id` filters for registered org-aware tables
- `RegisterOrgAwareTable()` for plugins to opt their tables into org scoping
- Org switching API: `POST /api/v1/session/org`, `GET /api/v1/session/orgs`
- Admin API: full CRUD for organisations, members, per-org config overrides
- Backward compatible — zero organisations = single-org mode

**Secure Settings (GoatKit PaaS Core)**
- AES-256-GCM encrypted key-value storage for plugin secrets
- Platform-managed encryption key (`GOATFLOW_SECURE_KEY` env var or auto-generated)
- HostAPI methods: `SecureConfigGet()`, `SecureConfigSet()` with sandbox plugin-name enforcement
- Org-scoped secrets (org-specific → global fallback)
- Masked display helpers (`ValueHint` last-4 chars, `MaskedDisplay` for admin UI)
- Database migration: `gk_secure_config`

**Entity Deletion (GoatKit PaaS Core)**
- Soft delete → recycle bin with configurable retention periods per entity type
- PII anonymisation on soft delete (configurable per entity type, irreversible `[DELETED]` replacement)
- Hard delete (purge) — physical removal with cascading linked data
- Restore from recycle bin
- Plugin cascade handlers via `CascadeSpec` in `GKRegistration` (OnSoftDelete, OnHardDelete)
- Immutable tombstone logging (`gk_deletion_log`)
- Auto-purge scheduled job with configurable retention
- Batch/scope delete: `ScopeSoftDelete()` and `ScopeHardDelete()` for bulk operations
- RBAC `entity.hard_delete` permission (admin-only)
- Recycle bin admin UI with HTMX restore/purge, entity type filter, deletion log viewer
- Database migrations: `gk_recycle_bin` + `gk_deletion_log`

**Reusable UI Components**
- `gk-daily-queue` — ordered task list with priority indicators, status badges, HTMX action buttons
- `gk-week-calendar` — week-at-a-glance grid with colour-coded events
- `gk-progress-bar` — counter with animated bar and configurable colour
- `gk-stat-card` — dashboard metric card with icon, trend indicator, optional link
- `gk-quick-action` — mobile-friendly tap targets with responsive grid
- `gk-file-dropzone` — drag-and-drop file upload with progress bars and XHR upload
- `gk-presence-indicator` — real-time collaborative viewing/editing indicators via SSE
- All components theme-aware (CSS variables) and WCAG 2.1 AA accessible

**Plugin Ecosystem Expansion**
- Plugin marketplace: `gk install/update/search` CLI commands with GitHub Releases backend
- Plugin dependency resolution: `Dependencies` field in manifest, `ResolveDependencies()`, `TopologicalSort()` with circular dependency detection
- Theme-as-plugin: `PluginType: "theme"` in manifest, auto-extraction to theme cache
- Plugin update notifications: `CheckUpdates()` compares installed versions against marketplace index
- Kubernetes pod isolation: `GOATFLOW_PLUGIN_ISOLATION=k8s`, generates Deployment + Service + NetworkPolicy YAML

**Self-Service Authentication**
- Customer password recovery with email-based reset tokens (1hr expiry, anti-enumeration)
- Customer self-registration with approval workflow (pending/approved/rejected)
- Email verification with token-based verification links (24hr expiry)
- CAPTCHA integration: reCAPTCHA v3 (score-based) and hCaptcha support
- Database migrations: `gk_auth_token` + `gk_registration_request`

**Accessibility & Enhancements**
- Keyboard navigation: skip-to-content link, focus-visible detection, arrow key menu nav, Escape closes dropdowns/modals, focus trapping in modals
- Screen reader support: `announceToSR()` for dynamic content, ARIA attributes on all new components
- `accessibility.js` module loaded globally

**Internationalisation**
- All new features translated to 15 native languages: Arabic, Chinese, English, Farsi, French, German, Hebrew, Japanese, Klingon, Polish, Portuguese, Russian, Ukrainian, Urdu

**Design Specifications**
- `docs/design/CUSTOM_FIELDS.md`
- `docs/design/PLUGIN_UIS.md`
- `docs/design/ORGANISATIONS.md`
- `docs/design/SECURE_SETTINGS.md`
- `docs/design/ENTITY_DELETION.md`
- `docs/design/PLUGIN_MARKETPLACE.md`

### Fixed
- **2FA login sets wrong JWT role**: The 2FA verification completion path generated JWTs with `role=user` and `isAdmin=false`, bypassing the admin group check. Admin users who logged in with 2FA were denied access to plugin management and other admin API endpoints. All login paths (direct, 2FA, demo) now use a shared `resolveUserRole()` function that checks admin group membership.
- **Plugin API auth middleware**: `SessionOrJWTAuth()` relied on a prior middleware setting `user_id` in context, but plugin API routes had no session middleware. Now validates the session cookie JWT directly, matching the same flow as `JWTAuthMiddleware()`.
- **Go 1.24.13 upgrade**: Updated from Go 1.24.0/1.24.11 to 1.24.13, fixing 6 stdlib vulnerabilities (html/template, os, net/url, crypto/tls).
- **JS dependency vulnerabilities**: Updated picomatch via `npm audit fix`, resolving 4 Dependabot alerts (2 high ReDoS, 2 medium method injection).
- **Test pollution across packages**: Added verify-and-recreate pattern for MCP and api/v1 test fixtures, TestMain teardown functions, fixed password hashing test to use fixed IDs (70000 range)

### Security
- **Stored XSS in ticket notes (gotrs-io/gotrs-ce#176)**: HTML content in ticket articles and notes was rendered unsanitised via `|safe` in templates. Malicious `<script>` tags injected via the rich text editor were executed on page view. Fixed with defence-in-depth: (1) write-side sanitisation on ticket creation and note submission, (2) read-side sanitisation when loading article bodies for both agent and customer views. All paths now use bluemonday HTML sanitiser.
- **Null byte injection in file uploads**: Filenames containing null bytes (e.g., `shell.php\x00.jpg`) could bypass extension validation — the OS truncates at the null byte, creating executable files. Now rejected outright in `validateFile()`, stripped in upload handler and `sanitizeFilename()`. Path traversal also blocked via `filepath.Base()`.
- **Content-Security-Policy header**: Added `SecurityHeaders` middleware setting CSP (`script-src 'self'` — blocks inline scripts), `X-Frame-Options: DENY` (anti-clickjacking), `X-Content-Type-Options: nosniff`, `Referrer-Policy`, and `Permissions-Policy`. Applied globally on all responses.
- **Org-scoped query injection**: `SandboxedHostAPI` auto-injects `org_id` filters on `DBQuery`/`DBExec` for org-aware tables, preventing cross-tenant data leakage in plugins.
- **Secure settings encryption**: AES-256-GCM with authenticated encryption prevents tampering. Plugins isolated by name — cannot access other plugins' secrets.

## [0.7.0]

### Fixed
- **Parallel test interference in article_create_test** — re-ensure RBAC permissions before each subtest and skip gracefully if a 404 is returned due to parallel `group_user` removal, preventing flaky failures in CI
- **Escalation integration test flakiness** — always create a fresh ticket in `ensureTestTicket` instead of reusing existing shared state, eliminating race conditions across parallel test runs
- **Plugin admin page missing sidebar**: `HandleAdminPlugins` and `HandleAdminPluginLogs` now pass `ActivePage: "admin"` to the template context, restoring the admin navigation sidebar on plugin pages.
- **Plugin admin page missing sidebar and context**: `HandleAdminPlugins` and `HandleAdminPluginLogs` now pass `ActivePage: "admin"`, `User`, and `IsInAdminGroup` to the template context, restoring the admin navigation sidebar and user-aware rendering on plugin pages.
- **Nineties-vibe dark mode login styling**: Added theme-specific overrides for login card, form inputs, buttons, and checkboxes to ensure proper contrast against the terminal-black background. Login card gets `#1a1a1a` background with visible border, inputs get dark background with light borders, and buttons use the primary colour.
- **sysconfig INSERT failures**: `seedDefaultDisabled` and `savePluginEnabled` now fill all NOT NULL columns for `sysconfig_default` and `sysconfig_modified` tables, preventing "Field 'X' doesn't have a default value" errors on strict-mode MySQL/MariaDB.
- **Customer ticket queue routing**: Tickets created via the customer portal were always routed to Postmaster (queue_id hardcoded to 1). Now resolves the customer's organisation queue via `group_customer` → `queue.group_id`, falling back to Postmaster only if no org queue mapping exists. (`internal/api/customer_routes.go`)

### Security
- **Plugin Signing** (`internal/plugin/signing/signing.go`): Ed25519 signature verification for plugin binaries. `SignBinary()` creates `.sig` files with SHA-256 hash signatures; `VerifyBinary()` checks against trusted public keys. Opt-in via `GOATFLOW_REQUIRE_SIGNATURES=1`.
- **SQL Table Whitelisting**: `extractTableNames()` parses SQL queries and validates table names against the `db` permission scope. Queries touching unallowed tables are rejected.
- **Call Depth Limiting**: Plugin-to-plugin call chains tracked via context with maximum depth of 10, preventing infinite recursion loops.
- **Config Key Blacklist**: Sensitive configuration patterns (database.*, password, secret, token, auth, ldap.*, smtp.*, aws.*, etc.) are blocked from plugin access by default.
- **Email Domain Scoping**: Email permission scope supports domain patterns (e.g. `["@example.com"]`); recipients outside allowed domains are rejected. Rate limited to 10 emails/minute per plugin.
- **Caller Identity Stamping**: gRPC HostAPI server stamps the authenticated plugin name on all calls; plugins cannot impersonate other plugins.
- **ZIP Extraction Security** (`internal/plugin/packaging/`): Symlink detection and rejection, 100MB per-file limit, 500MB total extraction limit, 1000 file maximum.
- **Live Policy Updates**: `SandboxedHostAPI.UpdatePolicy()` with RWMutex-protected policy pointer; policy changes take effect immediately without plugin restart.
- **Atomic Blue-Green Plugin Reload**: `Manager.ReplacePlugin()` initializes the new plugin before shutting down the old one, with atomic swap under mutex — no request-dropping window during hot reload.
- **Policy Persistence**: Resource policies serialized as JSON to `sysconfig_modified` table (key: `Plugin::<name>::Policy`); survives restarts.
- **WASM Security Verified**: Confirmed `SandboxedHostAPI` is correctly applied to WASM plugins, enforcing the same permission/rate-limit/accounting model as gRPC.

### Added
- **Dashboard Widget Grid (gridstack.js)**: Agent dashboard now uses [gridstack.js](https://gridstackjs.com/) for drag-and-resize widget layout. 12-column grid with lock/unlock toggle (locked by default). Layout auto-saves per user and restores on reload. Bundled locally in `static/vendor/` for offline use.
- **Stats Plugin — expanded widgets**: Stats WASM plugin now provides 6 dashboard widgets: Total Tickets, Open, Closed, New Today, Pending, and Overdue. Displayed in two rows of three.
- **Stats Plugin — RBAC queue filtering**: `GetPluginWidgets()` receives the caller's queue admin status and accessible queue IDs via Gin context, so non-admin agents only see stats for their own queues.
- **Admin Dashboard — Recent Activity**: Shows real entries from `admin_action_log` with action, target, actor, and human-readable relative timestamps ("5 minutes ago", "yesterday"). Falls back to "No recent admin activity" when empty.
- **Admin Dashboard — redesign**: Collapsible accordion sections (Platform, GoatFlow, Customers, Plugin Administration, Recent Activity) with localStorage-persisted open/close state. Rotating ticket activity metrics (created/closed by day/week/month, open right now).
- **Plugin Navigation Control**: Plugins can now hide built-in nav items (`HideMenuItems` field in `GKRegistration`) and set a custom landing page (`LandingPage` field). Well-known nav item IDs: `dashboard`, `tickets`, `queues`, `phone_ticket`, `email_ticket`, `admin`. Hidden items are removed from both desktop and mobile navigation. Landing page overrides the default post-login redirect for non-customer users.
- **Reminder Preferences**: Per-user toggle to enable/disable pending ticket reminder notifications. Default: enabled. Available on the agent profile page under Preferences. API: `GET/POST /agent/api/preferences/reminders-enabled`. When disabled, the reminder feed returns empty without deleting any data.
- **gRPC Plugin Hot Reload**: Loader discovers gRPC plugins via `plugin.yaml` in subdirectories, watches binaries via fsnotify, auto-reloads on change with 500ms debounce. New `plugin.yaml` files trigger discovery and loading; removing them triggers unload.
- **Plugin Isolation / SandboxedHostAPI** (`internal/plugin/sandbox.go`): Per-plugin permission enforcement wrapper around HostAPI
  - Permission checks on every HostAPI call (db read/write, cache, HTTP, email, config, plugin-to-plugin calls)
  - DDL blocking for read-only plugins (DROP, ALTER, TRUNCATE, CREATE, GRANT, REVOKE)
  - HTTP URL pattern filtering with wildcard subdomain matching
  - Cache key auto-namespacing (`plugin:<name>:<key>`) to prevent cross-plugin collisions
  - Plugin-to-plugin call scoping (allowlist of callable plugins)
  - Blocked status kills all HostAPI access
- **Rate Limiting per Plugin**: Sliding window rate limiters for DB queries/min, HTTP requests/min, and calls/sec. Configurable per plugin via ResourcePolicy.
- **Resource Accounting**: Atomic counters tracking DB queries, DB execs, cache ops, HTTP requests, plugin calls, errors, and last call timestamp per plugin. Accessible via `Manager.PluginStats()` and `Manager.AllPluginStats()`.
- **ResourceRequest / Permission / ResourcePolicy types** (`pkg/plugin/plugin.go`): Plugins declare what they need (`ResourceRequest` with `Permission` entries); platform enforces what they get (`ResourcePolicy` set by admin). `DefaultResourcePolicy()` provides restrictive defaults for new plugins (pending_review, DB read-only, cache RW, rate limited).
- **Manager policy integration**: Every plugin gets a SandboxedHostAPI on registration. Admin can set/get policies via `Manager.SetPolicy()` / `Manager.GetPolicy()`. Policy changes take full effect on next plugin reload.
- **gRPC call timeouts**: Context-based per-call deadlines with goroutine + select pattern in `GRPCPlugin.Call()`.
- **plugin.yaml format** for gRPC plugins: Declares name, version, runtime, binary path, and resource requests (memory, timeouts, permissions).
- **Session auth tests for plugin management**: 26 subtests with dynamic route discovery verifying admin access, non-admin 403s, and unauthenticated 401s for plugin enable/disable and log endpoints with session-based (cookie) authentication.
- **i18n keys for plugin admin**: `admin.select_plugin_file` and `admin.plugin_file_types` with proper translations across all 15 languages.
- **Unified dynamic engine** (`MountDynamicEngine`): YAML routes and plugin routes merged into a single hot-reloadable Gin engine mounted via `NoRoute`. Replaces the old `ROUTES_SELECTIVE` env var approach and separate `RegisterPluginRoutes` call. Rebuilt atomically when YAML files change or plugins are loaded/reloaded.
- **Plugin layout wrapping**: Plugin route responses returning `{html, title}` are automatically wrapped in the base layout template (`plugin_wrapper.pongo2`) with full sidebar/nav. Plugins can opt out with `{raw: true}` for bare HTML.
- **Plugin menu items in navigation**: Plugins register `MenuItemSpec` entries (with icon, label, path, location, order). These appear in the agent/customer nav bar and admin sidebar via `PluginMenuProvider` injection into all templates.
- **Dashboard plugin admin section**: Admin dashboard shows a "Plugin Administration" card grid for plugins that register admin menu items, with icon and plugin name.
- **HostAPI client for plugins** (`pkg/plugin/grpcutil/hostapi.go`): Full plugin-side HostAPI RPC client implementation. Plugins import this package to call back to the host for DB queries, cache, HTTP, email, config, and i18n — no need to hand-roll RPC boilerplate.
- **Plugin config via environment variables**: Plugins receive configuration from `GOATFLOW_PLUGIN_<NAME>_<KEY>` env vars, passed as lowercase keys in the `Init(config)` map. Documented in `docker-compose.yml`.
- **Plugin sandbox env forwarding**: gRPC plugin sandbox (`sandbox_linux.go`) now forwards `GOATFLOW_PLUGIN_*` environment variables to plugin subprocesses. Previously the sandbox's minimal environment stripped these, preventing plugins from receiving their configuration when running under OS-level isolation.
- **Plugin sandbox SkipHostEnv**: Set `SkipHostEnv: true` on go-plugin `ClientConfig` to prevent the host process environment from leaking into plugin subprocesses. Without this, `os.Environ()` is appended by go-plugin, shadowing sandbox-controlled variables and exposing DB credentials/JWT secrets.
- **Plugin group-based RBAC**: Plugins can declare access control groups via `GKRegistration.Groups` (`[]GroupSpec`). Groups are auto-created in the GoatFlow groups table on plugin load (`EnsurePluginGroups`). Routes reference groups with `group:<name>` middleware (e.g. `"group:myplugin-users"`). `RequireGroup()` middleware checks `group_user` membership; admin users bypass group checks.
- **Customer ID autocomplete**: Customer user creation form now uses the GoatKit autocomplete component (`data-gk-autocomplete`) with company seed data, replacing the plain text input. Supports typeahead search with `{name} ({customer_id})` display format.

### Changed
- **Makefile unit test command** — skip `./generated/...` package when no generated Go files exist (avoids `no Go files` build error on clean checkouts); also ensures `generated/test-results/` dir is created before running
- **test-runner.sh hardening** — add `set -eo pipefail`, fail fast if test stack doesn't start, detect missing `docker-buildx`, guard against zero-tests-ran scenario, and fix package pass/fail grep patterns to ignore bare summary lines
- **setup-test-admin.sh** — suppress CLI output via `>/dev/null 2>&1` and clarify fallback message to reference correct `goats` binary name
- **Plugin admin pages converted to self-hydrating templates**: `HandleAdminPlugins` and `HandleAdminPluginLogs` no longer fetch data server-side. Templates self-hydrate via client-side API calls (`GET /api/v1/plugins`, `GET /api/v1/plugins/logs`), following GoatKit's "YAML route + dynamic template" philosophy. Enable/disable and upload actions update the page without full reload. Go handlers reduced to a generic `renderAdminPage()` that passes standard admin context only.
- **Universal plugin package format**: Plugin packaging now uses `plugin.yaml` (YAML) instead of `manifest.json` (JSON) as the standard manifest. ZIP uploads support three runtime types: `wasm` (WebAssembly binary), `grpc` (native binary), and `template` (pure YAML routes + templates, no runtime). gRPC binaries are automatically made executable on extraction.
- **Shared `PluginManifest` type**: Moved to `pkg/plugin/manifest.go` so both the loader and packaging systems use the same struct. Added `description`, `author`, `license`, `homepage`, and `wasm` fields.
- **Test/example plugins disabled by default**: `hello`, `hello-wasm`, `hello-grpc`, and `test-hostapi` plugins are now disabled by default via sysconfig seeding on first registration. Can be enabled via admin UI/API; state persists to sysconfig.
- **Plugin management audit logging**: Plugin uploads, enables, disables, discovery, load/unload, and errors now log to the Plugin Logs page.
- **Plugin API dual auth**: Plugin management endpoints (`/api/v1/plugins/...`) now accept both session-based (cookie) and JWT authentication via `SessionOrJWTAuth()` middleware. Previously only JWT was accepted, blocking admin actions from the web UI.
- **Plugin handler tests use real test DB**: Removed `mockHostAPI` from plugin handler tests. All tests now use `ProdHostAPI` with `getTestDB(t)`, following the same patterns as every other test in the package.
- **Public plugin interfaces** (`pkg/plugin/`): Extracted plugin types (`Plugin`, `GKRegistration`, `HostAPI`, all spec types) from `internal/plugin` to `pkg/plugin/plugin.go` so external plugin authors can import them directly. Internal code uses type aliases for backwards compatibility.
- **Public gRPC plugin utilities** (`pkg/plugin/grpcutil/`): Extracted `ServePlugin()` helper and related types to `pkg/plugin/grpcutil/serve.go` for use by external gRPC plugins.
- **Refactored token extraction middleware**: Centralised token extraction logic in `internal/middleware/api_token.go`, removing duplicate code from auth, session, and routing packages.
- **Plugin documentation updated**: All 7 plugin docs rewritten to reflect implemented features — removed "planned"/"not yet implemented" language, added sandbox/security model documentation, corrected Host API signatures.
- **Universal plugin package format**: Plugin packaging now uses `plugin.yaml` (YAML) instead of `manifest.json` (JSON) as the standard manifest. ZIP uploads support three runtime types: `wasm` (WebAssembly binary), `grpc` (native binary), and `template` (pure YAML routes + templates, no runtime). gRPC binaries are automatically made executable on extraction.
- **Shared `PluginManifest` type**: Moved to `pkg/plugin/manifest.go` so both the loader and packaging systems use the same struct. Added `description`, `author`, `license`, `homepage`, and `wasm` fields.
- **Test/example plugins disabled by default**: `hello`, `hello-wasm`, `hello-grpc`, and `test-hostapi` plugins are now disabled by default via sysconfig seeding on first registration. Can be enabled via admin UI/API; state persists to sysconfig.
- **Plugin management audit logging**: Plugin uploads, enables, disables, discovery, load/unload, and errors now log to the Plugin Logs page.
- **Plugin API dual auth**: Plugin management endpoints (`/api/v1/plugins/...`) now accept both session-based (cookie) and JWT authentication via `SessionOrJWTAuth()` middleware. Previously only JWT was accepted, blocking admin actions from the web UI.
- **MCP test fixtures hardened**: Removed `sync.Once` caching (fixtures recreated each run to handle DB state contamination), replaced `t.Skip` with `t.Fatal` for missing DB, stronger assertions with `require`.
- **gRPC RPC encoding switched from gob to JSON**: `GKRegister` and HostAPI calls now use JSON serialization instead of Go's `encoding/gob`, enabling cross-language plugin development. Both client and server sides updated.
- **gRPC plugins eagerly loaded in lazy mode**: When `GOATFLOW_PLUGIN_LAZY_LOAD=true`, gRPC plugins are still eagerly loaded at startup because they register routes that Gin needs before accepting requests. WASM plugins remain lazy.
- **Plugin hot reload enabled by default**: No longer requires `GOATFLOW_PLUGIN_HOT_RELOAD=true` or `GOATFLOW_ENV=development`. Disable explicitly with `GOATFLOW_PLUGIN_HOT_RELOAD=false`.
- **Plugin loader watches new directories**: `handleFSEvent` now watches newly created subdirectories and checks them for `plugin.yaml`, enabling discovery of plugins added after startup.
- **Plugin LoadOrReload**: New `Loader.LoadOrReload()` method re-discovers plugins and reloads by name — used after ZIP upload to pick up newly extracted plugins without restart.
- **SSE (Server-Sent Events) for plugins** (`internal/plugin/sse.go`): Real-time server→browser event push for plugin UIs. `SSEBroker` with pub/sub channels, per-plugin event filtering, non-blocking publish (buffered channels, slow clients drop events). `PublishEvent(ctx, eventType, data)` added to HostAPI interface — plugins call it to push updates to connected browsers. Endpoint at `GET /api/v1/sse?plugin=<name>`. Supported by gRPC, WASM, and sandboxed HostAPI implementations. Includes htmx SSE extension (`static/js/htmx-sse.js`) for declarative UI binding via `sse-connect` / `sse-swap`.
- **Plugin work_dir and plugin_dir**: `buildPluginConfig()` now automatically provides `work_dir` (`data/plugins/<name>/`) and `plugin_dir` (`config/plugins/<name>/`) to every plugin. `work_dir` is auto-created on init for persistent writable storage (screenshots, media, etc.).
- **Plugin data persistence**: Docker Compose adds `goatflow_plugins` volume at `/app/config/plugins` so plugin binaries survive container restarts.

### Changed
- **Dockerfile**: Creates `/app/data/plugins` directory owned by `appuser:appgroup` for plugin runtime data.

### Changed
- **Dashboard stats migrated to WASM plugin**: Hardcoded ticket statistics grid removed from the agent dashboard template. Stats are now served entirely by the `stats` WASM plugin, making the dashboard extensible without code changes.
- **Dashboard "New Ticket" respects plugin nav hiding**: The "New Ticket" button is conditionally rendered based on `HiddenNavItems.tickets`, so plugins that hide the tickets nav item also hide the button.

### Fixed
- **Customer user creation missing timestamps**: `INSERT INTO customer_user` was missing `create_time` and `change_time` columns, causing `Error 1364: Field 'create_time' doesn't have a default value` on MariaDB strict mode.
- **`LandingPage()` skipped wrong plugins**: Plugin manager's landing page resolver had an inverted condition — it was skipping the plugin that *did* declare a landing page and checking ones that didn't.
- **Reminders preference route missing**: `GET/POST /agent/api/preferences/reminders-enabled` returned Guru Meditation (404) because the routes were absent from `routes/agent.yaml`. Added routes and registered handlers.
- **Stats plugin Overdue query wrong column**: Was querying `escalation_destination_date` (doesn't exist in default schema). Changed to `escalation_time` (epoch int) with correct `> 0 AND < UNIX_TIMESTAMP()` logic.
- **Gridstack CSS not loading**: Initial integration used `column: 2` but GoatKit only ships `gs-12` CSS rules. Changed to 12-column grid with correct size mappings (small=6×2, medium=6×3, large=12×4, full=12×2).
- **JWT missing admin role on login**: Login handler generated JWTs with `role: "user"` and `isAdmin: false` regardless of actual group membership. YAML route auth middleware compensated with a DB lookup, but plugin routes (dynamic engine) trust JWT claims directly — causing "admin access required" errors for admin users on plugin pages. Now checks `admin` group membership at login and bakes correct role/isAdmin into the JWT. Also fixed in 2FA completion path.
- **Dashboard widget ordering ignored saved position**: Plugin widgets rendered in discovery order rather than user-configured position. The handler filtered by enabled/disabled but discarded the `Position` field. Now sorts widgets by saved position after filtering.
- **Unified dynamic engine plugin route auth**: Plugin routes on the dynamic sub-engine now use `SessionOrJWTAuth()` instead of bare `JWTAuthMiddleware()`, consistent with plugin API routes.
- **Plugin log filter ignored level when filtering by plugin**: `HandlePluginLogs` used mutually exclusive `if/else if` for plugin name and level filters — specifying both `plugin=X&level=error` silently ignored the level. Now applies both filters together.
- **savePluginEnabled FK constraint violation**: Was inserting `sysconfig_default_id=0` into `sysconfig_modified`, which violates the foreign key constraint. Now looks up the actual `sysconfig_default` ID first.
- **Nineties-vibe dark mode login styling**: Added theme-specific overrides for login card, form inputs, buttons, and checkboxes to ensure proper contrast against the terminal-black background.
- **Customer ticket queue routing**: Tickets created via the customer portal were always routed to Postmaster (queue_id hardcoded to 1). Now resolves the customer's organisation queue via `group_customer` → `queue.group_id`, falling back to Postmaster only if no org queue mapping exists.

### Security
- **OS-Level gRPC Process Isolation** (`internal/plugin/grpc/sandbox_linux.go`): Linux namespace isolation (CLONE_NEWNS, CLONE_NEWPID), Pdeathsig to kill orphans, minimal environment (no DB credentials leaked to plugins). Container-aware: detects Docker/Podman/LXC/K8s environments and gracefully skips namespace creation where it would fail (EPERM). Non-Linux platforms log a warning.
- **Plugin Signing** (`internal/plugin/signing/signing.go`): Ed25519 signature verification for plugin binaries. `SignBinary()` creates `.sig` files with SHA-256 hash signatures; `VerifyBinary()` checks against trusted public keys. Opt-in via `GOATFLOW_REQUIRE_SIGNATURES=1`.
- **SQL Table Whitelisting**: `extractTableNames()` parses SQL queries and validates table names against the `db` permission scope. Queries touching unallowed tables are rejected.
- **Call Depth Limiting**: Plugin-to-plugin call chains tracked via context with maximum depth of 10, preventing infinite recursion loops.
- **Config Key Blacklist**: Sensitive configuration patterns (database.*, password, secret, token, auth, ldap.*, smtp.*, aws.*, etc.) are blocked from plugin access by default.
- **Email Domain Scoping**: Email permission scope supports domain patterns (e.g. `["@example.com"]`); recipients outside allowed domains are rejected. Rate limited to 10 emails/minute per plugin.
- **Caller Identity Stamping**: gRPC HostAPI server stamps the authenticated plugin name on all calls; plugins cannot impersonate other plugins.
- **ZIP Extraction Security** (`internal/plugin/packaging/`): Symlink detection and rejection, 100MB per-file limit, 500MB total extraction limit, 1000 file maximum.
- **Live Policy Updates**: `SandboxedHostAPI.UpdatePolicy()` with RWMutex-protected policy pointer; policy changes take effect immediately without plugin restart.
- **Atomic Blue-Green Plugin Reload**: `Manager.ReplacePlugin()` initializes the new plugin before shutting down the old one, with atomic swap under mutex — no request-dropping window during hot reload.
- **Policy Persistence**: Resource policies serialized as JSON to `sysconfig_modified` table (key: `Plugin::<name>::Policy`); survives restarts.
- **WASM Security Verified**: Confirmed `SandboxedHostAPI` is correctly applied to WASM plugins, enforcing the same permission/rate-limit/accounting model as gRPC.

## [0.6.5]

### Security
- **Two-Factor Authentication (TOTP)**: Complete 2FA implementation for agents and customers
  - **Agent 2FA**: Setup/disable via Settings page with QR code and recovery codes
  - **Customer 2FA**: Setup/disable via Profile page with password verification
  - **Login Flow**: 2FA prompt after password authentication, supports authenticator apps (Google Authenticator, Authy, etc.)
  - **Recovery Codes**: 8 single-use codes with 128-bit entropy for account recovery
  - **Admin Override**: Admins can disable 2FA for locked-out users (Customer Users and Users pages)
  - **Audit Trail**: All 2FA events logged (setup, disable, verify, recovery code use)
  - **Session Security**: 256-bit random tokens, 5-minute expiry, IP binding, rate limiting (5 attempts)
  - **i18n**: Full translations for all 15 supported languages
  - **Test Coverage**: 75 tests (15 unit, 35 security/contract, 2 Go E2E, 23 Playwright behavioral)
  - Security documentation: `docs/security/TOTP_THREAT_MODEL.md`
  - Files: `internal/service/totp_service.go`, `internal/auth/totp_session.go`, `internal/api/totp_handlers.go`

- **RBAC Enforcement on Statistics & Queue Endpoints**: Complete RBAC filtering across all data-leaking endpoints
  - `GET /api/v1/queues` — Only returns queues user has permission to access
  - `GET /api/v1/queues/:id/stats` — Returns 404 for inaccessible queues (prevents existence disclosure)
  - `GET /api/v1/statistics/dashboard` — Ticket counts filtered by accessible queues
  - `GET /api/v1/statistics/queues` — Queue metrics limited to permitted queues
  - `GET /api/v1/statistics/trends` — Trend data filtered by queue access
  - `GET /api/v1/statistics/agents` — Agent performance based on accessible tickets only
  - `GET /api/v1/statistics/customers` — Customer stats filtered by queue permissions
  - `GET /api/v1/statistics/export` — Export only includes permitted data
  - Dashboard HTMX (`/dashboard`) — Stats widget now uses RBAC filtering
  - Added helper functions: `extractUserIDForRBAC()`, `getAccessibleQueueIDs()`, `buildQueueFilterClause()`
  - New security test file: `internal/api/rbac_security_test.go` with attack tests verifying data isolation
  - Files: `internal/api/queue_list_handler.go`, `internal/api/queue_stats_handler.go`, `internal/api/statistics_handlers.go`, `internal/api/dashboard_htmx_handlers.go`

### Added
- **GoatKit Plugin Platform**: Complete plugin system with dual runtime support
  - **WASM Runtime** (wazero): Portable, sandboxed plugins for cross-platform distribution
    - Plugin loader with automatic discovery from `plugins/` directory
    - Production HostAPI: DB queries (multi-DB), cache, HTTP requests, email, i18n
    - Template tag: `{% use "plugin_name" %}` Pongo2 directive for plugin widgets
    - Scheduler integration for plugin cron jobs
    - Example WASM plugin (`plugins/hello-wasm/`) with routes, widgets, i18n
  - **gRPC Runtime** (HashiCorp go-plugin): Native Go plugins for I/O-heavy workloads
    - Separate process execution with gRPC communication
    - Full HostAPI access with native performance
    - Example gRPC plugin in `internal/plugin/grpc/example/`
  - Admin UI for plugin management (`/admin/plugins`) with enable/disable, logs viewer
  - Plugin state persistence via sysconfig tables (not separate state.json)
  - JWT auth for plugin API endpoints with admin-only enable/disable
  - Files: `internal/plugin/`, `internal/plugin/grpc/`, `internal/plugin/wasm/`
- **Plugin CLI Tooling**: `cmd/gk/` GoatKit CLI for plugin development
  - `gk init` scaffolding for WASM and gRPC plugins
  - `make plugin-init NAME=x RUNTIME=wasm|grpc` Makefile integration
  - Container-first model (TinyGo via Docker for WASM)
  - Templates: `grpc_build.sh.tmpl`, `grpc_main.go.tmpl`, `wasm_build.sh.tmpl`, `wasm_main.go.tmpl`
- **Plugin Documentation**: Comprehensive developer guides (2,469 lines total)
  - `docs/plugins/AUTHOR_GUIDE.md` - Plugin creation and packaging
  - `docs/plugins/HOST_API.md` - Host function reference
  - `docs/plugins/WASM_TUTORIAL.md` - Step-by-step WASM plugin tutorial
  - `docs/plugins/GRPC_TUTORIAL.md` - Step-by-step gRPC plugin tutorial
- **Plugin E2E Tests**: Enable/disable UI and API test coverage
- **API Tokens (Personal Access Tokens)**: Programmatic API access for agents AND customers
  - Token management UI at `/settings/api-tokens` (agent) and `/customer/settings/api-tokens` (customer)
  - CRUD endpoints: `POST/GET/DELETE /api/v1/tokens`, `GET /api/v1/tokens/:id`
  - Scoped permissions (e.g., `tickets:read`, `tickets:write`, `admin:*`) with RBAC inheritance
  - Configurable expiration (30d, 90d, 1yr, never)
  - Secure token format: `gf_` prefix with 32-byte random + SHA256 hash storage
  - Middleware integration: Bearer token auth alongside session cookies
  - Rate limiting per token (configurable, generous defaults)
  - Database migrations for `api_token` table (MySQL + PostgreSQL)
  - Enables MCP/AI integrations, automation scripts, CI/CD pipelines
  - 572 lines of handler tests + 350 lines of integration tests
  - Files: `internal/api/api_token_handlers.go`, `internal/service/api_token_service.go`, `internal/middleware/api_token.go`
  - Design spec: `docs/design/API_TOKENS.md`
- **OpenAPI 3.0 Documentation**: Comprehensive API specification
  - Expanded from ~2,500 to 4,845 lines covering 94 endpoints (71.2% coverage)
  - Swagger UI integration at `/swagger/` with interactive API explorer
  - Generated specs: `docs/api/swagger.json`, `docs/api/swagger.yaml`, `docs/api/docs.go`
  - All endpoints documented with request/response schemas, authentication, and examples
  - Files: `api/openapi.yaml`, `internal/api/swagger.go`
- **Structured API Error System**: Namespaced error codes for consistent error handling
  - Error registry pattern: `apierrors.Registry.Register(ErrorCode{...})`
  - Namespaced codes: `core:unauthorized`, `core:rate_limited`, `stats:export_failed`, etc.
  - Standard HTTP status mapping with customizable messages
  - Plugin-extensible: plugins can register their own error namespaces
  - Files: `internal/apierrors/codes.go`, `internal/apierrors/registry.go`, `internal/apierrors/response.go`
  - Design spec: `docs/design/API_ERRORS.md`
- **Granular RBAC Permission Service**: OTRS-compatible queue/ticket permissions
  - Permission methods: `CanReadQueue`, `CanWriteQueue`, `CanCreate`, `CanAddNote`, `CanChangePriority`, `CanBeOwner`, `CanMoveInto`
  - `rw` permission supersedes all others (OTRS behavior)
  - Integrated with all ticket/article handlers for authorization checks
  - Returns 404 (not 403) for unauthorized access (security: don't reveal ticket existence)
  - 1,305-line authorization test suite with 5 fixture agents (AgentNoteOnly, AgentCreateOnly, etc.)
  - Files: `internal/services/permission_service.go`, `internal/api/v1/authorization_test.go`
- **Statistics API**: Dashboard and reporting endpoints
  - `GET /api/v1/statistics/dashboard` - Ticket counts, trends, queue metrics
  - CSV/Excel export support
  - Chart-ready data structures for frontend visualization
  - Files: `internal/api/statistics_handlers.go`
- **Rate Limiting Middleware**: Request throttling per user/token
  - Configurable limits per endpoint
  - Token bucket algorithm with burst allowance
  - `X-RateLimit-*` response headers
  - Files: `internal/middleware/rate_limit.go`, `internal/middleware/rate_limit_test.go`
- **Behaviour Test Suite**: API contract verification tests
  - 356-line test suite validating API behaviour consistency
  - Tests response formats, error codes, pagination, filtering
  - Files: `internal/api/v1/behaviour_test.go`
- **Email Identity Handlers**: Email sender identity management
  - CRUD endpoints for email identities
  - Files: `internal/api/email_identity_handlers.go`
- **SLA Handlers**: Service Level Agreement management API
  - CRUD endpoints for SLA configuration
  - Files: `internal/api/sla_handlers.go`
- **User Delete Handler**: Soft delete for user accounts
  - Files: `internal/api/user_delete_handler.go`
- **i18n**: Added `common.year` translation key to all 15 languages
- **MCP Server Multi-User Proxy**: AI assistant integration with full RBAC enforcement
  - Multi-user proxy architecture: each API token owner's permissions apply to all operations
  - 10 fully-implemented tools: `list_tickets`, `get_ticket`, `search_tickets`, `create_ticket`, `update_ticket`, `add_article`, `list_queues`, `list_users`, `get_statistics`, `execute_sql`
  - Queue-based RBAC filtering on all ticket operations (read and write)
  - Write operations enforce granular permissions: `create`, `note`, `priority`, `owner`, `move_into`
  - Admin group gating for `execute_sql` tool (bypasses API layer, requires admin membership)
  - `IsInGroup(userID, groupName)` permission service method for group membership checks
  - Security: returns 404 (not 403) for unauthorized ticket access (don't reveal existence)
  - 750+ lines of authorization tests covering admin gating, queue access, write permissions, multi-user isolation
  - Documentation: `docs/api/MCP.md` with architecture diagram and permission model
  - Files: `internal/mcp/server.go`, `internal/mcp/authorization_test.go`, `internal/services/permission_service.go`

- **Demo Mode**: Restricted mode for public demo instances
  - `DemoMode` middleware sets `is_demo` flag on all requests
  - `DemoGuard` middleware blocks password/MFA changes for non-admin users (403)
  - Session-only preferences: language and theme stored in cookies (maxAge=0), no database writes — next visitor gets clean defaults
  - Profile page hides 2FA and password sections, shows "disabled in demo mode" message
  - Configured via `app.demo_mode: true` in config or `GOATFLOW_APP_DEMO_MODE=true` env var
  - Applied to both agent and customer portals
  - Files: `internal/middleware/demo.go`, `internal/api/preferences_handler.go`, `internal/api/customer_routes.go`
- **Coachmarks (Feature Spotlight)**: Declarative onboarding tooltip system
  - Register tips with `GoatFlow.coachmarks.register()` — auto-positioned balloons with arrow pointers
  - View tracking in localStorage with configurable max views per tip
  - Server-side dismissal persistence via `dismissed_coachmarks` preference (JSON array)
  - Theme-aware styling using CSS variables (`--gk-bg-surface`, `--gk-primary`, `--gk-glow-primary`)
  - "Reset feature highlights" checkbox on agent and customer profile pages
  - First tip: theme switcher introduction (appears after 2s, max 3 views)
  - Routes: `/api/preferences/coachmarks/dismiss` (agent and customer)
  - Files: `static/js/coachmarks.js`, `internal/api/coachmarks_handler.go`
- **Wallpaper Toggle**: Per-theme background wallpaper control
  - Checkbox in theme selector dropdown (Appearance section)
  - `html.gk-no-wallpaper` CSS class with high-specificity override
  - Cookie persistence (`goatflow_wallpaper=0/1`) plus server-side preference
  - Flash prevention: cookie read in `<head>` inline script
  - Smart disable: checkbox greys out for themes without wallpaper
  - Default ON (theme designer's intent preserved)
  - GoatFlow Classic theme: wallpaper images for light and dark modes
  - Routes: `/api/preferences/wallpaper` (agent and customer)
  - Files: `internal/api/wallpaper_handler.go`, `templates/partials/theme_selector.pongo2`

### Changed
- **Product Rebrand**: GOTRS is now **GoatFlow**
  - New repository: `github.com/goatkit/goatflow`
  - New domain: `goatflow.io`
  - All internal references updated (packages, configs, assets)
  - Part of GoatKit platform unification
- **Profile Page UX**: Save and Close behaviour
  - "Save Changes" renamed to "Save and Close" — saves preferences then redirects to dashboard
  - Cancel returns to dashboard without saving
  - Double-click protection: buttons disabled after first click until redirect completes
  - Demo mode shows "Apply and Close" (session-only changes)

### Fixed
- **Dark Theme Contrast**: 338 CSS overrides in `input.css` remapping hardcoded Tailwind utility classes (`bg-white`, `text-gray-*`, `border-gray-*`, status colours) to theme CSS variables — all dark themes benefit without template changes
- **Nineties Vibe Theme**: Synced builtin copy with latest CSS (fixed broken `--gk-text-muted: #555555` → `#888888`), added missing button contrast and reminder deck rules
- **GoatFlow Classic Theme**: Added wallpaper CSS rules (`background-repeat: repeat`, `background-size: 600px auto`, `background-attachment: fixed`)

### i18n
- New keys across all 15 languages: `theme.show_wallpaper`, `settings.reset_highlights`, `settings.highlights_reset`, `demo.security_disabled`, `common.save_close`, `common.apply_close`

## [0.6.4] - 2026-02-01

### Added
- **GoatKit Plugin Platform Documentation**: New `docs/PLUGIN_PLATFORM.md` describing the planned plugin architecture for v0.7.0
  - WASM runtime (wazero) for portable, sandboxed plugins
  - gRPC runtime (go-plugin) for native integrations
  - Host function API specification
  - Plugin packaging and lifecycle documentation

### Changed
- **Roadmap Update**: 0.7.0 now focused on GoatKit Plugin Platform
  - Dual runtime support (WASM + gRPC)
  - Statistics & Reporting ships as first WASM plugin
  - FAQ, Calendar, Process Management planned as subsequent plugins
- **Architecture Documentation**: Updated to reflect plugin platform vision
  - `docs/ARCHITECTURE.md`: Added Platform Roadmap section
  - `docs/DYNAMIC_MODULES.md`: Links to plugin platform evolution
  - `docs/MICROSERVICES_ARCHITECTURE.md`: Marked as design document for future consideration
  - `docs/VISION.md`: Aligned architecture evolution with plugin roadmap

### Fixed
- **Handler Registry Dual Registration**: `RegisterHandler()` now registers to both local `handlerRegistry` and `routing.GlobalHandlerMap`
  - Fixes "Handler not found" warnings for handlers like `HandleListServicesAPI` that used `RegisterHandler()` in their `init()` functions
  - YAML route loader looks in `GlobalHandlerMap`, so handlers must be in both registries
  - Prevents future handler wiring issues when adding new API endpoints
- **90s Theme Button Contrast**: Fixed poor text contrast on bright-colored buttons in dark mode
  - Buttons with ANSI bright backgrounds (green, yellow, cyan, red) now use dark text instead of white
  - Affects: `.gk-btn-success`, `.gk-btn-danger`, bulk action buttons, and any button with inline bright color styles
  - Improves readability across ticket actions, admin modals, and bulk operations

## [0.6.3]

### Added
- **Multi-arch Playwright E2E Tests**: E2E tests now run on both amd64 and arm64 (e.g., DGX Spark, Apple Silicon)
  - `Dockerfile.playwright-go` auto-detects architecture for Go toolchain and browser downloads
  - Playwright driver and browsers installed to shared location (`/opt/playwright-cache`) accessible by all users
  - Makefile uses `NATIVE_PLATFORM` detection for `docker build/run` commands
  - Files: `Dockerfile.playwright-go`, `Makefile`
- **Type Conversion Package**: New `internal/convert` package consolidating duplicate type conversion functions
  - `ToInt()`, `ToUint()`, `ToString()` functions with fallback values
  - Handles all numeric types (int8-64, uint8-64, float32/64) and string parsing
  - Breaks circular dependency between shared and middleware packages
  - Files: `internal/convert/convert.go`, `internal/convert/convert_test.go`

### Changed
- **Single YAML Route Loader**: Consolidated to one route loader for both production and tests
  - Tests now authenticate the same way production does (no test auth bypass)
  - `internal/routing/loader.go` is the single source of truth
  - `internal/api/yaml_router_loader.go` only used for manifest generation tooling
- **Test Database Setup**: Enhanced `resetTestDatabase()` with proper OTRS-compatible permissions
  - Creates canonical groups (users, admin, stats, support)
  - Grants user 1 'rw' permission via group_user table for queue access middleware
  - Sets queue group_id for proper queue access checks
- **Test Authentication**: All API tests now use centralized auth helpers
  - `GetTestAuthToken(t)` generates valid JWT tokens
  - `AddTestAuthCookie(req, token)` adds auth cookie to requests
  - Middleware files updated to use `convert` package instead of inline type switches

### Fixed
- **Customer User Lookup by Login or Email**: Fixed "Customer user not found" errors in ticket zoom when `customer_user_id` contains email instead of login
  - Customer user queries now match on `login = ? OR email = ?` since tickets may store either value
  - Updated 4 files: `ticket_detail_handlers.go`, `notifications/context.go`, `agent_ticket_actions.go`, `ticket_create_with_attachments.go`
- **Queue Access in Tests**: Fixed "You do not have access to any queues" errors
  - Test user now has proper group_user records with 'rw' permission
  - Queue records include group_id for permission checks
- **Test Database Connection**: Fixed "sql: database is closed" errors in attachment tests
  - Get fresh DB connection after `WithCleanDB(t)` call
- **UI Tests**: Fixed TestNavigationVisibility, TestAccessibility, TestErrorPages
  - Added proper authentication to all tests
  - Corrected route paths (`/ticket/new` not `/tickets/new`)
  - Simplified navigation test to focus on admin portal

## [0.6.2] - 2026-01-25

### Added
- **Multi-Theme System**: Pluggable theming architecture with four themes and light/dark mode support
  - **Synthwave** (default): Neon cyan/magenta color scheme with grid background and glow effects
  - **GOTRS Classic**: Professional blue theme with clean solid backgrounds, no visual patterns
  - **Seventies Vibes**: Warm retro palette with orange/brown tones and ogee wave pattern
  - **Nineties Vibe**: Dual-personality theme with distinct light and dark aesthetics
    - Light mode: Classic 90s Redmond desktop with gray windows, navy title bars, 3D beveled controls
    - Dark mode: Linux terminal aesthetic with pure black (#000000) background, ANSI bright colors, Hack Nerd Font
  - Theme switcher UI in settings and login pages with live preview
  - CSS custom properties architecture (`--gk-*` variables) for consistent theming
  - Theme-specific font loading via ThemeManager
  - Preference persistence to database for authenticated users
  - Files: `static/css/themes/*.css`, `static/js/theme-manager.js`, `templates/partials/theme_selector.pongo2`
- **Vendored Fonts**: Self-hosted web fonts for offline/air-gapped deployments
  - Inter (400, 500, 600, 700) - universal fallback
  - Space Grotesk - synthwave headings
  - Righteous - synthwave display
  - Nunito - seventies vibes body text
  - Hack Nerd Font - nineties vibe terminal mode (monospace with Nerd Font icons)
  - Archivo Black - nineties vibe light mode headings
  - Dynamic font loading based on active theme
  - Files: `static/fonts/`, `static/css/fonts-*.css`
- **Ticket Detail Page Refactoring**: Modular partial architecture for ticket zoom view
  - 17 reusable partials: header, description, notes, sidebar, tabs, meta_grid, alerts, attachments, note_form, priority_badge, status_badge, and 6 modal partials
  - Consistent theming via CSS custom properties
  - Improved maintainability and testability
  - Files: `templates/partials/ticket_detail/*.pongo2`
- **Bulk Ticket Actions**: Multi-select ticket operations on agent ticket list
  - Floating action bar appears when tickets are selected
  - Actions: bulk assign, bulk merge, bulk priority change, bulk queue transfer, bulk status change
  - Modal dialogs for each action with confirmation
  - Files: `templates/partials/agent/tickets/bulk_*.pongo2`, `internal/api/agent_ticket_bulk_actions.go`
- **Language Selector Partial**: Reusable dropdown for language selection
  - Shows all 15 supported languages with native names
  - Used in settings, profile, and login pages
  - File: `templates/partials/language_selector.pongo2`
- **Customer Password Change**: Password change functionality for customer portal
  - Accessible from customer profile page
  - Current password verification required
  - Files: `templates/pages/customer/password_form.pongo2`, `templates/pages/password_form.pongo2`
- **Ticket List Pagination**: Server-side pagination for ticket lists
  - Configurable page size
  - Page navigation controls
  - File: `templates/partials/tickets/pagination.pongo2`
- **Customer Profile Page**: Full profile management interface at `/customer/profile` for customer users
  - View and edit personal information (first name, last name, title, phone, mobile)
  - Language preference selection with all 15 supported languages
  - Session timeout preference with configurable durations (1 hour to 7 days)
  - Link to password change functionality
  - Avatar with customer initials display
  - Full i18n support for all 15 languages
  - Files: `templates/pages/customer/profile.pongo2`, `internal/api/customer_routes.go`
- **Customer Dashboard as Default Landing Page**: Customer login now redirects to `/customer` (dashboard) instead of `/customer/tickets`
  - Dashboard tiles are clickable and link to filtered ticket lists (open, closed, all)
  - Removed sysconfig override for customer landing page - now hardcoded in code
  - Files: `internal/api/auth_customer.go`, `templates/pages/customer/dashboard.pongo2`
- **Admin Ticket Attribute Relations**: Full CRUD interface at `/admin/ticket-attribute-relations` for managing ticket attribute relationships (OTRS AdminTicketAttributeRelations equivalent)
  - Define relationships between ticket attributes (Queue, State, Priority, Type, Service, SLA, Owner, Responsible, DynamicField_*)
  - CSV and Excel (.xlsx) file upload support for bulk relationship import
  - "Add missing values to dynamic field config" checkbox for auto-populating dropdown options
  - Priority-based ordering with drag-and-drop reordering
  - Red highlighting for values missing from dynamic field's PossibleValues
  - Download previously imported file
  - ACL-based filtering integrated with ticket forms via `/api/v1/ticket-attribute-relations/evaluate`
  - Full i18n support for all 15 languages
  - Files: `internal/services/ticketattributerelations/service.go`, `internal/api/admin_ticket_attribute_relations_handlers.go`, `templates/pages/admin/ticket_attribute_relations.pongo2`

### Changed
- **Separate Cookie Names for Agent/Customer Sessions**: Agent and customer portals now use distinct cookie names to allow simultaneous login in the same browser
  - Agent cookies: `access_token`, `auth_token`, `session_id`, `gotrs_logged_in`
  - Customer cookies: `customer_access_token`, `customer_auth_token`, `customer_session_id`, `gotrs_customer_logged_in`
  - Theme manager updated to detect both login indicators for preference persistence
  - Files: `internal/api/auth_customer.go`, `internal/api/auth_htmx_handlers.go`, `internal/api/handler_registry.go`, `internal/middleware/auth.go`, `internal/middleware/session.go`, `internal/routing/handlers.go`, `static/js/theme-manager.js`

### Fixed
- **Seventies Vibes Theme Background Interference**: Fixed dual-layer background pattern causing visual interference when scrolling
  - Body, sidebar, and grid pseudo-element all had the ogee wave pattern with different `background-attachment` values
  - Removed pattern from sidebar and `.gk-grid-bg::before`, keeping only body background
  - Changed `background-attachment` from `fixed` to `scroll` for natural scrolling behavior
  - Restored solid earthy brown backgrounds on cards and panels
  - File: `static/css/themes/seventies-vibes.css`
- **Guru Meditation HTMX Compatibility**: Fixed duplicate declaration errors when Guru Meditation component was loaded multiple times via HTMX
  - Added initialization guard to prevent re-declaration of functions
  - Changed local variables to window-scoped to avoid redeclaration errors
  - Functions now attached to window object for global access
  - File: `templates/components/guru_meditation.pongo2`
- **Customer-Authored Content Badge**: Fixed ticket description and notes showing "Customer sees this" badge instead of "Customer wrote this" when customer authored the content
  - Added `first_article_sender_type` field to ticket detail handler to track who wrote the initial description
  - Updated `description.pongo2` and `notes.pongo2` templates to check sender type before visibility
  - Added i18n translation key `tickets.customer_wrote_badge` to all 15 languages
  - Files: `internal/api/ticket_detail_handlers.go`, `templates/partials/ticket_detail/description.pongo2`, `templates/partials/ticket_detail/notes.pongo2`
- **Duplicate Attachment Upload in Customer Portal**: Removed duplicate file upload area from customer ticket view
  - Was showing both "Add Attachments" section and "Attach Files" in reply form
  - Now only shows attachment upload within the reply form (matching agent UI behavior)
  - File: `templates/pages/customer/ticket_view.pongo2`
- **Pending Reminder Snooze Toast Color**: Fixed snooze success showing red toast instead of green on admin/ticket-attribute-relations page
  - Page had local `showToast(type, message)` with reversed parameter order compared to global `showToast(message, type)` in common.js
  - Removed local function and updated all calls to use global signature
  - File: `templates/pages/admin/ticket_attribute_relations.pongo2`
- **Customer Initials Display**: Fixed customer initials in navigation showing only first letter instead of two letters (e.g., "E" instead of "ES" for Emma Scott)
  - Root cause: Template checked `User` before `Customer`, and session middleware set `user_name` to email (no space), causing only first letter to be extracted
  - Solution: Changed template to check `Customer.initials` first; added `is_customer` check in pongo2 renderer to skip auto-injecting incomplete `User` object for customer contexts
  - Added regression tests: unit test for template logic, integration test for `getCustomerInfo`, E2E Playwright test for header/profile initials
  - Files: `templates/layouts/base.pongo2`, `internal/template/pongo2.go`, `internal/template/pongo2_test.go`, `internal/api/customer_profile_test.go`, `tests/acceptance/customer-profile-initials.spec.js`
- **Missing i18n Translations**: Fixed untranslated strings in profile-related keys
  - `common.email` in Ukrainian: "Email" → "Ел. пошта"
  - `messages.unknown_error` in 10 languages (pt, pl, ru, zh, ja, ar, he, fa, ur, tlh) now properly translated
  - Files: `internal/i18n/translations/*.json`
- **Fix Agent Password Reset** : Fix regression in password reset feature.

## [0.6.1] - 2026-01-17

### Added
- **Pending Reminder/Auto-Close i18n**: Full internationalization for pending reminder and auto-close ticket state popups
  - Added translation keys for `tickets.pending_reminder.*` (overdue, scheduled, not_scheduled, was_scheduled_for, will_reopen_at, ago, in, no_time_scheduled, title, help)
  - Added translation keys for `tickets.auto_close.*` (overdue, scheduled, should_have_closed_at, will_close_at, while_pending, at, plus_title, plus_help, minus_title, minus_help)
  - Translations added for all 15 supported languages including RTL languages (Arabic, Hebrew, Persian, Urdu)
  - Files: `internal/i18n/translations/*.json`, `templates/pages/ticket_detail.pongo2`, `templates/pages/agent/ticket_view.pongo2`
- **Group-Based Queue Permission Enforcement**: Security-first middleware-layer enforcement of group-based queue permissions (Issue #160, OTRS-compatible)
  - **Architecture**: Permissions enforced at routing/middleware layer, not in handlers - secure by default
  - Permission types: `ro` (read-only), `rw` (full access - supersedes all), `create`, `move_into`, `note`, `owner`, `priority`
  - **Middleware Registration** (`internal/routing/handlers.go`):
    - `queue_ro`, `queue_rw`, `queue_create` - require access to at least one queue
    - `queue_access_*` - check specific queue from URL/query param
    - `ticket_access_*` - check ticket's queue from ticket ID/number in URL
  - **Route Protection** (YAML declarative): Routes declare required permissions in `middleware` list
    - `/ticket/:id` requires `ticket_access_ro`
    - `/tickets/:id/note` requires `ticket_access_note`
    - `/ticket/new` requires `queue_create`
    - Dashboard/ticket list require `queue_ro`
  - Queue Access Service (`internal/service/queue_access_service.go`): Core permission logic combining direct (group_user) and role-based (role_user → group_role) permissions
  - Context enrichment: Middleware sets `is_queue_admin` and `accessible_queue_ids` for downstream handlers
  - Ticket ID/number support: Middleware handles both numeric IDs and ticket numbers (tn field)
  - Full i18n support for queue permission messages in all 15 languages
  - Unit tests for service and middleware

### Changed
- **OTRS-Compatible Template Variable Substitution**: Unmatched template variables (`<OTRS_*>` and `<GOTRS_*>`) are now replaced with `-` instead of left unchanged
  - Matches OTRS behavior for cleaner rendered output
  - Handles both raw tags (`<GOTRS_VAR>`) and HTML-encoded tags (`&lt;GOTRS_VAR&gt;`)
  - Files: `internal/api/agent_templates_handlers.go`

### Fixed
- **Template Selector Editor Mode**: Fixed HTML templates not switching the rich text editor to HTML/richtext mode
  - Added HTML content auto-detection when `content_type` is incorrectly set to `text/plain`
  - Detects HTML tags in content and switches editor mode accordingly
  - File: `templates/partials/template_selector.pongo2`
- **Note Submission "Please enter note content" Error**: Fixed form submission failing with content validation error
  - Issue was that programmatic `setContent()` in TipTap editor didn't trigger the `onUpdate` callback
  - Added explicit manual sync to hidden textarea after setting template content
  - File: `static/js/tiptap-editor.js`
- **Template Variable Substitution for HTML-Encoded Tags**: Fixed GOTRS/OTRS variables not being substituted when stored as HTML entities
  - Template content in database had `&lt;GOTRS_*&gt;` instead of `<GOTRS_*>`
  - Now handles both raw and HTML-encoded template variable formats
  - File: `internal/api/agent_templates_handlers.go`

## [0.6.0] - 2026-01-16

### Added
- **Admin System Maintenance Module**: Full CRUD interface at `/admin/system-maintenance` for scheduling maintenance windows (OTRS AdminSystemMaintenance equivalent)
  - Schedule maintenance periods with start/stop times (epoch timestamps)
  - Display notifications to logged-in users via banner when maintenance is active or upcoming
  - Login page message display when ShowLoginMessage is enabled
  - Session management: view and kill agent/customer sessions during maintenance
  - Configurable notification timing via `maintenance.time_notify_upcoming_minutes` (default: 30 minutes)
  - Default messages configurable: `maintenance.default_notify_message`, `maintenance.default_login_message`
  - Full i18n support for all 15 languages with proper native translations
  - Date format: "from {start} until {stop}" with translated prepositions
  - Files: `internal/models/system_maintenance.go`, `internal/repository/system_maintenance_repository.go`, `internal/api/admin_system_maintenance_handlers.go`, `templates/pages/admin/system_maintenance*.pongo2`
- **Admin Session Management**: Full session management interface at `/admin/sessions` (OTRS AdminSession equivalent)
  - View all active user sessions with user details, IP address, browser info, login time, last activity
  - Kill individual sessions to force user logout
  - Kill all sessions for a specific user
  - Kill all sessions (emergency action with confirmation)
  - Current session indicator (asterisk) to avoid self-logout
  - Session enforcement in auth middleware - killed sessions immediately invalidate
  - Background session cleanup task via runner (configurable interval, default 5 minutes)
  - Cleans up sessions exceeding max age (7 days) and idle sessions (2 hours)
  - Configuration: `runner.session_cleanup.interval` in YAML config
  - Files: `internal/api/admin_session_handlers.go`, `internal/repository/session_repository.go`, `internal/service/session_service.go`, `internal/runner/tasks/session_cleanup.go`, `templates/pages/admin/sessions.pongo2`
- **Phone/Email Ticket Creation Entry Points**: Separate navigation links for creating phone and email tickets (mirrors OTRS AgentTicketPhone/AgentTicketEmail)
  - Two direct links in top navigation and agent dashboard quick actions
  - URL parameter `?type=phone|email` pre-selects interaction type on the new ticket form
  - Form displays colored left border based on interaction type (colors loaded from `article_color` database table)
  - i18n translations for all 15 languages: `tickets.new.phone_ticket`, `tickets.new.email_ticket`
  - Files: `internal/api/agent_ticket_new_handler.go`, `templates/pages/tickets/new.pongo2`, `templates/layouts/base.pongo2`, `templates/pages/agent/dashboard.pongo2`
- **Admin Article Color Module**: Dynamic module at `/admin/article-colors` for managing article sender type colors (OTRS AdminArticleColor equivalent)
  - Full CRUD for article color configuration (agent, customer, system sender colors)
  - Dashboard link with palette icon in System Administration section
  - i18n translations for all 15 languages (en, de, es, fr, pt, pl, ru, zh, ja, ar, he, fa, ur, uk, tlh)
- **Generic Agent Execution Engine**: Automated ticket processing via scheduled jobs (OTRS GenericAgent equivalent)
  - Job scheduler integration: runs jobs based on ScheduleDays/ScheduleHours/ScheduleMinutes
  - Ticket matching: StateIDs, QueueIDs, PriorityIDs, TypeIDs, LockIDs, OwnerIDs, ServiceIDs, SLAIDs, CustomerID (wildcards), Title, time-based filters (create/change/pending/escalation older/newer minutes)
  - Actions: NewStateID, NewQueueID, NewPriorityID, NewOwnerID, NewResponsibleID, NewLockID, NewTypeID, NewServiceID, NewSLAID, NewCustomerID, NewTitle, NoteBody/NoteSubject, NewPendingTime, Delete
  - Repository for OTRS key-value job storage format with schedule parsing
  - Comprehensive unit tests and end-to-end verification
  - Files: `internal/models/generic_agent_job.go`, `internal/repository/generic_agent_repository.go`, `internal/services/genericagent/service.go`
- **ACL Execution Engine**: Access Control List evaluation for filtering ticket form options (OTRS TicketACL equivalent)
  - Property matching: Properties (frontend values) and PropertiesDatabase (DB values)
  - Supports wildcards (`*`), negation (`[Not]`), and regex (`[RegExp]`)
  - Change rules: Possible (whitelist), PossibleAdd (add to options), PossibleNot (blacklist)
  - StopAfterMatch support for halting ACL chain processing
  - Filter methods for States, Queues, Priorities, Types, Services, SLAs, and Actions
  - API helper for easy integration with ticket handlers
  - Unit tests and integration tests against live database
  - Files: `internal/models/acl.go`, `internal/repository/acl_repository.go`, `internal/services/acl/service.go`, `internal/api/acl_helper.go`
- **i18n Expansion**: Extended language support from 8 to 15 languages with extensive native translations across the UI
  - Added Japanese (ja) with full native phrasing and typographic conventions
  - Added Russian (ru) with comprehensive Cyrillic translations and ₽ currency support
  - Added Ukrainian (uk) with dedicated Ukrainian vocabulary and ₴ currency support
  - Added Urdu (ur) including full RTL handling
  - Added Hebrew (he) with broad RTL coverage and localized phrasing
  - Added Chinese (zh) with extensive Simplified Chinese copy
  - Added Persian (fa) with deep RTL support and Persian numerals
  - Language configs in `rtl.go` include locale-specific date/time/number/currency formatting
  - `GetEnabledLanguages()` now auto-detects languages based on JSON file existence
- **Customer Groups Admin**: Full CRUD interface at `/admin/customer-groups` for managing customer company group permissions (OTRS AdminCustomerGroup equivalent)
  - Two-way management: edit permissions by customer or by group
  - Permission types: ro (read-only) and rw (read-write) access
  - Client-side group filtering with server-side customer search
  - Integration tests with real database
  - Files: `internal/api/admin_customer_groups_handlers.go`, templates in `templates/pages/admin/customer_group*.pongo2`
- **Customer User Groups Admin**: Full CRUD interface at `/admin/customer-user-groups` for managing individual customer user group permissions (OTRS AdminCustomerUserGroup equivalent)
  - Two-way management: edit permissions by customer user or by group
  - Permission types: ro (read-only) and rw (read-write) access for portal ticket visibility
  - Client-side group filtering with server-side customer user search (login, name, email)
  - Uses `group_customer_user` table from OTRS baseline schema
  - Comprehensive integration tests (11 test cases)
  - Files: `internal/api/admin_customer_user_groups_handlers.go`, templates in `templates/pages/admin/customer_user_group*.pongo2`
- **Queue Auto Response Admin**: Dynamic module at `/admin/queue-auto-responses` for mapping queues to auto-response templates
  - Lookup display resolution shows queue names and auto-response names instead of IDs
  - i18n translations for all 6 languages (en, de, es, fr, ar, tlh)
- **Auto Response Admin**: Dynamic module at `/admin/auto-responses` for managing automatic email response templates
  - Full CRUD with template variable support for dynamic content
  - i18n translations including template variable labels
- **Postmaster Filter Admin UI**: Full CRUD interface at `/admin/postmaster-filters` for managing database-backed email routing filters
  - Create, edit, and delete filters with match conditions (regex patterns on headers/body) and set actions (X-GOTRS-* headers)
  - Dynamic form inputs with type-ahead search for queue, priority, state, and type selections
  - Boolean dropdown for X-GOTRS-Ignore action
  - NOT operator support for negative match conditions
  - Stop flag to halt further filter processing after match
  - Navigation link added to admin dashboard under System Administration
- **HTML Structure Validation for Templates**: Centralized HTML tag balance validation in template test suite
  - `ValidateTagBalance()` function using stack-based approach with `golang.org/x/net/html` tokenizer
  - Integrated into `TemplateTestHelper.RenderAndValidate()` for automatic validation
  - All 90+ page templates now validated for missing/mismatched tags on every test run
  - Catches bugs like missing `</div>` that cause UI elements to become invisible
  - Files: `internal/template/html_validator.go`, `internal/template/html_validator_test.go`
- **Scalable Role Users Management**: Search-based user assignment for roles (handles thousands of users)
  - New API endpoint `GET /admin/roles/:id/users/search?q=xxx` with debounced typeahead
  - Replaces "load all users" pattern with search-first design (LIMIT 20 results)
  - Minimum 2 characters required to search, 300ms debounce
  - Member count display, loading spinner, auto-focus on search input
- **DBSourceFilter**: Email filter that loads postmaster filters from database and applies them to incoming mail (equivalent to OTRS `PostMaster::PreFilterModule###000-MatchDBSource`)
  - Runs first in filter chain before token extraction filters
  - Supports all X-GOTRS-* headers: Queue, QueueID, Priority, PriorityID, State, Type, Title, CustomerID, CustomerUser, Ignore
  - Comprehensive test coverage for VIP routing, spam filtering, NOT matches, multi-match conditions, and stop flag behavior
- **PostmasterFilter Repository**: Database repository for `postmaster_filter` table with YAML serialization for match/set rules
- **Dynamic Fields Import/Export**: Admin UI for importing and exporting dynamic field configurations (OTRS AdminDynamicFieldConfigurationImportExport equivalent)
  - Export: Select multiple dynamic fields and download as YAML file with complete field definitions
  - Import: Upload YAML file or paste YAML content directly, with preview before confirmation
  - Handles all dynamic field types: Text, Textarea, Checkbox, Date, DateTime, Dropdown, Multiselect
  - Routes: `/admin/dynamic-fields/export`, `/admin/dynamic-fields/import`, `/admin/dynamic-fields/import/confirm`
  - Files: `internal/api/admin_dynamic_fields_handlers.go`, `templates/pages/admin/dynamic_field_export.pongo2`, `templates/pages/admin/dynamic_field_import.pongo2`
- **Dynamic Fields Auto-Configuration**: Simplified field creation with automatic default configuration (OTRS AdminDynamicFieldAutoConfig equivalent)
  - Auto-config checkbox for supported field types: Text, TextArea, Checkbox, Date, DateTime
  - Automatically applies sensible defaults (MaxLength=200 for Text, Rows=4/Cols=60 for TextArea, YearsInPast/Future=5 for dates)
  - Hides type-specific configuration UI when auto-config is enabled
  - Auto-enabled by default for new fields of supported types
  - Dropdown and Multiselect still require manual PossibleValues configuration
  - Files: `internal/api/dynamic_field_types.go`, `templates/pages/admin/dynamic_field_form.pongo2`
- **GenericInterface Webservice Framework**: Full implementation of OTRS GenericInterface for external webservice integration
  - **Webservice Repository**: CRUD operations for `gi_webservice_config` table with YAML config serialization, history tracking, and restore functionality
  - **GenericInterface Service**: Core execution engine with transport abstraction, invoker routing, and request/response data mapping
  - **REST Transport**: Full HTTP REST support with GET/POST/PUT/DELETE, path parameter substitution (`:id`), query params, JSON body, Basic/APIKey authentication, custom headers
  - **SOAP Transport**: Full SOAP 1.1 support with envelope construction, SOAPAction header handling, namespace prefixes, fault parsing, Basic authentication
  - **WebserviceDropdown/WebserviceMultiselect Dynamic Fields**: New field types for autocomplete-based selection backed by external webservices
  - **WebserviceFieldService**: Autocomplete search, display value retrieval, result caching, multi-value support for multiselect fields
  - **OTRS-Compatible Response Format**: `StoredValue`/`DisplayValue` JSON field names matching OTRS expected format
  - **Admin UI**: Webservice management at `/admin/webservices` with create/edit/delete, connection testing, and configuration history
  - **AJAX Endpoints**: `/admin/api/dynamic-fields/:id/autocomplete` for field autocomplete, `/admin/api/dynamic-fields/:id/webservice-test` for config testing
  - **Comprehensive Integration Tests**: Mock REST and SOAP servers as fixtures, tests for transport execution, authentication, fault handling, data mapping, caching, and full service invocation
  - Files: `internal/repository/webservice_repository.go`, `internal/service/genericinterface/service.go`, `internal/service/genericinterface/transport_rest.go`, `internal/service/genericinterface/transport_soap.go`, `internal/service/genericinterface/webservice_field.go`, `internal/api/admin_webservice_handlers.go`, `internal/api/admin_dynamic_field_webservice_ajax.go`
- **Admin Queue Templates**: Full CRUD interface at `/admin/queue-templates` for managing queue↔template assignments (OTRS AdminQueueTemplates equivalent)
  - Two-column overview showing all queues and templates with assignment counts
  - Queue-side editing: assign templates to a queue at `/admin/queues/:id/templates`
  - Links to existing template-side editing at `/admin/templates/:id/queues`
  - Smart redirect back to overview after saving assignments
  - Dashboard navigation link with link icon
  - i18n translations for all 15 languages
  - Files: `internal/api/admin_queue_templates_handlers.go`, `templates/pages/admin/queue_templates.pongo2`, `templates/pages/admin/queue_templates_edit.pongo2`
- **Admin Template Attachments**: Full CRUD interface at `/admin/template-attachments` for managing template↔attachment assignments (OTRS AdminTemplateAttachment equivalent)
  - Two-column overview showing all templates and attachments with assignment counts
  - Attachment-side editing: assign templates to an attachment at `/admin/attachments/:id/templates`
  - Links to existing template-side editing at `/admin/templates/:id/attachments`
  - Smart redirect back to overview after saving assignments
  - Dashboard navigation link with file-circle-plus icon
  - i18n translations for all 15 languages
  - Files: `internal/api/admin_template_attachments_handlers.go`, `templates/pages/admin/template_attachments_overview.pongo2`, `templates/pages/admin/attachment_templates_edit.pongo2`

### Changed
- **Humanized Duration Display**: Reminder toast notifications now show overdue/due times in human-readable format (e.g., "4 months" instead of "3390h 20m") for periods exceeding 2 days
- **Translation Coverage Test Output**: `TestTranslationCompleteness` now prints a formatted ASCII table showing every enabled language and highlights the ones that are fully translated
- **Test Runner Enhancement**: `scripts/test-runner.sh` now tracks individual test counts (not just packages) and displays the i18n coverage table in the summary output
- **http-call Script**: `scripts/http-call.sh` now uses JSON API login to extract `access_token` via Bearer authentication instead of cookie-based session handling

### Fixed
- **Pending Reminder Snooze Buttons**: Fixed "response.json is not a function" error when clicking sleep/snooze buttons on pending reminder toast notifications
  - `snoozeReminder()` was using `apiFetch()` then calling `response.json()` on the result
  - But `apiFetch()` returns parsed JSON data, not a Response object
  - Fixed by using plain `fetch()` with proper credentials and Accept headers
  - File: `static/js/common.js`
- **Escalation History Recording**: Fixed "Field 'type_id' doesn't have a default value" error when recording escalation events
  - INSERT into `ticket_history` was missing required columns: `type_id`, `queue_id`, `owner_id`, `priority_id`, `state_id`
  - Fixed by fetching current ticket values before inserting history record
  - Added integration test `TestRecordEscalationEventIntegration` to prevent regression
  - File: `internal/services/escalation/check.go`
- **Customer Typeahead Race Condition**: Fixed GoatKit autocomplete initialization timing issue where the input was set up before seed data was loaded
  - Added retry logic in `setupInput()` to load seeds on-demand if not yet available
  - Added late seed loading in `refresh()` to check for seed data at query time
  - Restores customer user search, auto queue selection, and customer info panel on new ticket form
  - File: `static/js/goatkit-autocomplete.js`
- **Dynamic Module Lookup Display**: Fixed template rendering to show lookup display values (e.g., queue names) instead of raw IDs for integer foreign key fields in both `allFields` and regular `fields` template sections
- **Remaining $N Placeholder Conversion**: Fixed remaining `$%d` format-string placeholders that were missed in the v0.5.1 SQL portability refactor, causing `ConvertPlaceholders: $N placeholders are not allowed` panics
  - `internal/api/admin_attachment_handler.go` - handleAdminAttachmentUpdate
  - `internal/api/agent_templates_handlers.go` - GetTemplatesForQueue
  - `internal/api/admin_customer_company.go` - search query construction
  - `internal/api/v1/handlers_tickets.go` - handleUpdateTicket
  - `internal/repository/article_repository.go` - Create placeholder generation
  - `cmd/gotrs-storage/main.go` - storage migration query construction
- **Template Type Chips Display**: Fixed comma-separated template types (e.g., "Answer,Note,Snippet") to display as individual colored chips instead of a single unstyled text string
  - Uses pongo2 `|split:","` filter to iterate and render each type with its corresponding color
  - Applied to all template-related admin pages: queue_templates, queue_templates_edit, template_attachments_overview, attachment_templates_edit

### Internal
- **Lookup Display Tests**: Added unit tests for `processLookups`, `coerceString`, and lookup field configuration in `handler_lookup_test.go`


## [0.5.1] - 2026-01-08

### Added
- **AVIF/HEIC Thumbnail Support**: Thumbnail service now supports AVIF and HEIC image formats via govips/libvips; Dockerfile.toolbox updated with required vips packages for CGO compilation
- **Thumbnail Service Tests**: Comprehensive test coverage for `IsSupportedImageType`, `calculateThumbnailScale`, `GetPlaceholderThumbnail`, `DefaultThumbnailOptions`
- **Note Attachment Support**: Notes can now include file attachments; form uses `multipart/form-data` encoding and backend processes uploads after article creation
- **Enhanced Attachment Viewer**: Inline attachment viewer redesigned with:
  - Close button (primary color, dark mode compatible) with Esc key support
  - Collapsible metadata panel showing filename, type, size, upload date, attachment ID
  - Download button in header bar
  - Eye icon for view action (replaces ambiguous video icon)
  - Clicking attachment filename opens inline viewer by default (previously downloaded)
- **Version Display on Login**: Build version shown at bottom of agent login page; displays semantic version tag or branch name with short git commit hash in parentheses
- **Build Version Injection**: New `internal/version` package with ldflags injection; Makefile extracts git tag/branch/commit at build time and injects via `-X` flags; all build targets updated
- **SQL Portability Guard**: New `scripts/tools/check-sql.sh` script validates SQL queries for cross-database compatibility, blocking commits with PostgreSQL-specific `$N` placeholders or `ILIKE` operators
- **Helm Chart**: Production-ready Kubernetes deployment via `charts/gotrs/` with OCI registry publishing
  - Tag-mirroring: Chart `appVersion` matches git ref for GitOps workflows; `--version main` deploys `:main` images, `--version v0.5.0` deploys `:v0.5.0` images
  - Database selection: MySQL (default) or PostgreSQL via `database.type: mysql|postgresql` with custom StatefulSet templates
  - Valkey subchart: Official valkey-helm chart (BSD-3 licensed) as Redis-compatible cache dependency
  - extraResources: Arbitrary Kubernetes resources with full Helm templating support (`{{ .Release.Name }}`, `{{ .Values.* }}`, etc.)
  - Annotations and labels: Custom annotations/labels for cloud integrations (AWS IRSA, GKE Workload Identity, Prometheus scraping, Istio sidecar, AWS load balancers)
  - HPA support: Horizontal Pod Autoscaler for backend with configurable min/max replicas and CPU/memory targets
  - Ingress configuration: Flexible ingress with TLS, custom annotations, and multi-host support
  - Security contexts: All deployments include `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, `capabilities: drop: [ALL]`; database adds `runAsNonRoot`, `runAsUser: 999`, tmpfs for /tmp and /run/mysqld
  - CI integration: GitHub Actions publishes chart to `oci://ghcr.io/gotrs-io/charts/gotrs` on push to main or version tags
- **govulncheck Integration**: Go vulnerability scanning included in toolbox and security scans via `make scan-vulnerabilities`
- **Trivy Ignore File**: `.trivyignore` for configuring security scanner exclusions
- **Trivy Cache Persistence**: Trivy vulnerability database cached in `gotrs_cache` volume at `/cache/trivy`; eliminates re-download on every scan
- **Tool Cache Consolidation**: All standalone container tools now use `gotrs_cache` volume: golangci-lint (`/cache/golangci-lint`), Redocly/bun (`/cache/bun`), css-watch
- **Toolbox Entrypoint Script**: `scripts/toolbox-entrypoint.sh` for cache permission validation

### Changed
- **Attachment Click Behavior**: Clicking attachment filename now opens inline viewer instead of triggering download; download available via dedicated button
- **Attachment List Icons**: View button changed from video/play icon to eye icon for clearer UX
- **SQL Portability (MySQL/PostgreSQL)**: Comprehensive refactor of ~1,800 SQL queries across 127 files for cross-database compatibility
  - Converted all PostgreSQL-specific `$N` placeholders to portable `?` format with `database.ConvertPlaceholders()` wrapper
  - Replaced all `ILIKE` operators with `LOWER(column) LIKE LOWER(?)` for case-insensitive search portability
  - Updated repositories: ticket, article, user, queue, group, priority, state, permission, email_account, email_template, time_accounting
  - Updated API handlers: admin modules (users, groups, queues, priorities, states, types, roles, services, SLAs, customer companies/users), agent handlers, customer portal handlers
  - Updated components: dynamic field handlers, base CRUD handlers
  - All queries now use `database.GetAdapter().InsertWithReturning()` for portable INSERT operations with ID retrieval
- **Bun Package Manager Migration**: Replaced npm with bun for faster frontend builds and cleaner host filesystem
  - Dockerfile frontend stage uses `oven/bun:1.1-alpine` with Node.js for build tool compatibility
  - All `npm`/`npx` commands replaced with `bun`/`bunx` in Makefile and package.json scripts
  - Removed `package-lock.json`, now using `bun.lockb` binary lockfile
  - `make build` no longer runs frontend-build on host; Dockerfile handles CSS/JS build entirely
  - No `node_modules` directory created on host during builds
  - Bun global cache at `/cache/bun` in toolbox container
- **Go Version Single Source of Truth**: Go version centralized in `.env` as `GO_IMAGE` variable; all Dockerfiles, scripts, and Makefile targets inherit from this setting
- **Go Toolchain Upgrade**: Upgraded to Go 1.24.11 with toolchain directive in go.mod
- **Named Volume for Cache**: Changed `CACHE_USE_VOLUMES` default from `0` to `1`; development uses named Docker volume `gotrs_cache` instead of host bind mounts
- **Dockerfile.toolbox Simplified**: Removed complex entrypoint script, su-exec dependency, and USER root; container runs directly as `appuser` (UID 1000)
- **Dockerfile.playwright-go Security**: Creates and runs as non-root user `pwuser` with proper cache directory ownership
- **Production Reverse Proxy**: Replaced nginx with Caddy in `docker-compose.prod.yml`; Caddy provides automatic HTTPS via Let's Encrypt with embedded Caddyfile configuration
- **Dependency Updates**: Updated `golang.org/x/crypto`, `golang.org/x/net`, `golang.org/x/text`, `golang.org/x/sys`, and MCP SDK dependencies

### Security
- **Bun Installation Security**: Replaced insecure `curl|bash` Bun installation in Dockerfile.toolbox with GPG-verified tarball download; verifies signature against official Bun signing key before extraction
- **SDK Dependency Updates** (`sdk/go/go.mod`): Updated `github.com/go-resty/resty/v2` v2.10.0 → v2.16.5 (fixes HTTP request body disclosure), `golang.org/x/net` v0.17.0 → v0.34.0 (fixes XSS, IPv6 proxy bypass, header DoS vulnerabilities)
- **CVE-2023-36308 Mitigation**: Added panic recovery to `ThumbnailService.GenerateThumbnail` to gracefully handle crafted TIFF files that could cause server panic; no upstream patch available for `disintegration/imaging`

### Fixed
- **deleteAttachment JavaScript Error**: Added missing `deleteAttachment` function to ticket detail template; was called from HTMX-rendered attachment list but never defined
- **Note Content Field Not Found**: Fixed note form submission looking for wrong element ID (`note_content` instead of `body` used by rich text editor)
- **Note Form Null Errors**: Fixed `ensureErrorDiv` and `htmx.trigger` null reference errors by using correct element IDs and removing invalid element references
- **API Empty Response on Auth Failure**: Auth middleware now returns JSON 401 instead of HTML redirect when `Accept: application/json` header is present; `apiFetch()` helper automatically sets this header for all API calls
- **Direct fetch() API Calls**: Replaced 10 direct `fetch()` calls across 5 templates (profile, priorities, queues, dynamic_module, tickets) with `apiFetch()` to ensure proper Accept header and error handling
- **Thumbnail URL Generation**: Fixed broken thumbnail URLs in `ticket_messages_handler.go`; was generating `/api/attachments/:id/thumbnail` (non-existent route) instead of correct `/api/tickets/:id/attachments/:attachment_id/thumbnail`
- **History Recording Interface Mismatch**: Fixed `TicketRepository.AddTicketHistoryEntry` method signature to match `history.HistoryInserter` interface; changed `exec ExecContext` parameter to `exec interface{}` to enable proper type assertion in history recorder
- **SQL Argument Order Bugs**: Fixed argument order in `handleDeleteQueue` and `handleDeleteType` where `change_by` and `id` parameters were swapped
- **Missing SQL Arguments**: Fixed `insertArticle`, `insertArticleMimeData`, and `HandleRegisterWebhookAPI` missing `change_by` argument for MySQL NOT NULL columns
- **LOWER() Format String Typo**: Fixed `%LOWER(s)` typo in `base_crud.go` search query builder (should be `LOWER(%s)`)
- **Test Database Isolation**: Removed `defer db.Close()` calls from 7 test files that were closing the singleton database connection, causing "sql: database is closed" errors in subsequent tests
- **Makefile Toolbox Environment**: Added missing `TEST_DB_NAME`, `TEST_DB_USER`, `TEST_DB_PASSWORD` environment variables to 5 toolbox targets; fixed `TEST_DB_HOST`/`TEST_DB_PORT` to use `TOOLBOX_TEST_DB_HOST`/`TOOLBOX_TEST_DB_PORT` for host network mode
- **MariaDB Init Script**: Fixed `GRANT ALL PRIVILEGES ON otrs.* TO 'otrs'@'localhost'` error on fresh installs; `%` wildcard already covers localhost connections so removed redundant localhost grant
- **MariaDB Port Exposure**: Database port 3306 now exposed for host-based tools and MCP MySQL server access
- **Password Reset Modal**: Fixed JavaScript error when password reset API call fails
- **Gitignore Exception**: Added `!charts/gotrs/templates/secrets/` to prevent Helm secret templates from being ignored
- **Gitleaks Binary Allowlist**: Added `bun.lockb` to `.gitleaks.toml` allowlist; binary lockfile contains no secrets

### Removed
- **Legacy Kustomize Manifests**: Removed entire `k8s/` directory (22 files); Kubernetes deployments now use Helm chart at `charts/gotrs/`
- **Bare Metal Deployment**: Removed `docs/deployment/bare-metal.md` and all references; GOTRS supports containerized deployment only (Docker/Podman)
- **Nginx Configuration**: Removed `docker/nginx/` directory (Dockerfile, nginx.conf, error.html, entrypoint.sh); production deployments now use Caddy
- **DATABASE_URL Environment Variable**: Removed from compose files; use individual `DB_*` variables instead

### Documentation
- **Kubernetes Deployment Guide**: Rewritten for Helm chart usage with `helm install` commands, ArgoCD examples, and values customization
- **Helm Chart README**: Comprehensive documentation at `charts/gotrs/README.md` covering installation, configuration, database selection, annotations/labels, and extraResources
- **Docker Deployment Guide**: Completely rewritten with two deployment methods: Quick Deploy (curl files) and Development (full repo with make)
- **Podman Support**: Comprehensive Podman deployment instructions and notes
- **Migration Guide**: Major rewrite with accurate make targets (`migrate-analyze`, `migrate-import`, `migrate-import-force`, `migrate-validate`), migration paths table, article storage migration, and direct tool usage documentation
- **Demo Rate Limiting**: Updated from nginx to Caddyfile format
- **Schema Discovery**: Updated to reference `GO_IMAGE` environment variable

### Internal
- **Auth Middleware Tests**: Added 3 tests for `unauthorizedResponse` Accept header behavior verifying JSON vs HTML redirect based on Accept header
- **Note Attachment Test**: Added `TestTicketNoteWithAttachment` integration test with multipart form handling
- All Dockerfiles now accept `GO_IMAGE` build arg with consistent defaults
- Build targets (`make build`, `make build-cached`, etc.) pass `GO_IMAGE` and version build args to container builds
- Test and API scripts updated to use `GO_IMAGE` environment variable
- OpenAPI spec cleaned up (removed duplicate localhost:8000 server entry)
- Test suite now passes 876 tests with proper database isolation and MySQL compatibility
- SQL portability guard integrated into development workflow via check-sql.sh script

## [0.5.0] - 2026-01-03

### Added
- **CI/CD Pipeline Overhaul**: Complete rewrite of GitHub Actions workflows for containerized testing approach.
  - Security workflow: Go security scanning (gosec, govulncheck), Semgrep SAST, Hadolint for Dockerfiles, GitLeaks secret detection, license compliance checking, golangci-lint static analysis.
  - Build workflow: Single multi-stage Docker image build with GHCR publishing.
  - Test workflow: Containerized test execution via `make test`, coverage generation and upload to Codecov.
  - All workflows now use correct Dockerfile targets and container-first approach.
- **Codecov Integration**: Coverage reporting with OIDC authentication for private repositories.
- **Admin Templates Module**: Full CRUD functionality for standard response templates (OTRS AdminTemplate equivalent). Supports 8 template types (Answer, Create, Email, Forward, Note, PhoneCall, ProcessManagement, Snippet). Queue assignment UI for associating templates with specific queues. Attachment assignment UI for associating standard attachments with templates. Admin list page with search, filter by type/status, and sortable columns. Create/edit form with multi-select template type checkboxes, content type selector (HTML/Markdown). YAML import/export for template backup and migration. Agent integration with template selector in ticket reply/note modals with variable substitution (customer name, ticket number, queue, etc.). Template attachments auto-populate when template selected. 18 unit tests (type parsing, variable substitution, struct validation). Playwright E2E tests for admin UI. Self-registering handlers via init() pattern.
- **Admin Roles Module**: Full CRUD functionality for role management with database abstraction layer support. Includes role listing, create, update, soft delete, user-role assignments (add/remove users), and group permissions management. All queries use `database.ConvertPlaceholders()` for MySQL/PostgreSQL compatibility and `database.GetAdapter().InsertWithReturning()` for cross-database INSERT operations.
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
- **Admin Customer User Services**: New management page at `/admin/customer-user-services` for assigning services to individual customer users, with dual-view UI (customer→services and service→customers).
- **Service Filtering in Customer Portal**: Customer ticket creation form now filters services to only show those assigned to the logged-in customer user via `service_customer_user` table.
- **Service Field in Agent Ticket Form**: Agents can now select a Service when creating tickets, with the service_id saved to the ticket record.
- **Default Services for Customer Users**: Customer users can now have default services assigned that are automatically pre-selected when creating tickets via the customer portal.
- **Dynamic Fields Admin Module**: Full CRUD for dynamic field definitions with 7 field types (Text, TextArea, Dropdown, Multiselect, Checkbox, Date, DateTime). Screen configuration UI for enabling fields on 8 ticket screens (AgentTicketZoom, AgentTicketCreate, etc.). OTRS-compatible YAML config storage. 52+ unit tests covering validation, DB operations, and API responses. Alpine.js client-side validation with i18n support (EN/DE).

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
- **CI Workflow Failures**: Rewrote security.yml and build.yml workflows that referenced non-existent files (Dockerfile.dev, Dockerfile.frontend, web/ directory). Project is a monolithic Go+HTMX app, not separate frontend/backend.
- **golangci-lint v1.64+ Compatibility**: Updated .golangci.yml to use `issues.exclude-dirs` instead of deprecated `run.skip-dirs`, removed other deprecated options.
- **Coverage Generation in CI**: Added git safe.directory configuration and GOFLAGS for VCS stamping to fix coverage generation in containerized CI environment.
- **Customer User Typeahead JSON Escaping**: Fixed JSON parsing issues in customer user autocomplete seed data where HTML entities (e.g., `&amp;`) were causing parse errors. Added `|escapejs` filter to properly escape strings for JSON context.
- **Admin Navigation Bar**: Fixed navigation showing customer portal links on admin pages (e.g., `/admin/customer/companies/*/edit`) when `PortalConfig` was passed for portal settings tab. Added `isAdmin` flag check in `base.pongo2` to prevent `isCustomer` detection on admin pages.
- SLA admin update handler now converts PostgreSQL placeholders to MySQL (`ConvertPlaceholders`).
- SLA admin create handler properly handles NOT NULL columns by converting nil to 0.
- Admin customer company create now returns validation (400) instead of 404 for POST to `/customer/companies/new`.
- Database connectivity issues in test environments with proper network configuration for test containers.
- Auth middleware, YAML fallback guards, and legacy route middleware now respect `GOTRS_DISABLE_TEST_AUTH_BYPASS`, preventing unauthenticated access to admin surfaces during regression runs.
- SQL placeholder conversion issues for MySQL compatibility in user and group repositories.
- User title field length validation to prevent varchar(50) constraint violations.
- Admin groups overview now renders the `comments` column so descriptions entered in OTRS appear in the group list UI.
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
- Consolidated schema alignment with OTRS: added `ticket_number_counter`, surrogate primary key for `acl_sync`, `acl_ticket_attribute_relations`, `activity`, `article_color`, `permission_groups`, `translation`, `calendar_appointment_plugin`, `pm_process_preferences`, `smime_keys`, `oauth2_token_config`/`oauth2_token`, and `mention` tables via migration `000001_schema_alignment`.

### Changed
- Refactored customer user inline autocomplete logic on ticket creation form to generic GoatKit modules (removal of large inline JS block in `templates/pages/tickets/new.pongo2`).
- Display template placeholder format switched to single-brace form `{firstName}` to avoid template engine collision; template compiler now supports both `{{key}}` and `{key}`.
- Auth handlers adapted to new provider registry API.
- Ticket creation now relies on repository ticket number generator (post framework introduction).
- Dockerfile optimized for builds (layer caching / user customization notes).
- Activity stream handling cleaned (duplicate handlers removed).
- Added surrogate primary key to `acl_sync` as part of consolidated migration `000001_schema_alignment` to stay aligned with OTRS upstream schema.
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

[0.8.3]: https://github.com/goatkit/goatflow/compare/v0.8.2...v0.8.3
[0.8.2]: https://github.com/goatkit/goatflow/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/goatkit/goatflow/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/goatkit/goatflow/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/goatkit/goatflow/compare/v0.6.5...v0.7.0
[0.6.5]: https://github.com/goatkit/goatflow/compare/v0.6.4...v0.6.5
[0.6.4]: https://github.com/goatkit/goatflow/compare/v0.6.3...v0.6.4
[0.6.3]: https://github.com/goatkit/goatflow/compare/v0.6.2...v0.6.3
[0.6.2]: https://github.com/goatkit/goatflow/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/goatkit/goatflow/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/goatkit/goatflow/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/goatkit/goatflow/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/goatkit/goatflow/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/goatkit/goatflow/releases/tag/v0.4.0
[0.3.0]: https://github.com/goatkit/goatflow/releases/tag/v0.3.0
[0.2.0]: https://github.com/goatkit/goatflow/releases/tag/v0.2.0
[0.1.0]: https://github.com/goatkit/goatflow/releases/tag/v0.1.0
