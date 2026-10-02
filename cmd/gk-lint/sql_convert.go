package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// SQL portability rules. GoatFlow runs on MySQL/MariaDB and PostgreSQL, and
// every statement must go through the conversion layer in
// internal/platform/database (ConvertPlaceholders / ConvertQuery /
// ConvertUpsert). These rules type-check the module, production and test
// code alike, and report:
//
//   - sql-unconverted: SQL handed to a database/sql Exec/Query/QueryRow/
//     Prepare (and *Context) call that does not come from a Convert* call,
//     either directly or through a local variable whose every assignment is
//     a Convert* call.
//   - sql-last-insert-id: sql.Result.LastInsertId(), which PostgreSQL does
//     not support. Use database.GetAdapter().InsertWithReturning.
//   - sql-mysql-only: an SQL literal using MySQL syntax the converter cannot
//     rewrite (ON DUPLICATE KEY UPDATE / REPLACE INTO outside ConvertUpsert,
//     LAST_INSERT_ID, DATE_FORMAT, STR_TO_DATE, GROUP_CONCAT, IFNULL).
//   - sql-postgres-only: an SQL literal using PostgreSQL syntax MySQL
//     rejects or misreads: RETURNING outside InsertWithReturning, ON CONFLICT,
//     :: casts outside ConvertQuery, || (logical OR on MySQL), NULLS FIRST/LAST, INTERVAL '…'
//     strings, full-text functions (to_tsvector, ts_rank, …), string_agg.
//
// The dialect-literal rules skip internal/platform/database and its driver
// packages: the conversion layer and the per-dialect drivers hold dialect SQL
// by design.
//
// A reviewed exception carries a "// sql-converted: <reason>" comment on the
// call's first line or the line above (e.g. the conversion layer itself, a
// helper whose callers pass converted SQL, a branch that only runs on MySQL).
// Schema rule exceptions (a test-only temporary table, SQL that is data
// rather than a statement) use "// sql-schema: <reason>" the same way.
const sqlConvertedDirective = "sql-converted:"

const sqlSchemaDirective = "sql-schema:"

const databasePkgPath = "github.com/goatkit/goatflow/internal/platform/database"

// sqlArgIndex is the position of the SQL string for each database/sql method.
var sqlArgIndex = map[string]int{
	"Exec": 0, "Query": 0, "QueryRow": 0, "Prepare": 0,
	"ExecContext": 1, "QueryContext": 1, "QueryRowContext": 1, "PrepareContext": 1,
}

var (
	reUpsertSyntax   = regexp.MustCompile(`(?i)\bON\s+DUPLICATE\s+KEY\s+UPDATE\b|^\s*REPLACE\s+INTO\b`)
	reMySQLOnlySQL   = regexp.MustCompile(`(?i)\b(LAST_INSERT_ID|DATE_FORMAT|STR_TO_DATE|GROUP_CONCAT|IFNULL)\s*\(`)
	reReturningSQL   = regexp.MustCompile(`(?i)\bRETURNING\b`)
	reIntervalBefore = regexp.MustCompile(`(?i)\bINTERVAL\s*$`)
	rePGCast         = regexp.MustCompile(`::\s*[A-Za-z]`)
	rePGOnlySQL      = regexp.MustCompile(`(?i)\bON\s+CONFLICT\b|\bNULLS\s+(FIRST|LAST)\b|\|\||\bINTERVAL\s+'|\b(TO_TSVECTOR|TO_TSQUERY|PLAINTO_TSQUERY|TS_RANK|STRING_AGG)\s*\(`)
	sqlRuleKindNotes = map[string]string{
		"sql-unconverted":    "SQL reaches the database without database.ConvertPlaceholders/ConvertQuery/ConvertUpsert",
		"sql-last-insert-id": "LastInsertId does not work on PostgreSQL; use database.GetAdapter().InsertWithReturning",
		"sql-mysql-only":     "MySQL-only SQL; use database.ConvertUpsert for upserts, portable SQL otherwise",
		"sql-postgres-only":  "PostgreSQL-only SQL; use InsertWithReturning for RETURNING, ConvertUpsert for upserts, ConvertQuery for :: casts, portable SQL otherwise",
		"sql-unknown-table":  "no migration creates this table on both MySQL and PostgreSQL",
		"sql-unknown-column": "this column is not in the table on both MySQL and PostgreSQL migrations",
	}
)

// sqlLintBuildTags are the opt-in build tags used in the module. Packages
// are type-checked once without tags and once with all of them, so files
// behind //go:build integration (and the rest) are linted too.
const sqlLintBuildTags = "integration,e2e"

