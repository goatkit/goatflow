package plugin

import (
	"sort"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/sqllex"
)

// sqlTables returns the tables a statement reads and the tables it writes
// (INSERT/UPDATE/DELETE targets and DDL targets). It is a lexer, not a full
// parser: it understands comments, string literals, quoted identifiers,
// schema-qualified names, comma joins, aliases, CTE names, subqueries,
// parenthesised table references (FROM (t), FROM (a JOIN b ON ...)) and FROM
// inside function calls (EXTRACT(YEAR FROM x)). Statements that read a table
// without FROM (TABLE t, HANDLER t OPEN, COPY t TO, DESCRIBE t) name it too,
// and statements that run SQL the lexer cannot see (CALL, PREPARE, EXECUTE)
// are reported as a pseudo table, so unknown syntax fails closed against a
// scope.
func sqlTables(query string) (read, write []string) {
	toks := sqllex.Tokenize(query)
	readSet := map[string]bool{}
	writeSet := map[string]bool{}

	word := func(i int, w string) bool {
		return i >= 0 && i < len(toks) && toks[i].Kind == 'w' && toks[i].Text == w
	}
	punct := func(i int, p string) bool {
		return i >= 0 && i < len(toks) && toks[i].Kind == 'p' && toks[i].Text == p
	}
	ident := func(i int) bool { return i >= 0 && i < len(toks) && toks[i].IsIdent() }

	// CTE names: <name> AS ( after WITH, RECURSIVE or a comma.
	cte := map[string]bool{}
	for i := range toks {
		if ident(i) && word(i+1, "AS") && punct(i+2, "(") &&
			(word(i-1, "WITH") || word(i-1, "RECURSIVE") || punct(i-1, ",")) {
			cte[toks[i].Name] = true
		}
	}

	// tableList records the table at j (and, when list is set, the
	// comma-separated tables after it). skipFunc ignores name(...) table
	// functions such as generate_series(...). A parenthesised table
	// reference — FROM (t), JOIN (a JOIN b ON ...) — is entered, so the
	// table inside is recorded like one written without the parentheses.
	tableList := func(j int, set map[string]bool, list, skipFunc bool) {
		for {
			for word(j, "ONLY") || word(j, "LATERAL") || word(j, "LOW_PRIORITY") || word(j, "IGNORE") {
				j++
			}
			for punct(j, "(") && !word(j+1, "SELECT") && !word(j+1, "WITH") && !word(j+1, "VALUES") {
				j++
			}
			if !ident(j) {
				return
			}
			if skipFunc && punct(j+1, "(") {
				return
			}
			set[toks[j].Name] = true
			j++
			if word(j, "AS") {
				j += 2
			} else if j < len(toks) && (toks[j].Kind == 'q' || (toks[j].Kind == 'w' && !sqllex.AliasStop[toks[j].Text])) {
				j++
			}
			for punct(j, ")") {
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
	// (top level, a subquery, or a parenthesised table reference list);
	// false inside function calls and column lists.
	stack := []bool{true}
	inUpdateList := false
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.Kind == 'p' {
			switch t.Text {
			case "(":
				top := stack[len(stack)-1]
				tableRef := word(i-1, "FROM") || word(i-1, "JOIN") || word(i-1, "USING") ||
					word(i-1, "UPDATE") || word(i-1, "INTO") ||
					(top && (punct(i-1, "(") || punct(i-1, ",")))
				stack = append(stack, word(i+1, "SELECT") || word(i+1, "WITH") || tableRef)
			case ")":
				if len(stack) > 1 {
					stack = stack[:len(stack)-1]
				}
			case ";":
				inUpdateList = false
			}
			continue
		}
		if t.Kind != 'w' {
			continue
		}
		// Foreign keys may sit inside a CREATE TABLE column list.
		if t.Text == "REFERENCES" {
			tableList(i+1, readSet, false, false)
			continue
		}
		if !stack[len(stack)-1] {
			continue
		}
		switch t.Text {
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
				word(i-1, "TEMPORARY"), word(i-1, "TEMP"), word(i-1, "UNLOGGED"), word(i-1, "INTO"):
				// DDL targets, and LOAD DATA ... INTO TABLE t.
				tableList(skipIfExists(i+1), writeSet, true, false)
			case word(i-1, "RENAME"):
				for j := i + 1; j < len(toks) && !punct(j, ";"); j++ {
					if ident(j) && !word(j, "TO") {
						writeSet[toks[j].Name] = true
					}
				}
			default:
				// The TABLE statement (PostgreSQL, MySQL 8.0.19+): TABLE t is
				// SELECT * FROM t. LOCK TABLE and COMMENT ON TABLE land here
				// too and are reported as reads, which fails closed.
				tableList(skipIfExists(i+1), readSet, true, false)
			}
		case "HANDLER", "DESCRIBE", "DESC":
			// HANDLER t OPEN / READ reads t without FROM; DESCRIBE t exposes
			// its columns. DESC is only the statement when it opens one
			// (elsewhere it is ORDER BY ... DESC).
			if t.Text != "DESC" || i == 0 || punct(i-1, ";") {
				tableList(i+1, readSet, false, true)
			}
		case "COPY":
			// COPY t TO ... reads t, COPY t FROM ... writes it; both are
			// reported as writes, the stricter of the two.
			tableList(i+1, writeSet, false, true)
		case "CALL", "PREPARE", "EXECUTE", "DEALLOCATE", "LOAD":
			// Stored procedures and server-side prepared statements run SQL
			// this lexer never sees; LOAD DATA reads server files. Report a
			// pseudo table so only an unscoped grant allows them.
			readSet["statement:"+strings.ToLower(t.Text)] = true
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
