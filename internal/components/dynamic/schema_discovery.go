package dynamic

import (
	"database/sql"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TableInfo represents database table information.
type TableInfo struct {
	Name    string
	Comment string
}

// ColumnInfo represents database column information.
type ColumnInfo struct {
	Name         string
	DataType     string
	IsNullable   bool
	DefaultValue *string
	MaxLength    int
	Comment      string
	IsPrimaryKey bool
	IsUnique     bool
	IsForeignKey bool
}

// ConstraintInfo represents database constraint information.
type ConstraintInfo struct {
	Name       string
	Type       string
	ColumnName string
}

// SchemaDiscovery handles database schema introspection.
type SchemaDiscovery struct {
	db *sql.DB
}

// NewSchemaDiscovery creates a new schema discovery instance.
func NewSchemaDiscovery(db *sql.DB) *SchemaDiscovery {
	return &SchemaDiscovery{db: db}
}

// GetTables retrieves all tables from the database.
func (sd *SchemaDiscovery) GetTables() ([]TableInfo, error) {
	query := `
		SELECT 
			table_name,
			COALESCE(obj_description(pgclass.oid), '') as table_comment
		FROM information_schema.tables
		LEFT JOIN pg_class pgclass ON pgclass.relname = table_name
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := sd.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Name, &table.Comment); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	return tables, nil
}

// GetTableColumns retrieves column information for a specific table.
func (sd *SchemaDiscovery) GetTableColumns(tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT 
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length,
			COALESCE(col_description(pgclass.oid, ordinal_position), '') as column_comment
		FROM information_schema.columns
		LEFT JOIN pg_class pgclass ON pgclass.relname = table_name
		WHERE table_name = ? 
		AND table_schema = 'public'
		ORDER BY ordinal_position
	`

	rows, err := sd.db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var isNullable string
		var maxLength sql.NullInt64
		var defaultValue sql.NullString

		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&isNullable,
			&defaultValue,
			&maxLength,
			&col.Comment,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}

		col.IsNullable = isNullable == "YES"
		if maxLength.Valid {
			col.MaxLength = int(maxLength.Int64)
		}
		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}

		// Check if primary key (simple check based on name and default)
		if col.Name == "id" && col.DefaultValue != nil && strings.Contains(*col.DefaultValue, "nextval") {
			col.IsPrimaryKey = true
		}

		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	// Get constraints to properly identify primary keys, unique, foreign keys
	constraints, err := sd.GetTableConstraints(tableName)
	if err == nil {
		for i := range columns {
			for _, c := range constraints {
				if c.ColumnName == columns[i].Name {
					switch c.Type {
					case "PRIMARY KEY":
						columns[i].IsPrimaryKey = true
					case "UNIQUE":
						columns[i].IsUnique = true
					case "FOREIGN KEY":
						columns[i].IsForeignKey = true
					}
				}
			}
		}
	}

	return columns, nil
}

// GetTableConstraints retrieves constraint information for a table.
func (sd *SchemaDiscovery) GetTableConstraints(tableName string) ([]ConstraintInfo, error) {
	query := `
		SELECT 
			tc.constraint_name,
			tc.constraint_type,
			kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.table_name = ?
		AND tc.table_schema = 'public'
		ORDER BY tc.constraint_type, kcu.ordinal_position
	`

	rows, err := sd.db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query constraints: %w", err)
	}
	defer rows.Close()

	var constraints []ConstraintInfo
	for rows.Next() {
		var c ConstraintInfo
		if err := rows.Scan(&c.Name, &c.Type, &c.ColumnName); err != nil {
			return nil, fmt.Errorf("failed to scan constraint row: %w", err)
		}
		constraints = append(constraints, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating constraints: %w", err)
	}

	return constraints, nil
}

