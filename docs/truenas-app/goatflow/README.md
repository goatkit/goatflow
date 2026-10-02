# GoatFlow

[GoatFlow](https://github.com/goatkit/goatflow) is an open-source support and ticket
management platform: agent UI, customer portal, background task runner, and a WASM
plugin system, with MariaDB and Valkey bundled in the app.

## Services

| Container | Role |
| --- | --- |
| `goatflow-backend` | Main server (agent UI + customer portal + REST API) |
| `goatflow-runner` | Background task processor: sends queued outgoing email, delivers webhooks, cleans up expired sessions |
| `goatflow-customer-fe` | Optional standalone customer portal (`Enable Customer Portal`) |
| `mariadb` | Database (library-provided) |
| `valkey` | Cache (library-provided, temporary volume) |
| `permissions` | One-shot chown sidecar for the storage datasets |

## Upgrade / rollback

- **Upgrade**: bump the app version. The backend applies any pending schema migrations
  when it starts, so no manual migration step is needed. State (attachments, plugins,
  database) lives in the ixVolumes and is untouched by image upgrades.
- **Rollback**: downgrade the image tag. Down migrations exist (`*.down.sql`) but are
  untested — **take a database backup before upgrading** (TrueNAS → Applications →
  back up the app, or `mariadb-dump` the `mariadb` volume). A schema-only rollback
  is not supported for data written by new columns.
- The `permissions` sidecar re-chowns the plugin dataset after every start, so
  plugin ownership survives upgrades.
- New passwords are hashed with bcrypt (`PASSWORD_HASH_TYPE=bcrypt`). Imported
  OTRS sha2 hashes still log in. Set `MIGRATE_PASSWORD_HASHES=true` to rehash each
  such password to bcrypt on that user's next successful login. Use
  `PASSWORD_HASH_TYPE=sha256` only while an OTRS must read the same user tables.

## Notes

- App containers run as UID/GID from the wizard (default 1000, matching the image
  build args).
- Ports: backend `30484` (default), optional customer portal `30483`.
- Secrets (`JWT_SECRET`, `GOATFLOW_SECURE_KEY`, DB/SMTP passwords) are required and
  have no safe defaults. The Application Secure Key must be exactly 64 hex characters
  (`openssl rand -hex 32`); backend, runner and customer portal share it, and it must
  never change after install.
- **Public URL** (`BASE_URL`): set it (for example `https://helpdesk.example.com`) so
  password-reset and customer sign-up emails are sent; while it is empty they are not.
- Backend, runner and customer portal get the same SMTP settings: the runner sends the
  mail queue, the backend sends some mail (for example two-factor codes) directly.
