package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDynamicFieldFiltersFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    map[string][]string
		expected []DynamicFieldFilter
	}{
		{
			name:     "empty query",
			query:    map[string][]string{},
			expected: nil,
		},
		{
			name: "non-df parameters ignored",
			query: map[string][]string{
				"status": {"open"},
				"queue":  {"1"},
			},
			expected: nil,
		},
		{
			name: "simple equals filter",
			query: map[string][]string{
				"df_CustomerType": {"VIP"},
			},
			expected: []DynamicFieldFilter{
				{FieldName: "CustomerType", Operator: "eq", Value: "VIP"},
			},
		},
		{
			name: "contains operator",
			query: map[string][]string{
				"df_Notes_contains": {"important"},
			},
			expected: []DynamicFieldFilter{
				{FieldName: "Notes", Operator: "contains", Value: "important"},
			},
		},
		{
			name: "greater than operator",
			query: map[string][]string{
				"df_Amount_gt": {"1000"},
			},
			expected: []DynamicFieldFilter{
				{FieldName: "Amount", Operator: "gt", Value: "1000"},
			},
		},
		{
			name: "date range with gte and lte",
			query: map[string][]string{
				"df_DueDate_gte": {"2024-01-01"},
				"df_DueDate_lte": {"2024-12-31"},
			},
			expected: []DynamicFieldFilter{
				{FieldName: "DueDate", Operator: "gte", Value: "2024-01-01"},
				{FieldName: "DueDate", Operator: "lte", Value: "2024-12-31"},
			},
		},
		{
			name: "not equals operator",
			query: map[string][]string{
				"df_Status_ne": {"cancelled"},
			},
			expected: []DynamicFieldFilter{
				{FieldName: "Status", Operator: "ne", Value: "cancelled"},
			},
		},
		{
			name: "in operator",
			query: map[string][]string{
				"df_Priority_in": {"high,critical"},
			},
			expected: []DynamicFieldFilter{
				{FieldName: "Priority", Operator: "in", Value: "high,critical"},
			},
		},
		{
			name: "empty value ignored",
			query: map[string][]string{
				"df_EmptyField": {""},
			},
			expected: nil,
		},
		{
			name: "multiple filters",
			query: map[string][]string{
				"df_Type":        {"bug"},
				"df_Severity_gt": {"3"},
				"df_Assignee":    {"john"},
				"status":         {"open"}, // should be ignored
			},
			expected: []DynamicFieldFilter{
				{FieldName: "Type", Operator: "eq", Value: "bug"},
				{FieldName: "Severity", Operator: "gt", Value: "3"},
				{FieldName: "Assignee", Operator: "eq", Value: "john"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDynamicFieldFiltersFromQuery(tt.query)

			if tt.expected == nil {
				assert.Empty(t, result)
				return
			}

			assert.Equal(t, len(tt.expected), len(result), "expected %d filters, got %d", len(tt.expected), len(result))

			// Check each expected filter is present (order may vary due to map iteration)
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual.FieldName == expected.FieldName &&
						actual.Operator == expected.Operator &&
						actual.Value == expected.Value {
						found = true
						break
					}
				}
				assert.True(t, found, "expected filter not found: %+v", expected)
			}
		})
	}
}

func TestBuildDynamicFieldFilterSQL_EmptyFilters(t *testing.T) {
	sql, args, err := BuildDynamicFieldFilterSQL(nil, 1)
	assert.NoError(t, err)
	assert.Empty(t, sql)
	assert.Empty(t, args)

	sql, args, err = BuildDynamicFieldFilterSQL([]DynamicFieldFilter{}, 1)
	assert.NoError(t, err)
	assert.Empty(t, sql)
	assert.Empty(t, args)
}

func TestSearchableDynamicField_Options(t *testing.T) {
	df := DynamicField{
		ID:        1,
		Name:      "Priority",
		Label:     "Priority",
		FieldType: DFTypeDropdown,
		Config: &DynamicFieldConfig{
			PossibleValues: map[string]string{
				"low":    "Low Priority",
				"medium": "Medium Priority",
				"high":   "High Priority",
			},
		},
	}

	sdf := SearchableDynamicField{DynamicField: df}

	// Simulate the option population from GetFieldsForSearch
	if df.FieldType == DFTypeDropdown || df.FieldType == DFTypeMultiselect {
		if df.Config != nil && df.Config.PossibleValues != nil {
			for key, value := range df.Config.PossibleValues {
				sdf.Options = append(sdf.Options, map[string]string{
					"key":   key,
					"value": value,
				})
			}
		}
	}

	assert.Equal(t, 3, len(sdf.Options))

	// Verify options contain expected values
	keys := make(map[string]bool)
	for _, opt := range sdf.Options {
		keys[opt["key"]] = true
	}
	assert.True(t, keys["low"])
	assert.True(t, keys["medium"])
	assert.True(t, keys["high"])
}
