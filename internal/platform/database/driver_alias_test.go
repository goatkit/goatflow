package database

import "testing"

// Every PostgreSQL alias dbconfig accepts for connection settings must also
// switch SQL generation to PostgreSQL, or DB_DRIVER=pgsql connects to
// PostgreSQL but emits MySQL SQL.
func TestPostgresAliasesSelectPostgreSQL(t *testing.T) {
	for _, alias := range []string{"postgres", "postgresql", "pgsql", " PgSQL "} {
		t.Run(alias, func(t *testing.T) {
			t.Setenv("TEST_DB_DRIVER", "")
			t.Setenv("DB_DRIVER", alias)
			if !IsPostgreSQL() || IsMySQL() {
				t.Fatalf("DB_DRIVER=%q: IsPostgreSQL=%v IsMySQL=%v", alias, IsPostgreSQL(), IsMySQL())
			}
			if got := GetDBDriver(); got != "postgres" {
				t.Fatalf("GetDBDriver() = %q, want postgres", got)
			}
			if _, ok := buildAdapterFromEnv().(*PostgreSQLAdapter); !ok {
				t.Fatal("adapter must be PostgreSQLAdapter")
			}
			if got := ConvertPlaceholders("SELECT ? , ?"); got != "SELECT $1 , $2" {
				t.Fatalf("ConvertPlaceholders = %q, want PostgreSQL placeholders", got)
			}
		})
	}
	for _, alias := range []string{"mysql", "mariadb", "MariaDB", ""} {
		t.Setenv("TEST_DB_DRIVER", "")
		t.Setenv("DB_DRIVER", alias)
		if !IsMySQL() || IsPostgreSQL() {
			t.Fatalf("DB_DRIVER=%q must select MySQL", alias)
		}
	}
}
