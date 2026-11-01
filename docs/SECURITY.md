# GoatFlow Security

This page lists the security controls that exist in GoatFlow today.
To report a vulnerability, see the root [SECURITY.md](../SECURITY.md).
Security fixes for each release are listed in [CHANGELOG.md](../CHANGELOG.md).

## Transport (TLS)

GoatFlow does not terminate TLS itself. Run it behind a reverse proxy
(Caddy, nginx, Traefik, a Kubernetes ingress) that serves HTTPS.

In production, authentication cookies are always set with `Secure`,
and use the configured SameSite policy.

## Sign-in

### Auth providers

Providers are registered in `internal/platform/auth/`. Choose them in
`Config.yaml`, or override with the `AUTH_PROVIDERS` environment variable.

| Provider | Name | Notes |
|----------|------|-------|
| Database | `database` | Agents (`users`) and customers (`customer_user`) |
| LDAP / Active Directory | `ldap` | See [LDAP.md](LDAP.md) |
| OpenID Connect | `oidc` | Configure in Admin -> Identity Providers |
| SAML 2.0 | `saml2` | Configure in Admin -> Identity Providers |
| GitHub | `github` | OAuth2 client |
| Google | `google` | OAuth2 client |
| Static | `static` | Fixed `user:password:Role` accounts, for testing |

### Password hashing

Code: `internal/platform/auth/password_hash.go`.

| Env var | Default | Meaning |
|---------|---------|---------|
| `PASSWORD_HASH_TYPE` | `bcrypt` | `bcrypt` or `sha256` (unsalted OTRS format, only for side-by-side running with OTRS). Unknown values fall back to bcrypt. |
| `MIGRATE_PASSWORD_HASHES` | `false` | When `true`, rehash a password with the configured type on successful login. |

- Login accepts bcrypt, salted sha256 and OTRS sha2 hashes, so imported OTRS users can sign in.
- bcrypt passwords longer than 72 bytes are refused.

### Password policy

The policy (length, character classes and so on) comes from the OTRS-style
sysconfig `PreferencesGroups###Password` settings.
Admins edit it at Admin -> Password Policy (`/admin/password-policy`).
It applies to admin user forms, profile password changes and password reset.

### Login rate limit

`internal/platform/auth/login_ratelimit.go` tracks failed logins by IP and
username: 5 failures in 300 seconds trigger a backoff that starts at 2 seconds
and doubles up to 60 seconds.

The same limiter, with separate counters, covers:

- second-factor codes on the 2FA login step, counted per login name, so logging
  in again with the right password does not give fresh guesses;
- every password re-check by a signed-in agent or customer (password change,
  2FA setup/confirm/disable, new recovery codes, adding or removing a passkey),
  counted per account and IP.

### Two-factor authentication

Agents and customers can turn on 2FA from their profile.

- **TOTP** (authenticator app).
- **Passkeys / security keys** (WebAuthn). Profile lists each key with when it
  was added and last used. Removing a key needs the password.
- **Recovery codes.** Shown once. "New recovery codes" replaces the set
  (`POST /api/preferences/2fa/recovery-codes`, `/customer/api/preferences/2fa/recovery-codes`).
  A passkey-only account can sign in with a recovery code.
- Removing the last second factor turns 2FA off and deletes the recovery codes.
- There is no password-only fallback on the 2FA step.

There is no SMS or email one-time code.

### Forgotten password and self-registration

- `/forgot-password` and `/customer/forgot-password` give the same answer
  whether or not the account exists.
- Reset links last one hour and work once. Tokens are stored only as SHA-256
  hashes. A newer request revokes older links. Setting a new password ends the
  account's open sessions.
- `/customer/register` (only with `features.registration: true`) sends a
  24-hour confirmation link.
- The forgot-password, reset-password and registration forms share one budget
  of 10 posts per IP per hour. Each recipient gets at most 3 emails per hour.
- Switches: `features.lost_password` (on by default) and `features.registration` (off by default).

### API tokens and JWT

- `POST /api/v1/auth/refresh` swaps a refresh token for a new access token and a
  new refresh token. The account is reloaded, so disabled accounts are refused.
- An API token counts as admin only when it has the `admin:*` scope and its
  owner is currently in the admin group.
- `gf_` API tokens on `auth` routes without an identity get 401.

## Authorization

- **Admin** comes only from membership of the `admin` group (or the `Admin`
  role derived from it). No identity means no user.
- **Agent routes** are guarded by the `agent` middleware. Customer logins are refused.
- **Queue and ticket access** uses the same effective permissions everywhere:
  `group_user`, plus roles (`role_user` -> `group_role`). `ticket_access_*` and
  `queue_access_*` middlewares refuse customers and requests whose IDs disagree.
- **Customers** see only their own tickets and customer-visible articles.
- **Route authorization matrix test.** `TestRouteAuthorizationMatrix`
  (`internal/api/route_authz_test.go`) builds the production router and calls
  every route as anonymous, customer and non-admin agent. A new route without
  the right middleware fails CI.
