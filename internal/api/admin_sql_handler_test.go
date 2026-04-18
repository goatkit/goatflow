package api

import (
	"testing"
)

func TestIsAllowedStatement(t *testing.T) {
	tests := []struct {
		query   string
		allowed bool
	}{
		// Allowed
		{"SELECT * FROM tickets", true},
		{"select id from tickets", true},
		{"  SELECT id FROM tickets", true},
		{"DESCRIBE tickets", true},
		{"describe tickets", true},
		{"DESC tickets", true},
		{"EXPLAIN SELECT * FROM tickets", true},
		{"explain select * from tickets", true},
		{"SHOW TABLES", true},
		{"show tables", true},
		{"SHOW COLUMNS FROM tickets", true},
		{"show columns from tickets", true},

		// Blocked
		{"INSERT INTO tickets VALUES (1)", false},
		{"UPDATE tickets SET title='x'", false},
		{"DELETE FROM tickets", false},
		{"DROP TABLE tickets", false},
		{"CREATE TABLE foo (id int)", false},
		{"ALTER TABLE tickets ADD COLUMN x int", false},
		{"TRUNCATE tickets", false},
		{"SHOW GRANTS", false},
		{"SHOW PROCESSLIST", false},
		{"SHOW VARIABLES", false},
		{"SET @x = 1", false},
		{"", false},
		{"  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := isAllowedStatement(tt.query)
			if got != tt.allowed {
				t.Errorf("isAllowedStatement(%q) = %v, want %v", tt.query, got, tt.allowed)
			}
		})
	}
}