// GenerateModuleConfig generates a ModuleConfig from database schema.
func (sd *SchemaDiscovery) GenerateModuleConfig(tableName string) (*ModuleConfig, error) {
	columns, err := sd.GetTableColumns(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	constraints, err := sd.GetTableConstraints(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get constraints: %w", err)
	}

	config := &ModuleConfig{}

	// Set module metadata
	config.Module.Name = tableName
	config.Module.Table = tableName
	config.Module.Singular = sd.toSingular(tableName)
	config.Module.Plural = sd.toPlural(tableName)
	config.Module.Description = fmt.Sprintf("Manage %s", config.Module.Plural)
	config.Module.RoutePrefix = fmt.Sprintf("/admin/dynamic/%s", tableName)

	// Generate fields from columns
	for _, col := range columns {
		field := Field{
			Name:     col.Name,
			DBColumn: col.Name,
			Label:    sd.toLabel(col.Name),
			Type:     sd.InferFieldType(col.Name, col.DataType),
			Required: !col.IsNullable && col.DefaultValue == nil,
		}

		// Configure display settings
		field.ShowInList = sd.shouldShowInList(col)
		field.ShowInForm = sd.shouldShowInForm(col)
		field.Searchable = sd.isSearchable(col)
		field.Sortable = sd.isSortable(col)

		// Add help text from comment
		if col.Comment != "" {
			field.Help = col.Comment
		}

		// Handle special cases
		if col.IsPrimaryKey {
			field.ShowInForm = false
			field.Required = false
		}

		if col.MaxLength > 0 && field.Type == "string" {
			// Could add validation pattern based on length
		}

		config.Fields = append(config.Fields, field)
	}

	// Set features based on table structure
	config.Features.Search = true
	config.Features.SoftDelete = sd.hasColumn(columns, "valid_id")
	config.Features.ExportCSV = true

	// Set validation rules
	for _, constraint := range constraints {
		if constraint.Type == "UNIQUE" {
			config.Validation.UniqueFields = append(config.Validation.UniqueFields, constraint.ColumnName)
		}
	}

	return config, nil
}

// InferFieldType infers the appropriate field type from column name and data type.
func (sd *SchemaDiscovery) InferFieldType(columnName, dataType string) string {
	columnLower := strings.ToLower(columnName)
	dataTypeLower := strings.ToLower(dataType)

	// Check column name patterns first
	switch {
	case strings.Contains(columnLower, "password") || columnLower == "pw":
		return "password"
	case strings.Contains(columnLower, "email"):
		return "email"
	case strings.Contains(columnLower, "url") || strings.Contains(columnLower, "website"):
		return "url"
	case strings.Contains(columnLower, "phone") || strings.Contains(columnLower, "tel"):
		return "phone"
	case strings.Contains(columnLower, "color") || strings.Contains(columnLower, "colour"):
		return "color"
	case strings.Contains(columnLower, "notes") || strings.Contains(columnLower, "description") || strings.Contains(columnLower, "comment"):
		if dataTypeLower == "text" || strings.Contains(dataTypeLower, "text") {
			return "textarea"
		}
	}

	// Check data type
	switch {
	case strings.Contains(dataTypeLower, "int"):
		return "integer"
	case strings.Contains(dataTypeLower, "numeric") || strings.Contains(dataTypeLower, "decimal"):
		return "decimal"
	case strings.Contains(dataTypeLower, "bool"):
		return "checkbox"
	case dataTypeLower == "date":
		return "date"
	case strings.Contains(dataTypeLower, "timestamp"):
		return "datetime"
	case dataTypeLower == "time":
		return "time"
	case dataTypeLower == "text":
		return "textarea"
	case strings.Contains(dataTypeLower, "char"):
		return "string"
	default:
		return "string"
	}
}

// Helper methods

func (sd *SchemaDiscovery) toSingular(tableName string) string {
	// Simple singularization with title case
	singular := tableName
	if strings.HasSuffix(tableName, "ies") {
		singular = strings.TrimSuffix(tableName, "ies") + "y"
	} else if strings.HasSuffix(tableName, "es") {
		singular = strings.TrimSuffix(tableName, "es")
	} else if strings.HasSuffix(tableName, "s") {
		singular = strings.TrimSuffix(tableName, "s")
	}
	return cases.Title(language.English).String(singular)
}

func (sd *SchemaDiscovery) toPlural(tableName string) string {
	// Simple pluralization with title case
	plural := tableName
	if strings.HasSuffix(tableName, "y") {
		plural = strings.TrimSuffix(tableName, "y") + "ies"
	} else if !strings.HasSuffix(tableName, "s") {
		plural = tableName + "s"
	}
	return cases.Title(language.English).String(plural)
}

func (sd *SchemaDiscovery) toLabel(columnName string) string {
	// Convert snake_case to Title Case
	parts := strings.Split(columnName, "_")
	title := cases.Title(language.English)
	for i, part := range parts {
		parts[i] = title.String(part)
	}
	return strings.Join(parts, " ")
}

func (sd *SchemaDiscovery) shouldShowInList(col ColumnInfo) bool {
	// Don't show long text fields, passwords, or system fields in list
	if col.DataType == "text" || strings.Contains(col.Name, "password") || strings.Contains(col.Name, "pw") {
		return false
	}
	// Don't show too many columns
	if strings.Contains(col.Name, "_by") && !strings.Contains(col.Name, "create") {
		return false
	}
	return true
}

func (sd *SchemaDiscovery) shouldShowInForm(col ColumnInfo) bool {
	// Don't show auto-generated fields in form
	if col.IsPrimaryKey || strings.Contains(col.Name, "_time") || strings.Contains(col.Name, "_by") {
		return false
	}
	return true
}

func (sd *SchemaDiscovery) isSearchable(col ColumnInfo) bool {
	// Text fields are searchable
	return strings.Contains(col.DataType, "char") || col.DataType == "text"
}

func (sd *SchemaDiscovery) isSortable(col ColumnInfo) bool {
	// Most fields are sortable except very long text
	return col.DataType != "text"
}

func (sd *SchemaDiscovery) hasColumn(columns []ColumnInfo, name string) bool {
	for _, col := range columns {
		if col.Name == name {
			return true
		}
	}
	return false
}
