# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/) and this
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.10.0] - 2026-10-04

**Upgrading from 0.9.0**
- **Everyone signs in again once.** Tokens issued before 0.10.0 carry no session id and are refused.
- **PostgreSQL: install 0.10.0 fresh.** 0.9.0 could not run its migrations on PostgreSQL ("duplicate
  migration file"), so there is no 0.9.0 PostgreSQL database to upgrade.
- **Docker Compose (`deploy/docker-compose.yml`): add two settings to `.env` first.**
  `GOATFLOW_SECURE_KEY` (64 hex characters, `openssl rand -hex 32`; never change it afterwards) and
  `SMTP_HOST` are now required, and compose refuses to start without them. `EMAIL_ENABLED=false`
  keeps outgoing mail in the queue if you have no mail server.

### Added
- **Older GoatFlow versions can install older compatible plugin releases.** A marketplace entry may
  list `versions`, each with its own `min_host_version`; entries without it still work as a single
  version. Install picks the newest version this GoatFlow can run, update picks the newest
  compatible version newer than the installed one, and `gk install name@1.2.3` or the install API's
  optional `version` pins a listed one. `gk info <name>` lists every version with its
  compatibility. The admin marketplace shows "Requires GoatFlow ≥ X" and disables Install/Update
  when no version is compatible, and offers a version picker when several are listed. The index API
  adds `compatible`, `min_host_version`, `install_version` and per-version `compatible` flags.
- **ICS calendar attachments render as event cards.** `.ics` attachments show title, time, location
  and description as a structured card in the inline viewer instead of raw text.
- **Customer portal renders markdown articles.** Articles stored as markdown are shown as sanitised
  HTML in the customer portal, using the same rule as the agent ticket view.
- **gk-lint checks SQL against the schema.** New rules `sql-unknown-table` and `sql-unknown-column`
  read `migrations/mysql` and `migrations/postgres` and reject SQL that names a table, or an
  `INSERT`/`UPDATE` column, that does not exist on both databases.
- **LDAP / Active Directory agent login works.** The `ldap` auth provider used to return "not yet
  implemented"; it now searches the directory with a read-only service account (or anonymously),
  binds as the user to check the password, and maps the entry to the GoatFlow agent with the same
  login. StartTLS/LDAPS verify the server certificate (custom CA via `LDAP_TLS_CA_FILE`), empty
  passwords are always refused, the login is escaped before it goes into the filter, and a filter
  matching more than one entry refuses the login. Optional: create agents on first login
  (`LDAP_AUTO_CREATE_USERS`, initial groups `LDAP_INITIAL_GROUPS`), sync name/email
  (`LDAP_AUTO_UPDATE_USERS`), restrict logins to `LDAP_AGENT_GROUPS`, and make the GoatFlow
  `admin` group follow `LDAP_ADMIN_GROUPS`. Users without a directory entry fall through to the
  next provider. The provider order can now be set with `AUTH_PROVIDERS=ldap,database`
  (Config.yaml `Auth::Providers` was never actually read at startup; it is now). Invalid `LDAP_*`
  settings stop the server at startup with an error per variable. The Helm chart gained
  `config.authProviders` and `config.ldap.*`. See docs/LDAP.md; `make test-ldap-integration` runs
  the OpenLDAP (testcontainers) integration tests. The `/api/v1/ldap/*` endpoints listed in the API
  docs never existed and are no longer documented; the unused LDAP HTTP handlers were deleted.
- **Admin → Reports & Analytics is a real page.** `/admin/reports` (the admin dashboard card) no
  longer shows "under construction": it shows ticket totals, a created-vs-closed trend chart
  (7/30 days or 12 months), per-queue open/backlog counts, agent activity, top customers, and CSV
  (summary, ticket list) / JSON exports for the last 24 hours, 7 or 30 days. Everything comes from
  `/api/v1/statistics/*`, so each viewer only sees tickets in queues they can read.
- **Customer portal: own company pages.** `/customer/company` shows the customer's company
  (name, customer ID, address, website, number of users) and `/customer/company/users` lists the
  company's valid customer users (name, title, email). Both are read-only and only ever show the
  logged-in customer's own valid company (`customer_user.customer_id`); agent-only company
  comments are not shown. A "Company Info" link appears in the portal navigation when the
  customer belongs to a company.
- **Forgotten-password reset for agents and customers, and customer self-registration.** The
  "Forgot password" links on `/login` and `/customer/login` now lead somewhere:
  `/forgot-password` and `/customer/forgot-password` take a username or email address, answer the
  same way whether or not it matches an account, and email a one-hour, single-use link
  (`/reset-password`, `/customer/reset-password`). Tokens are stored only as SHA-256 hashes, are
  bound to the account and the address they were sent to, and a newer request revokes older
  links; the new password goes through the agent/customer password policy, and setting it ends
  the account's open sessions. With `features.registration: true`, `/customer/register` emails a
  24-hour confirmation link; following it and choosing a password creates the customer account
  (login and customer ID = email, as OTRS's CustomerPanelCreateAccount does). Both forms are
  limited to 10 posts per IP per hour and 3 emails per recipient per hour. Switches:
  `features.lost_password` (now on by default, as in OTRS) and `features.registration` (off).
  Email links are built from `BASE_URL` (the public URL; docker-compose passes `APP_URL`, Helm
  `config.baseUrl`) and are not sent while it is unset, because the request Host header cannot be
  trusted. The agent login page's "Sign up" link, `pages/register.pongo2` and the 501 stubs
  `POST /api/auth/register` / `HandleRegisterAPI` are gone: agents are created by administrators.
- **OTRS-compatible article storage with a working `goatflow-storage` CLI.** Attachments and raw
  emails are stored either in the database (`storage.type: db`, default) or in a true OTRS
  ArticleStorageFS tree (`fs`: `<STORAGE_PATH>/var/article/<YYYY/MM/DD>/<article_id>/` with
  `.content_type`/`.content_id`/`.content_alternative`/`.disposition` sidecars and `plain.txt`), so
  a mounted OTRS/Znuny `var/article` tree is read as-is. Every upload, download, thumbnail, viewer,
  postmaster and plugin path uses the one `internal/storage` ArticleStore. `goatflow-storage
  status|migrate|verify` copies between backends (repeatable, `-delete-source` explicit), ships in
  the backend image, and needs no tracking tables. See docs/ARTICLE_STORAGE.md.
- **Passkey management and recovery codes for passkey users (agents and customers).** Profile
  lists each passkey/security key with when it was added and last used, the host name it was set
  up on, and a "Does not work on this address" warning when that host differs from the current
  one (passkeys are bound to their host name). Each key has a Remove button (password required);
  removing the last second factor turns 2FA off and deletes the recovery codes. The first passkey
  now comes with a set of recovery codes, shown once, and "New recovery codes" replaces the set
  (`POST /api/preferences/2fa/recovery-codes`, `/customer/api/preferences/2fa/recovery-codes`).
  On the login 2FA step, a passkey-only account gets **Other ways to sign in** with a
  recovery-code form, or, with no codes, a pointer to an administrator. A password-only fallback
  is deliberately not offered: it would bypass 2FA.
- **Plugin menu location `profile`: Profile → Connected accounts.** Per-user settings pages
  (connecting your own calendar or mailbox) get a home next to 2FA and API tokens instead of a
  top-nav slot, which promises a place to work in. The section only renders when an enabled
  plugin declares a `profile` item. Agent profile only; customers are unaffected.
- **Plugins receive the user's language as `_lang`.** Route calls (`buildPluginArgs`) and plugin
  UI page calls (`pluginui`) now carry the i18n middleware's resolved language code next to the
  identity keys. gRPC plugins could not localise before: their `HostAPI.Translate` callback runs
  without the request context and always resolved `en`. A plugin that ships its own translations
  (`I18nSpec`) now looks strings up locally with `_lang` (first consumer: `goatkit-calendar`).
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
  (page 1, aspect-preserved). Thumbnails are validated by an ETag of the content instead of a
  disk cache. When a PDF can't be rasterized (or `pdftoppm` is missing), the endpoint returns
  a file-type placeholder image. First consumer: GoatCoach E5 deliverable exports.
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
  wizard password, and a second boot is a no-op.
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
  `cache_sets_total`, `cache_deletes_total`, `cache_operation_duration_seconds`,
  `cache_size_bytes`) into the scrape for the first time. New process gauges:
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
- **`database.ConvertUpsert(query, conflictCols...)`.** Write an upsert the MySQL way
  (`ON DUPLICATE KEY UPDATE … VALUES(c)` or `REPLACE INTO`) and name its unique key; PostgreSQL gets
  `ON CONFLICT (cols) DO UPDATE SET … EXCLUDED.c`. `ConvertPlaceholders` also now rewrites, on
  PostgreSQL: backtick identifiers, `INSERT IGNORE` (→ `ON CONFLICT DO NOTHING`), `UUID()`,
  `FROM_UNIXTIME`, `DATE_SUB/DATE_ADD` with a `?` interval, and `SET FOREIGN_KEY_CHECKS`.
- **SQL portability lint (`cmd/gk-lint`, `make lint-platform`, pre-commit).** Type-checks the whole
  module, tests and build-tagged files included, and fails on SQL that reaches `database/sql`
  without the conversion layer (`sql-unconverted`), `LastInsertId()` (`sql-last-insert-id`),
  MySQL-only SQL (`sql-mysql-only`) and PostgreSQL-only SQL (`sql-postgres-only`: `RETURNING`
  outside `InsertWithReturning`, `ON CONFLICT`, `::`, `||`, `INTERVAL '…'`, full-text functions).
  Reviewed exceptions carry `// sql-converted: <reason>`. See
  `docs/development/DATABASE_ACCESS_PATTERNS.md`.
- **Customer company pages.** `/admin/customer/companies/:id/users`, `/tickets` and `/services`
  were "Under Construction" placeholders; they are now real pages, linked from the company list
  and the company edit form. Users lists the company's customer users with ticket counts and
  status. Tickets lists `ticket.customer_id` tickets newest first (state, queue, priority, age,
  link to the ticket, 50 per page), limited to queues the admin can read unless they are in the
  `admin` group. Services is a matrix of valid services × the company's customer users stored in
  `service_customer_user` (OTRS has no company-level service table); assignments to invalid
  services are kept. The company edit form's Services tab, which ticked a service if any user had
  it and then overwrote every user's assignments, the JSON modals on the company list and the
  unused `PUT /admin/customer/companies/:id/services` route are gone.

### Fixed
- **Images record their commit, branch and build date.** The Dockerfile's `-X` flags pointed at a
  package path that no longer exists, so all three said `unknown`. The version itself now comes only
  from `internal/platform/version/version.go` (set by `make prepare-release`); the build no longer
  overrides it with a branch name, which would have turned the plugin host-version check off on
  `main` and `dev` images. A tag that does not match the constant now fails the release build.
- **Plugins that need a newer GoatFlow are refused at load.** `GKRegistration.MinHostVersion` was
  never checked; a gRPC or WASM plugin declaring a newer minimum is now refused before `Init` at
  boot, hot reload, upload and marketplace install, shut down, and the reason is written to the
  plugin log. A refused hot reload leaves the running version in place.
- **A failed marketplace update no longer deletes the installed plugin.** Update removed the
  plugin directory before downloading, so a failed download or signature check left nothing
  installed. The new version is now downloaded, verified and staged first, then swapped in, and the
  old version is restored if the swap fails.
- **Marketplace update ignored `min_host_version`.** Only install checked it, so update could
  install a release the running GoatFlow cannot run. Update and the update check now offer only
  compatible versions.
- **gRPC plugins were told they run on GoatFlow 0.6.4.** `host_version` in the plugin `Init` config
  was hardcoded; it is now the running GoatFlow version.
- **Demo mode crashed on short `Accept` headers and could redirect a page to itself.** On a demo
  instance, a non-admin request to a guarded page (password change, MFA setup) with an `Accept`
  header shorter than 16 characters, such as curl's `*/*`, panicked and returned 500, and a blocked
  page opened without a `Referer` redirected to itself. Browsers now go back to the page they came
  from, or to `/`.
- **Docker Compose deployments could not turn on demo mode.** `deploy/docker-compose.yml` did not
  pass `GOATFLOW_APP_DEMO_MODE` to the containers, so a demo instance ran without the demo guard. It
  is passed now (default `false`).
- **The public demo deploy.** The release job (`build.yml`, `deploy-demo`) kept the demo server's
  compose file from an older release, pulled `:latest`, ran without `GOATFLOW_SECURE_KEY`, and
  loaded the seed dump over the running database. Tables created by newer migrations were left
  next to the seed's older `schema_migrations` version, so the next migration run failed
  (`Duplicate column name 'captive_plugin'`), and the dump's `mysql` database replaced the database
  users. The job now installs `deploy/docker-compose.yml` and `deploy/demo/reset-demo-db.sh` from
  the tag, pins `GOATFLOW_TAG` to it, generates the secure key once on the server and keeps it,
  and checks the configuration before stopping anything. `reset-demo-db.sh`, which also runs
  nightly, drops each seeded database before loading it, skips the `mysql` database, and restarts
  the GoatFlow services so their migrations bring the seed up to the running release.
- **Compose stacks migrate the database once, under the migration lock.** The backend service ran
  `migrate up` in its start command before GoatFlow itself, outside the lock that the backend,
  runner and customer frontend share; a runner starting at the same time saw that in-progress
  migration as a failed one. The step is gone: GoatFlow runs the migrations at startup, under the
  lock.
- **A failed migration no longer gets marked as applied.** When `schema_migrations` was left dirty
  (a migration failed or was interrupted), every GoatFlow process forced that version as done at
  startup and went on migrating, so the failed migration was skipped for good and an upgrade could
  come up with a half-migrated schema without saying so. GoatFlow now stops migrating, logs which
  version is dirty and how to recover (`migrate force <version>` once it is fully applied, or the
  previous version to run it again), and leaves the flag for an operator.
- **The container image builds from a clean checkout.** The WASM plugin build stage created
  `/plugins` as root and then could not create its `tmp` directory as the tinygo user, so `docker
  build` failed on any checkout without a leftover `plugins/tmp`.
- **Upgrades install the new versions of the bundled plugins.** Docker Compose (and the TrueNAS
  app) keep `config/plugins` on a volume, so after an image upgrade the bundled WASM plugins stayed
  at the versions of the release that first filled the volume; a 0.9.0 install upgraded to 0.10.0
  kept running the old `stats` plugin without its queue-permission fixes. The image now carries a
  pristine copy in `/app/bundled-plugins`, and at startup GoatFlow installs each bundled file that
  is missing or still an untouched bundled copy (tracked in `config/plugins/.bundled-manifest.json`;
  for directories filled by 0.9.x and older, by the hashes those releases shipped). A bundled file
  an admin changed or replaced is kept and logged as a warning; delete it to get the bundled
  version back. The sync never writes outside `config/plugins`: a plugin directory there that is a
  symlink to somewhere else stops the sync with a warning instead of being written through.
- **Helm: uninstalling and reinstalling the chart broke the installation.** The database and
  storage volumes survived `helm uninstall` but the generated Secrets did not, so a reinstall made a
  new database password (GoatFlow could no longer log in to its own database) and a new
  `GOATFLOW_SECURE_KEY` (stored webhook and identity-provider secrets could no longer be decrypted).
  The generated Secrets are now kept on uninstall like the volumes. The chart also refuses
  `runner.replicaCount` above 1, because a second runner would send queued email twice.
- **API requests no longer write the session's last-request time on every call.** Every
  JWT-authenticated request checks that its session still exists, and that check also updated the
  session row each time, adding an UPDATE and row-lock contention on busy tokens. The existence
  check still runs on every request, so killing a session revokes its tokens at once; the
  last-request time is now refreshed at most once a minute, well inside the minimum five-minute
  idle timeout.
- **`make test-unit` could pass after running almost nothing.** If any package failed to load (for
  example a leftover `tmp/go-build*` folder), the package list came back empty and the script
  skipped every phase but the template tests, then exited 0. The script now skips git-ignored
  directories, keeps packages that fail to load so `go test` reports them, runs every phase, and
  exits 1 naming the failed phases.
- **SP-initiated SAML login now works.** The ACS rejected every response because the ID of the
  AuthnRequest GoatFlow sent was never kept, so the response's `InResponseTo` could not match it.
  The request ID is now stored with the login's RelayState and browser binding. The ACS accepts only
  the response to the request issued to that browser, and refuses replayed responses, responses to
  other or unknown requests, and responses posted from another browser. Unsolicited (IdP-initiated)
  responses are still refused. Providers with no SP entity ID now use the SP metadata URL
  (`/auth/:id/metadata`) at login, at the ACS and in the metadata; before, login and ACS used a
  different ID from the one the metadata advertised, so IdPs set up from that metadata refused the
  login.
- **Identity provider secrets are encrypted at rest.** The OIDC client secret and the SAML SP
  private key were stored in plain text, and the edit form sent both back to the browser. They are
  now encrypted with `GOATFLOW_SECURE_KEY` (as webhook secrets are) and never rendered. Existing
  plain-text values keep working and are encrypted the first time the provider is used. If
  `GOATFLOW_SECURE_KEY` is not set they stay unencrypted (with a warning), because a per-process
  key would lock everyone out of SSO after a restart.
- **New OIDC providers lost their client secret.** Creating a provider did not save the client
  secret, so the first SSO login failed until the provider was edited and the secret entered again.
- **Killed sessions and logouts did not revoke the token.** Deleting a session (Admin → Sessions, logout)
  only took effect on UI pages, and only when the browser sent the `session_id` cookie; every `/api/v1` route
  kept accepting the token until it expired. Access and refresh tokens now carry the session id (`sid`) and
  are refused on every route (API, plugin, customer) once the session is gone; refresh stops too.
  **Tokens issued before this release are no longer accepted: everyone signs in again once after upgrading.**
- **SSO and API logins had no session.** OIDC, SAML and `POST /api/v1/auth/login` logins now create a session,
  so they appear under Admin → Sessions and can be ended there.
- **SSO login CSRF.** OIDC `state` and SAML `RelayState` are now bound to the browser that started the login
  by a short-lived HttpOnly cookie; a callback completed in another browser is refused.
- **Identity-provider URLs reached internal addresses.** OIDC discovery and SAML metadata URLs are checked
  with the webhook address guard when saved and fetched through it (DNS pinning, no redirects, no proxy).
  Loopback, private and cloud-metadata addresses are refused unless `GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS=true`.
- **Plugin org scoping could be bypassed by the plugin's own SQL.** A query mentioning `org_id`, an
  `... OR 1=1` predicate, a JOIN or subquery reaching an org-aware table, or an INSERT for another
  organisation escaped the scoping. Every org-aware table reference is now scoped with the plugin's predicate
  in parentheses, UPDATE may not change `org_id`, INSERT must target the caller's organisation, and shapes
  that cannot be scoped are refused.
- **Stored XSS in the ticket view.** The first article of a ticket (the "description" card) was rendered
  with `|safe` without sanitising it, although customers and inbound e-mail write it. It now goes through the
  same HTML sanitiser as every other article; the customer ticket view sanitises any body that carries a tag.
- **`POST /api/v1/auth/login` ignored the second factor.** A password alone returned access and refresh
  tokens for accounts that had enrolled TOTP or a passkey. It now answers `403 mfa_required`; API clients
  should use API tokens.
- **Plugin sandbox DB table scope.** The SQL lexer missed parenthesised table references (`FROM (users)`),
  `TABLE t`, `HANDLER`, `DESCRIBE`, `COPY`, `CALL`/`PREPARE`/`EXECUTE`, `LOAD DATA` and vertical-tab whitespace,
  so a plugin scoped to its own tables could read any table. All of these are now named or fail closed.
- **Plugin sandbox HTTP scope.** `https://evil.com?x=.vendor.com` matched a `*.vendor.com` scope because the
  host was cut at the first `/`. The scope is now checked against the parsed URL host.
- **Webhook deliveries cut short by the end of a dispatch run.** When the run ended while an endpoint still had
  time to answer, the delivery was recorded as failed with "no response within N seconds" and used up a retry
  (or failed for good with no retries left). It now goes back to pending, due at once, with its attempt count
  unchanged and the message "run ended before the endpoint answered; will try again".
- **`make test` / `make test-unit` flaky webhook test.** The test stack's runner (`goatflow-runner-test`)
  shares the unit tests' database, and every 10 seconds its webhook dispatch claimed the deliveries a test
  had just queued (`TestService_SlowEndpointDoesNotStarveOthers` failed about 1 run in 4). `test-unit` and
  `test-fast` now stop the runner for the run and start it again afterwards.
- **`CUSTOMER_FE_ONLY` is read the same way everywhere.** `true`, `1`, `yes` and `on` (any case) now
  enable it in every place; before, `1` was ignored by the login page redirect.
- **`DB_DRIVER=pgsql` and `postgresql` work fully.** All PostgreSQL driver names now switch SQL
  generation and the database connection to PostgreSQL, not only the connection settings.
- **Plugin deletions record the acting agent.** Entity soft delete, restore, hard delete and secure
  config writes were recorded as user 1. They now record the agent the plugin call runs for
  (`_user_id`), and the system user only when there is none (jobs, customers, init).
- **gRPC CacheDelete, ConfigGet and Translate work.** CacheDelete always failed with an
  unknown-method error; ConfigGet always failed and Translate always returned the key because the
  client misread the host's answer.
- **WASM plugins support the full HostAPI.** cache_delete, plugin files, thumbnails, articles and
  attachments, ticket states and views and PDF rendering now work from WASM, through the same
  dispatcher as gRPC.
- **Plugin calls run in the caller's organisation and language**: API routes, widgets, plugin UI
  pages and their nav badges, setup tasks, MCP tools and event authorizers now pass the caller's
  active organisation (`_org_id`) and language (`_lang`) into the call context, in both runtimes. As
  a result `OrgID` returns the organisation, secure config and plugin files are kept per
  organisation, and `Translate` uses the caller's language. Before, plugin UI pages always
  translated to English, and plugin calls always ran without an organisation.
- **Plugin files kept per organisation**: plugin file storage read the organisation from a context
  key that nothing set, so all organisations shared one file namespace. Files are now stored under
  `<plugin>/org-<id>/` when the call has an organisation.
- **Organisation scoping only on tables that have `org_id`**: the sandbox's org-aware table list
  named `ticket`, `queue`, `customer_user` and `gk_custom_field_value`, none of which has an
  `org_id` column. Scoping them would have made every plugin query on them fail. Now only tables
  whose rows each belong to one organisation are scoped: `gk_org_plugin_access`,
  `gk_user_organisation`, `sysconfig_org` and `gk_identity_provider_org`. Core tickets, queues and
  customers are not separated by organisation. The docs no longer claim they are.
- **Plugin articles on PostgreSQL**: the plugin host call that adds an article to a ticket
  (`CreateArticle`) failed on PostgreSQL with a SQL syntax error. It now works on both MySQL and
  PostgreSQL.
- **A slow webhook endpoint no longer holds up the others.** The runner sends to up to 8 webhooks at
  once, one delivery at a time per webhook. Deliveries still due when a run ends stay pending
  instead of being stuck as "delivering" for 10 minutes, and an attempt that used up the run's time
  is still recorded.
- **Statistics trends report real open counts.** `trends[].open` is now the number of tickets still
  unresolved at the end of each day or month, including older tickets that are still open. It used
  to be a running sum of created minus closed inside the window.
- **Admin statistics count only valid queues.** Admin totals included tickets in invalid queues that
  no per-queue list showed. Now every caller counts valid queues only, so totals match the queue
  lists.
- **`/api/v1/statistics/customers` limits `top`.** `top` must be between 1 and 100; other values
  return 400 instead of returning every customer.
- **Statistics API documentation matches the real parameters** for trends, agents, analytics,
  customers and export.
- Soft-deleting an entity that doesn't exist returns an error. Before, it left a phantom recycle-bin
  entry. Ticket and agent hard deletes run in one transaction.
- Dynamic-module and generic CRUD create/update endpoints answer 400 on malformed JSON or
  non-numeric values in numeric fields. Before, they stored empty or zero values.
- An owner change on a ticket that doesn't exist answers 404 and a non-positive ticket id answers
  400. Generic-agent job updates roll back when replacing a key fails.
- Plugin uploads fail cleanly when the file can't be fully written. Plugin file storage reports
  metadata write failures.
- The 2FA audit log shows the numeric user id; it used to show a garbled character. Legacy
  custom-field migration keeps non-ASCII characters in field names. Kubernetes sidecar probe
  intervals such as `1m30s` are parsed correctly.
- SAML IdP metadata fetch times out after 30s. goatflow-migrate escapes quote characters in source
  identifiers.
- Many errors that were ignored before are now returned or logged: cache invalidation, OIDC/SAML
  group sync, lambda setup, route and plugin watchers, CLI flush/mkdir/chmod, and `goatflow-config
  watch`.
- **Helm chart works out of the box**: new runner Deployment (`runner.*`, image
  `ghcr.io/goatkit/goatflow-runner`) so outgoing email, webhooks, notification rules and session
  cleanup run on Kubernetes. The chart now sets the variables GoatFlow reads (`DB_DRIVER` +
  `DB_MYSQL_*`/`DB_PGSQL_*`, `JWT_SECRET`, `GOATFLOW_SECURE_KEY`, `GOATFLOW_VALKEY_*`,
  `GOATFLOW_EMAIL_*`, `STORAGE_*`, `LOG_LEVEL`, session lifetime) instead of `DB_TYPE`,
  `APP_SECRET`, `REDIS_*`, `SERVER_PORT` and `SESSION_TIMEOUT`. PostgreSQL installs now connect.
- **Helm: Ingress reaches the app**: the Ingress now sends every path to the backend; the nginx
  frontend that only proxied `/api/` and `/ws` is removed.
- **Helm: MariaDB becomes ready**: probes use `mariadb-admin` (the `mariadb:11` image has no
  `mysqladmin`); backend and runner wait for the database before starting.
- **Helm: Valkey**: the backend now finds the subchart Service (`<release>-valkey`); auth is off by
  default, and with auth on the password comes from the subchart's `default` user Secret (before: a
  Secret that never existed, `CreateContainerConfigError`).
