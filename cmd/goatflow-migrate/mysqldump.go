package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

// dumpValue is one value of a source row: SQL NULL or the exact bytes of a
// string, number or binary value.
type dumpValue struct {
	null bool
	data []byte
}

// dumpSpan locates one INSERT statement in the dump file.
type dumpSpan struct {
	off, n int64
}

// dumpTable is what a dump holds for one table.
type dumpTable struct {
	columns    []string // from CREATE TABLE, in dump order
	statements []dumpSpan
}

// mysqlDump is an indexed mysqldump / mariadb-dump file. Rows are read on
// demand, table by table, so the import can follow foreign-key order rather
// than the dump's alphabetical order.
type mysqlDump struct {
	file   *os.File
	dumped map[string]*dumpTable
}

var (
	createTablePrefix = []byte("CREATE TABLE ")
	insertPrefix      = []byte("INSERT INTO `")
)

// openMySQLDump indexes the CREATE TABLE column lists and INSERT statements of
// a dump. Statements may span several lines (mariadb-dump 11 writes one row per
// line); string values never contain a raw newline in mysqldump output, but a
// quote-aware scan finds the terminating ';' either way.
func openMySQLDump(path string) (*mysqlDump, error) {
	f, err := os.Open(path) //nolint:gosec // G304 CLI tool reads the dump the operator names
	if err != nil {
		return nil, fmt.Errorf("open SQL dump: %w", err)
	}
	d := &mysqlDump{file: f, dumped: make(map[string]*dumpTable)}
	if err := d.index(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return d, nil
}

func (d *mysqlDump) Close() error { return d.file.Close() }

func (d *mysqlDump) describe() string { return "mysqldump file " + d.file.Name() }

// tables lists the dumped tables. A dump written without CREATE TABLE
// (--no-create-info) names its columns in the INSERT statements.
func (d *mysqlDump) tables() map[string]*sourceTable {
	out := make(map[string]*sourceTable, len(d.dumped))
	for name, t := range d.dumped {
		cols := t.columns
		if cols == nil {
			_ = d.rows(name, func(columns []string, _ []dumpValue) error {
				cols = columns
				return errStopRows
			})
		}
		out[name] = &sourceTable{columns: cols, base64: map[string]bool{}}
	}
	return out
}

var errStopRows = errors.New("stop")

func (d *mysqlDump) count(table string) (int, error) {
	n := 0
	err := d.rows(table, func([]string, []dumpValue) error {
		n++
		return nil
	})
	return n, err
}

func (d *mysqlDump) table(name string) *dumpTable {
	t := d.dumped[name]
	if t == nil {
		t = &dumpTable{}
		d.dumped[name] = t
	}
	return t
}

func (d *mysqlDump) index() error {
	r := bufio.NewReaderSize(d.file, 1<<20)
	var (
		off      int64
		creating *dumpTable // inside CREATE TABLE
		inserted *dumpTable // inside a multi-line INSERT
		stmt     dumpSpan
		scan     statementScanner
	)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			start := off
			off += int64(len(line))
			switch {
			case inserted != nil:
				stmt.n += int64(len(line))
				if scan.feed(line) {
					inserted.statements = append(inserted.statements, stmt)
					inserted = nil
				}
			case creating != nil:
				t := bytes.TrimSpace(line)
				if len(t) > 0 && t[0] == ')' {
					creating = nil
				} else if name, ok := backtickName(t); ok {
					creating.columns = append(creating.columns, name)
				}
			case bytes.HasPrefix(line, createTablePrefix):
				rest := bytes.TrimPrefix(line[len(createTablePrefix):], []byte("IF NOT EXISTS "))
				if name, ok := backtickName(rest); ok {
					creating = d.table(name)
					creating.columns = nil
				}
			case bytes.HasPrefix(line, insertPrefix):
				name, ok := backtickName(line[len(insertPrefix)-1:])
				if !ok {
					return fmt.Errorf("SQL dump offset %d: malformed INSERT", start)
				}
				stmt = dumpSpan{off: start, n: int64(len(line))}
				scan = statementScanner{}
				if scan.feed(line[len(insertPrefix)+len(name):]) {
					d.table(name).statements = append(d.table(name).statements, stmt)
				} else {
					inserted = d.table(name)
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read SQL dump: %w", err)
		}
	}
	if inserted != nil {
		return fmt.Errorf("SQL dump ends inside an INSERT statement")
	}
	return nil
}

// backtickName returns the identifier of a line starting with `name`.
func backtickName(b []byte) (string, bool) {
	if len(b) < 2 || b[0] != '`' {
		return "", false
	}
	end := bytes.IndexByte(b[1:], '`')
	if end < 0 {
		return "", false
	}
	return string(b[1 : end+1]), true
}

// statementScanner finds the ';' that ends a statement outside string literals.
type statementScanner struct {
	inQuote bool
	escaped bool
}

func (s *statementScanner) feed(b []byte) bool {
	for _, c := range b {
		switch {
		case s.escaped:
			s.escaped = false
		case s.inQuote && c == '\\':
			s.escaped = true
		case c == '\'':
			s.inQuote = !s.inQuote
		case !s.inQuote && c == ';':
			return true
		}
	}
	return false
}

// rows calls fn for every row the dump holds for table, with the row's
// column names (the INSERT column list, or else the CREATE TABLE columns).
func (d *mysqlDump) rows(table string, fn func(columns []string, row []dumpValue) error) error {
	t := d.dumped[table]
	if t == nil {
		return nil
	}
	for _, s := range t.statements {
		buf := make([]byte, s.n)
		if _, err := d.file.ReadAt(buf, s.off); err != nil {
			return fmt.Errorf("read INSERT INTO `%s` at offset %d: %w", table, s.off, err)
		}
		if err := parseInsert(buf, t.columns, func(columns []string, row []dumpValue) error {
			if columns == nil {
				return fmt.Errorf("the dump has no CREATE TABLE for `%s` and its INSERT names no columns", table)
			}
			if len(row) != len(columns) {
				return fmt.Errorf("`%s` row has %d values for %d columns", table, len(row), len(columns))
			}
			return fn(columns, row)
		}); errors.Is(err, errStopRows) {
			return nil
		} else if err != nil {
			return fmt.Errorf("INSERT INTO `%s` at offset %d: %w", table, s.off, err)
		}
	}
	return nil
}

// parseInsert parses one "INSERT INTO `t` [(`c`,...)] VALUES (...),(...);" statement.
func parseInsert(stmt []byte, columns []string, fn func(columns []string, row []dumpValue) error) error {
	p := &valueParser{s: stmt}
	open := bytes.IndexByte(stmt, '`')
	if open < 0 {
		return errors.New("malformed INSERT")
	}
	end := bytes.IndexByte(stmt[open+1:], '`')
	if end < 0 {
		return errors.New("malformed INSERT")
	}
	p.pos = open + end + 2
	p.skipSpace()
	if p.peek() == '(' {
		columns = nil
		p.pos++
		for {
			p.skipSpace()
			name, ok := backtickName(p.s[p.pos:])
			if !ok {
				return p.errorf("column name expected")
			}
			columns = append(columns, name)
			p.pos += len(name) + 2
			p.skipSpace()
			if c := p.next(); c == ')' {
				break
			} else if c != ',' {
				return p.errorf("',' or ')' expected in column list")
			}
		}
		p.skipSpace()
	}
	if !p.keyword("VALUES") {
		return p.errorf("VALUES expected")
	}
	for {
		p.skipSpace()
		if p.next() != '(' {
			return p.errorf("'(' expected")
		}
		row, err := p.tuple()
		if err != nil {
			return err
		}
		if err := fn(columns, row); err != nil {
			return err
		}
		p.skipSpace()
		switch p.next() {
		case ',':
		case ';', 0:
			return nil
		default:
			return p.errorf("',' or ';' expected after row")
		}
	}
}

// valueParser reads MySQL literals from a dumped statement, byte by byte so
// that binary strings round-trip exactly.
type valueParser struct {
	s   []byte
	pos int
}

func (p *valueParser) errorf(format string, args ...interface{}) error {
	return fmt.Errorf("byte %d: "+format, append([]interface{}{p.pos}, args...)...)
}

func (p *valueParser) peek() byte {
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

func (p *valueParser) next() byte {
	c := p.peek()
	if p.pos < len(p.s) {
		p.pos++
	}
	return c
}

func (p *valueParser) skipSpace() {
	for p.pos < len(p.s) {
		switch p.s[p.pos] {
		case ' ', '\t', '\r', '\n':
			p.pos++
		default:
			return
		}
	}
}

func (p *valueParser) keyword(kw string) bool {
	if len(p.s)-p.pos >= len(kw) && bytes.EqualFold(p.s[p.pos:p.pos+len(kw)], []byte(kw)) {
		p.pos += len(kw)
		return true
	}
	return false
}

// tuple parses the values of one row after its '(' up to and including ')'.
func (p *valueParser) tuple() ([]dumpValue, error) {
	var row []dumpValue
	for {
		p.skipSpace()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		row = append(row, v)
		p.skipSpace()
		switch p.next() {
		case ',':
		case ')':
			return row, nil
		default:
			return nil, p.errorf("',' or ')' expected after value")
		}
	}
}

func (p *valueParser) value() (dumpValue, error) {
	// A character set introducer (_binary, _utf8mb4, ...) only labels the literal.
	if p.peek() == '_' {
		i := p.pos + 1
		for i < len(p.s) && (p.s[i] == '_' || p.s[i] >= 'a' && p.s[i] <= 'z' || p.s[i] >= 'A' && p.s[i] <= 'Z' || p.s[i] >= '0' && p.s[i] <= '9') {
			i++
		}
		p.pos = i
		p.skipSpace()
	}
	switch c := p.peek(); {
	case c == '\'':
		p.pos++
		return p.quoted()
	case (c == 'x' || c == 'X') && p.pos+1 < len(p.s) && p.s[p.pos+1] == '\'':
		p.pos += 2
		end := bytes.IndexByte(p.s[p.pos:], '\'')
		if end < 0 {
			return dumpValue{}, p.errorf("unterminated hex literal")
		}
		data, err := hex.DecodeString(string(p.s[p.pos : p.pos+end]))
		p.pos += end + 1
		if err != nil {
			return dumpValue{}, p.errorf("hex literal: %v", err)
		}
		return dumpValue{data: data}, nil
	case c == '0' && p.pos+1 < len(p.s) && (p.s[p.pos+1] == 'x' || p.s[p.pos+1] == 'X'):
		p.pos += 2
		start := p.pos
		for p.pos < len(p.s) && isHexDigit(p.s[p.pos]) {
			p.pos++
		}
		data, err := hex.DecodeString(string(p.s[start:p.pos]))
		if err != nil {
			return dumpValue{}, p.errorf("hex literal: %v", err)
		}
		return dumpValue{data: data}, nil
	}
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == ',' || c == ')' || c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			break
		}
		p.pos++
	}
	token := p.s[start:p.pos]
	if len(token) == 0 {
		return dumpValue{}, p.errorf("value expected")
	}
	if bytes.EqualFold(token, []byte("NULL")) {
		return dumpValue{null: true}, nil
	}
	return dumpValue{data: append([]byte(nil), token...)}, nil
}

// quoted decodes a single-quoted string after its opening quote, applying the
// escapes mysqldump writes (\0 \' \" \b \n \r \t \Z \\) and doubled quotes.
func (p *valueParser) quoted() (dumpValue, error) {
	out := make([]byte, 0, 64)
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		p.pos++
		switch c {
		case '\\':
			if p.pos >= len(p.s) {
				return dumpValue{}, p.errorf("unterminated string")
			}
			e := p.s[p.pos]
			p.pos++
			switch e {
			case '0':
				out = append(out, 0)
			case 'b':
				out = append(out, '\b')
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'Z':
				out = append(out, 0x1a)
			case '%', '_':
				// MySQL keeps the backslash of \% and \_ (LIKE escapes).
				out = append(out, '\\', e)
			default:
				out = append(out, e)
			}
		case '\'':
			if p.peek() == '\'' {
				p.pos++
				out = append(out, '\'')
				continue
			}
			return dumpValue{data: out}, nil
		default:
			out = append(out, c)
		}
	}
	return dumpValue{}, p.errorf("unterminated string")
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
