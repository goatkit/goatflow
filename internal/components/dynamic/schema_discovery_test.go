package dynamic

import (
	"testing"
	
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaDiscovery_GetTables(t *testing.T) {
	// Arrange
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	discovery := NewSchemaDiscovery(db)

	// Mock the query for getting tables
	rows := sqlmock.NewRows([]string{"table_name", "table_comment"}).
		AddRow("users", "User accounts").
		AddRow("ticket", "Support tickets").
		AddRow("customer_company", "Customer organizations")

	mock.ExpectQuery("SELECT(.+)FROM information_schema.tables(.+)").
		WillReturnRows(rows)

	// Act
	tables, err := discovery.GetTables()

	// Assert
	require.NoError(t, err)
	require.Len(t, tables, 3)
	assert.Equal(t, "users", tables[0].Name)
	assert.Equal(t, "User accounts", tables[0].Comment)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDiscovery_GetTableColumns(t *testing.T) {
	// Arrange
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	discovery := NewSchemaDiscovery(db)

	// Mock the query for getting columns
	rows := sqlmock.NewRows([]string{
		"column_name", "data_type", "is_nullable", 
		"column_default", "character_maximum_length", "column_comment",
	}).
		AddRow("id", "integer", "NO", "nextval('users_id_seq')", nil, "Primary key").
		AddRow("login", "character varying", "NO", nil, 200, "Username").
		AddRow("pw", "character varying", "YES", nil, 200, "Password hash").
		AddRow("first_name", "character varying", "YES", nil, 200, "First name").
		AddRow("last_name", "character varying", "YES", nil, 200, "Last name").
		AddRow("valid_id", "smallint", "NO", "1", nil, "Status")

	mock.ExpectQuery("SELECT (.+) FROM information_schema.columns (.+) WHERE table_name = ").
		WithArgs("users").
		WillReturnRows(rows)

	// Act
	columns, err := discovery.GetTableColumns("users")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, columns, 6)
	
	// Check first column
	assert.Equal(t, "id", columns[0].Name)
	assert.Equal(t, "integer", columns[0].DataType)
	assert.False(t, columns[0].IsNullable)
	assert.True(t, columns[0].IsPrimaryKey)
	
	// Check login column
	assert.Equal(t, "login", columns[1].Name)
	assert.Equal(t, "character varying", columns[1].DataType)
	assert.Equal(t, 200, columns[1].MaxLength)
	
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDiscovery_GetTableConstraints(t *testing.T) {
	// Arrange
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	discovery := NewSchemaDiscovery(db)

	// Mock the query for constraints
	rows := sqlmock.NewRows([]string{
		"constraint_name", "constraint_type", "column_name",
	}).
		AddRow("users_pkey", "PRIMARY KEY", "id").
		AddRow("users_login_key", "UNIQUE", "login").
		AddRow("users_valid_id_fkey", "FOREIGN KEY", "valid_id")

	mock.ExpectQuery("SELECT (.+) FROM information_schema.table_constraints (.+)").
		WithArgs("users").
		WillReturnRows(rows)

	// Act
	constraints, err := discovery.GetTableConstraints("users")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, constraints, 3)
	assert.Equal(t, "PRIMARY KEY", constraints[0].Type)
	assert.Equal(t, "UNIQUE", constraints[1].Type)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDiscovery_GenerateModuleConfig(t *testing.T) {
	// Arrange
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	discovery := NewSchemaDiscovery(db)

	// Mock columns query
	columnRows := sqlmock.NewRows([]string{
		"column_name", "data_type", "is_nullable", 
		"column_default", "character_maximum_length", "column_comment",
	}).
		AddRow("id", "integer", "NO", "nextval('ticket_id_seq')", nil, "").
		AddRow("tn", "character varying", "NO", nil, 50, "Ticket number").
		AddRow("title", "character varying", "YES", nil, 255, "Ticket title").
		AddRow("queue_id", "integer", "NO", nil, nil, "Queue reference").
		AddRow("ticket_state_id", "smallint", "NO", nil, nil, "State").
		AddRow("create_time", "timestamp", "NO", "CURRENT_TIMESTAMP", nil, "")

	// First query for columns
	mock.ExpectQuery("SELECT (.+) FROM information_schema.columns (.+)").
		WithArgs("ticket").
		WillReturnRows(columnRows)

	// First constraints query (called from GetTableColumns)
	constraintRows1 := sqlmock.NewRows([]string{
		"constraint_name", "constraint_type", "column_name",
	}).
		AddRow("ticket_pkey", "PRIMARY KEY", "id").
		AddRow("ticket_tn_key", "UNIQUE", "tn")

	mock.ExpectQuery("SELECT (.+) FROM information_schema.table_constraints (.+)").
		WithArgs("ticket").
		WillReturnRows(constraintRows1)
	
	// Second constraints query (called from GenerateModuleConfig)
	constraintRows2 := sqlmock.NewRows([]string{
		"constraint_name", "constraint_type", "column_name",
	}).
		AddRow("ticket_pkey", "PRIMARY KEY", "id").
		AddRow("ticket_tn_key", "UNIQUE", "tn")

	mock.ExpectQuery("SELECT (.+) FROM information_schema.table_constraints (.+)").
		WithArgs("ticket").
		WillReturnRows(constraintRows2)

	// Act
	config, err := discovery.GenerateModuleConfig("ticket")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "ticket", config.Module.Name)
	assert.Equal(t, "Ticket", config.Module.Singular)
	assert.Equal(t, "Tickets", config.Module.Plural)
	assert.Equal(t, "ticket", config.Module.Table)
	
	// Check fields
	assert.Len(t, config.Fields, 6)
	
	// ID field should not show in form
	idField := config.Fields[0]
	assert.Equal(t, "id", idField.Name)
	assert.False(t, idField.ShowInForm)
	assert.True(t, idField.ShowInList)
	
	// Title field
	titleField := findField(config.Fields, "title")
	assert.NotNil(t, titleField)
	assert.Equal(t, "string", titleField.Type)
	assert.True(t, titleField.ShowInForm)
	assert.True(t, titleField.ShowInList)
	
	// Timestamp field
	createTimeField := findField(config.Fields, "create_time")
	assert.NotNil(t, createTimeField)
	assert.Equal(t, "datetime", createTimeField.Type)
	assert.False(t, createTimeField.ShowInForm)
	
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDiscovery_InferFieldType(t *testing.T) {
	discovery := &SchemaDiscovery{}
	
	tests := []struct {
		columnName string
		dataType   string
		expected   string
	}{
		// By column name
		{"password", "varchar", "password"},
		{"pw", "text", "password"},
		{"email", "varchar", "email"},
		{"email_address", "text", "email"},
		{"url", "varchar", "url"},
		{"website", "text", "url"},
		{"phone", "varchar", "phone"},
		{"telephone", "text", "phone"},
		{"notes", "text", "textarea"},
		{"description", "text", "textarea"},
		{"comments", "text", "textarea"},
		
		// By data type
		{"anything", "integer", "integer"},
		{"anything", "bigint", "integer"},
		{"anything", "smallint", "integer"},
		{"anything", "numeric", "decimal"},
		{"anything", "decimal", "decimal"},
		{"anything", "boolean", "checkbox"},
		{"anything", "date", "date"},
		{"anything", "timestamp", "datetime"},
		{"anything", "time", "time"},
		{"anything", "text", "textarea"},
		{"anything", "varchar", "string"},
		{"anything", "character varying", "string"},
	}
	
	for _, tt := range tests {
		t.Run(tt.columnName+"_"+tt.dataType, func(t *testing.T) {
			result := discovery.InferFieldType(tt.columnName, tt.dataType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function
func findField(fields []Field, name string) *Field {
	for _, f := range fields {
		if f.Name == name {
			return &f
		}
	}
	return nil
}