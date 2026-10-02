# GoatFlow Database

GoatFlow runs on **MySQL/MariaDB** or **PostgreSQL**. `DB_DRIVER` picks one (`mysql` or
`postgres`). Oracle and SQL Server are not implemented: they return
`ErrDatabaseNotImplemented`.

The schema starts from the OTRS 6 / Znuny schema. GoatFlow features that OTRS does not have use
their own tables, most of them named `gk_*`.

## Writing SQL

Write every query with `?` placeholders and pass it through the conversion layer
(`database.ConvertPlaceholders`, `database.ConvertQuery`, ...). The layer turns `?` into `$1`,
`$2`, ... on PostgreSQL. Never write `$1` yourself. `gk-lint` and the pre-commit hook reject SQL
that skips the layer.

```go
query := database.ConvertPlaceholders(
    "SELECT id, login FROM customer_user WHERE customer_id = ? AND valid_id = ?")
rows, err := db.QueryContext(ctx, query, customerID, 1)
```

The full rules are in [DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md).

## Migrations

Migrations live in two folders with the **same version numbers**:

- `migrations/mysql/`
- `migrations/postgres/`

Each version has an `.up.sql` and a `.down.sql` file. Both folders hold versions 000001 to
000029.

| Version | Name | What it does |
|---------|------|--------------|
| 000001 | `schema_alignment` | OTRS/Znuny base schema (126 tables) |
| 000002 | `minimal_data` | Essential lookup rows |
| 000003 | `customer_portal_sysconfig` | Customer portal settings in `sysconfig_default` |
| 000004 | `dynamic_field_screen_config` | `dynamic_field_screen_config` |
| 000005 | `canned_response` | `canned_response`, `canned_response_category` |
| 000006 | `znuny_color_columns` | `color` column on `ticket_priority` and `ticket_state` (as in Znuny) |
| 000007 | `api_tokens` | `user_api_tokens` |
| 000008 | `admin_audit_log` | `admin_action_type`, `admin_action_log` |
| 000009 | `custom_fields` | `gk_custom_field_def`, `gk_custom_field_value` |
| 000010 | `plugin_uis` | `gk_plugin_ui` |
| 000011 | `organisations` | `gk_organisation`, `gk_user_organisation`, `sysconfig_org` |
| 000012 | `secure_settings` | `gk_secure_config` |
| 000013 | `entity_deletion` | `gk_deletion_log`, `gk_recycle_bin` |
| 000014 | `self_service_auth` | `gk_auth_token`, `gk_registration_request` (password reset and customer sign-up) |
| 000015 | `push_subscriptions` | `gk_push_subscription` |
| 000016 | `escalation_calendar_defaults` | Default business calendar row in `sysconfig_default` |
| 000017 | `org_plugin_access` | `gk_org_plugin_access` |
| 000018 | `captive_plugin` | `captive_plugin` column on `gk_organisation` |
| 000019 | `service_worker_cache_config` | Service worker cache settings in `sysconfig_default` |
| 000020 | `webauthn_credentials` | `gk_webauthn_credential` (passkeys) |
| 000021 | `webauthn_ceremonies` | `gk_webauthn_ceremony` |
| 000022 | `totp_pending_sessions` | `gk_totp_pending_session` |
| 000023 | `identity_providers` | `gk_identity_provider`, `gk_identity_provider_org` (OIDC/SAML) |
| 000024 | `user_table_for_idp_routing` | `user_table` column on `gk_identity_provider` |
| 000025 | `saml_fields` | SAML columns on `gk_identity_provider` |
| 000026 | `setup_assistant_sysconfig` | `setup.assistant.completed` flag in `sysconfig_default` |
| 000027 | `postgres_mysql_parity` | PostgreSQL only: matches MySQL column types, defaults and keys, and adds the missing foreign keys. No change on MySQL. |
| 000028 | `webhooks` | `gk_webhook`, `gk_webhook_delivery`, `gk_webhook_event_cursor` |
| 000029 | `otrs_default_lookups` | OTRS/Znuny default lookup rows a fresh install lacked (ticket states, lock types, history types, link types, auto response types) |

To check that both folders still match:

```bash
diff <(ls migrations/mysql/*.up.sql | xargs -n1 basename) \
     <(ls migrations/postgres/*.up.sql | xargs -n1 basename)
```

### Running migrations