- **Helm: first login**: the chart sets a first-boot admin password (`secrets.adminPassword`,
  generated when empty) for `root@localhost`.
- **Helm: metrics**: `metrics.enabled` turns on the unauthenticated listener and a
  `<fullname>-metrics` Service.
- **Compose: unread settings removed or renamed**: `VALKEY_HOST/PORT` → `GOATFLOW_VALKEY_HOST/PORT`;
  `TLS_CERT_FILE/TLS_KEY_FILE` (and the self-signed certificate generation), `MAX_UPLOAD_SIZE`,
  `ALLOWED_FILE_TYPES`, `ENABLE_YAML_ROUTING` removed.
- **Dev compose**: backend `BASE_URL` defaults to the published port 8081, customer-fe `BASE_URL`
  follows `APP_URL`, backend passes `GOATFLOW_ADMIN_PASSWORD` and the SMTP settings, `backend-test`
  health check tests `/health` instead of the runner command line.
- **Reference deployment**: runner, app and customer portal get the SMTP settings from `.env`
  (before the runner used `mailhog:1025` and no mail left the server); `BASE_URL` defaults to
  `https://$DOMAIN`; `GOATFLOW_ADMIN_PASSWORD` added; Caddy `/c/<path>` now redirects to
  `/customer/<path>` instead of `/customer/c/<path>` (also in `docker-compose.prod.yml`).
