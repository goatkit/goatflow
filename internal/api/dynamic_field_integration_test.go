//go:build integration

package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

func TestDynamicFieldIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Setup: Create test dynamic fields
	testFields := setupTestDynamicFields(t, db)
	defer cleanupTestDynamicFields(t, db, testFields)

	t.Run("DynamicFieldCRUDWorkflow", func(t *testing.T) {
		// Test creating, updating, and reading dynamic field values
		testDynamicFieldCRUDWorkflow(t, db, testFields)
	})

	t.Run("TicketCreateWithDynamicFields", func(t *testing.T) {
		testTicketCreateWithDynamicFields(t, db, testFields)
	})

	t.Run("AllFieldTypesValuePersistence", func(t *testing.T) {
		testAllFieldTypesValuePersistence(t, db, testFields)
	})

	t.Run("ScreenConfigFiltering", func(t *testing.T) {
		testScreenConfigFiltering(t, db, testFields)
	})

	t.Run("MultiselectFieldPersistence", func(t *testing.T) {
		testMultiselectFieldPersistence(t, db, testFields)
	})
}

type testDynamicFieldSet struct {
	textField        *DynamicField
	textAreaField    *DynamicField
	checkboxField    *DynamicField
	dropdownField    *DynamicField
	multiselectField *DynamicField
	dateField        *DynamicField
	dateTimeField    *DynamicField
}

func setupTestDynamicFields(t *testing.T, db *sql.DB) *testDynamicFieldSet {
	t.Helper()

	fields := &testDynamicFieldSet{}

	// Cleanup any leftover test data first
	db.Exec(`DELETE FROM dynamic_field_screen_config WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTest%')`)
	db.Exec(`DELETE FROM dynamic_field_value WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTest%')`)
	db.Exec(`DELETE FROM dynamic_field WHERE name LIKE 'IntTest%'`)

	// Create Text field
	_, err := db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestText', 'Integration Test Text', 100, 'Text', 'Ticket', 'DefaultValue: ""\nMaxLength: 200', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Create TextArea field
	_, err = db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestTextArea', 'Integration Test TextArea', 200, 'TextArea', 'Ticket', 'Rows: 5', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Create Checkbox field
	_, err = db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestCheckbox', 'Integration Test Checkbox', 300, 'Checkbox', 'Ticket', 'DefaultValue: 0', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Create Dropdown field
	_, err = db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestDropdown', 'Integration Test Dropdown', 400, 'Dropdown', 'Ticket', 'PossibleValues:\n  low: Low\n  medium: Medium\n  high: High', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Create Multiselect field
	_, err = db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestMultiselect', 'Integration Test Multiselect', 500, 'Multiselect', 'Ticket', 'PossibleValues:\n  opt1: Option 1\n  opt2: Option 2\n  opt3: Option 3', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Create Date field
	_, err = db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestDate', 'Integration Test Date', 600, 'Date', 'Ticket', 'YearsInPast: 5\nYearsInFuture: 5', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Create DateTime field
	_, err = db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestDateTime', 'Integration Test DateTime', 700, 'DateTime', 'Ticket', 'YearsInPast: 5\nYearsInFuture: 5', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Add screen config entries for AgentTicketPhone screen
	_, err = db.Exec(`INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by)
		SELECT id, 'AgentTicketPhone', 1, NOW(), 1, NOW(), 1 FROM dynamic_field WHERE name LIKE 'IntTest%'`)
	require.NoError(t, err)

	return fields
}

func cleanupTestDynamicFields(t *testing.T, db *sql.DB, fields *testDynamicFieldSet) {
	t.Helper()

	// Delete screen configs
	_, _ = db.Exec(`DELETE FROM dynamic_field_screen_config WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTest%')`)

	// Delete values
	_, _ = db.Exec(`DELETE FROM dynamic_field_value WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTest%')`)

	// Delete fields
	_, _ = db.Exec(`DELETE FROM dynamic_field WHERE name LIKE 'IntTest%'`)
}

