package repository

import (
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
)

// QueueRepository handles database operations for queues.
type QueueRepository struct {
	db *sql.DB
}

// NewQueueRepository creates a new queue repository.
func NewQueueRepository(db *sql.DB) *QueueRepository {
	return &QueueRepository{db: db}
}

// GetByID retrieves a queue by ID.
func (r *QueueRepository) GetByID(id uint) (*models.Queue, error) {
	query := `
		SELECT id, name, system_address_id, salutation_id, signature_id,
		       follow_up_id, follow_up_lock, unlock_timeout, group_id,
		       comments, valid_id, create_time, create_by, change_time, change_by
		FROM queue
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	var queue models.Queue
	var systemAddressID, salutationID, signatureID sql.NullInt32
	var comments sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&queue.ID,
		&queue.Name,
		&systemAddressID,
		&salutationID,
		&signatureID,
		&queue.FollowUpID,
		&queue.FollowUpLock,
		&queue.UnlockTimeout,
		&queue.GroupID,
		&comments,
		&queue.ValidID,
		&queue.CreateTime,
		&queue.CreateBy,
		&queue.ChangeTime,
		&queue.ChangeBy,
	)

	if systemAddressID.Valid {
		queue.SystemAddressID = int(systemAddressID.Int32)
	}
	if salutationID.Valid {
		queue.SalutationID = int(salutationID.Int32)
	}
	if signatureID.Valid {
		queue.SignatureID = int(signatureID.Int32)
	}
	if comments.Valid {
		queue.Comment = comments.String
	}

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("queue not found")
	}

	return &queue, err
}

// GetByName retrieves a queue by name.
func (r *QueueRepository) GetByName(name string) (*models.Queue, error) {
	query := `
		SELECT id, name, system_address_id, salutation_id, signature_id,
		       follow_up_id, follow_up_lock, unlock_timeout, group_id,
		       comments, valid_id, create_time, create_by, change_time, change_by
		FROM queue
		WHERE name = ? AND valid_id = 1`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	var queue models.Queue
	var systemAddressID, salutationID, signatureID sql.NullInt32
	var comments sql.NullString

	err := r.db.QueryRow(query, name).Scan(
		&queue.ID,
		&queue.Name,
		&systemAddressID,
		&salutationID,
		&signatureID,
		&queue.FollowUpID,
		&queue.FollowUpLock,
		&queue.UnlockTimeout,
		&queue.GroupID,
		&comments,
		&queue.ValidID,
		&queue.CreateTime,
		&queue.CreateBy,
		&queue.ChangeTime,
		&queue.ChangeBy,
	)

	if systemAddressID.Valid {
		queue.SystemAddressID = int(systemAddressID.Int32)
	}
	if salutationID.Valid {
		queue.SalutationID = int(salutationID.Int32)
	}
	if signatureID.Valid {
		queue.SignatureID = int(signatureID.Int32)
	}
	if comments.Valid {
		queue.Comment = comments.String
	}

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("queue '%s' not found", name)
	}

	return &queue, err
}

// List retrieves all active queues.
func (r *QueueRepository) List() ([]*models.Queue, error) {
	query := `
		SELECT q.id, q.name, q.system_address_id, q.salutation_id, q.signature_id,
		       q.follow_up_id, q.follow_up_lock, q.unlock_timeout, q.group_id,
		       q.comments, q.valid_id, q.create_time, q.create_by, q.change_time, q.change_by,
		       g.name as group_name
		FROM queue q
		LEFT JOIN groups g ON q.group_id = g.id
		WHERE q.valid_id = 1
		ORDER BY q.name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []*models.Queue
	for rows.Next() {
		var queue models.Queue
		var (
			systemAddressID, salutationID, signatureID sql.NullInt32
			comments, groupName                        sql.NullString
		)

		err := rows.Scan(
			&queue.ID,
			&queue.Name,
			&systemAddressID,
			&salutationID,
			&signatureID,
			&queue.FollowUpID,
			&queue.FollowUpLock,
			&queue.UnlockTimeout,
			&queue.GroupID,
			&comments,
			&queue.ValidID,
			&queue.CreateTime,
			&queue.CreateBy,
			&queue.ChangeTime,
			&queue.ChangeBy,
			&groupName,
		)
		if err != nil {
			return nil, err
		}

		if systemAddressID.Valid {
			queue.SystemAddressID = int(systemAddressID.Int32)
		}
		if salutationID.Valid {
			queue.SalutationID = int(salutationID.Int32)
		}
		if signatureID.Valid {
			queue.SignatureID = int(signatureID.Int32)
		}
		if comments.Valid {
			queue.Comment = comments.String
		}
		if groupName.Valid {
			queue.GroupName = groupName.String
		}

		queues = append(queues, &queue)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return queues, nil
}

// Create creates a new queue.
func (r *QueueRepository) Create(queue *models.Queue) error {
	query := `
		INSERT INTO queue (
			name, system_address_id, salutation_id, signature_id,
			follow_up_id, follow_up_lock, unlock_timeout, group_id,
			comments, valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		) RETURNING id`

	var systemAddressID, salutationID, signatureID sql.NullInt32
	var comments sql.NullString

	if queue.SystemAddressID > 0 {
		systemAddressID = sql.NullInt32{Int32: int32(queue.SystemAddressID), Valid: true}
	}
	if queue.SalutationID > 0 {
		salutationID = sql.NullInt32{Int32: int32(queue.SalutationID), Valid: true}
	}
	if queue.SignatureID > 0 {
		signatureID = sql.NullInt32{Int32: int32(queue.SignatureID), Valid: true}
	}
	if queue.Comment != "" {
		comments = sql.NullString{String: queue.Comment, Valid: true}
	}

	err := r.db.QueryRow(
		database.ConvertPlaceholders(query),
		queue.Name,
		systemAddressID,
		salutationID,
		signatureID,
		queue.FollowUpID,
		queue.FollowUpLock,
		queue.UnlockTimeout,
		queue.GroupID,
		comments,
		queue.ValidID,
		queue.CreateTime,
		queue.CreateBy,
		queue.ChangeTime,
		queue.ChangeBy,
	).Scan(&queue.ID)

	return err
}

// Update updates a queue.
func (r *QueueRepository) Update(queue *models.Queue) error {
	query := `
		UPDATE queue SET
			name = ?,
			system_address_id = ?,
			salutation_id = ?,
			signature_id = ?,
			follow_up_id = ?,
			follow_up_lock = ?,
			unlock_timeout = ?,
			group_id = ?,
			comments = ?,
			valid_id = ?,
			change_time = ?,
			change_by = ?
		WHERE id = ?`

	var systemAddressID, salutationID, signatureID sql.NullInt32
	var comments sql.NullString

	if queue.SystemAddressID > 0 {
		systemAddressID = sql.NullInt32{Int32: int32(queue.SystemAddressID), Valid: true}
	}
	if queue.SalutationID > 0 {
		salutationID = sql.NullInt32{Int32: int32(queue.SalutationID), Valid: true}
	}
	if queue.SignatureID > 0 {
		signatureID = sql.NullInt32{Int32: int32(queue.SignatureID), Valid: true}
	}
	if queue.Comment != "" {
		comments = sql.NullString{String: queue.Comment, Valid: true}
	}

	result, err := r.db.Exec(
		database.ConvertPlaceholders(query),
		queue.ID,
		queue.Name,
		systemAddressID,
		salutationID,
		signatureID,
		queue.FollowUpID,
		queue.FollowUpLock,
		queue.UnlockTimeout,
		queue.GroupID,
		comments,
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
