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

// Canonical driver names returned by NormalizeDriver and Driver.
const (
	DriverMySQL    = "mysql"
	DriverMariaDB  = "mariadb"
	DriverPostgres = "postgres"
)

// NormalizeDriver maps a configured driver name to its canonical form: every
// PostgreSQL alias ("postgres", "postgresql", "pgsql") becomes "postgres";
// other values are lower-cased and trimmed ("mysql", "mariadb", ...). An
// empty value means MySQL. This is the only place driver aliases are known.
func NormalizeDriver(raw string) string {
	d := strings.ToLower(strings.TrimSpace(raw))
	switch d {
	case "":
		return DriverMySQL
	case "postgres", "postgresql", "pgsql":
		return DriverPostgres
	}
	return d
}

// Driver returns the active, normalized driver: TEST_DB_DRIVER when set (test
// runs), else DB_DRIVER, else MySQL.
func Driver() string {
	raw := os.Getenv("TEST_DB_DRIVER")
	if strings.TrimSpace(raw) == "" {
		raw = os.Getenv("DB_DRIVER")
	}
	return NormalizeDriver(raw)
}

// IsPostgres reports whether the active driver is PostgreSQL.
func IsPostgres() bool {
	return Driver() == DriverPostgres
}

// Env returns the driver-scoped value of DB_<key>: DB_MYSQL_<key> when the
// active driver is MySQL/MariaDB, DB_PGSQL_<key> when it is PostgreSQL.
// Falls back to the legacy flat DB_<key> when the namespaced var is unset,
// so test targets and scripts that inject flat DB_* credentials keep working.
func Env(key string) string {
	pfx := "DB_MYSQL_"
	if IsPostgres() {
		pfx = "DB_PGSQL_"
	}
	if v := os.Getenv(pfx + key); v != "" {
		return v
	}
	return os.Getenv("DB_" + key)
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
