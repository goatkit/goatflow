package api

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// DynamicFieldFilter represents a filter condition for dynamic field values
type DynamicFieldFilter struct {
	FieldID   int    // ID of the dynamic field
	FieldName string // Name of the dynamic field (alternative to ID)
	Operator  string // eq, ne, contains, gt, lt, gte, lte, in, notin, empty, notempty
	Value     string // Value to compare against (comma-separated for 'in' operator)
}

// SearchableDynamicField wraps DynamicField with template-friendly Options
type SearchableDynamicField struct {
	DynamicField
	Options []map[string]string `json:"options"` // [{key: "value", value: "display_name"}, ...]
}

// Cache for searchable fields (reduces DB queries on every page load)
var (
	searchableFieldsCache     []SearchableDynamicField
	searchableFieldsCacheMu   sync.RWMutex
	searchableFieldsCacheTime time.Time
	searchableFieldsCacheTTL  = 30 * time.Second
)

// GetFieldsForSearch returns dynamic fields that are searchable (Ticket object type, valid)
// Results are cached for 30 seconds to improve performance
func GetFieldsForSearch() ([]SearchableDynamicField, error) {
	searchableFieldsCacheMu.RLock()
	if searchableFieldsCache != nil && time.Since(searchableFieldsCacheTime) < searchableFieldsCacheTTL {
		result := searchableFieldsCache
		searchableFieldsCacheMu.RUnlock()
		return result, nil
	}
	searchableFieldsCacheMu.RUnlock()

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	fields, err := getFieldsForSearchWithDB(db)
	if err != nil {
		return nil, err
	}

	searchableFieldsCacheMu.Lock()
	searchableFieldsCache = fields
	searchableFieldsCacheTime = time.Now()
	searchableFieldsCacheMu.Unlock()

	return fields, nil
}

