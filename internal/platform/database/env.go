package database

import "github.com/goatkit/goatflow/internal/platform/dbconfig"

// DB_* connection variables are namespaced per driver (DB_MYSQL_* / DB_PGSQL_*),
// selected by DB_DRIVER (see internal/platform/dbconfig). These wrappers expose
// the resolver to this package's callers.

// Env returns the driver-scoped value of DB_<key>.
func Env(key string) string { return dbconfig.Env(key) }

// EnvDefault returns Env(key), or def when unset/empty.
func EnvDefault(key, def string) string { return dbconfig.EnvDefault(key, def) }

// EnvInt returns Env(key) parsed as an int, or def when unset/unparseable.
func EnvInt(key string, def int) int { return dbconfig.EnvInt(key, def) }
