// Package notifications provides notification context and delivery management.
package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// BuildRenderContext fetches agent and customer names for placeholder interpolation.
//
// A customer login with no customer_user row is legitimate (tickets may carry a
// bare email/login), so the customer name falls back to that login. A missing
// database, a failed query, or an agent id with no users row is an error: the
// caller must not send an email with names it could not resolve.
func BuildRenderContext(ctx context.Context, db *sql.DB, customerLogin string, agentID int) (*RenderContext, error) {
	if db == nil {
		return nil, errors.New("render context: database unavailable")
	}

	rc := &RenderContext{}

	if login := strings.TrimSpace(customerLogin); login != "" {
		// Match on login OR email since customer_user_id could contain either
		var first, last sql.NullString
		err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
			`SELECT first_name, last_name FROM customer_user WHERE login = ? OR email = ?`),
			customerLogin, customerLogin).Scan(&first, &last)
		switch {
		case err == nil:
			rc.CustomerFullName = strings.TrimSpace(first.String + " " + last.String)
		case errors.Is(err, sql.ErrNoRows):
		default:
			return nil, fmt.Errorf("render context: customer %q lookup: %w", login, err)
		}
		if rc.CustomerFullName == "" {
			rc.CustomerFullName = login
		}
	}

	if agentID > 0 {
		var first, last, login sql.NullString
		err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
			`SELECT first_name, last_name, login FROM users WHERE id = ?`), agentID).Scan(&first, &last, &login)
		if err != nil {
			return nil, fmt.Errorf("render context: agent %d lookup: %w", agentID, err)
		}
		rc.AgentFirstName = strings.TrimSpace(first.String)
		rc.AgentLastName = strings.TrimSpace(last.String)
		if rc.AgentFirstName == "" && rc.AgentLastName == "" {
			rc.AgentFirstName = strings.TrimSpace(login.String)
		}
	}

	return rc, nil
}