- **TrueNAS app**: new Public URL question (`BASE_URL`); `GOATFLOW_VALKEY_*` and `DB_MYSQL_*` names;
  SMTP settings for the backend too; unread variables removed.
- **Graceful stop**: 45 s stop grace period for backend, customer portal and runner in compose,
  TrueNAS and Helm (`terminationGracePeriodSeconds`).
- **Helm: writable, persistent file storage**: the backend mounts a `<fullname>-storage` PVC at
  `config.storage.path` (`config.storage.persistence.*`: size, storage class, access modes,
  `existingClaim`). Plugin files (`<path>/plugins`) and `fs` attachments (`<path>/var/article`) used
  to fail on the read-only root filesystem. The PVC is kept on `helm uninstall`. With persistence
  off the chart mounts an emptyDir and refuses `config.storage.type: fs`. Several backend replicas
  on several nodes need a ReadWriteMany class.
- **Mail settings for every sending process**: in `docker-compose.yml` the customer portal,
  `backend-test` and `customer-fe-test` now get the same `GOATFLOW_EMAIL_*` settings as the backend
  and runner (one shared block). Customer-side direct sends used to go to the built-in `mailhog`
  default. `docker-compose.draft.yml` now gives its backend the runner's SMTP settings too.
- **`make synthesize` mail settings reach the dev mail sandbox**: it writes `SMTP_HOST=smtp4dev`,
  `SMTP_PORT=25` and `EMAIL_FROM`, which the compose files pass on. Before it wrote `mailhog:1025`
  (no such service) and `SMTP_FROM_*` names nothing read. `APP_URL` defaults to
  `http://localhost:8081`, the backend's published port (also in `.env.example`).
- **No false route-audit warning at startup**: the backend no longer logs "missing expected routes:
  [/api/v1/states /api/lookups/statuses /api/lookups/queues]" on a correct build. The audit looked
  only at the main router, but these are YAML routes on the dynamic engine; it now checks both.
- **Air-gapped bundle has every image the chart needs**: the release bundle now also contains the
  chart's `busybox` (wait-for-database), `mariadb`, `postgres` and `valkey` images, read from the
  packaged chart. `load-images.sh` loads all of them, and the bundle README shows how to push them
  to a local registry and point the chart at it. The release notes no longer call the runner image
  the "Temporal runner".
- **Docs**: ROADMAP no longer links to the removed "Known limitations (0.10.0 chart)" section;
  `packaging-audit.md` lists the environment variables GoatFlow actually reads (`GOATFLOW_VALKEY_*`,
  `STORAGE_TYPE`, attachment limits from the config) instead of `VALKEY_*`, `MAX_UPLOAD_SIZE`,
  `ALLOWED_FILE_TYPES`, `ENABLE_YAML_ROUTING` and `TLS_*`; the admin guide no longer says the chart
  has no runner.
- **Scheduled jobs run once across replicas.** Email polling, GenericAgent, escalation checks,
  reminders, auto-close and plugin jobs ran on every backend replica. Each run is now claimed in the
  new `gk_scheduler_job_lock` table (migration 000031), so one replica runs each tick.
- **Startup migrations no longer race.** Every goats process runs migrations at start. A second
  process could see the first one's in-progress (dirty) migration, force it and leave the schema
  dirty. Migrations now run under a database lock (`GET_LOCK` on MariaDB, an advisory lock on
  PostgreSQL); the other processes wait and then find nothing to do.
- **Runner stops on SIGTERM.** The runner kept running tasks for up to 5 minutes after SIGTERM. It
  now cancels running tasks and waits at most `DRAIN_TIMEOUT` for them.
- **Shutdown fits the stop grace period.** `DRAIN_TIMEOUT` now defaults to `5s` (was `10s`, the
  whole Docker grace period). The scheduler stops while HTTP drains, and the metrics listener drains
  at the same time instead of after plugin shutdown. The compose files and Helm chart give 45
  seconds.
- **Invalid `DRAIN_TIMEOUT` is reported.** A value that is not a positive Go duration (for example
  `30`) was ignored silently. It is now logged as an error at startup and the default is used.
- **Log fallback goes to stdout.** When the `LOG_OUTPUT` file cannot be opened, logs now go to
  stdout as documented (they went to stderr), and the reason is printed to stderr.
- **One JSON log layout.** In `LOG_FORMAT=json`, old-style `log` lines used `"level":"info"` while
  `slog` lines used `"INFO"`/`"WARN"`. Both now use the `slog` layout with upper-case levels.
- **Runner logs follow `LOG_FORMAT` and `LOG_OUTPUT`.** Runner lines were plain `[RUNNER]` text on
  stdout. They are now `slog` lines with `component=runner`.
- **Zinc and Elasticsearch search work.** With `SEARCH_BACKEND=zinc` or `elasticsearch`, GoatFlow
  searched an index nothing wrote to. The runner now builds the index on first start and keeps it in
  sync every 30 seconds: new and changed tickets, articles (including edits), queue moves and other
  history changes, and customer users. Documents of deleted rows are removed.
- **Reindex works for Zinc and Elasticsearch.** `POST /api/v1/search/reindex` used to answer 501. It
  now starts a full rebuild in the background (202, or 409 while one runs); progress and errors show
  in `GET /api/v1/search/health`.
- **External search honours queue permissions.** Agents who may not read every queue got 503 from
  Zinc/Elasticsearch search. Ticket and article hits are now limited to readable queues, like the
  database backend, and every hit is re-checked against the database, so a moved or deleted ticket
  never shows up even before the index catches up.
- **Search outages are errors, not empty results.** An unreachable service, wrong credentials or an
  index that is not built yet answer 503 with the reason and the backend name. An unknown
  `SEARCH_BACKEND` or a missing endpoint is reported as a configuration error instead of silently
  using another backend.
- **SLA escalation works.** Ticket escalation times (first response, update, solution) are now
  computed the OTRS way from the ticket's SLA, or its queue, in working time, and stored in
  `ticket.escalation_*`. A new scheduler job (`escalation-index`, every 15 seconds) rebuilds every
  ticket changed since its last run, and all tickets after an SLA, queue or calendar setting
  changes. Before, nothing filled these columns, so no ticket ever escalated.
- **Escalation events are raised.** The escalation check now writes `Escalation*TimeStart` and
  `Escalation*TimeNotifyBefore` history events, repeated at most every
  `OTRSEscalationEvents::DecayTime` minutes (default 1440, as in OTRS; it used to repeat every
  minute). A reply or state change that ends a started escalation writes `Escalation*TimeStop`.
  Webhooks and ticket notifications act on these events.
- **Business calendars follow OTRS.** Working hours are read per weekday (non-contiguous hours and
  days missing from the setting work), with recurring and one-time vacation days, numbered calendars
  1–9 and their `TimeZone::CalendarN`.
- **Ticket notifications are sent.** Rules in Admin -> Ticket Notifications were saved but never
  evaluated. The runner now checks new tickets, articles and ticket history every 10 seconds. For
  each valid rule subscribed to the event and matching its filters, it renders the subject and body
  in the recipient's language, with OTRS tags such as `<OTRS_TICKET_Title>` and
  `<OTRS_CUSTOMER_BODY[5]>`, and queues one email per recipient on `mail_queue`. Each event is
  evaluated once, even with several runners. Every email adds a `SendAgentNotification` or
  `SendCustomerNotification` ticket history entry, as in OTRS.
- **Notification recipients follow OTRS rules.** Agents must be valid, have an email address and
  read access to the ticket's queue. The agent who caused the event is not notified.
  `SkipRecipients`, `OncePerDay` and `Transports` from OTRS imports are honoured. A rule with a
  filter GoatFlow cannot apply (for example a dynamic field) is skipped and logged instead of
  notifying everyone.
- **Editing a ticket notification no longer breaks or loses data.** The edit form printed the stored
  messages as a Go map, which stopped the form's script. Messages in languages other than en, de,
  es, fr and ar were dropped on save. Filters, events and recipients the form has no control for
  (OTRS imports) were deleted on save. All of these are now kept.
