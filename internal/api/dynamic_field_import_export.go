package api

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// DynamicFieldExport represents the full export structure (Znuny-compatible format).
type DynamicFieldExport struct {
	DynamicFields       map[string]DynamicFieldExportItem `yaml:"DynamicFields"`
	DynamicFieldScreens map[string]map[string]int         `yaml:"DynamicFieldScreens,omitempty"`
}

// DynamicFieldExportItem represents a single field for export.
type DynamicFieldExportItem struct {
	Name       string              `yaml:"Name"`
	Label      string              `yaml:"Label"`
	FieldType  string              `yaml:"FieldType"`
	ObjectType string              `yaml:"ObjectType"`
	FieldOrder int                 `yaml:"FieldOrder"`
	ValidID    int                 `yaml:"ValidID"`
	Config     *DynamicFieldConfig `yaml:"Config,omitempty"`
}

// ImportResult contains the results of an import operation.
type ImportResult struct {
	Created   []string `json:"created"`
	Updated   []string `json:"updated"`
	Skipped   []string `json:"skipped"`
	Errors    []string `json:"errors"`
	ScreensOK int      `json:"screens_ok"`
}

// ImportPreviewItem represents a field in the import preview.
type ImportPreviewItem struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	FieldType    string `json:"field_type"`
	ObjectType   string `json:"object_type"`
	Exists       bool   `json:"exists"`
	HasScreens   bool   `json:"has_screens"`
	ScreenCount  int    `json:"screen_count"`
	WillCreate   bool   `json:"will_create"`
	WillOverwrite bool  `json:"will_overwrite"`
}

// ExportDynamicFields exports the specified dynamic fields to a Znuny-compatible format.
func ExportDynamicFields(fieldNames []string, includeScreens bool) (*DynamicFieldExport, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	// Get all fields if no specific names provided
	var fields []DynamicField
	if len(fieldNames) == 0 {
		fields, err = getDynamicFieldsWithDB(db, "", "")
		if err != nil {
			return nil, fmt.Errorf("failed to get dynamic fields: %w", err)
		}
	} else {
		// Get only the specified fields
		for _, name := range fieldNames {
			field, err := getDynamicFieldByNameWithDB(db, name)
			if err != nil {
				return nil, fmt.Errorf("failed to get field %s: %w", name, err)
			}
			if field != nil {
				fields = append(fields, *field)
			}
		}
	}

	export := &DynamicFieldExport{
		DynamicFields: make(map[string]DynamicFieldExportItem),
	}

	if includeScreens {
		export.DynamicFieldScreens = make(map[string]map[string]int)
	}

	for _, f := range fields {
		// Export field definition
		export.DynamicFields[f.Name] = DynamicFieldExportItem{
			Name:       f.Name,
			Label:      f.Label,
			FieldType:  f.FieldType,
			ObjectType: f.ObjectType,
			FieldOrder: f.FieldOrder,
			ValidID:    f.ValidID,
			Config:     f.Config,
		}

		// Export screen configurations if requested
		if includeScreens {
			screenConfigs, err := getScreenConfigForFieldWithDB(db, f.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get screen configs for %s: %w", f.Name, err)
			}

			if len(screenConfigs) > 0 {
				export.DynamicFieldScreens[f.Name] = make(map[string]int)
				for _, sc := range screenConfigs {
					export.DynamicFieldScreens[f.Name][sc.ScreenKey] = sc.ConfigValue
				}
			}
		}
	}

	return export, nil
}

// ExportDynamicFieldsYAML exports dynamic fields to YAML format.
func ExportDynamicFieldsYAML(fieldNames []string, includeScreens bool) ([]byte, error) {
	export, err := ExportDynamicFields(fieldNames, includeScreens)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(export)
}

// ParseDynamicFieldsYAML parses a YAML file into a DynamicFieldExport structure.
func ParseDynamicFieldsYAML(data []byte) (*DynamicFieldExport, error) {
	var export DynamicFieldExport
	if err := yaml.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if export.DynamicFields == nil {
		return nil, fmt.Errorf("invalid file: no DynamicFields section found")
	}

	return &export, nil
}

