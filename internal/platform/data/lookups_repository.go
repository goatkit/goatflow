// Package data provides data access repositories for lookup tables.
package data

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// LookupItem represents a database lookup value.
type LookupItem struct {
	ID      int
	Name    string
	ValidID int
	TypeID  int // For states
}

// LookupsRepository handles database operations for lookup values.
type LookupsRepository struct {
	db *sql.DB
}

// NewLookupsRepository creates a new lookups repository.
func NewLookupsRepository(db *sql.DB) *LookupsRepository {
	return &LookupsRepository{
		db: db,
	}
}

// GetTicketStates fetches all ticket states from the database.
func (r *LookupsRepository) GetTicketStates(ctx context.Context) ([]LookupItem, error) {
	// Workflow order by state type NAME (ids differ between installs).
	query := `
		SELECT s.id, s.name, s.valid_id, s.type_id
		FROM ticket_state s
		JOIN ticket_state_type st ON st.id = s.type_id
		ORDER BY
			CASE st.name
				WHEN 'new' THEN 1
				WHEN 'open' THEN 2
				WHEN 'pending reminder' THEN 3
				WHEN 'pending auto' THEN 4
				WHEN 'closed' THEN 5
				WHEN 'merged' THEN 6
				WHEN 'removed' THEN 7
				ELSE 8
			END,
			s.name
	`

	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(query))
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket states: %w", err)
	}
	defer rows.Close()

	var states []LookupItem
	for rows.Next() {
		var state LookupItem
		err := rows.Scan(&state.ID, &state.Name, &state.ValidID, &state.TypeID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket state: %w", err)
		}
		states = append(states, state)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticket states: %w", err)
	}

	return states, nil
}

// GetTicketPriorities fetches all ticket priorities from the database.
func (r *LookupsRepository) GetTicketPriorities(ctx context.Context) ([]LookupItem, error) {
	query := `
		SELECT id, name, valid_id
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY 
			CASE 
				WHEN name LIKE '1 %' THEN 1
				WHEN name LIKE '2 %' THEN 2
				WHEN name LIKE '3 %' THEN 3
				WHEN name LIKE '4 %' THEN 4
				WHEN name LIKE '5 %' THEN 5
				ELSE 6
			END,
			name
	`

	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(query))
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket priorities: %w", err)
	}
	defer rows.Close()

	var priorities []LookupItem
	for rows.Next() {
		var priority LookupItem
		err := rows.Scan(&priority.ID, &priority.Name, &priority.ValidID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket priority: %w", err)
		}
		priorities = append(priorities, priority)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticket priorities: %w", err)
	}

	return priorities, nil
}

// GetQueues fetches all queues from the database.
func (r *LookupsRepository) GetQueues(ctx context.Context) ([]LookupItem, error) {
	query := `
		SELECT id, name, valid_id
		FROM queue
		WHERE valid_id = 1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(query))
	if err != nil {
		return nil, fmt.Errorf("failed to query queues: %w", err)
	}
	defer rows.Close()

	var queues []LookupItem
	for rows.Next() {
		var queue LookupItem
		err := rows.Scan(&queue.ID, &queue.Name, &queue.ValidID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan queue: %w", err)
		}
		queues = append(queues, queue)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating queues: %w", err)
	}

	return queues, nil
}

// GetTicketTypes fetches the valid ticket types (OTRS ticket_type table).
func (r *LookupsRepository) GetTicketTypes(ctx context.Context) ([]LookupItem, error) {
	query := `
		SELECT id, name, valid_id
		FROM ticket_type
		WHERE valid_id = 1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(query))
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket types: %w", err)
	}
	defer rows.Close()

	var types []LookupItem
	for rows.Next() {
		var ticketType LookupItem
		err := rows.Scan(&ticketType.ID, &ticketType.Name, &ticketType.ValidID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket type: %w", err)
		}
		types = append(types, ticketType)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticket types: %w", err)
	}

	return types, nil
}
