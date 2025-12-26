package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDynamicFieldConfig_YAMLRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		config DynamicFieldConfig
	}{
		{
			name: "text field config",
			config: DynamicFieldConfig{
				DefaultValue: "default",
				MaxLength:    200,
				Link:         "https://example.com/$1",
			},
		},
		{
			name: "textarea config",
			config: DynamicFieldConfig{
				DefaultValue: "multi\nline",
				Rows:         5,
				Cols:         80,
			},
		},
		{
			name: "dropdown config",
			config: DynamicFieldConfig{
				PossibleValues: map[string]string{
					"low":    "Low Priority",
					"medium": "Medium Priority",
					"high":   "High Priority",
				},
				DefaultValue:       "medium",
				PossibleNone:       1,
				TranslatableValues: 1,
			},
		},
		{
			name: "checkbox config",
			config: DynamicFieldConfig{
				DefaultValue: "1",
			},
		},
		{
			name: "date config",
			config: DynamicFieldConfig{
				YearsInPast:     5,
				YearsInFuture:   10,
				DateRestriction: "DisablePastDates",
			},
		},
		{
			name: "datetime config",
			config: DynamicFieldConfig{
				YearsInPast:   1,
				YearsInFuture: 5,
				YearsPeriod:   1,
			},
		},
		{
			name: "text with regex validation",
			config: DynamicFieldConfig{
				RegExList: []RegEx{
					{Value: "^[A-Z]{3}-\\d{4}$", ErrorMessage: "Format must be XXX-0000"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DynamicField{
				Name:       "TestField",
				Label:      "Test Field",
				FieldType:  DFTypeText,
				ObjectType: DFObjectTicket,
				Config:     &tt.config,
			}

			// Serialize to YAML
			err := df.SerializeConfig()
			require.NoError(t, err)
			require.NotNil(t, df.ConfigRaw)

			// Create new field and parse
			df2 := &DynamicField{ConfigRaw: df.ConfigRaw}
			err = df2.ParseConfig()
			require.NoError(t, err)

			// Compare
			assert.Equal(t, tt.config.DefaultValue, df2.Config.DefaultValue)
			assert.Equal(t, tt.config.MaxLength, df2.Config.MaxLength)
			assert.Equal(t, tt.config.Rows, df2.Config.Rows)
			assert.Equal(t, tt.config.Cols, df2.Config.Cols)
			assert.Equal(t, tt.config.YearsInPast, df2.Config.YearsInPast)
			assert.Equal(t, tt.config.YearsInFuture, df2.Config.YearsInFuture)
			assert.Equal(t, tt.config.DateRestriction, df2.Config.DateRestriction)
			assert.Equal(t, len(tt.config.PossibleValues), len(df2.Config.PossibleValues))
			assert.Equal(t, len(tt.config.RegExList), len(df2.Config.RegExList))
		})
	}
}

func TestDynamicField_Validate(t *testing.T) {
	tests := []struct {
		name    string
		field   DynamicField
		wantErr string
	}{
		{
			name: "valid text field",
			field: DynamicField{
				Name:       "ContractID",
				Label:      "Contract ID",
				FieldType:  DFTypeText,
				ObjectType: DFObjectTicket,
			},
			wantErr: "",
		},
		{
			name: "missing name",
			field: DynamicField{
				Label:      "Test",
				FieldType:  DFTypeText,
				ObjectType: DFObjectTicket,
			},
			wantErr: "name is required",
		},
		{
			name: "invalid name with spaces",
			field: DynamicField{
				Name:       "Contract ID",
				Label:      "Contract ID",
				FieldType:  DFTypeText,
				ObjectType: DFObjectTicket,
			},
			wantErr: "alphanumeric",
		},
		{
			name: "invalid name with special chars",
			field: DynamicField{
				Name:       "Contract-ID",
				Label:      "Contract ID",
				FieldType:  DFTypeText,
				ObjectType: DFObjectTicket,
			},
			wantErr: "alphanumeric",
		},
		{
			name: "missing label",
			field: DynamicField{
				Name:       "ContractID",
				FieldType:  DFTypeText,
				ObjectType: DFObjectTicket,
			},
			wantErr: "label is required",
		},
		{
			name: "invalid field type",
			field: DynamicField{
				Name:       "Test",
				Label:      "Test",
				FieldType:  "InvalidType",
				ObjectType: DFObjectTicket,
			},
			wantErr: "invalid field type",
		},
		{
			name: "invalid object type",
			field: DynamicField{
				Name:       "Test",
				Label:      "Test",
				FieldType:  DFTypeText,
				ObjectType: "InvalidObject",
			},
			wantErr: "invalid object type",
		},
		{
			name: "dropdown without possible values",
			field: DynamicField{
				Name:       "Priority",
				Label:      "Priority",
				FieldType:  DFTypeDropdown,
				ObjectType: DFObjectTicket,
				Config:     &DynamicFieldConfig{},
			},
			wantErr: "requires at least one possible value",
		},
		{
			name: "dropdown with possible values",
			field: DynamicField{
				Name:       "Priority",
				Label:      "Priority",
				FieldType:  DFTypeDropdown,
				ObjectType: DFObjectTicket,
				Config: &DynamicFieldConfig{
					PossibleValues: map[string]string{"low": "Low"},
				},
			},
			wantErr: "",
		},
		{
			name: "multiselect without possible values",
			field: DynamicField{
				Name:       "Tags",
				Label:      "Tags",
				FieldType:  DFTypeMultiselect,
				ObjectType: DFObjectTicket,
				Config:     &DynamicFieldConfig{},
			},
			wantErr: "requires at least one possible value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.field.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDynamicField_GetValueColumn(t *testing.T) {
	tests := []struct {
		fieldType string
		want      string
	}{
		{DFTypeText, "value_text"},
		{DFTypeTextArea, "value_text"},
		{DFTypeDropdown, "value_text"},
		{DFTypeMultiselect, "value_text"},
		{DFTypeCheckbox, "value_int"},
		{DFTypeDate, "value_date"},
		{DFTypeDateTime, "value_date"},
	}

	for _, tt := range tests {
		t.Run(tt.fieldType, func(t *testing.T) {
			df := &DynamicField{FieldType: tt.fieldType}
			assert.Equal(t, tt.want, df.GetValueColumn())
		})
	}
}

func TestDynamicField_IsActive(t *testing.T) {
	df := &DynamicField{ValidID: 1}
	assert.True(t, df.IsActive())

	df.ValidID = 2
	assert.False(t, df.IsActive())
}

func TestDynamicField_IsInternal(t *testing.T) {
	df := &DynamicField{InternalField: 0}
	assert.False(t, df.IsInternal())

	df.InternalField = 1
	assert.True(t, df.IsInternal())
}

func TestValidFieldTypes(t *testing.T) {
	types := ValidFieldTypes()
	assert.Len(t, types, 7)
	assert.Contains(t, types, DFTypeText)
	assert.Contains(t, types, DFTypeTextArea)
	assert.Contains(t, types, DFTypeCheckbox)
	assert.Contains(t, types, DFTypeDropdown)
	assert.Contains(t, types, DFTypeMultiselect)
	assert.Contains(t, types, DFTypeDate)
	assert.Contains(t, types, DFTypeDateTime)
}

func TestValidObjectTypes(t *testing.T) {
	types := ValidObjectTypes()
	assert.Len(t, types, 4)
	assert.Contains(t, types, DFObjectTicket)
	assert.Contains(t, types, DFObjectArticle)
	assert.Contains(t, types, DFObjectCustomerUser)
	assert.Contains(t, types, DFObjectCustomerCompany)
}

func TestGetScreenDefinitions(t *testing.T) {
	screens := GetScreenDefinitions()
	assert.GreaterOrEqual(t, len(screens), 8)

	// Check some key screens exist
	var found map[string]bool = make(map[string]bool)
	for _, s := range screens {
		found[s.Key] = true
	}

	assert.True(t, found["AgentTicketZoom"], "AgentTicketZoom should exist")
	assert.True(t, found["AgentTicketClose"], "AgentTicketClose should exist")
	assert.True(t, found["CustomerTicketMessage"], "CustomerTicketMessage should exist")
}

func TestDynamicFieldConfig_EmptyConfig(t *testing.T) {
	df := &DynamicField{
		Name:       "Test",
		Label:      "Test",
		FieldType:  DFTypeText,
		ObjectType: DFObjectTicket,
		ConfigRaw:  nil,
	}

	err := df.ParseConfig()
	require.NoError(t, err)
	require.NotNil(t, df.Config)
}

func TestDynamicFieldConfig_NilConfig(t *testing.T) {
	df := &DynamicField{
		Name:       "Test",
		Label:      "Test",
		FieldType:  DFTypeText,
		ObjectType: DFObjectTicket,
		Config:     nil,
	}

	err := df.SerializeConfig()
	require.NoError(t, err)
	assert.Nil(t, df.ConfigRaw)
}
