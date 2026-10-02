# GoatFlow OTRS Migration Guide

## Overview

`goatflow-migrate` imports an OTRS 6 / Znuny 6.x database into GoatFlow, on MySQL/MariaDB or
PostgreSQL. It reads the OTRS database directly (OTRS on MySQL/MariaDB or on PostgreSQL) or a
`mysqldump` file, keeps every OTRS id, and imports agents with their group and role permissions,
customers, tickets, articles with attachments, history and the configuration around them. This
guide covers the sources, the per-table plan and how to bring an OTRS attachment tree along;
[MIGRATION.md](MIGRATION.md) has the step-by-step procedure.

## Migration Tool: `goatflow-migrate`

### Installation

```bash
# Build the migration tool
make toolbox-exec ARGS="go build -o bin/goatflow-migrate ./cmd/goatflow-migrate"
```

### Usage

```bash
# List the source's tables, row counts and what the import does with each
./bin/goatflow-migrate -cmd=analyze -source='otrs:secret@tcp(otrs-db:3306)/otrs'

# Print the import plan and row counts without writing (OTRS on PostgreSQL)
./bin/goatflow-migrate -cmd=import -source='postgres://otrs:secret@otrs-db/otrs?sslmode=disable' \
  -db='postgres://user:pass@localhost/goatflow?sslmode=disable' -dry-run

# Import
./bin/goatflow-migrate -cmd=import -source='otrs:secret@tcp(otrs-db:3306)/otrs' \
  -db='postgres://user:pass@localhost/goatflow?sslmode=disable'

# Import from a mysqldump file instead of a database
./bin/goatflow-migrate -cmd=import -sql=DatabaseBackup.sql -db='user:pass@tcp(localhost:3306)/goatflow'

# Validate imported data
./bin/goatflow-migrate -cmd=validate -db='postgres://user:pass@localhost/goatflow?sslmode=disable'
```

## Sources

| Source | How |
|--------|-----|
| OTRS on MySQL/MariaDB | `-source='user:pass@tcp(host:3306)/otrs'` (read-only access is enough) |
| OTRS on PostgreSQL | `-source='postgres://user:pass@host:5432/otrs?sslmode=disable'` |
| `mysqldump` / `mariadb-dump` file (e.g. OTRS `scripts/backup.pl` `DatabaseBackup.sql`) | `-sql=DatabaseBackup.sql` |
| `pg_dump` file | load it into a scratch PostgreSQL database, then use `-source` (below) |

Reading the source database with `database/sql` is the main path: both OTRS database engines,
no dump parsing, and the drivers return every value byte-exact. The `mysqldump` reader is kept
because the most common OTRS backup (`backup.pl` on MySQL) is a dump file and it imports without
a scratch database; both feed the same import code. A `pg_dump` file is not parsed: restoring it
is one command and avoids a third reader for PostgreSQL's `COPY` format:

```bash
createdb -h scratch-host otrs_restore
gunzip -c DatabaseBackup.sql.gz | psql -h scratch-host -d otrs_restore   # plain-format pg_dump
# (custom format: pg_restore -h scratch-host -d otrs_restore DatabaseBackup.dump)
./bin/goatflow-migrate -cmd=import -source='postgres://user:pass@scratch-host/otrs_restore?sslmode=disable' -db=...
```

OTRS on PostgreSQL stores binary article content base64-encoded in text columns (no
`DirectBlob` support); the importer decodes `article_data_mime_plain.body`,
`article_data_mime_attachment.content`, `standard_attachment.content` and `virtual_fs_db.content`
from such sources, so attachments arrive byte-exact.

## Import Plan

The import prints its plan first (the tables it imports, in foreign-key order, the source columns
GoatFlow has no column for, and the tables it skips with the reason) and a per-table report at the
end. `-cmd=analyze` shows the same decision per source table. Every table of the OTRS 6 / Znuny 6.x
schema has one of these decisions:

**Merge** — configuration a fresh GoatFlow may already hold (seed rows, admin-created entries).
Each OTRS row replaces the GoatFlow row with the same id, or is inserted. A GoatFlow row with
another id holding the same unique name is renamed `<name> (pre-import <id>)` first (reported).
`users`, `valid`, `groups`, `roles`, `permission_groups`, `ticket_state_type`, `ticket_state`,
`ticket_priority`, `ticket_type`, `ticket_lock_type`, `ticket_history_type`,
`article_sender_type`, `communication_channel`, `article_color`, `follow_up_possible`,
`salutation`, `signature`, `system_address`, `service`, `sla`, `queue`, `auto_response_type`,
`auto_response`, `standard_template`, `standard_attachment`, `notification_event`, `link_type`,
`link_state`, `link_object`, `dynamic_field`, `acl`, `calendar`, `mail_account`,
`gi_webservice_config`, `pm_process`, `pm_activity`, `pm_activity_dialog`, `pm_transition`,
`pm_transition_action`, `system_maintenance`, `oauth2_token_config`, `translation`.

