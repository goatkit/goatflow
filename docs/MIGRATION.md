# OTRS to GoatFlow Migration Guide

## Overview

GoatFlow provides `goatflow-migrate` to import data from OTRS SQL dumps into GoatFlow. The tool handles schema translation between different database engines and migrates core ticket data, users, and article storage.

> **Note**: Migration tooling is under active development. This document reflects current capabilities.

## Supported Migration Paths

### Source → Target Database Support

| Source Database | Target Database | Status |
|-----------------|-----------------|--------|
| MySQL 5.7+ / MariaDB 10.2+ | PostgreSQL 9.6+ | ✅ Supported |
| MySQL 5.7+ / MariaDB 10.2+ | MySQL 8.0+ / MariaDB 10.2+ | ✅ Supported |
| PostgreSQL 9.6+ | PostgreSQL 9.6+ | ✅ Supported |
| PostgreSQL 9.6+ | MySQL 8.0+ / MariaDB 10.2+ | ✅ Supported |

### Supported OTRS Versions
- ✅ OTRS 6.x (Full support)
- ✅ OTRS 5.x (Full support)
- ⚠️ OTRS 4.x (Limited support, requires pre-migration upgrade)
- ⚠️ OTRS 3.x (Requires intermediate migration to OTRS 5+)

## Pre-Migration Checklist

### 1. OTRS Preparation
- [ ] OTRS system in maintenance mode
- [ ] Database backup completed
- [ ] SQL dump exported (see export commands below)
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

**From PostgreSQL:**
```bash
pg_dump -U otrs_user -d otrs_database -f otrs_dump.sql
```

### Step 2: Analyze the Dump

```bash
make migrate-analyze SQL=/path/to/otrs_dump.sql
```

Output includes:
- Total lines and tables in the dump
- Tables with data vs empty tables
- Row counts per table
- Core OTRS tables verification (users, groups, queue, ticket, article, etc.)

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

> ⚠️ **Warning**: This will DELETE ALL EXISTING DATA before importing!

## Article Storage Migration

OTRS stores article attachments in the filesystem (typically `/opt/otrs/var/article/`). These must be migrated separately from the database.

### Locate OTRS Article Storage

```bash
# Check OTRS config for ArticleDir
grep -r "ArticleDir" /opt/otrs/Kernel/Config.pm

# Default location
ls -la /opt/otrs/var/article/
```

### Copy Article Storage

```bash
# Copy to GoatFlow storage location
rsync -avz /opt/otrs/var/article/ /path/to/goatflow/storage/articles/

# Or via container volume
make toolbox-exec ARGS="cp -r /source/articles /app/storage/articles"
```

### Verify Article Paths

After database import, verify attachment references:

```bash
make migrate-validate
```

## Makefile Targets Reference

| Target | Description |
|--------|-------------|
| `make migrate-analyze SQL=<file>` | Analyze OTRS dump structure and contents |
| `make migrate-import SQL=<file> DRY_RUN=true` | Test import without changes |
| `make migrate-import SQL=<file> DRY_RUN=false` | Execute actual import |
| `make migrate-import-force SQL=<file>` | Clear data and reimport (destructive) |
| `make migrate-validate` | Validate imported data integrity |

## Direct Tool Usage

For advanced usage, scripting, or CI/CD pipelines, `goatflow-migrate` can be called directly inside containers:

```bash
# Analyze dump
goatflow-migrate -cmd=analyze -sql=/path/to/dump.sql

# Import with dry run
goatflow-migrate -cmd=import \
  -sql=/path/to/dump.sql \
  -db="postgres://user:pass@host:5432/goatflow?sslmode=disable" \
  -dry-run -v

# Force import (clears existing data)
goatflow-migrate -cmd=import \
  -sql=/path/to/dump.sql \
  -db="postgres://user:pass@host:5432/goatflow?sslmode=disable" \
  -force -v

# Validate imported data
goatflow-migrate -cmd=validate \
  -db="postgres://user:pass@host:5432/goatflow?sslmode=disable" -v
```

### goatflow-migrate Options

| Option | Description |
|--------|-------------|
| `-cmd` | Command: `analyze`, `import`, or `validate` |
| `-sql` | Path to OTRS SQL dump file |
| `-db` | Database connection URL |
| `-dry-run` | Test import without making changes |
| `-force` | Clear existing data before import (destructive!) |
| `-v` | Verbose output |

### Database Connection URLs

```bash
# PostgreSQL
-db="postgres://user:password@host:5432/database?sslmode=disable"

# MySQL/MariaDB  
-db="mysql://user:password@tcp(host:3306)/database"
```

## Data Mapping

### Core Tables

| OTRS Table | GoatFlow Table | Notes |
|------------|-------------|-------|
| ticket | tickets | Core ticket data |
| article | ticket_messages | Ticket communications |
| users | users | Agent accounts |
| customer_user | customers | Customer accounts |
| queue | queues | Ticket queues |
| ticket_type | ticket_types | Ticket categories |
| ticket_state | ticket_states | Status values |
| ticket_priority | ticket_priorities | Priority levels |

### Status Mapping

| OTRS Status | GoatFlow Status |
|-------------|--------------|
| new | new |
| open | open |
| pending reminder | pending |
| closed successful | resolved |
| closed unsuccessful | closed |

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

#### Large Dumps
For very large databases, the import may take significant time. Use `-v` to see progress.

## Limitations

Current migration tool limitations:
- Article attachments require separate filesystem copy (see Article Storage Migration above)
- Custom fields (DynamicFields) require manual mapping review
- Workflows/processes need to be recreated in GoatFlow
- GenericInterface configurations are not migrated
- OTRS-specific modules and customizations are not migrated

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
