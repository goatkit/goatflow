package customfields

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"gopkg.in/yaml.v3"
)

// Legacy OTRS object_type → GoatKit entity_type mapping.
var legacyEntityMap = map[string]string{
	"Ticket":          EntityTicket,
	"Article":         EntityArticle,
	"CustomerUser":    EntityContact,
	"CustomerCompany": EntityCustomerGroup,
}

// Legacy OTRS field_type → GoatKit field_type mapping.
var legacyFieldTypeMap = map[string]string{
	"Text":                  FieldText,
	"TextArea":              FieldTextArea,
	"Checkbox":              FieldBoolean,
	"Dropdown":              FieldSelect,
	"Multiselect":           FieldMultiSelect,
	"Date":                  FieldDate,
	"DateTime":              FieldDateTime,
	"WebserviceDropdown":    FieldSelect,
	"WebserviceMultiselect": FieldMultiSelect,
}

// Legacy value column → GoatKit value column mapping for each legacy field type.
var legacyValueColumn = map[string]string{
	"Text":                  "value_text",
	"TextArea":              "value_text",
	"Checkbox":              "value_int",
	"Dropdown":              "value_text",
	"Multiselect":           "value_text",
	"Date":                  "value_date",
	"DateTime":              "value_date",
	"WebserviceDropdown":    "value_text",
	"WebserviceMultiselect": "value_text",
}

// MigrateLegacyFields copies legacy dynamic_field definitions and values into the
// gk_custom_field_* tables. This is idempotent and safe to run on every startup.
// Legacy tables are never modified.
func MigrateLegacyFields(logger *slog.Logger) error {
	db, err := database.GetDB()
	if err != nil {
		return fmt.Errorf("legacy migration: get db: %w", err)
	}

	// Check if legacy table exists (might be a fresh install with no OTRS schema).
	if !tableExists(db, "dynamic_field") {
		logger.Debug("legacy migration: dynamic_field table not found, skipping")
		return nil
	}

	// Check if target table exists (migration might not have run yet).
	if !tableExists(db, "gk_custom_field_def") {
		logger.Debug("legacy migration: gk_custom_field_def table not found, skipping")
		return nil
	}

	legacyFields, err := loadLegacyFields(db)
	if err != nil {
		return err
	}

	if len(legacyFields) == 0 {
		logger.Debug("legacy migration: no dynamic fields to migrate")
		return nil
	}

	repo := NewRepositoryWithDB(db)
	var migratedDefs, migratedVals int

	for _, lf := range legacyFields {
		entityType, ok := legacyEntityMap[lf.ObjectType]
		if !ok {
			logger.Warn("legacy migration: unknown object_type, skipping", "object_type", lf.ObjectType, "field", lf.Name)
			continue
		}

		newFieldType, ok := legacyFieldTypeMap[lf.FieldType]
		if !ok {
			logger.Warn("legacy migration: unknown field_type, skipping", "field_type", lf.FieldType, "field", lf.Name)
			continue
		}

		// Convert name to lowercase with underscores (legacy uses PascalCase).
		newName := toLowerSnake(lf.Name)

		// Check if already migrated.
		existing, err := repo.GetDefByEntityAndName(entityType, newName)
		if err != nil {
			return fmt.Errorf("legacy migration: check existing %q: %w", newName, err)
		}
		if existing != nil {
			continue // Already migrated.
		}

		// Convert legacy YAML config to new JSON config.
		newConfig, err := convertLegacyConfig(lf.FieldType, lf.ConfigRaw)
		if err != nil {
			logger.Warn("legacy migration: config conversion failed, using empty config", "field", lf.Name, "error", err)
			newConfig = nil
		}

		legacyID := int64(lf.ID)
		def := &FieldDef{
			Name:         newName,
			Label:        lf.Label,
			EntityType:   entityType,
			FieldType:    newFieldType,
			OwnerType:    OwnerLegacy,
			MigratedFrom: &legacyID,
			Section:      "legacy",
			FieldOrder:   lf.FieldOrder,
			Required:     false,
			Config:       newConfig,
			ValidID:      lf.ValidID,
		}

		newID, err := repo.CreateDef(def, 1) // userID=1 (system)
		if err != nil {
			return fmt.Errorf("legacy migration: create def %q: %w", newName, err)
		}
		migratedDefs++

		// Copy values.
		copied, err := copyLegacyValues(db, lf, newID, newFieldType)
		if err != nil {
			return fmt.Errorf("legacy migration: copy values for %q: %w", newName, err)
		}
		migratedVals += copied
	}

	if migratedDefs > 0 {
		logger.Info("legacy migration complete", "definitions", migratedDefs, "values", migratedVals)
	} else {
		logger.Debug("legacy migration: all fields already migrated")
	}

	return nil
}

// legacyField is a minimal representation of a dynamic_field row.
type legacyField struct {
	ID         int
	Name       string
	Label      string
	FieldOrder int
	FieldType  string
	ObjectType string
	ConfigRaw  []byte
	ValidID    int
}