// scanSQLConversion loads the given package patterns (relative to root) with
// type information and applies the SQL portability rules. A nil schema skips
// the schema rules.
func scanSQLConversion(root string, schema sqlSchema, patterns ...string) ([]violation, error) {
	var pkgs []*packages.Package
	for _, flags := range [][]string{nil, {"-tags=" + sqlLintBuildTags}} {
		cfg := &packages.Config{
			Dir:        root,
			Mode:       packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
			Tests:      true,
			BuildFlags: flags,
		}
		loaded, err := packages.Load(cfg, patterns...)
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, loaded...)
	}
	seen := make(map[string]bool)
	var violations []violation
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			directives, schemaDirectives := directiveLines(pkg.Fset, file)
			report := func(pos token.Pos, kind, detail string) {
				p := pkg.Fset.Position(pos)
				skip := directives
				if kind == "sql-unknown-table" || kind == "sql-unknown-column" {
					skip = schemaDirectives
				}
				if skip[p.Line] || skip[p.Line-1] {
					return
				}
				rel, relErr := filepath.Rel(root, p.Filename)
				if relErr != nil {
					rel = p.Filename
				}
				key := kind + rel + strconv.Itoa(p.Line) + detail
				if seen[key] {
					return
				}
				seen[key] = true
				violations = append(violations, violation{Kind: kind, File: filepath.ToSlash(rel), Line: p.Line, Detail: detail})
			}
			dialectLayer := pkg.PkgPath == databasePkgPath || strings.HasPrefix(pkg.PkgPath, databasePkgPath+"/")
			for _, decl := range file.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
					checkSQLFunc(pkg.TypesInfo, fd.Body, dialectLayer, schema, report)
				}
			}
		}
	}
	return violations, nil
}

// directiveLines returns the lines carrying a sql-converted and a sql-schema
// directive.
func directiveLines(fset *token.FileSet, file *ast.File) (converted, schema map[int]bool) {
	converted, schema = make(map[int]bool), make(map[int]bool)
	for _, group := range file.Comments {
		for _, c := range group.List {
			line := fset.Position(c.Pos()).Line
			if strings.Contains(c.Text, sqlConvertedDirective) {
				converted[line] = true
			}
			if strings.Contains(c.Text, sqlSchemaDirective) {
				schema[line] = true
			}
		}
	}
	return converted, schema
}

// convertCallName returns the database.Convert* function a call expression
// invokes, or "".
func convertCallName(info *types.Info, e ast.Expr) string {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return ""
	}
	var obj types.Object
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		obj = info.Uses[fn.Sel]
	case *ast.Ident:
		obj = info.Uses[fn]
	}
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != databasePkgPath || !strings.HasPrefix(obj.Name(), "Convert") {
		return ""
	}
	return obj.Name()
}

