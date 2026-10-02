// Package sqlconv is a gk-lint fixture. Lines marked "want <rule>" must be
// reported; every other DB call must not be.
package sqlconv

import (
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func cases(db *sql.DB, tx *sql.Tx, external string) {
	db.Exec("DELETE FROM t WHERE id = ?", 1) // #nosec G104 -- lint fixture, never executed // want sql-unconverted

	q := "SELECT id FROM t WHERE id = ?"
	db.QueryRow(q, 1) // want sql-unconverted

	db.Query(external) // #nosec G104 -- lint fixture, never executed // want sql-unconverted

	db.Exec(database.ConvertPlaceholders("DELETE FROM t WHERE id = ?"), 1) // #nosec G104 -- lint fixture, never executed

	converted := database.ConvertPlaceholders("SELECT id FROM t WHERE id = ?")
	tx.QueryRow(converted, 1)

	mixed := database.ConvertPlaceholders("SELECT 1")
	if external != "" {
		mixed = "SELECT 2"
	}
	db.Query(mixed) // #nosec G104 -- lint fixture, never executed // want sql-unconverted

	// sql-converted: fixture for the reviewed-exception directive
	db.Exec(external) // #nosec G104 -- lint fixture, never executed

	res, _ := db.Exec(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?)"), 1)
	res.LastInsertId() // #nosec G104 -- lint fixture, never executed // want sql-last-insert-id

	db.Exec(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) ON DUPLICATE KEY UPDATE a = VALUES(a)"), 1) // #nosec G104 -- lint fixture, never executed // want sql-mysql-only

	db.Exec(database.ConvertUpsert("INSERT INTO t (a) VALUES (?) ON DUPLICATE KEY UPDATE a = VALUES(a)", "a"), 1) // #nosec G104 -- lint fixture, never executed

	upsert := "REPLACE INTO t (a, b) VALUES (?, ?)"
	db.Exec(database.ConvertUpsert(upsert, "a"), 1, 2) // #nosec G104 -- lint fixture, never executed

	db.QueryRow(database.ConvertPlaceholders("SELECT DATE_FORMAT(NOW(), '%Y') FROM t")) // want sql-mysql-only

	db.QueryRow(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) RETURNING id"), 1) // want sql-postgres-only

	ins := database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) RETURNING id")
	database.GetAdapter().InsertWithReturning(db, ins, 1) // #nosec G104 -- lint fixture, never executed

	built := fmt.Sprintf("INSERT INTO %s (a) VALUES (?) RETURNING id", "t")               //nolint:gk-sql-sprintf // fixture
	database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(built), 1) // #nosec G104 -- lint fixture, never executed

	db.Exec(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) ON CONFLICT DO NOTHING"), 1) // #nosec G104 -- lint fixture, never executed // want sql-postgres-only

	db.Query(database.ConvertPlaceholders("SELECT id FROM t WHERE a::text = ?"), 1) // #nosec G104 -- lint fixture, never executed // want sql-postgres-only

	db.Query(database.ConvertQuery("SELECT id FROM t WHERE a::text = ?"), 1) // #nosec G104 -- lint fixture, never executed

	db.Query(database.ConvertPlaceholders("SELECT a || b FROM t")) // #nosec G104 -- lint fixture, never executed // want sql-postgres-only

	db.Query(database.ConvertPlaceholders("SELECT a FROM t ORDER BY a DESC NULLS LAST")) // #nosec G104 -- lint fixture, never executed // want sql-postgres-only

	db.Exec(database.ConvertPlaceholders("INSERT INTO t (a, b) VALUES ('Admin::Plugins', ' || ')")) // #nosec G104 -- lint fixture, never executed

	db.Query(database.ConvertPlaceholders("SELECT id FROM t WHERE create_time > NOW() - INTERVAL '1 day'")) // #nosec G104 -- lint fixture, never executed // want sql-postgres-only

	db.Query(database.ConvertPlaceholders("SELECT ts_rank(to_tsvector('english', a), plainto_tsquery('english', ?)) FROM t"), "x") // #nosec G104 -- lint fixture, never executed // want sql-postgres-only
}

// Schema rules: the fixture schema (testdata/sqlschema) has table t with
// id, a, b, added, new_name, create_time on both drivers; mysql_only only
// on MySQL; dropped and old_name were dropped/renamed; table gone was dropped.
func schemaCases(db *sql.DB) {
	db.Query(database.ConvertPlaceholders("SELECT id FROM missing_tbl WHERE id = ?"), 1) // #nosec G104 -- lint fixture, never executed // want sql-unknown-table

	db.Query(database.ConvertPlaceholders("SELECT id FROM gone")) // #nosec G104 -- lint fixture, never executed // want sql-unknown-table

	db.Exec(database.ConvertPlaceholders("INSERT INTO t (a, nope) VALUES (?, ?)"), 1, 2) // #nosec G104 -- lint fixture, never executed // want sql-unknown-column

	db.Exec(database.ConvertPlaceholders("UPDATE t SET b = ?, mysql_only = ? WHERE id = ?"), 1, 2, 3) // #nosec G104 -- lint fixture, never executed // want sql-unknown-column

	db.Exec(database.ConvertPlaceholders("UPDATE t SET dropped = 1")) // #nosec G104 -- lint fixture, never executed // want sql-unknown-column

	db.Exec(database.ConvertPlaceholders("UPDATE t x SET old_name = 1")) // #nosec G104 -- lint fixture, never executed // want sql-unknown-column

	db.Exec(database.ConvertPlaceholders("UPDATE t SET added = ?, new_name = ? WHERE a = ?"), 1, 2, "x") // #nosec G104 -- lint fixture, never executed

	db.Query(database.ConvertPlaceholders("WITH recent AS (SELECT id FROM t) SELECT id FROM recent")) // #nosec G104 -- lint fixture, never executed

	db.Query(database.ConvertPlaceholders("SELECT EXTRACT(YEAR FROM create_time) FROM t")) // #nosec G104 -- lint fixture, never executed

	db.Exec(database.ConvertUpsert("INSERT INTO t (id, a) VALUES (?, ?) ON DUPLICATE KEY UPDATE a = VALUES(a)", "id"), 1, "x") // #nosec G104 -- lint fixture, never executed

	db.Query(database.ConvertPlaceholders("SELECT id FROM t FOR UPDATE")) // #nosec G104 -- lint fixture, never executed

	q := "SELECT t.id " +
		"FROM t " +
		"JOIN gone g ON g.id = t.id" // want sql-unknown-table
	db.Query(database.ConvertPlaceholders(q)) // #nosec G104 -- lint fixture, never executed

	// sql-schema: fixture for the schema-rule directive
	db.Exec(database.ConvertPlaceholders("CREATE TABLE scratch (id INT)")) // #nosec G104 -- lint fixture, never executed

	_ = fmt.Errorf("update failed for %s", "missing_tbl")
}
