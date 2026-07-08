package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/database"
)

var (
	ErrIdentityProviderNotFound = errors.New("identity provider not found")
)

// IdentityProviderRepository handles database operations for identity providers.
type IdentityProviderRepository struct {
	db *sql.DB
}

// NewIdentityProviderRepository creates a new identity provider repository.
func NewIdentityProviderRepository(db *sql.DB) *IdentityProviderRepository {
	return &IdentityProviderRepository{db: db}
}

// CreateProvider inserts a new identity provider.
func (r *IdentityProviderRepository) CreateProvider(p *models.IdentityProvider) error {
	query := database.ConvertPlaceholders(
		`INSERT INTO gk_identity_provider
		(name, provider_type, client_id, discovery_url, scopes,
		 user_claim_email, user_claim_name, user_claim_groups,
		 org_id, enabled, auto_provision, user_table, auto_add_to_group,
		 create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	var orgIDVal *int64
	if p.OrgID != nil {
		v := int64(*p.OrgID)
		orgIDVal = &v
	}
	result, err := r.db.Exec(
		query,
		p.Name,
		p.ProviderType,
		p.ClientID,
		p.DiscoveryURL,
		p.Scopes,
		p.UserClaimEmail,
		p.UserClaimName,
		p.UserClaimGroups,
		orgIDVal,
		p.Enabled,
		p.AutoProvision,
		p.UserTable,
		p.AutoAddToGroup,
		p.CreateTime,
		p.CreateBy,
		p.ChangeTime,
		p.ChangeBy,
	)
	if err != nil {
		return fmt.Errorf("create identity provider: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	p.ID = uint(id)
	return nil
}

// GetProvider returns an identity provider by ID.
func (r *IdentityProviderRepository) GetProvider(id uint) (*models.IdentityProvider, error) {
	query := database.ConvertPlaceholders(
		`SELECT id, org_id, name, provider_type, client_id, client_secret, discovery_url, scopes,
		 user_claim_email, user_claim_name, user_claim_groups,
		 enabled, auto_provision, user_table, auto_add_to_group,
		 create_time, create_by, change_time, change_by
		FROM gk_identity_provider WHERE id = ?`,
	)
	var p models.IdentityProvider
	var orgID sql.NullInt64
	var claimGroups sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&p.ID, &orgID, &p.Name, &p.ProviderType, &p.ClientID, &p.ClientSecret,
		&p.DiscoveryURL, &p.Scopes,
		&p.UserClaimEmail, &p.UserClaimName,
		&claimGroups, &p.Enabled, &p.AutoProvision, &p.UserTable, &p.AutoAddToGroup,
		&p.CreateTime, &p.CreateBy, &p.ChangeTime, &p.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, ErrIdentityProviderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get identity provider: %w", err)
	}
	if orgID.Valid {
		v := uint(orgID.Int64)
		p.OrgID = &v
	}
	if claimGroups.Valid {
		p.UserClaimGroups = claimGroups.String
	}
	return &p, nil
}

// GetProvidersByOrg returns all providers for a specific org (including global).
func (r *IdentityProviderRepository) GetProvidersByOrg(orgID uint) ([]*models.IdentityProvider, error) {
	query := database.ConvertPlaceholders(
		`SELECT id, org_id, name, provider_type, client_id, discovery_url, scopes,
		 user_claim_email, user_claim_name, user_claim_groups,
		 enabled, auto_provision, user_table, auto_add_to_group,
		 create_time, create_by, change_time, change_by
		FROM gk_identity_provider
		WHERE (org_id = ? OR org_id IS NULL) AND enabled = 1
		ORDER BY org_id DESC, name`,
	)
	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list identity providers: %w", err)
	}
	defer rows.Close()
	return scanProviders(rows)
}

// GetGlobalProviders returns all global (org_id NULL) providers.
func (r *IdentityProviderRepository) GetGlobalProviders() ([]*models.IdentityProvider, error) {
	query := database.ConvertPlaceholders(
		`SELECT id, org_id, name, provider_type, client_id, discovery_url, scopes,
		 user_claim_email, user_claim_name, user_claim_groups,
		 enabled, auto_provision, user_table, auto_add_to_group,
		 create_time, create_by, change_time, change_by
		FROM gk_identity_provider
		WHERE org_id IS NULL AND enabled = 1
		ORDER BY name`,
	)
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list global identity providers: %w", err)
	}
	defer rows.Close()
	return scanProviders(rows)
}

// UpdateProvider updates an existing identity provider.
func (r *IdentityProviderRepository) UpdateProvider(p *models.IdentityProvider) error {
	query := database.ConvertPlaceholders(
		`UPDATE gk_identity_provider SET
		 name = ?, provider_type = ?, client_id = ?, client_secret = ?, discovery_url = ?, scopes = ?,
		 user_claim_email = ?, user_claim_name = ?, user_claim_groups = ?,
		 enabled = ?, auto_provision = ?, user_table = ?, auto_add_to_group = ?,
		 change_time = ?, change_by = ?
		WHERE id = ?`,
	)
	_, err := r.db.Exec(
		query,
		p.Name, p.ProviderType, p.ClientID, p.ClientSecret, p.DiscoveryURL, p.Scopes,
		p.UserClaimEmail, p.UserClaimName, p.UserClaimGroups,
		p.Enabled, p.AutoProvision, p.UserTable, p.AutoAddToGroup,
		p.ChangeTime, p.ChangeBy, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update identity provider: %w", err)
	}
	return nil
}

// DeleteProvider deletes an identity provider by ID.
func (r *IdentityProviderRepository) DeleteProvider(id uint) error {
	query := database.ConvertPlaceholders(`DELETE FROM gk_identity_provider WHERE id = ?`)
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete identity provider: %w", err)
	}
	return nil
}

// GetProviderByOrgAndType returns the best matching provider for an org and type.
// Prefers org-scoped, falls back to global.
func (r *IdentityProviderRepository) GetProviderByOrgAndType(orgID uint, providerType string) (*models.IdentityProvider, error) {
	query := database.ConvertPlaceholders(
		`SELECT id, org_id, name, provider_type, client_id, client_secret, discovery_url, scopes,
		 user_claim_email, user_claim_name, user_claim_groups,
		 enabled, auto_provision, user_table, auto_add_to_group,
		 create_time, create_by, change_time, change_by
		FROM gk_identity_provider
		WHERE (org_id = ? OR org_id IS NULL) AND provider_type = ? AND enabled = 1
		ORDER BY org_id DESC
		LIMIT 1`,
	)
	var p models.IdentityProvider
	var orgIDNull sql.NullInt64
	var claimGroups sql.NullString
	err := r.db.QueryRow(query, orgID, providerType).Scan(
		&p.ID, &orgIDNull, &p.Name, &p.ProviderType, &p.ClientID, &p.ClientSecret,
		&p.DiscoveryURL, &p.Scopes,
		&p.UserClaimEmail, &p.UserClaimName,
		&claimGroups, &p.Enabled, &p.AutoProvision, &p.UserTable, &p.AutoAddToGroup,
		&p.CreateTime, &p.CreateBy, &p.ChangeTime, &p.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get identity provider by org and type: %w", err)
	}
	if orgIDNull.Valid {
		v := uint(orgIDNull.Int64)
		p.OrgID = &v
	}
	if claimGroups.Valid {
		p.UserClaimGroups = claimGroups.String
	}
	return &p, nil
}

// ListProviders returns all providers (for admin list).
func (r *IdentityProviderRepository) ListProviders() ([]*models.IdentityProvider, error) {
	query := database.ConvertPlaceholders(
		`SELECT id, org_id, name, provider_type, client_id, discovery_url, scopes,
		 user_claim_email, user_claim_name, user_claim_groups,
		 enabled, auto_provision, user_table, auto_add_to_group,
		 create_time, create_by, change_time, change_by
		FROM gk_identity_provider
		ORDER BY name`,
	)
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list all identity providers: %w", err)
	}
	defer rows.Close()
	return scanProviders(rows)
}

func scanProviders(rows *sql.Rows) ([]*models.IdentityProvider, error) {
	var providers []*models.IdentityProvider
	for rows.Next() {
		var p models.IdentityProvider
		var orgID sql.NullInt64
		var claimGroups sql.NullString
		err := rows.Scan(
			&p.ID, &orgID, &p.Name, &p.ProviderType, &p.ClientID,
			&p.DiscoveryURL, &p.Scopes, &p.UserClaimEmail, &p.UserClaimName,
			&claimGroups, &p.Enabled, &p.AutoProvision, &p.UserTable, &p.AutoAddToGroup,
			&p.CreateTime, &p.CreateBy, &p.ChangeTime, &p.ChangeBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan identity provider: %w", err)
		}
		if orgID.Valid {
			v := uint(orgID.Int64)
			p.OrgID = &v
		}
		if claimGroups.Valid {
			p.UserClaimGroups = claimGroups.String
		}
		providers = append(providers, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan identity providers: %w", err)
	}
	return providers, nil
}
