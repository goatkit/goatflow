package organisation

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/sqllex"
)

// OrgAwareTables is the set of tables the plugin sandbox scopes to the
// caller's active organisation. It lists only tables whose rows each belong to
// exactly one organisation (org_id NOT NULL in both migration sets). Core
// tables such as ticket, queue and customer_user have no org_id column, so
// they are not scoped; tables with a nullable org_id (NULL = global row:
// gk_secure_config, gk_identity_provider, gk_recycle_bin, gk_deletion_log)
// are not scoped either, because "org_id = ?" would hide the global rows.
// Tables not in this set pass through unmodified.
var OrgAwareTables = map[string]bool{
	"gk_identity_provider_org": true,
	"gk_org_plugin_access":     true,
	"gk_user_organisation":     true,
	"sysconfig_org":            true,
}

// ScopeQuery rewrites a plugin SQL statement so that every org-aware table
// it touches is restricted to orgID. It returns the rewritten query and the
// args with the org id inserted at the matching placeholder positions.
//
// SELECT/UPDATE/DELETE: every reference to an org-aware table (main table,
// comma list, JOIN, subquery, CTE body, UNION branch) gets
// "<table or alias>.org_id = ?" in that query block's WHERE; the plugin's
// own predicate is wrapped in parentheses so "... OR 1=1" cannot escape the
// filter. A WHERE is added when the block has none. UPDATE may not assign
// org_id.
//
// INSERT/REPLACE into an org-aware table is not rewritten; it is accepted
// only when the column list names org_id and every VALUES row supplies
// orgID for it (as a ? argument or a literal), and nothing after the rows
// mentions org_id.
//
// Any other shape that mentions an org-aware table (DDL, TABLE t, a
// parenthesised table reference, a $N placeholder, more than one statement,
// an INSERT ... SELECT) is refused. Statements that mention no org-aware
// table, and every statement when orgID is 0 (single-org mode), pass through
// unmodified.
func ScopeQuery(query string, args []any, orgID int64) (string, []any, error) {
	if orgID == 0 {
		return query, args, nil
	}
	toks := sqllex.Tokenize(query)
	mentions := false
	for _, t := range toks {
		if t.IsIdent() && isOrgAware(t.Name) {
			mentions = true
			break
		}
	}
	if !mentions {
		return query, args, nil
	}

	s := &scoper{q: query, toks: toks, args: args, orgID: orgID, match: make([]int, len(toks))}
	var open []int
	placeholders := 0
	for i, t := range toks {
		s.match[i] = -1
		switch {
		case t.Kind == 'p' && t.Text == "(":
			open = append(open, i)
		case t.Kind == 'p' && t.Text == ")":
			if len(open) == 0 {
				return "", nil, errors.New("org scope: unbalanced parentheses")
			}
			s.match[open[len(open)-1]] = i
			open = open[:len(open)-1]
		case t.Kind == 'p' && t.Text == ";" && i < len(toks)-1:
			return "", nil, errors.New("org scope: only one statement per call")
		case t.Kind == 'p' && t.Text == "?":
			placeholders++
		case t.Kind == 'w' && len(t.Text) > 1 && t.Text[0] == '$':
			return "", nil, errors.New("org scope: use ? placeholders, not $N")
		}
	}
	if len(open) > 0 {
		return "", nil, errors.New("org scope: unbalanced parentheses")
	}
	if placeholders != len(args) {
		return "", nil, fmt.Errorf("org scope: %d placeholders but %d args", placeholders, len(args))
	}

	// Optional "@dbname:" prefix of the HostAPI DB methods.
	first := 0
	if len(toks) > 3 && toks[0].Kind == 'p' && toks[0].Text == "@" && toks[1].Kind == 'w' && toks[2].Kind == 'p' && toks[2].Text == ":" {
		first = 3
	}
	if first >= len(toks) || toks[first].Kind != 'w' {
		return "", nil, errors.New("org scope: statement cannot be org-scoped")
	}
	switch toks[first].Text {
	case "INSERT", "REPLACE":
		if err := s.checkInsert(first); err != nil {
			return "", nil, err
		}
		return query, args, nil
	case "SELECT", "WITH", "UPDATE", "DELETE":
		if err := s.block(first, len(toks)); err != nil {
			return "", nil, err
		}
		return s.apply()
	}
	return "", nil, fmt.Errorf("org scope: %s statements on org-aware tables are not permitted", toks[first].Text)
}