func testDynamicFieldCRUDWorkflow(t *testing.T, db *sql.DB, fields *testDynamicFieldSet) {
	// Get a test field ID
	var fieldID int
	err := db.QueryRow(`SELECT id FROM dynamic_field WHERE name = 'IntTestText' LIMIT 1`).Scan(&fieldID)
	require.NoError(t, err)
	require.NotZero(t, fieldID, "Test field should exist")

	// Create a test object ID (simulating a ticket)
	testObjectID := int64(99999)

	// Test setting a value
	textValue := "Integration Test Value"
	value := &DynamicFieldValue{
		FieldID:   fieldID,
		ObjectID:  testObjectID,
		ValueText: &textValue,
	}
	err = SetDynamicFieldValue(value)
	require.NoError(t, err)

	// Test reading the value back
	values, err := GetDynamicFieldValues(testObjectID)
	require.NoError(t, err)
	require.Len(t, values, 1, "Should have one value")
	assert.Equal(t, textValue, *values[0].ValueText)
	assert.Equal(t, fieldID, values[0].FieldID)

	// Test updating the value
	newTextValue := "Updated Integration Test Value"
	value.ValueText = &newTextValue
	err = SetDynamicFieldValue(value)
	require.NoError(t, err)

	// Verify update
	values, err = GetDynamicFieldValues(testObjectID)
	require.NoError(t, err)
	require.Len(t, values, 1)
	assert.Equal(t, newTextValue, *values[0].ValueText)

	// Cleanup
	db.Exec(`DELETE FROM dynamic_field_value WHERE object_id = ?`, testObjectID)
}

func testTicketCreateWithDynamicFields(t *testing.T, db *sql.DB, fields *testDynamicFieldSet) {
	router := setupIntegrationTestRouter(t)

	// Create form data with dynamic fields
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Standard ticket fields
	writer.WriteField("subject", "Dynamic Field Test Ticket")
	writer.WriteField("body", "This is a test ticket with dynamic fields")
	writer.WriteField("customer_user_id", "test@example.com")
	writer.WriteField("queue_id", "1")
	writer.WriteField("priority", "3")
	writer.WriteField("state_id", "1")

	// Dynamic fields
	writer.WriteField("DynamicField_IntTestText", "Test Text Value")
	writer.WriteField("DynamicField_IntTestCheckbox", "1")
	writer.WriteField("DynamicField_IntTestDropdown", "medium")
	writer.WriteField("DynamicField_IntTestDate", "2025-06-15")

	writer.Close()

	req := httptest.NewRequest("POST", "/api/tickets", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Logf("Response body: %s", w.Body.String())
	}
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
		"Expected 200 or 201, got %d", w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Extract ticket ID and verify dynamic fields were saved
	if data, ok := response["data"].(map[string]interface{}); ok {
		if ticketID, ok := data["id"].(float64); ok {
			// Verify dynamic field values were persisted
			values, err := GetDynamicFieldValues(int64(ticketID))
			require.NoError(t, err)
			assert.NotEmpty(t, values, "Dynamic field values should have been saved")
		}
	}
}

