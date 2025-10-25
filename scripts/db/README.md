# Database Scripts

PostgreSQL-specific helpers live under `postgres/` and MySQL-specific helpers live under `mysql/`. When adding new database tooling, group it by engine in these directories so that each driver stays isolated.

## Current layout

- `postgres/`
	- `init-db.sql` – bootstrap for containerised Postgres instances
	- `fix_sequences.sql` – maintenance helper, see `scripts/sql/fix_sequences.sql`
	- `reset-user-password.sh` – invokes the toolbox CLI to reset a user in Postgres scopes
	- `db-ops.sh` – compose-aware wrapper for migrations and SQL helpers
- `mysql/`
	- `reset-user-password.sh` – toolbox-driven password reset for MariaDB scopes

Both reset helpers are called via the shim at `scripts/reset-user-password.sh`, which chooses the correct implementation based on `DB_CONN_DRIVER`/`DB_DRIVER`.
