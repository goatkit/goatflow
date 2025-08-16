package repository

import (
	"database/sql"
	"fmt"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// QueueRepository handles database operations for queues
type QueueRepository struct {
	db *sql.DB
}

// NewQueueRepository creates a new queue repository
func NewQueueRepository(db *sql.DB) *QueueRepository {
	return &QueueRepository{db: db}
}

// GetByID retrieves a queue by ID
func (r *QueueRepository) GetByID(id uint) (*models.Queue, error) {
	query := `
		SELECT id, name, system_address_id, calendar_id, default_sign_key,
		       salutation_id, signature_id, follow_up_id, follow_up_lock,
		       unlock_timeout, group_id, email, realname, comments,
		       valid_id, create_time, create_by, change_time, change_by
		FROM queues
		WHERE id = $1`

	var queue models.Queue
	err := r.db.QueryRow(query, id).Scan(
		&queue.ID,
		&queue.Name,
		&queue.SystemAddressID,
		&queue.CalendarID,
		&queue.DefaultSignKey,
		&queue.SalutationID,
		&queue.SignatureID,
		&queue.FollowUpID,
		&queue.FollowUpLock,
		&queue.UnlockTimeout,
		&queue.GroupID,
		&queue.Email,
		&queue.RealName,
		&queue.Comments,
		&queue.ValidID,
		&queue.CreateTime,
		&queue.CreateBy,
		&queue.ChangeTime,
		&queue.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("queue not found")
	}

	return &queue, err
}

// GetByName retrieves a queue by name
func (r *QueueRepository) GetByName(name string) (*models.Queue, error) {
	query := `
		SELECT id, name, system_address_id, calendar_id, default_sign_key,
		       salutation_id, signature_id, follow_up_id, follow_up_lock,
		       unlock_timeout, group_id, email, realname, comments,
		       valid_id, create_time, create_by, change_time, change_by
		FROM queues
		WHERE name = $1 AND valid_id = 1`

	var queue models.Queue
	err := r.db.QueryRow(query, name).Scan(
		&queue.ID,
		&queue.Name,
		&queue.SystemAddressID,
		&queue.CalendarID,
		&queue.DefaultSignKey,
		&queue.SalutationID,
		&queue.SignatureID,
		&queue.FollowUpID,
		&queue.FollowUpLock,
		&queue.UnlockTimeout,
		&queue.GroupID,
		&queue.Email,
		&queue.RealName,
		&queue.Comments,
		&queue.ValidID,
		&queue.CreateTime,
		&queue.CreateBy,
		&queue.ChangeTime,
		&queue.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("queue '%s' not found", name)
	}

	return &queue, err
}

// List retrieves all active queues
func (r *QueueRepository) List() ([]*models.Queue, error) {
	query := `
		SELECT id, name, system_address_id, calendar_id, default_sign_key,
		       salutation_id, signature_id, follow_up_id, follow_up_lock,
		       unlock_timeout, group_id, email, realname, comments,
		       valid_id, create_time, create_by, change_time, change_by
		FROM queues
		WHERE valid_id = 1
		ORDER BY name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []*models.Queue
	for rows.Next() {
		var queue models.Queue
		err := rows.Scan(
			&queue.ID,
			&queue.Name,
			&queue.SystemAddressID,
			&queue.CalendarID,
			&queue.DefaultSignKey,
			&queue.SalutationID,
			&queue.SignatureID,
			&queue.FollowUpID,
			&queue.FollowUpLock,
			&queue.UnlockTimeout,
			&queue.GroupID,
			&queue.Email,
			&queue.RealName,
			&queue.Comments,
			&queue.ValidID,
			&queue.CreateTime,
			&queue.CreateBy,
			&queue.ChangeTime,
			&queue.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		queues = append(queues, &queue)
	}

	return queues, nil
}

// Create creates a new queue
func (r *QueueRepository) Create(queue *models.Queue) error {
	query := `
		INSERT INTO queues (
			name, system_address_id, calendar_id, default_sign_key,
			salutation_id, signature_id, follow_up_id, follow_up_lock,
			unlock_timeout, group_id, email, realname, comments,
			valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		) RETURNING id`

	err := r.db.QueryRow(
		query,
		queue.Name,
		queue.SystemAddressID,
		queue.CalendarID,
		queue.DefaultSignKey,
		queue.SalutationID,
		queue.SignatureID,
		queue.FollowUpID,
		queue.FollowUpLock,
		queue.UnlockTimeout,
		queue.GroupID,
		queue.Email,
		queue.RealName,
		queue.Comments,
		queue.ValidID,
		queue.CreateTime,
		queue.CreateBy,
		queue.ChangeTime,
		queue.ChangeBy,
	).Scan(&queue.ID)

	return err
}

// Update updates a queue
func (r *QueueRepository) Update(queue *models.Queue) error {
	query := `
		UPDATE queues SET
			name = $2,
			system_address_id = $3,
			calendar_id = $4,
			default_sign_key = $5,
			salutation_id = $6,
			signature_id = $7,
			follow_up_id = $8,
			follow_up_lock = $9,
			unlock_timeout = $10,
			group_id = $11,
			email = $12,
			realname = $13,
			comments = $14,
			valid_id = $15,
			change_time = $16,
			change_by = $17
		WHERE id = $1`

	result, err := r.db.Exec(
		query,
		queue.ID,
		queue.Name,
		queue.SystemAddressID,
		queue.CalendarID,
		queue.DefaultSignKey,
		queue.SalutationID,
		queue.SignatureID,
		queue.FollowUpID,
		queue.FollowUpLock,
		queue.UnlockTimeout,
		queue.GroupID,
		queue.Email,
		queue.RealName,
		queue.Comments,
		queue.ValidID,
		queue.ChangeTime,
		queue.ChangeBy,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("queue not found")
	}

	return nil
}