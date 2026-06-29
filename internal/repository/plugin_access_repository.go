package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// OrgPluginAccess is one row from gk_org_plugin_access — a binding that
// says "in org_id, members of group_id are allowed to use plugin_name".
type OrgPluginAccess struct {
	ID         int64
	OrgID      int64
	PluginName string
	GroupID    int
	GroupName  string // joined from `groups` for display
	CreateTime time.Time
	CreateBy   int
}

// PluginAccessRepository is the read/write API for gk_org_plugin_access.
type PluginAccessRepository struct {
	db *sql.DB
}

// NewPluginAccessRepository constructs a repository bound to db.
func NewPluginAccessRepository(db *sql.DB) *PluginAccessRepository {
	return &PluginAccessRepository{db: db}
}

// HasCustomerAccess is the hot-path check for RequirePluginAccess. Returns
// true when the customer_login is in at least one group entitled to use
// plugin_name in org_id. A single JOIN'd query covers the three-way
// check (enablement, group membership, customer) so the middleware stays
// cheap on every request.
func (r *PluginAccessRepository) HasCustomerAccess(orgID int64, pluginName, customerLogin string) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT 1
		  FROM gk_org_plugin_access opa
		  JOIN group_customer_user gcu ON gcu.group_id = opa.group_id
		 WHERE opa.org_id = ?
		   AND opa.plugin_name = ?
		   AND gcu.user_id = ?
		 LIMIT 1`)

	var one int
	err := r.db.QueryRow(query, orgID, pluginName, customerLogin).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("plugin access lookup: %w", err)
	}
	return true, nil
}

// ListForOrg returns every (plugin_name, group) binding for an org, used
// by the org admin page.
func (r *PluginAccessRepository) ListForOrg(orgID int64) ([]OrgPluginAccess, error) {
	query := database.ConvertPlaceholders(`
		SELECT opa.id, opa.org_id, opa.plugin_name, opa.group_id, g.name,
		       opa.create_time, opa.create_by
		  FROM gk_org_plugin_access opa
		  JOIN groups g ON g.id = opa.group_id
		 WHERE opa.org_id = ?
		 ORDER BY opa.plugin_name, g.name`)

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list plugin access: %w", err)
	}
	defer rows.Close()

	var out []OrgPluginAccess
	for rows.Next() {
		var e OrgPluginAccess
		if err := rows.Scan(&e.ID, &e.OrgID, &e.PluginName, &e.GroupID, &e.GroupName, &e.CreateTime, &e.CreateBy); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ReplaceForOrgPlugin enforces the "one group per (org, plugin)" UI rule:
// delete every existing row for (org_id, plugin_name) and insert the
// caller's chosen groupID. Passing groupID = 0 disables the plugin for
// the org (delete without re-insert). createdBy records the admin making
// the change for auditability.
func (r *PluginAccessRepository) ReplaceForOrgPlugin(orgID int64, pluginName string, groupID int, createdBy int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(database.ConvertPlaceholders(
		`DELETE FROM gk_org_plugin_access WHERE org_id = ? AND plugin_name = ?`),
		orgID, pluginName); err != nil {
		return fmt.Errorf("delete existing access: %w", err)
	}

	if groupID > 0 {
		if _, err := tx.Exec(database.ConvertPlaceholders(
			`INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id, create_time, create_by)
			 VALUES (?, ?, ?, ?, ?)`),
			orgID, pluginName, groupID, time.Now().UTC(), createdBy); err != nil {
			return fmt.Errorf("insert new access: %w", err)
		}
	}

	return tx.Commit()
}
