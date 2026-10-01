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

func TestConvertPlaceholdersPostgreSQLRewrites(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"parameterised DATE_SUB", "WHERE create_time >= DATE_SUB(NOW(), INTERVAL ? DAY)",
			"WHERE create_time >= (NOW() - ($1 * INTERVAL '1 day'))"},
		{"parameterised DATE_ADD", "SELECT DATE_ADD(NOW(), INTERVAL ? HOUR) WHERE id = ?",
			"SELECT (NOW() + ($1 * INTERVAL '1 hour')) WHERE id = $2"},
		{"INSERT IGNORE", "INSERT IGNORE INTO group_user (user_id, group_id) VALUES (?, ?)",
			"INSERT INTO group_user (user_id, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING"},
		{"INSERT IGNORE before RETURNING", "insert ignore into t (a) values (?) RETURNING id;",
			"INSERT INTO t (a) values ($1) ON CONFLICT DO NOTHING RETURNING id"},
		{"backticks quoted, literal kept", "SELECT `name` FROM `groups` WHERE note = 'a`b'",
			`SELECT "name" FROM "groups" WHERE note = 'a` + "`" + `b'`},
		{"UUID", "INSERT INTO c (salt) VALUES (UUID())", "INSERT INTO c (salt) VALUES (gen_random_uuid()::text)"},
		{"FK checks off", "SET FOREIGN_KEY_CHECKS = 0", "SET session_replication_role = replica"},
		{"FK checks on", "SET FOREIGN_KEY_CHECKS=1;", "SET session_replication_role = DEFAULT"},
		{"FROM_UNIXTIME", "WHERE FROM_UNIXTIME(t.escalation_time) < NOW()", "WHERE to_timestamp(t.escalation_time) < NOW()"},
	}
	withDriver(t, "postgres", func() {
		for _, c := range cases {
			if got := ConvertPlaceholders(c.in); got != c.want {
				t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
			}
		}
	})
}

func TestConvertPlaceholdersLeavesMySQLDialectOnMySQL(t *testing.T) {
	withDriver(t, "mysql", func() {
		for _, q := range []string{
			"INSERT IGNORE INTO group_user (user_id) VALUES (?)",
			"SELECT `name` FROM `groups` WHERE id = ?",
			"WHERE t >= DATE_SUB(NOW(), INTERVAL ? DAY) AND u = UUID()",
		} {
			if got := ConvertPlaceholders(q); got != q {
				t.Errorf("mysql query changed:\n got  %s\n want %s", got, q)
			}
		}
	})
}

func TestConvertUpsert(t *testing.T) {
	onDup := "INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, ?, ?) " +
		"ON DUPLICATE KEY UPDATE preferences_value = VALUES(preferences_value), change_by = ?"
	replace := "REPLACE INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, ?, ?)"

	withDriver(t, "postgres", func() {
		if got, want := ConvertUpsert(onDup, "user_id", "preferences_key"),
			"INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES ($1, $2, $3) "+
				"ON CONFLICT (user_id, preferences_key) DO UPDATE SET preferences_value = EXCLUDED.preferences_value, change_by = $4"; got != want {
			t.Errorf("on duplicate:\n got  %s\n want %s", got, want)
		}
		if got, want := ConvertUpsert(replace, "user_id", "preferences_key"),
			"INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES ($1, $2, $3) "+
				"ON CONFLICT (user_id, preferences_key) DO UPDATE SET preferences_value = EXCLUDED.preferences_value"; got != want {
			t.Errorf("replace:\n got  %s\n want %s", got, want)
		}
		if got, want := ConvertUpsert("REPLACE INTO service_sla (service_id, sla_id) VALUES (?, ?)", "service_id", "sla_id"),
			"INSERT INTO service_sla (service_id, sla_id) VALUES ($1, $2) ON CONFLICT (service_id, sla_id) DO NOTHING"; got != want {
			t.Errorf("all-key replace:\n got  %s\n want %s", got, want)
		}
		defer func() {
			if recover() == nil {
				t.Error("ConvertUpsert without conflict columns must panic on PostgreSQL")
			}
		}()
		ConvertUpsert(onDup)
	})
	withDriver(t, "mysql", func() {
		if got := ConvertUpsert(onDup, "user_id"); got != onDup {
			t.Errorf("mysql upsert changed: %s", got)
		}
	})
}
