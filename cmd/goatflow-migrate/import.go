// Package main provides the OTRS/Znuny import for goatflow-migrate.
package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// tableReport is what the import did with one table.
type tableReport struct {
	plan      tablePlan
	inSource  bool
	rows      int // source rows
	inserted  int
	updated   int // merge: target rows replaced by OTRS rows; sysconfig: settings overwritten
	deleted   int // replace: target rows removed before the insert
	copied    int // sysconfig: definitions copied from the source's sysconfig_default
	renamed   []string
	dropped   []string // source columns without a GoatFlow column
	unmatched []string // sysconfig: modified settings without a definition
}

// importer copies one OTRS database into a GoatFlow database.
type importer struct {
	src     source
	db      *sql.DB
	tx      *sql.Tx
	schema  targetSchema
	tables  map[string]*sourceTable
	reports map[string]*tableReport
	unknown []string // source tables GoatFlow has no table for
	order   []tablePlan
	verbose bool
	seed    *counterSeed // ticket number counters set from the imported tickets
}

// runImport imports src into the GoatFlow database at dbURL. The plan is
// printed first; with dryRun nothing is written. The data import runs in one
// transaction, so a failed import leaves the target unchanged.
func runImport(src source, dbURL string, verbose, dryRun, force bool) error {
	db, err := openDB(dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	im := &importer{src: src, db: db, tables: src.tables(), reports: map[string]*tableReport{}, verbose: verbose}
	if im.schema, err = loadTargetSchema(db); err != nil {
		return err
	}
	if err := im.buildPlan(); err != nil {
		return err
	}
	im.printPlan()

	if dryRun {
		fmt.Printf("\n🧪 DRY RUN: nothing was written.\n")
		return im.countSkipped(true)
	}

	if im.tx, err = db.Begin(); err != nil {
		return err
	}
	defer func() { _ = im.tx.Rollback() }()
	if err := im.prepareTarget(force); err != nil {
		return err
	}
	fmt.Printf("\n📥 Importing (OTRS ids are kept)...\n")
	for _, p := range im.order {
		if err := im.importTable(p); err != nil {
			return fmt.Errorf("import %s: %w", p.name, err)
		}
	}
	if im.reports["ticket"].inSource {
		if im.seed, err = im.setTicketNumberCounters(); err != nil {
			return fmt.Errorf("set ticket number counters: %w", err)
		}
	}
	if err := im.tx.Commit(); err != nil {
		return fmt.Errorf("commit import: %w", err)
	}
	if err := im.resetIDSequences(); err != nil {
		return err
	}
	if err := im.countSkipped(false); err != nil {
		return err
	}
	im.printReport()
	fmt.Printf("\n✅ Import completed successfully\n")
	return nil
}

// buildPlan matches the source tables against the plan and the target schema.
func (im *importer) buildPlan() error {
	if t := im.tables["article"]; t != nil && !containsString(t.columns, "communication_channel_id") {
		return fmt.Errorf("the source's article table has no communication_channel_id column, so it is not an OTRS 6 / Znuny 6 schema; " +
			"upgrade OTRS to version 6 (or Znuny 6.x) before migrating")
	}
	var imported []tablePlan
	for _, p := range importPlan {
		r := &tableReport{plan: p}
		im.reports[p.name] = r
		st, inSource := im.tables[p.name]
		tt, inTarget := im.schema[p.name]
		r.inSource = inSource
		if !p.imported() || !inSource {
			continue
		}
		if !inTarget {
			return fmt.Errorf("the GoatFlow database has no %s table; run the GoatFlow migrations first", p.name)
		}
		for _, c := range st.columns {
			if !tt.has(c) {
				r.dropped = append(r.dropped, c)
			}
		}
		if p.mode == modeMerge && !containsString(st.columns, p.key) {
			return fmt.Errorf("the source's %s table has no %s column", p.name, p.key)
		}
		imported = append(imported, p)
	}
	for name := range im.tables {
		if _, ok := planFor(name); !ok && !strings.HasPrefix(name, "gk_") {
			im.unknown = append(im.unknown, name)
		}
	}
	sort.Strings(im.unknown)
	var err error
	im.order, err = importOrder(imported, im.schema)
	return err
}

func (im *importer) printPlan() {
	fmt.Printf("\n📋 Import plan: %s → %s database\n", im.src.describe(), database.GetDBDriver())
	for i, p := range im.order {
		r := im.reports[p.name]
		fmt.Printf("  %3d. %-32s %s\n", i+1, p.name, p.mode)
		if len(r.dropped) > 0 {
			fmt.Printf("       ⚠️  source columns without a GoatFlow column are not imported: %s\n", strings.Join(r.dropped, ", "))
		}
	}
	var absent []string
	for _, p := range importPlan {
		if p.imported() && !im.reports[p.name].inSource {
			absent = append(absent, p.name)
		}
	}
	if len(absent) > 0 {
		fmt.Printf("  Not in the source (nothing to import): %s\n", strings.Join(absent, ", "))
	}
	fmt.Printf("  Not imported:\n")
	for _, p := range importPlan {
		if p.mode == modeSkip && im.reports[p.name].inSource {
			fmt.Printf("     - %-30s %s\n", p.name, p.reason)
		}
	}
	for _, name := range im.unknown {
		fmt.Printf("     - %-30s not an OTRS 6 / Znuny 6 table GoatFlow has (add-on?)\n", name)
	}
}

// countSkipped counts the source rows of every table the import does not
// write (with all, of the imported tables too: the dry run).
func (im *importer) countSkipped(all bool) error {
	if all {
		fmt.Printf("\n📊 Source rows:\n")
	}
	for _, p := range importPlan {
		r := im.reports[p.name]
		if !r.inSource || (p.imported() && !all) {
			continue
		}
		n, err := im.src.count(p.name)
		if err != nil {
			return fmt.Errorf("count source table %s: %w", p.name, err)
		}
		r.rows = n
		if all {
			fmt.Printf("  %-32s %-9s %d\n", p.name, p.mode, n)
		}
	}
	return nil
}

func (im *importer) printReport() {
	fmt.Printf("\n📊 Per-table report:\n")
	var absent []string
	for _, p := range importPlan {
		r := im.reports[p.name]
		switch {
		case p.mode == modeGoatFlow:
			continue
		case !r.inSource:
			absent = append(absent, p.name)
			continue
		case p.mode == modeSkip:
			fmt.Printf("  ⏭  %-32s %-9s %d rows not imported: %s\n", p.name, p.mode, r.rows, p.reason)
		case p.mode == modeMerge:
			fmt.Printf("  ✅ %-32s %-9s %d rows: %d inserted, %d replaced\n", p.name, p.mode, r.rows, r.inserted, r.updated)
		case p.mode == modeReplace:
			fmt.Printf("  ✅ %-32s %-9s %d rows inserted, %d target rows replaced\n", p.name, p.mode, r.inserted, r.deleted)
		case p.mode == modeSysconfig:
			fmt.Printf("  ✅ %-32s %-9s %d rows: %d inserted, %d overwritten, %d definitions copied\n",
				p.name, p.mode, r.rows, r.inserted, r.updated, r.copied)
		default:
			fmt.Printf("  ✅ %-32s %-9s %d rows inserted\n", p.name, p.mode, r.inserted)
		}
		for _, n := range r.renamed {
			fmt.Printf("       ↳ existing row renamed to %q (an OTRS row with another id uses its name)\n", n)
		}
		if len(r.unmatched) > 0 {
			fmt.Printf("       ⚠️  modified settings without a definition in either database, not imported: %s\n", strings.Join(r.unmatched, ", "))
		}
	}
	for _, name := range im.unknown {
		n, err := im.src.count(name)
		if err != nil {
			fmt.Printf("  ⚠️  %-32s %-9s not imported: GoatFlow has no such table (count failed: %v)\n", name, "unknown", err)
			continue
		}
		fmt.Printf("  ⚠️  %-32s %-9s %d rows not imported: GoatFlow has no such table\n", name, "unknown", n)
	}
	if s := im.seed; s != nil {
		fmt.Printf("  🔢 ticket number counters: %d set from the imported ticket numbers (OTRS generator %s, SystemID %s);"+
			" give GoatFlow the same SystemID to continue the numbering\n", len(s.counters), s.generator, s.systemID)
	}
	if len(absent) > 0 {
		fmt.Printf("  ·  not in the source: %s\n", strings.Join(absent, ", "))
	}
}

// dataTables are the plan's data tables present in the target.
func (im *importer) dataTables() []string {
	var names []string
	for _, p := range importPlan {
		if p.mode == modeData && im.schema[p.name] != nil {
			names = append(names, p.name)
		}
	}
	return names
}

// forceClears remove rows that point at tickets without a foreign key.
var forceClears = []string{
	"DELETE FROM form_draft WHERE object_type = 'Ticket'",
}

// prepareTarget refuses a target whose data tables hold rows. Imported rows
// keep their OTRS ids, so tickets, articles and customers must not exist
// yet. With force they are deleted first, with every row referencing them.
// Merged and replaced tables are never cleared.
func (im *importer) prepareTarget(force bool) error {
	var nonEmpty []string
	for _, name := range im.dataTables() {
		var n int
		if err := im.tx.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM `" + name + "`")).Scan(&n); err != nil {
			return fmt.Errorf("count %s: %w", name, err)
		}
		if n > 0 {
			nonEmpty = append(nonEmpty, fmt.Sprintf("%s (%d rows)", name, n))
		}
	}
	if len(nonEmpty) == 0 {
		fmt.Printf("\n✅ Target database holds no tickets, articles or customers\n")
		return nil
	}
	if !force {
		fmt.Printf("\n❌ ERROR: The following tables contain data:\n")
		for _, t := range nonEmpty {
			fmt.Printf("   - %s\n", t)
		}
		fmt.Printf("\n⚠️  Import keeps OTRS ids, so tickets, articles and customers must not exist yet.\n")
		fmt.Printf("\n📘 Options:\n")
		fmt.Printf("   1. Use --force to delete all tickets, articles, customers and their data before import (DESTRUCTIVE!)\n")
		fmt.Printf("   2. Use 'make db-reset' to reset the database\n\n")
		return fmt.Errorf("database contains existing data, use --force to clear it")
	}

	fmt.Printf("\n⚠️  WARNING: Force mode enabled - deleting:\n")
	for _, t := range nonEmpty {
		fmt.Printf("   - %s\n", t)
	}
	for _, stmt := range im.clearStatements() {
		if im.verbose {
			fmt.Printf("     %s\n", stmt)
		}
		if _, err := im.tx.Exec(database.ConvertPlaceholders(stmt)); err != nil {
			return fmt.Errorf("clear existing data (%s): %w", stmt, err)
		}
	}
	return nil
}