func loadLegacyFields(db *sql.DB) ([]legacyField, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, label, field_order, field_type, object_type, config, valid_id
		FROM dynamic_field
		ORDER BY id
	`)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("legacy migration: query dynamic_field: %w", err)
	}
	defer rows.Close()

	var fields []legacyField
	for rows.Next() {
		var f legacyField
		if err := rows.Scan(&f.ID, &f.Name, &f.Label, &f.FieldOrder, &f.FieldType, &f.ObjectType, &f.ConfigRaw, &f.ValidID); err != nil {
			return nil, fmt.Errorf("legacy migration: scan: %w", err)
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

func copyLegacyValues(db *sql.DB, lf legacyField, newFieldID int64, newFieldType string) (int, error) {
	srcCol := legacyValueColumn[lf.FieldType]
	if srcCol == "" {
		srcCol = "value_text"
	}

	// Read legacy values.
	query := database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT object_id, %s FROM dynamic_field_value WHERE field_id = ?
	`, srcCol))

	rows, err := db.Query(query, lf.ID)
	if err != nil {
		return 0, fmt.Errorf("query legacy values: %w", err)
	}
	defer rows.Close()

	insQuery := database.ConvertPlaceholders(`
		INSERT INTO gk_custom_field_value
			(field_id, object_id, val_text, val_int, val_decimal, val_date, val_datetime, val_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)

	count := 0
	for rows.Next() {
		var objectID int64
		var rawVal any

		switch srcCol {
		case "value_text":
			var v *string
			if err := rows.Scan(&objectID, &v); err != nil {
				return count, err
			}
			if v == nil {
				continue
			}
			rawVal = v
		case "value_int":
			var v *int64
			if err := rows.Scan(&objectID, &v); err != nil {
				return count, err
			}
			if v == nil {
				continue
			}
			rawVal = v
		case "value_date":
			var v *time.Time
			if err := rows.Scan(&objectID, &v); err != nil {
				return count, err
			}
			if v == nil {
				continue
			}
			rawVal = v
		}

		// Map legacy value to new columns.
		var valText *string
		var valInt *int64
		var valDecimal *float64
		var valDate *time.Time
		var valDatetime *time.Time
		var valJSON *json.RawMessage

		switch newFieldType {
		case FieldText, FieldTextArea, FieldSelect, FieldURL, FieldEmail, FieldPhone:
			if s, ok := rawVal.(*string); ok {
				valText = s
			}
		case FieldBoolean, FieldInteger:
			if n, ok := rawVal.(*int64); ok {
				valInt = n
			}
		case FieldDecimal:
			if n, ok := rawVal.(*int64); ok {
				f := float64(*n)
				valDecimal = &f
			}
		case FieldDate:
			if t, ok := rawVal.(*time.Time); ok {
				valDate = t
			}
		case FieldDateTime:
			if t, ok := rawVal.(*time.Time); ok {
				valDatetime = t
			}
		case FieldMultiSelect:
			// Legacy multiselect uses "||" separator in value_text.
			if s, ok := rawVal.(*string); ok && s != nil {
				parts := strings.Split(*s, "||")
				raw, _ := json.Marshal(parts)
				jrm := json.RawMessage(raw)
				valJSON = &jrm
			}
		}

		_, err := db.Exec(insQuery, newFieldID, objectID, valText, valInt, valDecimal, valDate, valDatetime, valJSON)
		if err != nil {
			return count, fmt.Errorf("insert migrated value: %w", err)
		}
		count++
	}

	return count, rows.Err()
}

// convertLegacyConfig converts OTRS YAML config to GoatKit JSON config.
func convertLegacyConfig(legacyFieldType string, yamlBytes []byte) (*json.RawMessage, error) {
	if len(yamlBytes) == 0 {
		return nil, nil
	}

	var legacy map[string]any
	if err := yaml.Unmarshal(yamlBytes, &legacy); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	cfg := &FieldConfig{}

	switch legacyFieldType {
	case "Text":
		if v, ok := legacy["MaxLength"].(int); ok {
			cfg.MaxLength = &v
		}
	case "TextArea":
		// No specific config to migrate.
	case "Dropdown", "WebserviceDropdown":
		if pv, ok := legacy["PossibleValues"].(map[string]any); ok {
			for k, v := range pv {
				label := fmt.Sprintf("%v", v)
				cfg.Options = append(cfg.Options, SelectOption{Value: k, Label: label})
			}
		}
	case "Multiselect", "WebserviceMultiselect":
		if pv, ok := legacy["PossibleValues"].(map[string]any); ok {
			for k, v := range pv {
				label := fmt.Sprintf("%v", v)
				cfg.Options = append(cfg.Options, SelectOption{Value: k, Label: label})
			}
		}
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	// Don't store empty config.
	if string(raw) == "{}" {
		return nil, nil
	}

	jrm := json.RawMessage(raw)
	return &jrm, nil
}

// toLowerSnake converts PascalCase/camelCase to lower_snake_case.
func toLowerSnake(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+'a'-'A'))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// tableExists checks if a table exists in the database.
func tableExists(db *sql.DB, tableName string) bool {
	query := database.ConvertPlaceholders(
		"SELECT 1 FROM information_schema.tables WHERE table_name = ? LIMIT 1",
	)
	var dummy int
	err := db.QueryRow(query, tableName).Scan(&dummy)
	return err == nil
}
