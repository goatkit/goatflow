# GOTRS Database Schema Documentation

## Overview

GOTRS uses a **100% OTRS-compatible database schema** to ensure seamless migration and interoperability. The schema is frozen (see [SCHEMA_FREEZE.md](../../SCHEMA_FREEZE.md)) and uses the exact OTRS table structures with integer primary keys, not UUIDs.

## Database Access Policy

All SQL must use the mandatory `database.ConvertPlaceholders` wrapper to support both PostgreSQL and MySQL. See `../../DATABASE_ACCESS_PATTERNS.md`.

## Schema Management Approach

### Baseline Schema (Current)
As of August 2025, GOTRS uses a baseline schema initialization approach instead of sequential migrations:

```bash
# Fast initialization (<1 second)
make db-init        # Apply baseline schema + required lookups
make synthesize     # Generate secure test credentials
make db-apply-test-data  # Apply generated test data
```

### Migration Files Structure
```
migrations/
├── 000004_generated_test_data.up.sql  # Dynamically generated test data
└── legacy/                             # Old sequential migrations (archived)
    ├── 000001_otrs_schema.up.sql
    ├── 000002_otrs_initial_data.up.sql
    └── ... (28 migration files)

schema/
├── baseline/
│   ├── otrs_complete.sql      # Complete OTRS schema (116 tables)
│   └── required_lookups.sql   # Essential lookup data
└── seed/
    └── minimal.sql             # Minimal seed data for development
```

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

1. **Integer Primary Keys**: All tables use SERIAL/BIGSERIAL, not UUIDs
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
PGPASSWORD=$YOUR_PASSWORD psql -h localhost -p 5432 -U gotrs_user -d gotrs
```

### View Schema
```sql
-- List all tables (116 OTRS tables)
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

The system uses dynamic test data generation to avoid hardcoded passwords:

```bash
# Generate new test data with secure passwords
make synthesize

# View generated credentials
make show-dev-creds

# Output format:
# root@localhost / X2xSOca5gGy3-1Lq!1
# agent.smith / TRCzvGXJyGZJUf9s!1
# john.customer / Yq2PuMbRjW4JLQQK!1
```

## Performance Considerations

1. **Indexes**: All foreign keys and commonly queried fields are indexed
2. **Partitioning**: Large tables (ticket, article) can be partitioned by date
3. **Vacuum**: Regular VACUUM ANALYZE recommended for PostgreSQL
4. **Connection Pooling**: Use PgBouncer for high-traffic deployments

## Migration from OTRS

Since GOTRS uses the exact OTRS schema:

1. **Direct Import**: OTRS database dumps can be imported directly
2. **No Schema Translation**: Table structures are 100% compatible
3. **Data Preservation**: All OTRS data relationships maintained
4. **Backward Compatible**: Can switch between OTRS and GOTRS

```bash
# Import OTRS dump
psql -U gotrs_user -d gotrs < otrs_backup.sql

# Or use the migration tool
make otrs-import DUMP=path/to/otrs_backup.sql
```

## Schema Freeze Policy

**CRITICAL**: The database schema is FROZEN for OTRS compatibility:
- NO modifications to existing OTRS tables
- NO new columns in OTRS tables  
- NO changing data types
- NO renaming fields
- New features must use separate tables or JSONB columns

See [SCHEMA_FREEZE.md](../../SCHEMA_FREEZE.md) for detailed policy.