// clearStatements empty the data tables and delete the rows of other tables
// referencing them, referencing tables first.
func (im *importer) clearStatements() []string {
	data := im.dataTables()
	cleared := make(map[string]bool)
	for _, n := range data {
		cleared[n] = true
	}
	// Tables referencing cleared tables lose those rows too (mail_queue,
	// article_search_index, ticket_index, ...), transitively.
	for changed := true; changed; {
		changed = false
		for name, t := range im.schema {
			if cleared[name] {
				continue
			}
			for _, ref := range t.refs {
				if cleared[ref] && ref != name {
					cleared[name] = true
					changed = true
					break
				}
			}
		}
	}
	var names []string
	for n := range cleared {
		names = append(names, n)
	}
	sort.Strings(names)
	stmts := append([]string(nil), forceClears...)
	// Delete in reverse dependency order: a table goes once nothing left
	// references it.
	for len(names) > 0 {
		var next []string
		for _, n := range names {
			referenced := false
			for _, other := range names {
				if other == n {
					continue
				}
				for _, ref := range im.schema[other].refs {
					if ref == n {
						referenced = true
					}
				}
			}
			if referenced {
				next = append(next, n)
				continue
			}
			t := im.schema[n]
			for _, c := range t.selfRefs(n) {
				stmts = append(stmts, "UPDATE `"+n+"` SET `"+c+"` = NULL WHERE `"+c+"` IS NOT NULL")
			}
			if containsString(data, n) {
				stmts = append(stmts, "DELETE FROM `"+n+"`")
				continue
			}
			var conds []string
			for _, c := range t.columns {
				if ref, ok := t.refs[c]; ok && cleared[ref] && ref != n {
					conds = append(conds, "`"+c+"` IS NOT NULL")
				}
			}
			stmts = append(stmts, "DELETE FROM `"+n+"` WHERE "+strings.Join(conds, " OR "))
		}
		if len(next) == len(names) {
			// A reference cycle: break it by deleting the rest in name order.
			for _, n := range next {
				stmts = append(stmts, "DELETE FROM `"+n+"`")
			}
			break
		}
		names = next
	}
	return stmts
}

