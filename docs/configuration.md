# Configuration

This page explains where GoatFlow reads its settings from, which settings win, and which environment variables matter in 0.10.0.

## Where settings come from

GoatFlow reads settings from five places.

| Source | What it holds | How to change it |
|--------|---------------|------------------|
| `config/default.yaml` | Operational defaults (server, email, storage, features, maintenance, ...). Shipped with the image. No secrets. | Do not edit. Override it (see below). |
| `config/config.yaml` | Your local overrides of `default.yaml`. Optional and gitignored. | Create the file next to `default.yaml`. |
| Environment variables | Secrets, deployment values, and overrides of any `default.yaml` key. | Set them in `.env`, docker-compose, or Helm `backend.extraEnv`. |
| `config/Config.yaml` | The SysConfig registry: named settings such as `SystemID`, `Ticket::NumberGenerator` and `Auth::Providers`, each with a `default` and an optional `value`. | Edit the `value` and restart. |
| Database (`sysconfig_default` / `sysconfig_modified`) | OTRS-style settings changed in the admin UI or imported from OTRS. | Admin pages, for example the customer portal settings at `/admin/customer/portal/settings`. |

The config directory is `/app/config` in the container. Change it with `CONFIG_DIR`.

## `default.yaml`, `config.yaml` and `GOATFLOW_*` variables

The loader (`internal/platform/config/config.go`) works like this:

1. Read `default.yaml` from the config directory.
2. Merge `config.yaml` from the same directory, if it exists.
3. Apply environment variables named `GOATFLOW_<SECTION>_<KEY>`.

The variable name is the YAML path in upper case, with `.` replaced by `_`. Examples:

| YAML key | Environment variable |
|----------|---------------------|
| `features.registration` | `GOATFLOW_FEATURES_REGISTRATION=true` |
| `features.lost_password` | `GOATFLOW_FEATURES_LOST_PASSWORD=false` |
| `email.smtp.host` | `GOATFLOW_EMAIL_SMTP_HOST=mail.example.com` |
| `valkey.host` | `GOATFLOW_VALKEY_HOST=valkey.internal` |
| `app.timezone` | `GOATFLOW_APP_TIMEZONE=Europe/London` |

Rules:

- Only keys that exist in `default.yaml` (or in your `config.yaml`) can be overridden this way.
- `config.yaml` is plain YAML. `${VAR}` is **not** expanded. Use a `GOATFLOW_*` variable for secrets instead.
- Some values are not read from `default.yaml` at all. They come only from their own environment variables (see the tables below).

Example `config/config.yaml`:

```yaml
app:
  timezone: Europe/London
features:
  registration: true
```

### `default.yaml` sections