// isOrgAware reports whether a (possibly schema-qualified) name is an
// org-aware table.
func isOrgAware(name string) bool {
	return OrgAwareTables[name] || OrgAwareTables[name[strings.LastIndexByte(name, '.')+1:]]
}

// isOrgIDColumn reports whether an identifier is the org_id column, bare or
// qualified.
func isOrgIDColumn(name string) bool {
	return name == "org_id" || strings.HasSuffix(name, ".org_id")
}

type scoper struct {
	q     string
	toks  []sqllex.Token
	args  []any
	orgID int64
	match []int // '(' index -> matching ')' index, else -1

	inserts []insertion
}

// insertion is text added to the query at byte offset at; n is the number of
// "?" placeholders in text, each bound to the org id.
type insertion struct {
	at   int
	text string
	n    int
}

func (s *scoper) word(i int, w string) bool {
	return i >= 0 && i < len(s.toks) && s.toks[i].Kind == 'w' && s.toks[i].Text == w
}

func (s *scoper) punct(i int, p string) bool {
	return i >= 0 && i < len(s.toks) && s.toks[i].Kind == 'p' && s.toks[i].Text == p
}

func (s *scoper) ident(i int) bool { return i >= 0 && i < len(s.toks) && s.toks[i].IsIdent() }

func (s *scoper) raw(i int) string { return s.q[s.toks[i].Start:s.toks[i].End] }

// block scopes the tokens in [a, b) at one parenthesis level: nested
// parentheses are scoped recursively, the level itself is split into query
// segments at set operators.
func (s *scoper) block(a, b int) error {
	seg := a
	for i := a; i < b; i++ {
		t := s.toks[i]
		if s.punct(i, "(") {
			if err := s.block(i+1, s.match[i]); err != nil {
				return err
			}
			i = s.match[i]
			continue
		}
		if t.Kind == 'w' && (t.Text == "UNION" || t.Text == "EXCEPT" || t.Text == "INTERSECT") || s.punct(i, ";") {
			if err := s.segment(seg, i); err != nil {
				return err
			}
			seg = i + 1
			for s.word(seg, "ALL") || s.word(seg, "DISTINCT") {
				seg++
			}
		}
	}
	return s.segment(seg, b)
}

// predicateStop lists the words that end a WHERE predicate at its own level
// and before which a missing WHERE is inserted.
var predicateStop = map[string]bool{
	"GROUP": true, "HAVING": true, "ORDER": true, "LIMIT": true, "OFFSET": true,
	"FETCH": true, "FOR": true, "WINDOW": true, "RETURNING": true, "INTO": true,
	"LOCK": true, "PROCEDURE": true,
}

// segment scopes one query (a SELECT, UPDATE or DELETE, or one branch of a
// set operation) whose tokens at this level are [a, b).
func (s *scoper) segment(a, b int) error {
	if a >= b {
		return nil
	}
	// next returns the index after i at this level.
	next := func(i int) int {
		if s.punct(i, "(") {
			return s.match[i] + 1
		}
		return i + 1
	}

	var preds []string
	firstRef := -1
	for i := a; i < b; i = next(i) {
		t := s.toks[i]
		if !t.IsIdent() || !isOrgAware(t.Name) {
			continue
		}
		if !s.isTableRef(i, a) {
			return fmt.Errorf("org scope: table %s is referenced in a position that cannot be org-scoped", t.Name)
		}
		if firstRef < 0 {
			firstRef = i
		}
		qual := s.raw(i)
		if s.word(i+1, "AS") && s.ident(i+2) {
			qual = s.raw(i + 2)
		} else if s.ident(i+1) && (s.toks[i+1].Kind == 'q' || !sqllex.AliasStop[s.toks[i+1].Text]) {
			qual = s.raw(i + 1)
		}
		preds = append(preds, qual+".org_id = ?")
	}
	if len(preds) == 0 {
		return nil
	}
	kind := s.toks[a].Text
	if s.toks[a].Kind != 'w' || (kind != "SELECT" && kind != "WITH" && kind != "UPDATE" && kind != "DELETE") {
		return fmt.Errorf("org scope: %s cannot be org-scoped", s.raw(a))
	}

	// Locate WHERE and the first clause after the predicate, at this level,
	// after the table references.
	where, stop := -1, b
	for i := firstRef; i < b; i = next(i) {
		if s.word(i, "WHERE") && where < 0 {
			where = i
			continue
		}
		if s.toks[i].Kind == 'w' && predicateStop[s.toks[i].Text] {
			stop = i
			break
		}
	}
	if kind == "UPDATE" {
		// Assigning org_id would move rows to another organisation.
		end := where
		if end < 0 {
			end = stop
		}
		inSet := false
		for i := a; i < end; i++ {
			if s.word(i, "SET") && !inSet {
				inSet = true
				continue
			}
			if inSet && s.toks[i].IsIdent() && isOrgIDColumn(s.toks[i].Name) {
				return errors.New("org scope: UPDATE may not assign org_id")
			}
		}
	}

	filter := strings.Join(preds, " AND ")
	if where >= 0 {
		if where+1 >= stop {
			return errors.New("org scope: empty WHERE clause")
		}
		s.inserts = append(s.inserts,
			insertion{at: s.toks[where].End, text: " " + filter + " AND", n: len(preds)},
			insertion{at: s.toks[where+1].Start, text: "("},
			insertion{at: s.toks[stop-1].End, text: ")"})
		return nil
	}
	if stop < b {
		s.inserts = append(s.inserts, insertion{at: s.toks[stop].Start, text: "WHERE " + filter + " ", n: len(preds)})
	} else {
		s.inserts = append(s.inserts, insertion{at: s.toks[b-1].End, text: " WHERE " + filter, n: len(preds)})
	}
	return nil
}

