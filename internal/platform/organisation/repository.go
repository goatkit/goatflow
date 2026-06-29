package organisation

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Repository provides CRUD operations for organisations, memberships, and per-org sysconfig.
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

// --- Organisation CRUD ---

const orgColumns = `id, name, slug, parent_id, status, customer_company_id, captive_plugin,
	valid_id, create_time, create_by, change_time, change_by`

const orgColumnsAliased = `o.id, o.name, o.slug, o.parent_id, o.status, o.customer_company_id, o.captive_plugin,
	o.valid_id, o.create_time, o.create_by, o.change_time, o.change_by`

// ListOrgs retrieves organisations with optional filters.
func (r *Repository) ListOrgs(status string, activeOnly bool) ([]Organisation, error) {
	query := "SELECT " + orgColumns + " FROM gk_organisation"
	var args []any
	var conditions []string

	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if activeOnly {
		conditions = append(conditions, "valid_id = 1 AND status = 'active'")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY name"
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orgs: %w", err)
	}
	defer rows.Close()
	return scanOrgs(rows)
}

// GetOrg retrieves an organisation by ID.
func (r *Repository) GetOrg(id int64) (*Organisation, error) {
	query := database.ConvertPlaceholders("SELECT " + orgColumns + " FROM gk_organisation WHERE id = ?")
	return scanOrg(r.db.QueryRow(query, id))
}

// GetOrgBySlug retrieves an organisation by slug.
func (r *Repository) GetOrgBySlug(slug string) (*Organisation, error) {
	query := database.ConvertPlaceholders("SELECT " + orgColumns + " FROM gk_organisation WHERE slug = ?")
	return scanOrg(r.db.QueryRow(query, slug))
}

// CreateOrg inserts a new organisation. Returns the new row ID.
func (r *Repository) CreateOrg(o *Organisation, userID int) (int64, error) {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_organisation (name, slug, parent_id, status, customer_company_id,
			valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	result, err := r.db.Exec(query,
		o.Name, o.Slug, o.ParentID, o.Status, o.CustomerCompanyID,
		o.ValidID, now, userID, now, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("create org: %w", err)
	}
	return result.LastInsertId()
}

// UpdateOrg updates an existing organisation.
func (r *Repository) UpdateOrg(o *Organisation, userID int) error {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		UPDATE gk_organisation SET name = ?, slug = ?, parent_id = ?, status = ?,
			customer_company_id = ?, captive_plugin = ?, valid_id = ?,
			change_time = ?, change_by = ?
		WHERE id = ?
	`)
	_, err := r.db.Exec(query,
		o.Name, o.Slug, o.ParentID, o.Status,
		o.CustomerCompanyID, o.CaptivePlugin, o.ValidID, now, userID, o.ID,
	)
	if err != nil {
		return fmt.Errorf("update org: %w", err)
	}
	return nil
}

// SetCaptivePlugin updates only the captive_plugin column. `plugin` empty
// or nil disables capture. Kept as a narrow API so the admin UI can wire
// the Portal-tab checkbox without having to re-send the whole org row.
func (r *Repository) SetCaptivePlugin(orgID int64, plugin *string, userID int) error {
	query := database.ConvertPlaceholders(`
		UPDATE gk_organisation SET captive_plugin = ?, change_time = ?, change_by = ?
		WHERE id = ?
	`)
	_, err := r.db.Exec(query, plugin, time.Now(), userID, orgID)
	if err != nil {
		return fmt.Errorf("set captive plugin: %w", err)
	}
	return nil
}

// GetCaptivePlugin returns the captive_plugin for an org, or empty string
// if none / org not found. Hot path on customer login and the portal
// guard middleware — keep it a single-column lookup, not a full Get.
func (r *Repository) GetCaptivePlugin(orgID int64) (string, error) {
	query := database.ConvertPlaceholders(`SELECT captive_plugin FROM gk_organisation WHERE id = ?`)
	var cp sql.NullString
	if err := r.db.QueryRow(query, orgID).Scan(&cp); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	if !cp.Valid {
		return "", nil
	}
	return cp.String, nil
}

// DeleteOrg removes an organisation. Memberships and sysconfig_org cascade.
func (r *Repository) DeleteOrg(id int64) error {
	query := database.ConvertPlaceholders("DELETE FROM gk_organisation WHERE id = ?")
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete org: %w", err)
	}
	return nil
}

// --- Membership CRUD ---

// ListMembers retrieves all members of an organisation.
func (r *Repository) ListMembers(orgID int64) ([]UserOrganisation, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, org_id, user_id, customer_login, role, is_default, create_time, create_by
		FROM gk_user_organisation WHERE org_id = ? ORDER BY role, user_id, customer_login
	`)
	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	return scanMembers(rows)
}