func testAllFieldTypesValuePersistence(t *testing.T, db *sql.DB, fields *testDynamicFieldSet) {
	testObjectID := int64(88888)

	// Get field IDs for each type
	fieldIDs := make(map[string]int)
	for _, name := range []string{"IntTestText", "IntTestTextArea", "IntTestCheckbox", "IntTestDropdown", "IntTestDate", "IntTestDateTime"} {
		var id int
		err := db.QueryRow(`SELECT id FROM dynamic_field WHERE name = ? LIMIT 1`, name).Scan(&id)
		require.NoError(t, err, "Field %s should exist", name)
		fieldIDs[name] = id
	}

	// Test Text field
	t.Run("Text", func(t *testing.T) {
		textValue := "Test String Value"
		value := &DynamicFieldValue{
			FieldID:   fieldIDs["IntTestText"],
			ObjectID:  testObjectID,
			ValueText: &textValue,
		}
		err := SetDynamicFieldValue(value)
		require.NoError(t, err)

		values, _ := GetDynamicFieldValues(testObjectID)
		found := false
		for _, v := range values {
			if v.FieldID == fieldIDs["IntTestText"] {
				assert.Equal(t, textValue, *v.ValueText)
				found = true
			}
		}
		assert.True(t, found, "Text field value should be found")
	})

	// Test TextArea field
	t.Run("TextArea", func(t *testing.T) {
		textAreaValue := "This is a multi-line\ntext area value\nwith several lines."
		value := &DynamicFieldValue{
			FieldID:   fieldIDs["IntTestTextArea"],
			ObjectID:  testObjectID,
			ValueText: &textAreaValue,
		}
		err := SetDynamicFieldValue(value)
		require.NoError(t, err)

		values, _ := GetDynamicFieldValues(testObjectID)
		for _, v := range values {
			if v.FieldID == fieldIDs["IntTestTextArea"] {
				assert.Equal(t, textAreaValue, *v.ValueText)
			}
		}
	})

	// Test Checkbox field
	t.Run("Checkbox", func(t *testing.T) {
		var intVal int64 = 1
		value := &DynamicFieldValue{
			FieldID:  fieldIDs["IntTestCheckbox"],
			ObjectID: testObjectID,
			ValueInt: &intVal,
		}
		err := SetDynamicFieldValue(value)
		require.NoError(t, err)

		values, _ := GetDynamicFieldValues(testObjectID)
		for _, v := range values {
			if v.FieldID == fieldIDs["IntTestCheckbox"] {
				assert.Equal(t, int64(1), *v.ValueInt)
			}
		}
	})

	// Test Dropdown field
	t.Run("Dropdown", func(t *testing.T) {
		dropdownValue := "high"
		value := &DynamicFieldValue{
			FieldID:   fieldIDs["IntTestDropdown"],
			ObjectID:  testObjectID,
			ValueText: &dropdownValue,
		}
		err := SetDynamicFieldValue(value)
		require.NoError(t, err)

		values, _ := GetDynamicFieldValues(testObjectID)
		for _, v := range values {
			if v.FieldID == fieldIDs["IntTestDropdown"] {
				assert.Equal(t, dropdownValue, *v.ValueText)
			}
		}
	})

	// Test Date field
	t.Run("Date", func(t *testing.T) {
		dateValue := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
		value := &DynamicFieldValue{
			FieldID:   fieldIDs["IntTestDate"],
			ObjectID:  testObjectID,
			ValueDate: &dateValue,
		}
		err := SetDynamicFieldValue(value)
		require.NoError(t, err)

		values, _ := GetDynamicFieldValues(testObjectID)
		for _, v := range values {
			if v.FieldID == fieldIDs["IntTestDate"] {
				assert.NotNil(t, v.ValueDate)
				assert.Equal(t, 2025, v.ValueDate.Year())
				assert.Equal(t, time.December, v.ValueDate.Month())
				assert.Equal(t, 25, v.ValueDate.Day())
			}
		}
	})

	// Test DateTime field
	t.Run("DateTime", func(t *testing.T) {
		dateTimeValue := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
		value := &DynamicFieldValue{
			FieldID:   fieldIDs["IntTestDateTime"],
			ObjectID:  testObjectID,
			ValueDate: &dateTimeValue,
		}
		err := SetDynamicFieldValue(value)
		require.NoError(t, err)

		values, _ := GetDynamicFieldValues(testObjectID)
		for _, v := range values {
			if v.FieldID == fieldIDs["IntTestDateTime"] {
				assert.NotNil(t, v.ValueDate)
				assert.Equal(t, 14, v.ValueDate.Hour())
				assert.Equal(t, 30, v.ValueDate.Minute())
			}
		}
	})

	// Cleanup
	db.Exec(`DELETE FROM dynamic_field_value WHERE object_id = ?`, testObjectID)
}

func testScreenConfigFiltering(t *testing.T, db *sql.DB, fields *testDynamicFieldSet) {
	// Test that GetFieldsForScreenWithConfig only returns fields enabled for the screen
	screenFields, err := GetFieldsForScreenWithConfig("AgentTicketPhone", DFObjectTicket)
	require.NoError(t, err)

	// Should find our test fields
	foundTestFields := 0
	for _, fwc := range screenFields {
		if strings.HasPrefix(fwc.Field.Name, "IntTest") {
			foundTestFields++
			assert.True(t, fwc.ConfigValue >= 1, "Field should be enabled (config_value >= 1)")
		}
	}
	assert.Greater(t, foundTestFields, 0, "Should find at least one test field")

	// Test with a screen that has no config
	noScreenFields, err := GetFieldsForScreenWithConfig("NonExistentScreen", DFObjectTicket)
	require.NoError(t, err)

	// Should not find our test fields (they're only configured for AgentTicketPhone)
	for _, fwc := range noScreenFields {
		if strings.HasPrefix(fwc.Field.Name, "IntTest") {
			t.Error("Should not find test fields on unconfigured screen")
		}
	}
}