**Replace** — permission, preference and link rows without an identity of their own. The OTRS rows
are authoritative: GoatFlow's rows are deleted and the OTRS rows inserted, so an imported agent
gets exactly the queues OTRS granted. `group_user`, `group_role`, `role_user`,
`group_customer_user`, `group_customer`, `user_preferences`, `personal_queues`,
`personal_services`, `queue_preferences`, `service_preferences`, `sla_preferences`, `service_sla`,
`service_customer_user`, `queue_standard_template`, `standard_template_attachment`,
`queue_auto_response`, `notification_event_item`, `notification_event_message`,
`pm_process_preferences`.

**Import** — data only the OTRS system has. The GoatFlow table must be empty (or use `-force`).
`customer_company`, `customer_user`, `customer_preferences`, `customer_user_customer`, `ticket`,
`article`, `article_data_mime`, `article_data_mime_plain`, `article_data_mime_attachment`,
`article_data_mime_send_error`, `article_data_otrs_chat`, `article_flag`, `ticket_history`,
`ticket_flag`, `ticket_watcher`, `time_accounting`, `dynamic_field_obj_id_name`,
`dynamic_field_value`, `link_relation`, `mention`, `calendar_appointment`,
`calendar_appointment_ticket`, `calendar_appointment_plugin`, `generic_agent_jobs`,
`postmaster_filter`, `search_profile`, `xml_storage`, `virtual_fs`, `virtual_fs_preferences`,
`virtual_fs_db`, `smime_keys`, `smime_signer_cert_relations`, `oauth2_token`,
`gi_webservice_config_history`, `acl_ticket_attribute_relations`, `activity`.

**Settings** — `sysconfig_modified` (the settings an OTRS admin changed). OTRS and GoatFlow number
their setting definitions differently, so overrides are matched by setting name: an override of a
setting GoatFlow defines (e.g. `TimeWorkingHours`) points at GoatFlow's definition and replaces
GoatFlow's override for the same user; for any other setting (e.g. `TimeWorkingHours::Calendar1`)
the OTRS definition from `sysconfig_default` is copied with it. GoatFlow reads OTRS settings by
name in OTRS's YAML format (escalation calendars, ...).

**Skipped**, with the reason:

| Table | Reason |
|-------|--------|
| `sysconfig_default` | OTRS's XML setting definitions, not data; only definitions of modified settings GoatFlow does not define are copied |
| `sysconfig_default_version`, `sysconfig_modified_version`, `sysconfig_deployment`, `sysconfig_deployment_lock` | OTRS deployment history and lock |
| `sessions` | login sessions; agents and customers log in again |
| `web_upload_cache` | temporary uploads of unsaved forms |
| `form_draft` | unsent OTRS form drafts, Perl-serialized for OTRS screens |
| `article_search_index` | GoatFlow does not read it: search runs on `article_data_mime` (database backend) or Elasticsearch/Zinc, so there is nothing to rebuild |
| `ticket_index`, `ticket_lock_index` | OTRS StaticDB caches GoatFlow does not read |
| `ticket_loop_protection` | per-day auto-reply counters |
| `ticket_number_counter` | OTRS writes one row per ticket number under a random `counter_uid`; the import sets GoatFlow's counters from the imported ticket numbers instead (see Ticket numbers) |
| `mail_queue` | outgoing mail the OTRS daemon has not sent; flush it before migrating |
| `communication_log*` | mail transport log OTRS purges after a few days |
| `gi_debugger_entry`, `gi_debugger_entry_content` | web service debug log |
| `scheduler_task`, `scheduler_future_task`, `scheduler_recurrent_task`, `process_id` | OTRS daemon state |
| `acl_sync`, `pm_entity_sync` | OTRS deployment state |
| `package_repository` | OTRS Perl add-on packages, not installable in GoatFlow |
| `cloud_service_config`, `system_data` | OTRS Group cloud services, registration and system state |