- **Escalation notifications fire once per check.** `NotificationEscalation` and
  `NotificationEscalationNotifyBefore` rules send one email per ticket and escalation check, not one
  per escalation type.
- **Per-company portal settings now apply.** The overrides on a company's Portal Settings tab
  (Enable, Require Login, Title, Footer Text, Landing Page) were stored but the portal only read the
  global values. A signed-in customer now gets their company's settings, with the global value for
  every field the company does not override. Visitors who are not signed in get the global settings.
  A company with Enable off is refused while other companies keep the portal, and a company with
  Enable on keeps the portal while it is off globally.
- **The portal Landing Page setting is used.** `CustomerPortal::LandingPage` was stored but never
  read, and its built-in default (`/customer`) did not match the seeded value (`/customer/tickets`).
  Customers now go to their company's landing page after signing in with a password, an
  authenticator code or a passkey, and when a signed-in customer opens `/customer/login`. On a
  customer-only instance without `ROOT_REDIRECT_PATH`, `/` redirects there too. The one default is
  `/customer/tickets`. Values that are not local paths are ignored.
- **Duplicate customer company shows a form error.** Creating a customer company with a customer ID
  or name that is already used, or without a name, answered the browser form with raw JSON. The New
  Company form now comes back with the error and the values you entered. API callers still get JSON.
  The Status chosen on the New Company form is now saved (it was always Active).
- **Customer portal toggle label.** The help text under "Enable" on the Customer Portal settings
  page said "Enable Help Link". It now describes the portal switch, and the page is translated in
  all 15 languages.
- **Broken Dashboard link in the portal.** The breadcrumb on the customer New Ticket and ticket
  pages pointed at `/customer/dashboard`, which does not exist; it now points at `/customer`.
- **`api/openapi.yaml` matches the API again.** The REST API v1 spec was rewritten from the
  handlers: every documented path, method, parameter, request body, status code and JSON field now
  comes from the code, and every operation states who may call it. Errors are documented in both
  shapes the server sends (`{"error":"..."}` from handlers, `{"error":{"code","message"}}` from auth
  and scope checks). Unused schemas, invented enums and the made-up `/health` response are gone.
  `make openapi-lint` and `make openapi-bundle` now pass with 0 errors and 0 warnings; the Redocly
  CLI version is pinned (2.57.0) and the targets use the same `oven/bun:1.3-alpine` image as the
  Dockerfile.
- **Generated route docs listed `/admin/admin/identity-providers`.** `make generate-route-docs` and
  `make api-docs` joined the group prefix and the route path naively, while the server does not
  double a prefix the path already contains. The generator now uses the server's own path rule
  (`routing.FullRoutePath`), skips disabled route groups, reads per-method `handlers:` maps, lists
  route middleware, and writes the Markdown with a text template (no more `&#39;` in `api.md`). The
  identity provider routes in `routes/admin.yaml` are now written relative to the `/admin` prefix.
- **Swagger UI advertised endpoints that do not exist.** Swag annotations produced
  `/api/v1/api/v1/i18n/...` and `/api/v1/api/v1/admin/sql`, and listed unrouted `POST
  /api/v1/tickets/{id}/close`, `.../assign`, `POST /api/v1/auth/logout`, plus MCP and
  `/api/queues/...` routes under the wrong base path. The annotations are fixed or removed and
  `docs/api/swagger.*` regenerated.
- **`make synthesize` no longer breaks migrations.** The generated test data used to be written to
  `migrations/postgres/000004_generated_test_data.up.sql`, a duplicate migration version. It now
  goes to `schema/seed/generated_test_data.postgres.sql` (gitignored, owner-only). The SQL now loads
  on the migrated schema, can be re-run, and the listed logins work.
- **The `synthesize` targets run the right binary.** `make synthesize`, `rotate-secrets`,
  `synthesize-force`, `gen-test-data` and `synthesize-credentials` called the server binary
  (`goats`), which started a server. They now call `goatflow`.
- **Dev database targets follow `DB_DRIVER`.** `db-status`, `db-rollback`, `db-force`, `db-migrate`
  and `db-shell` work on MariaDB and PostgreSQL. `db-init` (and `db-reset`) recreates the dev
  database from all migrations on the selected driver.
- **PostgreSQL dev database.** The `postgres` service in `docker-compose.yml` is back, in the
  `postgres` profile. The Makefile enables it when `DB_DRIVER=postgres`.
- **`make gen-migration`** creates the next six-digit version in both migration folders.
- **`make bench`** runs every benchmark in every package that has one. The old default listed
  packages that do not exist.
- **`make toolbox-test-integration`** runs every integration-tagged package by default. The old
  default package did not exist.
- **`make toolbox-test-run`** uses the test database settings from `.env`.
- **`make toolbox-run`** opens a shell in the toolbox.
- **Unit test failures are no longer hidden.** `scripts/unit-test-phases.sh` ignored template test
  failures and failures in the parallel phase. It now fails when any phase fails.
- **`make prepare-release`** checks versions only in the toolbox. `make verify-container-first` now
  finds host `go` commands.
- **E2E video and slow motion.** Videos are recorded only with `VIDEOS=true`. `SLOW_MO` sets the
  delay in milliseconds. The E2E targets pass `SLOW_MO`, `SCREENSHOTS` and `VIDEOS` into the
  container.
- **CI uploads E2E screenshots and videos** from `tests/e2e/test-results/` and
  `tests/e2e/playwright/test-results/`.
- **Admin role user search reports database errors** instead of returning a partial list.
- **Plugin articles on PostgreSQL**: the plugin host call that adds an article to a ticket
  (`CreateArticle`) failed on PostgreSQL with a SQL syntax error. It now works on both MySQL and
  PostgreSQL.
- **Customer portal tickets get proper ticket numbers.** Tickets created at
  `/customer/tickets/new` took a `YYYYMMDDHHMMSS` timestamp as their number instead of the
  configured generator (e.g. DateChecksum), so two customers submitting in the same second got
  "Failed to create ticket" (duplicate `tn`). They now go through the ticket repository like agent
  tickets; an invalid `priority_id`/`service_id` answers 400.
- **Turning 2FA off works on single-label host names.** On hosts such as `helpdesk` (no dot),
  "Disable 2FA" removed the authenticator app and then answered 500 "failed to remove passkeys",
  and the 2FA status, recovery-code and admin-override checks counted no passkeys, because
  counting/deleting stored passkeys built a WebAuthn relying-party config from the request host.
  Those steps are plain database operations now; the admin override also reports a failed passkey
  removal instead of ignoring it.
- **Stats dashboard widgets and `/api/plugins/stats/*` count the right tickets.** "New today",
  the 30-day chart, `?range=` windows and overdue/SLA checks are computed from the server's local
  time and bound as parameters instead of `CURDATE()`/`NOW()`/`DATE()` (which followed the
  database time zone and differed between MySQL and PostgreSQL); `?range=` was ignored entirely
  and now works (unknown values answer 400 `invalid_range`). Non-admin agents now see only queues
  they hold `rw` on, directly or through a role (previously `ro` counted, role grants did not, and
  every API route plus a call without a user returned all tickets). A failed query shows the
  "widget unavailable" state or `{"error":"query_failed"}` instead of zeros. The SLA widget/route
  no longer counts tickets without an SLA or closed tickets (whose escalation times are cleared)
  as "met"; it reports open tickets within SLA per queue. Time tracking sums DECIMAL minutes
  correctly, `by-priority` works on PostgreSQL, queue/agent names are HTML-escaped, the weekly
  report reads admin emails from `UserEmail` preferences (the `users.email` column it queried does
  not exist) and refuses to send invented zeros, and plugin logging uses the real `log` host
  function. WASM plugins now get the real system clock (wazero's default was a fake 2022 clock)
  and a `time_now` host function returning the server's local time with offset.
- **Dashboard shows real data or says it can't.** A dashboard widget whose data cannot be loaded
  (plugin error, database failure) now stays on the dashboard with a "This widget is currently
  unavailable" notice instead of disappearing; the built-in Recent Tickets and Queue Status
  widgets no longer turn a failed query (including a failed admin-group check) into "No recent
  tickets"/"No queues available". An unreadable saved widget layout, a failed ticket-state lookup
  on the ticket list and queue pages (these used to fall back to hard-coded state ids 1-4), and a
  failed reminders-preference lookup (`/agent/api/preferences/reminders-enabled`,
  `/api/notifications/pending`) are now server errors rather than invented defaults. The canned
  `/api/dashboard/notifications`, `/quick-actions`, `/activity` and `/performance` endpoints (no UI
  used them), the unused real-time dashboard template and its random-number "live" stats,
  `dashboard-simple.pongo2`, `internal/components/dashboard`, the dashboard page's never-displayed
  ticket counts (whose "closed today" used `CURDATE()`) and the API docs for `/api/v1/dashboard/*`
  routes that never existed are removed; dashboard statistics are served by
  `/api/v1/statistics/dashboard`.
- **Admin changes record who made them.** Creating, updating or deleting agents, group and role
  memberships, ticket types, services, SLAs, ACLs, roles, standard attachments, customer companies,
  customer users, service assignments, customer group permissions, signatures, web services, system
  maintenance windows and organisation captive plugins stamped `create_by`/`change_by` with user 1
  (or fell back to it because the handler read the wrong context key); they now record the signed-in
  admin and refuse unauthenticated writes with 401. `PUT /api/types/:id` keeps fields the body omits
  instead of blanking the name and re-validating the type. The unrouted `PUT` branch of
  `/admin/roles/:id/permissions` (which could never insert, lacking `create_time`) is removed.
- **No more fake successes when the database is missing or a lookup fails.** `/api/canned-responses`
  served a hard-coded in-memory list and ignored the `canned_response` table (including the
  responses the setup assistant creates); it now reads and writes the table, adds create, update,
  delete, use, share, copy, import, export and statistics, and refuses customers. `POST`, `PUT` and
  `DELETE /api/v1/users*` now require an admin; any authenticated token could previously create
  agents or reset passwords. `PUT /api/v1/users/:id` no longer reports success without a database,
  stores the e-mail address in the `UserEmail` preference (email changes used to fail), rejects
  login changes and blank names. Login (password, customer, OIDC, SAML, passkey) now fails closed
  when the second-factor status or the admin-group lookup cannot be read instead of skipping 2FA or
  issuing a non-admin token. Pages render a 500 instead of a "GoatFlow" stub when templates are
  missing, the server refuses to start without a templates directory, the dashboard returns 500
  instead of zero counts, scheduler jobs fail instead of "succeeding" without a database, a plugin
  whose enabled state cannot be read stays disabled, and the postmaster DB filter stage fails
  instead of skipping configured filters.
- **Plugin routes, plugin UIs, the plugin API and MCP enforce the caller's permissions.** Plugin
  route middleware other than `auth`/`admin`/`group:`/`plugin:`/`webhook` (including the
  documented `customer`) was silently ignored, so such a route, a typo, or a malformed `plugin:`
  entry served everyone; `agent` and `customer` now work and anything else keeps the route
  unregistered. `group:` no longer admits a customer whose `customer_user` id matches a group
  member's `users` id. Plugin UIs with `token` or `pin` auth had no auth at all and `auth.groups`
  was ignored: `token` now authenticates like `session`, `pin` (no PIN flow exists) and unknown
  methods leave the UI unregistered, groups are enforced, `admin_page`/`agent_app` UIs refuse
  customers and `customer_app` UIs refuse agents. Client-sent `_is_admin`, `_customer_login`,
  `_user_*` and other host envelope args no longer reach plugins. `POST
  /api/v1/plugins/:name/call/:fn` is admin-only; plugin list, health, widgets and SSE channels
  refuse customers. MCP tools run with the request's real auth context (an admin's API token
  without `admin:*` scope no longer gets admin tools), tools whose middleware is missing are
  refused, plugin tools apply their route's middleware, and SSE sessions are bound to agent vs
  customer principals.