// rowMapper turns source rows into insert arguments for the target columns.
type rowMapper struct {
	table   string
	target  *targetTable
	b64     map[string]bool
	lastSrc []string
	cols    []string
	idx     []int
}

func (im *importer) mapper(table string) *rowMapper {
	return &rowMapper{table: table, target: im.schema[table], b64: im.tables[table].base64}
}

// row maps one source row onto the target columns present in the source.
func (m *rowMapper) row(columns []string, row []dumpValue) ([]string, []interface{}, error) {
	if len(columns) != len(m.lastSrc) || len(columns) > 0 && &columns[0] != &m.lastSrc[0] {
		m.lastSrc, m.cols, m.idx = columns, nil, nil
		for _, c := range m.target.columns {
			if i := indexOf(columns, c); i >= 0 {
				m.cols = append(m.cols, c)
				m.idx = append(m.idx, i)
			}
		}
	}
	args := make([]interface{}, len(m.cols))
	for j, c := range m.cols {
		v := row[m.idx[j]]
		switch {
		case v.null:
			args[j] = nil
		case m.b64[c]:
			decoded, err := decodeOTRSBase64(v.data)
			if err != nil {
				return nil, nil, fmt.Errorf("%s.%s is not base64 as OTRS stores it on PostgreSQL: %w", m.table, c, err)
			}
			args[j] = decoded
		case m.target.binary[c]:
			args[j] = v.data
		default:
			args[j] = string(v.data)
		}
	}
	return m.cols, args, nil
}