Tables only GoatFlow has (`gk_*`, `admin_action_log`, `admin_action_type`, `canned_response`,
`canned_response_category`, `user_api_tokens`, `sysconfig_org`, `dynamic_field_screen_config`)
are left alone. Source tables GoatFlow has no table for (add-ons such as ITSM) are listed with
their row counts and not imported.

## Import Details

### IDs, order and safety
Every imported row keeps its OTRS primary key, so ticket numbers, ticket and article ids (the
ArticleStorageFS directory names), history links, permissions and every other foreign key
survive. The import order comes from the GoatFlow schema's foreign keys; self references
(`users.create_by`, `calendar_appointment.parent_id`) are written once every row exists.
Afterwards the PostgreSQL sequences / MySQL `AUTO_INCREMENT` counters of every imported table are
reset to the highest id + 1.

All data is written in one transaction: an import that fails rolls back completely and reports
the table and the database error. A target that already holds tickets, articles, customers or other OTRS data is
refused; `-force` deletes that data (and the rows referencing it) in the same transaction first.
Merged and replaced tables are never cleared, so re-running an import with `-force` gives the same
result.

### Columns
Columns are matched by name between the source and the GoatFlow schema, so OTRS 6 and Znuny 6.x
column sets both import; source columns GoatFlow has no column for are listed in the plan. The
dump reader handles one-line and one-row-per-line (`mariadb-dump` 11) extended INSERTs and binary
values dumped with `--hex-blob` (`0x...`), as MySQL 8 `_binary '...'` strings, or as escaped
strings.

### Passwords
Agent and customer password hashes are copied unchanged. GoatFlow verifies the OTRS formats
(sha2, sha512, sha1, md5-crypt, apr1 and OTRS `BCRYPT:cost:salt:hash`), so imported accounts log in
with their OTRS passwords; DES crypt and plaintext (`CryptType=plain`) hashes do not verify and
need a password reset.

### Ticket numbers
GoatFlow keeps one ticket number counter per SystemID (AutoIncrement) or per SystemID and day
(Date, DateChecksum). The import sets these counters to the highest counter found in the imported
ticket numbers, parsed with the generator and SystemID the OTRS source was configured with
(`Ticket::NumberGenerator`, `SystemID`; OTRS defaults DateChecksum and 10). Tickets created after
the import then continue OTRS's numbering, and a ticket created on the day of the migration does
not get the number of a ticket OTRS created that day. Configure GoatFlow with the OTRS SystemID
(`SystemID` in GoatFlow's config) so its numbers continue the same sequence.

## Migration Process

```bash
# 1. Analyze the OTRS database
./bin/goatflow-migrate -cmd=analyze -source="$OTRS_DSN"

# 2. Dry run: plan and per-table row counts
./bin/goatflow-migrate -cmd=import -source="$OTRS_DSN" -db="$DB_URL" -dry-run

# 3. Import
./bin/goatflow-migrate -cmd=import -source="$OTRS_DSN" -db="$DB_URL"

# 4. Validate
./bin/goatflow-migrate -cmd=validate -db="$DB_URL"
```

After the import, check ticket and article display, customer portal access and agent logins.

## Mounting an Existing OTRS Filesystem (Attachments)

OTRS/Znuny installs that use ArticleStorageFS keep attachments and raw emails on
disk under `var/article/<YYYY/MM/DD>/<ArticleID>/` (the date is the article's
`article_data_mime.content_path`). GoatFlow's `fs` article storage backend uses
exactly this layout, including the `.content_type` / `.content_id` /
`.content_alternative` / `.disposition` sidecar files and `plain.txt`, so the tree
can be mounted and read as-is. The import keeps OTRS article ids and
`content_path`, which is what lets the imported articles find their directories.
See [Article Storage](ARTICLE_STORAGE.md) for details.

### Recommended docker-compose configuration

```yaml
services:
  backend:
    environment:
      STORAGE_TYPE: fs
      STORAGE_PATH: /app/storage
    volumes:
      - ./storage:/app/storage:Z
      - /opt/znuny/var/article:/app/storage/var/article:Z
```

### Notes

- Read-only option: use `:ro,Z` on the OTRS mount if GoatFlow must not write
  there; new attachments then fail to save, so only do this for a read-only
  archive.
- Permissions: containers run as UID 1000; the tree must be readable (and
  writable for new attachments) by that user. On SELinux systems `:Z` is required.
- To move the attachments into the database instead, keep `STORAGE_TYPE=fs` for
  the import, then run `goatflow-storage migrate -target DB`, `goatflow-storage
  verify -target DB`, and switch to `STORAGE_TYPE=db`.
