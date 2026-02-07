// Package schema provides database schema loading and management.
package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/database"
)

// YAMLSchema represents the structure of a YAML schema file.
type YAMLSchema struct {
	Tables map[string]YAMLTable `yaml:",inline"`
}

// YAMLTable represents a single table definition in YAML.
type YAMLTable struct {
	PK         string                 `yaml:"pk"`
	Columns    map[string]YAMLColumn  `yaml:"columns"`
	Indexes    []string               `yaml:"indexes"`
	Unique     []string               `yaml:"unique"`
	Timestamps bool                   `yaml:"timestamps"`
	Meta       map[string]interface{} `yaml:"meta"`
}

// YAMLColumn represents a column definition in YAML.
type YAMLColumn struct {
	Type     string      `yaml:"type"`
	Required bool        `yaml:"required"`
	Unique   bool        `yaml:"unique"`
	Default  interface{} `yaml:"default"`
	Index    bool        `yaml:"index"`
}

// SchemaLoader loads YAML schema definitions.
type SchemaLoader struct {
	schemaDir string
	schemas   map[string]database.TableSchema
}

// NewSchemaLoader creates a new schema loader.
func NewSchemaLoader(schemaDir string) *SchemaLoader {
	return &SchemaLoader{
		schemaDir: schemaDir,
		schemas:   make(map[string]database.TableSchema),
	}
}

// LoadAll loads all YAML schema files from the schema directory.
func (l *SchemaLoader) LoadAll() error {
	files, err := os.ReadDir(l.schemaDir)
	if err != nil {
		return fmt.Errorf("failed to read schema directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		if err := l.LoadFile(file.Name()); err != nil {
			return fmt.Errorf("failed to load %s: %w", file.Name(), err)
		}
	}

	return nil
}

// LoadFile loads a single YAML schema file.
func (l *SchemaLoader) LoadFile(filename string) error {
	path := filepath.Join(l.schemaDir, filename)
	data, err := os.ReadFile(path) //nolint:gosec // G304 false positive - schema file
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var yamlSchema YAMLSchema
	if err := yaml.Unmarshal(data, &yamlSchema); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert YAML schema to database.TableSchema
	for tableName, table := range yamlSchema.Tables {
		schema := l.convertToTableSchema(tableName, table)
		l.schemas[tableName] = schema
	}

	return nil
}

// LoadFromString loads schema from a YAML string (useful for testing).
func (l *SchemaLoader) LoadFromString(yamlContent string) error {
	var yamlSchema map[string]YAMLTable
	if err := yaml.Unmarshal([]byte(yamlContent), &yamlSchema); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert YAML schema to database.TableSchema
	for tableName, table := range yamlSchema {
		schema := l.convertToTableSchema(tableName, table)
		l.schemas[tableName] = schema
	}

	return nil
}

// convertToTableSchema converts a YAML table definition to database.TableSchema.
func (l *SchemaLoader) convertToTableSchema(name string, table YAMLTable) database.TableSchema {
	columns := make(map[string]database.ColumnDef)

	for colName, col := range table.Columns {
		// Parse column type with modifiers
		colType := col.Type
		required := col.Required
		unique := col.Unique

		// Handle shorthand syntax: "varchar(200)!" means required
		if strings.HasSuffix(colType, "!") {
			colType = strings.TrimSuffix(colType, "!")
			required = true
		}

		// Handle shorthand syntax: "varchar(200)?" means nullable
		if strings.HasSuffix(colType, "?") {
			colType = strings.TrimSuffix(colType, "?")
			required = false
		}

		// Check for unique modifier in type
		if strings.Contains(colType, " unique") {
			colType = strings.Replace(colType, " unique", "", 1)
			unique = true
		}

		// Parse default value
		var defaultVal interface{}
		if col.Default != nil {
			defaultVal = col.Default
		} else if strings.Contains(colType, " default(") {
			// Extract default value from type string
			start := strings.Index(colType, " default(")
			end := strings.Index(colType[start:], ")") + start
			if end > start {
				defaultStr := colType[start+9 : end]
				colType = colType[:start] + colType[end+1:]

				// Parse default value
				if defaultStr == "1" || defaultStr == "true" {
					defaultVal = true
				} else if defaultStr == "0" || defaultStr == "false" {
					defaultVal = false
				} else if strings.HasPrefix(defaultStr, "'") && strings.HasSuffix(defaultStr, "'") {
					defaultVal = strings.Trim(defaultStr, "'")
				} else {
					defaultVal = defaultStr
				}
			}
		}

		columns[colName] = database.ColumnDef{
			Type:     strings.TrimSpace(colType),
			Required: required,
			Unique:   unique,
			Default:  defaultVal,
			Index:    col.Index,
		}
	}

	return database.TableSchema{
		Name:       name,
		PK:         table.PK,
		Columns:    columns,
		Indexes:    table.Indexes,
		Unique:     table.Unique,
		Timestamps: table.Timestamps,
		Meta:       table.Meta,
	}
}

// GetSchema returns a schema by table name.
func (l *SchemaLoader) GetSchema(tableName string) (database.TableSchema, bool) {
	schema, ok := l.schemas[tableName]
	return schema, ok
}

// GetAllSchemas returns all loaded schemas.
func (l *SchemaLoader) GetAllSchemas() map[string]database.TableSchema {
	return l.schemas
}

// GenerateSQL generates SQL for all schemas using the specified driver.
func (l *SchemaLoader) GenerateSQL(driver database.DatabaseDriver) ([]string, error) {
	queries := make([]string, 0, len(l.schemas))

	// Sort tables by dependencies (simple approach - you might need topological sort for complex schemas)
	for _, schema := range l.schemas {
		query, err := driver.CreateTable(schema)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SQL for %s: %w", schema.Name, err)
		}
		queries = append(queries, query.SQL)
	}

	return queries, nil
}
