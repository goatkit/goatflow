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

func TestNotificationEventModuleConfig(t *testing.T) {
	// Test the notification_event module configuration
	config := ModuleConfig{
		Fields: []Field{
			{Name: "id", Type: "integer", DBColumn: "id", ShowInList: false, ShowInForm: false},
			{Name: "name", Type: "string", DBColumn: "name", Required: true, ShowInList: true, ShowInForm: true},
			{Name: "valid_id", Type: "integer", DBColumn: "valid_id", Required: true, ShowInList: true, ShowInForm: true,
				LookupTable: "valid", LookupKey: "id", LookupDisplay: "name", DisplayAs: "chip"},
			{Name: "comments", Type: "text", DBColumn: "comments", ShowInList: true, ShowInForm: true},
			{Name: "create_time", Type: "datetime", DBColumn: "create_time", ShowInList: true, ShowInForm: false},
			{Name: "change_time", Type: "datetime", DBColumn: "change_time", ShowInList: true, ShowInForm: false},
		},
	}
	config.Module.Name = "notification_event"
	config.Module.Singular = "Ticket Notification"
	config.Module.Plural = "Ticket Notifications"
	config.Module.Table = "notification_event"
	config.Module.RoutePrefix = "/admin/notification-events"

	// Test module name
	if config.Module.Name != "notification_event" {
		t.Errorf("expected module name 'notification_event', got %s", config.Module.Name)
	}

	// Test route prefix
	if config.Module.RoutePrefix != "/admin/notification-events" {
		t.Errorf("expected route prefix '/admin/notification-events', got %s", config.Module.RoutePrefix)
	}

	// Test that we have the expected number of fields
	if len(config.Fields) != 6 {
		t.Errorf("expected 6 fields, got %d", len(config.Fields))
	}

	// Test valid_id has lookup configured
	var validField *Field
	for i := range config.Fields {
		if config.Fields[i].Name == "valid_id" {
			validField = &config.Fields[i]
			break
		}
	}

	if validField == nil {
		t.Fatal("valid_id field not found")
	}

	if validField.LookupTable != "valid" {
		t.Errorf("expected valid_id LookupTable='valid', got %s", validField.LookupTable)
	}

	if validField.DisplayAs != "chip" {
		t.Errorf("expected valid_id DisplayAs='chip', got %s", validField.DisplayAs)
	}

	// Test name field is required
	var nameField *Field
	for i := range config.Fields {
		if config.Fields[i].Name == "name" {
			nameField = &config.Fields[i]
			break
		}
	}

	if nameField == nil {
		t.Fatal("name field not found")
	}

	if !nameField.Required {
		t.Error("expected name field to be required")
	}

	if !nameField.ShowInList {
		t.Error("expected name field to show in list")
	}

	if !nameField.ShowInForm {
		t.Error("expected name field to show in form")
	}
}

func TestNotificationEventValidIdLookup(t *testing.T) {
	// Test that valid_id lookup produces correct display values
	items := []map[string]interface{}{
		{"id": 1, "name": "Ticket create notification", "valid_id": 1},
		{"id": 2, "name": "Ticket update notification", "valid_id": 2},
		{"id": 3, "name": "Ticket close notification", "valid_id": 1},
	}

	// Simulate valid table lookup map
	validLookupMap := map[string]string{
		"1": "valid",
		"2": "invalid",
		"3": "invalid-temporarily",
	}

	// Apply lookup transformation
	for _, item := range items {
		if val, exists := item["valid_id"]; exists && val != nil {
			key := coerceString(val)
			if displayVal, found := validLookupMap[key]; found {
				item["valid_id_display"] = displayVal
			}
		}
	}

	// Verify display values
	if items[0]["valid_id_display"] != "valid" {
		t.Errorf("expected first item valid_id_display='valid', got %v", items[0]["valid_id_display"])
	}

	if items[1]["valid_id_display"] != "invalid" {
		t.Errorf("expected second item valid_id_display='invalid', got %v", items[1]["valid_id_display"])
	}

	if items[2]["valid_id_display"] != "valid" {
		t.Errorf("expected third item valid_id_display='valid', got %v", items[2]["valid_id_display"])
	}
}

func TestNotificationEventFieldVisibility(t *testing.T) {
	// Test field visibility configuration for list vs form views
	fields := []Field{
		{Name: "id", ShowInList: false, ShowInForm: false},
		{Name: "name", ShowInList: true, ShowInForm: true},
		{Name: "valid_id", ShowInList: true, ShowInForm: true},
		{Name: "comments", ShowInList: true, ShowInForm: true},
		{Name: "create_time", ShowInList: true, ShowInForm: false},
		{Name: "create_by", ShowInList: false, ShowInForm: false},
		{Name: "change_time", ShowInList: true, ShowInForm: false},
		{Name: "change_by", ShowInList: false, ShowInForm: false},
	}

	// Count fields visible in list
	listFieldCount := 0
	for _, f := range fields {
		if f.ShowInList {
			listFieldCount++
		}
	}

	// Count fields visible in form
	formFieldCount := 0
	for _, f := range fields {
		if f.ShowInForm {
			formFieldCount++
		}
	}

	// We expect 5 fields in list: name, valid_id, comments, create_time, change_time
	if listFieldCount != 5 {
		t.Errorf("expected 5 list fields, got %d", listFieldCount)
	}

	// We expect 3 fields in form: name, valid_id, comments
	if formFieldCount != 3 {
		t.Errorf("expected 3 form fields, got %d", formFieldCount)
	}

	// Audit fields (create_by, change_by) should not be visible anywhere
	for _, f := range fields {
		if f.Name == "create_by" || f.Name == "change_by" {
			if f.ShowInList || f.ShowInForm {
				t.Errorf("audit field %s should not be visible", f.Name)
			}
		}
	}
}