// isTableRef reports whether the org-aware identifier at i stands where a
// table reference is expected: after FROM, JOIN, UPDATE, USING or ONLY, or
// after a comma in a FROM/UPDATE table list. a is the start of the segment.
func (s *scoper) isTableRef(i, a int) bool {
	for _, w := range []string{"FROM", "JOIN", "UPDATE", "USING", "ONLY"} {
		if s.word(i-1, w) {
			return true
		}
	}
	if !s.punct(i-1, ",") {
		return false
	}
	// Walk back at this level to the clause keyword that owns the list.
	for j := i - 2; j >= a; j-- {
		if s.punct(j, ")") {
			for k := j - 1; k >= a; k-- {
				if s.match[k] == j {
					j = k
					break
				}
			}
			continue
		}
		if s.toks[j].Kind != 'w' {
			continue
		}
		switch s.toks[j].Text {
		case "FROM", "UPDATE":
			return true
		case "SELECT", "SET", "WHERE", "JOIN", "ON", "GROUP", "ORDER", "HAVING",
			"VALUES", "INTO", "USING", "LIMIT", "WITH", "RETURNING", "DELETE":
			return false
		}
	}
	return false
}

// checkInsert validates INSERT/REPLACE INTO an org-aware table: org_id must
// be a listed column and each VALUES row must set it to the caller's org.
func (s *scoper) checkInsert(first int) error {
	i := first + 1
	for i < len(s.toks) && !s.word(i, "INTO") {
		i++
	}
	i++
	if !s.ident(i) {
		return errors.New("org scope: INSERT target not found")
	}
	table := s.toks[i].Name
	if !isOrgAware(table) {
		return errors.New("org scope: INSERT may not read org-aware tables")
	}
	// No second mention of an org-aware table anywhere (subqueries, ON
	// CONFLICT ... SELECT, ...).
	for j, t := range s.toks {
		if j != i && t.IsIdent() && isOrgAware(t.Name) {
			return fmt.Errorf("org scope: INSERT may only name %s once", table)
		}
	}
	i++
	if s.word(i, "AS") && s.ident(i+1) {
		i += 2
	} else if s.ident(i) && !sqllex.AliasStop[s.toks[i].Text] {
		i++
	}
	if !s.punct(i, "(") {
		return fmt.Errorf("org scope: INSERT INTO %s must list its columns, including org_id", table)
	}
	colEnd := s.match[i]
	orgCol := -1
	for j, n := i+1, 0; j < colEnd; j++ {
		if s.punct(j, ",") {
			n++
			continue
		}
		if s.toks[j].IsIdent() && isOrgIDColumn(s.toks[j].Name) {
			orgCol = n
		}
	}
	if orgCol < 0 {
		return fmt.Errorf("org scope: INSERT INTO %s must set org_id", table)
	}
	i = colEnd + 1
	if !s.word(i, "VALUES") {
		return fmt.Errorf("org scope: INSERT INTO %s must use VALUES", table)
	}
	i++
	ordinal := 0 // "?" placeholders before the current token
	for j := range i {
		if s.punct(j, "?") {
			ordinal++
		}
	}
	for {
		if !s.punct(i, "(") {
			return fmt.Errorf("org scope: INSERT INTO %s: malformed VALUES", table)
		}
		rowEnd := s.match[i]
		col, valStart := 0, i+1
		found := false
		for j := i + 1; j <= rowEnd; j++ {
			if s.punct(j, "(") {
				j = s.match[j]
				continue
			}
			if j == rowEnd || s.punct(j, ",") {
				if col == orgCol {
					n := ordinal
					for k := i + 1; k < valStart; k++ {
						if s.punct(k, "?") {
							n++
						}
					}
					if err := s.checkOrgValue(valStart, j, n); err != nil {
						return fmt.Errorf("org scope: INSERT INTO %s: %w", table, err)
					}
					found = true
					break
				}
				col++
				valStart = j + 1
			}
		}
		if !found {
			return fmt.Errorf("org scope: INSERT INTO %s must set org_id", table)
		}
		for j := i; j <= rowEnd; j++ {
			if s.punct(j, "?") {
				ordinal++
			}
		}
		i = rowEnd + 1
		if !s.punct(i, ",") {
			break
		}
		i++
	}
	for ; i < len(s.toks); i++ {
		if s.toks[i].IsIdent() && isOrgIDColumn(s.toks[i].Name) {
			return fmt.Errorf("org scope: INSERT INTO %s may not touch org_id after VALUES", table)
		}
	}
	return nil
}

