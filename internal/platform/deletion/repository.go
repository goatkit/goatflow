package deletion

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Repository provides CRUD for the recycle bin and deletion log.
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

// --- Recycle Bin ---

// AddToRecycleBin adds an entry to the recycle bin.
func (r *Repository) AddToRecycleBin(entry *RecycleBinEntry) (int64, error) {
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_recycle_bin (entity_type, entity_id, entity_name, deleted_by, deleted_at, expires_at, org_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	result, err := r.db.Exec(query,
		entry.EntityType, entry.EntityID, entry.EntityName,
		entry.DeletedBy, entry.DeletedAt, entry.ExpiresAt, entry.OrgID,
	)
	if err != nil {
		return 0, fmt.Errorf("add to recycle bin: %w", err)
	}
	return result.LastInsertId()
}

// ListRecycleBin lists entries in the recycle bin, optionally filtered by entity type.
func (r *Repository) ListRecycleBin(entityType string, orgID int64) ([]RecycleBinEntry, error) {
	query := "SELECT id, entity_type, entity_id, entity_name, deleted_by, deleted_at, expires_at, org_id FROM gk_recycle_bin"
	var args []any
	var conditions []string

	if entityType != "" {
		conditions = append(conditions, "entity_type = ?")
		args = append(args, entityType)
	}
	if orgID > 0 {
		conditions = append(conditions, "(org_id = ? OR org_id IS NULL)")
		args = append(args, orgID)
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, c := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += c
		}
	}
	query += " ORDER BY deleted_at DESC"
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recycle bin: %w", err)
	}
	defer rows.Close()

	var entries []RecycleBinEntry
	for rows.Next() {
		var e RecycleBinEntry
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.EntityName,
			&e.DeletedBy, &e.DeletedAt, &e.ExpiresAt, &e.OrgID); err != nil {
			return nil, fmt.Errorf("scan recycle bin: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetRecycleBinEntry retrieves a specific recycle bin entry.
func (r *Repository) GetRecycleBinEntry(entityType string, entityID int64) (*RecycleBinEntry, error) {
	query := database.ConvertPlaceholders(
		"SELECT id, entity_type, entity_id, entity_name, deleted_by, deleted_at, expires_at, org_id FROM gk_recycle_bin WHERE entity_type = ? AND entity_id = ?")
	var e RecycleBinEntry
	err := r.db.QueryRow(query, entityType, entityID).Scan(
		&e.ID, &e.EntityType, &e.EntityID, &e.EntityName,
		&e.DeletedBy, &e.DeletedAt, &e.ExpiresAt, &e.OrgID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get recycle bin entry: %w", err)
	}
	return &e, nil
}

// RemoveFromRecycleBin removes an entry from the recycle bin (after restore or hard delete).
func (r *Repository) RemoveFromRecycleBin(entityType string, entityID int64) error {
	query := database.ConvertPlaceholders("DELETE FROM gk_recycle_bin WHERE entity_type = ? AND entity_id = ?")
	_, err := r.db.Exec(query, entityType, entityID)
	if err != nil {
		return fmt.Errorf("remove from recycle bin: %w", err)
	}
	return nil
}

// ListExpired returns recycle bin entries that have passed their expiry date.
func (r *Repository) ListExpired() ([]RecycleBinEntry, error) {
	query := database.ConvertPlaceholders(
		"SELECT id, entity_type, entity_id, entity_name, deleted_by, deleted_at, expires_at, org_id FROM gk_recycle_bin WHERE expires_at IS NOT NULL AND expires_at < ?")
	rows, err := r.db.Query(query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("list expired: %w", err)
	}
	defer rows.Close()

	var entries []RecycleBinEntry
	for rows.Next() {
		var e RecycleBinEntry
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.EntityName,
			&e.DeletedBy, &e.DeletedAt, &e.ExpiresAt, &e.OrgID); err != nil {
			return nil, fmt.Errorf("scan expired: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// --- Tombstone Log ---

// LogDeletion writes an immutable tombstone entry.
func (r *Repository) LogDeletion(entry *DeletionLog) error {
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_deletion_log (entity_type, entity_id, action, deleted_by, deleted_at, org_id, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	_, err := r.db.Exec(query,
		entry.EntityType, entry.EntityID, entry.Action,
		entry.DeletedBy, entry.DeletedAt, entry.OrgID, entry.Reason,
	)
	if err != nil {
		return fmt.Errorf("log deletion: %w", err)
	}
	return nil
}

// GetDeletionLog retrieves deletion history for an entity.
func (r *Repository) GetDeletionLog(entityType string, entityID int64) ([]DeletionLog, error) {
	query := database.ConvertPlaceholders(
		"SELECT id, entity_type, entity_id, action, deleted_by, deleted_at, org_id, reason FROM gk_deletion_log WHERE entity_type = ? AND entity_id = ? ORDER BY deleted_at DESC")
	rows, err := r.db.Query(query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("get deletion log: %w", err)
	}
	defer rows.Close()

	var logs []DeletionLog
	for rows.Next() {
		var l DeletionLog
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID, &l.Action,
			&l.DeletedBy, &l.DeletedAt, &l.OrgID, &l.Reason); err != nil {
			return nil, fmt.Errorf("scan deletion log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- Anonymisation ---

// AnonymiseEntity replaces PII fields with [DELETED] for a given entity.
func (r *Repository) AnonymiseEntity(entityType string, entityID int64) error {
	fields, ok := AnonymiseConfig[entityType]
	if !ok || len(fields) == 0 {
		return nil // No PII to anonymise for this entity type.
	}

	for _, f := range fields {
		value := f.Value
		if value == "" {
			value = DefaultAnonymiseValue
		}
		query := database.ConvertPlaceholders(
			fmt.Sprintf("UPDATE %s SET %s = ? WHERE id = ?", f.Table, f.Column)) //nolint:gk-sql-sprintf // internal schema identifier; values bound via ?
		if _, err := r.db.Exec(query, value, entityID); err != nil {
			return fmt.Errorf("anonymise %s.%s: %w", f.Table, f.Column, err)
		}
	}
	return nil
}
