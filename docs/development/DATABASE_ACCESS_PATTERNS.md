# Database Access Patterns

GoatFlow runs on **MySQL/MariaDB and PostgreSQL**. Nothing may be MySQL-only or
PostgreSQL-only. Every SQL statement — production code and tests — goes through
the conversion layer in `internal/platform/database` before it reaches a
`database/sql` handle (`*sql.DB`, `*sql.Tx`, `*sql.Conn`, prepared statements).

Write SQL in MySQL dialect with `?` placeholders; the layer adapts it to the
active driver (`DB_DRIVER`, or `TEST_DB_DRIVER` in tests).

## The conversion API

| Need | Use |
|---|---|
| Any query | `database.ConvertPlaceholders(sql)` |
| Query that also uses PostgreSQL `::` casts | `database.ConvertQuery(sql)` |
| Upsert (`ON DUPLICATE KEY UPDATE`, `REPLACE INTO`) | `database.ConvertUpsert(sql, conflictCols...)` |
| Insert that needs the new id | `database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders("INSERT … RETURNING id"), args...)` (`…Tx` for transactions) |

```go
row := db.QueryRow(database.ConvertPlaceholders(`
    SELECT id, title FROM ticket WHERE queue_id = ? AND ticket_state_id = ?
`), queueID, stateID)
```

Convert once, at the point of use. `ConvertPlaceholders` **panics** on `$N`
placeholders and on stacked (`;`-separated) statements, so never pass it an
already-converted string.

### What `ConvertPlaceholders` rewrites on PostgreSQL

- `?` → `$1, $2, …`
- `` `identifier` `` → `"identifier"`
- `INSERT IGNORE INTO …` → `INSERT INTO … ON CONFLICT DO NOTHING` (skips any
  row that violates a unique constraint, like MySQL)
- `DATE_SUB/DATE_ADD(expr, INTERVAL n UNIT)` and `INTERVAL ? UNIT`
- `UNIX_TIMESTAMP(…)`, `FROM_UNIXTIME(x)` → `to_timestamp(x)`, `CURDATE()`,
  `UUID()` → `gen_random_uuid()::text`
- `SET FOREIGN_KEY_CHECKS = 0|1` → `SET session_replication_role = replica|DEFAULT`
  (test fixtures only; needs a superuser, which the test database user is)

On MySQL it rewrites `ILIKE` → `LIKE` and `EXTRACT(EPOCH FROM x)::bigint` →
`UNIX_TIMESTAMP(x)`.

### Upserts

PostgreSQL needs the conflict target spelled out, so name the unique key:

```go
_, err := db.Exec(database.ConvertUpsert(`
    INSERT INTO user_preferences (user_id, preferences_key, preferences_value)
    VALUES (?, ?, ?)
    ON DUPLICATE KEY UPDATE preferences_value = VALUES(preferences_value)
`, "user_id", "preferences_key"), userID, key, value)
```

MySQL receives the query unchanged. PostgreSQL receives
`ON CONFLICT (user_id, preferences_key) DO UPDATE SET preferences_value = EXCLUDED.preferences_value`.
`REPLACE INTO t (cols) VALUES (…)` becomes an upsert that updates every
non-key column. The conflict columns must match a real `UNIQUE`/primary key in
both `migrations/mysql` and `migrations/postgres`.

### Inserted ids

`sql.Result.LastInsertId()` does not work on PostgreSQL. Use the adapter:

```go
id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
    INSERT INTO ticket (tn, title, queue_id) VALUES (?, ?, ?) RETURNING id
`), tn, title, queueID)
```

PostgreSQL runs the `RETURNING`; the MySQL adapter strips it and uses
`LastInsertId`.

### Dynamic queries

Build with `?`, convert at execution:

```go
query := "SELECT t.id, t.tn, t.title FROM ticket t WHERE 1=1"
var args []interface{}
if search != "" {
    query += " AND (t.tn ILIKE ? OR t.title ILIKE ?)"
    args = append(args, "%"+search+"%", "%"+search+"%")
}
rows, err := db.Query(database.ConvertPlaceholders(query), args...)
```

Values always go in `args`, never into the SQL text (`gk-lint` rejects SQL built
with `%s`/`%v`).

## SQL with no portable rewrite

Write these portably instead:

| MySQL-only | Portable |
|---|---|
| `IFNULL(a, b)` | `COALESCE(a, b)` |
| `LAST_INSERT_ID()` | `InsertWithReturning` |
| `DATE_FORMAT(…)`, `STR_TO_DATE(…)` | compute the date in Go and bind it |
| `GROUP_CONCAT(…)` | aggregate in Go |

## Schema conventions that affect queries

- Flag columns that the Go code writes as `bool` are `BOOLEAN` on PostgreSQL
  (`gk_custom_field_def.required`, `gk_identity_provider.enabled`/`auto_provision`,
  `gk_plugin_ui.enabled`, `gk_totp_pending_session.is_customer`,
  `gk_user_organisation.is_default`): bind a Go `bool` or write `= TRUE`/`= FALSE`
  and scan into `bool`. Never compare with `= 1`.
- OTRS lookup ids (`ticket_state`, `ticket_type`, `ticket_priority`, …) are
  `SMALLINT` on both databases.
- `migrations/mysql` and `migrations/postgres` must list the same versions;
  every schema change ships in both.

## sqlmock tests

The SQL a test sees depends on the active driver, so expectations must not
hard-code `\?` or `$1`. Match a placeholder-free statement prefix
(`mock.ExpectExec("DELETE FROM gk_webauthn_credential WHERE id")`) or build the
expectation with `regexp.QuoteMeta(database.ConvertPlaceholders(q))`.

## Enforcement

`cmd/gk-lint` type-checks the module (production and tests) and fails on:

- **`sql-unconverted`** — SQL reaching a `database/sql` call without
  `database.Convert*` (directly, or via a local variable whose every
  assignment is a `Convert*` call);
- **`sql-last-insert-id`** — any `sql.Result.LastInsertId()`;
- **`sql-mysql-only`** — an SQL literal with `ON DUPLICATE KEY UPDATE` /
  `REPLACE INTO` that doesn't reach `ConvertUpsert`, or with `LAST_INSERT_ID`,
  `DATE_FORMAT`, `STR_TO_DATE`, `GROUP_CONCAT`, `IFNULL`;
- **`sql-postgres-only`** — `RETURNING` outside `InsertWithReturning`,
  `ON CONFLICT`, `::` casts outside `ConvertQuery`, `||`, `NULLS FIRST/LAST`,
  `INTERVAL '…'`, full-text functions, `string_agg`;
- **`sql-unknown-table`** — SQL reaching the database names a table that the
  migrations do not create on **both** drivers (a legacy OTRS name such as
  `tickets`, or a table only a test creates);
- **`sql-unknown-column`** — an `INSERT` column list or `UPDATE … SET` names a
  column that table lacks on either driver;
- **`sql-sprintf`** — SQL built with `%s`/`%v`.

The schema rules read the schema from `migrations/mysql` and
`migrations/postgres` (`CREATE TABLE`, `ALTER TABLE … ADD/DROP/CHANGE/RENAME
COLUMN`, `DROP TABLE`), so a new table counts as soon as its migration exists.
`SELECT` column lists are not checked.

A reviewed exception — the conversion layer itself, a helper whose callers pass
converted SQL — carries `// sql-converted: <reason>` on the call's first line
or the line above. A genuine scratch table (an in-memory SQLite table in a
unit test) carries `// sql-schema: <reason>` the same way.

It runs in the pre-commit hook (`.githooks/pre-commit`, host, without cgo) and
in CI (`make lint-platform`, toolbox). The hook also runs
`scripts/tools/check-sql.sh --staged`, which rejects raw `$N` placeholders,
direct `Rebind()` calls and warns on `ILIKE` outside the layer (`// sql-ok`
suppresses a false positive).

Run the Go suite against both databases before merging database changes. The
test databases (`mariadb-test`, `postgres-test` in `docker-compose.testdb.yml`)
are built at container start by `docker/*/testdb/10-apply-migrations.sh`,
which applies every migration in order.