func testMultiselectFieldPersistence(t *testing.T, db *sql.DB, fields *testDynamicFieldSet) {
	testObjectID := int64(77777)

	// Get multiselect field ID
	var fieldID int
	err := db.QueryRow(`SELECT id FROM dynamic_field WHERE name = 'IntTestMultiselect' LIMIT 1`).Scan(&fieldID)
	require.NoError(t, err)
	require.NotZero(t, fieldID)

	// Test ProcessDynamicFieldsFromForm with multiselect values
	formValues := url.Values{}
	formValues["DynamicField_IntTestMultiselect[]"] = []string{"opt1", "opt2", "opt3"}

	err = ProcessDynamicFieldsFromForm(formValues, int(testObjectID), DFObjectTicket, "AgentTicketPhone")
	require.NoError(t, err)

	// Verify the joined value was stored
	values, err := GetDynamicFieldValues(testObjectID)
	require.NoError(t, err)

	for _, v := range values {
		if v.FieldID == fieldID {
			assert.NotNil(t, v.ValueText)
			assert.Contains(t, *v.ValueText, "opt1")
			assert.Contains(t, *v.ValueText, "opt2")
			assert.Contains(t, *v.ValueText, "opt3")
			// Values should be joined with ||
			assert.Equal(t, "opt1||opt2||opt3", *v.ValueText)
		}
	}

	// Cleanup
	db.Exec(`DELETE FROM dynamic_field_value WHERE object_id = ?`, testObjectID)
}

func TestProcessDynamicFieldsFromFormIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Setup test fields
	testFields := setupTestDynamicFields(t, db)
	defer cleanupTestDynamicFields(t, db, testFields)

	testObjectID := int64(66666)

	t.Run("ProcessTextFields", func(t *testing.T) {
		formValues := url.Values{}
		formValues["DynamicField_IntTestText"] = []string{"Form Text Value"}
		formValues["DynamicField_IntTestTextArea"] = []string{"Form TextArea\nMultiple Lines"}

		err := ProcessDynamicFieldsFromForm(formValues, int(testObjectID), DFObjectTicket, "AgentTicketPhone")
		require.NoError(t, err)

		values, err := GetDynamicFieldValues(testObjectID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(values), 2, "Should have at least 2 values")
	})

	t.Run("ProcessCheckboxField", func(t *testing.T) {
		formValues := url.Values{}
		formValues["DynamicField_IntTestCheckbox"] = []string{"1"}

		err := ProcessDynamicFieldsFromForm(formValues, int(testObjectID), DFObjectTicket, "AgentTicketPhone")
		require.NoError(t, err)
	})

	t.Run("ProcessDateField", func(t *testing.T) {
		formValues := url.Values{}
		formValues["DynamicField_IntTestDate"] = []string{"2025-12-31"}

		err := ProcessDynamicFieldsFromForm(formValues, int(testObjectID), DFObjectTicket, "AgentTicketPhone")
		require.NoError(t, err)
	})

	t.Run("ProcessDateTimeField", func(t *testing.T) {
		formValues := url.Values{}
		formValues["DynamicField_IntTestDateTime"] = []string{"2025-06-15T09:30"}

		err := ProcessDynamicFieldsFromForm(formValues, int(testObjectID), DFObjectTicket, "AgentTicketPhone")
		require.NoError(t, err)
	})

	// Cleanup
	db.Exec(`DELETE FROM dynamic_field_value WHERE object_id = ?`, testObjectID)
}

func TestGetDynamicFieldValuesForDisplayIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Setup test fields
	testFields := setupTestDynamicFields(t, db)
	defer cleanupTestDynamicFields(t, db, testFields)

	testObjectID := int64(55555)

	// Set some values first
	formValues := url.Values{}
	formValues["DynamicField_IntTestText"] = []string{"Display Test Value"}
	formValues["DynamicField_IntTestCheckbox"] = []string{"1"}
	formValues["DynamicField_IntTestDropdown"] = []string{"high"}

	err = ProcessDynamicFieldsFromForm(formValues, int(testObjectID), DFObjectTicket, "AgentTicketPhone")
	require.NoError(t, err)

	// Test GetDynamicFieldValuesForDisplay
	displayFields, err := GetDynamicFieldValuesForDisplay(int(testObjectID), DFObjectTicket, "AgentTicketPhone")
	require.NoError(t, err)

	// Verify display values are formatted correctly
	for _, df := range displayFields {
		if df.Field.Name == "IntTestText" {
			assert.Equal(t, "Display Test Value", df.DisplayValue)
		}
		if df.Field.Name == "IntTestCheckbox" {
			// Checkbox should show "Yes" for value 1
			assert.Contains(t, []string{"Yes", "1", "true"}, df.DisplayValue)
		}
		if df.Field.Name == "IntTestDropdown" {
			// Should show display value from PossibleValues
			assert.Equal(t, "High", df.DisplayValue)
		}
	}

	// Cleanup
	db.Exec(`DELETE FROM dynamic_field_value WHERE object_id = ?`, testObjectID)
}

// TestArticleDynamicFieldsIntegration tests Article-level dynamic fields.
func TestArticleDynamicFieldsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Setup: Create test dynamic fields for Article object type
	articleFields := setupTestArticleDynamicFields(t, db)
	defer cleanupTestArticleDynamicFields(t, db)

	t.Run("ArticleFieldScreenDefinitions", func(t *testing.T) {
		// Verify Article screens exist in GetScreenDefinitions
		screens := GetScreenDefinitions()
		var articleScreens []string
		for _, s := range screens {
			if s.ObjectType == DFObjectArticle {
				articleScreens = append(articleScreens, s.Key)
			}
		}
		assert.Contains(t, articleScreens, "AgentArticleNote", "Should have AgentArticleNote screen")
		assert.Contains(t, articleScreens, "AgentArticleClose", "Should have AgentArticleClose screen")
		assert.Contains(t, articleScreens, "AgentArticleZoom", "Should have AgentArticleZoom screen")
	})

	t.Run("ProcessArticleDynamicFieldsFromForm", func(t *testing.T) {
		// Get an existing ticket ID
		var ticketID int
		err := db.QueryRow(`SELECT id FROM ticket ORDER BY id LIMIT 1`).Scan(&ticketID)
		if err != nil {
			t.Skip("No existing ticket in test database, skipping article test")
		}

		// Create a test article (article body is stored separately in article_data_mime)
		var articleID int64
		result, err := db.Exec(`INSERT INTO article 
			(ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, 
			 search_index_needs_rebuild, create_time, create_by, change_time, change_by)
			VALUES (?, 1, 3, 0, 0, NOW(), 1, NOW(), 1)`, ticketID)
		require.NoError(t, err)
		articleID, err = result.LastInsertId()
		require.NoError(t, err)
		defer db.Exec("DELETE FROM article WHERE id = ?", articleID)
		defer db.Exec("DELETE FROM dynamic_field_value WHERE object_id = ?", articleID)

		// Create form values with Article prefix
		formValues := map[string][]string{
			"ArticleDynamicField_" + articleFields.textField.Name: {"Article test value"},
		}

		// Process Article dynamic fields
		err = ProcessArticleDynamicFieldsFromForm(formValues, int(articleID), "AgentArticleNote")
		require.NoError(t, err)

		// Verify value was saved
		var savedValue string
		err = db.QueryRow(`SELECT value_text FROM dynamic_field_value WHERE field_id = ? AND object_id = ?`,
			articleFields.textField.ID, articleID).Scan(&savedValue)
		require.NoError(t, err)
		assert.Equal(t, "Article test value", savedValue)
	})

	t.Run("GetArticleDynamicFieldValuesForDisplay", func(t *testing.T) {
		// Get an existing ticket ID
		var ticketID int
		err := db.QueryRow(`SELECT id FROM ticket ORDER BY id LIMIT 1`).Scan(&ticketID)
		if err != nil {
			t.Skip("No existing ticket in test database, skipping article test")
		}

		// Create a test article
		var articleID int64
		result, err := db.Exec(`INSERT INTO article 
			(ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, 
			 search_index_needs_rebuild, create_time, create_by, change_time, change_by)
			VALUES (?, 1, 3, 0, 0, NOW(), 1, NOW(), 1)`, ticketID)
		require.NoError(t, err)
		articleID, err = result.LastInsertId()
		require.NoError(t, err)
		defer db.Exec("DELETE FROM article WHERE id = ?", articleID)
		defer db.Exec("DELETE FROM dynamic_field_value WHERE object_id = ?", articleID)

		// Save a value directly
		_, err = db.Exec(`INSERT INTO dynamic_field_value (field_id, object_id, value_text) VALUES (?, ?, ?)`,
			articleFields.textField.ID, articleID, "Display test value")
		require.NoError(t, err)

		// Retrieve for display
		displayFields, err := GetDynamicFieldValuesForDisplay(int(articleID), DFObjectArticle, "AgentArticleZoom")
		require.NoError(t, err)

		// Should find our test field
		found := false
		for _, df := range displayFields {
			if df.Field.Name == articleFields.textField.Name {
				found = true
				assert.Equal(t, "Display test value", df.DisplayValue)
				break
			}
		}
		assert.True(t, found, "Should find Article text field in display results")
	})
}

