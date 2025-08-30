package repository

import (
	"database/sql"
	"fmt"

	"github.com/gotrs-io/gotrs-ce/internal/database"
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
		SELECT id, name, system_address_id, salutation_id, signature_id,
		       follow_up_id, follow_up_lock, unlock_timeout, group_id,
		       comments, valid_id, create_time, create_by, change_time, change_by
		FROM queue
		WHERE id = $1`
	
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

// GetByName retrieves a queue by name
func (r *QueueRepository) GetByName(name string) (*models.Queue, error) {
	query := `
		SELECT id, name, system_address_id, salutation_id, signature_id,
		       follow_up_id, follow_up_lock, unlock_timeout, group_id,
		       comments, valid_id, create_time, create_by, change_time, change_by
		FROM queue
		WHERE name = $1 AND valid_id = 1`
	
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

// List retrieves all active queues
func (r *QueueRepository) List() ([]*models.Queue, error) {
	query := `
		SELECT id, name, system_address_id, salutation_id, signature_id,
		       follow_up_id, follow_up_lock, unlock_timeout, group_id,
		       comments, valid_id, create_time, create_by, change_time, change_by
		FROM queue
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
		var systemAddressID, salutationID, signatureID sql.NullInt32
		var comments sql.NullString
		
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
		
		queues = append(queues, &queue)
	}

	return queues, nil
}

// Create creates a new queue
func (r *QueueRepository) Create(queue *models.Queue) error {
	query := `
		INSERT INTO queue (
			name, system_address_id, salutation_id, signature_id,
			follow_up_id, follow_up_lock, unlock_timeout, group_id,
			comments, valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
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
		query,
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

// Update updates a queue
func (r *QueueRepository) Update(queue *models.Queue) error {
	query := `
		UPDATE queue SET
			name = $2,
			system_address_id = $3,
			salutation_id = $4,
			signature_id = $5,
			follow_up_id = $6,
			follow_up_lock = $7,
			unlock_timeout = $8,
			group_id = $9,
			comments = $10,
			valid_id = $11,
			change_time = $12,
			change_by = $13
		WHERE id = $1`

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
		query,
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