- **Ticket write APIs save what they report, and test modes no longer fake them.**
  `POST /api/tickets/:id/reply` never saved the reply. It now writes the article and its MIME
  data in one transaction (internal replies are not visible to the customer), links time units
  to that article and answers 404 for an unknown ticket. Messages added through
  `/api/tickets/:id/messages` are stored as articles instead of in memory. Internal notes
  (`/api/v1/tickets/:id/internal-notes`) are stored as internal articles; only the author or an
  admin can edit or delete one. Assign answers 404 for an unknown ticket. `PUT /api/v1/tickets/:id`
  no longer reports success for a missing ticket under `APP_ENV=test`. The `X-Test-Mode` header
  no longer bypasses authentication on ticket update, ticket create and article create.
  `DELETE /api/v1/tickets/:id` works for tickets without a customer (was 500) and answers 500
  instead of 401 when the database is down. The unauthenticated in-memory
  `/api/tickets/:id/merge`, `/unmerge` and `/merge-history` endpoints are removed; merging uses
  `/agent/tickets/:id/merge` and bulk merge.
- **`PUT /api/tickets/:id` and `GET /api/tickets/:id/history` are real.** The update answered 200
  without writing anything; it now runs the same handler as `PUT /api/v1/tickets/:id`. Both
  update the ticket and write one OTRS history entry per changed field (`TitleUpdate`, `Move`,
  `TypeUpdate`, `StateUpdate`, `PriorityUpdate`, `OwnerUpdate`, `ResponsibleUpdate`,
  `CustomerUpdate`, `Lock`/`Unlock`) in the same transaction. Unknown fields, wrongly typed values,
  unknown or invalid owners/responsibles/lock types and a `null` responsible are rejected with 400
  (an unknown owner or `null` responsible used to fail with 500). The history endpoint returned
  two hard-coded entries; it now reads `ticket_history` with type, actor, queue, state and
  priority names (`?limit=`, default 100). The `/agent/tickets/:id/draft` route only logged the
  request and nothing called it; it and the ticket-zoom "auto-save draft" placeholder are removed,
  as are the unrouted queue view/lock and customer-list stub handlers.
- **Ticket search, filtering and the ticket API read paths.** `/api/tickets/search` always
  returned no results (its query had two placeholders but got one argument, and the error was
  swallowed); it now finds tickets by title or number. Search and `/api/tickets/filter` only
  return tickets from queues the agent can read; before, they returned tickets from every queue.
  The `X-Test-Mode` header no longer bypasses authentication on `GET /api/v1/tickets` and
  `GET /api/v1/tickets/:id`. `GET /api/tickets` returns the real paginated ticket list instead of
  an empty stub, and the ticket list's filter form now refreshes from `/tickets`. Ticket reads,
  lists, messages, search and lookup lists no longer return canned sample tickets, queues or
  states when the database is unavailable; they answer 500 with a logged error. Lookup dropdowns
  list every ticket state instead of only the first five.
- **Admin ticket-type API no longer skips the admin check under `APP_ENV=test` (security), and
  admin pages stop faking data.** `POST/PUT/DELETE /api/types` skipped the admin role check when
  `APP_ENV=test`. They now always require the Admin role, and with no database they return 500
  instead of an echoed success. The admin type, state, service, postmaster filter, notification
  event, customer-user service, setup task and dashboard pages, and `/api/queues`, return a logged
  500 when the database, a query or the template renderer fails. They no longer return canned HTML
  with 200, empty dropdowns or zero counts. Service create/update/delete no longer report success
  without a database. Queue names are HTML-escaped in the `/api/queues` fragment. Unused
  test-only queue handlers (`queue_frontend_handlers.go`, `queue_test_helpers.go`) are removed.