type testArticleDynamicFieldSet struct {
	textField *DynamicField
}

func setupTestArticleDynamicFields(t *testing.T, db *sql.DB) *testArticleDynamicFieldSet {
	t.Helper()

	fields := &testArticleDynamicFieldSet{}

	// Cleanup any leftover test data first
	db.Exec(`DELETE FROM dynamic_field_screen_config WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTestArticle%')`)
	db.Exec(`DELETE FROM dynamic_field_value WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTestArticle%')`)
	db.Exec(`DELETE FROM dynamic_field WHERE name LIKE 'IntTestArticle%'`)

	// Create Text field for Article object type
	_, err := db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestArticleText', 'Integration Test Article Text', 100, 'Text', 'Article', 'DefaultValue: ""', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Get the created field
	row := db.QueryRow(`SELECT id, name, label, field_type, object_type FROM dynamic_field WHERE name = 'IntTestArticleText'`)
	textField := &DynamicField{}
	err = row.Scan(&textField.ID, &textField.Name, &textField.Label, &textField.FieldType, &textField.ObjectType)
	require.NoError(t, err)
	fields.textField = textField

	// Enable field for Article screens
	_, err = db.Exec(`INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by) VALUES (?, 'AgentArticleNote', 1, NOW(), 1, NOW(), 1)`, textField.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by) VALUES (?, 'AgentArticleClose', 1, NOW(), 1, NOW(), 1)`, textField.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by) VALUES (?, 'AgentArticleZoom', 1, NOW(), 1, NOW(), 1)`, textField.ID)
	require.NoError(t, err)

	return fields
}

func cleanupTestArticleDynamicFields(t *testing.T, db *sql.DB) {
	t.Helper()
	db.Exec(`DELETE FROM dynamic_field_screen_config WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTestArticle%')`)
	db.Exec(`DELETE FROM dynamic_field_value WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTestArticle%')`)
	db.Exec(`DELETE FROM dynamic_field WHERE name LIKE 'IntTestArticle%'`)
}

func setupIntegrationTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err == nil {
		repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
		t.Cleanup(func() {
			repository.SetTicketNumberGenerator(nil, nil)
		})
	}

	router := gin.New()

	// Add authentication middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("user_email", "integration@test.local")
		c.Set("user_role", "Agent")
		c.Set("is_authenticated", true)
		c.Next()
	})

	// Register ticket create endpoint
	router.POST("/api/tickets", handleCreateTicketWithAttachments)

	return router
}

