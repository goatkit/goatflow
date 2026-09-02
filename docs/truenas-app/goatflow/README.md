# GoatFlow

[GoatFlow](https://github.com/goatkit/goatflow) is an open-source support and ticket
management platform: agent UI, customer portal, background task runner, and a WASM
plugin system, with MariaDB and Valkey bundled in the app.

## Services

| Container | Role |
| --- | --- |
| `goatflow-backend` | Main server (agent UI + customer portal + REST API) |
| `goatflow-runner` | Background task processor (email queue, session cleanup) |
| `goatflow-customer-fe` | Optional standalone customer portal (`Enable Customer Portal`) |
| `mariadb` | Database (library-provided) |
| `valkey` | Cache (library-provided, temporary volume) |
| `permissions` | One-shot chown sidecar for the storage datasets |

## Upgrade / rollback

- **Upgrade**: bump the app version. Schema migrations run automatically and are
  idempotent on every start (compose `./migrate up` + in-app `RunMigrations`), so no
  manual migration step is needed. State (attachments, plugins, database) lives in
  the ixVolumes and is untouched by image upgrades.
- **Rollback**: downgrade the image tag. Down migrations exist (`*.down.sql`) but are
  untested — **take a database backup before upgrading** (TrueNAS → Applications →
  back up the app, or `mariadb-dump` the `mariadb` volume). A schema-only rollback
  is not supported for data written by new columns.
- The `permissions` sidecar re-chowns the plugin dataset after every start, so
  plugin ownership survives upgrades.
- `PASSWORD_HASH_TYPE` stays `sha256` (OTRS-compatible); moving to argon2/bcrypt
  later requires a one-off `MIGRATE_PASSWORD_HASHES=true` backfill.

## Notes

- App containers run as UID/GID from the wizard (default 1000, matching the image
  build args).
- Ports: backend `30484` (default), optional customer portal `30483`.
- Secrets (`JWT_SECRET`, `GOATFLOW_SECURE_KEY`, DB/SMTP passwords) are required and
  have no safe defaults.