- **Admin agent editing: agents with no title, unknown ids and password policy.** Opening an
  agent whose `users.title` is NULL (the seeded root agent, any OTRS-imported agent without one)
  answered 404 "User not found"; it now loads. Updating, deleting, toggling or resetting the
  password of an agent id that does not exist answers 404 (updates used to insert a new agent
  under `APP_ENV=test` and report success; the other actions reported success). Create and update
  reject unknown groups with 400 instead of creating them (create) or ignoring them (update),
  and write the agent and its groups in one transaction. Passwords set by an admin (create,
  update, reset) must satisfy the agent password policy (`PreferencesGroups###Password`, the one
  agents' own password change enforces) and, from the form, match the confirmation field; a
  generated reset password satisfies the policy. `/admin/password-policy` now returns that
  policy instead of a hard-coded one, and the users page checks it client-side with the same
  rules. A missing database is a 500 on every admin agent endpoint, and the users page no
  longer answers a bare `<h1>Users</h1>` in tests or when the database is down.
- **Customer password change with no stored password** answered 500; it now answers 401
  "Current password is incorrect".
- **Admin permissions: "Clone Permissions" works.** The modal posted to
  `/admin/permissions/clone`, which had no route (404). The route now replaces the target agent's
  group permissions with a copy of the source agent's in one transaction, as the modal warns
  (the service used to add the source's permissions on top of the target's); unknown users give
  404 and source = target gives 400.
- **Placeholder admin pages removed.** `/admin/settings` and `/admin/backup` showed "Under
  Construction" and were linked from nowhere; the routes and handlers are gone, as are the
  never-rendered `group_form`, `group_members` and `group_view` placeholder templates.
- **Admin → Groups: search, status filter, sort, Enter and "Inactive" work.** The search box,
  the Active/Inactive filter and the column sorting only logged to the browser console; they now
  filter and sort the list (search covers name and description, a "No groups found" row shows when
  nothing matches, and search/filter survive a reload). Pressing Enter in the Add/Edit Group modal
  threw a script error instead of saving; it now submits the form with the usual required-field
  check. Creating a group with status Inactive stored it as active (`POST /admin/groups` ignored
  `valid_id`). The delete dialog claimed the action could not be undone and removed all
  memberships, but delete sets the group inactive (OTRS-style) and keeps its members; the dialog
  now says so. A failure to load a group for editing shows a proper Guru Meditation code.
- **Attachments:** single-attachment URLs are now article-scoped
  (`/api/tickets/:id/articles/:article_id/attachments/:file_id`, same under `/customer`). The
  agent attachment routes now require ticket queue permissions. Filenames are HTML/JS-escaped in
  attachment lists and safely encoded in `Content-Disposition`. The local-storage double write
  (file on disk plus a DB copy) and the in-memory mock attachments are gone. Thumbnails are
  ETag-validated instead of cached under `./storage/thumbs`. The old storage CLI wrote to tables
  that existed on neither database (`article_storage_references`, `article_storage_migration`).
- **OTRS import** keeps OTRS ids for every imported table and `content_path`, imports
  `article_data_mime_attachment`/`article_data_mime_plain` byte-exact, maps columns by name, resets
  id sequences on both drivers, and runs in one transaction that rolls back on the first rejected
  row instead of reporting success. It now covers the whole OTRS 6 / Znuny 6.x schema with a
  printed per-table plan and report: agent group/role/customer-group permissions (imported agents
  log in and see exactly the queues OTRS granted), preferences, dynamic fields, time accounting,
  flags, links, watchers, templates with attachments, auto responses, notifications, generic agent
  jobs, ACLs, services/SLAs and changed settings (`sysconfig_modified`, matched by name); sessions,
  caches, logs and daemon state are skipped with a reason. `-source` reads an OTRS database on
  MySQL/MariaDB or PostgreSQL directly (decoding OTRS's base64 blobs on PostgreSQL); `-sql` still
  reads mysqldump files. The import sets GoatFlow's ticket number counters from the imported ticket
  numbers (OTRS generator and SystemID), so a ticket created after a same-day migration no longer
  fails on a duplicate `tn` and AutoIncrement numbering continues. `make migrate-analyze`,
  `migrate-import`, `migrate-import-force` and `otrs-import` all run goatflow-migrate with `SQL=` or
  `SOURCE=`; `otrs-import` on MariaDB no longer drops every table and loads the dump raw.
  `make migrate-import-force` no longer hides failures, and the migrate targets build a MySQL or
  PostgreSQL URL from `DB_DRIVER`.
- **Ticket attribute relations failed open.** When relations restricted a list (states,
  priorities, types, services, queues) but none of the allowed values matched an item, the list
  endpoints returned the full unrestricted list. They now return an empty list, matching OTRS ACL
  semantics.
- **Outbound webhooks never worked.** The webhook API queried tables (`webhooks`,
  `webhook_deliveries`, `webhook`) that no migration creates, its handlers were not routed, and
  nothing ever sent an event. Rebuilt end to end: migration 000028 (`gk_webhook`,
  `gk_webhook_delivery`, `gk_webhook_event_cursor`); admin API at `/api/v1/webhooks` (CRUD,
  `/events`, `/:id/test`, `/:id/deliveries`, `/deliveries/:id`, `/deliveries/:id/redeliver`);
  the runner's `webhook-dispatch` task publishes `ticket.created`, `article.created`,
  `ticket.closed`, `ticket.state_changed`, `ticket.queue_moved`, `ticket.assigned`,
  `ticket.priority_changed`, `ticket.merged`, `ticket.escalated` and `ticket.updated` from the
  ticket, article and ticket_history tables (every write path, exactly once) and delivers them
  with `X-Webhook-Signature: sha256=<HMAC>`, recorded attempts and exponential-backoff retries.
  Signing secrets are encrypted with `GOATFLOW_SECURE_KEY`, which the runner now also receives
  (docker-compose, deploy/docker-compose.yml). The unused `internal/api/v1` webhook handlers and
  the repository-less `platform/webhook` manager were removed.
- **Webhooks admin page; admin-scoped API tokens reach admin API routes.** Admin > Webhooks
  (`/admin/webhooks`, linked from the admin dashboard) lists webhooks with their last delivery,
  creates and edits them (events, custom headers, retries, timeout, write-only signing secret with
  a remove option), activates/deactivates and deletes them, sends test deliveries, and shows each
  webhook's delivery log with payload, response and Redeliver. The YAML `admin` route middleware
  now also admits callers the auth layer marks as admin-group members (`*` / `admin:*` API tokens,
  admin JWT claims) instead of answering 403 unless the role is `Admin`. The unused
  `features.webhooks` / `integrations.webhook` settings and the `FEATURE_WEBHOOKS` variable were
  removed (each webhook's active flag is the switch), as was the empty routes/redirects.yaml.
- **YAML route groups with an empty prefix leaked their middleware.** A group with `prefix: ""`
  and middleware (routes/redirects.yaml: `auth`) registered it on the engine's root group, so every
  group loaded after it (random map order) inherited it; e.g. unauthenticated API calls got a 303
  login redirect instead of 401 on some starts.
- **Internal notes could be shown to customers.** Creating an article through the article
  repository turned "not visible to the customer" into "visible", so internal notes (ticket notes,
  GenericAgent notes, internal-note ticket creation) appeared in the customer portal. Ticket notes
  and close notes also failed outright on both databases (they used a communication channel that
  does not exist).
- **Article REST API (`/api/v1/tickets/:id/articles`) works on MySQL/MariaDB and PostgreSQL.**
  Get, list, update and delete queried tables and columns that do not exist (`article_type`,
  `article_attachment`, subject/body on `article`). They now read `article` +
  `article_data_mime`, list attachments from the article content store, and delete removes the
  article with its MIME data, attachments, flags and search index while keeping history and time
  accounting. The API's `article_type` is mapped to communication channel + customer visibility
  in one place (`internal/core/channel_mapping.go`); unknown types are rejected and customers
  always write as customer.
- **Plugin HostAPI article attachment calls use the article content store.** `pkg/plugin`
  `ArticleAttachment` no longer carries `created_at`/`created_by`; `id` is the attachment's id
  within its article.
- **"Disable 2FA" left passkeys active, which could lock users out.** Turning 2FA off removed
  only the authenticator app; any passkey stayed required at sign-in, and a passkey made on another
  host name could not be used at all. Turning 2FA off now removes the authenticator app, every
  passkey and the recovery codes, and accepts a recovery code as well as an authenticator code.
- **Recovery codes only worked alongside an authenticator app.** Code validation required a TOTP
  secret before checking recovery codes; recovery codes now work on their own.
- **Recovery codes could not be submitted on the 2FA login form.** The code field had a digits-only
  `pattern`, so the browser blocked the 12-character recovery codes the hint invited.
- **Plugin menu items render in a stable order and translate their labels.** `Manager.MenuItems`
  iterated a map, so plugin nav links could swap places between restarts and `Order` was
  ignored; items now sort by `Order`, then plugin name, then ID. Labels go through `t()` in the
  top nav, mobile sidebar, admin cards and Profile, so a plugin can use a key from its
  `I18nSpec` (plain labels render unchanged).
- **Plugin pages and YAML routes now see the user's language.** Both are served by the dynamic
  engine via `NoRoute` → `HandleContext`, which resets the gin.Context keys the main engine's i18n
  middleware set, so `c.Get("language")` was always empty there (plugin UI shells rendered LTR
  English even with `?lang=ar`). The dynamic engine now runs the i18n middleware itself.
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
- **E2E ticket-search test raced the page load.** `TestTicketSearchFiltersResults`
  waited for the URL to contain `search=…` and then counted matching rows once;
  the URL flips at navigation commit, before the new HTML is parsed, so the count
  could run on a half-loaded document and flake to 0. The count now polls
  (`require.Eventually`, 15 s) until the row is present. The precondition before the search
  counted the two new tickets right after the same commit-only navigation and flaked the same
  way; it now waits for both rows (`tests/e2e/playwright/ticket_search_test.go`).
- **PostgreSQL installs could not run migrations at all.** `migrations/postgres/` still held the
  old copies of three migrations after they were renumbered to match MySQL
  (`000005_customer_portal_sysconfig`, `000024_saml_fields`, `000025_user_table_for_idp_routing`), so
  golang-migrate stopped with "duplicate migration file". The stale copies are removed (the
  customer-portal down migration moves to `000003`); the PostgreSQL and MySQL migration sets now have
  the same versions and `migrate up` runs them all on an empty PostgreSQL database. The PostgreSQL
  test DB init script (`docker/postgres/testdb/10-apply-migrations.sh`) applied only migration 1; it
  now applies every `*.up.sql` in order, like the MariaDB one.
- **MariaDB test DB came up without most of its schema.** `docker/mariadb/testdb/10-apply-migrations.sh`
  applied a hard-coded list of migrations 1–4 (one name stale), and the test DB lives on tmpfs, so
  every container recreation lost `canned_response`, `user_api_tokens`, `gk_identity_provider` and
  the rest; 12 `internal/api` tests failed. The script now applies every `*.up.sql` in order.
- **Playwright Go test image apt failures on the MCR base.** `Dockerfile.playwright-go`
  now restores `/tmp` to the standard mode 1777: the base image ships `/tmp` as
  `0755 root:root`, and apt's unprivileged `_apt` signature-verification user
  cannot create its queue/config temp files there.
- **`make openapi-lint` and `make openapi-bundle` work again.** Both read `/spec/openapi.yaml`,
  a path that is not mounted in the container, so they always failed. They now use
  `api/openapi.yaml`, and the bundle (`api/openapi.bundle.yaml`) is regenerated.
- **Browser e2e pages crashed after a handful of navigations.** In the `make test-e2e-go` /
  `test-e2e-playwright-go` container an authenticated page failed after 7-9 navigations with
  `net::ERR_INSUFFICIENT_RESOURCES` or "Page crashed". Playwright starts Chromium with
  `--disable-dev-shm-usage`, so the browser keeps every shared-memory segment as a file under
  `TMPDIR`, which pointed at the bind-mounted repository (`/workspace/tmp`). Each navigation
  leaves about 38 MB of 2 MiB shared-memory buffers in the renderer (one per `no-cache` asset
  revalidated with a 304) until its garbage collector runs, so a nearly full repository
  filesystem ran out of space within a few pages. The container now gets its own tmpfs for
  `/tmp` (`E2E_TMPFS_SIZE`, default 4g) and `TMPDIR` points there.
- **Deactivate/Activate on the customer company list did nothing.** The buttons posted without
  `Accept: application/json`, so the server answered with a redirect to the HTML list,
  `response.json()` failed and the page neither reloaded nor showed an error (the company was
  deactivated anyway). The requests now ask for JSON and failures are shown. The company ID is
  passed through a `data-` attribute instead of being written into the `onclick` JavaScript
  string, and the unused company name argument (which an admin-entered name containing a quote
  could break out of) is gone.
- **Saving a company's Portal Settings no longer switches its portal off.** Saving the per-company
  portal form with no override ticked stored `enabled=false`/`login_required=false` overrides,
  which disabled the customer portal for that company, and a browser post landed on raw JSON. The
  form now always posts every override flag, unticked settings stay inherited, and a browser post
  returns to the company's Portal Settings tab with a success message. The "saved (sysconfig
  unavailable)" fake-success branches and the unused, fake customer portal logo upload route are
  removed; the portal settings help text now describes the per-company tab (15 languages).
- **Web service dynamic fields can be created and tested from the admin UI.** The New/Edit forms
  never listed web services and ignored every `webservice_*` input, so a WebserviceDropdown or
  WebserviceMultiselect field always failed with "requires a webservice". The form now offers the
  valid web services that have invokers (plus the field's current one), saves the web service,
  invokers, stored/displayed values, separator and autocomplete limits, and rejects bad numbers
  with 400. "Test Connection" calls `POST /admin/api/dynamic-fields/:id/webservice-test` and shows
  the result instead of a placeholder toast; an unsaved field asks to be saved first. The field
  list's tab badges show the number of fields per tab instead of the number of object types.
- **Admin → Users: duplicate logins get a clear error.** Creating a user with an existing login
  answered 200 with `success:false` (the page showed "An error occurred"), detection only matched
  PostgreSQL's error text, and renaming a user to another agent's login returned 500. Both now
  answer 409 "User already exists", which the page shows next to the login field.
- **Admin → Queues lists disabled queues.** The page listed only valid queues, so a disabled queue
  could not be re-enabled from the UI.
- **`POST /api/v1/search` returned nothing on every database.** The built-in search backend
  (`postgresql`) used PostgreSQL full-text functions and queried tables that exist on neither
  driver (`tickets`, `queues`, `article.subject/body`), so every search errored and the handler
  answered with empty hits. It is replaced by the portable `database` backend
  (`internal/platform/search/database_backend.go`): every query word must match (case-insensitive
  `LIKE`) ticket number/title/article text, article subject/body/sender, or customer
  login/email/name; results rank exact ticket number/title/subject/login matches first. Works
  identically on MySQL/MariaDB and PostgreSQL; `/api/v1/search/health` and `/reindex` now report
  backend `database`.

- **GoatFlow did not work on PostgreSQL, and several paths were broken on MySQL too.** About 470
  queries reached the database without the conversion layer, plus 70 `LastInsertId()` calls,
  MySQL-only upserts/`INSERT IGNORE`/`DATE_FORMAT`/`FROM_UNIXTIME`/`GROUP_CONCAT`, and
  PostgreSQL-only `RETURNING`/`ON CONFLICT`/`||` SQL. Every statement now goes through
  `database.ConvertPlaceholders` / `ConvertUpsert` / `InsertWithReturning`, and the Go suite passes
  on both MySQL/MariaDB and PostgreSQL. Bugs found on the way that were wrong on both databases
  include: ticket repository updates/lock/unlock passing arguments in the wrong order; placeholder
  and argument counts not matching (customer search, canned responses, attachment insert,
  sysconfig, `GetTicketsByOwner`, dashboard "my tickets"); `REPLACE INTO user_preferences`
  duplicating rows (the table has no unique key); `INSERT IGNORE` into `role_user` without its
  NOT NULL timestamps; v1 admin settings writing a non-existent `value` column; queries on columns
  or tables that exist on neither database (`ticket_type.comments`, `group_user.permission_value`,
  `queue.comment`, `user_group`, the ticket-state in-use check on `tickets`).
- **PostgreSQL: lookup handlers returned 500 for ids above 32767.** Priority, state and type
  update/delete now answer not-found on both databases (the ids are `SMALLINT`).
- **PostgreSQL: some test packages hung for 10 minutes.** `adapter.GetDirectDB` always opened a
  MySQL connection, which waited out PostgreSQL's 60-second authentication timeout on every call.
  It now uses the configured driver, with 5-second connect timeouts.
- **PostgreSQL schema had 37 foreign keys; MySQL has 304.** Migration `000027` adds the missing 267
  to PostgreSQL with the same names and rules (a no-op on MySQL). On an existing database, a key
  that old rows would violate is left `NOT VALID` with a notice instead of failing the migration.
- **PostgreSQL test DB setup.** Its seed now matches MySQL (roles, de-duplicated `group_user`), and a
  new `docker/postgres/testdb/60-set-admin-password.sh` sets the test admin password like the
  MariaDB one. `scripts/tools/check-sql.sh` (the pre-commit SQL guard) ran with `|| true` and could
  never block a commit; it now does.

### Changed
- **Declared plugin permissions take effect.** Without an admin-stored policy a plugin gets exactly
  the permissions it declares (re-read on reload), instead of the fixed default. `http` scope `*`
  allows any host. gRPC plugins can implement `grpcutil.GKPluginWithContext` to run host calls in
  the call's context (acting user, language, deadline).
- **Webhook delivery log is cleaned up.** The runner deletes delivered and failed deliveries older
  than 30 days. Change this with `GOATFLOW_WEBHOOK_DELIVERY_RETENTION_DAYS` (0 keeps them forever).
- **Webhook delivery routes moved** to `GET /api/v1/webhook-deliveries/:id` and `POST
  /api/v1/webhook-deliveries/:id/redeliver` (were under `/api/v1/webhooks/deliveries/`). The Go,
  Python and TypeScript SDKs use the new paths.
- **Zinc credentials use `ZINC_USER`.** GoatFlow now reads `ZINC_USER` (as Docker Compose and the
  config generator already did) instead of `ZINC_USERNAME`. New optional `SEARCH_INDEX_PREFIX`
  (default `goatflow_`). The backend and the runner need the same search settings.
- **Docker Compose has a `zinc` service** in the `search` profile, and passes the search settings to
  the backend and the runner.
- **Migration 000034** adds `change_time` indexes on `ticket`, `article` and `customer_user` for the
  search sync.
- **Ticket view and list show escalation.** The ticket view shows the ticket's service, SLA and each
  escalation due time, highlighted when overdue. The agent ticket list marks escalated tickets and
  has an "Escalated" status filter.
- **More notification recipients and OTRS events in the form.** The form adds ticket creator, agents
  with the queue in My Queues, all agents with read or write access to the queue, and additional
  email addresses (checked on save). It also lists the OTRS events used by imported notifications
  (NotificationNewTicket, NotificationMove, ...).
- **New migration 000033** adds `gk_notification_event_cursor`, the position of the notification
  evaluator.
- **Spec drift now fails the test suite.** `TestOpenAPISpecMatchesRoutes` checks that every
  operation in `api/openapi.yaml` is served by the production router (path, parameter names, method)
  and that only public routes are documented without authentication.
  `TestSwaggerAnnotationsMatchRoutes` does the same for the swag `@Router` annotations behind
  `/swagger/`.
- **MCP tools read their input schemas from `api/openapi.yaml`.** Path parameters in the spec use
  the route names (`{id}`, `{article_id}`, `{name}`) so the MCP tool generator finds the operations.
- **Documentation brought in line with 0.10.0.** ROADMAP, README, FEATURES, configuration,
  security, architecture, deployment (Docker, Helm, TrueNAS), API, HostAPI, testing and
  database docs were checked against the code; claims about features that do not exist (for
  example Excel export, WebSocket, SMS MFA, sign-up approval/CAPTCHA, OTRS 5.x import) are gone.
  New guides: `docs/WEBHOOKS.md`, `docs/REPORTS.md`, `docs/OBSERVABILITY.md` and
  `docs/CUSTOMER_PORTAL.md`; the admin guide and agent manual are real guides now. The hand-written
  `api/openapi.yaml` lists only routed endpoints. Obsolete status pages removed
  (`docs/development/MVP.md`, `docs/MYSQL_COMPATIBILITY_ISSUES.md`,
  `docs/development/TICKET_NUMBER_CONFIG.md`, `docs/development/TICKET_REOPEN_CONFIG.md`).
  The Helm chart README lists the chart's known limitations for this release.
- **Placeholder pages removed.** The `pages/under_construction.pongo2` template and its helper,
  the never-routed customer placeholder templates' `customer.placeholder_pages` i18n block, the
  unused customer KB keys (`customer.knowledge_base`, `customer.kb_search`,
  `customer.kb_article`), the `/admin/modules` dynamic-module comparison page
  (`pages/admin/dynamic_test.pongo2`, `dynamic_test.*` keys; `/admin/modules/:module` is unchanged)
  and the misrouted `GET /admin/groups/new` (the create handler, always 400) and
  `GET /admin/groups/:id/edit` (JSON) are gone. The customer dashboard's Knowledge Base card is
  shown only when the goat-kb plugin registers its `/customer/kb` menu item; before, it linked to
  a 404 when the plugin was not installed.
- **Storage config:** `storage.type` is `db` or `fs` (env `STORAGE_TYPE`). The unimplemented
  `storage.s3` and the unused `storage.local.public_path` are removed, and so are the Helm `s3`
  values. The unused `/api/files/*path` route and the unrendered `pages/agent/ticket_view.pongo2`
  (with 12 i18n keys only it used) are deleted.
- **Dead queue delete dialog removed from Admin → Queues.** No control ever opened it. Queues are
  removed by disabling them with the status toggle; `DELETE /api/v1/queues/:id` is unchanged.
- **Dead pre-plugin customer KB handlers removed.** `handleCustomerKnowledgeBase` /
  `handleCustomerKBSearch` / `handleCustomerKBArticle` (plus their `GlobalHandlerMap`
  registry entries and the commented-out `/kb*` route stubs) rendered
  `pages/customer/{knowledge_base,kb_search,kb_article}.pongo2` — templates that
  were deleted when the goat-kb plugin took over the customer KB
  (`/customer/kb`, `/customer/kb/article/:id` in its `GKRegister` routes), leaving
  the handlers as landmines: a route registered against them would 500 on a
  missing template. No functional change to live behaviour; the customer KB was
  already served entirely by the plugin.
- **Health/metrics endpoints de-fingerprinted and admin-gated.** `GET
  /health/detailed` and `GET /metrics` on the app port now require
  admin authentication (route-level `auth` + `admin` middleware in
  `routes/basic.yaml`), so build version, git commit, process uptime
  and Valkey cache telemetry are no longer reachable unauthenticated.
  The public liveness `GET /health` keeps the lean probe payload
  (status + `version.Short()`, no commit SHA) so Docker HEALTHCHECK,
  TrueNAS and k8s liveness/readiness probes continue to work;
  `version.String()` (semver + short SHA) is now served nowhere over
  HTTP. Prometheus scrapers use the unauthenticated standalone
  listener on `METRICS_PORT` (default 9090) — internal network only,
  not a published port. Both paths are also removed from the
  customer-FE allowlist (`internal/api/customer_only_guard.go`), so
  customer-facing deployments 404 them instead of serving them to the
  portal network.
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
  `appVersion: "1.0.0"` (no such app release exists) and had no `kubeVersion` gate; it now
  declares `appVersion: "0.10.0"` and `kubeVersion: ">=1.25.0"`, matching the declared support
  floor in `charts/goatflow/README.md` and the system requirements. The install examples in the
  chart README now point at `0.10.0`. Chart `version` in `Chart.yaml` stays `0.1.0`; on a release
  tag, CI publishes the chart with the tag's version (e.g. `0.10.0`). Verified with `helm lint`
  (0 failures) and `helm template` render.
- **Helm chart now references the real container image.** `charts/goatflow/values.yaml`
  defaulted `backend.image.repository` to `goatflow/backend` — an image that never existed
  (Docker Hub 404 / ghcr 403) and a leftover from the pre-rename era — so any out-of-the-box
  `helm install` of the published charts ImagePullBackOff'd. The default is now
  `ghcr.io/goatkit/goatflow`, matching the app's published image. Also, the CI chart-publish
  step (`.github/workflows/build.yml`) wrote `appVersion` with a `v` prefix (`v0.9.0`) while the
  container image tags have no `v` (`ghcr.io/goatkit/goatflow:0.9.0`), so the rendered image tag
  never matched a real tag; it now strips the `v`. Verified: `helm template` renders
  `image: ghcr.io/goatkit/goatflow:<appVersion>`, which is pullable from ghcr once that release is
  published.
- **Route YAML schema normalised and API-doc generation wired into the build.** All 33 route
  groups now declare the fully-qualified `apiVersion: goatflow.io/v1` (previously bare `v1`),
  and the multi-document route files were renamed to unambiguous names
  (`api-tokens.yaml` → `api-tokens-global.yaml`, `api-v1.yaml` → `api-v1-global.yaml`,
  `admin_dynamic_aliases.yaml` → `admin-dynamic-aliases.yaml`,
  `admin_mail_account_status.yaml` → `admin-mail-account-status.yaml`). A shared
  `routing.ParseYAMLDocuments` (`internal/platform/routing/parse.go`) decodes every document in a
  multi-doc route file — a single `yaml.Unmarshal` call would silently return only the first —
  and is now used by the `route-docs` / `route-lint` / `route-test` / `route-version` tools.
  New `make api-docs` target builds `Dockerfile.route-tools` and regenerates `generated-docs/`
  (OpenAPI + Swagger) from `routes/*.yaml`; the checked-in `docs/api/` and `generated-docs/`
  outputs are that regeneration.

### Removed
- **`SessionMiddleware` and its demo tokens.** The unused middleware accepted any `demo_session_*` value as an
  admin and `demo_customer_*` as a customer. Docs, scripts and acceptance tests now sign in for real
  (`scripts/lib/admin-login.sh`, `tests/acceptance/login.js`).
- **`cmd/generator`.** The admin-module code generator had no callers and emitted handlers with
  PostgreSQL-only SQL (`$1`, `ILIKE`), raw database errors and unfinished scan code; it is deleted.
- **GraphQL scaffold.** `internal/api/graphql` never compiled, had no route and no generated code;
  it is deleted.
- **Oracle and SQL Server backends.** They were never usable. Selecting either now fails with
  `ErrDatabaseNotImplemented`; supported databases are MySQL/MariaDB and PostgreSQL.
- **Unused `handleTOTPVerify` handler.** It had no route; 2FA login uses `/api/auth/2fa/verify`.
- **`organisation.RegisterOrgAwareTable`**: nothing called it, and plugins could not reach it.
- **Helm**: nginx frontend Deployment/Service/ConfigMap and the unused `backend-config` ConfigMap.
- **`docker-compose.prod.yml`**: it overlaid services that no longer exist (`frontend`, `mailhog`,
  an unprofiled `postgres`), so `docker compose -f docker-compose.yml -f docker-compose.prod.yml`
  failed ("service \"frontend\" has neither an image nor a build context"). Use
  `deploy/docker-compose.yml` (Caddy with automatic TLS, released images, no dev tools);
  docs/DATABASE_SAFETY.md now points there.
- **Dead `default.yaml` sections.** `logging`, `metrics`, `rate_limiting`, `integrations` and
  `database` and the unused `features.*` keys (`social_login`, `two_factor_auth`, `api_keys`,
  `ldap`, `saml`, `knowledge_base`, `customer_portal`, `agent_collision_detection`) were read by
  nothing. Logging and metrics are set by `LOG_*` and `METRICS_*`, the database by `DB_*`. Old keys
  in `config.yaml` are ignored.
- **`cache_size_bytes` metric.** It was registered but never set, so it always read 0.
- **Unused Go modules.** `github.com/gorilla/websocket` and `github.com/xeonx/timeago` were dropped
  from `go.mod`.
- **Unused `internal/platform/zinc` package** (a second Zinc client with no callers) and its search
  request/result models.
- **Mock SLA endpoints.** The unrouted `ticket_sla_handler.go` handlers (priority-based fake SLA
  status, escalate, SLA report and SLA config) and their `sla_status` template are gone.
- **`rickar/cal` dependency.** Replaced by the OTRS-compatible calendar code.
- **Notification events that never fire.** TicketDelete, TicketServiceUpdate, TicketSLAUpdate,
  TicketSubscribe, TicketUnsubscribe, TicketFlagSet, TicketFlagDelete, ArticleSend, ArticleBounce,
  ArticleAgentNotification and ArticleCustomerNotification are no longer offered. Nothing in
  GoatFlow records these events.
- **Fake ticket split route.** `POST /agent/tickets/:id/split` added a note to the ticket instead of
  splitting it, and nothing in the UI called it. The route and its API doc entries are gone.
  GoatFlow has no ticket split.
- **`docs/api/openapi.yaml`**, a second hand-written spec in which 61 of 97 operations were not
  routed.
- **The unused OpenAPI response validator** (`internal/platform/middleware/openapi.go`); nothing
  installed it.
- **`scripts/generate-docs.js`, `scripts/generate-types.js`** and the `generate-types`,
  `generate-docs`, `validate-api`, `serve-docs`, `test:contract` and `contracts:all` npm scripts.
  They wrote into a `web/` directory that does not exist and would have overwritten
  `docs/api/README.md`. The `js-yaml` dev dependency they needed is gone too.
- **Broken make targets:** `test-legacy`, `test-specific`, `test-frontend`, `toolbox-test-all`,
  `toolbox-test-email-integration`, `playwright-build`, `test-e2e-playwright`,
  `test-e2e-playwright-watch`, `test-e2e-playwright-debug`, `test-e2e-playwright-report`,
  `test-acceptance-playwright`, `api-call-test`, `api-call-form-test`, `reset-db`, `frontend-logs`,
  `frontend-logs-follow`, `db-migrate-schema-only`, `db-migrate-schema-only-test`, `db-seed-dev`,
  `db-reset-dev`, `db-refresh`, `db-init-dev` and `db-init-import`, and
  `docker-compose.playwright.yml`. Use `test-e2e-playwright-go` / `test-e2e-go` for browser tests
  and `db-init` to recreate the dev database. `make test-all` now runs `make test` and `make
  test-contracts`.
- **Unused files** `migrations/0024_user_table_for_idp_routing.sql` and
  `migrations/0025_saml2_idp_columns.sql`.
- **Mock-router tests** in `internal/api/htmx_routes_test.go`, and the leftover `TEST_AUTH_*` /
  `GOATFLOW_DISABLE_TEST_AUTH_BYPASS` settings in tests.

### Security
- **Customer ticket list SQL injection fixed.** The `order` parameter of the customer portal ticket
  list was put into the SQL `ORDER BY` as given. It now only accepts `asc` or `desc`.
- **Database errors no longer reach the client.** Handlers that sent the raw database error text in
  the response (customer portal tickets, agent ticket and queue lists, roles, customer users,
  notification events, generic agent jobs, ticket archive, setup assistant customer search, dynamic
  modules) now log it and answer with a generic message.
- **Production needs a real JWT secret.** `config/default.yaml` no longer ships a placeholder JWT
  secret. With `APP_ENV=production` the server refuses to start when `JWT_SECRET` is missing,
  shorter than 32 characters or a known placeholder, and when the database password,
  `SESSION_SECRET` or `ZINC_PASSWORD` is the example value. `APP_ENV` from the environment now
  always wins over `app.env` from the config files.
- **Secure settings key is never logged.** When `GOATFLOW_SECURE_KEY` is unset, the generated key is
  no longer written to the log.
- **Agent web login is rate limited.** `/api/auth/login` uses the same lockout as the other logins
  (5 failures in 5 minutes per IP and login name, then `429`).
- **Second-factor codes are rate limited.** Agent and customer 2FA code checks count failures per IP
  and account, so logging in again with the right password no longer gives fresh guesses.
- **API token rate limit is enforced.** Requests with a `gf_*` token are limited to the token's
  `rate_limit` per hour (default 1000) and get `X-RateLimit-*` headers; over the limit they get
  `429` with `Retry-After`.
- **Customer-only instances expose less.** With `CUSTOMER_FE_ONLY` set, `/health/detailed`, the
  agent login, agent 2FA and agent passkey APIs and `/login/2fa` now answer `404`.
- **Password re-checks are rate limited.** Agents and customers who are already signed in must enter
  their password to change it, set up or turn off 2FA, get new recovery codes, or add or remove a
  passkey. Wrong passwords now count per account and IP, with the same backoff as login. A stolen
  session can no longer be used to guess the password quickly.
- **Reset links are rate limited.** Posting a new password with a reset link now counts toward the
  same budget of 10 posts per IP per hour as the forgot-password and sign-up forms, so reset tokens
  cannot be guessed at an unlimited rate.
- **Secrets no longer appear in logs.** A failed dynamic-module insert, query or update logged the
  values, which could include password hashes. It now logs only the table and column names, or the
  SQL with placeholders. Every API token check logged the first 20 characters of the raw token. The
  auth middleware logged the session ID on every request. The user lookup logged the start of the
  password hash. All of these are removed.
- **gRPC plugins now go through the plugin sandbox.** Host callbacks from gRPC plugin processes used
  the raw host API, so no gRPC plugin ever had a permission check or rate limit. Every callback now
  runs through the plugin's SandboxedHostAPI; callbacks before Init are refused.
- **Permission checks for all privileged HostAPI methods.** Entity soft delete, restore and recycle
  bin need `entity` permissions; permanent deletion needs an explicit `entity` `hard_delete` grant
  in an admin-stored policy. Articles, attachments, ticket status/states/views and plugin files need
  the new `article`, `ticket` and `file` permissions.
- **Stricter SQL table scoping.** The sandbox now reads tables from comma joins, quoted and
  schema-qualified names, subqueries and DDL targets, requires a write grant for every table a
  statement writes (also via DBQuery), anchors wildcard scopes and unions several db entries.
- **Plugin setup tasks use the host's caller envelope**: both setup-task routes used to pass the
  request body to the plugin unchanged, so a body with `_user_id`, `_user_role`, `_org_id` or
  `_lang` chose the acting user, organisation and language of the call. They now drop those keys and
  send the host envelope, like every other plugin call. A body that is not a JSON object is refused
  with 400.
- **PDF rendering fetches nothing remote**: `RenderMarkdownToPdf` used to let the headless Chromium
  load any `https` image a plugin put in its Markdown or `BrandLogoURL`. That was outbound traffic
  outside the plugin's `http` grant. Images now print only from inline `data:` URIs (png, jpeg, gif,
  webp). Other image sources are replaced by an empty image. `BrandLogoURL` must be a `data:` URI.
  Plugins that want a remote image fetch it with `HTTPRequest` and embed it.
- **Webhooks no longer reach internal addresses.** Webhook URLs that point at loopback, private,
  link-local (including the cloud metadata address 169.254.169.254), multicast, unspecified or other
  internal addresses are rejected when saved. Every delivery resolves the host itself, refuses it if
  any address is internal, and connects only to the checked address, so DNS rebinding cannot get
  around the check. Set `GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS=true` on the backend and runner to
  allow internal targets on purpose. Webhook deliveries no longer use `HTTP_PROXY`/`HTTPS_PROXY`.
- **Webhook custom header values are encrypted and write-only.** They are stored encrypted with
  `GOATFLOW_SECURE_KEY`, like the signing secret, and are never returned again: the API returns
  `header_hints` instead of `headers`. On update, a `null` header value keeps the stored value.
  Migration 000030 drops the old plain-text column, so header values saved by pre-release builds
  must be entered again. Header values with line breaks or other control characters are rejected.
- **`/api/v1/statistics/agents` no longer lists every agent.** Admins still see every valid agent.
  Other agents see only the agents with activity on tickets in queues they can read.
- gosec now fails the CI build on any finding. CI and the new `make gosec` (toolbox) target use the
  same pinned version (`GOSEC_VERSION` in Dockerfile.toolbox, now v2.29.0; CI used v2.22.4 before)
  and the same flags (`GOSEC_FLAGS`). All 478 findings were fixed or annotated with `// #nosec
  G<rule> -- <reason>`.
- The HTTP server and the Prometheus metrics listener set a 10s ReadHeaderTimeout, which blocks
  Slowloris-style slow-header connections.
- User ids taken from the request context go through one range check, so an out-of-range id can't
  wrap around into another user's id. Custom-field access checks reject non-positive queue ids with
  404.
- Theme plugin install and uninstall reject theme names that would escape the theme cache directory.
- `gk plugin init` validates plugin names. `goatflow-config import` reads through a root-scoped
  handle and `goatflow-config export` rejects names that would escape the output directory. The YAML
  version store doesn't follow symlinks out of `.versions`.
- Plugin package extraction drops setuid, setgid, sticky and world-access bits from archive modes.
- Files that can hold secrets are created owner-only (0600): `.env` backups, the sysconfig deployed
  config, YAML version snapshots, restored config documents, plugin file storage and new log files.
- **Shared secure key everywhere**: `GOATFLOW_SECURE_KEY` is now required by `docker-compose.yml`,
  `docker-compose.draft.yml` and `deploy/docker-compose.yml` (they refuse to start without it) and
  is given to every GoatFlow container. Before, an empty default made each process generate its own
  key, so the runner could not decrypt webhook signing secrets.
- **Helm: generated secrets survive upgrades**: database passwords, `JWT_SECRET`,
  `GOATFLOW_SECURE_KEY` and the admin password are generated once and read back from the live
  Secrets on `helm upgrade`. Before, every upgrade re-rolled them and the database volume kept the
  old password.
- **TrueNAS: secure key must be 64 hex characters**: the wizard and the template now reject anything
  else (before: "minimum 32 characters", which GoatFlow cannot use).
- **`make synthesize` generates `GOATFLOW_SECURE_KEY`**: the synthesized `.env` now contains a
  64-hex-character secure key, which the compose files require. `--rotate-secrets` keeps an existing
  key (a new one would make every stored secret unreadable) and adds one when it is missing.
  `.env.example` documents the key and how to generate it.
- **Search reindex is admin-only in the handler too.** `POST /api/v1/search/reindex` now answers 403
  "Admin access required" to callers who are not admins, even if the route middleware is bypassed.
- **Admin status comes only from admin-group membership.** The template user map treated any agent
  whose login contained "admin" (or user id 1) as an admin, and helpers that build the current user
  fell back to user 1 with role `Admin` when the request carried no identity. Admin now comes only
  from the admin group (or the `Admin` role the auth layer derives from it); no identity means no
  user. The unreachable routing fallback `HandleAgentNewTicket` (with the same login heuristic) and
  the unused manifest-engine `checkAdmin` were deleted.
- **Admin → Groups: group names can no longer inject script.** The delete button put the group
  name into an inline JavaScript string and the "deleted" toast inserted it as HTML, so a group
  named e.g. `x');alert(1);('` ran script for the admin deleting it. The name is now read from a
  data attribute and the toast message is set as text.
- **Test/demo login shortcuts removed.** `handleHTMXLogin` (`DEMO_LOGIN_*` issued a user-1 JWT,
  `TEST_AUTH_*` returned the literal token `test-token`) and `handleDemoCustomerLogin` (fake
  customer cookie) had no route and are deleted. Demo mode (`app.demo_mode`) never logs anyone in.
- **Ticket create/update APIs honour role-based and admin permissions.** `PermissionService` read
  only `group_user`, so agents granted a queue through a role (`role_user` → `group_role`) or via
  the admin group got 403 from `POST/PUT /api/v1/tickets` although the queue middleware admitted
  them. It now uses the same effective permissions as the queue access middleware.
- **Public plugin UI rate limit enforced.** `UISpec.RateLimit` (requests per minute per client IP,
  60 when unset) is now applied to public (`auth: none`) plugin UIs; excess requests get 429.
- **Plugins can authorize event subscriptions.** `GKRegistration.EventAuthorizer` names a plugin
  function the host calls with the caller's identity and the channel before opening
  `GET /api/v1/plugins/:name/events/:channel`; only `{"allow": true}` subscribes. Plugins without
  one keep the previous behaviour (every agent may subscribe). See docs/plugins/AUTHOR_GUIDE.md.
- **Dashboard recent-tickets fragment escapes ticket data.** `/api/dashboard/recent-tickets`
  inserted ticket titles, numbers, customers and state/priority names into HTML unescaped, so a
  ticket title could carry markup or script into an agent's page; the dashboard-core widget also
  left the ticket-number link unescaped. Both now escape every value.
- **Every route now enforces who may call it, and a route-table test keeps it that way.**
  `TestRouteAuthorizationMatrix` (internal/api/route_authz_test.go) builds the production router
  (YAML routes, main-engine routes, built-in plugin routes) and sends every route requests as an
  anonymous caller, a customer (JWT and API token), a non-admin agent (JWT and API token): the
  classes the route excludes must get 401/403 or a login redirect. A new route without the right
  middleware fails CI. Gaps closed:
  - Customer logins reached agent routes: `auth`/`unified_auth` accepted customer JWTs and the
    queue/ticket permission middlewares looked up the customer's `customer_user.id` as an agent
    id (the id spaces overlap, so a customer could act with an agent's or the admin's queue
    permissions, change that agent's preferences/2FA, read every ticket via `/api/v1`). A new
    `agent` middleware now guards every agent route group, and `queue_*` / `ticket_access_*`
    refuse customers. Customers keep `GET /api/v1/tickets`, `/tickets/:id` and its articles
    (own tickets and customer-visible articles only; `customer_or_queue_ro`,
    `ticket_access_customer_ro`). MCP and `/customer/api/v1/tokens` (now `customer` only) follow.
  - The customer portal gate ran the route handler before checking the caller: any agent or
    anonymous request executed customer handlers (e.g. saved theme/wallpaper preferences) and only
    then got 403.
  - Every agent's unscoped API token counted as admin (`admin` middleware passed); a token is now
    admin only with the `admin:*` scope and an owner who is currently an admin.
  - `/admin/debug/config-sources` and `/admin/debug/ticket-number` had no authentication; they are
    now admin routes in `routes/admin.yaml`. `/api/v1/sse` (all plugin events) now needs a login.
  - Agents could change system configuration: ticket types (`/api/types`), salutations,
    signatures, system addresses (`/api/v1/...` POST/PUT), search reindex and the lookup cache are
    admin only. `/api/v1/queues/:id/stats` and `/api/tickets/:id/messages` check queue access.
  - `ticket_access_*` resolved a numeric path value as a ticket number first; when a ticket's tn
    equalled another ticket's id the check used one ticket and the handler changed the other. The
    permission is now required on every ticket the value can name. `queue_access_*` refuses
    requests whose path, query, form and JSON `queue_id` disagree.
  - `auth` routes passed `gf_` API tokens through with no identity; they now get 401.
  - Unrouted "TODO" stub handlers (`handleAgentSearch`, `…SearchResults`, `…CustomerTickets`,
    `…CustomerView`) and the `GET /tickets/api/search` stub route were removed.

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
