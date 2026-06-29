package database

import (
	"testing"
)

func TestRemapArgsForRepeatedPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		args     []interface{}
		expected []interface{}
	}{
		{
			name:     "no placeholders",
			query:    "SELECT * FROM users",
			args:     []interface{}{},
			expected: []interface{}{},
		},
		{
			name:     "simple sequential placeholders",
			query:    "INSERT INTO t (a, b, c) VALUES ($1, $2, $3)",
			args:     []interface{}{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "repeated placeholder at end",
			query:    "INSERT INTO t (a, b, c, d) VALUES ($1, $2, $3, $3)",
			args:     []interface{}{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c", "c"},
		},
		{
			name:     "repeated placeholder create_by/change_by pattern",
			query:    "INSERT INTO t (name, create_by, change_by) VALUES ($1, $2, $2)",
			args:     []interface{}{"test", 42},
			expected: []interface{}{"test", 42, 42},
		},
		{
			name:     "multiple repeated placeholders",
			query:    "INSERT INTO t (a, b, c, d, e) VALUES ($1, $2, $1, $3, $2)",
			args:     []interface{}{"x", "y", "z"},
			expected: []interface{}{"x", "y", "x", "z", "y"},
		},
		{
			name:     "typical audit fields pattern",
			query:    "INSERT INTO webhooks (name, url, create_time, create_by, change_time, change_by) VALUES ($1, $2, NOW(), $3, NOW(), $3) RETURNING id",
			args:     []interface{}{"webhook1", "http://example.com", 1},
			expected: []interface{}{"webhook1", "http://example.com", 1, 1},
		},
		{
			name:     "placeholder index out of range",
			query:    "INSERT INTO t (a) VALUES ($5)",
			args:     []interface{}{"a", "b"},
			expected: []interface{}{"a", "b"}, // Falls back to original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remapArgsForRepeatedPlaceholders(tt.query, tt.args)
			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("arg[%d]: got %v, want %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestPrepareQueryForMySQL(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		args         []interface{}
		expectedSQL  string
		expectedArgs []interface{}
	}{
		{
			name:         "already converted query with ?",
			query:        "INSERT INTO t (a, b) VALUES (?, ?) RETURNING id",
			args:         []interface{}{"x", "y"},
			expectedSQL:  "INSERT INTO t (a, b) VALUES (?, ?)",
			expectedArgs: []interface{}{"x", "y"},
		},
		{
			name:         "PostgreSQL style simple",
			query:        "INSERT INTO t (a, b) VALUES ($1, $2) RETURNING id",
			args:         []interface{}{"x", "y"},
			expectedSQL:  "INSERT INTO t (a, b) VALUES (?, ?)",
			expectedArgs: []interface{}{"x", "y"},
		},
		{
			name:         "PostgreSQL style with repeats",
			query:        "INSERT INTO t (a, b, c) VALUES ($1, $2, $2) RETURNING id",
			args:         []interface{}{"name", 42},
			expectedSQL:  "INSERT INTO t (a, b, c) VALUES (?, ?, ?)",
			expectedArgs: []interface{}{"name", 42, 42},
		},
		{
			name:         "no RETURNING clause",
			query:        "INSERT INTO t (a) VALUES ($1)",
			args:         []interface{}{"test"},
			expectedSQL:  "INSERT INTO t (a) VALUES (?)",
			expectedArgs: []interface{}{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultSQL, resultArgs := prepareQueryForMySQL(tt.query, tt.args)
			if resultSQL != tt.expectedSQL {
				t.Errorf("SQL mismatch:\ngot:  %q\nwant: %q", resultSQL, tt.expectedSQL)
			}
			if len(resultArgs) != len(tt.expectedArgs) {
				t.Errorf("args length mismatch: got %d, want %d", len(resultArgs), len(tt.expectedArgs))
				return
			}
			for i := range resultArgs {
				if resultArgs[i] != tt.expectedArgs[i] {
					t.Errorf("arg[%d]: got %v, want %v", i, resultArgs[i], tt.expectedArgs[i])
				}
			}
		})
	}
}

func TestRemoveReturningClause(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "with RETURNING id",
			query:    "INSERT INTO t (a) VALUES (?) RETURNING id",
			expected: "INSERT INTO t (a) VALUES (?)",
		},
		{
			name:     "with RETURNING multiple columns",
			query:    "INSERT INTO t (a) VALUES (?) RETURNING id, name",
			expected: "INSERT INTO t (a) VALUES (?)",
		},
		{
			name:     "lowercase returning",
			query:    "INSERT INTO t (a) VALUES (?) returning id",
			expected: "INSERT INTO t (a) VALUES (?)",
		},
		{
			name:     "no RETURNING",
			query:    "INSERT INTO t (a) VALUES (?)",
			expected: "INSERT INTO t (a) VALUES (?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeReturningClause(tt.query)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}
