package api

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// getDynamicFieldsWithDB retrieves dynamic fields with optional filters
func getDynamicFieldsWithDB(db *sql.DB, objectType, fieldType string) ([]DynamicField, error) {
	query := `
		SELECT id, internal_field, name, label, field_order,
		       field_type, object_type, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM dynamic_field
	`

	var args []interface{}
	var conditions []string

	if objectType != "" {
		conditions = append(conditions, "object_type = ?")
		args = append(args, objectType)
	}

	if fieldType != "" {
		conditions = append(conditions, "field_type = ?")
		args = append(args, fieldType)
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	query += " ORDER BY object_type, field_order, name"
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query dynamic fields: %w", err)
	}
	defer rows.Close()

	return scanDynamicFields(rows)
}

// GetDynamicFields retrieves all dynamic fields
func GetDynamicFields(objectType, fieldType string) ([]DynamicField, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getDynamicFieldsWithDB(db, objectType, fieldType)
}

// getDynamicFieldWithDB retrieves a single dynamic field by ID
func getDynamicFieldWithDB(db *sql.DB, id int) (*DynamicField, error) {
	query := `
		SELECT id, internal_field, name, label, field_order,
		       field_type, object_type, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM dynamic_field
		WHERE id = ?
	`
	query = database.ConvertPlaceholders(query)

	row := db.QueryRow(query, id)
	return scanDynamicField(row)
}

// GetDynamicField retrieves a single dynamic field by ID
func GetDynamicField(id int) (*DynamicField, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getDynamicFieldWithDB(db, id)
}

// getDynamicFieldByNameWithDB retrieves a single dynamic field by name
func getDynamicFieldByNameWithDB(db *sql.DB, name string) (*DynamicField, error) {
	query := `
		SELECT id, internal_field, name, label, field_order,
		       field_type, object_type, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM dynamic_field
		WHERE name = ?
	`
	query = database.ConvertPlaceholders(query)

	row := db.QueryRow(query, name)
	return scanDynamicField(row)
}

// GetDynamicFieldByName retrieves a single dynamic field by name
func GetDynamicFieldByName(name string) (*DynamicField, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getDynamicFieldByNameWithDB(db, name)
}