func getFieldsForSearchWithDB(db *sql.DB) ([]SearchableDynamicField, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, internal_field, name, label, field_order,
		       field_type, object_type, config, valid_id,
		       create_time, create_by, change_time, change_by
		FROM dynamic_field
		WHERE object_type = $1 AND valid_id = 1
		ORDER BY field_order, name
	`)

	rows, err := db.Query(query, DFObjectTicket)
	if err != nil {
		return nil, fmt.Errorf("failed to query searchable dynamic fields: %w", err)
	}
	defer rows.Close()

	fields, err := scanDynamicFields(rows)
	if err != nil {
		return nil, err
	}

	result := make([]SearchableDynamicField, len(fields))
	for i, field := range fields {
		result[i] = SearchableDynamicField{DynamicField: field}
		if field.FieldType == DFTypeDropdown || field.FieldType == DFTypeMultiselect {
			if field.Config != nil && field.Config.PossibleValues != nil {
				for key, value := range field.Config.PossibleValues {
					result[i].Options = append(result[i].Options, map[string]string{
						"key":   key,
						"value": value,
					})
				}
			}
		}
	}

	return result, nil
}

// BuildDynamicFieldFilterSQL builds SQL conditions for filtering tickets by dynamic field values
// Returns the WHERE clause fragment and arguments to append
func BuildDynamicFieldFilterSQL(filters []DynamicFieldFilter, startArgNum int) (string, []interface{}, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	db, err := database.GetDB()
	if err != nil {
		return "", nil, err
	}

	var conditions []string
	var args []interface{}
	argNum := startArgNum

	for i, filter := range filters {
		// Get field info if we only have name
		var field *DynamicField
		if filter.FieldID > 0 {
			field, err = getDynamicFieldWithDB(db, filter.FieldID)
		} else if filter.FieldName != "" {
			field, err = getDynamicFieldByNameWithDB(db, filter.FieldName)
		}
		if err != nil || field == nil {
			continue // Skip invalid filters
		}

		alias := fmt.Sprintf("dfv%d", i)
		joinCondition := fmt.Sprintf("EXISTS (SELECT 1 FROM dynamic_field_value %s WHERE %s.object_id = t.id AND %s.field_id = $%d",
			alias, alias, alias, argNum)
		args = append(args, field.ID)
		argNum++

		// Determine which value column to use based on field type
		valueCol := fmt.Sprintf("%s.value_text", alias)
		switch field.FieldType {
		case DFTypeCheckbox:
			valueCol = fmt.Sprintf("%s.value_int", alias)
		case DFTypeDate, DFTypeDateTime:
			valueCol = fmt.Sprintf("%s.value_date", alias)
		}

		// Build condition based on operator
		switch filter.Operator {
		case "eq", "":
			if field.FieldType == DFTypeCheckbox {
				// Checkbox: 1 = checked, 0 or NULL = unchecked
				if filter.Value == "1" || filter.Value == "true" || filter.Value == "on" {
					joinCondition += fmt.Sprintf(" AND %s = 1", valueCol)
				} else {
					joinCondition += fmt.Sprintf(" AND (%s = 0 OR %s IS NULL)", valueCol, valueCol)
				}
			} else {
				joinCondition += fmt.Sprintf(" AND %s = $%d", valueCol, argNum)
				args = append(args, filter.Value)
				argNum++
			}
		case "ne":
			joinCondition += fmt.Sprintf(" AND (%s != $%d OR %s IS NULL)", valueCol, argNum, valueCol)
			args = append(args, filter.Value)
			argNum++
		case "contains":
			joinCondition += fmt.Sprintf(" AND %s LIKE $%d", valueCol, argNum)
			args = append(args, "%"+filter.Value+"%")
			argNum++
		case "gt":
			joinCondition += fmt.Sprintf(" AND %s > $%d", valueCol, argNum)
			args = append(args, filter.Value)
			argNum++
		case "lt":
			joinCondition += fmt.Sprintf(" AND %s < $%d", valueCol, argNum)
			args = append(args, filter.Value)
			argNum++
		case "gte":
			joinCondition += fmt.Sprintf(" AND %s >= $%d", valueCol, argNum)
			args = append(args, filter.Value)
			argNum++
		case "lte":
			joinCondition += fmt.Sprintf(" AND %s <= $%d", valueCol, argNum)
			args = append(args, filter.Value)
			argNum++
		case "in":
			// Split value by comma for IN clause
			values := strings.Split(filter.Value, ",")
			placeholders := make([]string, len(values))
			for j, v := range values {
				placeholders[j] = fmt.Sprintf("$%d", argNum)
				args = append(args, strings.TrimSpace(v))
				argNum++
			}
			joinCondition += fmt.Sprintf(" AND %s IN (%s)", valueCol, strings.Join(placeholders, ","))
		case "notin":
			// Split value by comma for NOT IN clause
			values := strings.Split(filter.Value, ",")
			placeholders := make([]string, len(values))
			for j, v := range values {
				placeholders[j] = fmt.Sprintf("$%d", argNum)
				args = append(args, strings.TrimSpace(v))
				argNum++
			}
			joinCondition += fmt.Sprintf(" AND %s NOT IN (%s)", valueCol, strings.Join(placeholders, ","))
		case "empty":
			// Field value is empty or doesn't exist
			joinCondition = fmt.Sprintf("NOT EXISTS (SELECT 1 FROM dynamic_field_value %s WHERE %s.object_id = t.id AND %s.field_id = $%d AND %s IS NOT NULL AND %s != '')",
				alias, alias, alias, argNum-1, valueCol, valueCol)
		case "notempty":
			joinCondition += fmt.Sprintf(" AND %s IS NOT NULL AND %s != ''", valueCol, valueCol)
		default:
			// Default to equals
			joinCondition += fmt.Sprintf(" AND %s = $%d", valueCol, argNum)
			args = append(args, filter.Value)
			argNum++
		}

		joinCondition += ")"
		conditions = append(conditions, joinCondition)
	}

	if len(conditions) == 0 {
		return "", nil, nil
	}

	return " AND " + strings.Join(conditions, " AND "), args, nil
}

// ParseDynamicFieldFiltersFromQuery parses URL query parameters into DynamicFieldFilter slice
// Expected format: df_FieldName=value or df_FieldName_op=value (e.g., df_CustomerType=VIP, df_Amount_gt=1000)
func ParseDynamicFieldFiltersFromQuery(queryParams map[string][]string) []DynamicFieldFilter {
	var filters []DynamicFieldFilter

	for key, values := range queryParams {
		if !strings.HasPrefix(key, "df_") || len(values) == 0 || values[0] == "" {
			continue
		}

		// Remove df_ prefix
		remainder := strings.TrimPrefix(key, "df_")

		// Check for operator suffix (e.g., _gt, _lt, _contains)
		fieldName := remainder
		operator := "eq"

		operators := []string{"_contains", "_gt", "_gte", "_lt", "_lte", "_ne", "_in", "_notin", "_empty", "_notempty"}
		for _, op := range operators {
			if strings.HasSuffix(remainder, op) {
				fieldName = strings.TrimSuffix(remainder, op)
				operator = strings.TrimPrefix(op, "_")
				break
			}
		}

		filters = append(filters, DynamicFieldFilter{
			FieldName: fieldName,
			Operator:  operator,
			Value:     values[0],
		})
	}

	return filters
}

// GetDistinctDynamicFieldValues returns distinct values for a dropdown/multiselect field
// Useful for populating filter dropdown options
func GetDistinctDynamicFieldValues(fieldID int, limit int) ([]string, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	query := database.ConvertPlaceholders(`
		SELECT DISTINCT value_text
		FROM dynamic_field_value
		WHERE field_id = $1 AND value_text IS NOT NULL AND value_text != ''
		ORDER BY value_text
		LIMIT $2
	`)

	rows, err := db.Query(query, fieldID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct values: %w", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			continue
		}
		values = append(values, val)
	}

	return values, nil
}
