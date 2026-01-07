package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// AutoCloseResult captures how many tickets each transition touched.
type AutoCloseResult struct {
	Transitions map[string]int64
	Total       int64
}

// AutoClosePendingTickets moves tickets out of pending auto-close states when deadlines expire.
func (r *TicketRepository) AutoClosePendingTickets(
	ctx context.Context,
	now time.Time,
	transitions map[string]string,
	systemUserID int,
) (*AutoCloseResult, error) {
	if len(transitions) == 0 {
		return &AutoCloseResult{Transitions: make(map[string]int64)}, nil
	}

	names := make([]string, 0, len(transitions)*2)
	nameSet := make(map[string]struct{})
	for from, to := range transitions {
		if _, ok := nameSet[from]; !ok {
			nameSet[from] = struct{}{}
			names = append(names, from)
		}
		if _, ok := nameSet[to]; !ok {
			nameSet[to] = struct{}{}
			names = append(names, to)
		}
	}

	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, name := range names {
		placeholders[i] = "?"
		args[i] = name
	}

	query := fmt.Sprintf(
		"SELECT id, name FROM ticket_state WHERE name IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, fmt.Errorf("lookup ticket states: %w", err)
	}
	defer rows.Close()

	stateIDs := make(map[string]int)
	for rows.Next() {
		var (
			id   int
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan ticket state: %w", err)
		}
		stateIDs[name] = id
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ticket states: %w", err)
	}

	result := &AutoCloseResult{Transitions: make(map[string]int64)}

	for from, to := range transitions {
		sourceID, ok := stateIDs[from]
		if !ok {
			return nil, fmt.Errorf("ticket state %q not found", from)
		}
		targetID, ok := stateIDs[to]
		if !ok {
			return nil, fmt.Errorf("ticket state %q not found", to)
		}

		update := `
			UPDATE ticket
			SET ticket_state_id = ?,
			    until_time = 0,
			    change_time = CURRENT_TIMESTAMP,
			    change_by = ?
			WHERE ticket_state_id = ?
			  AND until_time > 0
			  AND until_time <= ?
			  AND archive_flag = 0
		`

		res, err := r.db.ExecContext(
			ctx,
			database.ConvertPlaceholders(update),
			targetID,
			systemUserID,
			sourceID,
			now.Unix(),
		)
		if err != nil {
			return nil, fmt.Errorf("auto-close transition %s -> %s: %w", from, to, err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("rows affected for transition %s -> %s: %w", from, to, err)
		}

		result.Transitions[from] = affected
		result.Total += affected
	}

	return result, nil
}
