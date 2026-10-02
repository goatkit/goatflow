# Preventing PostgreSQL Sequence Issues

> **Note**: This page is about PostgreSQL. For SQL that works on both databases, see
> [development/DATABASE_ACCESS_PATTERNS.md](development/DATABASE_ACCESS_PATTERNS.md).

## Problem

Rows inserted with explicit IDs do not move a PostgreSQL sequence forward.
The next normal insert then gets an ID that already exists, and fails with
`duplicate key value violates unique constraint`.

## OTRS imports: handled for you

`goatflow-migrate` (`make migrate-import`, `make otrs-import`) resets the ID
counters itself when the import finishes:

- PostgreSQL: `setval(pg_get_serial_sequence(...))` for each imported table.
- MySQL/MariaDB: `ALTER TABLE ... AUTO_INCREMENT = ...`.

Code: `resetIDSequences` in `cmd/goatflow-migrate/import.go`.
You do not need to run anything extra after an import.

## When you still need `make db-fix-sequences`

Run it after loading data with explicit IDs by some other route:

- Restoring a PostgreSQL backup or a hand-made SQL dump.
- Manual `INSERT`s with explicit IDs.
- If you see `duplicate key value violates unique constraint` on a `pkey`.

`make db-fix-sequences` runs a PostgreSQL `DO` block that sets every sequence
to the highest ID in its table. It is safe to run more than once.

## Fix one table by hand

```sql
SELECT setval('article_id_seq', (SELECT MAX(id) FROM article));
SELECT setval('ticket_id_seq', (SELECT MAX(id) FROM ticket));
```

## Check a sequence

```bash
docker exec goatflow-postgres psql -U goatflow_user -d goatflow -c "
    SELECT MAX(id) AS max_id,
           (SELECT last_value FROM article_id_seq) AS sequence_value
    FROM article;"
```

`sequence_value` should be at least `max_id`.
