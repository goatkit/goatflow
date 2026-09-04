# GoatFlow Packaging Constraints Audit

Audit date: 2026-09-02 · Branch: dev @ 13d91c8d

## 1. Service topology

GoatFlow is **not** a single container — it is one application image running
multiple roles, plus supporting data services.

| Role | Image | Command | Notes |
|---|---|---|---|
| `backend` | `ghcr.io/goatkit/goatflow` | `./goats` | Main HTMX server (agent UI + customer portal + REST API). Migrations run pre-start via `./migrate` in the compose command. |
| `customer-fe` | same image | `./goats` with `CUSTOMER_FE_ONLY=true` | Dedicated customer-facing instance; scheduler disabled, admin routes guarded. Optional. |
| `runner` | same image | `./goats -mode runner` | Background task processor (email queue, session cleanup). Same DB; **must** have DB. |
| `mariadb` | `mariadb:11` | — | Default DB (OTRS-compatible). Postgres 15 supported via `DB_DRIVER=postgres`. |
| `valkey` | `valkey/valkey:7-alpine` | — | Redis-compatible cache; app degrades gracefully if absent. |
| `smtp4dev` | `rnwood/smtp4dev` | — | Dev-only mail sandbox. |
| `caddy` | `caddy:2-alpine` | — | Prod-only reverse proxy with ACME TLS (docker-compose.prod.yml). |

The application itself is a **single static binary** (`goats`) — templates
(Pongo2), static assets, YAML routes, migrations and WASM plugins are all
shipped inside the image. No Node/webpack frontend service; frontend is
server-rendered HTMX. Two modes via `-mode` flag: `server` (default) and
`runner`.

## 2. Ports

| Service | Container port | Default host mapping |
|---|---|---|
| backend | 8080 (`APP_PORT`) | 8081 (`BACKEND_PORT`) |
| customer-fe | 8080 | 8083 |
| backend-test | 8080 | 8082 (profile `testdb`) |
| customer-fe-test | 8080 | 8084 (profile `testdb`) |
| valkey | 6379 | 6388 |
| mariadb | 3306 | 3306 |
| smtp4dev | 25 / 143 / 110 / 80 | 1025 / 1143 / 1110 / 8025 |
| caddy (prod) | 80 / 443 | 80 / 443 (+443/udp) |
| adminer | 8080 | 8090 (profile `tools`) |

Optional metrics port: `METRICS_PORT=9090` is declared in `.env.example`
(Prometheus client is in go.mod) but is not wired into the compose file.

## 3. Environment variables

Required (no usable default):
- `JWT_SECRET` — mandatory; blank values fail auth
- `DB_MYSQL_*` / `DB_PGSQL_*` credentials — DB_DRIVER selects mariadb (default) or postgres
- `GOATFLOW_ADMIN_PASSWORD` — for initial admin bootstrap (db-init targets)

Key configuration (with defaults):
- `APP_ENV` (development|test|production), `APP_PORT` (8080), `GOATFLOW_SECURE_KEY`
- `DB_DRIVER` (mariadb), `DB_MAX_CONNECTIONS=100`, `DB_MAX_IDLE_CONNECTIONS=10`, `DB_CONNECTION_MAX_LIFETIME=30m`
- `VALKEY_HOST`/`VALKEY_PORT`/`VALKEY_PASSWORD`/`VALKEY_DB` (Redis aliases `REDIS_*` also accepted)
- `STORAGE_PATH` (/app/storage), `MAX_UPLOAD_SIZE` (10MB), `ALLOWED_FILE_TYPES`
- `ENABLE_YAML_ROUTING` (true), `ROUTES_DIR` (/app/routes), `CONFIG_DIR` (/app/config), `PLUGIN_DIR` (<CONFIG_DIR>/plugins), `TEMPLATES_DIR`
- `PASSWORD_HASH_TYPE` (sha256, OTRS-compatible), `MIGRATE_PASSWORD_HASHES`
- `GOATFLOW_PLUGIN_LAZY_LOAD` (true), `GOATFLOW_PLUGIN_HOT_RELOAD` (false in prod image), `GOATFLOW_PLUGIN_HEALTH_CHECK`, `GOATFLOW_PLUGIN_AUTO_RESTART`; per-plugin config via `GOATFLOW_PLUGIN_<NAME>_<KEY>`
- Runner: `GOATFLOW_EMAIL_SMTP_*`, `GOATFLOW_EMAIL_FROM`, `GOATFLOW_EMAIL_ENABLED`
- TLS: `TLS_CERT_FILE`, `TLS_KEY_FILE` (compose auto-generates self-signed certs if absent)
- Identity: OIDC (coreos/go-oidc), SAML (crewjam/saml), LDAP, WebAuthn support — all optional, config-driven

## 4. Persistent data

| Path / volume | Content | Persistence requirement |
|---|---|---|
| `./storage` → `/app/storage` | Ticket attachments (local storage backend; DB-backed or S3 optional via `STORAGE_BACKEND`) | **Must persist** |
| `goatflow_certs` volume → `/app/certs` | Self-signed TLS certs | Persist or accept regeneration |
| `goatflow_plugins` volume → `/app/config/plugins` | Uploaded WASM plugins | Persist; **needs chown fix** (plugin-init sidecar, UID 1000:1000) |
| `mariadb_data` / `postgres_data` | Database | Must persist |
| `valkey_data` | Cache | Optional |

