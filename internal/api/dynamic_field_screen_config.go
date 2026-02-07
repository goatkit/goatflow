package api

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// FieldWithScreenConfig pairs a field with its screen configuration value.
type FieldWithScreenConfig struct {
	Field       DynamicField
	ConfigValue int // 0=disabled, 1=enabled, 2=required
}

// ScreenConfigMatrix provides a full view of field-screen mappings for admin UI.
type ScreenConfigMatrix struct {
	ObjectType string
	Fields     []DynamicField
	Screens    []ScreenDefinition
	ConfigMap  map[int]map[string]int // fieldID -> screenKey -> configValue
}

// getScreenConfigForFieldWithDB gets all screen configs for a single field.
func getScreenConfigForFieldWithDB(db *sql.DB, fieldID int) ([]DynamicFieldScreenConfig, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, field_id, screen_key, config_value, create_time, create_by, change_time, change_by
		FROM dynamic_field_screen_config
		WHERE field_id = ?
		ORDER BY screen_key
	`)

	rows, err := db.Query(query, fieldID)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen configs: %w", err)
	}
	defer rows.Close()

	return scanScreenConfigs(rows)
}

// GetScreenConfigForField gets all screen configs for a single field.
func GetScreenConfigForField(fieldID int) ([]DynamicFieldScreenConfig, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getScreenConfigForFieldWithDB(db, fieldID)
}

// getScreenConfigForScreenWithDB gets all field configs for a specific screen.
func getScreenConfigForScreenWithDB(db *sql.DB, screenKey string) ([]DynamicFieldScreenConfig, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, field_id, screen_key, config_value, create_time, create_by, change_time, change_by
		FROM dynamic_field_screen_config
		WHERE screen_key = ?
		ORDER BY field_id
	`)

	rows, err := db.Query(query, screenKey)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen configs: %w", err)
	}
	defer rows.Close()

	return scanScreenConfigs(rows)
}

// GetScreenConfigForScreen gets all field configs for a specific screen.
func GetScreenConfigForScreen(screenKey string) ([]DynamicFieldScreenConfig, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getScreenConfigForScreenWithDB(db, screenKey)
}

