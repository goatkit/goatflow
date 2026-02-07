package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
)

// ACLRepository handles database operations for ACLs.
type ACLRepository struct {
	db *sql.DB
}

// NewACLRepository creates a new ACL repository.
func NewACLRepository(db *sql.DB) *ACLRepository {
	return &ACLRepository{db: db}
}

// GetValidACLs returns all valid (enabled) ACLs ordered by name.
func (r *ACLRepository) GetValidACLs(ctx context.Context) ([]*models.ACL, error) {
	return r.getACLsWithFilter(ctx, true)
}

// GetAllACLs returns all ACLs regardless of validity.
func (r *ACLRepository) GetAllACLs(ctx context.Context) ([]*models.ACL, error) {
	return r.getACLsWithFilter(ctx, false)
}

// getACLsWithFilter retrieves ACLs with optional validity filter.
func (r *ACLRepository) getACLsWithFilter(ctx context.Context, validOnly bool) ([]*models.ACL, error) {
	query := database.ConvertPlaceholders(`
		SELECT
			id, name, comments, description, valid_id,
			stop_after_match, config_match, config_change,
			create_time, create_by, change_time, change_by
		FROM acl
	`)

	if validOnly {
		query = database.ConvertPlaceholders(`
			SELECT
				id, name, comments, description, valid_id,
				stop_after_match, config_match, config_change,
				create_time, create_by, change_time, change_by
			FROM acl
			WHERE valid_id = 1
			ORDER BY name
		`)
	} else {
		query = database.ConvertPlaceholders(`
			SELECT
				id, name, comments, description, valid_id,
				stop_after_match, config_match, config_change,
				create_time, create_by, change_time, change_by
			FROM acl
			ORDER BY name
		`)
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var acls []*models.ACL
	for rows.Next() {
		acl, err := r.scanACL(rows)
		if err != nil {
			continue
		}
		acls = append(acls, acl)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return acls, nil
}

// GetACL returns a single ACL by ID.
func (r *ACLRepository) GetACL(ctx context.Context, id int) (*models.ACL, error) {
	query := database.ConvertPlaceholders(`
		SELECT
			id, name, comments, description, valid_id,
			stop_after_match, config_match, config_change,
			create_time, create_by, change_time, change_by
		FROM acl
		WHERE id = ?
	`)

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanACLRow(row)
}

// GetACLByName returns a single ACL by name.
func (r *ACLRepository) GetACLByName(ctx context.Context, name string) (*models.ACL, error) {
	query := database.ConvertPlaceholders(`
		SELECT
			id, name, comments, description, valid_id,
			stop_after_match, config_match, config_change,
			create_time, create_by, change_time, change_by
		FROM acl
		WHERE name = ?
	`)

	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanACLRow(row)
}

// scanACL scans a single ACL from rows.
func (r *ACLRepository) scanACL(rows *sql.Rows) (*models.ACL, error) {
	var acl models.ACL
	var comments, description sql.NullString
	var stopAfterMatch sql.NullInt32
	var configMatch, configChange []byte

	err := rows.Scan(
		&acl.ID, &acl.Name, &comments, &description, &acl.ValidID,
		&stopAfterMatch, &configMatch, &configChange,
		&acl.CreateTime, &acl.CreateBy, &acl.ChangeTime, &acl.ChangeBy,
	)
	if err != nil {
		return nil, err
	}

	if comments.Valid {
		acl.Comments = &comments.String
	}
	if description.Valid {
		acl.Description = &description.String
	}
	acl.StopAfterMatch = stopAfterMatch.Valid && stopAfterMatch.Int32 != 0

	// Parse config_match JSON
	if len(configMatch) > 0 {
		acl.ConfigMatch = parseConfigMatch(configMatch)
	}

	// Parse config_change JSON
	if len(configChange) > 0 {
		acl.ConfigChange = parseConfigChange(configChange)
	}

	return &acl, nil
}

// scanACLRow scans a single ACL from a row.
func (r *ACLRepository) scanACLRow(row *sql.Row) (*models.ACL, error) {
	var acl models.ACL
	var comments, description sql.NullString
	var stopAfterMatch sql.NullInt32
	var configMatch, configChange []byte

	err := row.Scan(
		&acl.ID, &acl.Name, &comments, &description, &acl.ValidID,
		&stopAfterMatch, &configMatch, &configChange,
		&acl.CreateTime, &acl.CreateBy, &acl.ChangeTime, &acl.ChangeBy,
	)
	if err != nil {
		return nil, err
	}

	if comments.Valid {
		acl.Comments = &comments.String
	}
	if description.Valid {
		acl.Description = &description.String
	}
	acl.StopAfterMatch = stopAfterMatch.Valid && stopAfterMatch.Int32 != 0

	// Parse config_match JSON
	if len(configMatch) > 0 {
		acl.ConfigMatch = parseConfigMatch(configMatch)
	}

	// Parse config_change JSON
	if len(configChange) > 0 {
		acl.ConfigChange = parseConfigChange(configChange)
	}

	return &acl, nil
}

// parseConfigMatch parses the config_match JSON blob into ACLConfigMatch.
// The JSON format uses map[string]interface{} in admin handlers, so we need
// to convert to the structured format for the evaluator.
func parseConfigMatch(data []byte) *models.ACLConfigMatch {
	// First try to unmarshal directly into the structured format
	var cm models.ACLConfigMatch
	if err := json.Unmarshal(data, &cm); err == nil {
		return &cm
	}

	// Fallback: parse as generic structure and convert
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	cm = models.ACLConfigMatch{
		Properties:         make(map[string]map[string][]string),
		PropertiesDatabase: make(map[string]map[string][]string),
	}

	if props, ok := raw["Properties"].(map[string]interface{}); ok {
		cm.Properties = convertToStringMap(props)
	}
	if propsDB, ok := raw["PropertiesDatabase"].(map[string]interface{}); ok {
		cm.PropertiesDatabase = convertToStringMap(propsDB)
	}

	return &cm
}

// parseConfigChange parses the config_change JSON blob into ACLConfigChange.
func parseConfigChange(data []byte) *models.ACLConfigChange {
	// First try to unmarshal directly into the structured format
	var cc models.ACLConfigChange
	if err := json.Unmarshal(data, &cc); err == nil {
		return &cc
	}

	// Fallback: parse as generic structure and convert
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	cc = models.ACLConfigChange{
		Possible:    make(map[string]map[string][]string),
		PossibleAdd: make(map[string]map[string][]string),
		PossibleNot: make(map[string]map[string][]string),
	}

	if possible, ok := raw["Possible"].(map[string]interface{}); ok {
		cc.Possible = convertToStringMap(possible)
	}
	if possibleAdd, ok := raw["PossibleAdd"].(map[string]interface{}); ok {
		cc.PossibleAdd = convertToStringMap(possibleAdd)
	}
	if possibleNot, ok := raw["PossibleNot"].(map[string]interface{}); ok {
		cc.PossibleNot = convertToStringMap(possibleNot)
	}

	return &cc
}

// convertToStringMap converts a generic map to the structured format.
// Handles: map[string]interface{} where values can be:
// - map[string]interface{} containing []interface{} of strings
// - []interface{} of strings
func convertToStringMap(raw map[string]interface{}) map[string]map[string][]string {
	result := make(map[string]map[string][]string)

	for category, value := range raw {
		switch v := value.(type) {
		case map[string]interface{}:
			result[category] = make(map[string][]string)
			for field, vals := range v {
				result[category][field] = toStringSlice(vals)
			}
		case []interface{}:
			// Direct array under category (e.g., Action: ["AgentTicketNote"])
			result[category] = map[string][]string{
				"": toStringSlice(v),
			}
		}
	}

	return result
}

// toStringSlice converts an interface{} to []string.
func toStringSlice(v interface{}) []string {
	switch arr := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return arr
	case string:
		return []string{arr}
	default:
		return nil
	}
}