Config files read at runtime (inside image, overridable via mounts):
`config/default.yaml`, optional `config.yaml` override,
`email_dispatch.yaml`, `external_ticket_rules.yaml`, `lookups/`,
`routes/*.yaml`, `templates/`, `static/`, `migrations/{mysql,postgres}/`.
UID/GID of the app user is build-arg-configurable (`UID`/`GID`, default 1000)
so host bind mounts work without root.

## 5. Health endpoints

- Container-level: `HEALTHCHECK` in Dockerfile — `wget --spider http://localhost:8080/health`, 30s interval, 5s start period.
- App endpoints: `GET /health`, `GET /health/detailed`, `GET /healthz`, `GET /metrics`.
- Runner has **no HTTP endpoint** — compose healthcheck greps `/proc/1/cmdline` for `-mode runner`.

## 6. Resource usage

- Runtime base: `alpine:3.19` + curl, ca-certificates, postgresql15-client,
  poppler-utils (PDF thumbnails), vips + vips-heif (image processing), tzdata.
  Runs as non-root `appuser` (UID 1000). Static binary + assets; order of tens
  of MB. No CPU/memory limits are set anywhere in compose or Helm chart —
  packagers should add them.
- Build requirements: Go 1.25.12 (alpine, CGO, vips-dev), TinyGo 0.32 (WASM
  plugin build), bun 1.3 + node (tailwind/esbuild frontend build), network
  access to unpkg/jsdelivr/fontawesome CDN for asset download.
- DB defaults: 100 max connections. Multipart buffer: 128MB in-memory cap.

## 7. License

Apache-2.0 (`LICENSE`, `package.json`).
~~**Discrepancy to flag:** the Swagger annotation in `cmd/goats/main.go`
declares `AGPL-3.0` — stale doc comment, should be aligned to Apache-2.0.~~
**Resolved (2026-09-02):** annotation in `cmd/goats/main.go` and all OpenAPI /
Swagger specs now declare Apache-2.0; see CHANGELOG.md [Unreleased] → Changed.

## 8. Versioning scheme

- Binary version injected at build time via ldflags:
  `internal/version.Version` (default `0.10.0` in
  `internal/platform/version/version.go`), plus `GitCommit`, `GitBranch`, `BuildDate`.
- Container image: `ghcr.io/goatkit/goatflow:${GOATFLOW_VERSION:-latest}`;
  dev build default `VERSION=dev`.
- Helm chart `charts/goatflow` now tracks the app: `appVersion: 0.10.0`,
  `kubeVersion: ">=1.25.0"` (was stale: `version 0.1.0` with appVersion "1.0.0"
  and no kubeVersion gate). Chart release `version` stays 0.1.0 — bump it to
  1.0.0 (or 0.2.0) when the first chart release ships.
- `package.json` version 0.1.0 is frontend-tooling only, not the app version.

## 9. Upgrade behavior

- **Schema migrations are automatic and idempotent.** Both paths exist and are
  safe: the compose entrypoint runs `./migrate ... up` (golang-migrate, with
  `.down.sql` files) and the app itself runs `database.RunMigrations()` on
  startup (IF NOT EXISTS patterns; failures are non-fatal warnings).
- Upgrade = swap image tag; state lives in named volumes/bind mounts
  (storage, plugins, certs, DB), none of which is baked into the image.
- Plugins live in a named volume: after an image upgrade they may be owned by
  the old UID — the `plugin-init` sidecar (root chown to HOST_UID:HOST_GID)
  runs before backend start. Keep it in any upgraded stack.
- Downgrade path exists (down migrations) but is untested for rollback.
- `PASSWORD_HASH_TYPE` default `sha256` preserves OTRS-compatible hashes;
  switching to argon2/bcrypt requires `MIGRATE_PASSWORD_HASHES=true` backfill.

## 10. Packaging constraints summary

1. One image, multiple roles (`server` default, `-mode runner`,
   `CUSTOMER_FE_ONLY=true`) — packagers can scale roles independently.
2. Hard dependencies: DB (MariaDB/Postgres) is fatal for runner, tolerable for
   server (degraded); Valkey is soft; SMTP is optional.
3. Secrets: `JWT_SECRET`, `GOATFLOW_SECURE_KEY`, DB creds, SMTP creds —
   none have safe defaults.
4. Persistence: attachments, plugins, certs, DB. Four named volumes + one
   bind mount in the reference stack.
5. Non-root (UID/GID build args) with a root one-shot sidecar for plugin
   volume ownership.
6. Health: HTTP `/health` for server roles; cmdline-grep healthcheck for
   runner (no port exposed).
7. No resource limits defined — set `deploy.resources` / compose limits.
8. License: Apache-2.0 (fix stale AGPL annotation in main.go).
9. Build is network-dependent (CDN asset downloads) — consider vendoring
   assets for air-gapped builds.
10. Migrations auto-run on every start — safe to `docker compose pull && up`
    for upgrades; no manual migration step needed.