// setScreenConfigWithDB sets a single field-screen config.
func setScreenConfigWithDB(db *sql.DB, fieldID int, screenKey string, configValue int, userID int) error {
	now := time.Now()

	// Always delete existing first
	delQuery := database.ConvertPlaceholders(`DELETE FROM dynamic_field_screen_config WHERE field_id = ? AND screen_key = ?`)
	_, err := db.Exec(delQuery, fieldID, screenKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing config: %w", err)
	}

	// Only insert if value is non-zero (enabled or required)
	if configValue > 0 {
		insQuery := database.ConvertPlaceholders(`
			INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		_, err = db.Exec(insQuery, fieldID, screenKey, configValue, now, userID, now, userID)
		if err != nil {
			return fmt.Errorf("failed to insert screen config: %w", err)
		}
	}

	return nil
}

// SetScreenConfig sets a single field-screen config.
func SetScreenConfig(fieldID int, screenKey string, configValue int, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return setScreenConfigWithDB(db, fieldID, screenKey, configValue, userID)
}

// bulkSetScreenConfigForFieldWithDB replaces all screen configs for a field.
func bulkSetScreenConfigForFieldWithDB(db *sql.DB, fieldID int, configs map[string]int, userID int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()

	// Delete all existing configs for this field
	delQuery := database.ConvertPlaceholders(`DELETE FROM dynamic_field_screen_config WHERE field_id = ?`)
	_, err = tx.Exec(delQuery, fieldID)
	if err != nil {
		return fmt.Errorf("failed to delete existing configs: %w", err)
	}

	// Insert new configs (only non-zero values)
	insQuery := database.ConvertPlaceholders(`
		INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)

	for screenKey, configValue := range configs {
		if configValue > 0 {
			_, err = tx.Exec(insQuery, fieldID, screenKey, configValue, now, userID, now, userID)
			if err != nil {
				return fmt.Errorf("failed to insert config for %s: %w", screenKey, err)
			}
		}
	}

	return tx.Commit()
}

// BulkSetScreenConfigForField replaces all screen configs for a field.
func BulkSetScreenConfigForField(fieldID int, configs map[string]int, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return bulkSetScreenConfigForFieldWithDB(db, fieldID, configs, userID)
}

// getFieldsForScreenWithConfigWithDB gets fields enabled for a specific screen with their config values.
func getFieldsForScreenWithConfigWithDB(db *sql.DB, screenKey, objectType string) ([]FieldWithScreenConfig, error) {
	query := database.ConvertPlaceholders(`
		SELECT df.id, df.internal_field, df.name, df.label, df.field_order,
		       df.field_type, df.object_type, df.config, df.valid_id,
		       df.create_time, df.create_by, df.change_time, df.change_by,
		       sc.config_value
		FROM dynamic_field df
		INNER JOIN dynamic_field_screen_config sc ON df.id = sc.field_id
		WHERE sc.screen_key = ? AND df.object_type = ? AND df.valid_id = 1
		ORDER BY df.field_order, df.name
	`)

	rows, err := db.Query(query, screenKey, objectType)
	if err != nil {
		return nil, fmt.Errorf("failed to query fields for screen: %w", err)
	}
	defer rows.Close()

	var results []FieldWithScreenConfig
	for rows.Next() {
		var f DynamicField
		var configValue int
		err := rows.Scan(
			&f.ID, &f.InternalField, &f.Name, &f.Label, &f.FieldOrder,
			&f.FieldType, &f.ObjectType, &f.ConfigRaw, &f.ValidID,
			&f.CreateTime, &f.CreateBy, &f.ChangeTime, &f.ChangeBy,
			&configValue,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan field: %w", err)
		}
		if err := f.ParseConfig(); err != nil {
			return nil, err
		}
		results = append(results, FieldWithScreenConfig{Field: f, ConfigValue: configValue})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fields: %w", err)
	}

	return results, nil
}

// GetFieldsForScreenWithConfig gets fields enabled for a specific screen.
func GetFieldsForScreenWithConfig(screenKey, objectType string) ([]FieldWithScreenConfig, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getFieldsForScreenWithConfigWithDB(db, screenKey, objectType)
}

// getScreenConfigMatrixWithDB builds a matrix of all fields and their screen configs.
func getScreenConfigMatrixWithDB(db *sql.DB, objectType string) (*ScreenConfigMatrix, error) {
	// Get all fields for object type
	fieldQuery := database.ConvertPlaceholders(`
		SELECT id, internal_field, name, label, field_order,
		       field_type, object_type, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM dynamic_field
		WHERE object_type = ?
		ORDER BY field_order, name
	`)

	fieldRows, err := db.Query(fieldQuery, objectType)
	if err != nil {
		return nil, fmt.Errorf("failed to query fields: %w", err)
	}
	defer fieldRows.Close()

	fields, err := scanDynamicFields(fieldRows)
	if err != nil {
		return nil, err
	}

	if len(fields) == 0 {
		return &ScreenConfigMatrix{
			ObjectType: objectType,
			Fields:     []DynamicField{},
			Screens:    GetScreenDefinitions(),
			ConfigMap:  make(map[int]map[string]int),
		}, nil
	}

	// Build field IDs for IN clause
	fieldIDs := make([]interface{}, len(fields))
	placeholders := make([]string, len(fields))
	for i, f := range fields {
		fieldIDs[i] = f.ID
		placeholders[i] = "?"
	}

	// Get all screen configs for these fields
	configQuery := database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT id, field_id, screen_key, config_value, create_time, create_by, change_time, change_by
		FROM dynamic_field_screen_config
		WHERE field_id IN (%s)
	`, strings.Join(placeholders, ",")))

	configRows, err := db.Query(configQuery, fieldIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen configs: %w", err)
	}
	defer configRows.Close()

	configs, err := scanScreenConfigs(configRows)
	if err != nil {
		return nil, err
	}

	// Build the config map
	configMap := make(map[int]map[string]int)
	for _, f := range fields {
		configMap[f.ID] = make(map[string]int)
	}
	for _, c := range configs {
		if _, ok := configMap[c.FieldID]; ok {
			configMap[c.FieldID][c.ScreenKey] = c.ConfigValue
		}
	}

	return &ScreenConfigMatrix{
		ObjectType: objectType,
		Fields:     fields,
		Screens:    GetScreenDefinitions(),
		ConfigMap:  configMap,
	}, nil
}

// GetScreenConfigMatrix builds a matrix of all fields and their screen configs.
func GetScreenConfigMatrix(objectType string) (*ScreenConfigMatrix, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getScreenConfigMatrixWithDB(db, objectType)
}

// Returns []interface{} to avoid import cycles.
func GetDynamicFieldsForScreenGeneric(screenKey, objectType string) ([]interface{}, error) {
	fields, err := GetFieldsForScreenWithConfig(screenKey, objectType)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, len(fields))
	for i, f := range fields {
		result[i] = f
	}
	return result, nil
}

// Helper functions

func scanScreenConfigs(rows *sql.Rows) ([]DynamicFieldScreenConfig, error) {
	var configs []DynamicFieldScreenConfig
	for rows.Next() {
		var c DynamicFieldScreenConfig
		err := rows.Scan(&c.ID, &c.FieldID, &c.ScreenKey, &c.ConfigValue,
			&c.CreateTime, &c.CreateBy, &c.ChangeTime, &c.ChangeBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan screen config: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, nil
}