// GetImportPreview returns a preview of what will happen when importing.
func GetImportPreview(export *DynamicFieldExport) ([]ImportPreviewItem, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	var preview []ImportPreviewItem

	for name, item := range export.DynamicFields {
		existing, err := getDynamicFieldByNameWithDB(db, name)
		if err != nil {
			return nil, fmt.Errorf("failed to check field %s: %w", name, err)
		}

		screenCount := 0
		hasScreens := false
		if export.DynamicFieldScreens != nil {
			if screens, ok := export.DynamicFieldScreens[name]; ok {
				screenCount = len(screens)
				hasScreens = screenCount > 0
			}
		}

		preview = append(preview, ImportPreviewItem{
			Name:          name,
			Label:         item.Label,
			FieldType:     item.FieldType,
			ObjectType:    item.ObjectType,
			Exists:        existing != nil,
			HasScreens:    hasScreens,
			ScreenCount:   screenCount,
			WillCreate:    existing == nil,
			WillOverwrite: existing != nil,
		})
	}

	return preview, nil
}

// ImportDynamicFields imports dynamic fields from an export structure.
func ImportDynamicFields(
	export *DynamicFieldExport,
	selectedFields []string,
	selectedScreens []string,
	overwrite bool,
	userID int,
) (*ImportResult, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	result := &ImportResult{
		Created: []string{},
		Updated: []string{},
		Skipped: []string{},
		Errors:  []string{},
	}

	// Build lookup maps for selected items
	selectedFieldMap := make(map[string]bool)
	for _, name := range selectedFields {
		selectedFieldMap[name] = true
	}

	selectedScreenMap := make(map[string]bool)
	for _, name := range selectedScreens {
		selectedScreenMap[name] = true
	}

	now := time.Now()

	// Import fields
	for name, item := range export.DynamicFields {
		// Skip if not selected
		if !selectedFieldMap[name] {
			continue
		}

		// Validate field type
		if !isValidFieldType(item.FieldType) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid field type %s", name, item.FieldType))
			continue
		}

		// Validate object type
		if !isValidObjectType(item.ObjectType) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid object type %s", name, item.ObjectType))
			continue
		}

		// Check if field exists
		existing, err := getDynamicFieldByNameWithDB(db, name)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		if existing != nil && !overwrite {
			result.Skipped = append(result.Skipped, name)
			continue
		}

		// Build field struct
		field := &DynamicField{
			Name:       item.Name,
			Label:      item.Label,
			FieldType:  item.FieldType,
			ObjectType: item.ObjectType,
			FieldOrder: item.FieldOrder,
			ValidID:    item.ValidID,
			Config:     item.Config,
		}

		if field.ValidID == 0 {
			field.ValidID = 1
		}
		if field.FieldOrder == 0 {
			field.FieldOrder = 1
		}

		if existing != nil {
			// Update existing field
			field.ID = existing.ID
			field.InternalField = existing.InternalField

			if err := field.SerializeConfig(); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to serialize config: %v", name, err))
				continue
			}

			query := database.ConvertPlaceholders(`
				UPDATE dynamic_field SET
					label = ?,
					field_order = ?,
					field_type = ?,
					object_type = ?,
					config = ?,
					valid_id = ?,
					change_time = ?,
					change_by = ?
				WHERE id = ?
			`)

			_, err := db.Exec(query,
				field.Label,
				field.FieldOrder,
				field.FieldType,
				field.ObjectType,
				field.ConfigRaw,
				field.ValidID,
				now,
				userID,
				field.ID,
			)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to update: %v", name, err))
				continue
			}

			result.Updated = append(result.Updated, name)
		} else {
			// Create new field
			if err := field.SerializeConfig(); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to serialize config: %v", name, err))
				continue
			}

			query := database.ConvertPlaceholders(`
				INSERT INTO dynamic_field (
					internal_field, name, label, field_order,
					field_type, object_type, config, valid_id,
					create_time, create_by, change_time, change_by
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`)

			_, err := db.Exec(query,
				0, // internal_field
				field.Name,
				field.Label,
				field.FieldOrder,
				field.FieldType,
				field.ObjectType,
				field.ConfigRaw,
				field.ValidID,
				now,
				userID,
				now,
				userID,
			)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to create: %v", name, err))
				continue
			}

			result.Created = append(result.Created, name)
		}
	}

	// Import screen configurations
	if export.DynamicFieldScreens != nil {
		for name, screens := range export.DynamicFieldScreens {
			// Skip if not selected
			if !selectedScreenMap[name] {
				continue
			}

			// Get the field ID
			field, err := getDynamicFieldByNameWithDB(db, name)
			if err != nil || field == nil {
				continue // Field doesn't exist, skip screen config
			}

			// Set screen configs
			for screenKey, configValue := range screens {
				err := setScreenConfigWithDB(db, field.ID, screenKey, configValue, userID)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s screen %s: %v", name, screenKey, err))
					continue
				}
				result.ScreensOK++
			}
		}
	}

	return result, nil
}