// decodeOTRSBase64 decodes MIME::Base64 output (76-character lines).
func decodeOTRSBase64(b []byte) ([]byte, error) {
	clean := bytes.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, b)
	out := make([]byte, base64.StdEncoding.DecodedLen(len(clean)))
	n, err := base64.StdEncoding.Decode(out, clean)
	return out[:n], err
}

func containsString(list []string, s string) bool { return indexOf(list, s) >= 0 }

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

func (im *importer) importTable(p tablePlan) error {
	r := im.reports[p.name]
	switch p.mode {
	case modeMerge:
		return im.mergeTable(p, r)
	case modeSysconfig:
		return im.importSysconfig(r)
	case modeReplace:
		res, err := im.tx.Exec(database.ConvertPlaceholders("DELETE FROM `" + p.name + "`"))
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		r.deleted = int(n)
	}
	err := im.insertAll(p.name, r)
	if err == nil {
		fmt.Printf("  ✅ %-32s %d rows\n", p.name, r.inserted)
	}
	return err
}

// deferredValue is a self reference written once every row exists.
type deferredValue struct {
	id    interface{}
	col   string
	value interface{}
}

// insertAll inserts every source row of table, in batches. Columns
// referencing the table itself first get the row's own id, then their value.
func (im *importer) insertAll(table string, r *tableReport) error {
	m := im.mapper(table)
	self := im.schema[table].selfRefs(table)
	var deferred []deferredValue
	b := &batch{tx: im.tx, table: table}
	err := im.src.rows(table, func(columns []string, row []dumpValue) error {
		cols, args, err := m.row(columns, row)
		if err != nil {
			return err
		}
		r.rows++
		if len(self) > 0 {
			ki := indexOf(cols, "id")
			for i, c := range cols {
				if containsString(self, c) && args[i] != nil && ki >= 0 && fmt.Sprint(args[i]) != fmt.Sprint(args[ki]) {
					deferred = append(deferred, deferredValue{args[ki], c, args[i]})
					args[i] = args[ki]
				}
			}
		}
		r.inserted++
		return b.add(cols, args)
	})
	if err != nil {
		return err
	}
	if err := b.flush(); err != nil {
		return err
	}
	return im.writeDeferred(table, deferred)
}

