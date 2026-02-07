# GoatFlow Database Schema Documentation

## Overview

GoatFlow's database schema is **rooted in OTRS compatibility** — the baseline 116 tables match OTRS Community Edition exactly, enabling direct migration from OTRS to GoatFlow. As GoatFlow adds features beyond OTRS's scope, new tables are added alongside the original schema without modifying it.

**Compatibility status:** OTRS baseline tables are frozen and unmodified. GoatFlow extensions use separate tables (see [Schema Extensions](#schema-extensions) below).

## Database Access Policy

All SQL must use the mandatory `database.ConvertPlaceholders` wrapper to support both PostgreSQL and MySQL. See [DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md).

## Password Reset Utilities

Use the provided make targets instead of connecting directly to the databases:

```bash
make reset-password             # Primary scope (defaults to DB_DRIVER)
make test-pg-reset-password     # PostgreSQL test scope
make test-mysql-reset-password  # MariaDB test scope
```

The targets route through `scripts/reset-user-password.sh`, which dispatches to `scripts/db/postgres/reset-user-password.sh` or `scripts/db/mysql/reset-user-password.sh` based on `DB_CONN_DRIVER`. Both helpers invoke the toolbox CLI inside the compose network, so no direct `mysql`/`psql` usage is required and `.env` credentials stay in sync.

## Schema Management Approach

### Baseline Schema (Current)
As of August 2025, GoatFlow uses a baseline schema initialization approach instead of sequential migrations:

```bash
# Fast initialization (<1 second)
make db-init        # Apply baseline schema + required lookups
make synthesize     # Generate secure test credentials
make db-apply-test-data  # Apply generated test data
```

### Migration Files Structure
```
migrations/
├── mysql/
│   ├── 000001_schema_alignment.up.sql   # MySQL schema (OTRS baseline, 127 tables)
│   ├── 000002_minimal_data.up.sql       # Essential lookup data
│   ├── 000003_customer_portal_sysconfig.up.sql
│   ├── 000004_dynamic_field_screen_config.up.sql
│   ├── 000005_canned_response.up.sql    # GoatFlow extension tables
│   ├── 000006_znuny_color_columns.up.sql # Znuny-compatible additions
│   ├── 000007_api_tokens.up.sql         # GoatFlow extension: API tokens
│   └── 000008_admin_audit_log.up.sql    # GoatFlow extension: audit logging
├── postgres/
│   ├── 000001_schema_alignment.up.sql   # PostgreSQL schema alignment
│   └── 000004_generated_test_data.up.sql  # Dynamically generated test data (optional)
└── legacy/                              # Old sequential migrations (archived)

schema/
├── baseline/
│   ├── otrs_complete.sql      # Complete OTRS schema (116 tables)
│   └── required_lookups.sql   # Essential lookup data
└── seed/
    └── minimal.sql             # Minimal seed data for development
```

## Schema Architecture

### OTRS Baseline (Frozen)

The 116 original OTRS tables are **frozen** — no modifications to structure, column types, or field names. See [SCHEMA_FREEZE.md](../architecture/SCHEMA_FREEZE.md) for the full policy.

This means:
- ✅ An OTRS database dump can be imported directly into GoatFlow
- ✅ SQL queries written for OTRS work unchanged against GoatFlow
- ✅ Third-party OTRS tools and reporting continue to work

### Schema Extensions

GoatFlow adds new tables for features that go beyond OTRS. These **never modify** existing OTRS tables:

| Table | Migration | Purpose |
|---|---|---|
| `dynamic_field_screen_config` | 000004 | Dynamic field screen assignments |
| `canned_response` | 000005 | Canned response templates |
| `canned_response_category` | 000005 | Canned response categorisation |
| `user_api_tokens` | 000007 | Personal access tokens for API auth |
| `admin_action_type` | 000008 | Audit log action type definitions |
| `admin_action_log` | 000008 | Admin action audit trail |

**Column additions to existing tables** (Znuny-compatible):

| Table | Column | Migration | Notes |
|---|---|---|---|
| `ticket_priority` | `color VARCHAR(25)` | 000006 | Znuny extension, not breaking |
| `ticket_state` | `color VARCHAR(25)` | 000006 | Znuny extension, not breaking |

### Features Using Existing OTRS Tables

Some GoatFlow features are built entirely on existing OTRS infrastructure:

- **Two-Factor Auth (TOTP):** Secrets and recovery codes stored in `user_preferences` / `customer_preferences`
- **SLA Management:** Uses existing `sla`, `sla_preferences`, `service_sla` tables
- **Knowledge Base:** Uses existing `faq_*` tables
- **GenericAgent:** Uses existing `generic_agent_jobs` table
- **Plugin state:** Uses existing `sysconfig_default` / `sysconfig_modified` tables

## Core Tables (OTRS-Compatible)

### User Management

```sql
-- Users table (OTRS-compatible with integer IDs)
CREATE TABLE users (
    id SERIAL PRIMARY KEY,              -- Integer, not UUID
    login varchar(200) NOT NULL,
    pw varchar(150) DEFAULT NULL,       -- OTRS legacy password field
    title varchar(50) DEFAULT NULL,
    first_name varchar(100) DEFAULT NULL,
    last_name varchar(100) DEFAULT NULL,
    email varchar(150) DEFAULT NULL,
    valid_id SMALLINT NOT NULL,         -- 1=valid, 2=invalid
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL,
    UNIQUE (login)
);

-- Group management
CREATE TABLE groups (
    id SERIAL PRIMARY KEY,
    name varchar(200) NOT NULL,
    comments varchar(250) DEFAULT NULL,
    valid_id SMALLINT NOT NULL,
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL,
    UNIQUE (name)
);

-- User to group mapping with permissions
CREATE TABLE group_user (
    user_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    permission_key varchar(20) NOT NULL,  -- rw, move_into, create, owner, priority, note
    permission_value SMALLINT NOT NULL,   -- 0 or 1
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL
);
```

### Ticket System

```sql
-- Main ticket table
CREATE TABLE ticket (
    id BIGSERIAL PRIMARY KEY,           -- BIGINT for tickets
    tn varchar(50) NOT NULL,            -- Ticket number
    title varchar(255) DEFAULT NULL,
    queue_id INTEGER NOT NULL,
    ticket_state_id SMALLINT NOT NULL,
    ticket_priority_id SMALLINT NOT NULL,
    ticket_lock_id SMALLINT NOT NULL,
    type_id SMALLINT NOT NULL,          -- Ticket type
    service_id INTEGER DEFAULT NULL,
    sla_id INTEGER DEFAULT NULL,
    user_id INTEGER NOT NULL,           -- Owner
    responsible_user_id INTEGER DEFAULT NULL,  -- Responsible agent
    group_id INTEGER DEFAULT NULL,
    customer_id varchar(150) DEFAULT NULL,     -- Company ID
    customer_user_id varchar(250) DEFAULT NULL, -- Customer email/login
    timeout INTEGER NOT NULL DEFAULT '0',
    until_time INTEGER NOT NULL DEFAULT '0',
    escalation_time INTEGER NOT NULL DEFAULT '0',
    escalation_update_time INTEGER NOT NULL DEFAULT '0',
    escalation_response_time INTEGER NOT NULL DEFAULT '0',
    escalation_solution_time INTEGER NOT NULL DEFAULT '0',
    archive_flag SMALLINT NOT NULL DEFAULT '0',
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL,
    UNIQUE (tn)
);

-- Article (ticket messages)
CREATE TABLE article (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL,
    article_sender_type_id SMALLINT NOT NULL,
    communication_channel_id BIGINT NOT NULL,
    is_visible_for_customer SMALLINT NOT NULL,
    search_index_needs_rebuild SMALLINT NOT NULL DEFAULT '1',
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL
);

-- Article content (MIME format)
CREATE TABLE article_data_mime (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL,
    a_from TEXT,
    a_to TEXT,
    a_cc TEXT,
    a_subject TEXT,
    a_body BYTEA,                       -- Body stored as bytea
    incoming_time INTEGER NOT NULL,     -- Unix timestamp
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL
);
```

### Customer Management

```sql
-- Customer companies
CREATE TABLE customer_company (
    customer_id varchar(150) PRIMARY KEY,  -- Company ID (not integer)
    name varchar(200) NOT NULL,
    street varchar(200) DEFAULT NULL,
    zip varchar(200) DEFAULT NULL,
    city varchar(200) DEFAULT NULL,
    country varchar(200) DEFAULT NULL,
    url varchar(200) DEFAULT NULL,
    comments varchar(250) DEFAULT NULL,
    valid_id SMALLINT NOT NULL,
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL
);

-- Customer users
CREATE TABLE customer_user (
    login varchar(200) PRIMARY KEY,     -- Email/login (not integer)
    email varchar(150) NOT NULL,
    customer_id varchar(150) NOT NULL,  -- Links to customer_company
    pw varchar(150) DEFAULT NULL,
    title varchar(50) DEFAULT NULL,
    first_name varchar(100) NOT NULL,
    last_name varchar(100) NOT NULL,
    phone varchar(150) DEFAULT NULL,
    mobile varchar(150) DEFAULT NULL,
    valid_id SMALLINT NOT NULL,
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL
);
```

### Lookup Tables

```sql
-- Ticket states
CREATE TABLE ticket_state (
    id SMALLSERIAL PRIMARY KEY,
    name varchar(200) NOT NULL,
    comments varchar(250) DEFAULT NULL,
    type_id SMALLINT NOT NULL,          -- Links to ticket_state_type
    valid_id SMALLINT NOT NULL,
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL,
    UNIQUE (name)
);

-- Ticket priorities
CREATE TABLE ticket_priority (
    id SMALLSERIAL PRIMARY KEY,
    name varchar(200) NOT NULL,
    valid_id SMALLINT NOT NULL,
    color varchar(25) NOT NULL,         -- Znuny extension
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL,
    UNIQUE (name)
);

-- Queues
CREATE TABLE queue (
    id SERIAL PRIMARY KEY,
    name varchar(200) NOT NULL,
    group_id INTEGER NOT NULL,
    system_address_id SMALLINT NOT NULL DEFAULT '0',
    salutation_id SMALLINT NOT NULL DEFAULT '0',
    signature_id SMALLINT NOT NULL DEFAULT '0',
    valid_id SMALLINT NOT NULL,
    create_time TIMESTAMP NOT NULL,
    create_by INTEGER NOT NULL,
    change_time TIMESTAMP NOT NULL,
    change_by INTEGER NOT NULL,
    UNIQUE (name)
);
```

## Important Schema Constraints

1. **Integer Primary Keys**: All OTRS tables use SERIAL/BIGSERIAL, not UUIDs
2. **OTRS Field Names**: Exact field names preserved (e.g., `pw` not `password`)
3. **Valid ID Pattern**: `valid_id` field where 1=valid, 2=invalid, 3=invalid-temporarily
4. **Audit Fields**: All tables include `create_time`, `create_by`, `change_time`, `change_by`
5. **Bytea Storage**: Article bodies stored as BYTEA, not TEXT
6. **Unix Timestamps**: Some fields use integer Unix timestamps (e.g., `incoming_time`)

## Database Operations

### Initialize Database
```bash
# Complete reset with baseline schema
make db-reset       # Drops and recreates database
make db-init        # Applies baseline schema (<1 second)
make synthesize     # Generates secure credentials
make db-apply-test-data  # Applies test data
```

### Connect to Database
```bash
# PostgreSQL shell
make db-shell

# Direct connection
PGPASSWORD=$YOUR_PASSWORD psql -h localhost -p 5432 -U goatflow_user -d goatflow
```

### View Schema
```sql
-- List all tables
\dt

-- View table structure
\d ticket
\d users
\d customer_user

-- Count tables
SELECT COUNT(*) FROM information_schema.tables 
WHERE table_schema = 'public';
```

## Test Data Generation

The minimal seed only creates a disabled `root@localhost` placeholder (no password, `valid_id=2`). Run `make synthesize` or `make reset-password` to provision credentials before attempting to sign in.

The system uses dynamic test data generation to avoid hardcoded passwords:

```bash
# Generate new test data with secure passwords
make synthesize

# View generated credentials
make show-dev-creds

# Output format:
# root@localhost / <generated via make synthesize>
# agent.smith / TRCzvGXJyGZJUf9s!1
# john.customer / Yq2PuMbRjW4JLQQK!1
```

## Test Database Containers

Both database engines can be brought up for integration testing via the container-first targets:

```bash
# Default (MariaDB) test stack
make test-db-up

# Force PostgreSQL
TEST_DB_DRIVER=postgres make test-db-up
```

The compose file (`docker-compose.testdb.yml`) mounts the same OTRS-aligned fixtures into both services:

- `schema/seed/test_integration.sql` for PostgreSQL (`postgres-test`)
- `schema/seed/test_integration_mysql.sql` for MariaDB (`mariadb-test`)

Each container runs its respective init script (`docker/postgres/testdb/10-apply-migrations.sh` or `docker/mariadb/testdb/10-apply-migrations.sh`) which applies `000001_schema_alignment.up.sql`, the minimal lookup data, and the integration fixtures. This keeps the API and HTMX suites database-agnostic—if it passes against one driver, it should pass against the other.

## Migration from OTRS

GoatFlow's OTRS baseline tables are structurally identical, so migration is straightforward:

1. **Direct Import**: OTRS database dumps can be imported directly — the baseline tables match
2. **No Schema Translation**: Core table structures are compatible
3. **Data Preservation**: All OTRS data relationships maintained
4. **Extension tables**: GoatFlow's additional tables (`user_api_tokens`, `canned_response`, etc.) are created by running migrations after import

```bash
# Import OTRS dump
psql -U goatflow_user -d goatflow < otrs_backup.sql

# Apply GoatFlow extension migrations
make db-migrate

# Or use the migration tool
make otrs-import DUMP=path/to/otrs_backup.sql
```

**Note:** GoatFlow features that use extension tables (API tokens, canned responses, audit logging) will initialise with empty data after migration. Features built on existing OTRS tables (TOTP via `user_preferences`, SLA, queues, etc.) will work immediately with migrated data.

## Performance Considerations

1. **Indexes**: All foreign keys and commonly queried fields are indexed
2. **Partitioning**: Large tables (ticket, article) can be partitioned by date
3. **Vacuum**: Regular VACUUM ANALYZE recommended for PostgreSQL
4. **Connection Pooling**: Use PgBouncer for high-traffic deployments

## Schema Freeze Policy

The OTRS baseline schema is **frozen** for compatibility:
- NO modifications to existing OTRS table structures
- NO renaming fields or changing data types
- New GoatFlow features use **separate extension tables** or existing infrastructure (e.g., `user_preferences`)
- Column additions only where Znuny-compatible (e.g., `color` on priority/state)

See [SCHEMA_FREEZE.md](../architecture/SCHEMA_FREEZE.md) for the detailed policy.
