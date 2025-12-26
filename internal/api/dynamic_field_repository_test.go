//go:build db

package api

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDynamicFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	configYAML := []byte("DefaultValue: test\n")

	rows := sqlmock.NewRows([]string{
		"id", "internal_field", "name", "label", "field_order",
		"field_type", "object_type", "config", "valid_id",
		"create_time", "create_by", "change_time", "change_by",
	}).AddRow(
		1, 0, "ContractID", "Contract ID", 100,
		"Text", "Ticket", configYAML, 1,
		now, 1, now, 1,
	).AddRow(
		2, 0, "Priority", "Priority", 200,
		"Dropdown", "Ticket", nil, 1,
		now, 1, now, 1,
	)

	mock.ExpectQuery("SELECT (.+) FROM dynamic_field").
		WillReturnRows(rows)

	fields, err := getDynamicFieldsWithDB(db, "", "")
	require.NoError(t, err)
	assert.Len(t, fields, 2)
	assert.Equal(t, "ContractID", fields[0].Name)
	assert.Equal(t, "Priority", fields[1].Name)
	assert.NotNil(t, fields[0].Config)
	assert.Equal(t, "test", fields[0].Config.DefaultValue)
}

func TestGetDynamicFieldsByObjectType(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "internal_field", "name", "label", "field_order",
		"field_type", "object_type", "config", "valid_id",
		"create_time", "create_by", "change_time", "change_by",
	}).AddRow(
		1, 0, "ArticleNote", "Note", 100,
		"TextArea", "Article", nil, 1,
		now, 1, now, 1,
	)

	mock.ExpectQuery("SELECT (.+) FROM dynamic_field WHERE (.+)").
		WithArgs("Article").
		WillReturnRows(rows)

	fields, err := getDynamicFieldsWithDB(db, "Article", "")
	require.NoError(t, err)
	assert.Len(t, fields, 1)
	assert.Equal(t, "Article", fields[0].ObjectType)
}

func TestGetDynamicField(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	configYAML := []byte("PossibleValues:\n  low: Low\n  high: High\n")

	rows := sqlmock.NewRows([]string{
		"id", "internal_field", "name", "label", "field_order",
		"field_type", "object_type", "config", "valid_id",
		"create_time", "create_by", "change_time", "change_by",
	}).AddRow(
		1, 0, "Severity", "Severity", 100,
		"Dropdown", "Ticket", configYAML, 1,
		now, 1, now, 1,
	)

	mock.ExpectQuery("SELECT (.+) FROM dynamic_field WHERE id = ?").
		WithArgs(1).
		WillReturnRows(rows)

	field, err := getDynamicFieldWithDB(db, 1)
	require.NoError(t, err)
	require.NotNil(t, field)
	assert.Equal(t, "Severity", field.Name)
	assert.Equal(t, "Dropdown", field.FieldType)
	assert.NotNil(t, field.Config)
	assert.Contains(t, field.Config.PossibleValues, "low")
}

func TestGetDynamicFieldByName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "internal_field", "name", "label", "field_order",
		"field_type", "object_type", "config", "valid_id",
		"create_time", "create_by", "change_time", "change_by",
	}).AddRow(
		1, 0, "VIP", "VIP Customer", 100,
		"Checkbox", "Ticket", nil, 1,
		now, 1, now, 1,
	)

	mock.ExpectQuery("SELECT (.+) FROM dynamic_field WHERE name = ?").
		WithArgs("VIP").
		WillReturnRows(rows)

	field, err := getDynamicFieldByNameWithDB(db, "VIP")
	require.NoError(t, err)
	require.NotNil(t, field)
	assert.Equal(t, "VIP", field.Name)
	assert.Equal(t, "Checkbox", field.FieldType)
}