// TestCustomerPortalDynamicFieldsIntegration tests dynamic field integration for customer portal ticket view.
func TestCustomerPortalDynamicFieldsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "integration")

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Clean up before test
	cleanupTestCustomerPortalFields(t, db)

	t.Run("CustomerTicketZoomScreenDefinition", func(t *testing.T) {
		// Verify CustomerTicketZoom screen is defined
		screens := GetScreenDefinitions()
		found := false
		for _, s := range screens {
			if s.Key == "CustomerTicketZoom" {
				found = true
				assert.Equal(t, DFObjectTicket, s.ObjectType)
				assert.True(t, s.IsDisplayOnly, "CustomerTicketZoom should be display-only")
				break
			}
		}
		assert.True(t, found, "CustomerTicketZoom screen should be defined")
	})

	t.Run("CustomerArticleReplyScreenDefinition", func(t *testing.T) {
		// Verify CustomerArticleReply screen is defined
		screens := GetScreenDefinitions()
		found := false
		for _, s := range screens {
			if s.Key == "CustomerArticleReply" {
				found = true
				assert.Equal(t, DFObjectArticle, s.ObjectType)
				assert.True(t, s.SupportsRequired, "CustomerArticleReply should support required fields")
				break
			}
		}
		assert.True(t, found, "CustomerArticleReply screen should be defined")
	})

	t.Run("CustomerPortalTicketDynamicFieldsDisplay", func(t *testing.T) {
		// Setup test field for CustomerTicketZoom
		fields := setupTestCustomerPortalFields(t, db)
		defer cleanupTestCustomerPortalFields(t, db)

		// Get an existing ticket ID
		var ticketID int
		err := db.QueryRow("SELECT id FROM ticket ORDER BY id LIMIT 1").Scan(&ticketID)
		if err != nil {
			t.Skip("No existing ticket in test database")
		}

		// Save a test value
		_, err = db.Exec(`INSERT INTO dynamic_field_value (field_id, object_id, value_text) VALUES (?, ?, ?)`,
			fields.ticketTextField.ID, ticketID, "Customer visible value")
		require.NoError(t, err)
		defer db.Exec("DELETE FROM dynamic_field_value WHERE field_id = ? AND object_id = ?", fields.ticketTextField.ID, ticketID)

		// Retrieve for display via CustomerTicketZoom screen
		displayFields, err := GetDynamicFieldValuesForDisplay(ticketID, DFObjectTicket, "CustomerTicketZoom")
		require.NoError(t, err)

		// Should find our test field
		found := false
		for _, df := range displayFields {
			if df.Field.Name == fields.ticketTextField.Name {
				found = true
				assert.Equal(t, "Customer visible value", df.DisplayValue)
				break
			}
		}
		assert.True(t, found, "Should find CustomerTicketZoom field in display results")
	})

	t.Run("CustomerArticleReplyFieldsLoad", func(t *testing.T) {
		// Setup test field for CustomerArticleReply
		fields := setupTestCustomerPortalFields(t, db)
		defer cleanupTestCustomerPortalFields(t, db)

		// Load fields for CustomerArticleReply screen
		articleFields, err := GetFieldsForScreenWithConfig("CustomerArticleReply", DFObjectArticle)
		require.NoError(t, err)

		// Should find our test field
		found := false
		for _, dfc := range articleFields {
			if dfc.Field.Name == fields.articleTextField.Name {
				found = true
				assert.Equal(t, 2, dfc.ConfigValue, "CustomerArticleReply field should be required (ConfigValue=2)")
				break
			}
		}
		assert.True(t, found, "Should find CustomerArticleReply field in form fields")
	})

	t.Run("CustomerArticleReplyFieldsSave", func(t *testing.T) {
		// Setup test field for CustomerArticleReply
		fields := setupTestCustomerPortalFields(t, db)
		defer cleanupTestCustomerPortalFields(t, db)

		// Get an existing ticket ID
		var ticketID int
		err := db.QueryRow("SELECT id FROM ticket ORDER BY id LIMIT 1").Scan(&ticketID)
		if err != nil {
			t.Skip("No existing ticket in test database")
		}

		// Create a test article for customer reply
		var articleID int64
		result, err := db.Exec(`INSERT INTO article 
			(ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, 
			 search_index_needs_rebuild, create_time, create_by, change_time, change_by)
			VALUES (?, 3, 1, 1, 0, NOW(), 1, NOW(), 1)`, ticketID)
		require.NoError(t, err)
		articleID, err = result.LastInsertId()
		require.NoError(t, err)
		defer db.Exec("DELETE FROM article WHERE id = ?", articleID)
		defer db.Exec("DELETE FROM dynamic_field_value WHERE object_id = ?", articleID)

		// Simulate form submission with article dynamic field
		formData := url.Values{}
		formData.Set("ArticleDynamicField_"+fields.articleTextField.Name, "Customer reply note")

		// Process the form data
		err = ProcessArticleDynamicFieldsFromForm(formData, int(articleID), "CustomerArticleReply")
		require.NoError(t, err)

		// Verify the value was saved
		var savedValue string
		err = db.QueryRow(`SELECT value_text FROM dynamic_field_value WHERE field_id = ? AND object_id = ?`,
			fields.articleTextField.ID, articleID).Scan(&savedValue)
		require.NoError(t, err)
		assert.Equal(t, "Customer reply note", savedValue)
	})
}

