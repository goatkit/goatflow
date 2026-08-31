// Package ticketstate provides shared ticket-state operations used by both
// core API handlers and the plugin HostAPI, so state-change semantics live in
// exactly one place.
package ticketstate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// StateIsPending reports whether a ticket state is a pending-type state.
// Pending detection is by type name (prefix "pending", case-insensitive) —
// no hardcoded state ids: live deployments carry custom states whose ids do
// not match the seed migration numbering. Returns an error when the state
// does not exist or is invalid.
func StateIsPending(ctx context.Context, db *sql.DB, stateID int64) (bool, error) {
	var typeName string
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT tsst.name
		FROM ticket_state ts
		JOIN ticket_state_type tsst ON ts.type_id = tsst.id
		WHERE ts.id = ? AND ts.valid_id = 1
	`), stateID).Scan(&typeName)
	if err == sql.ErrNoRows {
		return false, fmt.Errorf("ticket state %d not found or invalid", stateID)
	}
	if err != nil {
		return false, fmt.Errorf("lookup ticket state %d: %w", stateID, err)
	}
	return strings.HasPrefix(strings.ToLower(typeName), "pending"), nil
}

// ChangeTicketStatus changes a ticket's state with core semantics:
// pending-type states require untilTime > 0; non-pending states clear
// until_time. Mirrors handleAgentTicketStatus in internal/api exactly.
func ChangeTicketStatus(ctx context.Context, db *sql.DB, ticketID, stateID, userID int64, untilTime int64) error {
	pending, err := StateIsPending(ctx, db, stateID)
	if err != nil {
		return err
	}
	if pending && untilTime == 0 {
		return fmt.Errorf("pending time is required for pending states")
	}
	if !pending {
		untilTime = 0
	}

	res, err := db.ExecContext(ctx, database.ConvertPlaceholders(`
		UPDATE ticket
		SET ticket_state_id = ?, until_time = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
		WHERE id = ?
	`), stateID, untilTime, userID, ticketID)
	if err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("ticket %d not found", ticketID)
	}
	return nil
}

// ListTicketStates returns all valid ticket states with their type info,
// ordered by id (the table has no sort_order column).
func ListTicketStates(ctx context.Context, db *sql.DB) ([]pkgplugin.TicketStateInfo, error) {
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT ts.id, ts.name, COALESCE(ts.color, ''), ts.type_id, tsst.name
		FROM ticket_state ts
		JOIN ticket_state_type tsst ON ts.type_id = tsst.id
		WHERE ts.valid_id = 1
		ORDER BY ts.id
	`))
	if err != nil {
		return nil, fmt.Errorf("list ticket states: %w", err)
	}
	defer rows.Close()

	states := make([]pkgplugin.TicketStateInfo, 0, 8)
	for rows.Next() {
		var s pkgplugin.TicketStateInfo
		if err := rows.Scan(&s.ID, &s.Name, &s.Color, &s.TypeID, &s.TypeName); err != nil {
			return nil, fmt.Errorf("scan ticket state: %w", err)
		}
		states = append(states, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ticket states: %w", err)
	}
	return states, nil
}
