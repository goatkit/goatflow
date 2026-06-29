package pluginui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Repository provides CRUD operations for plugin UI registrations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a repository using the global DB connection.
func NewRepository() (*Repository, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// NewRepositoryWithDB creates a repository with an explicit DB connection.
func NewRepositoryWithDB(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// List retrieves plugin UIs with optional filters.
func (r *Repository) List(pluginName, uiType string, enabledOnly bool) ([]PluginUI, error) {
	query := `SELECT id, plugin_name, ui_id, full_id, name, description,
	                 ui_type, shell, icon, config, enabled, custom_domain,
	                 valid_id, create_time, create_by, change_time, change_by
	          FROM gk_plugin_ui`

	var args []any
	var conditions []string

	if pluginName != "" {
		conditions = append(conditions, "plugin_name = ?")
		args = append(args, pluginName)
	}
	if uiType != "" {
		conditions = append(conditions, "ui_type = ?")
		args = append(args, uiType)
	}
	if enabledOnly {
		conditions = append(conditions, "enabled = 1 AND valid_id = 1")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY plugin_name, ui_type, name"
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list plugin UIs: %w", err)
	}
	defer rows.Close()

	return scanPluginUIs(rows)
}

// GetByFullID retrieves a plugin UI by its full ID ({plugin}_{ui_id}).
func (r *Repository) GetByFullID(fullID string) (*PluginUI, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, plugin_name, ui_id, full_id, name, description,
		       ui_type, shell, icon, config, enabled, custom_domain,
		       valid_id, create_time, create_by, change_time, change_by
		FROM gk_plugin_ui WHERE full_id = ?
	`)
	row := r.db.QueryRow(query, fullID)
	return scanPluginUI(row)
}

// GetByID retrieves a plugin UI by its database ID.
func (r *Repository) GetByID(id int64) (*PluginUI, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, plugin_name, ui_id, full_id, name, description,
		       ui_type, shell, icon, config, enabled, custom_domain,
		       valid_id, create_time, create_by, change_time, change_by
		FROM gk_plugin_ui WHERE id = ?
	`)
	row := r.db.QueryRow(query, id)
	return scanPluginUI(row)
}

// Create inserts a new plugin UI. Returns the new row ID.
func (r *Repository) Create(u *PluginUI, userID int) (int64, error) {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_plugin_ui (
			plugin_name, ui_id, full_id, name, description,
			ui_type, shell, icon, config, enabled, custom_domain,
			valid_id, create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	result, err := r.db.Exec(query,
		u.PluginName, u.UIID, u.FullID, u.Name, u.Description,
		u.UIType, u.Shell, u.Icon, u.Config, u.Enabled, u.CustomDomain,
		u.ValidID, now, userID, now, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("create plugin UI: %w", err)
	}
	return result.LastInsertId()
}

// Update updates an existing plugin UI.
func (r *Repository) Update(u *PluginUI, userID int) error {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		UPDATE gk_plugin_ui SET
			name = ?, description = ?, shell = ?, icon = ?,
			config = ?, enabled = ?, custom_domain = ?,
			valid_id = ?, change_time = ?, change_by = ?
		WHERE id = ?
	`)
	_, err := r.db.Exec(query,
		u.Name, u.Description, u.Shell, u.Icon,
		u.Config, u.Enabled, u.CustomDomain,
		u.ValidID, now, userID, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update plugin UI: %w", err)
	}
	return nil
}

// SetEnabled enables or disables a plugin UI.
func (r *Repository) SetEnabled(id int64, enabled bool, userID int) error {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		UPDATE gk_plugin_ui SET enabled = ?, change_time = ?, change_by = ? WHERE id = ?
	`)
	_, err := r.db.Exec(query, enabled, now, userID, id)
	if err != nil {
		return fmt.Errorf("set plugin UI enabled: %w", err)
	}
	return nil
}

// UpdateAdminOverrides updates administrator-managed presentation settings.
func (r *Repository) UpdateAdminOverrides(id int64, customDomain *string, branding *UIBrandingConfig, userID int) (*PluginUI, error) {
	u, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil //nolint:nilnil
	}

	u.CustomDomain = cleanOptionalString(customDomain)
	if err := u.SetBranding(branding); err != nil {
		return nil, fmt.Errorf("set branding: %w", err)
	}
	if err := r.Update(u, userID); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

// Delete removes a plugin UI permanently.
func (r *Repository) Delete(id int64) error {
	query := database.ConvertPlaceholders("DELETE FROM gk_plugin_ui WHERE id = ?")
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete plugin UI: %w", err)
	}
	return nil
}

// ListActive returns all enabled, valid plugin UIs. Used at startup for route registration.
func (r *Repository) ListActive() ([]PluginUI, error) {
	return r.List("", "", true)
}

// --- Scan helpers ---

func scanPluginUIs(rows *sql.Rows) ([]PluginUI, error) {
	var uis []PluginUI
	for rows.Next() {
		var u PluginUI
		err := rows.Scan(
			&u.ID, &u.PluginName, &u.UIID, &u.FullID, &u.Name, &u.Description,
			&u.UIType, &u.Shell, &u.Icon, &u.Config, &u.Enabled, &u.CustomDomain,
			&u.ValidID, &u.CreateTime, &u.CreateBy, &u.ChangeTime, &u.ChangeBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan plugin UI: %w", err)
		}
		uis = append(uis, u)
	}
	return uis, rows.Err()
}

func scanPluginUI(row *sql.Row) (*PluginUI, error) {
	var u PluginUI
	err := row.Scan(
		&u.ID, &u.PluginName, &u.UIID, &u.FullID, &u.Name, &u.Description,
		&u.UIType, &u.Shell, &u.Icon, &u.Config, &u.Enabled, &u.CustomDomain,
		&u.ValidID, &u.CreateTime, &u.CreateBy, &u.ChangeTime, &u.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("scan plugin UI: %w", err)
	}
	return &u, nil
}

// BuildFullID constructs the full ID from plugin name and UI ID.
func BuildFullID(pluginName, uiID string) string {
	return pluginName + "_" + uiID
}

// UISpecToConfig converts a plugin UISpec to the stored JSON config.
func UISpecToConfig(spec interface{}) (*json.RawMessage, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	jrm := json.RawMessage(raw)
	return &jrm, nil
}
