// Package sqllex is the SQL lexer shared by the plugin sandbox's table scope
// check and the organisation query scoper. It splits a statement into
// words, quoted identifiers, string literals and punctuation; it is not a
// parser.
package sqllex

import "strings"

// Token is one lexical token of a SQL statement.
type Token struct {
	Kind  byte   // 'w' word, 'q' quoted identifier, 's' string literal, 'p' punctuation
	Text  string // word: upper case; punctuation: the character
	Name  string // identifier, lower case, schema-qualified as "schema.table"
	Start int    // byte offset of the token in the source
	End   int    // byte offset one past the token
}

// IsIdent reports whether the token can name a table or column.
func (t Token) IsIdent() bool { return t.Kind == 'w' || t.Kind == 'q' }

// AliasStop lists the words that end a table reference, so they are never
// taken for an alias.
var AliasStop = map[string]bool{
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

// Tokenize splits a statement into words, quoted identifiers, string
// literals and punctuation. Comments are dropped. Dotted names are joined
// ("otrs.users"); a trailing ".*" is ignored.
func Tokenize(q string) []Token {
	var toks []Token
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
		start := i
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
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
			toks = append(toks, Token{Kind: 's', Start: start, End: i})
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
			tok := Token{Kind: kind, Name: strings.ToLower(name), Start: start, End: i}
			if kind == 'w' {
				tok.Text = strings.ToUpper(name)
			}
			toks = append(toks, tok)
		default:
			toks = append(toks, Token{Kind: 'p', Text: string(c), Start: start, End: i + 1})
			i++
		}
	}
	return toks
}
