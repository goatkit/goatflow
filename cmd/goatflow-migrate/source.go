package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// source is an OTRS 6 / Znuny 6 database to import from: a live database
// (MySQL/MariaDB or PostgreSQL) or a mysqldump file.
type source interface {
	// tables lists the source's tables with their columns.
	tables() map[string]*sourceTable
	// rows calls fn for every row of table with the row's column names.
	rows(table string, fn func(columns []string, row []dumpValue) error) error
	// count returns the number of rows of table.
	count(table string) (int, error)
	describe() string
	Close() error
}

// sourceTable describes one source table.
type sourceTable struct {
	columns []string
	// base64 holds the columns whose values OTRS stored base64-encoded.
	// OTRS writes binary content base64-encoded on databases without
	// DirectBlob support (PostgreSQL), into text columns.
	base64 map[string]bool
}

// otrsBase64Columns are the columns OTRS (Kernel::System::Ticket::Article::
// Backend::MIMEBase::ArticleStorageDB, StdAttachment, VirtualFS::DB) encodes
// with MIME::Base64 when the database has no DirectBlob support.
var otrsBase64Columns = map[string][]string{
	"article_data_mime_plain":      {"body"},
	"article_data_mime_attachment": {"content"},
	"standard_attachment":          {"content"},
	"virtual_fs_db":                {"content"},
}

// sourceDriver returns the database/sql driver for a source or target DSN:
// a MySQL DSN (user:pass@tcp(host:3306)/db, optionally mysql://-prefixed)
// selects mysql; anything else is a PostgreSQL URL or keyword DSN.
func sourceDriver(dsn string) (driver, dataSource string) {
	if strings.HasPrefix(dsn, "mysql://") || strings.Contains(dsn, "@tcp(") || strings.Contains(dsn, "@unix(") {
		return "mysql", strings.TrimPrefix(dsn, "mysql://")
	}
	return "postgres", dsn
}

// dbSource reads an OTRS database directly.
type dbSource struct {
	db     *sql.DB
	driver string
	name   string
	tabs   map[string]*sourceTable
}

func openDBSource(dsn string) (*dbSource, error) {
	driver, dataSource := sourceDriver(dsn)
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		return nil, fmt.Errorf("open source database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to source database: %w", err)
	}
	s := &dbSource{db: db, driver: driver, tabs: make(map[string]*sourceTable)}
	if err := s.load(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *dbSource) load() error {
	// The source's own dialect: it need not be the target driver the
	// database package converts for. Neither query takes parameters.
	query := `SELECT DATABASE(), table_name, column_name, data_type FROM information_schema.columns
		WHERE table_schema = DATABASE() ORDER BY table_name, ordinal_position`
	if s.driver == "postgres" {
		query = `SELECT current_database(), table_name, column_name, data_type FROM information_schema.columns
			WHERE table_schema = current_schema() ORDER BY table_name, ordinal_position`
	}
	rows, err := s.db.Query(query) // sql-converted: source-database dialect, no parameters
	if err != nil {
		return fmt.Errorf("read source schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, column, dataType string
		if err := rows.Scan(&s.name, &table, &column, &dataType); err != nil {
			return fmt.Errorf("read source schema: %w", err)
		}
		t := s.tabs[table]
		if t == nil {
			t = &sourceTable{base64: map[string]bool{}}
			s.tabs[table] = t
		}
		t.columns = append(t.columns, column)
		if s.driver == "postgres" && dataType != "bytea" && containsString(otrsBase64Columns[table], column) {
			t.base64[column] = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read source schema: %w", err)
	}
	if len(s.tabs) == 0 {
		return fmt.Errorf("the source database has no tables")
	}
	return nil
}

func (s *dbSource) describe() string { return s.driver + " database " + s.name }

func (s *dbSource) Close() error { return s.db.Close() }

func (s *dbSource) tables() map[string]*sourceTable { return s.tabs }

func (s *dbSource) ident(name string) string {
	if s.driver == "postgres" {
		return `"` + name + `"`
	}
	return "`" + name + "`"
}

func (s *dbSource) count(table string) (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM " + s.ident(table)).Scan(&n) // sql-converted: source-database dialect, no parameters
	return n, err
}

func (s *dbSource) rows(table string, fn func(columns []string, row []dumpValue) error) error {
	t := s.tabs[table]
	if t == nil {
		return nil
	}
	cols := make([]string, len(t.columns))
	for i, c := range t.columns {
		cols[i] = s.ident(c)
	}
	query := "SELECT " + strings.Join(cols, ", ") + " FROM " + s.ident(table)
	rows, err := s.db.Query(query) // sql-converted: source-database dialect, no parameters
	if err != nil {
		return fmt.Errorf("read source table %s: %w", table, err)
	}
	defer rows.Close()
	raw := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("read source table %s: %w", table, err)
		}
		row := make([]dumpValue, len(cols))
		for i, v := range raw {
			row[i] = sourceValue(v)
		}
		if err := fn(t.columns, row); err != nil {
			return err
		}
	}
	return rows.Err()
}

// sourceValue turns a driver value into the bytes the import writes.
func sourceValue(v interface{}) dumpValue {
	switch x := v.(type) {
	case nil:
		return dumpValue{null: true}
	case []byte:
		return dumpValue{data: append([]byte(nil), x...)}
	case string:
		return dumpValue{data: []byte(x)}
	case int64:
		return dumpValue{data: strconv.AppendInt(nil, x, 10)}
	case float64:
		return dumpValue{data: strconv.AppendFloat(nil, x, 'f', -1, 64)}
	case bool:
		if x {
			return dumpValue{data: []byte("1")}
		}
		return dumpValue{data: []byte("0")}
	case time.Time:
		layout := "2006-01-02 15:04:05"
		if x.Nanosecond() != 0 {
			layout += ".999999"
		}
		return dumpValue{data: []byte(x.Format(layout))}
	default:
		return dumpValue{data: []byte(fmt.Sprint(x))}
	}
}