// GetUserOrgs retrieves all organisations a user (agent) belongs to.
func (r *Repository) GetUserOrgs(userID int) ([]Organisation, error) {
	query := database.ConvertPlaceholders(`
		SELECT ` + orgColumnsAliased + `
		FROM gk_organisation o
		JOIN gk_user_organisation uo ON uo.org_id = o.id
		WHERE uo.user_id = ? AND o.valid_id = 1 AND o.status = 'active'
		ORDER BY uo.is_default DESC, o.name
	`)
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user orgs: %w", err)
	}
	defer rows.Close()
	return scanOrgs(rows)
}

// GetCustomerOrgs retrieves all organisations a customer belongs to.
func (r *Repository) GetCustomerOrgs(customerLogin string) ([]Organisation, error) {
	query := database.ConvertPlaceholders(`
		SELECT ` + orgColumnsAliased + `
		FROM gk_organisation o
		JOIN gk_user_organisation uo ON uo.org_id = o.id
		WHERE uo.customer_login = ? AND o.valid_id = 1 AND o.status = 'active'
		ORDER BY uo.is_default DESC, o.name
	`)
	rows, err := r.db.Query(query, customerLogin)
	if err != nil {
		return nil, fmt.Errorf("get customer orgs: %w", err)
	}
	defer rows.Close()
	return scanOrgs(rows)
}

// GetDefaultOrgForUser returns the user's default org, or nil if none.
func (r *Repository) GetDefaultOrgForUser(userID int) (*Organisation, error) {
	query := database.ConvertPlaceholders(`
		SELECT ` + orgColumnsAliased + `
		FROM gk_organisation o
		JOIN gk_user_organisation uo ON uo.org_id = o.id
		WHERE uo.user_id = ? AND uo.is_default = TRUE AND o.valid_id = 1
		LIMIT 1
	`)
	return scanOrg(r.db.QueryRow(query, userID))
}

