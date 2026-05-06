package sysconfig

import "database/sql"

// Value reads a sysconfig value, preferring modified overrides and falling back
// to sysconfig_default or the embedded defaults.yaml when the tables are absent.
func Value(db *sql.DB, name string) (string, bool) {
	return sysconfigValue(db, name)
}
