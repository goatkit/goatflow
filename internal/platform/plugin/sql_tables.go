package plugin

import (
	"sort"
	"strings"
)

// sqlToken is one lexical token of a SQL statement.
type sqlToken struct {
	kind byte   // 'w' word, 'q' quoted identifier, 's' string literal, 'p' punctuation
	text string // word: upper case; punctuation: the character
	name string // identifier, lower case, schema-qualified as "schema.table"
}

func (t sqlToken) isIdent() bool { return t.kind == 'w' || t.kind == 'q' }

// sqlTables returns the tables a statement reads and the tables it writes
// (INSERT/UPDATE/DELETE targets and DDL targets). It is a lexer, not a full
// parser: it understands comments, string literals, quoted identifiers,
// schema-qualified names, comma joins, aliases, CTE names, subqueries and
// FROM inside function calls (EXTRACT(YEAR FROM x)). Anything it cannot place
// is reported as a table, so unknown syntax fails closed against a scope.
func sqlTables(query string) (read, write []string) {
	toks := tokenizeSQL(query)
	readSet := map[string]bool{}
	writeSet := map[string]bool{}

	word := func(i int, w string) bool {
		return i >= 0 && i < len(toks) && toks[i].kind == 'w' && toks[i].text == w
	}
	punct := func(i int, p string) bool {
		return i >= 0 && i < len(toks) && toks[i].kind == 'p' && toks[i].text == p
	}
	ident := func(i int) bool { return i >= 0 && i < len(toks) && toks[i].isIdent() }

	// CTE names: <name> AS ( after WITH, RECURSIVE or a comma.
	cte := map[string]bool{}
	for i := range toks {
		if ident(i) && word(i+1, "AS") && punct(i+2, "(") &&
			(word(i-1, "WITH") || word(i-1, "RECURSIVE") || punct(i-1, ",")) {
			cte[toks[i].name] = true
		}
	}

	// tableList records the table at j (and, when list is set, the
	// comma-separated tables after it). skipFunc ignores name(...) table
	// functions such as generate_series(...).
	tableList := func(j int, set map[string]bool, list, skipFunc bool) {
		for {
			for word(j, "ONLY") || word(j, "LATERAL") || word(j, "LOW_PRIORITY") || word(j, "IGNORE") {
				j++
			}
			if !ident(j) {
				return
			}
			if skipFunc && punct(j+1, "(") {
				return
			}
			set[toks[j].name] = true
			j++
			if word(j, "AS") {
				j += 2
			} else if j < len(toks) && (toks[j].kind == 'q' || (toks[j].kind == 'w' && !sqlAliasStop[toks[j].text])) {
				j++
			}
			if !list || !punct(j, ",") {
				return
			}
			j++
		}
	}
	// skipIfExists moves past IF [NOT] EXISTS.
	skipIfExists := func(j int) int {
		if word(j, "IF") {
			j++
			if word(j, "NOT") {
				j++
			}
			if word(j, "EXISTS") {
				j++
			}
		}
		return j
	}

	// Paren stack: true when the innermost parenthesis is statement context
	// (top level or a subquery); false inside function calls and column lists.
	stack := []bool{true}
	inUpdateList := false
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == 'p' {
			switch t.text {
			case "(":
				stack = append(stack, word(i+1, "SELECT") || word(i+1, "WITH"))
			case ")":
				if len(stack) > 1 {
					stack = stack[:len(stack)-1]
				}
			case ";":
				inUpdateList = false
			}
			continue
		}
		if t.kind != 'w' {
			continue
		}
		// Foreign keys may sit inside a CREATE TABLE column list.
		if t.text == "REFERENCES" {
			tableList(i+1, readSet, false, false)
			continue
		}
		if !stack[len(stack)-1] {
			continue
		}
		switch t.text {
		case "FROM":
			if word(i-1, "DISTINCT") && (word(i-2, "IS") || word(i-2, "NOT")) {
				continue
			}
			if word(i-1, "DELETE") {
				tableList(i+1, writeSet, true, false)
			} else {
				tableList(i+1, readSet, true, true)
			}
		case "JOIN":
			if inUpdateList {
				tableList(i+1, writeSet, false, true)
			} else {
				tableList(i+1, readSet, false, true)
			}
		case "USING":
			if !punct(i+1, "(") {
				tableList(i+1, readSet, true, true)
			}
		case "INTO":
			if word(i-1, "INSERT") || word(i-1, "REPLACE") || word(i-1, "MERGE") || word(i-1, "IGNORE") {
				tableList(i+1, writeSet, false, false)
			}
		case "UPDATE":
			if word(i-1, "KEY") || word(i-1, "DO") || word(i-1, "FOR") || word(i-1, "ON") {
				continue
			}
			inUpdateList = true
			tableList(i+1, writeSet, true, false)
		case "SET":
			inUpdateList = false
		case "TABLE":
			switch {
			case word(i-1, "CREATE"), word(i-1, "ALTER"), word(i-1, "DROP"), word(i-1, "TRUNCATE"),
				word(i-1, "TEMPORARY"), word(i-1, "TEMP"), word(i-1, "UNLOGGED"):
				tableList(skipIfExists(i+1), writeSet, true, false)
			case word(i-1, "RENAME"):
				for j := i + 1; j < len(toks) && !punct(j, ";"); j++ {
					if ident(j) && !word(j, "TO") {
						writeSet[toks[j].name] = true
					}
				}
			}
		case "TRUNCATE":
			if !word(i+1, "TABLE") {
				tableList(i+1, writeSet, true, false)
			}
		case "VIEW":
			if word(i-1, "CREATE") || word(i-1, "REPLACE") || word(i-1, "DROP") || word(i-1, "ALTER") {
				tableList(skipIfExists(i+1), writeSet, false, false)
			}
		case "INDEX":
			if word(i-1, "CREATE") || word(i-1, "UNIQUE") || word(i-1, "DROP") {
				for j := i + 1; j < len(toks) && !punct(j, ";") && !punct(j, "("); j++ {
					if word(j, "ON") {
						tableList(j+1, writeSet, false, false)
						break
					}
				}
			}
		}
	}

	for name := range readSet {
		if !cte[name] && name != "dual" {
			read = append(read, name)
		}
	}
	for name := range writeSet {
		if !cte[name] {
			write = append(write, name)
		}
	}
	sort.Strings(read)
	sort.Strings(write)
	return read, write
}

