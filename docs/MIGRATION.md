# OTRS to GoatFlow Migration Guide

## Overview

GoatFlow provides `goatflow-migrate` to import an OTRS 6 / Znuny 6.x database into GoatFlow, reading the OTRS database directly (MySQL/MariaDB or PostgreSQL) or a `mysqldump` file. It keeps every OTRS id and migrates agents with their group and role permissions, customers, tickets, articles with attachments, history, dynamic fields, time accounting, templates, auto responses, notifications and changed settings; see the [import plan](OTRS_MIGRATION_GUIDE.md#import-plan) for every table.

## Supported Migration Paths

### Source → Target Database Support

| Source | Target Database | Status |
|--------|-----------------|--------|
| OTRS MySQL/MariaDB database (`-source`) | PostgreSQL, MySQL 8.0+ / MariaDB 10.2+ | ✅ Supported |
| OTRS PostgreSQL database (`-source`) | PostgreSQL, MySQL 8.0+ / MariaDB 10.2+ | ✅ Supported |
| `mysqldump` / `mariadb-dump` file (`-sql`) | PostgreSQL, MySQL 8.0+ / MariaDB 10.2+ | ✅ Supported |
| `pg_dump` file | any | ✅ Restore into a scratch PostgreSQL database, then `-source` (Step 1) |

### Supported OTRS Versions
- ✅ OTRS 6.x and Znuny 6.x (the importer maps columns by name, so Znuny's extra columns such as `ticket_state.color` are picked up)
- ❌ OTRS 5.x and older: the article tables changed in OTRS 6, and the importer refuses such sources. Upgrade the OTRS installation to 6.x before migrating.

## Pre-Migration Checklist

### 1. OTRS Preparation
- [ ] OTRS system in maintenance mode
- [ ] Pending mail sent (`bin/otrs.Console.pl Maint::Email::MailQueue --send`) and the OTRS daemon stopped: `mail_queue` is not migrated
- [ ] Database backup completed
- [ ] SQL dump exported, or database access for `-source` (see Step 1)
- [ ] Article storage location identified

### 2. GoatFlow Preparation
- [ ] GoatFlow installed and running (`make up`)
- [ ] Database connection verified
- [ ] Storage paths configured for article attachments

## Migration Process

All migration commands use **Makefile targets** which handle container orchestration automatically.

### Step 1: Export OTRS Data

**From MySQL/MariaDB:**
```bash
mysqldump -u otrs_user -p \
  --default-character-set=utf8mb4 \
  --single-transaction \
  otrs_database > otrs_dump.sql
```

The importer reads the `CREATE TABLE` and `INSERT INTO` statements of a
`mysqldump`/`mariadb-dump` file (one-line or one-row-per-line extended INSERTs,
`--hex-blob` or escaped binary strings).

**Without a dump:** `goatflow-migrate` can read the OTRS database directly, on MySQL/MariaDB or
PostgreSQL (`-source`, see [Direct Tool Usage](#direct-tool-usage)). Read-only database access is
enough.

**From PostgreSQL:** restore the `pg_dump` (e.g. OTRS `backup.pl`'s `DatabaseBackup.sql.gz`) into
a scratch database and import from it with `-source`:
```bash
createdb -h scratch-host otrs_restore
gunzip -c DatabaseBackup.sql.gz | psql -h scratch-host -d otrs_restore
goatflow-migrate -cmd=import -source="postgres://user:pass@scratch-host/otrs_restore?sslmode=disable" -db="$DB_URL" -dry-run
```

### Step 2: Analyze the Source

```bash
make migrate-analyze SQL=/path/to/otrs_dump.sql
# or read the OTRS database directly (MySQL DSN or PostgreSQL URL)
make migrate-analyze SOURCE='otrs:secret@tcp(otrs-db:3306)/otrs'
```

Output lists every table of the source with its row count and what the import does with it
(merge, replace, import, sysconfig, skip with the reason, or unknown for tables GoatFlow does
not have). The dry run (Step 3) additionally prints the import order and the source columns
GoatFlow has no column for.

### Step 3: Dry Run Import

Test the migration without making changes:

```bash
make migrate-import SQL=/path/to/otrs_dump.sql DRY_RUN=true
```

### Step 4: Execute Import

Run the actual import:

```bash
make migrate-import SQL=/path/to/otrs_dump.sql DRY_RUN=false
```

### Step 5: Validate Migration

Verify the imported data:

```bash
make migrate-validate
```

### Step 6: Force Reimport (if needed)

To clear existing data and reimport from scratch:

```bash
make migrate-import-force SQL=/path/to/otrs_dump.sql
```

> ⚠️ **Warning**: This DELETES all tickets, articles (with their attachments, history, flags,
> time accounting, links and dynamic field values), customer users, customer companies and the
> other imported OTRS data (generic agent jobs, postmaster filters, ...) before importing, in the
> same transaction as the import. Merged and replaced tables (agents, groups, queues, permissions,
> ...) are not cleared; see Data Mapping.

## Article Storage Migration

OTRS keeps attachments either in the database (ArticleStorageDB, imported with
the dump) or on disk (ArticleStorageFS, typically `/opt/otrs/var/article/`).
Check which one the source uses:

```bash
grep -r "ArticleStorage\|ArticleDataDir" /opt/otrs/Kernel/Config.pm
```

For ArticleStorageFS, mount or copy the tree to `<STORAGE_PATH>/var/article`
(default `/app/storage/var/article`) and run GoatFlow with `STORAGE_TYPE=fs`;
the layout is identical, so no conversion is needed:

```bash
rsync -a /opt/otrs/var/article/ /path/to/goatflow/storage/var/article/
```

To keep everything in the database instead, copy the tree as above, then copy it
into the database and switch:

```bash
docker compose exec backend ./goatflow-storage migrate -target DB
docker compose exec backend ./goatflow-storage verify -target DB
# then set STORAGE_TYPE=db (or storage.type: db) and restart
```

See [Article Storage](ARTICLE_STORAGE.md).

## Makefile Targets Reference

| Target | Description |
|--------|-------------|
| `make migrate-analyze SQL=<file>` / `SOURCE=<dsn>` | List the source's tables, row counts and the import plan |
| `make migrate-import SQL=<file>` / `SOURCE=<dsn>` `DRY_RUN=true` | Print the plan without writing |
| `make migrate-import SQL=<file>` / `SOURCE=<dsn>` `DRY_RUN=false` | Execute actual import |
| `make migrate-import-force SQL=<file>` / `SOURCE=<dsn>` | Clear data and reimport (destructive) |
| `make otrs-import SQL=<file>` / `SOURCE=<dsn>` [`DRY_RUN=1`] [`FORCE=1`] | Import (writes unless `DRY_RUN` is set) |
| `make migrate-validate` | Validate imported data integrity |

Every target runs `goatflow-migrate` in the toolbox container against the GoatFlow database
selected by `DB_DRIVER`/`DB_*`. `SQL=` mounts the dump file read-only; `SOURCE=` must be reachable
from the `goatflow_goatflow-network` container network.

## Direct Tool Usage

For advanced usage, scripting, or CI/CD pipelines, `goatflow-migrate` can be called directly
inside containers. Reading the OTRS database directly (`-source`, `SOURCE=` in the Makefile
targets) works for OTRS on MySQL/MariaDB and on PostgreSQL.

```bash
# Analyze the OTRS database
goatflow-migrate -cmd=analyze -source="otrs:secret@tcp(otrs-db:3306)/otrs"

# Import with dry run (OTRS on PostgreSQL)
goatflow-migrate -cmd=import \
  -source="postgres://otrs:secret@otrs-db:5432/otrs?sslmode=disable" \
  -db="postgres://user:pass@host:5432/goatflow?sslmode=disable" \
  -dry-run

# Force import from a dump file (clears existing data)
goatflow-migrate -cmd=import \
  -sql=/path/to/dump.sql \
  -db="postgres://user:pass@host:5432/goatflow?sslmode=disable" \
  -force -v

# Validate imported data
goatflow-migrate -cmd=validate \
  -db="postgres://user:pass@host:5432/goatflow?sslmode=disable"
```

### goatflow-migrate Options

| Option | Description |
|--------|-------------|
| `-cmd` | Command: `analyze`, `import`, or `validate` |
| `-source` | OTRS database to read: a PostgreSQL URL or a MySQL DSN |
| `-sql` | OTRS `mysqldump` / `mariadb-dump` file to read (instead of `-source`) |
| `-db` | GoatFlow database URL (or `DATABASE_URL`) |
| `-dry-run` | Print the import plan and source row counts without writing |
| `-force` | Delete the target's tickets, articles, customers and their data before import (destructive!) |
| `-v` | Verbose output (prints the `-force` delete statements) |

### Database Connection URLs

`-source` and `-db` take the same formats:

```bash
# PostgreSQL
-db="postgres://user:password@host:5432/database?sslmode=disable"

# MySQL/MariaDB (go-sql-driver DSN, optionally prefixed with mysql://)
-db="user:password@tcp(host:3306)/database"
```

## Data Mapping

Every imported row keeps its OTRS primary key, so ticket numbers, ticket and article ids (the
ArticleStorageFS directory names), history links, permissions and every other reference survive
unchanged. After the import the id sequences (PostgreSQL) and `AUTO_INCREMENT` counters
(MySQL/MariaDB) of every imported table are reset, so the next row GoatFlow creates gets the
highest imported id + 1. The import runs in one transaction: if the database rejects a row the
import stops, reports the table and error, and leaves the target unchanged.

Every OTRS 6 / Znuny 6.x table is either merged (configuration, matched by id), replaced
(permissions and preferences: the OTRS rows are authoritative), imported (tickets, articles,
customers and their data: the target must be empty or `-force` is given), matched by name
(`sysconfig_modified`) or skipped with a reason. The full per-table list is the
[import plan](OTRS_MIGRATION_GUIDE.md#import-plan); the import prints it before writing and a
per-table report at the end, and `-cmd=analyze` shows it for a source.

For merged tables, each OTRS row replaces the GoatFlow row with the same id, or is inserted. A
GoatFlow row with another id whose unique name an OTRS row uses is renamed to
`<name> (pre-import <id>)`; the import prints every such rename. States, priorities and types are
imported as they are in OTRS, not translated.

## Post-Migration Tasks

### 1. Verify Data
- [ ] Check ticket counts match
- [ ] Verify user logins work
- [ ] Test customer portal access
- [ ] Review attachment accessibility

### 2. Update Configuration
- [ ] Configure email settings via environment variables
- [ ] Set up LDAP if needed (see LDAP documentation)
- [ ] Configure storage paths

### 3. User Communication
- [ ] Notify agents of new system
- [ ] Update customer documentation
- [ ] Provide login instructions

## URL Redirects

If users have bookmarked old OTRS URLs, you can add redirects to your reverse proxy:

```caddyfile
# Optional: Redirect old OTRS bookmarks
handle /otrs/* {
    redir / permanent
}
```

For the recommended Caddy configuration, see [deploy/docker-compose.yml](../deploy/docker-compose.yml).

## Troubleshooting

### Common Issues

#### Connection Errors
Ensure your database URL is correct:
```bash
# PostgreSQL format
-db="postgres://user:password@host:5432/database?sslmode=disable"
```

#### Character Encoding
If you see encoding issues, ensure your OTRS dump was exported with UTF-8:
```bash
mysqldump --default-character-set=utf8mb4 ...
```

#### Large Databases
The import prints each table as it finishes. Everything is written in one transaction, so the
target database needs room for the whole import before it commits.

## Limitations

Current migration tool limitations:
- Only ArticleStorageDB attachments are in the database; ArticleStorageFS trees are mounted or copied separately (see Article Storage Migration above)
- `pg_dump` files are restored into a scratch PostgreSQL database and imported with `-source` (see Step 1)
- The skipped tables listed in the [import plan](OTRS_MIGRATION_GUIDE.md#import-plan) (sessions, caches, logs, daemon state, OTRS deployment history, unsent mail) are not migrated
- Process management, GenericInterface, ACL and generic agent rows are preserved, but GoatFlow only acts on the features it implements
- Tables of OTRS add-ons GoatFlow has no table for (e.g. ITSM) are listed and not migrated
- Agent passwords stored as DES crypt or plaintext (`CryptType=plain`) do not verify; reset them after the import

---

## Database Schema Migrations

GoatFlow uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema versioning. The `migrate` tool is available in the backend container.

### Usage

```bash
# Check current migration version
./migrate -database 'postgres://...' version

# Apply all pending migrations
./migrate -path /app/db/migrations -database 'postgres://...' up

# Apply N migrations
./migrate -path /app/db/migrations -database 'postgres://...' up 3

# Rollback last migration
./migrate -path /app/db/migrations -database 'postgres://...' down 1

# Migrate to specific version
./migrate -path /app/db/migrations -database 'postgres://...' goto 20250101120000

# Force version (fix dirty state)
./migrate -path /app/db/migrations -database 'postgres://...' force 20250101120000
```

### Creating New Migrations

```bash
./migrate create -ext sql -dir db/migrations -seq add_customer_preferences

# Creates:
#   db/migrations/000042_add_customer_preferences.up.sql
#   db/migrations/000042_add_customer_preferences.down.sql
```

### Supported Database Drivers
- `postgres` / `postgresql`
- `mysql`

### Options
- `-source` - Migration files location (driver://url)
- `-path` - Shorthand for -source=file://path
- `-database` - Database connection URL
- `-verbose` - Print verbose logging
- `-lock-timeout N` - Database lock timeout in seconds (default: 15)
- `-prefetch N` - Migrations to load in advance (default: 10)

---

## Support

- **Documentation**: https://goatflow.io/docs
- **GitHub Issues**: https://github.com/goatkit/goatflow/issues
- **Community**: See CONTRIBUTING.md

---

*Last updated: January 2026*
