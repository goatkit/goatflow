package repository

import (
	"database/sql"
	"fmt"

	"github.com/gotrs-io/gotrs-ce/internal/database"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// PriorityRepository handles database operations for ticket priorities.
type PriorityRepository struct {
	db *sql.DB
}

// NewPriorityRepository creates a new priority repository.
func NewPriorityRepository(db *sql.DB) *PriorityRepository {
	return &PriorityRepository{db: db}
}

// GetByID retrieves a priority by ID.
func (r *PriorityRepository) GetByID(id uint) (*models.TicketPriority, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE id = $1`)

	var priority models.TicketPriority
	err := r.db.QueryRow(query, id).Scan(
		&priority.ID,
		&priority.Name,
		&priority.ValidID,
		&priority.CreateTime,
		&priority.CreateBy,
		&priority.ChangeTime,
		&priority.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("priority not found")
	}

	return &priority, err
}

// List retrieves all active priorities.
func (r *PriorityRepository) List() ([]*models.TicketPriority, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY id`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var priorities []*models.TicketPriority
	for rows.Next() {
		var priority models.TicketPriority
		err := rows.Scan(
			&priority.ID,
			&priority.Name,
			&priority.ValidID,
			&priority.CreateTime,
			&priority.CreateBy,
			&priority.ChangeTime,
			&priority.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		priorities = append(priorities, &priority)
	}

	return priorities, nil
}

// GetByName retrieves a priority by name.
func (r *PriorityRepository) GetByName(name string) (*models.TicketPriority, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE name = $1 AND valid_id = 1`)

	var priority models.TicketPriority
	err := r.db.QueryRow(query, name).Scan(
		&priority.ID,
		&priority.Name,
		&priority.ValidID,
		&priority.CreateTime,
		&priority.CreateBy,
		&priority.ChangeTime,
		&priority.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("priority not found")
	}

	return &priority, err
}