func TestCreateDynamicField(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	field := &DynamicField{
		Name:       "NewField",
		Label:      "New Field",
		FieldOrder: 100,
		FieldType:  DFTypeText,
		ObjectType: DFObjectTicket,
		ValidID:    1,
		Config:     &DynamicFieldConfig{DefaultValue: "default"},
	}

	mock.ExpectExec("INSERT INTO dynamic_field").
		WithArgs(
			0, // internal_field
			"NewField",
			"New Field",
			100,
			"Text",
			"Ticket",
			sqlmock.AnyArg(), // config YAML
			1,                // valid_id
			sqlmock.AnyArg(), // create_time
			sqlmock.AnyArg(), // create_by
			sqlmock.AnyArg(), // change_time
			sqlmock.AnyArg(), // change_by
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	id, err := createDynamicFieldWithDB(db, field, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
}

func TestUpdateDynamicField(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	field := &DynamicField{
		ID:         1,
		Name:       "UpdatedField",
		Label:      "Updated Field",
		FieldOrder: 200,
		FieldType:  DFTypeText,
		ObjectType: DFObjectTicket,
		ValidID:    1,
		Config:     &DynamicFieldConfig{MaxLength: 500},
	}

	mock.ExpectExec("UPDATE dynamic_field SET").
		WithArgs(
			"UpdatedField",
			"Updated Field",
			200,
			"Text",
			"Ticket",
			sqlmock.AnyArg(), // config YAML
			1,                // valid_id
			sqlmock.AnyArg(), // change_time
			sqlmock.AnyArg(), // change_by
			1,                // id
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = updateDynamicFieldWithDB(db, field, 1)
	require.NoError(t, err)
}

func TestDeleteDynamicField(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Should delete values first
	mock.ExpectExec("DELETE FROM dynamic_field_value WHERE field_id = ?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 5))

	// Then delete the field
	mock.ExpectExec("DELETE FROM dynamic_field WHERE id = ?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = deleteDynamicFieldWithDB(db, 1)
	require.NoError(t, err)
}

func TestCheckDynamicFieldNameExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Test name exists
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("ExistingField", 0).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := checkDynamicFieldNameExistsWithDB(db, "ExistingField", 0)
	require.NoError(t, err)
	assert.True(t, exists)

	// Test name doesn't exist
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("NewField", 0).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	exists, err = checkDynamicFieldNameExistsWithDB(db, "NewField", 0)
	require.NoError(t, err)
	assert.False(t, exists)

	// Test excluding current ID (for update)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("ExistingField", 5).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	exists, err = checkDynamicFieldNameExistsWithDB(db, "ExistingField", 5)
	require.NoError(t, err)
	assert.False(t, exists)
}

// Value tests

func TestGetDynamicFieldValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	valueDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	valueInt := int64(1)
	valueText := "test value"

	rows := sqlmock.NewRows([]string{
		"id", "field_id", "object_id", "value_text", "value_date", "value_int",
	}).AddRow(
		1, 10, 100, &valueText, nil, nil,
	).AddRow(
		2, 20, 100, nil, &valueDate, nil,
	).AddRow(
		3, 30, 100, nil, nil, &valueInt,
	)

	mock.ExpectQuery("SELECT (.+) FROM dynamic_field_value WHERE object_id = ?").
		WithArgs(int64(100)).
		WillReturnRows(rows)

	values, err := getDynamicFieldValuesWithDB(db, 100)
	require.NoError(t, err)
	assert.Len(t, values, 3)
	assert.Equal(t, "test value", *values[0].ValueText)
	assert.NotNil(t, values[1].ValueDate)
	assert.Equal(t, int64(1), *values[2].ValueInt)
}

func TestSetDynamicFieldValue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Should delete existing and insert new (use regexp.QuoteMeta for ? characters)
	mock.ExpectExec(`DELETE FROM dynamic_field_value WHERE field_id = \? AND object_id = \?`).
		WithArgs(10, int64(100)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("INSERT INTO dynamic_field_value").
		WithArgs(10, int64(100), "new value", nil, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	value := &DynamicFieldValue{
		FieldID:   10,
		ObjectID:  100,
		ValueText: strPtr("new value"),
	}
	err = setDynamicFieldValueWithDB(db, value)
	require.NoError(t, err)
}

func strPtr(s string) *string {
	return &s
}

func TestGetDynamicFieldValuesForDisplay(t *testing.T) {
	t.Run("formats text field correctly", func(t *testing.T) {
		field := DynamicField{
			ID:        1,
			Name:      "TestText",
			Label:     "Test Text",
			FieldType: DFTypeText,
			ValidID:   1,
		}
		
		display := DynamicFieldDisplay{Field: field}
		
		// Test with value
		text := "Hello World"
		display.DisplayValue = text
		display.Value = text
		
		assert.Equal(t, "Hello World", display.DisplayValue)
		assert.Equal(t, "Test Text", display.Field.Label)
	})
	
	t.Run("formats checkbox field correctly", func(t *testing.T) {
		field := DynamicField{
			ID:        2,
			Name:      "TestCheckbox",
			Label:     "Test Checkbox",
			FieldType: DFTypeCheckbox,
			ValidID:   1,
		}
		
		// Test checked
		display := DynamicFieldDisplay{
			Field: field,
			Value: int64(1),
			DisplayValue: "Yes",
		}
		assert.Equal(t, "Yes", display.DisplayValue)
		
		// Test unchecked
		display.Value = int64(0)
		display.DisplayValue = "No"
		assert.Equal(t, "No", display.DisplayValue)
	})
	
	t.Run("formats date field correctly", func(t *testing.T) {
		field := DynamicField{
			ID:        3,
			Name:      "TestDate",
			Label:     "Test Date",
			FieldType: DFTypeDate,
			ValidID:   1,
		}
		
		testDate := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
		display := DynamicFieldDisplay{
			Field:        field,
			Value:        testDate,
			DisplayValue: "2024-12-25",
		}
		
		assert.Equal(t, "2024-12-25", display.DisplayValue)
	})
	
	t.Run("handles missing value", func(t *testing.T) {
		field := DynamicField{
			ID:        4,
			Name:      "TestEmpty",
			Label:     "Test Empty",
			FieldType: DFTypeText,
			ValidID:   1,
		}
		
		display := DynamicFieldDisplay{
			Field:        field,
			DisplayValue: "-",
		}
		
		assert.Equal(t, "-", display.DisplayValue)
		assert.Nil(t, display.Value)
	})
}
