package api

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetScreenConfigForField(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "field_id", "screen_key", "config_value", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(1, 10, "AgentTicketCreate", 1, time.Now(), 1, time.Now(), 1).
		AddRow(2, 10, "AgentTicketZoom", 2, time.Now(), 1, time.Now(), 1)

	mock.ExpectQuery("SELECT .* FROM dynamic_field_screen_config WHERE field_id").
		WithArgs(10).
		WillReturnRows(rows)

	configs, err := getScreenConfigForFieldWithDB(db, 10)
	require.NoError(t, err)
	assert.Len(t, configs, 2)
	assert.Equal(t, "AgentTicketCreate", configs[0].ScreenKey)
	assert.Equal(t, DFScreenEnabled, configs[0].ConfigValue)
	assert.Equal(t, "AgentTicketZoom", configs[1].ScreenKey)
	assert.Equal(t, DFScreenRequired, configs[1].ConfigValue)
}

func TestGetScreenConfigForScreen(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "field_id", "screen_key", "config_value", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(1, 10, "AgentTicketCreate", 1, time.Now(), 1, time.Now(), 1).
		AddRow(2, 20, "AgentTicketCreate", 2, time.Now(), 1, time.Now(), 1)

	mock.ExpectQuery("SELECT .* FROM dynamic_field_screen_config WHERE screen_key").
		WithArgs("AgentTicketCreate").
		WillReturnRows(rows)

	configs, err := getScreenConfigForScreenWithDB(db, "AgentTicketCreate")
	require.NoError(t, err)
	assert.Len(t, configs, 2)
	assert.Equal(t, 10, configs[0].FieldID)
	assert.Equal(t, 20, configs[1].FieldID)
}

func TestSetScreenConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect delete first, then insert
	mock.ExpectExec(`DELETE FROM dynamic_field_screen_config WHERE field_id = \? AND screen_key = \?`).
		WithArgs(10, "AgentTicketCreate").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("INSERT INTO dynamic_field_screen_config").
		WithArgs(10, "AgentTicketCreate", 1, sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = setScreenConfigWithDB(db, 10, "AgentTicketCreate", DFScreenEnabled, 1)
	require.NoError(t, err)
}

func TestSetScreenConfigDisabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// When setting to disabled (0), just delete the row
	mock.ExpectExec(`DELETE FROM dynamic_field_screen_config WHERE field_id = \? AND screen_key = \?`).
		WithArgs(10, "AgentTicketCreate").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = setScreenConfigWithDB(db, 10, "AgentTicketCreate", DFScreenDisabled, 1)
	require.NoError(t, err)
}

func TestBulkSetScreenConfigForField(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Allow expectations to be matched in any order since map iteration is random
	mock.MatchExpectationsInOrder(false)

	mock.ExpectBegin()

	// Delete all existing configs for this field
	mock.ExpectExec(`DELETE FROM dynamic_field_screen_config WHERE field_id = \?`).
		WithArgs(10).
		WillReturnResult(sqlmock.NewResult(0, 5))

	// Insert new configs (only non-zero) - order doesn't matter
	mock.ExpectExec("INSERT INTO dynamic_field_screen_config").
		WithArgs(10, "AgentTicketCreate", 1, sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO dynamic_field_screen_config").
		WithArgs(10, "AgentTicketZoom", 2, sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(2, 1))

	mock.ExpectCommit()

	configs := map[string]int{
		"AgentTicketCreate": DFScreenEnabled,
		"AgentTicketZoom":   DFScreenRequired,
		"AgentTicketClose":  DFScreenDisabled, // Should not insert
	}

	err = bulkSetScreenConfigForFieldWithDB(db, 10, configs, 1)
	require.NoError(t, err)
}

func TestGetFieldsForScreenWithConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "internal_field", "name", "label", "field_order",
		"field_type", "object_type", "config", "valid_id",
		"create_time", "create_by", "change_time", "change_by", "config_value",
	}).
		AddRow(10, 0, "CustomField1", "Custom Field 1", 1, "Text", "Ticket", "---\nDefaultValue: test\n", 1, now, 1, now, 1, 1).
		AddRow(20, 0, "CustomField2", "Custom Field 2", 2, "Dropdown", "Ticket", "---\nPossibleValues:\n  a: A\n  b: B\n", 1, now, 1, now, 1, 2)

	mock.ExpectQuery("SELECT df.id, df.internal_field, df.name, df.label").
		WithArgs("AgentTicketCreate", "Ticket").
		WillReturnRows(rows)

	fields, err := getFieldsForScreenWithConfigWithDB(db, "AgentTicketCreate", "Ticket")
	require.NoError(t, err)
	assert.Len(t, fields, 2)
	assert.Equal(t, "CustomField1", fields[0].Field.Name)
	assert.Equal(t, DFScreenEnabled, fields[0].ConfigValue)
	assert.Equal(t, "CustomField2", fields[1].Field.Name)
	assert.Equal(t, DFScreenRequired, fields[1].ConfigValue)
}

func TestGetAllScreenConfigsMatrix(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// First query for fields
	fieldRows := sqlmock.NewRows([]string{"id", "internal_field", "name", "label", "field_order", "field_type", "object_type", "config", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(10, 0, "Field1", "Field 1", 1, "Text", "Ticket", "---\n", 1, time.Now(), 1, time.Now(), 1).
		AddRow(20, 0, "Field2", "Field 2", 2, "Text", "Ticket", "---\n", 1, time.Now(), 1, time.Now(), 1)

	mock.ExpectQuery("SELECT .* FROM dynamic_field WHERE object_type").
		WithArgs("Ticket").
		WillReturnRows(fieldRows)

	// Second query for configs
	configRows := sqlmock.NewRows([]string{"id", "field_id", "screen_key", "config_value", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(1, 10, "AgentTicketCreate", 1, time.Now(), 1, time.Now(), 1).
		AddRow(2, 10, "AgentTicketZoom", 2, time.Now(), 1, time.Now(), 1).
		AddRow(3, 20, "AgentTicketCreate", 1, time.Now(), 1, time.Now(), 1)

	mock.ExpectQuery("SELECT .* FROM dynamic_field_screen_config WHERE field_id IN").
		WillReturnRows(configRows)

	matrix, err := getScreenConfigMatrixWithDB(db, "Ticket")
	require.NoError(t, err)

	assert.Len(t, matrix.Fields, 2)
	assert.Contains(t, matrix.ConfigMap[10], "AgentTicketCreate")
	assert.Equal(t, DFScreenEnabled, matrix.ConfigMap[10]["AgentTicketCreate"])
	assert.Equal(t, DFScreenRequired, matrix.ConfigMap[10]["AgentTicketZoom"])
}
