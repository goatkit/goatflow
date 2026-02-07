package repository

import (
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/database"

	"github.com/goatkit/goatflow/internal/models"
)

// TicketStateRepository handles database operations for ticket states.
type TicketStateRepository struct {
	db *sql.DB
}

// NewTicketStateRepository creates a new ticket state repository.
func NewTicketStateRepository(db *sql.DB) *TicketStateRepository {
	return &TicketStateRepository{db: db}
}

// GetByID retrieves a ticket state by ID.
func (r *TicketStateRepository) GetByID(id uint) (*models.TicketState, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, type_id, comments, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE id = ?`)

	var state models.TicketState
	err := r.db.QueryRow(query, id).Scan(
		&state.ID,
		&state.Name,
		&state.TypeID,
		&state.Comments,
		&state.ValidID,
		&state.CreateTime,
		&state.CreateBy,
		&state.ChangeTime,
		&state.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket state not found")
	}

	return &state, err
}

// GetByName retrieves a ticket state by name.
func (r *TicketStateRepository) GetByName(name string) (*models.TicketState, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, type_id, comments, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE name = ? AND valid_id = 1`)

	var state models.TicketState
	err := r.db.QueryRow(query, name).Scan(
		&state.ID,
		&state.Name,
		&state.TypeID,
		&state.Comments,
		&state.ValidID,
		&state.CreateTime,
		&state.CreateBy,
		&state.ChangeTime,
		&state.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket state '%s' not found", name)
	}

	return &state, err
}

// GetByTypeID retrieves all ticket states for a specific type.
func (r *TicketStateRepository) GetByTypeID(typeID uint) ([]*models.TicketState, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, type_id, comments, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE type_id = ? AND valid_id = 1
		ORDER BY name`)

	rows, err := r.db.Query(query, typeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []*models.TicketState
	for rows.Next() {
		var state models.TicketState
		err := rows.Scan(
			&state.ID,
			&state.Name,
			&state.TypeID,
			&state.Comments,
			&state.ValidID,
			&state.CreateTime,
			&state.CreateBy,
			&state.ChangeTime,
			&state.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		states = append(states, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}

// List retrieves all active ticket states.
func (r *TicketStateRepository) List() ([]*models.TicketState, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, type_id, comments, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE valid_id = 1
		ORDER BY name`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []*models.TicketState
	for rows.Next() {
		var state models.TicketState
		err := rows.Scan(
			&state.ID,
			&state.Name,
			&state.TypeID,
			&state.Comments,
			&state.ValidID,
			&state.CreateTime,
			&state.CreateBy,
			&state.ChangeTime,
			&state.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		states = append(states, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}

// Create creates a new ticket state.
func (r *TicketStateRepository) Create(state *models.TicketState) error {
	query := database.ConvertPlaceholders(`
		INSERT INTO ticket_state (
			name, type_id, comments, valid_id,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?
		) RETURNING id`)

	err := r.db.QueryRow(
		query,
		state.Name,
		state.TypeID,
		state.Comments,
		state.ValidID,
		state.CreateTime,
		state.CreateBy,
		state.ChangeTime,
		state.ChangeBy,
	).Scan(&state.ID)

	return err
}

// Update updates a ticket state.
func (r *TicketStateRepository) Update(state *models.TicketState) error {
	query := database.ConvertPlaceholders(`
		UPDATE ticket_state SET
			name = ?,
			type_id = ?,
			comments = ?,
			valid_id = ?,
			change_time = ?,
			change_by = ?
		WHERE id = ?`)

	result, err := r.db.Exec(
		query,
		state.ID,
		state.Name,
		state.TypeID,
		state.Comments,
		state.ValidID,
		state.ChangeTime,
		state.ChangeBy,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ticket state not found")
	}

	return nil
}

// GetOpenStates returns all ticket states that are considered "open".
func (r *TicketStateRepository) GetOpenStates() ([]*models.TicketState, error) {
	// Type IDs: 1=new, 2=open, 3=pending reminder, 4=pending auto
	openTypeIDs := []int{1, 2, 3, 4}

	query := database.ConvertPlaceholders(`
		SELECT id, name, type_id, comments, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE type_id = ANY(?) AND valid_id = 1
		ORDER BY name`)

	rows, err := r.db.Query(query, openTypeIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []*models.TicketState
	for rows.Next() {
		var state models.TicketState
		err := rows.Scan(
			&state.ID,
			&state.Name,
			&state.TypeID,
			&state.Comments,
			&state.ValidID,
			&state.CreateTime,
			&state.CreateBy,
			&state.ChangeTime,
			&state.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		states = append(states, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}

// GetClosedStates returns all ticket states that are considered "closed".
func (r *TicketStateRepository) GetClosedStates() ([]*models.TicketState, error) {
	// Type IDs: 5=closed successful, 6=closed unsuccessful, 9=merged
	closedTypeIDs := []int{5, 6, 9}

	query := database.ConvertPlaceholders(`
		SELECT id, name, type_id, comments, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE type_id = ANY(?) AND valid_id = 1
		ORDER BY name`)

	rows, err := r.db.Query(query, closedTypeIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []*models.TicketState
	for rows.Next() {
		var state models.TicketState
		err := rows.Scan(
			&state.ID,
			&state.Name,
			&state.TypeID,
			&state.Comments,
			&state.ValidID,
			&state.CreateTime,
			&state.CreateBy,
			&state.ChangeTime,
			&state.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		states = append(states, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}
