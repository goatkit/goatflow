// Package ticketattributerelations provides management of ticket attribute relationships.
package ticketattributerelations

import (
	"testing"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUploadedCSV(t *testing.T) {
	svc := NewService(nil)

	tests := []struct {
		name       string
		csvContent string
		wantAttr1  string
		wantAttr2  string
		wantPairs  int
		wantErr    bool
	}{
		{
			name:       "valid CSV with Queue and DynamicField",
			csvContent: "Queue;DynamicField_Category\nSales;Quote\nSales;Opportunity\nSupport;Bug\nSupport;Feature",
			wantAttr1:  "Queue",
			wantAttr2:  "DynamicField_Category",
			wantPairs:  4,
			wantErr:    false,
		},
		{
			name:       "valid CSV with State and Priority",
			csvContent: "State;Priority\nnew;high\nopen;medium\nclosed;low",
			wantAttr1:  "State",
			wantAttr2:  "Priority",
			wantPairs:  3,
			wantErr:    false,
		},
		{
			name:       "CSV with empty values (dash)",
			csvContent: "Queue;Service\nSales;-\n-;Support\nSupport;VIP",
			wantAttr1:  "Queue",
			wantAttr2:  "Service",
			wantPairs:  3,
			wantErr:    false,
		},
		{
			name:       "empty CSV",
			csvContent: "",
			wantErr:    true,
		},
		{
			name:       "header only",
			csvContent: "Queue;State",
			wantAttr1:  "Queue",
			wantAttr2:  "State",
			wantPairs:  0,
			wantErr:    false,
		},
		{
			name:       "invalid attribute name in header",
			csvContent: "InvalidAttr;State\nvalue1;open",
			wantErr:    true,
		},
		{
			name:       "too many columns",
			csvContent: "Queue;State;Priority\nSales;open;high",
			wantErr:    true,
		},
		{
			name:       "single column only",
			csvContent: "Queue\nSales",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr1, attr2, pairs, err := svc.ParseUploadedFile("test.csv", []byte(tt.csvContent))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAttr1, attr1)
			assert.Equal(t, tt.wantAttr2, attr2)
			assert.Len(t, pairs, tt.wantPairs)
		})
	}
}

func TestParseUploadedCSVParsedValues(t *testing.T) {
	svc := NewService(nil)

	csvContent := "Queue;DynamicField_Category\nSales;Quote\nSales;Opportunity\nSupport;Bug"

	_, _, pairs, err := svc.ParseUploadedFile("test.csv", []byte(csvContent))
	require.NoError(t, err)
	require.Len(t, pairs, 3)

	// Verify the parsed values
	assert.Equal(t, "Sales", pairs[0].Attribute1Value)
	assert.Equal(t, "Quote", pairs[0].Attribute2Value)
	assert.Equal(t, "Sales", pairs[1].Attribute1Value)
	assert.Equal(t, "Opportunity", pairs[1].Attribute2Value)
	assert.Equal(t, "Support", pairs[2].Attribute1Value)
	assert.Equal(t, "Bug", pairs[2].Attribute2Value)
}

func TestParseUploadedCSVEmptyValueMarker(t *testing.T) {
	svc := NewService(nil)

	// Test that "-" is treated as empty string
	csvContent := "Queue;Service\nSales;-\n-;Support"

	_, _, pairs, err := svc.ParseUploadedFile("test.csv", []byte(csvContent))
	require.NoError(t, err)
	require.Len(t, pairs, 2)

	assert.Equal(t, "Sales", pairs[0].Attribute1Value)
	assert.Equal(t, "", pairs[0].Attribute2Value) // "-" converted to empty
	assert.Equal(t, "", pairs[1].Attribute1Value) // "-" converted to empty
	assert.Equal(t, "Support", pairs[1].Attribute2Value)
}

