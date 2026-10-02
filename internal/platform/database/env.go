package database

import "github.com/goatkit/goatflow/internal/platform/dbconfig"

// DB_* connection variables are namespaced per driver (DB_MYSQL_* / DB_PGSQL_*),
// selected by DB_DRIVER (see internal/platform/dbconfig).

// Env returns the driver-scoped value of DB_<key>.
func Env(key string) string { return dbconfig.Env(key) }
