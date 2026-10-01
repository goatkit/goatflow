// Package sqlconv is a gk-lint fixture. Lines marked "want <rule>" must be
// reported; every other DB call must not be.
package sqlconv

import (
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func cases(db *sql.DB, tx *sql.Tx, external string) {
	db.Exec("DELETE FROM t WHERE id = ?", 1) // want sql-unconverted

	q := "SELECT id FROM t WHERE id = ?"
	db.QueryRow(q, 1) // want sql-unconverted

	db.Query(external) // want sql-unconverted

	db.Exec(database.ConvertPlaceholders("DELETE FROM t WHERE id = ?"), 1)

	converted := database.ConvertPlaceholders("SELECT id FROM t WHERE id = ?")
	tx.QueryRow(converted, 1)

	mixed := database.ConvertPlaceholders("SELECT 1")
	if external != "" {
		mixed = "SELECT 2"
	}
	db.Query(mixed) // want sql-unconverted

	// sql-converted: fixture for the reviewed-exception directive
	db.Exec(external)

	res, _ := db.Exec(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?)"), 1)
	res.LastInsertId() // want sql-last-insert-id

	db.Exec(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) ON DUPLICATE KEY UPDATE a = VALUES(a)"), 1) // want sql-mysql-only

	db.Exec(database.ConvertUpsert("INSERT INTO t (a) VALUES (?) ON DUPLICATE KEY UPDATE a = VALUES(a)", "a"), 1)

	upsert := "REPLACE INTO t (a, b) VALUES (?, ?)"
	db.Exec(database.ConvertUpsert(upsert, "a"), 1, 2)

	db.QueryRow(database.ConvertPlaceholders("SELECT DATE_FORMAT(NOW(), '%Y') FROM t")) // want sql-mysql-only

	db.QueryRow(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) RETURNING id"), 1) // want sql-postgres-only

	ins := database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) RETURNING id")
	database.GetAdapter().InsertWithReturning(db, ins, 1)

	built := fmt.Sprintf("INSERT INTO %s (a) VALUES (?) RETURNING id", "t") //nolint:gk-sql-sprintf // fixture
	database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(built), 1)

	db.Exec(database.ConvertPlaceholders("INSERT INTO t (a) VALUES (?) ON CONFLICT DO NOTHING"), 1) // want sql-postgres-only

	db.Query(database.ConvertPlaceholders("SELECT id FROM t WHERE a::text = ?"), 1) // want sql-postgres-only

	db.Query(database.ConvertQuery("SELECT id FROM t WHERE a::text = ?"), 1)

	db.Query(database.ConvertPlaceholders("SELECT a || b FROM t")) // want sql-postgres-only

	db.Query(database.ConvertPlaceholders("SELECT a FROM t ORDER BY a DESC NULLS LAST")) // want sql-postgres-only

	db.Exec(database.ConvertPlaceholders("INSERT INTO t (nav, sep) VALUES ('Admin::Plugins', ' || ')"))

	db.Query(database.ConvertPlaceholders("SELECT id FROM t WHERE create_time > NOW() - INTERVAL '1 day'")) // want sql-postgres-only

	db.Query(database.ConvertPlaceholders("SELECT ts_rank(to_tsvector('english', a), plainto_tsquery('english', ?)) FROM t"), "x") // want sql-postgres-only
}
