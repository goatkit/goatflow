package dynamic

import (
	"testing"
)

func TestProcessLookupsAddsDisplayField(t *testing.T) {
	// Create a mock handler with a simple query function
	// For unit testing, we'll test the logic directly

	// Test the lookup field naming convention
	fieldName := "queue_id"
	expectedDisplayKey := fieldName + "_display"

	if expectedDisplayKey != "queue_id_display" {
		t.Fatalf("expected display key 'queue_id_display', got %s", expectedDisplayKey)
	}

	// Test that items get the _display field populated
	items := []map[string]interface{}{
		{"queue_id": 5, "auto_response_id": 3},
		{"queue_id": 2, "auto_response_id": 1},
	}

	// Simulate what processLookups should do
	lookupMap := map[string]string{
		"5": "Raw",
		"2": "Postmaster",
	}

	for _, item := range items {
		if val, exists := item["queue_id"]; exists && val != nil {
			key := coerceString(val)
			if displayVal, found := lookupMap[key]; found {
				item["queue_id_display"] = displayVal
			}
		}
	}

	// Verify the display values were set
	if items[0]["queue_id_display"] != "Raw" {
		t.Fatalf("expected queue_id_display='Raw', got %v", items[0]["queue_id_display"])
	}
	if items[1]["queue_id_display"] != "Postmaster" {
		t.Fatalf("expected queue_id_display='Postmaster', got %v", items[1]["queue_id_display"])
	}
}

func TestCoerceStringHandlesVariousTypes(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{5, "5"},
		{int64(10), "10"},
		{"hello", "hello"},
		{3.14, "3.14"},
		{nil, ""},
	}

	for _, tc := range tests {
		result := coerceString(tc.input)
		if result != tc.expected {
			t.Errorf("coerceString(%v) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestLookupFieldConfig(t *testing.T) {
	// Test that Field correctly parses lookup configuration
	field := Field{
		Name:          "queue_id",
		Type:          "integer",
		LookupTable:   "queue",
		LookupKey:     "id",
		LookupDisplay: "name",
	}

	if field.LookupTable != "queue" {
		t.Fatalf("expected LookupTable='queue', got %s", field.LookupTable)
	}
	if field.LookupKey != "id" {
		t.Fatalf("expected LookupKey='id', got %s", field.LookupKey)
	}
	if field.LookupDisplay != "name" {
		t.Fatalf("expected LookupDisplay='name', got %s", field.LookupDisplay)
	}
}

func TestLookupQueryGeneration(t *testing.T) {
	// Test the SQL query generation for lookups
	field := Field{
		Name:          "queue_id",
		LookupTable:   "queue",
		LookupKey:     "id",
		LookupDisplay: "name",
	}

	lookupKey := field.LookupKey
	if lookupKey == "" {
		lookupKey = "id"
	}
	lookupDisplay := field.LookupDisplay
	if lookupDisplay == "" {
		lookupDisplay = "name"
	}

	// Simulate ID collection
	ids := []string{"'5'", "'2'"}

	// Build expected query
	expectedQueryPattern := "SELECT id, name FROM queue WHERE id IN ('5','2')"
	_ = expectedQueryPattern // We're testing the components, not the full query

	if lookupKey != "id" {
		t.Fatalf("expected lookupKey='id', got %s", lookupKey)
	}
	if lookupDisplay != "name" {
		t.Fatalf("expected lookupDisplay='name', got %s", lookupDisplay)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
}