| Section | Read by GoatFlow 0.10.0? | Notes |
|---------|--------------------------|-------|
| `app` | Yes: `env`, `timezone`, `demo_mode`, `name` | `app.env` defaults to `development`. |
| `server` | Partly | The listen port is `APP_PORT` (default `8080`), not `server.port`. |
| `database` | Removed | The connection comes from `DB_*` variables only. See [Database](#database). A `database:` block in an old `config.yaml` is ignored. |
| `valkey` | Yes | Cache host, port, password, pool and TTL. |
| `auth` | Yes: `jwt.*`, `session.*` | `JWT_SECRET` and the `JWT_*_EXPIRY` variables win over `auth.jwt.*`. |
| `email` | Yes | SMTP sending (`email.enabled` and `email.smtp.host` must be set) and inbound mail polling. |
| `storage` | Yes | `type`, `local.path`, `attachments.max_size` (10 MiB), `attachments.allowed_types`. |
| `ticket` | Yes: `frontend.agent_ticket_note.required_time_units`, `bulk_actions.max_select_all`, `service.default_unknown_customer` | |
| `features` | Yes: `registration`, `lost_password` | The only two keys. |
| `maintenance` | Yes: `time_notify_upcoming_minutes`, `default_notify_message` | |
| `runner` | Yes: `session_cleanup.interval` | |

0.10.0 removed the `logging`, `metrics`, `rate_limiting`, `integrations` and `database` sections and the unused `features.*` keys (`social_login`, `two_factor_auth`, `api_keys`, `ldap`, `saml`, `knowledge_base`, `customer_portal`, `agent_collision_detection`): nothing read them. Logging is set by `LOG_*` and metrics by `METRICS_*` variables (below). Leftover keys in your `config.yaml` are ignored.

## Settings table (0.10.0)

Defaults below are the values in the code or in `config/default.yaml`.

### Public URL and self-service

| Setting | Default | What it does |
|---------|---------|--------------|
| `BASE_URL` | unset | Public URL of this instance, e.g. `https://helpdesk.example.com`. Password-reset and sign-up emails link to it. While it is unset (or not an absolute `http(s)` URL) those emails are **not sent**. The request Host header is never used. |
| `APP_URL` | `http://localhost:8080` | docker-compose only: compose passes `APP_URL` into the backend as `BASE_URL`. The Helm chart uses `config.baseUrl`. |
| `features.lost_password` | `true` | "Forgot password" for agents (`/forgot-password`) and customers (`/customer/forgot-password`). |
| `features.registration` | `false` | Customer self-registration at `/customer/register`, with a 24-hour email confirmation link. |

### Database

| Setting | Default | What it does |
|---------|---------|--------------|
| `DB_DRIVER` | `mysql` | `mysql` or `mariadb` for MySQL/MariaDB, `postgres` for PostgreSQL. Use exactly `postgres`: other spellings are not recognised by the SQL layer. Oracle and SQL Server are not implemented. |
| `DB_MYSQL_HOST`, `_PORT`, `_NAME`, `_USER`, `_PASSWORD` | port `3306` | Connection when the driver is MySQL/MariaDB. |
| `DB_PGSQL_HOST`, `_PORT`, `_NAME`, `_USER`, `_PASSWORD`, `_SSLMODE` | port `5432`, sslmode `disable` | Connection when the driver is PostgreSQL. |
| `DB_HOST`, `DB_PORT`, ... | | Legacy names. Used only when the driver-specific variable is unset. |
| `DATABASE_URL` | unset | A full connection URL. Used instead of the variables above when set. |

GoatFlow runs its own database migrations at startup.

### Article storage

See [ARTICLE_STORAGE.md](ARTICLE_STORAGE.md) for the full guide.

| Setting | Default | What it does |
|---------|---------|--------------|
| `STORAGE_TYPE` / `storage.type` | `db` | `db` stores attachments in the database (OTRS ArticleStorageDB). `fs` stores them on disk in the OTRS ArticleStorageFS layout. The variable wins over the YAML key. |
| `STORAGE_PATH` / `storage.local.path` | `/app/storage` | Storage root. The `fs` backend uses `<STORAGE_PATH>/var/article`. |
| `storage.attachments.max_size` | `10485760` (10 MiB) | Upload size limit. |
| `storage.attachments.allowed_types` | images, PDF, text, Word, Excel | Allowed MIME types for uploads. |

Use the `goatflow-storage` command to move attachments between `db` and `fs`.

### Authentication

| Setting | Default | What it does |
|---------|---------|--------------|
| `AUTH_PROVIDERS` | unset | Comma-separated password login providers, tried in order: `database`, `ldap`, `static`. Example: `AUTH_PROVIDERS=ldap,database`. When unset, the `Auth::Providers` setting in `Config.yaml` is used, and then `database`. Read at startup. |
| `LDAP_ENABLED` and `LDAP_*` | `LDAP_ENABLED` off | LDAP / Active Directory agent login. All variables are in [LDAP.md](LDAP.md). Invalid `LDAP_*` values stop the server at startup. |
| `GOATFLOW_STATIC_USERS` | unset | Demo/test users for the `static` provider. See the README. |
| `JWT_SECRET` | unset | Key that signs login tokens. **Always set it** (32+ random characters, e.g. `openssl rand -hex 32`). `auth.jwt.secret` (or `GOATFLOW_AUTH_JWT_SECRET`) is used when it is unset; `default.yaml` ships no value. With `APP_ENV=production` the server refuses to start when the secret is missing, shorter than 32 characters or a published placeholder. In other environments a missing secret is replaced by a random one per process (tokens stop working after a restart). |
| `JWT_ACCESS_TOKEN_EXPIRY` | `auth.jwt.access_token_ttl` (`15m`) | Access token lifetime, e.g. `30m`, `4h`. |
| `JWT_REFRESH_TOKEN_EXPIRY` | `auth.jwt.refresh_token_ttl` (`168h`) | Refresh token lifetime. |
| `PASSWORD_HASH_TYPE` | `bcrypt` | Hash for new passwords: `bcrypt` or `sha256`. Unknown values fall back to `bcrypt` with a warning. Logins accept bcrypt, salted sha256 and the OTRS/Znuny formats whatever this is set to. |
| `MIGRATE_PASSWORD_HASHES` | `false` | When `true`, a password stored in another format is re-hashed with `PASSWORD_HASH_TYPE` after a successful login. |
| `GOATFLOW_ADMIN_PASSWORD` | unset | First boot only. If the seeded admin `root@localhost` is still in its factory-disabled state, GoatFlow sets this password and enables the account, then records `admin.bootstrap.applied` in `sysconfig_modified`. Later boots do nothing. Failures are logged and do not stop startup. |

### Passkeys (WebAuthn)

| Setting | Default | What it does |
|---------|---------|--------------|
| `GOATFLOW_WEBAUTHN_RP_ID` | host name of the request | Relying party ID. Passkeys are bound to it. Set it when users reach GoatFlow under more than one host name. |
| `GOATFLOW_WEBAUTHN_RP_NAME` | `GoatFlow` | Name shown by the browser or authenticator. |
| `GOATFLOW_WEBAUTHN_ORIGINS` | origin of the request | Comma-separated allowed origins, e.g. `https://helpdesk.example.com`. The request origin honours `X-Forwarded-Proto` and `X-Forwarded-Host`. |

### Secrets for stored settings

| Setting | Default | What it does |
|---------|---------|--------------|
| `GOATFLOW_SECURE_KEY` | generated at startup | AES-256 key (64 hex characters = 32 bytes) that encrypts stored secrets: plugin secure settings, webhook signing secrets and webhook custom header values. When unset, a random key is generated and logged with a warning, so secrets saved under it cannot be read after a restart. Set it in production, and give the same value to the backend and the runner. A value that is not 64 hex characters is rejected with an error. |

### Outbound webhooks

| Setting | Default | What it does |
|---------|---------|--------------|
| `GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS` | unset (deny) | `true` lets webhooks reach loopback, private, link-local and other internal addresses (for example on-premises services). Unset, such URLs are rejected when saved and every delivery refuses hosts that resolve to them. Set the same value on the backend and the runner. See [WEBHOOKS.md](WEBHOOKS.md#internal-and-private-addresses). |
| `GOATFLOW_WEBHOOK_DELIVERY_RETENTION_DAYS` | `30` | The runner deletes delivered and failed webhook deliveries older than this many days. `0` keeps them forever. |

### Logging

| Setting | Default | What it does |
|---------|---------|--------------|
| `LOG_FORMAT` | `text` | `text` or `json`. In `json` mode, old-style `log` lines are also written as JSON records. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Applies to structured (`slog`) lines. |
| `LOG_OUTPUT` | `stdout` | `stdout`, `stderr` or a file path. Missing directories are created. If the file cannot be opened, logs go to `stdout` and the reason is printed to stderr. |
| `LOG_FILE_PATH` | unset | Old name for the log file. Used only when `LOG_OUTPUT` is unset. |

### Metrics, health and shutdown

See [OBSERVABILITY.md](OBSERVABILITY.md) for details.

| Setting | Default | What it does |
|---------|---------|--------------|
| `METRICS_ENABLED` | off | `true` (or `1`) starts a separate Prometheus listener with no login. |
| `METRICS_PORT` | `9090` | Port of that listener. Keep it on the internal network. `/metrics` on the main port needs an admin login. |
| `DRAIN_TIMEOUT` | `5s` | On SIGTERM/SIGINT, how long the server lets in-flight requests finish before closing them, and how long the runner waits for its cancelled tasks. Positive Go duration (`5s`, `1m`). Anything else (for example a plain `30`) logs an error at startup and the default is used. |
| `APP_PORT` | `8080` | Main HTTP port. |
| `APP_ENV` | unset | `production` (or `prod`) turns on Secure cookies, Gin release mode and the startup secret check: the server exits when `JWT_SECRET` is missing, short or a placeholder, or when the database, `SESSION_SECRET` or `ZINC_PASSWORD` is the example value. `APP_ENV` always wins over `app.env` in the config files. The runner (`-mode runner`) does not run the check. |

## SysConfig settings (`Config.yaml`)

`config/Config.yaml` lists named settings with metadata. Each setting has a `default` and may have a `value`:

```yaml
settings:
  - name: Ticket::NumberGenerator
    group: Ticket
    description: Select ticket number generator implementation
    default: Date
    value: Increment
```

Effective value:

- If `value` is set (even to an empty string), it is used.
- Otherwise `default` is used.
- If neither exists, reading the setting returns an error.

At startup GoatFlow logs a warning if two settings have the same name. The first one wins.

## Settings stored in the database

Some features read OTRS-style settings from the `sysconfig_modified` table (newest valid row), falling back to `sysconfig_default`. Examples:

- Password policy: `PreferencesGroups###Password::*` for agents (shown at `/admin/password-policy`) and `CustomerPreferencesGroups###Password::*` for customers.
- Customer portal settings (`CustomerPortal::*`), edited at `/admin/customer/portal/settings` and on each company's Portal Settings tab.
- SLA escalation calendars, as in OTRS: `TimeWorkingHours`, `TimeVacationDays`, `TimeVacationDaysOneTime` (default calendar, read in `app.timezone`) and their `::Calendar1`..`::Calendar9` variants with `TimeZone::CalendarN` (a numbered calendar is used when its working hours are set and `TimeZone::CalendarNName` is not empty). A weekday missing from the working hours is not a working day. `OTRSEscalationEvents::DecayTime` (minutes, default 1440) sets how often an escalation event repeats.

`goatflow-migrate` copies OTRS's changed settings into `sysconfig_modified`. GoatFlow uses the ones whose names it reads, such as those above.

## Ticket number generators

Full guide: [ticket_number_generators.md](ticket_number_generators.md).

| Name | Core idea | When to use |
|------|-----------|-------------|
| Increment | Global counter | Audits, sortable numbers |
| Date | Date + daily counter | Grouping by day |
| DateChecksum | Date + checksum | Grouping by day with a light integrity check |
| Random | SystemID + 10 random digits | Hide ticket volume |

To switch:

1. In `config/Config.yaml`, set `value` of `Ticket::NumberGenerator` to the generator name.
2. Restart GoatFlow (`make restart` in the container workflow).
3. Check with `GET /admin/debug/ticket-number` (admin login required).

If the name is unknown, GoatFlow logs a warning and uses `DateChecksum`.

The Random generator retries up to 5 times when a new number collides with an existing one.

## Checking the effective configuration

Both endpoints need an admin login.

- `GET /admin/debug/ticket-number` - current generator and whether it is date based.
- `GET /admin/debug/config-sources` - every `Config.yaml` setting with its default, value, effective value and source (`value` or `default`).

## Safety notes

- Do not edit `default.yaml`. It is replaced on upgrade and must stay free of secrets.
- Put local changes in `config/config.yaml` or in `GOATFLOW_*` variables.
- Put secrets (`JWT_SECRET`, `GOATFLOW_SECURE_KEY`, database passwords) in environment variables, not in YAML files under version control.