func (im *importer) writeDeferred(table string, deferred []deferredValue) error {
	for _, d := range deferred {
		if _, err := im.tx.Exec(database.ConvertPlaceholders(
			"UPDATE `"+table+"` SET `"+d.col+"` = ? WHERE id = ?"), d.value, d.id); err != nil {
			return fmt.Errorf("id %v %s: %w", d.id, d.col, err)
		}
	}
	return nil
}

// batch collects rows for multi-row INSERT statements.
type batch struct {
	tx    *sql.Tx
	table string
	cols  []string
	args  []interface{}
	rows  int
	bytes int
}

const batchBytes = 4 << 20

func (b *batch) add(cols []string, args []interface{}) error {
	if b.rows > 0 && strings.Join(cols, ",") != strings.Join(b.cols, ",") {
		if err := b.flush(); err != nil {
			return err
		}
	}
	b.cols = cols
	b.args = append(b.args, args...)
	b.rows++
	for _, a := range args {
		switch v := a.(type) {
		case []byte:
			b.bytes += len(v)
		case string:
			b.bytes += len(v)
		}
	}
	maxRows := 60000 / len(cols)
	if maxRows > 500 {
		maxRows = 500
	}
	if b.rows >= maxRows || b.bytes >= batchBytes {
		return b.flush()
	}
	return nil
}

func (b *batch) flush() error {
	if b.rows == 0 {
		return nil
	}
	tuple := "(" + strings.TrimSuffix(strings.Repeat("?, ", len(b.cols)), ", ") + ")"
	query := "INSERT INTO `" + b.table + "` (`" + strings.Join(b.cols, "`, `") + "`) VALUES " +
		strings.TrimSuffix(strings.Repeat(tuple+", ", b.rows), ", ")
	if _, err := b.tx.Exec(database.ConvertPlaceholders(query), b.args...); err != nil {
		return err
	}
	b.args, b.rows, b.bytes = b.args[:0], 0, 0
	return nil
}

type sourceRow struct {
	cols []string
	args []interface{}
}

// mergeTable makes the target hold every OTRS row under its OTRS key.
func (im *importer) mergeTable(p tablePlan, r *tableReport) error {
	m := im.mapper(p.name)
	var rows []sourceRow
	if err := im.src.rows(p.name, func(columns []string, row []dumpValue) error {
		cols, args, err := m.row(columns, row)
		if err != nil {
			return err
		}
		rows = append(rows, sourceRow{cols, args})
		return nil
	}); err != nil {
		return err
	}
	r.rows = len(rows)
	keys := make(map[string]bool, len(rows))
	for _, row := range rows {
		keys[fmt.Sprint(row.args[indexOf(row.cols, p.key)])] = true
	}

	// Free the unique values the OTRS rows need from rows with other keys.
	if p.unique != "" {
		for _, row := range rows {
			ui := indexOf(row.cols, p.unique)
			if ui < 0 || row.args[ui] == nil {
				continue
			}
			key := row.args[indexOf(row.cols, p.key)]
			holders, err := im.queryKeys("SELECT `"+p.key+"` FROM `"+p.name+"` WHERE `"+p.unique+"` = ? AND `"+p.key+"` <> ?", row.args[ui], key)
			if err != nil {
				return err
			}
			for _, holder := range holders {
				renamed := preImportName(fmt.Sprint(row.args[ui]), holder)
				if _, err := im.tx.Exec(database.ConvertPlaceholders(
					"UPDATE `"+p.name+"` SET `"+p.unique+"` = ? WHERE `"+p.key+"` = ?"), renamed, holder); err != nil {
					return fmt.Errorf("rename %s %s: %w", p.key, holder, err)
				}
				if !keys[holder] {
					r.renamed = append(r.renamed, renamed)
				}
			}
		}
	}

	self := im.schema[p.name].selfRefs(p.name)
	var deferred []deferredValue
	for _, row := range rows {
		ki := indexOf(row.cols, p.key)
		key := row.args[ki]
		var n int
		if err := im.tx.QueryRow(database.ConvertPlaceholders(
			"SELECT COUNT(*) FROM `"+p.name+"` WHERE `"+p.key+"` = ?"), key).Scan(&n); err != nil {
			return fmt.Errorf("%s %v: %w", p.key, key, err)
		}
		args := append([]interface{}(nil), row.args...)
		for i, c := range row.cols {
			if containsString(self, c) && args[i] != nil {
				deferred = append(deferred, deferredValue{key, c, args[i]})
				args[i] = key // a valid self reference until every row exists
			}
		}
		if n > 0 {
			var sets []string
			var setArgs []interface{}
			for i, c := range row.cols {
				if i == ki || containsString(self, c) {
					continue
				}
				sets = append(sets, "`"+c+"` = ?")
				setArgs = append(setArgs, args[i])
			}
			if _, err := im.tx.Exec(database.ConvertPlaceholders(
				"UPDATE `"+p.name+"` SET "+strings.Join(sets, ", ")+" WHERE `"+p.key+"` = ?"), append(setArgs, key)...); err != nil {
				return fmt.Errorf("replace %s %v: %w", p.key, key, err)
			}
			r.updated++
			continue
		}
		if err := im.insertRow(p.name, row.cols, args); err != nil {
			return fmt.Errorf("%s %v: %w", p.key, key, err)
		}
		r.inserted++
	}
	if err := im.writeDeferred(p.name, deferred); err != nil {
		return err
	}
	fmt.Printf("  ✅ %-32s %d inserted, %d replaced\n", p.name, r.inserted, r.updated)
	return nil
}

