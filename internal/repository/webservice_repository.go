package repository

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"gopkg.in/yaml.v3"
)

// WebserviceRepository handles database operations for GenericInterface webservices.
type WebserviceRepository struct {
	db *sql.DB
}

// NewWebserviceRepository creates a new webservice repository.
func NewWebserviceRepository(db *sql.DB) *WebserviceRepository {
	return &WebserviceRepository{db: db}
}

// GetByID retrieves a webservice configuration by ID.
func (r *WebserviceRepository) GetByID(ctx context.Context, id int) (*models.WebserviceConfig, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, config, valid_id, create_time, create_by, change_time, change_by
		FROM gi_webservice_config
		WHERE id = ?
	`)

	ws := &models.WebserviceConfig{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&ws.ID, &ws.Name, &ws.ConfigRaw, &ws.ValidID,
		&ws.CreateTime, &ws.CreateBy, &ws.ChangeTime, &ws.ChangeBy,
	)
	if err != nil {
		return nil, err
	}

	if err := r.parseConfig(ws); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return ws, nil
}

// GetByName retrieves a webservice configuration by name.
func (r *WebserviceRepository) GetByName(ctx context.Context, name string) (*models.WebserviceConfig, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, config, valid_id, create_time, create_by, change_time, change_by
		FROM gi_webservice_config
		WHERE name = ?
	`)

	ws := &models.WebserviceConfig{}
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&ws.ID, &ws.Name, &ws.ConfigRaw, &ws.ValidID,
		&ws.CreateTime, &ws.CreateBy, &ws.ChangeTime, &ws.ChangeBy,
	)
	if err != nil {
		return nil, err
	}

	if err := r.parseConfig(ws); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return ws, nil
}

// List retrieves all webservice configurations.
func (r *WebserviceRepository) List(ctx context.Context) ([]*models.WebserviceConfig, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, config, valid_id, create_time, create_by, change_time, change_by
		FROM gi_webservice_config
		ORDER BY name
	`)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webservices []*models.WebserviceConfig
	for rows.Next() {
		ws := &models.WebserviceConfig{}
		err := rows.Scan(
			&ws.ID, &ws.Name, &ws.ConfigRaw, &ws.ValidID,
			&ws.CreateTime, &ws.CreateBy, &ws.ChangeTime, &ws.ChangeBy,
		)
		if err != nil {
			return nil, err
		}

		if err := r.parseConfig(ws); err != nil {
			// Log but continue - don't fail entire list for one bad config
			ws.Config = &models.WebserviceConfigData{}
		}

		webservices = append(webservices, ws)
	}

	return webservices, rows.Err()
}

// ListValid retrieves only valid (active) webservice configurations.
func (r *WebserviceRepository) ListValid(ctx context.Context) ([]*models.WebserviceConfig, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, config, valid_id, create_time, create_by, change_time, change_by
		FROM gi_webservice_config
		WHERE valid_id = 1
		ORDER BY name
	`)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webservices []*models.WebserviceConfig
	for rows.Next() {
		ws := &models.WebserviceConfig{}
		err := rows.Scan(
			&ws.ID, &ws.Name, &ws.ConfigRaw, &ws.ValidID,
			&ws.CreateTime, &ws.CreateBy, &ws.ChangeTime, &ws.ChangeBy,
		)
		if err != nil {
			return nil, err
		}

		if err := r.parseConfig(ws); err != nil {
			ws.Config = &models.WebserviceConfigData{}
		}

		webservices = append(webservices, ws)
	}

	return webservices, rows.Err()
}

// Create creates a new webservice configuration.
func (r *WebserviceRepository) Create(ctx context.Context, ws *models.WebserviceConfig, userID int) (int, error) {
	// Serialize config to YAML
	configYAML, err := yaml.Marshal(ws.Config)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize config: %w", err)
	}

	now := time.Now()

	query := database.ConvertPlaceholders(`
		INSERT INTO gi_webservice_config (name, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)

	result, err := r.db.ExecContext(ctx, query,
		ws.Name, configYAML, ws.ValidID, now, userID, now, userID,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Create history entry
	if err := r.createHistoryEntry(ctx, int(id), configYAML, userID); err != nil {
		// Log but don't fail - history is secondary
		_ = err
	}

	return int(id), nil
}

// Update updates an existing webservice configuration.
func (r *WebserviceRepository) Update(ctx context.Context, ws *models.WebserviceConfig, userID int) error {
	// Serialize config to YAML
	configYAML, err := yaml.Marshal(ws.Config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	now := time.Now()

	query := database.ConvertPlaceholders(`
		UPDATE gi_webservice_config
		SET name = ?, config = ?, valid_id = ?, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := r.db.ExecContext(ctx, query,
		ws.Name, configYAML, ws.ValidID, now, userID, ws.ID,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	// Create history entry
	if err := r.createHistoryEntry(ctx, ws.ID, configYAML, userID); err != nil {
		_ = err
	}

	return nil
}

// Delete deletes a webservice configuration.
func (r *WebserviceRepository) Delete(ctx context.Context, id int) error {
	// Delete history entries first (foreign key constraint)
	historyQuery := database.ConvertPlaceholders(`
		DELETE FROM gi_webservice_config_history WHERE config_id = ?
	`)
	if _, err := r.db.ExecContext(ctx, historyQuery, id); err != nil {
		return fmt.Errorf("failed to delete history: %w", err)
	}

	// Delete the config
	query := database.ConvertPlaceholders(`
		DELETE FROM gi_webservice_config WHERE id = ?
	`)

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Exists checks if a webservice with the given name exists.
func (r *WebserviceRepository) Exists(ctx context.Context, name string) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(SELECT 1 FROM gi_webservice_config WHERE name = ? LIMIT 1)
	`)
	var exists bool
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	return exists, err
}

// ExistsExcluding checks if a webservice with the given name exists, excluding a specific ID.
func (r *WebserviceRepository) ExistsExcluding(ctx context.Context, name string, excludeID int) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(SELECT 1 FROM gi_webservice_config WHERE name = ? AND id != ? LIMIT 1)
	`)
	var exists bool
	err := r.db.QueryRowContext(ctx, query, name, excludeID).Scan(&exists)
	return exists, err
}