type testCustomerPortalFieldSet struct {
	ticketTextField  *DynamicField
	articleTextField *DynamicField
}

func setupTestCustomerPortalFields(t *testing.T, db *sql.DB) *testCustomerPortalFieldSet {
	t.Helper()

	fields := &testCustomerPortalFieldSet{}

	// Cleanup any leftover test data first
	cleanupTestCustomerPortalFields(t, db)

	// Create Ticket Text field for CustomerTicketZoom
	_, err := db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestCustPortalTicket', 'Customer Portal Ticket Field', 200, 'Text', 'Ticket', 'DefaultValue: ""', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Get the created ticket field
	row := db.QueryRow(`SELECT id, name, label, field_type, object_type FROM dynamic_field WHERE name = 'IntTestCustPortalTicket'`)
	ticketField := &DynamicField{}
	err = row.Scan(&ticketField.ID, &ticketField.Name, &ticketField.Label, &ticketField.FieldType, &ticketField.ObjectType)
	require.NoError(t, err)
	fields.ticketTextField = ticketField

	// Enable field for CustomerTicketZoom screen
	_, err = db.Exec(`INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by) VALUES (?, 'CustomerTicketZoom', 1, NOW(), 1, NOW(), 1)`, ticketField.ID)
	require.NoError(t, err)

	// Create Article Text field for CustomerArticleReply
	_, err = db.Exec(`INSERT INTO dynamic_field 
		(internal_field, name, label, field_order, field_type, object_type, config, valid_id, create_time, create_by, change_time, change_by)
		VALUES (0, 'IntTestCustPortalArticle', 'Customer Portal Article Field', 201, 'Text', 'Article', 'DefaultValue: ""', 1, NOW(), 1, NOW(), 1)`)
	require.NoError(t, err)

	// Get the created article field
	row = db.QueryRow(`SELECT id, name, label, field_type, object_type FROM dynamic_field WHERE name = 'IntTestCustPortalArticle'`)
	articleField := &DynamicField{}
	err = row.Scan(&articleField.ID, &articleField.Name, &articleField.Label, &articleField.FieldType, &articleField.ObjectType)
	require.NoError(t, err)
	fields.articleTextField = articleField

	// Enable field for CustomerArticleReply screen with required flag (config_value=2)
	_, err = db.Exec(`INSERT INTO dynamic_field_screen_config (field_id, screen_key, config_value, create_time, create_by, change_time, change_by) VALUES (?, 'CustomerArticleReply', 2, NOW(), 1, NOW(), 1)`, articleField.ID)
	require.NoError(t, err)

	return fields
}

func cleanupTestCustomerPortalFields(t *testing.T, db *sql.DB) {
	t.Helper()
	db.Exec(`DELETE FROM dynamic_field_screen_config WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTestCustPortal%')`)
	db.Exec(`DELETE FROM dynamic_field_value WHERE field_id IN (SELECT id FROM dynamic_field WHERE name LIKE 'IntTestCustPortal%')`)
	db.Exec(`DELETE FROM dynamic_field WHERE name LIKE 'IntTestCustPortal%'`)
}