func (im *importer) insertRow(table string, cols []string, args []interface{}) error {
	query := "INSERT INTO `" + table + "` (`" + strings.Join(cols, "`, `") + "`) VALUES (" +
		strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ") + ")"
	_, err := im.tx.Exec(database.ConvertPlaceholders(query), args...)
	return err
}

func (im *importer) queryKeys(query string, args ...interface{}) ([]string, error) {
	rows, err := im.tx.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// preImportName is the name given to an existing row whose name an OTRS row
// takes over; it fits the 200-character name columns.
func preImportName(name, id string) string {
	suffix := " (pre-import " + id + ")"
	r := []rune(name)
	if max := 200 - len([]rune(suffix)); len(r) > max {
		r = r[:max]
	}
	return string(r) + suffix
}

// importSysconfig imports the OTRS setting overrides. sysconfig_modified rows
// reference sysconfig_default by id, and the two databases number their
// definitions differently, so rows are matched by setting name: an override
// of a setting GoatFlow defines points at GoatFlow's definition; for any other
// setting the OTRS definition is copied first. GoatFlow reads OTRS settings
// (escalation calendars, ...) by name in OTRS's YAML format. The override
// replaces the target's override of the same setting and user.
func (im *importer) importSysconfig(r *tableReport) error {
	defIDs := map[string]int64{}
	rows, err := im.tx.Query(database.ConvertPlaceholders("SELECT id, name FROM sysconfig_default"))
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return err
		}
		defIDs[name] = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	m := im.mapper("sysconfig_modified")
	var modified []sourceRow
	missing := map[string]bool{}
	if err := im.src.rows("sysconfig_modified", func(columns []string, row []dumpValue) error {
		cols, args, err := m.row(columns, row)
		if err != nil {
			return err
		}
		ni := indexOf(cols, "name")
		if ni < 0 || args[ni] == nil {
			return fmt.Errorf("the source's sysconfig_modified has no name column")
		}
		if _, ok := defIDs[args[ni].(string)]; !ok {
			missing[args[ni].(string)] = true
		}
		modified = append(modified, sourceRow{cols, args})
		return nil
	}); err != nil {
		return err
	}
	r.rows = len(modified)

	if len(missing) > 0 && im.tables["sysconfig_default"] != nil {
		dm := im.mapper("sysconfig_default")
		if err := im.src.rows("sysconfig_default", func(columns []string, row []dumpValue) error {
			cols, args, err := dm.row(columns, row)
			if err != nil {
				return err
			}
			ni := indexOf(cols, "name")
			if ni < 0 || args[ni] == nil || !missing[args[ni].(string)] {
				return nil
			}
			var insCols []string
			var insArgs []interface{}
			for i, c := range cols {
				if c != "id" {
					insCols = append(insCols, c)
					insArgs = append(insArgs, args[i])
				}
			}
			id, err := database.GetAdapter().InsertWithReturningTx(im.tx, database.ConvertPlaceholders(
				"INSERT INTO sysconfig_default (`"+strings.Join(insCols, "`, `")+"`) VALUES ("+
					strings.TrimSuffix(strings.Repeat("?, ", len(insCols)), ", ")+") RETURNING id"), insArgs...)
			if err != nil {
				return fmt.Errorf("copy definition %v: %w", args[ni], err)
			}
			defIDs[args[ni].(string)] = id
			r.copied++
			return nil
		}); err != nil {
			return err
		}
	}

	for _, row := range modified {
		name := row.args[indexOf(row.cols, "name")].(string)
		defID, ok := defIDs[name]
		if !ok {
			r.unmatched = append(r.unmatched, name)
			continue
		}
		var cols []string
		var args []interface{}
		var userID interface{}
		for i, c := range row.cols {
			switch c {
			case "id":
				continue
			case "sysconfig_default_id":
				args = append(args, defID)
			default:
				args = append(args, row.args[i])
			}
			if c == "user_id" {
				userID = row.args[i]
			}
			cols = append(cols, c)
		}
		if indexOf(cols, "sysconfig_default_id") < 0 {
			cols = append(cols, "sysconfig_default_id")
			args = append(args, defID)
		}
		var existing []string
		if userID == nil {
			existing, err = im.queryKeys("SELECT id FROM sysconfig_modified WHERE sysconfig_default_id = ? AND user_id IS NULL", defID)
		} else {
			existing, err = im.queryKeys("SELECT id FROM sysconfig_modified WHERE sysconfig_default_id = ? AND user_id = ?", defID, userID)
		}
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			sets := make([]string, len(cols))
			for i, c := range cols {
				sets[i] = "`" + c + "` = ?"
			}
			if _, err := im.tx.Exec(database.ConvertPlaceholders(
				"UPDATE sysconfig_modified SET "+strings.Join(sets, ", ")+" WHERE id = ?"), append(args, existing[0])...); err != nil {
				return fmt.Errorf("setting %s: %w", name, err)
			}
			r.updated++
			continue
		}
		if err := im.insertRow("sysconfig_modified", cols, args); err != nil {
			return fmt.Errorf("setting %s: %w", name, err)
		}
		r.inserted++
	}
	sort.Strings(r.unmatched)
	fmt.Printf("  ✅ %-32s %d inserted, %d overwritten, %d definitions copied\n", "sysconfig_modified", r.inserted, r.updated, r.copied)
	return nil
}