// GetHistory retrieves the configuration history for a webservice.
func (r *WebserviceRepository) GetHistory(ctx context.Context, configID int) ([]*models.WebserviceConfigHistory, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, config_id, config, config_md5, create_time, create_by, change_time, change_by
		FROM gi_webservice_config_history
		WHERE config_id = ?
		ORDER BY create_time DESC
	`)

	rows, err := r.db.QueryContext(ctx, query, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*models.WebserviceConfigHistory
	for rows.Next() {
		h := &models.WebserviceConfigHistory{}
		err := rows.Scan(
			&h.ID, &h.ConfigID, &h.Config, &h.ConfigMD5,
			&h.CreateTime, &h.CreateBy, &h.ChangeTime, &h.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, rows.Err()
}

// GetHistoryEntry retrieves a specific history entry.
func (r *WebserviceRepository) GetHistoryEntry(ctx context.Context, historyID int64) (*models.WebserviceConfigHistory, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, config_id, config, config_md5, create_time, create_by, change_time, change_by
		FROM gi_webservice_config_history
		WHERE id = ?
	`)

	h := &models.WebserviceConfigHistory{}
	err := r.db.QueryRowContext(ctx, query, historyID).Scan(
		&h.ID, &h.ConfigID, &h.Config, &h.ConfigMD5,
		&h.CreateTime, &h.CreateBy, &h.ChangeTime, &h.ChangeBy,
	)
	if err != nil {
		return nil, err
	}

	return h, nil
}

// RestoreFromHistory restores a webservice configuration from a history entry.
func (r *WebserviceRepository) RestoreFromHistory(ctx context.Context, historyID int64, userID int) error {
	// Get the history entry
	history, err := r.GetHistoryEntry(ctx, historyID)
	if err != nil {
		return fmt.Errorf("failed to get history entry: %w", err)
	}

	// Update the current config with the historical config
	now := time.Now()
	query := database.ConvertPlaceholders(`
		UPDATE gi_webservice_config
		SET config = ?, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := r.db.ExecContext(ctx, query, history.Config, now, userID, history.ConfigID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	// Create new history entry for the restore action
	if err := r.createHistoryEntry(ctx, history.ConfigID, history.Config, userID); err != nil {
		_ = err
	}

	return nil
}

// parseConfig parses the raw YAML config into structured data.
func (r *WebserviceRepository) parseConfig(ws *models.WebserviceConfig) error {
	if len(ws.ConfigRaw) == 0 {
		ws.Config = &models.WebserviceConfigData{}
		return nil
	}

	config := &models.WebserviceConfigData{}
	if err := yaml.Unmarshal(ws.ConfigRaw, config); err != nil {
		return err
	}
	ws.Config = config
	return nil
}

// createHistoryEntry creates a history entry for config changes.
func (r *WebserviceRepository) createHistoryEntry(ctx context.Context, configID int, configYAML []byte, userID int) error {
	// Calculate MD5 hash
	hash := md5.Sum(configYAML)
	configMD5 := hex.EncodeToString(hash[:])

	// Check if this exact config already exists in history (avoid duplicates)
	checkQuery := database.ConvertPlaceholders(`
		SELECT EXISTS(SELECT 1 FROM gi_webservice_config_history WHERE config_md5 = ? LIMIT 1)
	`)
	var exists bool
	if err := r.db.QueryRowContext(ctx, checkQuery, configMD5).Scan(&exists); err != nil {
		return err
	}
	if exists {
		// Skip if identical config already in history
		return nil
	}

	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO gi_webservice_config_history (config_id, config, config_md5, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)

	_, err := r.db.ExecContext(ctx, query,
		configID, configYAML, configMD5, now, userID, now, userID,
	)
	return err
}

// GetValidWebservicesForField returns valid webservices suitable for dynamic field configuration.
// This is used by the WebserviceDropdown/WebserviceMultiselect field types.
func (r *WebserviceRepository) GetValidWebservicesForField(ctx context.Context) ([]*models.WebserviceConfig, error) {
	// Get valid webservices that have Requester config with invokers
	webservices, err := r.ListValid(ctx)
	if err != nil {
		return nil, err
	}

	// Filter to only those with at least one invoker
	var result []*models.WebserviceConfig
	for _, ws := range webservices {
		if ws.Config != nil && len(ws.Config.Requester.Invoker) > 0 {
			result = append(result, ws)
		}
	}

	return result, nil
}
