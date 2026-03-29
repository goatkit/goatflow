package customfields

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// Repository provides CRUD operations for custom field definitions and values.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new custom fields repository using the global DB connection.
func NewRepository() (*Repository, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// NewRepositoryWithDB creates a new custom fields repository with an explicit DB connection.
func NewRepositoryWithDB(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- Definition CRUD ---

// ListDefs retrieves field definitions with optional filters.
func (r *Repository) ListDefs(entityType, ownerType, ownerName string, activeOnly bool) ([]FieldDef, error) {
	query := `
		SELECT id, name, label, entity_type, field_type,
		       owner_type, owner_name, migrated_from,
		       section, field_order, description, placeholder,
		       required, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM gk_custom_field_def
	`
	var args []any
	var conditions []string

	if entityType != "" {
		conditions = append(conditions, "entity_type = ?")
		args = append(args, entityType)
	}
	if ownerType != "" {
		conditions = append(conditions, "owner_type = ?")
		args = append(args, ownerType)
	}
	if ownerName != "" {
		conditions = append(conditions, "owner_name = ?")
		args = append(args, ownerName)
	}
	if activeOnly {
		conditions = append(conditions, "valid_id = 1")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY entity_type, section, field_order, name"
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list custom field defs: %w", err)
	}
	defer rows.Close()

	return scanFieldDefs(rows)
}

// GetDef retrieves a single field definition by ID.
func (r *Repository) GetDef(id int64) (*FieldDef, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, label, entity_type, field_type,
		       owner_type, owner_name, migrated_from,
		       section, field_order, description, placeholder,
		       required, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM gk_custom_field_def WHERE id = ?
	`)

	row := r.db.QueryRow(query, id)
	return scanFieldDef(row)
}

// GetDefByEntityAndName retrieves a field definition by entity type and name.
func (r *Repository) GetDefByEntityAndName(entityType, name string) (*FieldDef, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, label, entity_type, field_type,
		       owner_type, owner_name, migrated_from,
		       section, field_order, description, placeholder,
		       required, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM gk_custom_field_def WHERE entity_type = ? AND name = ?
	`)

	row := r.db.QueryRow(query, entityType, name)
	return scanFieldDef(row)
}

// CreateDef inserts a new field definition. Returns the new row ID.
func (r *Repository) CreateDef(f *FieldDef, userID int) (int64, error) {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_custom_field_def (
			name, label, entity_type, field_type,
			owner_type, owner_name, migrated_from,
			section, field_order, description, placeholder,
			required, config, valid_id,
			create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	result, err := r.db.Exec(query,
		f.Name, f.Label, f.EntityType, f.FieldType,
		f.OwnerType, f.OwnerName, f.MigratedFrom,
		f.Section, f.FieldOrder, f.Description, f.Placeholder,
		f.Required, f.Config, f.ValidID,
		now, userID, now, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("create custom field def: %w", err)
	}
	return result.LastInsertId()
}

// UpdateDef updates an existing field definition.
func (r *Repository) UpdateDef(f *FieldDef, userID int) error {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		UPDATE gk_custom_field_def SET
			label = ?, section = ?, field_order = ?,
			description = ?, placeholder = ?,
			required = ?, config = ?, valid_id = ?,
			change_time = ?, change_by = ?
		WHERE id = ?
	`)

	_, err := r.db.Exec(query,
		f.Label, f.Section, f.FieldOrder,
		f.Description, f.Placeholder,
		f.Required, f.Config, f.ValidID,
		now, userID,
		f.ID,
	)
	if err != nil {
		return fmt.Errorf("update custom field def: %w", err)
	}
	return nil
}

// SoftDeleteDef marks a field as invalid (valid_id=2). Data is preserved.
func (r *Repository) SoftDeleteDef(id int64, userID int) error {
	now := time.Now()
	query := database.ConvertPlaceholders(`
		UPDATE gk_custom_field_def SET valid_id = 2, change_time = ?, change_by = ? WHERE id = ?
	`)
	_, err := r.db.Exec(query, now, userID, id)
	if err != nil {
		return fmt.Errorf("soft delete custom field def: %w", err)
	}
	return nil
}

// HardDeleteDef removes a field definition and all its values permanently.
func (r *Repository) HardDeleteDef(id int64) error {
	// CASCADE on FK handles values, but be explicit for safety.
	valQuery := database.ConvertPlaceholders("DELETE FROM gk_custom_field_value WHERE field_id = ?")
	if _, err := r.db.Exec(valQuery, id); err != nil {
		return fmt.Errorf("hard delete custom field values: %w", err)
	}

	defQuery := database.ConvertPlaceholders("DELETE FROM gk_custom_field_def WHERE id = ?")
	if _, err := r.db.Exec(defQuery, id); err != nil {
		return fmt.Errorf("hard delete custom field def: %w", err)
	}
	return nil
}

// --- Value CRUD ---

// GetValues retrieves all custom field values for an entity, joined with definitions.
// If fieldNames is non-empty, only those fields are returned.
func (r *Repository) GetValues(entityType string, objectID int64, fieldNames []string) (map[string]any, error) {
	query := `
		SELECT d.name, d.field_type,
		       v.val_text, v.val_int, v.val_decimal, v.val_decimal2,
		       v.val_date, v.val_datetime, v.val_json
		FROM gk_custom_field_def d
		JOIN gk_custom_field_value v ON v.field_id = d.id
		WHERE d.entity_type = ? AND v.object_id = ? AND d.valid_id = 1
	`
	args := []any{entityType, objectID}

	if len(fieldNames) > 0 {
		placeholders := make([]string, len(fieldNames))
		for i, fn := range fieldNames {
			placeholders[i] = "?"
			args = append(args, fn)
		}
		query += " AND d.name IN (" + strings.Join(placeholders, ",") + ")"
	}

	query = database.ConvertPlaceholders(query)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get custom field values: %w", err)
	}
	defer rows.Close()

	result := make(map[string]any)
	for rows.Next() {
		var (
			name, fieldType                string
			valText                        *string
			valInt                         *int64
			valDecimal, valDecimal2        *float64
			valDate, valDatetime           *time.Time
			valJSON                        *json.RawMessage
		)
		if err := rows.Scan(&name, &fieldType, &valText, &valInt, &valDecimal, &valDecimal2, &valDate, &valDatetime, &valJSON); err != nil {
			return nil, fmt.Errorf("scan custom field value: %w", err)
		}
		result[name] = extractValue(fieldType, valText, valInt, valDecimal, valDecimal2, valDate, valDatetime, valJSON)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom field values: %w", err)
	}
	return result, nil
}

// SetValues stores custom field values for an entity.
// values is field_name → value (or field_name → FieldOp for atomic operations).
// The caller is responsible for validation.
func (r *Repository) SetValues(entityType string, objectID int64, values map[string]any) error {
	// Look up all field defs we need.
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}

	defs, err := r.getDefsByNames(entityType, names)
	if err != nil {
		return err
	}

	for name, val := range values {
		def, ok := defs[name]
		if !ok {
			return fmt.Errorf("unknown custom field %q for entity type %q", name, entityType)
		}

		// Route atomic operations to dedicated handlers.
		if op, isOp := AsFieldOp(val); isOp {
			if err := r.atomicUpdate(def, objectID, op); err != nil {
				return fmt.Errorf("atomic op %q on field %q: %w", op.Op, name, err)
			}
			continue
		}

		if err := r.upsertValue(def, objectID, val); err != nil {
			return fmt.Errorf("set custom field %q: %w", name, err)
		}
	}
	return nil
}

// QueryByFields finds entity object IDs matching the given custom field filters.
func (r *Repository) QueryByFields(entityType string, filters []FieldFilter) ([]int64, error) {
	if len(filters) == 0 {
		return nil, nil
	}

	// Each filter becomes a subquery; we intersect them.
	var subqueries []string
	var args []any

	for _, f := range filters {
		def, err := r.GetDefByEntityAndName(entityType, f.Field)
		if err != nil {
			return nil, fmt.Errorf("query filter field %q: %w", f.Field, err)
		}
		if def == nil {
			return nil, fmt.Errorf("unknown custom field %q for entity type %q", f.Field, entityType)
		}

		col := StorageColumn(def.FieldType)
		clause, filterArgs, err := buildFilterClause(def.ID, col, f)
		if err != nil {
			return nil, err
		}
		subqueries = append(subqueries, clause)
		args = append(args, filterArgs...)
	}

	// Intersect all subqueries.
	query := strings.Join(subqueries, " INTERSECT ")
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query custom fields: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan query result: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- Atomic operations ---

// atomicUpdate dispatches to the appropriate atomic handler.
func (r *Repository) atomicUpdate(def *FieldDef, objectID int64, op *FieldOp) error {
	switch op.Op {
	case OpIncrement:
		return r.atomicIncrement(def, objectID, op)
	case OpAppend:
		return r.atomicAppend(def, objectID, op)
	case OpRemove:
		return r.atomicRemove(def, objectID, op)
	case OpCAS:
		return r.atomicCAS(def, objectID, op)
	case OpToggle:
		return r.atomicToggle(def, objectID)
	default:
		return fmt.Errorf("unsupported operation %q", op.Op)
	}
}

// atomicIncrement adds a delta to a numeric field. If the row doesn't exist,
// it inserts with the delta as the initial value.
func (r *Repository) atomicIncrement(def *FieldDef, objectID int64, op *FieldOp) error {
	col := StorageColumn(def.FieldType)
	delta, err := toFloat64(op.Value)
	if err != nil {
		return fmt.Errorf("increment value: %w", err)
	}

	// Build the WHERE clause with optional floor/ceiling constraints.
	where := "field_id = ? AND object_id = ?"
	args := []any{delta, def.ID, objectID}

	if op.Floor != nil {
		where += fmt.Sprintf(" AND %s + ? >= ?", col)
		args = append(args, delta, *op.Floor)
	}
	if op.Ceiling != nil {
		where += fmt.Sprintf(" AND %s + ? <= ?", col)
		args = append(args, delta, *op.Ceiling)
	}

	updateQuery := database.ConvertPlaceholders(
		fmt.Sprintf("UPDATE gk_custom_field_value SET %s = %s + ? WHERE %s", col, col, where),
	)
	result, err := r.db.Exec(updateQuery, args...)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 1 {
		return nil
	}

	// No row was updated. Either the row doesn't exist, or a floor/ceiling
	// constraint was violated. Check which case.
	existsQuery := database.ConvertPlaceholders(
		"SELECT 1 FROM gk_custom_field_value WHERE field_id = ? AND object_id = ?",
	)
	var dummy int
	err = r.db.QueryRow(existsQuery, def.ID, objectID).Scan(&dummy)
	if err == nil {
		// Row exists but constraint was violated.
		if op.Floor != nil || op.Ceiling != nil {
			return fmt.Errorf("increment would breach bounds")
		}
	}

	// Row doesn't exist — insert with delta as initial value, respecting bounds.
	if op.Floor != nil && delta < *op.Floor {
		return fmt.Errorf("increment would breach bounds")
	}
	if op.Ceiling != nil && delta > *op.Ceiling {
		return fmt.Errorf("increment would breach bounds")
	}
	fv := &FieldValue{FieldID: def.ID, ObjectID: objectID}
	if err := assignValue(fv, def.FieldType, delta); err != nil {
		return err
	}
	insQuery := database.ConvertPlaceholders(`
		INSERT INTO gk_custom_field_value
			(field_id, object_id, val_text, val_int, val_decimal, val_decimal2, val_date, val_datetime, val_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err = r.db.Exec(insQuery,
		fv.FieldID, fv.ObjectID,
		fv.ValText, fv.ValInt, fv.ValDecimal, fv.ValDecimal2,
		fv.ValDate, fv.ValDatetime, fv.ValJSON,
	)
	return err
}

// atomicAppend adds an item to a multi_select JSON array.
func (r *Repository) atomicAppend(def *FieldDef, objectID int64, op *FieldOp) error {
	item, ok := op.Value.(string)
	if !ok {
		return fmt.Errorf("append requires string value")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Read current value.
	selQuery := database.ConvertPlaceholders(
		"SELECT val_json FROM gk_custom_field_value WHERE field_id = ? AND object_id = ?",
	)
	var current []string
	var valJSON *json.RawMessage
	err = tx.QueryRow(selQuery, def.ID, objectID).Scan(&valJSON)

	if err == sql.ErrNoRows {
		// No existing row — insert with single-element array.
		current = []string{item}
	} else if err != nil {
		return err
	} else if valJSON != nil {
		if err := json.Unmarshal(*valJSON, &current); err != nil {
			return fmt.Errorf("unmarshal multi_select: %w", err)
		}
		// Check for duplicates.
		for _, v := range current {
			if v == item {
				return tx.Commit() // Already present, no-op.
			}
		}
		current = append(current, item)
	} else {
		current = []string{item}
	}

	raw, err := json.Marshal(current)
	if err != nil {
		return err
	}

	if valJSON == nil {
		// Insert new row.
		jrm := json.RawMessage(raw)
		insQuery := database.ConvertPlaceholders(`
			INSERT INTO gk_custom_field_value
				(field_id, object_id, val_text, val_int, val_decimal, val_decimal2, val_date, val_datetime, val_json)
			VALUES (?, ?, NULL, NULL, NULL, NULL, NULL, NULL, ?)
		`)
		if _, err := tx.Exec(insQuery, def.ID, objectID, jrm); err != nil {
			return err
		}
	} else {
		updQuery := database.ConvertPlaceholders(
			"UPDATE gk_custom_field_value SET val_json = ? WHERE field_id = ? AND object_id = ?",
		)
		if _, err := tx.Exec(updQuery, json.RawMessage(raw), def.ID, objectID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// atomicRemove removes an item from a multi_select JSON array.
func (r *Repository) atomicRemove(def *FieldDef, objectID int64, op *FieldOp) error {
	item, ok := op.Value.(string)
	if !ok {
		return fmt.Errorf("remove requires string value")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	selQuery := database.ConvertPlaceholders(
		"SELECT val_json FROM gk_custom_field_value WHERE field_id = ? AND object_id = ?",
	)
	var valJSON *json.RawMessage
	err = tx.QueryRow(selQuery, def.ID, objectID).Scan(&valJSON)
	if err == sql.ErrNoRows || valJSON == nil {
		return tx.Commit() // Nothing to remove from.
	}
	if err != nil {
		return err
	}

	var current []string
	if err := json.Unmarshal(*valJSON, &current); err != nil {
		return fmt.Errorf("unmarshal multi_select: %w", err)
	}

	filtered := make([]string, 0, len(current))
	for _, v := range current {
		if v != item {
			filtered = append(filtered, v)
		}
	}

	raw, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	updQuery := database.ConvertPlaceholders(
		"UPDATE gk_custom_field_value SET val_json = ? WHERE field_id = ? AND object_id = ?",
	)
	if _, err := tx.Exec(updQuery, json.RawMessage(raw), def.ID, objectID); err != nil {
		return err
	}
	return tx.Commit()
}

// atomicCAS performs a compare-and-swap: set to new value only if current equals expected.
func (r *Repository) atomicCAS(def *FieldDef, objectID int64, op *FieldOp) error {
	col := StorageColumn(def.FieldType)

	// Build the expected value for the WHERE clause.
	expectFV := &FieldValue{FieldID: def.ID, ObjectID: objectID}
	if err := assignValue(expectFV, def.FieldType, op.Expect); err != nil {
		return fmt.Errorf("cas expect: %w", err)
	}
	expectCol := fieldValueColumn(expectFV, col)

	// Build the new value.
	newFV := &FieldValue{FieldID: def.ID, ObjectID: objectID}
	if err := assignValue(newFV, def.FieldType, op.Value); err != nil {
		return fmt.Errorf("cas new value: %w", err)
	}
	newCol := fieldValueColumn(newFV, col)

	updateQuery := database.ConvertPlaceholders(
		fmt.Sprintf("UPDATE gk_custom_field_value SET %s = ? WHERE field_id = ? AND object_id = ? AND %s = ?", col, col),
	)
	result, err := r.db.Exec(updateQuery, newCol, def.ID, objectID, expectCol)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cas: current value does not match expected")
	}
	return nil
}

// atomicToggle flips a boolean field value.
func (r *Repository) atomicToggle(def *FieldDef, objectID int64) error {
	updateQuery := database.ConvertPlaceholders(
		"UPDATE gk_custom_field_value SET val_int = CASE WHEN val_int = 1 THEN 0 ELSE 1 END WHERE field_id = ? AND object_id = ?",
	)
	result, err := r.db.Exec(updateQuery, def.ID, objectID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		// No existing row — insert as true (toggling from implicit false).
		var one int64 = 1
		fv := &FieldValue{FieldID: def.ID, ObjectID: objectID, ValInt: &one}
		insQuery := database.ConvertPlaceholders(`
			INSERT INTO gk_custom_field_value
				(field_id, object_id, val_text, val_int, val_decimal, val_decimal2, val_date, val_datetime, val_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		_, err = r.db.Exec(insQuery,
			fv.FieldID, fv.ObjectID,
			fv.ValText, fv.ValInt, fv.ValDecimal, fv.ValDecimal2,
			fv.ValDate, fv.ValDatetime, fv.ValJSON,
		)
		return err
	}
	return nil
}

// fieldValueColumn extracts the typed value from a FieldValue for the given column name.
func fieldValueColumn(fv *FieldValue, col string) any {
	switch col {
	case "val_text":
		if fv.ValText != nil {
			return *fv.ValText
		}
	case "val_int":
		if fv.ValInt != nil {
			return *fv.ValInt
		}
	case "val_decimal":
		if fv.ValDecimal != nil {
			return *fv.ValDecimal
		}
	case "val_date":
		if fv.ValDate != nil {
			return *fv.ValDate
		}
	case "val_datetime":
		if fv.ValDatetime != nil {
			return *fv.ValDatetime
		}
	case "val_json":
		if fv.ValJSON != nil {
			return *fv.ValJSON
		}
	}
	return nil
}

// --- Internal helpers ---

func (r *Repository) getDefsByNames(entityType string, names []string) (map[string]*FieldDef, error) {
	placeholders := make([]string, len(names))
	args := []any{entityType}
	for i, n := range names {
		placeholders[i] = "?"
		args = append(args, n)
	}

	query := database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT id, name, label, entity_type, field_type,
		       owner_type, owner_name, migrated_from,
		       section, field_order, description, placeholder,
		       required, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM gk_custom_field_def
		WHERE entity_type = ? AND name IN (%s) AND valid_id = 1
	`, strings.Join(placeholders, ",")))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get defs by names: %w", err)
	}
	defer rows.Close()

	defs, err := scanFieldDefs(rows)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*FieldDef, len(defs))
	for i := range defs {
		result[defs[i].Name] = &defs[i]
	}
	return result, nil
}

func (r *Repository) upsertValue(def *FieldDef, objectID int64, val any) error {
	// Delete existing value.
	delQuery := database.ConvertPlaceholders("DELETE FROM gk_custom_field_value WHERE field_id = ? AND object_id = ?")
	if _, err := r.db.Exec(delQuery, def.ID, objectID); err != nil {
		return fmt.Errorf("delete old value: %w", err)
	}

	// If nil/empty, just clear.
	if val == nil {
		return nil
	}

	fv := &FieldValue{FieldID: def.ID, ObjectID: objectID}
	if err := assignValue(fv, def.FieldType, val); err != nil {
		return err
	}

	insQuery := database.ConvertPlaceholders(`
		INSERT INTO gk_custom_field_value
			(field_id, object_id, val_text, val_int, val_decimal, val_decimal2, val_date, val_datetime, val_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err := r.db.Exec(insQuery,
		fv.FieldID, fv.ObjectID,
		fv.ValText, fv.ValInt, fv.ValDecimal, fv.ValDecimal2,
		fv.ValDate, fv.ValDatetime, fv.ValJSON,
	)
	return err
}

// assignValue populates the correct FieldValue column based on field type.
func assignValue(fv *FieldValue, fieldType string, val any) error {
	switch fieldType {
	case FieldText, FieldTextArea, FieldSelect, FieldURL, FieldEmail, FieldPhone:
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("expected string for %s field, got %T", fieldType, val)
		}
		fv.ValText = &s

	case FieldInteger:
		n, err := toInt64(val)
		if err != nil {
			return fmt.Errorf("expected integer: %w", err)
		}
		fv.ValInt = &n

	case FieldBoolean:
		b, err := toBool(val)
		if err != nil {
			return fmt.Errorf("expected boolean: %w", err)
		}
		var n int64
		if b {
			n = 1
		}
		fv.ValInt = &n

	case FieldDecimal:
		d, err := toFloat64(val)
		if err != nil {
			return fmt.Errorf("expected decimal: %w", err)
		}
		fv.ValDecimal = &d

	case FieldDate:
		t, err := toDate(val)
		if err != nil {
			return fmt.Errorf("expected date: %w", err)
		}
		fv.ValDate = &t

	case FieldDateTime:
		t, err := toDateTime(val)
		if err != nil {
			return fmt.Errorf("expected datetime: %w", err)
		}
		fv.ValDatetime = &t

	case FieldPoint:
		// Expect map with lat/lng. Store in val_json + denormalised decimal columns.
		raw, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("marshal point: %w", err)
		}
		jrm := json.RawMessage(raw)
		fv.ValJSON = &jrm

		// Extract lat/lng for indexed queries.
		var ll LatLng
		if err := json.Unmarshal(raw, &ll); err == nil {
			fv.ValDecimal = &ll.Lat
			fv.ValDecimal2 = &ll.Lng
		}

	case FieldMultiSelect, FieldPolygon, FieldAddress:
		raw, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", fieldType, err)
		}
		jrm := json.RawMessage(raw)
		fv.ValJSON = &jrm

		// For address, extract postcode into val_text for indexed lookup.
		if fieldType == FieldAddress {
			var addr Address
			if err := json.Unmarshal(raw, &addr); err == nil && addr.Postcode != "" {
				fv.ValText = &addr.Postcode
			}
			// Extract lat/lng for proximity queries.
			if err := json.Unmarshal(raw, &addr); err == nil && (addr.Lat != 0 || addr.Lng != 0) {
				fv.ValDecimal = &addr.Lat
				fv.ValDecimal2 = &addr.Lng
			}
		}

	default:
		return fmt.Errorf("unsupported field type %q", fieldType)
	}
	return nil
}

// extractValue reads the correct column and returns a typed Go value.
func extractValue(fieldType string, valText *string, valInt *int64, valDecimal, valDecimal2 *float64, valDate, valDatetime *time.Time, valJSON *json.RawMessage) any {
	switch fieldType {
	case FieldText, FieldTextArea, FieldSelect, FieldURL, FieldEmail, FieldPhone:
		if valText != nil {
			return *valText
		}
	case FieldInteger:
		if valInt != nil {
			return *valInt
		}
	case FieldBoolean:
		if valInt != nil {
			return *valInt == 1
		}
	case FieldDecimal:
		if valDecimal != nil {
			return *valDecimal
		}
	case FieldDate:
		if valDate != nil {
			return valDate.Format("2006-01-02")
		}
	case FieldDateTime:
		if valDatetime != nil {
			return valDatetime.Format(time.RFC3339)
		}
	case FieldPoint:
		if valJSON != nil {
			var ll LatLng
			if json.Unmarshal(*valJSON, &ll) == nil {
				return ll
			}
		}
	case FieldAddress:
		if valJSON != nil {
			var addr Address
			if json.Unmarshal(*valJSON, &addr) == nil {
				return addr
			}
		}
	case FieldMultiSelect:
		if valJSON != nil {
			var arr []string
			if json.Unmarshal(*valJSON, &arr) == nil {
				return arr
			}
		}
	case FieldPolygon:
		if valJSON != nil {
			var raw any
			if json.Unmarshal(*valJSON, &raw) == nil {
				return raw
			}
		}
	}
	return nil
}

// buildFilterClause builds a SQL subquery for one filter condition.
func buildFilterClause(fieldID int64, col string, f FieldFilter) (string, []any, error) {
	base := "SELECT object_id FROM gk_custom_field_value WHERE field_id = ?"
	args := []any{fieldID}

	// Special case for "near" on point fields — bounding box pre-filter.
	if f.Operator == "near" {
		ll, ok := f.Value.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("near filter requires {lat, lng} value")
		}
		lat, _ := toFloat64(ll["lat"])
		lng, _ := toFloat64(ll["lng"])
		radiusKm, _ := toFloat64(f.Value2)
		if radiusKm <= 0 {
			radiusKm = 25 // default 25km
		}
		// Rough degree offset (1 degree ≈ 111km).
		dlat := radiusKm / 111.0
		dlng := radiusKm / (111.0 * 0.7) // rough cos(lat) approximation

		clause := base + " AND val_decimal BETWEEN ? AND ? AND val_decimal2 BETWEEN ? AND ?"
		args = append(args, lat-dlat, lat+dlat, lng-dlng, lng+dlng)
		return clause, args, nil
	}

	var op string
	switch f.Operator {
	case "eq":
		op = "="
	case "neq":
		op = "!="
	case "gt":
		op = ">"
	case "lt":
		op = "<"
	case "gte":
		op = ">="
	case "lte":
		op = "<="
	case "like":
		op = "LIKE"
	case "between":
		clause := base + fmt.Sprintf(" AND %s BETWEEN ? AND ?", col)
		args = append(args, f.Value, f.Value2)
		return clause, args, nil
	case "in":
		vals, ok := f.Value.([]any)
		if !ok {
			return "", nil, fmt.Errorf("in filter requires array value")
		}
		placeholders := make([]string, len(vals))
		for i, v := range vals {
			placeholders[i] = "?"
			args = append(args, v)
		}
		clause := base + fmt.Sprintf(" AND %s IN (%s)", col, strings.Join(placeholders, ","))
		return clause, args, nil
	default:
		return "", nil, fmt.Errorf("unsupported filter operator %q", f.Operator)
	}

	clause := base + fmt.Sprintf(" AND %s %s ?", col, op)
	args = append(args, f.Value)
	return clause, args, nil
}

// --- Scan helpers ---

func scanFieldDefs(rows *sql.Rows) ([]FieldDef, error) {
	var defs []FieldDef
	for rows.Next() {
		var f FieldDef
		err := rows.Scan(
			&f.ID, &f.Name, &f.Label, &f.EntityType, &f.FieldType,
			&f.OwnerType, &f.OwnerName, &f.MigratedFrom,
			&f.Section, &f.FieldOrder, &f.Description, &f.Placeholder,
			&f.Required, &f.Config, &f.ValidID,
			&f.CreateTime, &f.CreateBy, &f.ChangeTime, &f.ChangeBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan custom field def: %w", err)
		}
		defs = append(defs, f)
	}
	return defs, rows.Err()
}

func scanFieldDef(row *sql.Row) (*FieldDef, error) {
	var f FieldDef
	err := row.Scan(
		&f.ID, &f.Name, &f.Label, &f.EntityType, &f.FieldType,
		&f.OwnerType, &f.OwnerName, &f.MigratedFrom,
		&f.Section, &f.FieldOrder, &f.Description, &f.Placeholder,
		&f.Required, &f.Config, &f.ValidID,
		&f.CreateTime, &f.CreateBy, &f.ChangeTime, &f.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, fmt.Errorf("scan custom field def: %w", err)
	}
	return &f, nil
}

// --- Type coercion helpers ---

func toInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int64:
		return n, nil
	case float64:
		return int64(n), nil
	case json.Number:
		return n.Int64()
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func toFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		return n.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func toBool(v any) (bool, error) {
	switch b := v.(type) {
	case bool:
		return b, nil
	case int:
		return b != 0, nil
	case int64:
		return b != 0, nil
	case float64:
		return b != 0, nil
	case string:
		return b == "1" || b == "true" || b == "on", nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

func toDate(v any) (time.Time, error) {
	switch d := v.(type) {
	case string:
		return time.Parse("2006-01-02", d)
	case time.Time:
		return d, nil
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to date", v)
	}
}

func toDateTime(v any) (time.Time, error) {
	switch d := v.(type) {
	case string:
		if t, err := time.Parse(time.RFC3339, d); err == nil {
			return t, nil
		}
		if t, err := time.Parse("2006-01-02T15:04", d); err == nil {
			return t, nil
		}
		return time.Parse("2006-01-02 15:04:05", d)
	case time.Time:
		return d, nil
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to datetime", v)
	}
}
