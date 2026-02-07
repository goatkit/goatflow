package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
)

// SystemMaintenanceRepository handles database operations for system maintenance records.
type SystemMaintenanceRepository struct {
	db *sql.DB
}

// NewSystemMaintenanceRepository creates a new system maintenance repository.
func NewSystemMaintenanceRepository(db *sql.DB) *SystemMaintenanceRepository {
	return &SystemMaintenanceRepository{db: db}
}

// Create inserts a new system maintenance record.
func (r *SystemMaintenanceRepository) Create(m *models.SystemMaintenance) error {
	query := `
		INSERT INTO system_maintenance (
			start_date, stop_date, comments, login_message, show_login_message,
			notify_message, valid_id, create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`

	now := time.Now()
	m.CreateTime = now
	m.ChangeTime = now

	adapter := database.GetAdapter()
	id, err := adapter.InsertWithReturning(
		r.db,
		query,
		m.StartDate,
		m.StopDate,
		m.Comments,
		m.LoginMessage,
		m.ShowLoginMessage,
		m.NotifyMessage,
		m.ValidID,
		m.CreateTime,
		m.CreateBy,
		m.ChangeTime,
		m.ChangeBy,
	)
	if err != nil {
		return err
	}

	m.ID = int(id)
	return nil
}

// GetByID retrieves a system maintenance record by ID.
func (r *SystemMaintenanceRepository) GetByID(id int) (*models.SystemMaintenance, error) {
	query := `
		SELECT id, start_date, stop_date, comments, login_message, show_login_message,
			notify_message, valid_id, create_time, create_by, change_time, change_by
		FROM system_maintenance
		WHERE id = ?`

	adapter := database.GetAdapter()
	var m models.SystemMaintenance
	err := adapter.QueryRow(r.db, query, id).Scan(
		&m.ID,
		&m.StartDate,
		&m.StopDate,
		&m.Comments,
		&m.LoginMessage,
		&m.ShowLoginMessage,
		&m.NotifyMessage,
		&m.ValidID,
		&m.CreateTime,
		&m.CreateBy,
		&m.ChangeTime,
		&m.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("system maintenance record not found")
	}

	return &m, err
}

// Update updates an existing system maintenance record.
func (r *SystemMaintenanceRepository) Update(m *models.SystemMaintenance) error {
	query := `
		UPDATE system_maintenance SET
			start_date = ?,
			stop_date = ?,
			comments = ?,
			login_message = ?,
			show_login_message = ?,
			notify_message = ?,
			valid_id = ?,
			change_time = ?,
			change_by = ?
		WHERE id = ?`

	m.ChangeTime = time.Now()

	adapter := database.GetAdapter()
	result, err := adapter.Exec(
		r.db,
		query,
		m.StartDate,
		m.StopDate,
		m.Comments,
		m.LoginMessage,
		m.ShowLoginMessage,
		m.NotifyMessage,
		m.ValidID,
		m.ChangeTime,
		m.ChangeBy,
		m.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("system maintenance record not found")
	}

	return nil
}

// Delete removes a system maintenance record by ID.
func (r *SystemMaintenanceRepository) Delete(id int) error {
	query := `DELETE FROM system_maintenance WHERE id = ?`

	adapter := database.GetAdapter()
	result, err := adapter.Exec(r.db, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("system maintenance record not found")
	}

	return nil
}

// List retrieves all system maintenance records.
func (r *SystemMaintenanceRepository) List() ([]*models.SystemMaintenance, error) {
	query := `
		SELECT id, start_date, stop_date, comments, login_message, show_login_message,
			notify_message, valid_id, create_time, create_by, change_time, change_by
		FROM system_maintenance
		ORDER BY start_date DESC`

	adapter := database.GetAdapter()
	rows, err := adapter.Query(r.db, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*models.SystemMaintenance
	for rows.Next() {
		var m models.SystemMaintenance
		err := rows.Scan(
			&m.ID,
			&m.StartDate,
			&m.StopDate,
			&m.Comments,
			&m.LoginMessage,
			&m.ShowLoginMessage,
			&m.NotifyMessage,
			&m.ValidID,
			&m.CreateTime,
			&m.CreateBy,
			&m.ChangeTime,
			&m.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, &m)
	}

	return records, rows.Err()
}

// ListValid retrieves all valid (active) system maintenance records.
func (r *SystemMaintenanceRepository) ListValid() ([]*models.SystemMaintenance, error) {
	query := `
		SELECT id, start_date, stop_date, comments, login_message, show_login_message,
			notify_message, valid_id, create_time, create_by, change_time, change_by
		FROM system_maintenance
		WHERE valid_id = 1
		ORDER BY start_date DESC`

	adapter := database.GetAdapter()
	rows, err := adapter.Query(r.db, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*models.SystemMaintenance
	for rows.Next() {
		var m models.SystemMaintenance
		err := rows.Scan(
			&m.ID,
			&m.StartDate,
			&m.StopDate,
			&m.Comments,
			&m.LoginMessage,
			&m.ShowLoginMessage,
			&m.NotifyMessage,
			&m.ValidID,
			&m.CreateTime,
			&m.CreateBy,
			&m.ChangeTime,
			&m.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, &m)
	}

	return records, rows.Err()
}

// IsActive returns the currently active maintenance record, or nil if none.
// A maintenance is active if current time is between start_date and stop_date and valid_id = 1.
func (r *SystemMaintenanceRepository) IsActive() (*models.SystemMaintenance, error) {
	now := time.Now().Unix()

	query := `
		SELECT id, start_date, stop_date, comments, login_message, show_login_message,
			notify_message, valid_id, create_time, create_by, change_time, change_by
		FROM system_maintenance
		WHERE valid_id = 1
			AND start_date <= ?
			AND stop_date >= ?
		ORDER BY start_date ASC
		LIMIT 1`

	adapter := database.GetAdapter()
	var m models.SystemMaintenance
	err := adapter.QueryRow(r.db, query, now, now).Scan(
		&m.ID,
		&m.StartDate,
		&m.StopDate,
		&m.Comments,
		&m.LoginMessage,
		&m.ShowLoginMessage,
		&m.NotifyMessage,
		&m.ValidID,
		&m.CreateTime,
		&m.CreateBy,
		&m.ChangeTime,
		&m.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No active maintenance
	}
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// IsComing returns an upcoming maintenance record within the specified minutes, or nil if none.
// An upcoming maintenance is one that starts within the next X minutes and is valid.
func (r *SystemMaintenanceRepository) IsComing(withinMinutes int) (*models.SystemMaintenance, error) {
	now := time.Now().Unix()
	futureThreshold := now + int64(withinMinutes*60)

	query := `
		SELECT id, start_date, stop_date, comments, login_message, show_login_message,
			notify_message, valid_id, create_time, create_by, change_time, change_by
		FROM system_maintenance
		WHERE valid_id = 1
			AND start_date > ?
			AND start_date <= ?
		ORDER BY start_date ASC
		LIMIT 1`

	adapter := database.GetAdapter()
	var m models.SystemMaintenance
	err := adapter.QueryRow(r.db, query, now, futureThreshold).Scan(
		&m.ID,
		&m.StartDate,
		&m.StopDate,
		&m.Comments,
		&m.LoginMessage,
		&m.ShowLoginMessage,
		&m.NotifyMessage,
		&m.ValidID,
		&m.CreateTime,
		&m.CreateBy,
		&m.ChangeTime,
		&m.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No upcoming maintenance
	}
	if err != nil {
		return nil, err
	}

	return &m, nil
}