// checkOrgValue checks that the single value token in [from, to) is the
// caller's org id: a "?" bound to args[ordinal] or a numeric literal.
func (s *scoper) checkOrgValue(from, to, ordinal int) error {
	if to-from != 1 {
		return errors.New("org_id must be a ? placeholder or a literal")
	}
	t := s.toks[from]
	if s.punct(from, "?") {
		if ordinal >= len(s.args) {
			return errors.New("org_id argument missing")
		}
		if !isOrgID(s.args[ordinal], s.orgID) {
			return errors.New("org_id must be the caller's organisation")
		}
		return nil
	}
	if t.Kind == 'w' {
		if v, err := strconv.ParseInt(t.Text, 10, 64); err == nil && v == s.orgID {
			return nil
		}
	}
	return errors.New("org_id must be the caller's organisation")
}

// isOrgID reports whether a bound argument equals orgID.
func isOrgID(v any, orgID int64) bool {
	switch x := v.(type) {
	case int:
		return int64(x) == orgID
	case int8:
		return int64(x) == orgID
	case int16:
		return int64(x) == orgID
	case int32:
		return int64(x) == orgID
	case int64:
		return x == orgID
	case uint:
		return uint64(x) <= math.MaxInt64 && int64(x) == orgID // #nosec G115 -- bounded by the MaxInt64 check
	case uint8:
		return int64(x) == orgID
	case uint16:
		return int64(x) == orgID
	case uint32:
		return int64(x) == orgID
	case uint64:
		return x <= math.MaxInt64 && int64(x) == orgID // #nosec G115 -- bounded by the MaxInt64 check
	case float32:
		return float64(x) == float64(orgID)
	case float64:
		return x == float64(orgID)
	case json.Number:
		n, err := x.Int64()
		return err == nil && n == orgID
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		return err == nil && n == orgID
	}
	return false
}

// apply builds the rewritten query and the args with the org id spliced in
// at each inserted placeholder's position.
func (s *scoper) apply() (string, []any, error) {
	sort.SliceStable(s.inserts, func(i, j int) bool { return s.inserts[i].at < s.inserts[j].at })
	var b strings.Builder
	b.Grow(len(s.q) + 48*len(s.inserts))
	args := make([]any, 0, len(s.args)+len(s.inserts))
	pos, tok, ordinal := 0, 0, 0
	for _, ins := range s.inserts {
		b.WriteString(s.q[pos:ins.at])
		b.WriteString(ins.text)
		pos = ins.at
		for ; tok < len(s.toks) && s.toks[tok].Start < ins.at; tok++ {
			if s.punct(tok, "?") {
				args = append(args, s.args[ordinal])
				ordinal++
			}
		}
		for range ins.n {
			args = append(args, s.orgID)
		}
	}
	b.WriteString(s.q[pos:])
	args = append(args, s.args[ordinal:]...)
	return b.String(), args, nil
}
