# Running GoatFlow on an Existing OTRS Database

GoatFlow uses the OTRS 6 / Znuny 6 table layout. It can connect to an
existing OTRS MySQL/MariaDB database and work with the same tickets,
users and articles. There is no data conversion step.

It is not "zero change". Read the next section before you point GoatFlow
at a live OTRS database.

## What GoatFlow changes in the database

Every time it starts, `goats` runs its own migrations
(`database.RunMigrations` in `cmd/goats/main.go`). On an OTRS database this:

- Creates a `schema_migrations` table to track the migration version.
- Creates GoatFlow tables. Most start with `gk_` (for example `gk_identity_provider`,
  `gk_webhook`, `gk_webauthn_credential`); API tokens use `user_api_tokens`.
- Adds a `color` column to `ticket_priority` and `ticket_state` if it is
  missing (as Znuny 6.5.1 does), and sets default colours.

OTRS ignores the extra tables and columns. They are still schema changes.
Your database backup and change process should treat them as such.

## Recommended approach

1. Back up the OTRS database.
2. Try GoatFlow first against a **copy** of the OTRS database.
3. When you are ready to move, either:
   - point GoatFlow at the live database, or
   - import into a fresh GoatFlow database with `goatflow-migrate`
     (see [MIGRATION.md](MIGRATION.md) and [OTRS_MIGRATION_GUIDE.md](OTRS_MIGRATION_GUIDE.md)).

Backup:

```bash
mysqldump -h localhost -u otrs -p otrs > otrs-backup.sql
```

There is no read-only mode.

## Configuration

Database connection variables are per driver (`internal/platform/dbconfig/env.go`).

```bash
DB_DRIVER=mysql
DB_MYSQL_HOST=your-otrs-db.example.com
DB_MYSQL_PORT=3306
DB_MYSQL_NAME=otrs
DB_MYSQL_USER=otrs
DB_MYSQL_PASSWORD=your-password
```

The old flat names (`DB_HOST`, `DB_NAME`, ...) still work as a fallback,
but use the `DB_MYSQL_*` names.

### Article storage

If OTRS stored articles on disk (`ArticleStorageFS`), mount that tree and set
`STORAGE_TYPE=fs`. See [ARTICLE_STORAGE.md](ARTICLE_STORAGE.md).

## Run GoatFlow

```bash
docker run -d \
  --name goatflow \
  --env-file .env \
  -p 8080:8080 \
  ghcr.io/goatkit/goatflow:0.10.0
```

If your OTRS database runs in Docker, it must be reachable from the
GoatFlow container (same network, or an exposed port).

## Running OTRS and GoatFlow side by side

Both can run against the same database. For example, OTRS on port 80 and
GoatFlow on port 8080. Keep these points in mind:

- Passwords: GoatFlow reads OTRS password hashes. If OTRS must also read
  passwords that GoatFlow sets, use `PASSWORD_HASH_TYPE=sha256`
  (see [SECURITY.md](SECURITY.md)).
- Switching back to OTRS leaves the GoatFlow tables and columns in place.

## Support

GitHub Issues: [github.com/goatkit/goatflow/issues](https://github.com/goatkit/goatflow/issues)
