package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// IsConnectionError reports whether the provided error indicates the database
// connection is unavailable. It is intentionally broad so handlers can return
// a 503 response instead of treating these failures as bad requests.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "host is unreachable"),
		strings.Contains(msg, "network is unreachable"),
		strings.Contains(msg, "broken pipe"),
		strings.Contains(msg, "bad connection"),
		strings.Contains(msg, "database is closed"):
		return true
	}
	return false
}
