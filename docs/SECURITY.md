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
- Both forms allow 10 posts per IP per hour and 3 emails per recipient per hour.
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

- 2FA events are logged (`internal/platform/auth/totp_audit.go`).
- Admin changes to agents, groups and roles record who made them.

## CI security checks

From `.github/workflows/test.yml`:

| Job | Tool |
|-----|------|
| Secret Scanning | Gitleaks |
| Security Analysis | gosec (report uploaded, does not fail the build), `go vet` |
| Tests | Route authorization matrix test, platform boundary lint |

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
