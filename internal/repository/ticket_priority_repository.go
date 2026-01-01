package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// TicketPriorityRepository handles database operations for ticket priorities.
type TicketPriorityRepository struct {
	db *sql.DB
}

// NewTicketPriorityRepository creates a new ticket priority repository.
func NewTicketPriorityRepository(db *sql.DB) *TicketPriorityRepository {
	return &TicketPriorityRepository{db: db}
}

// GetByID retrieves a ticket priority by ID.
func (r *TicketPriorityRepository) GetByID(id uint) (*models.TicketPriority, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, color, create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE id = $1`)

	var priority models.TicketPriority
	err := r.db.QueryRow(query, id).Scan(
		&priority.ID,
		&priority.Name,
		&priority.ValidID,
		&priority.Color,
		&priority.CreateTime,
		&priority.CreateBy,
		&priority.ChangeTime,
		&priority.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket priority not found")
	}

	return &priority, err
}

// GetByName retrieves a ticket priority by name.
func (r *TicketPriorityRepository) GetByName(name string) (*models.TicketPriority, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, color, create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE name = $1 AND valid_id = 1`)

	var priority models.TicketPriority
	err := r.db.QueryRow(query, name).Scan(
		&priority.ID,
		&priority.Name,
		&priority.ValidID,
		&priority.Color,
		&priority.CreateTime,
		&priority.CreateBy,
		&priority.ChangeTime,
		&priority.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket priority '%s' not found", name)
	}

	return &priority, err
}

// List retrieves all active ticket priorities.
func (r *TicketPriorityRepository) List() ([]*models.TicketPriority, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, color, create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY id`) // Order by ID to maintain priority order (1=very low, 5=very high)

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
			&priority.Color,
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

// Create creates a new ticket priority.
func (r *TicketPriorityRepository) Create(priority *models.TicketPriority) error {
	if priority.CreateTime.IsZero() {
		priority.CreateTime = time.Now().UTC()
	}

	if priority.ChangeTime.IsZero() {
		priority.ChangeTime = priority.CreateTime
	}

	query := database.ConvertPlaceholders(`
		INSERT INTO ticket_priority (
			name, valid_id, color, create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING id`)

	err := r.db.QueryRow(
		query,
		priority.Name,
		priority.ValidID,
		priority.Color,
		priority.CreateTime,
		priority.CreateBy,
		priority.ChangeTime,
		priority.ChangeBy,
	).Scan(&priority.ID)

	return err
}

// Update updates a ticket priority.
func (r *TicketPriorityRepository) Update(priority *models.TicketPriority) error {
	if priority.ChangeTime.IsZero() {
		priority.ChangeTime = time.Now().UTC()
	}

	query := database.ConvertPlaceholders(`
		UPDATE ticket_priority SET
			name = $2,
			valid_id = $3,
			color = $4,
			change_time = $5,
			change_by = $6
		WHERE id = $1`)

	result, err := r.db.Exec(
		query,
		priority.ID,
		priority.Name,
		priority.ValidID,
		priority.Color,
		priority.ChangeTime,
		priority.ChangeBy,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ticket priority not found")
	}

	return nil
}

// GetDefault returns the default priority (usually "normal" with ID 3).
func (r *TicketPriorityRepository) GetDefault() (*models.TicketPriority, error) {
	return r.GetByName("normal")
}

// GetHighPriorities returns all priorities considered "high" (4=high, 5=very high).
func (r *TicketPriorityRepository) GetHighPriorities() ([]*models.TicketPriority, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, color, create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE id >= 4 AND valid_id = 1
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
			&priority.Color,
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
