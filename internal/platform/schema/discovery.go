package schema

import (
    "database/sql"
    "fmt"
    "strings"
    
    "github.com/gotrs-io/gotrs-ce/internal/database"
)

// Discovery handles database schema discovery
type Discovery struct {
	db      *sql.DB
	verbose bool
}

// TableInfo contains information about a database table
type TableInfo struct {
	Name        string
	Columns     []ColumnInfo
	PrimaryKey  string
	ForeignKeys []ForeignKeyInfo
	Indexes     []IndexInfo
	HasValidID  bool
	HasDeleted  bool
}

// ColumnInfo contains information about a table column
type ColumnInfo struct {
	Name         string
	DataType     string
	IsNullable   bool
	DefaultValue sql.NullString
	MaxLength    sql.NullInt64
	IsPrimaryKey bool
	IsForeignKey bool
	IsUnique     bool
	Comment      sql.NullString
}

// ForeignKeyInfo contains foreign key relationship information
type ForeignKeyInfo struct {
	Column           string
	ReferencedTable  string
	ReferencedColumn string
}

// IndexInfo contains index information
type IndexInfo struct {
	Name    string
	Columns []string
	IsUnique bool
}

// ModuleConfig represents the generated module configuration
type ModuleConfig struct {
	Module struct {
		Name        string `yaml:"name"`
		Table       string `yaml:"table"`
		DisplayName string `yaml:"display_name"`
		Description string `yaml:"description"`
	} `yaml:"module"`
	Fields         []Field         `yaml:"fields"`
	ComputedFields []ComputedField `yaml:"computed_fields,omitempty"`
	Features       Features        `yaml:"features"`
	Filters        []Filter        `yaml:"filters,omitempty"`
}

// Field represents a module field
type Field struct {
	Name          string `yaml:"name"`
	Type          string `yaml:"type"`
	Label         string `yaml:"label"`
	Required      bool   `yaml:"required,omitempty"`
	PrimaryKey    bool   `yaml:"primary_key,omitempty"`
	ShowInList    bool   `yaml:"show_in_list"`
	ShowInForm    bool   `yaml:"show_in_form"`
	Searchable    bool   `yaml:"searchable,omitempty"`
	LookupTable   string `yaml:"lookup_table,omitempty"`
	LookupKey     string `yaml:"lookup_key,omitempty"`
	LookupDisplay string `yaml:"lookup_display,omitempty"`
	MaxLength     int    `yaml:"max_length,omitempty"`
	Format        string `yaml:"format,omitempty"`
}

// ComputedField represents a computed field with lambda function
type ComputedField struct {
	Name       string `yaml:"name"`
	Label      string `yaml:"label"`
	ShowInList bool   `yaml:"show_in_list"`
	Lambda     string `yaml:"lambda"`
}

// Features represents module features
type Features struct {
	Search      bool `yaml:"search"`
	ExportCSV   bool `yaml:"export_csv"`
	Pagination  bool `yaml:"pagination"`
	SoftDelete  bool `yaml:"soft_delete"`
	BulkActions bool `yaml:"bulk_actions"`
}

// Filter represents a module filter
type Filter struct {
	Field       string   `yaml:"field"`
	Type        string   `yaml:"type"`
	Label       string   `yaml:"label"`
	Source      string   `yaml:"source,omitempty"`
	Query       string   `yaml:"query,omitempty"`
	Options     []Option `yaml:"options,omitempty"`
}

// Option represents a filter option
type Option struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

// NewDiscovery creates a new Discovery instance
func NewDiscovery(db *sql.DB, verbose bool) *Discovery {
	return &Discovery{
		db:      db,
		verbose: verbose,
	}
}