- **Plugins.** Public (`auth: none`) plugin UIs are rate limited
  (`UISpec.RateLimit`, 60 requests per minute per IP when unset; excess gets 429).
  A plugin can set `EventAuthorizer` to decide who may subscribe to its
  event channels. See [plugins/AUTHOR_GUIDE.md](plugins/AUTHOR_GUIDE.md).
- **Plugin limits.** Every plugin has a memory limit and call and start-up
  timeouts; gRPC plugin processes also have a CPU limit and can't open network
  connections without the `network` permission (seccomp). Memory and CPU are
  hard cgroup limits where the host delegates a cgroup, otherwise a heap limit,
  `GOMAXPROCS` and lower priority. Plugin HTTP requests through the host reach
  internal addresses only when the plugin's scope names the host, never cloud
  metadata, with at most 5 checked redirects and 32 MiB per response. Admins
  view and change limits at Admin -> Plugins -> Limits. gRPC plugin processes
  run in a Landlock sandbox: they read only their own directory and system
  paths, write only their temp directory, have no `/proc` (other processes are
  invisible), can't read GoatFlow's files, memory or environment, and on Linux
  6.12 or later can't signal other processes; the Limits page says which parts
  the kernel enforces. `goats` marks itself non-dumpable, so same-user
  processes can't read its `/proc` files either.
  Details: [PLUGIN_PLATFORM.md](PLUGIN_PLATFORM.md#os-level-process-isolation-grpc).
- **Plugin signatures.** Every gRPC binary and WASM module must be signed by a
  trusted publisher (the built-in GoatKit key, plus `GOATFLOW_TRUSTED_KEYS`),
  checked before an install or upload replaces anything and on every start.
  The plugins bundled in release images are signed. `GOATFLOW_REQUIRE_SIGNATURES=0`
  turns this off for development systems only. Details:
  [PLUGIN_PLATFORM.md](PLUGIN_PLATFORM.md#plugin-signing).

There is no attribute-based (ABAC) policy engine.

## Data

- No application-level field encryption. Use database or disk encryption if needed.
- Webhook signing secrets are write-only in the admin UI.

## Input handling

### SQL

All SQL uses `?` placeholders and goes through the conversion layer, which
rewrites them for PostgreSQL. `$1`-style placeholders make
`database.ConvertPlaceholders` panic, and the gk-lint rules catch other
MySQL-only or PostgreSQL-only SQL.

```go
query := database.ConvertPlaceholders(
    "SELECT id, tn, title FROM ticket WHERE ticket_state_id = ?")
rows, err := db.QueryContext(ctx, query, stateID)
```

See [development/DATABASE_ACCESS_PATTERNS.md](development/DATABASE_ACCESS_PATTERNS.md).

### HTML

User HTML (articles, Markdown) is sanitised on the server with bluemonday.
Templates escape output by default.

## Security headers

Set on every response by `internal/platform/middleware/security_headers.go`:

| Header | Value |
|--------|-------|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: *.giphy.com *.tenor.com media.tenor.com; media-src 'self' blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'` |
| `X-Frame-Options` | `DENY` |
| `X-Content-Type-Options` | `nosniff` |
| `X-XSS-Protection` | `1; mode=block` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `interest-cohort=()` |

`unsafe-inline` and `unsafe-eval` are needed by Alpine.js and HTMX.
XSS protection comes from server-side sanitising, not from CSP.

GoatFlow does not set `Strict-Transport-Security`. Set it at your proxy.

## Health and metrics endpoints

- `GET /health` is public and returns only status and short version.
- `GET /health/detailed` and `GET /metrics` on the app port need an admin login.
- Prometheus should scrape the standalone listener on `METRICS_PORT`
  (default 9090). It has no auth: keep it on an internal network.

## Audit

Security-relevant events are written to the `gk_audit_event` table (`internal/platform/audit`), and
each one is also written to the application log as an `audit` line for log shippers. Secrets
(passwords, tokens, keys, client secrets, TOTP seeds, recovery codes) are never recorded: changed
secret fields show as `[redacted]`.

| Category | Recorded |
|----------|----------|
| `auth` | Sign-in success and failure for every method (password, LDAP, OIDC, SAML, passkey) with the reason for a failure; rate-limited attempts; the 2FA step (success, failure, lock, recovery code use, IP mismatch); sign-out; 2FA and passkey changes; password changes, reset requests and resets; API tokens created and revoked |
| `admin` | Every write through an admin page or admin API, and every refused admin request from a signed-in user. Users, groups, roles, permissions, queues, customer users and companies, organisations, identity providers, webhooks, plugins, PostMaster filters, the decision filter and every dynamic module (System Configuration, mail accounts and more) record the fields that changed. Admin password resets, admin 2FA overrides and ended sessions are recorded with their target |
| `access` | Off by default. When "Record read access" is on: ticket views (agent and customer), attachment downloads and statistics exports. Audit log CSV exports are always recorded |
| `system` | Retention purges |

Admin -> Audit Log (`/admin/audit-log`) lists events with filters and exports them as CSV
(at most 100,000 rows; cells that a spreadsheet would run as a formula are prefixed with `'`).
Events are kept for 365 days by default and purged daily by the runner; set the period, or 0 to
keep them forever, on the same page. Ticket changes are also kept in each ticket's history, and
deletions in the deletion log. The log is an ordinary database table: an administrator with
database access can change it. Tamper evidence is for the goatkit-audit plugin.

## Containers

Every container in the compose files (`docker-compose.yml`, `deploy/docker-compose.yml`, the test
files) and the Helm chart runs as a non-root user, and so do the containers the Makefile and CI
start:

| Container | User |
|-----------|------|
| GoatFlow backend, customer-fe, runner, toolbox | 1000 (`appuser`) |
| MariaDB | 999 |
| PostgreSQL (alpine) | 70 |
| Valkey | 999 compose, 1000 Helm subchart |
| Caddy, smtp4dev | 1000; rebuilt from the upstream image with their data directories owned by 1000 |
| Makefile tools (Trivy, Gitleaks, Dive, curl) | the calling user; container-socket tools add its group |

Helm pods also have a read-only root filesystem, no privilege escalation and all capabilities
dropped. `goats` (backend, runner and customer-fe) refuses to start as root (uid 0). The one
exception is the TrueNAS app, whose TrueNAS permissions helper runs as root (without network) to
set dataset ownership, as in every TrueNAS catalog app.

## Release artifacts

Release builds (`.github/workflows/build.yml`) publish the main and runner images for linux/amd64
and linux/arm64 with an SBOM, and sign the images keylessly with Sigstore cosign. The signing
identity is the workflow at the release tag. The images, the Helm chart (in the OCI registry and as
the `.tgz` release asset) and the air-gapped bundle each get a signed SLSA v1.0 build provenance
attestation:

```bash
cosign verify ghcr.io/goatkit/goatflow:<version> \
  --certificate-identity=https://github.com/goatkit/goatflow/.github/workflows/build.yml@refs/tags/v<version> \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/goatkit/goatflow:<version> --repo goatkit/goatflow
gh attestation verify oci://ghcr.io/goatkit/charts/goatflow:<version> --repo goatkit/goatflow
gh attestation verify goatflow-airgap-container-bundle-<version>.tar.gz --repo goatkit/goatflow
```

The provenance is generated by GitHub on GitHub-hosted runners, which GitHub documents as meeting
SLSA v1.0 Build Level 2. First-party plugin releases (goat-kb, goat-kanban) get the same
attestation for their `.zip`, besides the ed25519 signature the marketplace checks.

Up to and including 0.10.0, only the images had a provenance attestation, and none had a cosign
signature. The air-gapped bundle contains third-party images (databases, Valkey); its attestation
records how the bundle was assembled, not how those images were built.

## CI security checks

From `.github/workflows/test.yml`:

| Job | Tool |
|-----|------|
| Secret Scanning | Gitleaks |
| Security Analysis | gosec (fails the build on any finding; same pinned version and flags as `make gosec`; JSON report uploaded; false positives carry a `// #nosec G<rule> -- <reason>` annotation), `go vet` |
| Tests | Route authorization matrix test, platform boundary lint |

Outside the repository's workflows, Semgrep managed scans cover the repository: Semgrep Code
(static analysis, Pro and community rules) and Semgrep Supply Chain (`go.mod`, `sdk/go/go.mod` and
`bun.lock`, with reachability analysis and malicious-package rules).

Dependabot alerts are on. `.github/dependabot.yml` opens weekly update PRs against `dev` for Go
modules (`go.mod`, `sdk/go/go.mod`), frontend packages (`bun.lock`), GitHub Actions, Dockerfile and
Compose base images and the Helm chart's dependencies. Minor and patch updates are grouped into one
PR per ecosystem; major updates come one PR each. Nothing merges automatically: each PR runs CI and
is reviewed. The Go toolchain version is updated by hand.

## Security changes in 0.10.0

Full details: the `0.10.0` section of [CHANGELOG.md](../CHANGELOG.md), "Security" heading.

- Route authorization matrix test; new `agent` middleware; customers can no longer reach agent routes.
- Admin status only from admin-group membership.
- API tokens are admin only with `admin:*` scope and an admin owner.
- Unauthenticated debug routes moved to admin; `/api/v1/sse` needs a login.
- Forgotten-password reset with hashed, single-use tokens and rate limits.
- Refresh tokens rotate on use.
- Recovery codes for passkey users; disabling 2FA now also removes passkeys.
- Public plugin UI rate limit enforced; plugin `EventAuthorizer`.
- Test/demo login shortcuts removed.
- XSS fixes: group names in Admin -> Groups, dashboard recent tickets, attachment filenames.
- Internal notes no longer shown to customers.
- `/health/detailed` and `/metrics` admin-gated.
