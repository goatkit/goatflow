package database

import (
	"os"
	"testing"
)

func withDriver(t *testing.T, driver string, fn func()) {
	t.Helper()
	old := os.Getenv("DB_DRIVER")
	os.Setenv("DB_DRIVER", driver)
	defer os.Setenv("DB_DRIVER", old)
	// Clear TEST_DB_DRIVER so it doesn't interfere
	oldTest := os.Getenv("TEST_DB_DRIVER")
	os.Setenv("TEST_DB_DRIVER", "")
	defer os.Setenv("TEST_DB_DRIVER", oldTest)
	fn()
}

func TestRewriteDateSubForPostgreSQL(t *testing.T) {
	withDriver(t, "postgres", func() {
		tests := []struct {
			input    string
			expected string
		}{
			{
				"WHERE t.create_time >= DATE_SUB(NOW(), INTERVAL 30 DAY)",
				"WHERE t.create_time >= (NOW() - INTERVAL '30 day')",
			},
			{
				"WHERE x >= DATE_SUB(NOW(), INTERVAL 7 HOUR)",
				"WHERE x >= (NOW() - INTERVAL '7 hour')",
			},
			{
				"SELECT * FROM t WHERE id = $1",
				"SELECT * FROM t WHERE id = $1",
			},
		}
		for _, tc := range tests {
			result := rewriteForPostgreSQL(tc.input)
			if result != tc.expected {
				t.Errorf("rewriteForPostgreSQL(%q)\n  got:  %q\n  want: %q", tc.input, result, tc.expected)
			}
		}
	})
}

func TestRewriteDateAddForPostgreSQL(t *testing.T) {
	withDriver(t, "postgres", func() {
		input := "DATE_ADD(NOW(), INTERVAL 1 MINUTE)"
		expected := "(NOW() + INTERVAL '1 minute')"
		result := rewriteForPostgreSQL(input)
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})
}

func TestRewriteUnixTimestampForPostgreSQL(t *testing.T) {
	withDriver(t, "postgres", func() {
		tests := []struct {
			input    string
			expected string
		}{
			{
				"WHERE t.escalation_time < UNIX_TIMESTAMP()",
				"WHERE t.escalation_time < EXTRACT(EPOCH FROM NOW())::bigint",
			},
			{
				"UNIX_TIMESTAMP(t.change_time)",
				"EXTRACT(EPOCH FROM t.change_time)::bigint",
			},
		}
		for _, tc := range tests {
			result := rewriteForPostgreSQL(tc.input)
			if result != tc.expected {
				t.Errorf("rewriteForPostgreSQL(%q)\n  got:  %q\n  want: %q", tc.input, result, tc.expected)
			}
		}
	})
}

func TestRewriteCurdateForPostgreSQL(t *testing.T) {
	withDriver(t, "postgres", func() {
		input := "WHERE DATE(t.create_time) = CURDATE()"
		expected := "WHERE DATE(t.create_time) = CURRENT_DATE"
		result := rewriteForPostgreSQL(input)
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})
}

func TestRewriteExtractEpochForMySQL(t *testing.T) {
	withDriver(t, "mysql", func() {
		input := "WHERE t.escalation_time < EXTRACT(EPOCH FROM NOW())::bigint"
		expected := "WHERE t.escalation_time < UNIX_TIMESTAMP(NOW())"
		result := rewriteForMySQL(input)
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})
}

func TestConvertPlaceholdersWithRewriting(t *testing.T) {
	t.Run("MySQL passthrough", func(t *testing.T) {
		withDriver(t, "mysql", func() {
			q := ConvertPlaceholders("SELECT * FROM t WHERE x >= DATE_SUB(NOW(), INTERVAL 7 DAY) AND id = ?")
			if q != "SELECT * FROM t WHERE x >= DATE_SUB(NOW(), INTERVAL 7 DAY) AND id = ?" {
				t.Errorf("MySQL should pass through DATE_SUB, got: %s", q)
			}
		})
	})

	t.Run("PostgreSQL rewrites DATE_SUB and placeholders", func(t *testing.T) {
		withDriver(t, "postgres", func() {
			q := ConvertPlaceholders("SELECT * FROM t WHERE x >= DATE_SUB(NOW(), INTERVAL 7 DAY) AND id = ?")
			expected := "SELECT * FROM t WHERE x >= (NOW() - INTERVAL '7 day') AND id = $1"
			if q != expected {
				t.Errorf("got:  %q\nwant: %q", q, expected)
			}
		})
	})

	t.Run("PostgreSQL rewrites UNIX_TIMESTAMP", func(t *testing.T) {
		withDriver(t, "postgres", func() {
			q := ConvertPlaceholders("WHERE t.escalation_time < UNIX_TIMESTAMP() AND id = ?")
			expected := "WHERE t.escalation_time < EXTRACT(EPOCH FROM NOW())::bigint AND id = $1"
			if q != expected {
				t.Errorf("got:  %q\nwant: %q", q, expected)
			}
		})
	})

	t.Run("PostgreSQL rewrites CURDATE", func(t *testing.T) {
		withDriver(t, "postgres", func() {
			q := ConvertPlaceholders("WHERE DATE(t.create_time) = CURDATE() AND id = ?")
			expected := "WHERE DATE(t.create_time) = CURRENT_DATE AND id = $1"
			if q != expected {
				t.Errorf("got:  %q\nwant: %q", q, expected)
			}
		})
	})
}

func TestNoRewriteWhenNotNeeded(t *testing.T) {
	withDriver(t, "mysql", func() {
		q := "SELECT * FROM users WHERE id = ?"
		result := ConvertPlaceholders(q)
		if result != q {
			t.Errorf("simple query should not be modified for MySQL, got: %s", result)
		}
	})
}