The backend applies all pending migrations when it starts (`cmd/goats/main.go` calls
`database.RunMigrations`). It picks `migrations/mysql` or `migrations/postgres` from
`DB_DRIVER`. A migration error is logged and does not stop the server.

| Command | What it does |
|---------|--------------|
| `make db-migrate` | Runs `migrate up` inside the `backend` container, for `DB_DRIVER`. |
| `make gen-migration` | Asks for a name and creates an empty `<timestamp>_<name>` up/down pair in the folder for `DB_DRIVER`. Rename both files to the next six-digit version, and add the same version to the other folder. |

`make db-status` and `make db-rollback` always use the PostgreSQL URL and `migrations/postgres`.

### Adding a migration

1. Add `NNNNNN_name.up.sql` and `NNNNNN_name.down.sql` to **both** `migrations/mysql` and
   `migrations/postgres`, with the next free number.
2. Do not change OTRS tables. Add a new `gk_*` table instead (see
   [SCHEMA_FREEZE.md](../architecture/SCHEMA_FREEZE.md)).
3. Run the Go tests on both databases (see [TESTING.md](TESTING.md#running-tests-on-postgresql)).
   The test databases apply every migration when they start, so a broken migration shows up
   there.

`gk-lint` reads `CREATE TABLE` and `ALTER TABLE` from both folders. SQL that names a table or
column that does not exist on both drivers fails the lint.

## OTRS schema rules

The OTRS tables are frozen. See [SCHEMA_FREEZE.md](../architecture/SCHEMA_FREEZE.md).

- Keep the OTRS names: `ticket`, `article`, `users`, `customer_user`, `customer_company`,
  `queue`, `ticket_state`, ... (not `tickets`).
- Keep the OTRS column names, for example `pw` (not `password`), `tn` for the ticket number.
- Primary keys are integers (`SERIAL`/`BIGSERIAL` on PostgreSQL, `AUTO_INCREMENT` on MySQL).
  `customer_company` is keyed by its `customer_id` string.
- `valid_id`: 1 = valid, 2 = invalid, 3 = invalid-temporarily. "Delete" in the admin UI
  usually sets `valid_id = 2`.
- Most tables have `create_time`, `create_by`, `change_time`, `change_by`.
- Some times are Unix timestamps in integer columns, for example `article_data_mime.incoming_time`.

The real definitions are in `migrations/mysql/000001_schema_alignment.up.sql` and
`migrations/postgres/000001_schema_alignment.up.sql`. Read them, not a copy.

Some GoatFlow features use OTRS tables and need no new table:

| Feature | Tables |
|---------|--------|
| TOTP two-factor login | `user_preferences` / `customer_preferences` (for example `UserTOTPEnabled`) |
| SLAs | `sla`, `sla_preferences`, `service_sla` |
| Settings | `sysconfig_default`, `sysconfig_modified` |

## Working with the dev database

The dev stack in `docker-compose.yml` runs **MariaDB** (`mariadb` service, volume
`mariadb_data`). Its `postgres` service is commented out, so the dev targets below only work
with `DB_DRIVER=postgres` if you add a `postgres` service yourself. PostgreSQL is tested
through the `postgres-test` container (see [Test databases](#test-databases)).

| Command | What it does |
|---------|--------------|
| `make db-query QUERY="SELECT 1"` | Runs one query against the dev database for `DB_DRIVER`. `QUERY_FILE=path` or stdin also work. |
| `make db-shell` | Interactive shell: the `mariadb` client in the `mariadb` container, or `psql` in a `postgres` container when `DB_DRIVER=postgres`. |
| `make db-shell-test` | Shell on the test database for `TEST_DB_DRIVER` (starts it first). |
| `make db-query-test` | Query against the test database. |

### Passwords

Migration 000002 creates `root@localhost` disabled (`valid_id = 2`) with a random password.
Set a password and enable it with:

| Command | Database |
|---------|----------|
| `make reset-password` | Dev database (`DB_DRIVER`) |
| `make test-pg-reset-password` | PostgreSQL test database |
| `make test-mysql-reset-password` | MariaDB test database |

These run `scripts/reset-user-password.sh`. It calls `scripts/db/postgres/reset-user-password.sh`
or `scripts/db/mysql/reset-user-password.sh`, which run the `goats reset-user` command in the
toolbox container.

When the backend container gets `GOATFLOW_ADMIN_PASSWORD`, it enables `root@localhost` with
that password on first boot (see [docs/configuration.md](../configuration.md)). The dev
`docker-compose.yml` does not pass this variable to the backend, so use `make reset-password`
in dev.

### Generated test credentials

```bash
make synthesize        # writes .env secrets (only when .env does not exist yet) and test_credentials.csv
make show-dev-creds    # prints the generated users
```

When it creates a new `.env`, `make synthesize` also writes
`migrations/postgres/000004_generated_test_data.up.sql` (gitignored). This file is not a real
migration. `make db-apply-test-data` loads it on PostgreSQL. On MariaDB, `db-apply-test-data`
only enables `root@localhost` when `ADMIN_PASSWORD` is set.

**Delete that file before you run PostgreSQL migrations or start `postgres-test`.** Version
000004 already exists (`000004_dynamic_field_screen_config`), and golang-migrate stops with
"duplicate migration file". The `postgres-test` init script applies every `*.up.sql` in the
folder, so it would load the file too.

### Baseline loaders

`make db-init` and `make db-reset` come from before the numbered migrations:

- With `DB_DRIVER=postgres`, `make db-init` drops the `public` schema and loads
  `schema/baseline/otrs_complete.sql` and `schema/baseline/required_lookups.sql`. This does not
  create the GoatFlow tables and does not record a migration version.
- With MariaDB, `make db-init` only starts `mariadb` and enables `root@localhost` with
  `GOATFLOW_ADMIN_PASSWORD`.
- `make db-reset` runs `db-init` and then loads `schema/seed/minimal.sql` into the `postgres`
  container.

To get a clean, fully migrated MariaDB dev database: `make down`, remove the `mariadb_data`
volume, then `make up`. The backend applies all migrations at start-up. Then set the admin
password with `make reset-password`.

## Test databases

The test databases are defined in `docker-compose.testdb.yml` (profile `testdb`):

| Service | Driver | Host port |
|---------|--------|-----------|
| `mariadb-test` | MariaDB 11 | `TEST_DB_MYSQL_PORT` |
| `postgres-test` | PostgreSQL 15 | `TEST_DB_POSTGRES_PORT` |

Start one with `make test-db-up` (MariaDB by default) or
`make test-db-up TEST_DB_DRIVER=postgres`.

Their data is on tmpfs, so each container start begins empty. The container init scripts run
in this order:

| Step | MariaDB (`mariadb-test`) | PostgreSQL (`postgres-test`) |
|------|--------------------------|------------------------------|
| 01 | | `docker/postgres/01-init-databases.sql` |
| 10 | `docker/mariadb/testdb/10-apply-migrations.sh`: every `migrations/mysql/*.up.sql` in order | `docker/postgres/testdb/10-apply-migrations.sh`: every `migrations/postgres/*.up.sql` in order |
| 30 | | `schema/baseline/required_lookups.sql` |
| 40 | | `schema/seed/minimal.sql` |
| 50 | `schema/seed/test_integration_mysql.sql` | `schema/seed/test_integration.sql` |
| 60 | `docker/mariadb/testdb/60-set-admin-password.sh` | `docker/postgres/testdb/60-set-admin-password.sh` |

How to run the Go tests on each driver is in [TESTING.md](TESTING.md).

## Importing from OTRS or Znuny

Use `goatflow-migrate`. The full guide is [OTRS_MIGRATION_GUIDE.md](../OTRS_MIGRATION_GUIDE.md).
The make targets take the OTRS source as **one** of:

- `SQL=<path to a mysqldump file>`
- `SOURCE=<OTRS database DSN>`, for example `'user:pass@tcp(host:3306)/otrs'` (MySQL) or
  `'postgres://user:pass@host:5432/otrs?sslmode=disable'` (PostgreSQL)

| Command | What it does |
|---------|--------------|
| `make migrate-analyze SQL=...` | Lists the source tables, row counts and what the import does with each. |
| `make migrate-import SQL=...` | Dry run by default. `DRY_RUN=false` writes. |
| `make migrate-import-force SQL=...` | Deletes the existing tickets, articles and customers first. Destructive. |
| `make otrs-import SQL=...` | Writes by default. `DRY_RUN=1` plans only. `FORCE=1` clears existing data first. |
| `make migrate-validate` | Checks the imported data. |

The import writes into the GoatFlow database for `DB_DRIVER`. It keeps the OTRS ids.