func checkSQLFunc(info *types.Info, body *ast.BlockStmt, dialectLayer bool, schema sqlSchema, report func(token.Pos, string, string)) {
	// Every value assigned to each local variable in this function.
	assigns := make(map[types.Object][]ast.Expr)
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if s.Tok == token.ADD_ASSIGN || len(s.Lhs) != len(s.Rhs) {
				return true
			}
			for i, lhs := range s.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				obj := info.Defs[id]
				if obj == nil {
					obj = info.Uses[id]
				}
				if obj != nil {
					assigns[obj] = append(assigns[obj], s.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if i < len(s.Values) {
					if obj := info.Defs[id]; obj != nil {
						assigns[obj] = append(assigns[obj], s.Values[i])
					}
				}
			}
		}
		return true
	})

	converted := func(e ast.Expr) bool {
		if convertCallName(info, e) != "" {
			return true
		}
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok {
			return false
		}
		values := assigns[info.Uses[id]]
		if len(values) == 0 {
			return false
		}
		for _, v := range values {
			if convertCallName(info, v) == "" {
				return false
			}
		}
		return true
	}

	// sqlSources returns the string literals an expression's SQL is built
	// from, following Convert* calls, fmt.Sprintf, + concatenation and local
	// variables.
	var sqlSources func(e ast.Expr, depth int, out map[ast.Expr]bool)
	sqlSources = func(e ast.Expr, depth int, out map[ast.Expr]bool) {
		if depth > 4 {
			return
		}
		e = ast.Unparen(e)
		switch x := e.(type) {
		case *ast.BasicLit:
			out[x] = true
		case *ast.CallExpr:
			if (convertCallName(info, x) != "" || isSprintf(info, x)) && len(x.Args) > 0 {
				sqlSources(x.Args[0], depth+1, out)
			}
		case *ast.BinaryExpr:
			if x.Op == token.ADD {
				sqlSources(x.X, depth+1, out)
				sqlSources(x.Y, depth+1, out)
			}
		case *ast.Ident:
			for _, v := range assigns[info.Uses[x]] {
				sqlSources(v, depth+1, out)
			}
		}
	}
	upsertLits := make(map[ast.Expr]bool)    // reach ConvertUpsert
	castLits := make(map[ast.Expr]bool)      // reach ConvertQuery
	returningLits := make(map[ast.Expr]bool) // reach InsertWithReturning(Tx)
	dbLits := make(map[ast.Expr]bool)        // reach a Convert* or database/sql call
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch convertCallName(info, call) {
		case "ConvertUpsert":
			sqlSources(call, 0, upsertLits)
		case "ConvertQuery":
			sqlSources(call, 0, castLits)
		}
		if convertCallName(info, call) != "" {
			sqlSources(call, 0, dbLits)
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if selection := info.Selections[sel]; selection != nil && selection.Obj().Pkg() != nil && selection.Obj().Pkg().Path() == "database/sql" {
				if idx, ok := sqlArgIndex[sel.Sel.Name]; ok && len(call.Args) > idx {
					sqlSources(call.Args[idx], 0, dbLits)
				}
			}
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) > 1 &&
			(sel.Sel.Name == "InsertWithReturning" || sel.Sel.Name == "InsertWithReturningTx") {
			if obj := info.Uses[sel.Sel]; obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == databasePkgPath {
				sqlSources(call.Args[1], 0, returningLits)
			}
		}
		return true
	})

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			if node.Kind != token.STRING {
				return true
			}
			raw, err := strconv.Unquote(node.Value)
			if dialectLayer || err != nil {
				return true
			}
			// Schema rules run on every literal that reaches the database,
			// fragments included; checkSQLSchema skips non-statements itself.
			if schema != nil && dbLits[node] {
				for _, f := range checkSQLSchema(schema, raw) {
					report(node.Pos(), f.kind, f.detail)
				}
			}
			if !database.IsSQLQuery(raw) {
				return true
			}
			code := withoutStringLiterals(raw)
			switch {
			case reUpsertSyntax.MatchString(code) && !upsertLits[node], reMySQLOnlySQL.MatchString(code):
				report(node.Pos(), "sql-mysql-only", firstSQLLine(raw))
			case reReturningSQL.MatchString(code) && !returningLits[node],
				rePGCast.MatchString(code) && !castLits[node],
				rePGOnlySQL.MatchString(code):
				report(node.Pos(), "sql-postgres-only", firstSQLLine(raw))
			}
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			selection := info.Selections[sel]
			if selection == nil || selection.Obj().Pkg() == nil || selection.Obj().Pkg().Path() != "database/sql" {
				return true
			}
			if sel.Sel.Name == "LastInsertId" {
				report(node.Pos(), "sql-last-insert-id", "")
				return true
			}
			idx, ok := sqlArgIndex[sel.Sel.Name]
			if !ok || len(node.Args) <= idx {
				return true
			}
			if !converted(node.Args[idx]) {
				report(node.Pos(), "sql-unconverted", describeSQLArg(info, node.Args[idx], assigns))
			}
		}
		return true
	})
}

func describeSQLArg(info *types.Info, arg ast.Expr, assigns map[types.Object][]ast.Expr) string {
	arg = ast.Unparen(arg)
	if lit, ok := arg.(*ast.BasicLit); ok {
		if raw, err := strconv.Unquote(lit.Value); err == nil {
			return firstSQLLine(raw)
		}
	}
	if id, ok := arg.(*ast.Ident); ok {
		for _, v := range assigns[info.Uses[id]] {
			if lit, ok := v.(*ast.BasicLit); ok {
				if raw, err := strconv.Unquote(lit.Value); err == nil {
					return id.Name + " = " + firstSQLLine(raw)
				}
			}
		}
		return "query from " + id.Name
	}
	return ""
}

func firstSQLLine(sql string) string {
	s := strings.Join(strings.Fields(sql), " ")
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	return s
}

// isSprintf reports whether call is fmt.Sprintf, whose format string is the
// SQL text when a query is built from validated identifiers.
func isSprintf(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" {
		return false
	}
	obj := info.Uses[sel.Sel]
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "fmt"
}

// withoutStringLiterals blanks the contents of single-quoted SQL literals so
// values such as 'Admin::Plugins' or 'a || b' are not read as syntax.
// INTERVAL '…' is kept, as the quoted interval is itself the dialect issue.
func withoutStringLiterals(sql string) string {
	b := []byte(sql)
	inString := false
	for i, c := range b {
		if c == '\'' {
			if !inString && reIntervalBefore.Match(b[:i]) {
				continue
			}
			inString = !inString
			continue
		}
		if inString {
			b[i] = ' '
		}
	}
	return string(b)
}
