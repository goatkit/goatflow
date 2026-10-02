package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// targetTable is one table of the GoatFlow database, read from its catalog.
type targetTable struct {
	columns []string
	binary  map[string]bool   // columns written as []byte (bytea / blob)
	refs    map[string]string // foreign key column -> referenced table
	autoID  bool              // the id column takes its value from a sequence / AUTO_INCREMENT
}

func (t *targetTable) has(column string) bool { return containsString(t.columns, column) }

// selfRefs are the columns referencing the table itself (users.create_by,
// calendar_appointment.parent_id, ...).
func (t *targetTable) selfRefs(table string) []string {
	var cols []string
	for _, c := range t.columns {
		if t.refs[c] == table {
			cols = append(cols, c)
		}
	}
	return cols
}

type targetSchema map[string]*targetTable

var binaryTypes = map[string]bool{
	"bytea": true, "blob": true, "tinyblob": true, "mediumblob": true, "longblob": true, "binary": true, "varbinary": true,
}

// loadTargetSchema reads the columns and foreign keys of the GoatFlow database.
func loadTargetSchema(db *sql.DB) (targetSchema, error) {
	columns := `SELECT table_name, column_name, data_type, extra, '' FROM information_schema.columns
		WHERE table_schema = DATABASE() ORDER BY table_name, ordinal_position`
	fks := `SELECT table_name, column_name, referenced_table_name FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE() AND referenced_table_name IS NOT NULL`
	if database.IsPostgreSQL() {
		columns = `SELECT table_name, column_name, data_type, COALESCE(column_default, ''), is_identity
			FROM information_schema.columns WHERE table_schema = current_schema() ORDER BY table_name, ordinal_position`
		fks = `SELECT kcu.table_name, kcu.column_name, ccu.table_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON kcu.constraint_schema = tc.constraint_schema AND kcu.constraint_name = tc.constraint_name AND kcu.table_name = tc.table_name
			JOIN information_schema.constraint_column_usage ccu
				ON ccu.constraint_schema = tc.constraint_schema AND ccu.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = current_schema()`
	}

	schema := targetSchema{}
	rows, err := db.Query(database.ConvertPlaceholders(columns))
	if err != nil {
		return nil, fmt.Errorf("read target schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, column, dataType, extra, identity string
		if err := rows.Scan(&table, &column, &dataType, &extra, &identity); err != nil {
			return nil, fmt.Errorf("read target schema: %w", err)
		}
		t := schema[table]
		if t == nil {
			t = &targetTable{binary: map[string]bool{}, refs: map[string]string{}}
			schema[table] = t
		}
		t.columns = append(t.columns, column)
		if binaryTypes[strings.ToLower(dataType)] {
			t.binary[column] = true
		}
		if column == "id" && (strings.Contains(strings.ToLower(extra), "auto_increment") ||
			strings.HasPrefix(extra, "nextval(") || identity == "YES") {
			t.autoID = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read target schema: %w", err)
	}

	fkRows, err := db.Query(database.ConvertPlaceholders(fks))
	if err != nil {
		return nil, fmt.Errorf("read target foreign keys: %w", err)
	}
	defer fkRows.Close()
	for fkRows.Next() {
		var table, column, refTable string
		if err := fkRows.Scan(&table, &column, &refTable); err != nil {
			return nil, fmt.Errorf("read target foreign keys: %w", err)
		}
		if t := schema[table]; t != nil {
			t.refs[column] = refTable
		}
	}
	return schema, fkRows.Err()
}

// importOrder sorts the tables to import so that every table comes after the
// tables its foreign keys reference. users goes first: every table references
// it (create_by, change_by), and its own references (valid) point at rows
// every GoatFlow database is seeded with. Ties keep importPlan order.
func importOrder(tables []tablePlan, schema targetSchema) ([]tablePlan, error) {
	pending := make(map[string]bool, len(tables))
	for _, p := range tables {
		pending[p.name] = true
	}
	var order []tablePlan
	for len(order) < len(tables) {
		progressed := false
		for _, p := range tables {
			if !pending[p.name] {
				continue
			}
			ready := true
			if p.name != "users" {
				for _, ref := range schema[p.name].refs {
					if ref != p.name && ref != "users" && pending[ref] {
						ready = false
						break
					}
				}
			}
			if ready {
				order = append(order, p)
				delete(pending, p.name)
				progressed = true
			}
		}
		if !progressed {
			var stuck []string
			for _, p := range tables {
				if pending[p.name] {
					stuck = append(stuck, p.name)
				}
			}
			return nil, fmt.Errorf("foreign keys between %s form a cycle", strings.Join(stuck, ", "))
		}
	}
	return order, nil
}