// AddMember adds a user or customer to an organisation.
func (r *Repository) AddMember(m *UserOrganisation, createdBy int) (int64, error) {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_user_organisation (org_id, user_id, customer_login, role, is_default, create_time, create_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	result, err := r.db.Exec(query,
		m.OrgID, m.UserID, m.CustomerLogin, m.Role, m.IsDefault, now, createdBy,
	)
	if err != nil {
		return 0, fmt.Errorf("add member: %w", err)
	}
	return result.LastInsertId()
}

// RemoveMember removes a membership by ID.
func (r *Repository) RemoveMember(membershipID int64) error {
	query := database.ConvertPlaceholders("DELETE FROM gk_user_organisation WHERE id = ?")
	_, err := r.db.Exec(query, membershipID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// SetDefaultOrg sets the default org for a user. Clears other defaults first.
func (r *Repository) SetDefaultOrg(userID int, orgID int64) error {
	// Clear existing defaults for this user.
	clearQuery := database.ConvertPlaceholders("UPDATE gk_user_organisation SET is_default = FALSE WHERE user_id = ?")
	if _, err := r.db.Exec(clearQuery, userID); err != nil {
		return fmt.Errorf("clear defaults: %w", err)
	}
	// Set the new default.
	setQuery := database.ConvertPlaceholders("UPDATE gk_user_organisation SET is_default = TRUE WHERE user_id = ? AND org_id = ?")
	if _, err := r.db.Exec(setQuery, userID, orgID); err != nil {
		return fmt.Errorf("set default: %w", err)
	}
	return nil
}

// --- Per-Org Sysconfig ---

// GetOrgConfig retrieves a per-org sysconfig value. Returns nil if not overridden.
func (r *Repository) GetOrgConfig(orgID int64, name string) ([]byte, error) {
	query := database.ConvertPlaceholders(`
		SELECT effective_value FROM sysconfig_org
		WHERE org_id = ? AND name = ? AND is_valid = 1
	`)
	var value []byte
	err := r.db.QueryRow(query, orgID, name).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get org config %q: %w", name, err)
	}
	return value, nil
}

// SetOrgConfig creates or updates a per-org sysconfig override.
func (r *Repository) SetOrgConfig(orgID int64, name string, value []byte, userID int) error {
	now := time.Now()
	// Try update first.
	updateQuery := database.ConvertPlaceholders(`
		UPDATE sysconfig_org SET effective_value = ?, is_valid = 1, change_time = ?, change_by = ?
		WHERE org_id = ? AND name = ?
	`)
	result, err := r.db.Exec(updateQuery, value, now, userID, orgID, name)
	if err != nil {
		return fmt.Errorf("update org config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		return nil
	}
	// Insert if not exists.
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO sysconfig_org (org_id, name, effective_value, is_valid, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 1, ?, ?, ?, ?)
	`)
	_, err = r.db.Exec(insertQuery, orgID, name, value, now, userID, now, userID)
	if err != nil {
		return fmt.Errorf("insert org config: %w", err)
	}
	return nil
}

// DeleteOrgConfig removes a per-org sysconfig override (falls through to system default).
func (r *Repository) DeleteOrgConfig(orgID int64, name string) error {
	query := database.ConvertPlaceholders("DELETE FROM sysconfig_org WHERE org_id = ? AND name = ?")
	_, err := r.db.Exec(query, orgID, name)
	if err != nil {
		return fmt.Errorf("delete org config: %w", err)
	}
	return nil
}

// ListOrgConfigs lists all sysconfig overrides for an org.
func (r *Repository) ListOrgConfigs(orgID int64) ([]SysconfigOrg, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, org_id, name, effective_value, is_valid, create_time, create_by, change_time, change_by
		FROM sysconfig_org WHERE org_id = ? AND is_valid = 1 ORDER BY name
	`)
	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list org configs: %w", err)
	}
	defer rows.Close()

	var configs []SysconfigOrg
	for rows.Next() {
		var c SysconfigOrg
		if err := rows.Scan(&c.ID, &c.OrgID, &c.Name, &c.EffectiveValue, &c.IsValid,
			&c.CreateTime, &c.CreateBy, &c.ChangeTime, &c.ChangeBy); err != nil {
			return nil, fmt.Errorf("scan org config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// --- Scan helpers ---

func scanOrgs(rows *sql.Rows) ([]Organisation, error) {
	var orgs []Organisation
	for rows.Next() {
		var o Organisation
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.ParentID, &o.Status,
			&o.CustomerCompanyID, &o.CaptivePlugin, &o.ValidID, &o.CreateTime, &o.CreateBy,
			&o.ChangeTime, &o.ChangeBy); err != nil {
			return nil, fmt.Errorf("scan org: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

func scanOrg(row *sql.Row) (*Organisation, error) {
	var o Organisation
	err := row.Scan(&o.ID, &o.Name, &o.Slug, &o.ParentID, &o.Status,
		&o.CustomerCompanyID, &o.CaptivePlugin, &o.ValidID, &o.CreateTime, &o.CreateBy,
		&o.ChangeTime, &o.ChangeBy)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("scan org: %w", err)
	}
	return &o, nil
}

func scanMembers(rows *sql.Rows) ([]UserOrganisation, error) {
	var members []UserOrganisation
	for rows.Next() {
		var m UserOrganisation
		if err := rows.Scan(&m.ID, &m.OrgID, &m.UserID, &m.CustomerLogin,
			&m.Role, &m.IsDefault, &m.CreateTime, &m.CreateBy); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