// resetIDSequences moves the id generator of every imported table past the
// highest id, so the next row GoatFlow creates gets max(id)+1. PostgreSQL
// sequences do not see explicit ids; on MySQL the AUTO_INCREMENT counter only
// ever grows, and still holds the ids of rows the force mode deleted.
func (im *importer) resetIDSequences() error {
	for _, p := range im.order {
		names := []string{p.name}
		if p.mode == modeSysconfig {
			names = append(names, "sysconfig_default")
		}
		for _, name := range names {
			if t := im.schema[name]; t == nil || !t.autoID {
				continue
			}
			var next int64
			if err := im.db.QueryRow(database.ConvertPlaceholders(
				"SELECT COALESCE(MAX(id), 0) + 1 FROM `" + name + "`")).Scan(&next); err != nil {
				return fmt.Errorf("reset id sequence of %s: %w", name, err)
			}
			var err error
			if database.IsPostgreSQL() {
				err = im.db.QueryRow(database.ConvertPlaceholders(
					"SELECT setval(pg_get_serial_sequence(?, 'id'), ?, false)"), name, next).Scan(new(int64))
			} else {
				// AUTO_INCREMENT cannot be a bind parameter; next is an integer from the database.
				_, err = im.db.Exec(database.ConvertPlaceholders("ALTER TABLE `" + name + "` AUTO_INCREMENT = " + strconv.FormatInt(next, 10)))
			}
			if err != nil {
				return fmt.Errorf("reset id sequence of %s: %w", name, err)
			}
		}
	}
	return nil
}