// GetTables returns all user tables in the database
func (d *Discovery) GetTables() ([]string, error) {
    query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = $1 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name
    `
	
    rows, err := d.db.Query(database.ConvertPlaceholders(query), "public")
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %v", err)
	}
	defer rows.Close()
	
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %v", err)
		}
		tables = append(tables, table)
	}
	
	return tables, nil
}

// GetTableInfo retrieves detailed information about a table
func (d *Discovery) GetTableInfo(tableName string) (*TableInfo, error) {
	info := &TableInfo{
		Name: tableName,
	}
	
	// Get columns
	columns, err := d.getColumns(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}
	info.Columns = columns
	
	// Check for special columns
	for _, col := range columns {
		if col.IsPrimaryKey {
			info.PrimaryKey = col.Name
		}
		if col.Name == "valid_id" {
			info.HasValidID = true
		}
		if col.Name == "deleted" || col.Name == "deleted_at" {
			info.HasDeleted = true
		}
	}
	
	// Get foreign keys
	foreignKeys, err := d.getForeignKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %v", err)
	}
	info.ForeignKeys = foreignKeys
	
	// Get indexes
	indexes, err := d.getIndexes(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %v", err)
	}
	info.Indexes = indexes
	
	return info, nil
}

// getColumns retrieves column information for a table
func (d *Discovery) getColumns(tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT 
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			c.character_maximum_length,
			COALESCE(pk.is_primary, false) as is_primary_key,
			COALESCE(fk.is_foreign, false) as is_foreign_key,
			COALESCE(u.is_unique, false) as is_unique,
			col_description(pgc.oid, c.ordinal_position) as column_comment
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name, true as is_primary
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu 
				ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 
			AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		LEFT JOIN (
			SELECT DISTINCT kcu.column_name, true as is_foreign
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu 
				ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 
			AND tc.constraint_type = 'FOREIGN KEY'
		) fk ON c.column_name = fk.column_name
		LEFT JOIN (
			SELECT DISTINCT kcu.column_name, true as is_unique
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu 
				ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 
			AND tc.constraint_type = 'UNIQUE'
		) u ON c.column_name = u.column_name
		LEFT JOIN pg_class pgc ON pgc.relname = c.table_name
		WHERE c.table_name = $1
		ORDER BY c.ordinal_position
	`
	
    rows, err := d.db.Query(database.ConvertPlaceholders(query), tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var isNullable string
		
		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&isNullable,
			&col.DefaultValue,
			&col.MaxLength,
			&col.IsPrimaryKey,
			&col.IsForeignKey,
			&col.IsUnique,
			&col.Comment,
		)
		if err != nil {
			return nil, err
		}
		
		col.IsNullable = (isNullable == "YES")
		columns = append(columns, col)
	}
	
	return columns, nil
}

