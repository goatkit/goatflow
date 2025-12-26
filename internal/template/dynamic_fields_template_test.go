package template

import (
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/stretchr/testify/require"
)

// TestDynamicFieldFormCreate tests the create form uses hx-post
func TestDynamicFieldFormCreate(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	ctx := pongo2.Context{
		"IsNew": true,
		"Field": map[string]interface{}{
			"ID":        0,
			"Name":      "",
			"Label":     "",
			"FieldType": "Text",
			"Config": map[string]interface{}{
				"DefaultValue": "",
				"MaxLength":    0,
			},
			"ValidID": 1,
		},
		"FieldTypes":   []string{"Text", "Textarea", "Dropdown", "Checkbox", "Date", "DateTime"},
		"ValidOptions": []map[string]interface{}{{"ID": 1, "Name": "valid"}, {"ID": 2, "Name": "invalid"}},
		"t":            func(key string, args ...interface{}) string { return key },
		"User":         map[string]interface{}{"Username": "admin", "IsAdmin": true},
		"CurrentYear":  2025,
	}

	html, err := helper.RenderTemplate("pages/admin/dynamic_field_form.pongo2", ctx)
	require.NoError(t, err, "Template should render without error")

	asserter := NewHTMLAsserter(t, html)

	// Create form should use hx-post
	asserter.HasHTMXPost("/admin/api/dynamic-fields")

	// Create form should NOT have hx-put
	asserter.HasNoHTMXPut()

	// Form action should match
	asserter.HasFormAction("/admin/api/dynamic-fields")

	// Title should indicate create
	asserter.Contains("Create Dynamic Field")
}

// TestDynamicFieldFormEdit tests the edit form uses hx-put
func TestDynamicFieldFormEdit(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	ctx := pongo2.Context{
		"IsNew": false,
		"Field": map[string]interface{}{
			"ID":        42,
			"Name":      "test_field",
			"Label":     "Test Field",
			"FieldType": "Text",
			"Config": map[string]interface{}{
				"DefaultValue": "",
				"MaxLength":    255,
			},
			"ValidID": 1,
		},
		"FieldTypes":   []string{"Text", "Textarea", "Dropdown", "Checkbox", "Date", "DateTime"},
		"ValidOptions": []map[string]interface{}{{"ID": 1, "Name": "valid"}, {"ID": 2, "Name": "invalid"}},
		"t":            func(key string, args ...interface{}) string { return key },
		"User":         map[string]interface{}{"Username": "admin", "IsAdmin": true},
		"CurrentYear":  2025,
	}

	html, err := helper.RenderTemplate("pages/admin/dynamic_field_form.pongo2", ctx)
	require.NoError(t, err, "Template should render without error")

	asserter := NewHTMLAsserter(t, html)

	// Edit form should use hx-put with field ID
	asserter.HasHTMXPut("/admin/api/dynamic-fields/42")

	// Edit form should NOT have hx-post
	asserter.HasNoHTMXPost()

	// Form action should include ID
	asserter.HasFormAction("/admin/api/dynamic-fields/42")

	// Title should indicate edit
	asserter.Contains("Edit Dynamic Field")
}

// TestDynamicFieldsListDeletePath tests delete buttons use correct API path
func TestDynamicFieldsListDeletePath(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	ctx := pongo2.Context{
		"DynamicFields": []map[string]interface{}{
			{
				"ID":        1,
				"Name":      "field_one",
				"Label":     "Field One",
				"FieldType": "Text",
				"ValidID":   1,
			},
		},
		"t":           func(key string, args ...interface{}) string { return key },
		"User":        map[string]interface{}{"Username": "admin", "IsAdmin": true},
		"CurrentYear": 2025,
	}

	html, err := helper.RenderTemplate("pages/admin/dynamic_fields.pongo2", ctx)
	require.NoError(t, err, "Template should render without error")

	asserter := NewHTMLAsserter(t, html)

	// Delete should use /admin/api path
	asserter.Contains("/admin/api/dynamic-fields/")
}

// TestDynamicFieldScreensAPIPath tests screen config uses correct API path
func TestDynamicFieldScreensAPIPath(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	ctx := pongo2.Context{
		"Field": map[string]interface{}{
			"ID":        5,
			"Name":      "test_field",
			"Label":     "Test Field",
			"FieldType": "Text",
		},
		"Screens": []map[string]interface{}{
			{"ID": 1, "Name": "AgentTicketCreate"},
		},
		"ScreenConfigs": []map[string]interface{}{},
		"t":             func(key string, args ...interface{}) string { return key },
		"User":          map[string]interface{}{"Username": "admin", "IsAdmin": true},
		"CurrentYear":   2025,
	}

	html, err := helper.RenderTemplate("pages/admin/dynamic_field_screens.pongo2", ctx)
	require.NoError(t, err, "Template should render without error")

	asserter := NewHTMLAsserter(t, html)

	// Screen config API should use /admin/api path (singular "screen")
	asserter.Contains("/admin/api/dynamic-fields/")
	asserter.Contains("/screen")
}
