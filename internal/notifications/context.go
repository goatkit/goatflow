// Package notifications provides notification context and delivery management.
package notifications

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// BuildRenderContext fetches agent and customer names for placeholder interpolation.
func BuildRenderContext(ctx context.Context, db *sql.DB, customerLogin string, agentID int) *RenderContext {
	if db == nil {
		return &RenderContext{}
	}

	rc := &RenderContext{}

	if strings.TrimSpace(customerLogin) != "" {
		// Match on login OR email since customer_user_id could contain either
		var first, last sql.NullString
		if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(`SELECT first_name, last_name FROM customer_user WHERE login = ? OR email = ?`), customerLogin, customerLogin).Scan(&first, &last); err == nil {
			rc.CustomerFullName = strings.TrimSpace(strings.TrimSpace(first.String + " " + last.String))
		}
		if rc.CustomerFullName == "" {
			rc.CustomerFullName = strings.TrimSpace(customerLogin)
		}
	}

	if agentID > 0 {
		var first, last, login sql.NullString
		if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(`SELECT first_name, last_name, login FROM users WHERE id = ?`), agentID).Scan(&first, &last, &login); err == nil {
			rc.AgentFirstName = strings.TrimSpace(first.String)
			rc.AgentLastName = strings.TrimSpace(last.String)
			if rc.AgentFirstName == "" && rc.AgentLastName == "" {
				rc.AgentFirstName = strings.TrimSpace(login.String)
			}
		}
	}

	return rc
}