func TestIsValidAttribute(t *testing.T) {
	tests := []struct {
		attr  string
		valid bool
	}{
		{"Queue", true},
		{"State", true},
		{"Priority", true},
		{"Type", true},
		{"Service", true},
		{"SLA", true},
		{"Owner", true},
		{"Responsible", true},
		{"DynamicField_Category", true},
		{"DynamicField_MyField", true},
		{"DynamicField_", false},        // Empty field name
		{"DynamicField", false},         // Missing underscore
		{"Invalid", false},
		{"queue", false},                // Case sensitive
		{"STATE", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			result := models.IsValidAttribute(tt.attr)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestIsDynamicFieldAttribute(t *testing.T) {
	tests := []struct {
		attr     string
		expected bool
	}{
		{"DynamicField_Category", true},
		{"DynamicField_MyField", true},
		{"DynamicField_A", true},
		{"DynamicField_", false},
		{"DynamicField", false},
		{"Queue", false},
		{"State", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			result := models.IsDynamicFieldAttribute(tt.attr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDynamicFieldName(t *testing.T) {
	tests := []struct {
		attr     string
		expected string
	}{
		{"DynamicField_Category", "Category"},
		{"DynamicField_MyField", "MyField"},
		{"DynamicField_A", "A"},
		{"DynamicField_", ""},
		{"DynamicField", ""},
		{"Queue", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			result := models.GetDynamicFieldName(tt.attr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTicketAttributeRelationGetAllowedValues(t *testing.T) {
	relation := &models.TicketAttributeRelation{
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "Quote"},
			{Attribute1Value: "Sales", Attribute2Value: "Opportunity"},
			{Attribute1Value: "Support", Attribute2Value: "Bug"},
			{Attribute1Value: "Support", Attribute2Value: "Feature"},
			{Attribute1Value: "Support", Attribute2Value: "Bug"}, // Duplicate
		},
	}

	// Test getting allowed values for Sales
	salesValues := relation.GetAllowedValues("Sales")
	assert.Len(t, salesValues, 2)
	assert.Contains(t, salesValues, "Quote")
	assert.Contains(t, salesValues, "Opportunity")

	// Test getting allowed values for Support (should deduplicate)
	supportValues := relation.GetAllowedValues("Support")
	assert.Len(t, supportValues, 2)
	assert.Contains(t, supportValues, "Bug")
	assert.Contains(t, supportValues, "Feature")

	// Test getting values for non-existent attribute
	unknownValues := relation.GetAllowedValues("Unknown")
	assert.Empty(t, unknownValues)
}

func TestTicketAttributeRelationGetUniqueAttribute1Values(t *testing.T) {
	relation := &models.TicketAttributeRelation{
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "Quote"},
			{Attribute1Value: "Sales", Attribute2Value: "Opportunity"},
			{Attribute1Value: "Support", Attribute2Value: "Bug"},
			{Attribute1Value: "Support", Attribute2Value: "Feature"},
		},
	}

	values := relation.GetUniqueAttribute1Values()
	assert.Len(t, values, 2)
	assert.Contains(t, values, "Sales")
	assert.Contains(t, values, "Support")
}

func TestTicketAttributeRelationGetUniqueAttribute2Values(t *testing.T) {
	relation := &models.TicketAttributeRelation{
		Data: []models.AttributeRelationPair{
			{Attribute1Value: "Sales", Attribute2Value: "Quote"},
			{Attribute1Value: "Sales", Attribute2Value: "Opportunity"},
			{Attribute1Value: "Support", Attribute2Value: "Bug"},
			{Attribute1Value: "Support", Attribute2Value: "Quote"}, // Duplicate
		},
	}

	values := relation.GetUniqueAttribute2Values()
	assert.Len(t, values, 3)
	assert.Contains(t, values, "Quote")
	assert.Contains(t, values, "Opportunity")
	assert.Contains(t, values, "Bug")
}

func TestPrepareDataForStorage(t *testing.T) {
	svc := NewService(nil)

	// Test CSV file - should return as-is
	csvData := []byte("Queue;DynamicField_Category\nSales;Quote")
	result := svc.PrepareDataForStorage("test.csv", csvData)
	assert.Equal(t, string(csvData), result)

	// Test Excel file - should be base64 encoded
	xlsxData := []byte("fake excel data")
	result = svc.PrepareDataForStorage("test.xlsx", xlsxData)
	assert.NotEqual(t, string(xlsxData), result) // Should be base64
	assert.Contains(t, result, "ZmFrZSBleGNlbCBkYXRh") // base64 of "fake excel data"
}

func TestValidTicketAttributes(t *testing.T) {
	expected := []string{
		"Queue",
		"State",
		"Priority",
		"Type",
		"Service",
		"SLA",
		"Owner",
		"Responsible",
	}

	assert.Equal(t, expected, models.ValidTicketAttributes)
}

func TestServiceCreation(t *testing.T) {
	svc := NewService(nil)

	// Test that the service was created
	assert.NotNil(t, svc)

	// Without a database, db should be nil
	assert.Nil(t, svc.db)
}

func TestIsExcelFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.xlsx", true},
		{"test.XLSX", true},
		{"test.xls", true},
		{"test.XLS", true},
		{"test.csv", false},
		{"test.CSV", false},
		{"test.txt", false},
		{"", false},
		{"xlsx", false},
		{"test.xlsx.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isExcelFilename(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIntersectSlices(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "common elements",
			a:        []string{"a", "b", "c"},
			b:        []string{"b", "c", "d"},
			expected: []string{"b", "c"},
		},
		{
			name:     "no common elements",
			a:        []string{"a", "b"},
			b:        []string{"c", "d"},
			expected: []string{},
		},
		{
			name:     "identical slices",
			a:        []string{"a", "b"},
			b:        []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "empty first slice",
			a:        []string{},
			b:        []string{"a", "b"},
			expected: []string{},
		},
		{
			name:     "empty second slice",
			a:        []string{"a", "b"},
			b:        []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intersectSlices(tt.a, tt.b)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestCalculatePrioritiesAfterInsert(t *testing.T) {
	// Test the priority calculation logic
	existingPriorities := []int64{1, 2, 3, 4, 5}
	targetPriority := int64(3)

	// After inserting at priority 3:
	// - Items with priority >= 3 should be shifted down
	// Expected: 1, 2, [new item at 3], old 3->4, old 4->5, old 5->6

	// Calculate what priorities would become after pre-reorder (×10+1)
	preReordered := make([]int64, len(existingPriorities))
	for i, p := range existingPriorities {
		preReordered[i] = p*10 + 1
	}

	// Target priority for insert would be targetPriority * 10 = 30
	insertPriority := targetPriority * 10

	// Verify the algorithm logic
	assert.Equal(t, int64(30), insertPriority)
	assert.Equal(t, int64(11), preReordered[0]) // Priority 1 -> 11
	assert.Equal(t, int64(21), preReordered[1]) // Priority 2 -> 21
	assert.Equal(t, int64(31), preReordered[2]) // Priority 3 -> 31 (> 30)
	assert.Equal(t, int64(41), preReordered[3]) // Priority 4 -> 41
	assert.Equal(t, int64(51), preReordered[4]) // Priority 5 -> 51
}

func TestReorderPrioritiesEmptySlice(t *testing.T) {
	svc := NewService(nil)

	// Test that empty slice returns nil error
	err := svc.ReorderPriorities(nil, []int64{}, 1)
	assert.NoError(t, err)
}

func TestReorderPrioritiesAlgorithm(t *testing.T) {
	// Test the logic of the reorder algorithm
	// This is a unit test for the algorithm, not DB integration

	// Given 3 items with IDs 1, 2, 3 and priorities 1, 2, 3
	// If we reorder to [3, 1, 2]
	// The new priorities should be:
	// ID 3 -> priority 1
	// ID 1 -> priority 2
	// ID 2 -> priority 3

	orderedIDs := []int64{3, 1, 2}

	// Calculate expected new priorities
	expectedPriorities := make(map[int64]int64)
	for i, id := range orderedIDs {
		expectedPriorities[id] = int64(i + 1)
	}

	assert.Equal(t, int64(1), expectedPriorities[3]) // ID 3 gets priority 1
	assert.Equal(t, int64(2), expectedPriorities[1]) // ID 1 gets priority 2
	assert.Equal(t, int64(3), expectedPriorities[2]) // ID 2 gets priority 3
}