// getForeignKeys retrieves foreign key information for a table
func (d *Discovery) getForeignKeys(tableName string) ([]ForeignKeyInfo, error) {
	query := `
		SELECT
			kcu.column_name,
			ccu.table_name AS referenced_table,
			ccu.column_name AS referenced_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
		WHERE tc.table_name = $1
		AND tc.constraint_type = 'FOREIGN KEY'
	`
	
    rows, err := d.db.Query(database.ConvertPlaceholders(query), tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var foreignKeys []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		if err := rows.Scan(&fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}
	
	return foreignKeys, nil
}

// getIndexes retrieves index information for a table
func (d *Discovery) getIndexes(tableName string) ([]IndexInfo, error) {
	query := `
		SELECT 
			indexname,
			indexdef,
			CASE WHEN indexdef LIKE '%UNIQUE%' THEN true ELSE false END as is_unique
		FROM pg_indexes
		WHERE tablename = $1
		AND indexname NOT LIKE '%_pkey'
	`
	
    rows, err := d.db.Query(database.ConvertPlaceholders(query), tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var indexes []IndexInfo
	for rows.Next() {
		var idx IndexInfo
		var indexDef string
		if err := rows.Scan(&idx.Name, &indexDef, &idx.IsUnique); err != nil {
			return nil, err
		}
		// Parse column names from index definition
		// This is simplified - real implementation would parse more carefully
		indexes = append(indexes, idx)
	}
	
	return indexes, nil
}

// GenerateModule generates a module configuration from table info
func (d *Discovery) GenerateModule(tableName string) (*ModuleConfig, error) {
	// Get table information
	tableInfo, err := d.GetTableInfo(tableName)
	if err != nil {
		return nil, err
	}
	
	// Create module config
	module := &ModuleConfig{}
	module.Module.Name = tableName
	module.Module.Table = tableName
	module.Module.DisplayName = formatDisplayName(tableName)
	module.Module.Description = fmt.Sprintf("Manage %s records", formatDisplayName(tableName))
	
	// Generate fields
	module.Fields = d.generateFields(tableInfo)
	
	// Generate computed fields
	module.ComputedFields = d.generateComputedFields(tableInfo)
	
	// Set features
	module.Features = Features{
		Search:      true,
		ExportCSV:   true,
		Pagination:  true,
		SoftDelete:  tableInfo.HasValidID,
		BulkActions: true,
	}
	
	// Generate filters
	module.Filters = d.generateFilters(tableInfo)
	
	return module, nil
}

// generateFields generates field configurations from table info
func (d *Discovery) generateFields(info *TableInfo) []Field {
	var fields []Field
	
	for _, col := range info.Columns {
		field := Field{
			Name:     col.Name,
			Type:     mapDataType(col.DataType),
			Label:    formatLabel(col.Name),
			Required: !col.IsNullable && col.Name != "id",
		}
		
		// Set primary key
		if col.IsPrimaryKey {
			field.PrimaryKey = true
			field.ShowInList = true
			field.ShowInForm = false
		} else {
			// Determine visibility
			field.ShowInList = shouldShowInList(col.Name, col.DataType)
			field.ShowInForm = shouldShowInForm(col.Name)
		}
		
		// Set searchable for text fields
		if isTextType(col.DataType) && field.ShowInList {
			field.Searchable = true
		}
		
		// Handle foreign keys
		if col.IsForeignKey {
			fk := findForeignKey(info.ForeignKeys, col.Name)
			if fk != nil {
				field.Type = "select"
				field.LookupTable = fk.ReferencedTable
				field.LookupKey = fk.ReferencedColumn
				field.LookupDisplay = guessDisplayColumn(fk.ReferencedTable)
			}
		}
		
		// Set max length for string fields
		if col.MaxLength.Valid && col.MaxLength.Int64 > 0 {
			field.MaxLength = int(col.MaxLength.Int64)
		}
		
		// Set format for datetime fields
		if col.DataType == "timestamp" || col.DataType == "timestamp without time zone" {
			if strings.Contains(col.Name, "create") || strings.Contains(col.Name, "change") {
				field.Format = "relative"
			}
		}
		
		fields = append(fields, field)
	}
	
	return fields
}

// generateComputedFields generates computed fields with lambda functions
func (d *Discovery) generateComputedFields(info *TableInfo) []ComputedField {
	var computed []ComputedField
	
	// Check for name fields to combine
	hasFirstName := false
	hasLastName := false
	for _, col := range info.Columns {
		if col.Name == "first_name" {
			hasFirstName = true
		}
		if col.Name == "last_name" {
			hasLastName = true
		}
	}
	
	if hasFirstName && hasLastName {
		computed = append(computed, ComputedField{
			Name:       "full_name",
			Label:      "Full Name",
			ShowInList: true,
			Lambda: `
				const firstName = record.first_name || '';
				const lastName = record.last_name || '';
				return firstName && lastName ? firstName + ' ' + lastName : firstName || lastName || '-';
			`,
		})
	}
	
	// Add status badge for valid_id
	if info.HasValidID {
		computed = append(computed, ComputedField{
			Name:       "status_badge",
			Label:      "Status",
			ShowInList: true,
			Lambda: `
				if (record.valid_id == 1) {
					return '<span class="px-2 py-1 text-xs font-medium bg-green-100 text-green-800 rounded-full">Active</span>';
				} else if (record.valid_id == 2) {
					return '<span class="px-2 py-1 text-xs font-medium bg-red-100 text-red-800 rounded-full">Inactive</span>';
				} else {
					return '<span class="px-2 py-1 text-xs font-medium bg-gray-100 text-gray-800 rounded-full">Invalid</span>';
				}
			`,
		})
	}
	
	return computed
}

// generateFilters generates filter configurations
func (d *Discovery) generateFilters(info *TableInfo) []Filter {
	var filters []Filter
	
	// Add valid_id filter if present
	if info.HasValidID {
		filters = append(filters, Filter{
			Field: "valid_id",
			Type:  "select",
			Label: "Status",
			Options: []Option{
				{Value: "", Label: "All Status"},
				{Value: "1", Label: "Active"},
				{Value: "2", Label: "Inactive"},
			},
		})
	}
	
	// Add filters for foreign keys
	for _, fk := range info.ForeignKeys {
		// Skip certain foreign keys
		if strings.HasSuffix(fk.Column, "_by") || strings.HasSuffix(fk.Column, "_id") && strings.Contains(fk.Column, "create") {
			continue
		}
		
		filters = append(filters, Filter{
			Field:  fk.Column,
			Type:   "select",
			Label:  formatLabel(fk.Column),
			Source: "database",
			Query:  fmt.Sprintf("SELECT %s, name FROM %s ORDER BY name", fk.ReferencedColumn, fk.ReferencedTable),
		})
	}
	
	// Add date range filters for timestamp columns
	for _, col := range info.Columns {
		if strings.Contains(col.DataType, "timestamp") {
			if strings.Contains(col.Name, "create") || strings.Contains(col.Name, "change") {
				filters = append(filters, Filter{
					Field: col.Name,
					Type:  "date_range",
					Label: formatLabel(col.Name),
				})
			}
		}
	}
	
	return filters
}

// Helper functions

func ShouldSkipTable(tableName string) bool {
	skipPrefixes := []string{"pg_", "sql_", "information_", "tmp_", "temp_"}
	skipSuffixes := []string{"_log", "_backup", "_old", "_temp"}
	
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(tableName, prefix) {
			return true
		}
	}
	
	for _, suffix := range skipSuffixes {
		if strings.HasSuffix(tableName, suffix) {
			return true
		}
	}
	
	// Skip migration tracking table
	if tableName == "schema_migrations" {
		return true
	}
	
	return false
}

func mapDataType(pgType string) string {
	switch {
	case strings.Contains(pgType, "int"):
		return "integer"
	case strings.Contains(pgType, "numeric"), strings.Contains(pgType, "decimal"):
		return "decimal"
	case strings.Contains(pgType, "bool"):
		return "boolean"
	case strings.Contains(pgType, "timestamp"), strings.Contains(pgType, "date"):
		return "datetime"
	case strings.Contains(pgType, "time"):
		return "time"
	case strings.Contains(pgType, "text"):
		return "text"
	case strings.Contains(pgType, "json"):
		return "json"
	case strings.Contains(pgType, "uuid"):
		return "uuid"
	case strings.Contains(pgType, "char"):
		return "string"
	default:
		return "string"
	}
}

func formatDisplayName(tableName string) string {
	// Convert snake_case to Title Case
	words := strings.Split(tableName, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ") + " Management"
}

func formatLabel(columnName string) string {
	// Remove common suffixes
	name := strings.TrimSuffix(columnName, "_id")
	name = strings.TrimSuffix(name, "_by")
	name = strings.TrimSuffix(name, "_time")
	name = strings.TrimSuffix(name, "_date")
	
	// Convert snake_case to Title Case
	words := strings.Split(name, "_")
	for i, word := range words {
		if len(word) > 0 {
			// Handle common abbreviations
			switch strings.ToLower(word) {
				case "id":
					words[i] = "ID"
				case "url":
					words[i] = "URL"
				case "api":
					words[i] = "API"
				case "sla":
					words[i] = "SLA"
				default:
					words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
	}
	return strings.Join(words, " ")
}

func shouldShowInList(columnName, dataType string) bool {
	// Always hide certain columns in list
	hideInList := []string{"password", "secret", "token", "body", "content", "description"}
	for _, hide := range hideInList {
		if strings.Contains(columnName, hide) {
			return false
		}
	}
	
	// Hide large text fields
	if dataType == "text" {
		return false
	}
	
	// Show most other fields
	return true
}

func shouldShowInForm(columnName string) bool {
	// Never show these in forms
	noForm := []string{"id", "created", "changed", "create_time", "change_time", "create_by", "change_by"}
	for _, no := range noForm {
		if strings.Contains(columnName, no) {
			return false
		}
	}
	return true
}

func isTextType(dataType string) bool {
	textTypes := []string{"char", "text", "varchar"}
	for _, tt := range textTypes {
		if strings.Contains(dataType, tt) {
			return true
		}
	}
	return false
}

func findForeignKey(foreignKeys []ForeignKeyInfo, columnName string) *ForeignKeyInfo {
	for _, fk := range foreignKeys {
		if fk.Column == columnName {
			return &fk
		}
	}
	return nil
}

func guessDisplayColumn(tableName string) string {
	// Common display column names in order of preference
	_ = []string{"name", "title", "login", "email", "code", "id"} // displayColumns - for future use
	
	// For now, return "name" as default
	// In a real implementation, we'd query the table to see which columns exist
	return "name"
}