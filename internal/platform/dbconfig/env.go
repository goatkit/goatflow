// Package dbconfig resolves the driver-scoped database connection environment.
//
// Connection variables are namespaced per driver — DB_MYSQL_* and DB_PGSQL_* —
// and DB_DRIVER selects the active set at runtime. This leaf package lets both
// internal/platform/database and internal/platform/services/adapter resolve the
// active connection without an import cycle.
package dbconfig

import (
	"os"
	"strconv"
	"strings"
)

func isPostgres() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER"))) {
	case "postgres", "postgresql", "pgsql":
		return true
	default:
		return false
	}
}

// Env returns the driver-scoped value of DB_<key>: DB_MYSQL_<key> when the
// active driver is MySQL/MariaDB, DB_PGSQL_<key> when it is PostgreSQL.
func Env(key string) string {
	pfx := "DB_MYSQL_"
	if isPostgres() {
		pfx = "DB_PGSQL_"
	}
	return os.Getenv(pfx + key)
}

// EnvDefault returns Env(key), or def when the driver-scoped value is unset or
// empty.
func EnvDefault(key, def string) string {
	if v := Env(key); v != "" {
		return v
	}
	return def
}

// EnvInt returns Env(key) parsed as an int, or def when unset or unparseable.
func EnvInt(key string, def int) int {
	if v := Env(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