// createDynamicFieldWithDB creates a new dynamic field
func createDynamicFieldWithDB(db *sql.DB, field *DynamicField, userID int) (int64, error) {
	if err := field.SerializeConfig(); err != nil {
		return 0, fmt.Errorf("failed to serialize config: %w", err)
	}

	now := time.Now()
	query := `
		INSERT INTO dynamic_field (
			internal_field, name, label, field_order,
			field_type, object_type, config, valid_id,
			create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	query = database.ConvertPlaceholders(query)

	result, err := db.Exec(query,
		field.InternalField,
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
		return 0, fmt.Errorf("failed to create dynamic field: %w", err)
	}

	return result.LastInsertId()
}

// CreateDynamicField creates a new dynamic field
func CreateDynamicField(field *DynamicField, userID int) (int64, error) {
	db, err := database.GetDB()
	if err != nil {
		return 0, err
	}
	return createDynamicFieldWithDB(db, field, userID)
}

// updateDynamicFieldWithDB updates an existing dynamic field
func updateDynamicFieldWithDB(db *sql.DB, field *DynamicField, userID int) error {
	if err := field.SerializeConfig(); err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	now := time.Now()
	query := `
		UPDATE dynamic_field SET
			name = ?,
			label = ?,
			field_order = ?,
			field_type = ?,
			object_type = ?,
			config = ?,
			valid_id = ?,
			change_time = ?,
			change_by = ?
		WHERE id = ?
	`
	query = database.ConvertPlaceholders(query)

	_, err := db.Exec(query,
		field.Name,
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
		return fmt.Errorf("failed to update dynamic field: %w", err)
	}

	return nil
}

// UpdateDynamicField updates an existing dynamic field
func UpdateDynamicField(field *DynamicField, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return updateDynamicFieldWithDB(db, field, userID)
}

// deleteDynamicFieldWithDB deletes a dynamic field and its values
func deleteDynamicFieldWithDB(db *sql.DB, id int) error {
	// First delete all values for this field
	valueQuery := database.ConvertPlaceholders("DELETE FROM dynamic_field_value WHERE field_id = ?")
	_, err := db.Exec(valueQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete dynamic field values: %w", err)
	}

	// Then delete the field itself
	fieldQuery := database.ConvertPlaceholders("DELETE FROM dynamic_field WHERE id = ?")
	_, err = db.Exec(fieldQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete dynamic field: %w", err)
	}

	return nil
}

// DeleteDynamicField deletes a dynamic field and its values
func DeleteDynamicField(id int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return deleteDynamicFieldWithDB(db, id)
}

// checkDynamicFieldNameExistsWithDB checks if a field name already exists
func checkDynamicFieldNameExistsWithDB(db *sql.DB, name string, excludeID int) (bool, error) {
	query := `
		SELECT COUNT(*) FROM dynamic_field 
		WHERE name = ? AND id != ?
	`
	query = database.ConvertPlaceholders(query)

	var count int
	err := db.QueryRow(query, name, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check field name existence: %w", err)
	}

	return count > 0, nil
}

// CheckDynamicFieldNameExists checks if a field name already exists
func CheckDynamicFieldNameExists(name string, excludeID int) (bool, error) {
	db, err := database.GetDB()
	if err != nil {
		return false, err
	}
	return checkDynamicFieldNameExistsWithDB(db, name, excludeID)
}

// Value operations

// getDynamicFieldValuesWithDB retrieves all values for an object
func getDynamicFieldValuesWithDB(db *sql.DB, objectID int64) ([]DynamicFieldValue, error) {
	query := `
		SELECT id, field_id, object_id, value_text, value_date, value_int
		FROM dynamic_field_value
		WHERE object_id = ?
		ORDER BY field_id
	`
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, objectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query dynamic field values: %w", err)
	}
	defer rows.Close()

	var values []DynamicFieldValue
	for rows.Next() {
		var v DynamicFieldValue
		err := rows.Scan(&v.ID, &v.FieldID, &v.ObjectID, &v.ValueText, &v.ValueDate, &v.ValueInt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dynamic field value: %w", err)
		}
		values = append(values, v)
	}

	return values, nil
}

// GetDynamicFieldValues retrieves all values for an object
func GetDynamicFieldValues(objectID int64) ([]DynamicFieldValue, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return getDynamicFieldValuesWithDB(db, objectID)
}

// setDynamicFieldValueWithDB sets a value for a dynamic field on an object
func setDynamicFieldValueWithDB(db *sql.DB, value *DynamicFieldValue) error {
	// Delete existing value first
	delQuery := database.ConvertPlaceholders("DELETE FROM dynamic_field_value WHERE field_id = ? AND object_id = ?")
	_, err := db.Exec(delQuery, value.FieldID, value.ObjectID)
	if err != nil {
		return fmt.Errorf("failed to delete existing value: %w", err)
	}

	// Insert new value (if any value is set)
	if value.ValueText != nil || value.ValueDate != nil || value.ValueInt != nil {
		insQuery := database.ConvertPlaceholders(`
			INSERT INTO dynamic_field_value (field_id, object_id, value_text, value_date, value_int)
			VALUES (?, ?, ?, ?, ?)
		`)
		_, err = db.Exec(insQuery, value.FieldID, value.ObjectID, value.ValueText, value.ValueDate, value.ValueInt)
		if err != nil {
			return fmt.Errorf("failed to insert dynamic field value: %w", err)
		}
	}

	return nil
}

// SetDynamicFieldValue sets a value for a dynamic field on an object
func SetDynamicFieldValue(value *DynamicFieldValue) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return setDynamicFieldValueWithDB(db, value)
}

// Helper functions

func scanDynamicFields(rows *sql.Rows) ([]DynamicField, error) {
	var fields []DynamicField
	for rows.Next() {
		var f DynamicField
		err := rows.Scan(
			&f.ID, &f.InternalField, &f.Name, &f.Label, &f.FieldOrder,
			&f.FieldType, &f.ObjectType, &f.ConfigRaw, &f.ValidID,
			&f.CreateTime, &f.CreateBy, &f.ChangeTime, &f.ChangeBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dynamic field: %w", err)
		}
		if err := f.ParseConfig(); err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, nil
}

func scanDynamicField(row *sql.Row) (*DynamicField, error) {
	var f DynamicField
	err := row.Scan(
		&f.ID, &f.InternalField, &f.Name, &f.Label, &f.FieldOrder,
		&f.FieldType, &f.ObjectType, &f.ConfigRaw, &f.ValidID,
		&f.CreateTime, &f.CreateBy, &f.ChangeTime, &f.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan dynamic field: %w", err)
	}
	if err := f.ParseConfig(); err != nil {
		return nil, err
	}
	return &f, nil
}

// GetDynamicFieldsGroupedByObjectType returns fields grouped by object type for admin display
func GetDynamicFieldsGroupedByObjectType() (map[string][]DynamicField, error) {
	fields, err := GetDynamicFields("", "")
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]DynamicField)
	for _, ot := range ValidObjectTypes() {
		grouped[ot] = []DynamicField{}
	}

	for _, f := range fields {
		grouped[f.ObjectType] = append(grouped[f.ObjectType], f)
	}

	return grouped, nil
}

// DynamicFieldDisplay represents a dynamic field with its value for display purposes
type DynamicFieldDisplay struct {
	Field        DynamicField
	Value        interface{} // The raw value (string, int, time, etc.)
	DisplayValue string      // Human-readable display value
}

// GetDynamicFieldValuesForDisplay returns dynamic field values formatted for display
// objectID is the ticket/article ID
// objectType is "Ticket" or "Article"
// screenKey is the screen (e.g., "AgentTicketZoom") - if empty, returns all enabled fields
func GetDynamicFieldValuesForDisplay(objectID int, objectType string, screenKey string) ([]DynamicFieldDisplay, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	// Get fields to display
	var fields []FieldWithScreenConfig
	if screenKey != "" {
		fields, err = getFieldsForScreenWithConfigWithDB(db, screenKey, objectType)
		if err != nil {
			return nil, err
		}
	} else {
		// Get all valid fields for the object type
		allFields, err := GetDynamicFields("", objectType)
		if err != nil {
			return nil, err
		}
		for _, f := range allFields {
			if f.ValidID == 1 {
				fields = append(fields, FieldWithScreenConfig{Field: f, ConfigValue: DFScreenEnabled})
			}
		}
	}

	// Get all values for this object in one query
	allValues, err := getDynamicFieldValuesWithDB(db, int64(objectID))
	if err != nil {
		return nil, err
	}

	// Build a map of fieldID -> value for quick lookup
	valueMap := make(map[int]*DynamicFieldValue)
	for i := range allValues {
		valueMap[allValues[i].FieldID] = &allValues[i]
	}

	var result []DynamicFieldDisplay
	for _, fwc := range fields {
		field := fwc.Field
		display := DynamicFieldDisplay{
			Field: field,
		}

		// Get the value for this field from the map
		dfValue, exists := valueMap[field.ID]
		if !exists || dfValue == nil {
			// No value set for this field
			display.DisplayValue = "-"
			result = append(result, display)
			continue
		}

		// Format the display value based on field type
		switch field.FieldType {
		case DFTypeText, DFTypeTextArea:
			if dfValue.ValueText != nil {
				display.Value = *dfValue.ValueText
				display.DisplayValue = *dfValue.ValueText
			} else {
				display.DisplayValue = "-"
			}
		case DFTypeDropdown:
			if dfValue.ValueText != nil {
				display.Value = *dfValue.ValueText
				// For dropdown, try to get the display name from possible values
				display.DisplayValue = *dfValue.ValueText
				if field.Config != nil && field.Config.PossibleValues != nil {
					if name, ok := field.Config.PossibleValues[*dfValue.ValueText]; ok {
						display.DisplayValue = name
					}
				}
			} else {
				display.DisplayValue = "-"
			}
		case DFTypeMultiselect:
			if dfValue.ValueText != nil {
				display.Value = *dfValue.ValueText
				// Split multiselect values and resolve display names
				keys := strings.Split(*dfValue.ValueText, "||")
				var displayNames []string
				for _, key := range keys {
					displayName := key
					if field.Config != nil && field.Config.PossibleValues != nil {
						if name, ok := field.Config.PossibleValues[key]; ok {
							displayName = name
						}
					}
					displayNames = append(displayNames, displayName)
				}
				display.DisplayValue = strings.Join(displayNames, ", ")
			} else {
				display.DisplayValue = "-"
			}
		case DFTypeCheckbox:
			if dfValue.ValueInt != nil {
				display.Value = *dfValue.ValueInt
				if *dfValue.ValueInt == 1 {
					display.DisplayValue = "Yes"
				} else {
					display.DisplayValue = "No"
				}
			} else {
				display.DisplayValue = "-"
			}
		case DFTypeDate:
			if dfValue.ValueDate != nil {
				display.Value = *dfValue.ValueDate
				display.DisplayValue = dfValue.ValueDate.Format("2006-01-02")
			} else {
				display.DisplayValue = "-"
			}
		case DFTypeDateTime:
			if dfValue.ValueDate != nil {
				display.Value = *dfValue.ValueDate
				display.DisplayValue = dfValue.ValueDate.Format("2006-01-02 15:04")
			} else {
				display.DisplayValue = "-"
			}
		default:
			if dfValue.ValueText != nil {
				display.Value = *dfValue.ValueText
				display.DisplayValue = *dfValue.ValueText
			} else {
				display.DisplayValue = "-"
			}
		}

		result = append(result, display)
	}

	return result, nil
}

// ProcessDynamicFieldsFromForm extracts and saves dynamic field values from form data
// formValues is a map of form field names to values (e.g., from c.Request.PostForm)
// objectID is the ID of the object the fields are being attached to (e.g., ticket ID)
// objectType should be "Ticket", "Article", etc.
// screenKey is the screen being used (e.g., "AgentTicketPhone")
func ProcessDynamicFieldsFromForm(formValues map[string][]string, objectID int, objectType, screenKey string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// Get fields enabled for this screen
	fields, err := getFieldsForScreenWithConfigWithDB(db, screenKey, objectType)
	if err != nil {
		return fmt.Errorf("failed to get screen fields: %w", err)
	}

	for _, fwc := range fields {
		field := fwc.Field
		formKey := "DynamicField_" + field.Name
		values, ok := formValues[formKey]

		// Handle multiselect fields (array values)
		if !ok {
			formKey = "DynamicField_" + field.Name + "[]"
			values, ok = formValues[formKey]
		}

		if !ok || len(values) == 0 {
			// Field not submitted - skip (or could clear existing value)
			continue
		}

		value := values[0] // For single-value fields
		if value == "" {
			continue // Skip empty values
		}

		dfValue := &DynamicFieldValue{
			FieldID:  field.ID,
			ObjectID: int64(objectID),
		}

		// Set appropriate value field based on field type
		switch field.FieldType {
		case DFTypeText, DFTypeTextArea, DFTypeDropdown:
			dfValue.ValueText = &value
		case DFTypeMultiselect:
			// Join multiple values with ||
			joined := strings.Join(values, "||")
			dfValue.ValueText = &joined
		case DFTypeCheckbox:
			var intVal int64 = 0
			if value == "1" || value == "on" || value == "true" {
				intVal = 1
			}
			dfValue.ValueInt = &intVal
		case DFTypeDate:
			// Parse date in YYYY-MM-DD format
			if t, err := time.Parse("2006-01-02", value); err == nil {
				dfValue.ValueDate = &t
			} else {
				dfValue.ValueText = &value // Fallback to text
			}
		case DFTypeDateTime:
			// Parse datetime-local format
			if t, err := time.Parse("2006-01-02T15:04", value); err == nil {
				dfValue.ValueDate = &t
			} else if t, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
				dfValue.ValueDate = &t
			} else {
				dfValue.ValueText = &value // Fallback to text
			}
		default:
			dfValue.ValueText = &value
		}

		if err := setDynamicFieldValueWithDB(db, dfValue); err != nil {
			return fmt.Errorf("failed to set value for field %s: %w", field.Name, err)
		}
	}

	return nil
}

// ProcessArticleDynamicFieldsFromForm extracts and saves Article dynamic field values
// Similar to ProcessDynamicFieldsFromForm but uses "ArticleDynamicField_" prefix
func ProcessArticleDynamicFieldsFromForm(formValues map[string][]string, articleID int, screenKey string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	fields, err := getFieldsForScreenWithConfigWithDB(db, screenKey, DFObjectArticle)
	if err != nil {
		return fmt.Errorf("failed to get screen fields: %w", err)
	}

	for _, fwc := range fields {
		field := fwc.Field
		formKey := "ArticleDynamicField_" + field.Name
		values, ok := formValues[formKey]

		if !ok {
			formKey = "ArticleDynamicField_" + field.Name + "[]"
			values, ok = formValues[formKey]
		}

		if !ok || len(values) == 0 {
			continue
		}

		value := values[0]
		if value == "" {
			continue
		}

		dfValue := &DynamicFieldValue{
			FieldID:  field.ID,
			ObjectID: int64(articleID),
		}

		switch field.FieldType {
		case DFTypeText, DFTypeTextArea, DFTypeDropdown:
			dfValue.ValueText = &value
		case DFTypeMultiselect:
			joined := strings.Join(values, "||")
			dfValue.ValueText = &joined
		case DFTypeCheckbox:
			var intVal int64 = 0
			if value == "1" || value == "on" || value == "true" {
				intVal = 1
			}
			dfValue.ValueInt = &intVal
		case DFTypeDate:
			if t, err := time.Parse("2006-01-02", value); err == nil {
				dfValue.ValueDate = &t
			} else {
				dfValue.ValueText = &value
			}
		case DFTypeDateTime:
			if t, err := time.Parse("2006-01-02T15:04", value); err == nil {
				dfValue.ValueDate = &t
			} else if t, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
				dfValue.ValueDate = &t
			} else {
				dfValue.ValueText = &value
			}
		default:
			dfValue.ValueText = &value
		}

		if err := setDynamicFieldValueWithDB(db, dfValue); err != nil {
			return fmt.Errorf("failed to set value for field %s: %w", field.Name, err)
		}
	}

	return nil
}