// sqlAliasStop lists the words that end a table reference, so they are never
// taken for an alias.
var sqlAliasStop = map[string]bool{
	"WHERE": true, "ON": true, "USING": true, "JOIN": true, "INNER": true, "LEFT": true,
	"RIGHT": true, "FULL": true, "CROSS": true, "OUTER": true, "NATURAL": true,
	"STRAIGHT_JOIN": true, "GROUP": true, "ORDER": true, "LIMIT": true, "OFFSET": true,
	"HAVING": true, "UNION": true, "EXCEPT": true, "INTERSECT": true, "SET": true,
	"VALUES": true, "FOR": true, "WINDOW": true, "RETURNING": true, "INTO": true,
	"SELECT": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true, "AND": true,
	"OR": true, "FETCH": true, "LOCK": true, "PARTITION": true, "TABLESAMPLE": true,
	"USE": true, "FORCE": true, "IGNORE": true, "DEFAULT": true, "TO": true, "ADD": true,
	"DROP": true, "MODIFY": true, "CHANGE": true, "ALTER": true, "RENAME": true,
}

// tokenizeSQL splits a statement into words, quoted identifiers, string
// literals and punctuation. Comments are dropped. Dotted names are joined
// ("otrs.users"); a trailing ".*" is ignored.
func tokenizeSQL(q string) []sqlToken {
	var toks []sqlToken
	i := 0
	isWordByte := func(c byte) bool {
		return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
	}
	// readName reads one identifier part at i (word or quoted).
	readName := func() (string, bool, bool) {
		if i >= len(q) {
			return "", false, false
		}
		c := q[i]
		if c == '"' || c == '`' {
			end := strings.IndexByte(q[i+1:], c)
			if end < 0 {
				s := q[i+1:]
				i = len(q)
				return s, true, true
			}
			s := q[i+1 : i+1+end]
			i += end + 2
			return s, true, true
		}
		if isWordByte(c) {
			start := i
			for i < len(q) && isWordByte(q[i]) {
				i++
			}
			return q[start:i], false, true
		}
		return "", false, false
	}
	for i < len(q) {
		c := q[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f':
			i++
		case c == '-' && i+1 < len(q) && q[i+1] == '-', c == '#':
			for i < len(q) && q[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(q) && q[i+1] == '*':
			end := strings.Index(q[i+2:], "*/")
			if end < 0 {
				i = len(q)
			} else {
				i += end + 4
			}
		case c == '\'':
			i++
			for i < len(q) {
				if q[i] == '\\' {
					i += 2
					continue
				}
				if q[i] == '\'' {
					if i+1 < len(q) && q[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			toks = append(toks, sqlToken{kind: 's'})
		case c == '"' || c == '`' || isWordByte(c):
			part, quoted, _ := readName()
			kind := byte('w')
			if quoted {
				kind = 'q'
			}
			name := part
			for i+1 < len(q) && q[i] == '.' && (q[i+1] == '"' || q[i+1] == '`' || isWordByte(q[i+1])) {
				i++
				next, nq, _ := readName()
				name += "." + next
				if nq {
					kind = 'q'
				}
			}
			tok := sqlToken{kind: kind, name: strings.ToLower(name)}
			if kind == 'w' {
				tok.text = strings.ToUpper(name)
			}
			toks = append(toks, tok)
		default:
			toks = append(toks, sqlToken{kind: 'p', text: string(c)})
			i++
		}
	}
	return toks
}
