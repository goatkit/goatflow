package database

import (
	"database/sql"
	"fmt"
)

// ScanFunc is a function that scans a single row.
// It receives the rows object and should call rows.Scan() to extract values.
// Return an error to abort iteration, or nil to continue.
type ScanFunc func(rows *sql.Rows) error

// ScanRows iterates over sql.Rows, calling scanFn for each row, and properly
// checks rows.Err() after iteration completes. This is the correct pattern for
// database row iteration that prevents silent iteration errors.
//
// Usage:
//
//	var results []MyType
//	err := database.ScanRows(rows, func(rows *sql.Rows) error {
//	    var item MyType
//	    if err := rows.Scan(&item.Field1, &item.Field2); err != nil {
//	        return err
//	    }
//	    results = append(results, item)
//	    return nil
//	})
func ScanRows(rows *sql.Rows, scanFn ScanFunc) error {
	for rows.Next() {
		if err := scanFn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// CollectRows is a generic helper that collects rows into a slice.
// The scanFn should scan a single row and return the value.
// This is a convenience wrapper around ScanRows for the common case.
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

// CollectInts scans rows containing a single int column.
// This is a common pattern for queries like "SELECT id FROM table".
func CollectInts(rows *sql.Rows) ([]int, error) {
	return CollectRows(rows, func(r *sql.Rows) (int, error) {
		var i int
		err := r.Scan(&i)
		return i, err
	})
}

// CollectInt64s scans rows containing a single int64 column.
func CollectInt64s(rows *sql.Rows) ([]int64, error) {
	return CollectRows(rows, func(r *sql.Rows) (int64, error) {
		var i int64
		err := r.Scan(&i)
		return i, err
	})
}

// MustCloseRows closes rows and wraps any error with context.
// Use as: defer database.MustCloseRows(rows, &err)
// This ensures rows.Close() errors are captured even if the function
// already has an error.
func MustCloseRows(rows *sql.Rows, errPtr *error) {
	closeErr := rows.Close()
	if closeErr != nil && *errPtr == nil {
		*errPtr = fmt.Errorf("closing rows: %w", closeErr)
	}
}

// RowsErr checks rows.Err() and returns it if non-nil, otherwise returns existingErr.
// This is useful for the common pattern of checking rows.Err() at the end of iteration.
// Deprecated: Use ScanRows or CollectRows instead which handle this automatically.
func RowsErr(rows *sql.Rows, existingErr error) error {
	if existingErr != nil {
		return existingErr
	}
	return rows.Err()
}
