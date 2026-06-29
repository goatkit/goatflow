package secureconfig

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// SecureEntry represents a row in gk_secure_config.
type SecureEntry struct {
	ID             int64     `json:"id" db:"id"`
	PluginName     string    `json:"plugin_name" db:"plugin_name"`
	Name           string    `json:"name" db:"name"`
	EncryptedValue []byte    `json:"-" db:"encrypted_value"`
	ValueHint      *string   `json:"value_hint,omitempty" db:"value_hint"`
	OrgID          *int64    `json:"org_id,omitempty" db:"org_id"`
	CreateTime     time.Time `json:"create_time" db:"create_time"`
	CreateBy       int       `json:"create_by" db:"create_by"`
	ChangeTime     time.Time `json:"change_time" db:"change_time"`
	ChangeBy       int       `json:"change_by" db:"change_by"`
}

// Repository provides CRUD for encrypted secrets.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a repository using the global DB.
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

// Get retrieves an encrypted value. Checks org-specific first, then global.
func (r *Repository) Get(pluginName, name string, orgID int64) (*SecureEntry, error) {
	// Try org-specific first.
	if orgID > 0 {
		entry, err := r.getEntry(pluginName, name, &orgID)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			return entry, nil
		}
	}
	// Fall back to global.
	return r.getEntry(pluginName, name, nil)
}

func (r *Repository) getEntry(pluginName, name string, orgID *int64) (*SecureEntry, error) {
	var query string
	var args []any
	if orgID != nil {
		query = database.ConvertPlaceholders(
			"SELECT id, plugin_name, name, encrypted_value, value_hint, org_id, create_time, create_by, change_time, change_by FROM gk_secure_config WHERE plugin_name = ? AND name = ? AND org_id = ?")
		args = []any{pluginName, name, *orgID}
	} else {
		query = database.ConvertPlaceholders(
			"SELECT id, plugin_name, name, encrypted_value, value_hint, org_id, create_time, create_by, change_time, change_by FROM gk_secure_config WHERE plugin_name = ? AND name = ? AND org_id IS NULL")
		args = []any{pluginName, name}
	}

	var e SecureEntry
	err := r.db.QueryRow(query, args...).Scan(
		&e.ID, &e.PluginName, &e.Name, &e.EncryptedValue, &e.ValueHint,
		&e.OrgID, &e.CreateTime, &e.CreateBy, &e.ChangeTime, &e.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get secure config: %w", err)
	}
	return &e, nil
}

// Set creates or updates an encrypted value.
func (r *Repository) Set(pluginName, name string, encryptedValue []byte, hint string, orgID int64, userID int) error {
	now := time.Now()
	var orgIDPtr *int64
	if orgID > 0 {
		orgIDPtr = &orgID
	}

	// Try update first.
	var updateQuery string
	var updateArgs []any
	if orgIDPtr != nil {
		updateQuery = database.ConvertPlaceholders(
			"UPDATE gk_secure_config SET encrypted_value = ?, value_hint = ?, change_time = ?, change_by = ? WHERE plugin_name = ? AND name = ? AND org_id = ?")
		updateArgs = []any{encryptedValue, hint, now, userID, pluginName, name, *orgIDPtr}
	} else {
		updateQuery = database.ConvertPlaceholders(
			"UPDATE gk_secure_config SET encrypted_value = ?, value_hint = ?, change_time = ?, change_by = ? WHERE plugin_name = ? AND name = ? AND org_id IS NULL")
		updateArgs = []any{encryptedValue, hint, now, userID, pluginName, name}
	}

	result, err := r.db.Exec(updateQuery, updateArgs...)
	if err != nil {
		return fmt.Errorf("update secure config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		return nil
	}

	// Insert.
	insertQuery := database.ConvertPlaceholders(
		"INSERT INTO gk_secure_config (plugin_name, name, encrypted_value, value_hint, org_id, create_time, create_by, change_time, change_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
	_, err = r.db.Exec(insertQuery, pluginName, name, encryptedValue, hint, orgIDPtr, now, userID, now, userID)
	if err != nil {
		return fmt.Errorf("insert secure config: %w", err)
	}
	return nil
}

// Delete removes a secret.
func (r *Repository) Delete(pluginName, name string, orgID int64) error {
	var query string
	var args []any
	if orgID > 0 {
		query = database.ConvertPlaceholders("DELETE FROM gk_secure_config WHERE plugin_name = ? AND name = ? AND org_id = ?")
		args = []any{pluginName, name, orgID}
	} else {
		query = database.ConvertPlaceholders("DELETE FROM gk_secure_config WHERE plugin_name = ? AND name = ? AND org_id IS NULL")
		args = []any{pluginName, name}
	}
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete secure config: %w", err)
	}
	return nil
}

// ListForPlugin returns all secrets for a plugin (with masked values for admin display).
func (r *Repository) ListForPlugin(pluginName string) ([]SecureEntry, error) {
	query := database.ConvertPlaceholders(
		"SELECT id, plugin_name, name, encrypted_value, value_hint, org_id, create_time, create_by, change_time, change_by FROM gk_secure_config WHERE plugin_name = ? ORDER BY name, org_id")
	rows, err := r.db.Query(query, pluginName)
	if err != nil {
		return nil, fmt.Errorf("list secure configs: %w", err)
	}
	defer rows.Close()

	var entries []SecureEntry
	for rows.Next() {
		var e SecureEntry
		if err := rows.Scan(&e.ID, &e.PluginName, &e.Name, &e.EncryptedValue, &e.ValueHint,
			&e.OrgID, &e.CreateTime, &e.CreateBy, &e.ChangeTime, &e.ChangeBy); err != nil {
			return nil, fmt.Errorf("scan secure config: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
