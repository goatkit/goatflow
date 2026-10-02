package database

import "database/sql"

// CollectRows is a generic helper that collects rows into a slice.
// The scanFn should scan a single row and return the value.
//
// Usage:
//
//	users, err := database.CollectRows(rows, func(rows *sql.Rows) (*User, error) {
//	    var u User
//	    err := rows.Scan(&u.ID, &u.Name, &u.Email)
//	    return &u, err
//	})
func CollectRows[T any](rows *sql.Rows, scanFn func(rows *sql.Rows) (T, error)) ([]T, error) {
	var results []T
	for rows.Next() {
		item, err := scanFn(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// CollectStrings scans rows containing a single string column.
// This is a common pattern for queries like "SELECT name FROM table".
func CollectStrings(rows *sql.Rows) ([]string, error) {
	return CollectRows(rows, func(r *sql.Rows) (string, error) {
		var s string
		err := r.Scan(&s)
		return s, err
	})
}
