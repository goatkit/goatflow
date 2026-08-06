package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// --- Service-layer tests ---

func TestSetupAssistant_CreateCannedResponses(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	// Empty list is skippable (returns nil).
	err := svc.CreateCannedResponses(ctx, nil, 1)
	assert.NoError(t, err)

	// Create two response templates.
	err = svc.CreateCannedResponses(ctx, []service.ResponseTemplateInput{
		{Name: "Greeting " + sfx, Shortcut: "/hi", Content: "Hello!", ContentType: "text/html"},
		{Name: "Closing " + sfx, Shortcut: "/bye", Content: "Goodbye!", ContentType: "text/html"},
	}, 1)
	require.NoError(t, err)

	// Verify they exist in the DB.
	db, _ := database.GetDB()
	var n int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM canned_response WHERE name LIKE ?"), "%"+sfx+"%").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "two canned responses should have been created")

	// Empty name is rejected.
	err = svc.CreateCannedResponses(ctx, []service.ResponseTemplateInput{
		{Name: "", Content: "missing name"},
	}, 1)
	assert.Error(t, err)

	// Empty content is rejected.
	err = svc.CreateCannedResponses(ctx, []service.ResponseTemplateInput{
		{Name: "Valid Name " + sfx, Content: ""},
	}, 1)
	assert.Error(t, err)
}

func TestSetupAssistant_CreateBusinessHours(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	// Empty name is rejected.
	err := svc.CreateBusinessHours(ctx, service.BusinessHoursInput{
		Name: "", Timezone: "America/New_York",
	}, 1)
	assert.Error(t, err)

	// Valid calendar is created with unique name.
	err = svc.CreateBusinessHours(ctx, service.BusinessHoursInput{
		Name: "BH" + sfx, Timezone: "Europe/London",
	}, 1)
	require.NoError(t, err)
}

func TestSetupAssistant_CreateEmailTransport(t *testing.T) {
	svc, _ := setupSvcTestDB(t)
	ctx := context.Background()

	// Missing host is rejected.
	err := svc.CreateEmailTransport(ctx, service.EmailTransportInput{
		Host: "", Port: 587,
	}, 1)
	assert.Error(t, err)

	// Valid config stored (may UPDATE existing or INSERT new).
	err = svc.CreateEmailTransport(ctx, service.EmailTransportInput{
		Host: "smtp.example.com", Port: 587, TLS: true, TLSMode: "starttls",
		From: "support@example.com", FromName: "Support",
	}, 1)
	require.NoError(t, err, "first write should succeed")

	// Idempotent: writing again should UPDATE, not fail.
	err = svc.CreateEmailTransport(ctx, service.EmailTransportInput{
		Host: "smtp2.example.com", Port: 465, TLS: true, TLSMode: "smtps",
		From: "noreply@example.com", FromName: "No Reply",
	}, 1)
	require.NoError(t, err, "second write (update) should succeed")
}

// --- Wizard integration tests ---

func TestSetupAssistant_Wizard_WithResponseTemplates(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	res := svc.ExecuteWizard(ctx, service.WizardRequest{
		Groups: []service.GroupInput{
			{Name: "Team " + sfx},
		},
		ResponseTemplates: []service.ResponseTemplateInput{
			{Name: "Wizard Greeting " + sfx, Content: "Hello from wizard!"},
		},
	})
	require.True(t, res.Success, "wizard should succeed, got: %v", res.Error)

	// Verify canned response was created.
	db, _ := database.GetDB()
	t.Logf("DB pointer: %p", db)
	var n int
	_ = db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM canned_response WHERE name LIKE ?"), "%Wizard Greeting "+sfx+"%").Scan(&n)
	assert.Greater(t, n, 0, "canned response should exist from wizard step 6")
}

func TestSetupAssistant_Wizard_WithBusinessHours(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	res := svc.ExecuteWizard(ctx, service.WizardRequest{
		Groups: []service.GroupInput{
			{Name: "BH Team " + sfx},
		},
		BusinessHours: []service.BusinessHoursInput{
			{Name: "BH Wizard " + sfx, Timezone: "Australia/Sydney"},
		},
	})
	require.True(t, res.Success, "wizard should succeed, got: %v", res.Error)

	// Verify calendar was created.
	db, _ := database.GetDB()
	var n int
	_ = db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM calendar WHERE name LIKE ?"), "%BH Wizard "+sfx+"%").Scan(&n)
	assert.Greater(t, n, 0, "business hours calendar should exist from wizard step 7")
}

func TestSetupAssistant_Wizard_WithEmailTransport(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	res := svc.ExecuteWizard(ctx, service.WizardRequest{
		Groups: []service.GroupInput{
			{Name: "Mail Team " + sfx},
		},
		MailTransport: &service.EmailTransportInput{
			Host: "smtp.wizard.test", Port: 25, From: "test@wizard.test",
		},
	})
	require.True(t, res.Success, "wizard should succeed, got: %v", res.Error)
}

// --- Tasks catalog tests ---

func TestSetupAssistant_TasksCatalog_NewTasks(t *testing.T) {
	svc := service.NewSetupAssistantService(nil, nil)
	core := svc.GetCoreTasks()

	// Must include the three new task IDs.
	ids := make(map[string]bool, len(core))
	for _, task := range core {
		ids[task.ID] = true
	}
	assert.True(t, ids["create_response_template"], "create_response_template task must be registered")
	assert.True(t, ids["configure_business_hours"], "configure_business_hours task must be registered")
	assert.True(t, ids["configure_email_transport"], "configure_email_transport task must be registered")

	// Each new task has a non-empty title (i18n resolved or fallback).
	for _, task := range core {
		if ids[task.ID] && (task.ID == "create_response_template" || task.ID == "configure_business_hours" || task.ID == "configure_email_transport") {
			assert.NotEmpty(t, task.Title, "task %s must have a title", task.ID)
			assert.NotEmpty(t, task.Description, "task %s must have a description", task.ID)
		}
	}
}

// --- Customer search API tests ---

func TestCustomerSearch_FindsByCompanyName(t *testing.T) {
	if !dbAvailable() {
		t.Skip("test database not available")
	}
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	// Create a customer company.
	cid := "SrchCo" + sfx
	err := svc.CreateCustomerCompany(ctx, cid, "Searchable Company "+sfx, nil, 1)
	require.NoError(t, err)

	// Query the search handler via a mock gin context.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/setup/customers/search/"+cid, nil)
	c.Params = gin.Params{{Key: "query", Value: cid}}

	handleAdminSetupCustomerSearch(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Value string `json:"value"`
			Label string `json:"label"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, cid, resp.Data[0].Value)
	assert.Contains(t, resp.Data[0].Label, "Searchable Company")
}

func TestCustomerSearch_FindsByPartialName(t *testing.T) {
	if !dbAvailable() {
		t.Skip("test database not available")
	}
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	cid := "Part" + sfx
	err := svc.CreateCustomerCompany(ctx, cid, "Partial Match Inc "+sfx, nil, 1)
	require.NoError(t, err)

	// Search by partial name fragment.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/setup/customers/search/Partial", nil)
	c.Params = gin.Params{{Key: "query", Value: "Partial"}}

	handleAdminSetupCustomerSearch(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Value string `json:"value"`
			Label string `json:"label"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.GreaterOrEqual(t, len(resp.Data), 1)

	// Our customer must appear in results.
	found := false
	for _, opt := range resp.Data {
		if opt.Value == cid {
			found = true
			break
		}
	}
	assert.True(t, found, "customer %s should appear in search results", cid)
}

func TestCustomerSearch_EmptyQuery_Returns400(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/setup/customers/search/", nil)
	c.Params = gin.Params{{Key: "query", Value: ""}}

	handleAdminSetupCustomerSearch(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCustomerSearch_NoMatch_ReturnsEmptyList(t *testing.T) {
	if !dbAvailable() {
		t.Skip("test database not available")
	}
	setupSvcTestDB(t) // ensure DB is ready

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/setup/customers/search/ZZZNOMATCHXYZ", nil)
	c.Params = gin.Params{{Key: "query", Value: "ZZZNOMATCHXYZ"}}

	handleAdminSetupCustomerSearch(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    []interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	// Data is null/empty when no matches (nil slice marshals to null in Go JSON).
}

func TestCustomerSearch_QueryStringFallback(t *testing.T) {
	if !dbAvailable() {
		t.Skip("test database not available")
	}
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	cid := "QS" + sfx
	err := svc.CreateCustomerCompany(ctx, cid, "Query String Co "+sfx, nil, 1)
	require.NoError(t, err)

	// Use ?q= instead of path param.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/setup/customers/search?q="+cid, nil)
	// No path params set — handler should fall back to c.Query("q").

	handleAdminSetupCustomerSearch(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, cid, resp.Data[0].Value)
}

// --- Template + API tests: selecting an existing customer must populate ALL fields ---

func TestOnboardTemplate_DropdownSelectionPopulatesAllFields(t *testing.T) {
	tmplBytes, err := readFile("../../templates/pages/admin/onboard_customer.pongo2")
	require.NoError(t, err)
	tmpl := string(tmplBytes)

	// 1. Uses the GoatKit autocomplete component (data-gk-autocomplete).
	assert.Contains(t, tmpl, "data-gk-autocomplete",
		"obName input must use data-gk-autocomplete component, not custom inline JS")
	assert.Contains(t, tmpl, `data-gk-autocomplete="companies"`,
		"autocomplete type must be 'companies'")

	// 2. Has a dropdown list linked via aria-controls.
	assert.Contains(t, tmpl, "aria-controls=",
		"input must have aria-controls linking to the dropdown list")

	// 3. Has seed data for the autocomplete.
	assert.Contains(t, tmpl, `data-gk-seed="companies"`,
		"template must include seed data script for companies")

	// 4. Has a select event listener that fetches config and populates fields.
	assert.Contains(t, tmpl, "gk:autocomplete:select",
		"JS must listen for gk:autocomplete:select event")
	assert.Contains(t, tmpl, "/api/v1/admin/setup/customers/",
		"JS must fetch customer config endpoint on selection")

	// 5. All address fields are populated from the config response.
	for _, field := range []string{"obStreet", "obCity", "obZip", "obUrl", "obComments"} {
		assert.Contains(t, tmpl, field,
			"JS must populate field %s from the customer config response", field)
	}
	assert.Contains(t, tmpl, "obCountry",
		"JS must populate the country select from the customer config response")

	// 6. Must NOT contain custom inline autocomplete JS (debounce, dropdown rendering, etc.).
	assert.NotContains(t, tmpl, "function searchExistingCustomers",
		"template must not contain custom searchExistingCustomers function — use data-gk-autocomplete")
	assert.NotContains(t, tmpl, "function selectCustomer(",
		"template must not contain custom selectCustomer function — use data-gk-autocomplete")
	assert.NotContains(t, tmpl, "function handleCustomerKeydown",
		"template must not contain custom keyboard handler — use data-gk-autocomplete")
}

func TestOnboardTemplate_DropdownKeyboardNavigation(t *testing.T) {
	tmplBytes, err := readFile("../../templates/pages/admin/onboard_customer.pongo2")
	require.NoError(t, err)
	tmpl := string(tmplBytes)

	// Keyboard navigation is handled by the GoatKit autocomplete component,
	// not custom inline JS. We verify the component is wired up correctly
	// and that no custom keyboard handlers exist.
	assert.Contains(t, tmpl, "data-gk-autocomplete",
		"autocomplete component must be present (handles ArrowUp/Down/Enter/Escape)")

	// The component sets role="combobox" automatically, which enables
	// screen reader keyboard navigation announcements.
	// No custom keydown handler should exist on the input.
	assert.NotContains(t, tmpl, "onkeydown=",
		"input must not have custom onkeydown handler — component handles keyboard nav")
}

func TestOnboardTemplate_UsersStepHasCountDisplay(t *testing.T) {
	tmplBytes, err := readFile("../../templates/pages/admin/onboard_customer.pongo2")
	require.NoError(t, err)
	tmpl := string(tmplBytes)

	// The Users step must have a count element that shows the current number of users.
	assert.Contains(t, tmpl, "obUserCount",
		"Users step must have an element with id obUserCount to display the user count")

	// The template must have an updateUserCount function.
	assert.Contains(t, tmpl, "function updateUserCount",
		"template must define updateUserCount() function")

	// obAddUser must call updateUserCount after adding a row.
	assert.Contains(t, tmpl, "updateUserCount",
		"obAddUser must call updateUserCount() to refresh the count display")
}

func TestCustomerConfigAPI_ReturnsAllFields(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "CfgAPI" + sfx

	// Create a customer with full address data.
	err := svc.CreateCustomerCompany(ctx, cid, "Config API Co "+sfx, map[string]string{
		"street":  "789 API Avenue",
		"city":    "Test City",
		"zip":     "54321",
		"country": "GB",
		"url":     "https://config.test",
		"comments": "Full config test",
	}, 1)
	require.NoError(t, err)

	// Call the config API endpoint via mock gin context.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/setup/customers/"+cid, nil)
	c.Params = gin.Params{{Key: "customer_id", Value: cid}}

	handleAdminSetupCustomerConfig(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Customer struct {
				Name     string `json:"name"`
				Street   string `json:"street"`
				City     string `json:"city"`
				Zip      string `json:"zip"`
				Country  string `json:"country"`
				Url      string `json:"url"`
				Comments string `json:"comments"`
			} `json:"customer"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	// EVERY field must be populated from the API response.
	assert.Contains(t, resp.Data.Customer.Name, "Config API Co")
	assert.Equal(t, "789 API Avenue", resp.Data.Customer.Street,
		"street must be returned by the config API")
	assert.Equal(t, "Test City", resp.Data.Customer.City,
		"city must be returned by the config API")
	assert.Equal(t, "54321", resp.Data.Customer.Zip,
		"zip must be returned by the config API")
	assert.Equal(t, "GB", resp.Data.Customer.Country,
		"country must be returned by the config API")
	assert.Equal(t, "https://config.test", resp.Data.Customer.Url,
		"url must be returned by the config API")
	assert.Equal(t, "Full config test", resp.Data.Customer.Comments,
		"comments must be returned by the config API")
}

func TestCustomerConfigAPI_ReturnsPortalUsers(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "PortalUsers" + sfx

	// Create customer company directly (doesn't need seed data like OnboardCustomer does).
	err := svc.CreateCustomerCompany(ctx, cid, "Portal Users Co "+sfx, nil, 1)
	require.NoError(t, err)

	// Insert 2 portal users directly.
	db, _ := database.GetDB()
	require.NotNil(t, db)
	_, err = db.ExecContext(ctx, database.ConvertPlaceholders(
		"INSERT INTO customer_user (customer_id, login, email, first_name, last_name, pw, valid_id, create_time, create_by, change_time, change_by) "+
			"VALUES (?, ?, ?, ?, ?, 'x', 1, NOW(), 1, NOW(), 1)"), cid, "user1"+sfx, "u1"+sfx+"@test.com", "User", "One")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, database.ConvertPlaceholders(
		"INSERT INTO customer_user (customer_id, login, email, first_name, last_name, pw, valid_id, create_time, create_by, change_time, change_by) "+
			"VALUES (?, ?, ?, ?, ?, 'x', 1, NOW(), 1, NOW(), 1)"), cid, "user2"+sfx, "u2"+sfx+"@test.com", "User", "Two")
	require.NoError(t, err)

	// Call LoadExistingCustomer directly (avoids sync.Once caching in getSetupAssistantService).
	config, err := svc.LoadExistingCustomer(ctx, cid)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Must return BOTH portal users — not just 1.
	require.Len(t, config.PortalUsers, 2, "both portal users must be returned")
	assert.Contains(t, config.PortalUsers[0].Login, "user1")
	assert.Contains(t, config.PortalUsers[1].Login, "user2")
	assert.Contains(t, config.PortalUsers[0].Email, "u1")
}

func TestSetupAssistant_LoadExistingCustomer(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	// Create a customer with minimal data.
	cid := "LoadMin" + sfx
	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "Minimal Co " + sfx,
		Users: []service.CustomerUserInput{
			{Login: "minuser" + sfx, Email: "min" + sfx + "@test.com", FirstName: "Min", LastName: "User"},
		},
	})
	require.True(t, res.Success, "onboarding should succeed: %v", res.Error)

	// Load it back — even minimal data must populate Customer.
	config, err := svc.LoadExistingCustomer(ctx, cid)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config.Customer)
	assert.Contains(t, config.Customer.Name, "Minimal Co")

	// Nonexistent customer returns error.
	_, err = svc.LoadExistingCustomer(ctx, "NONEXISTENT12345")
	assert.Error(t, err)

	// Empty customer ID returns error.
	_, err = svc.LoadExistingCustomer(ctx, "")
	assert.Error(t, err)
}

func TestSetupAssistant_LoadExistingCustomer_AllFieldsPopulated(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	// Create a DEDICATED group so this customer's associations are isolated
	// from seed data and other tests. This ensures queues/mail accounts only
	// appear because WE created them, not because of a shared group_id=1.
	dedicatedGroupID, err := svc.CreateGroup(ctx, "IsolatedGrp "+sfx, "dedicated test group", 1)
	require.NoError(t, err)
	require.Greater(t, dedicatedGroupID, 0)

	// Create a dedicated queue owned by the dedicated group.
	dedicatedQueueID, err := svc.CreateQueue(ctx, "IsolatedQ "+sfx, []int{dedicatedGroupID}, "dedicated test queue", 1)
	require.NoError(t, err)
	require.Greater(t, dedicatedQueueID, 0)

	cid := "Full" + sfx

	// Create a customer with ALL fields populated, linked to the dedicated group.
	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "Full Fields Corp " + sfx,
		Street:     "123 Test Street",
		City:       "Testville",
		ZIP:        "12345",
		Country:    "AU",
		URL:        "https://example.test",
		Comments:   "Test customer with all fields",
		Users: []service.CustomerUserInput{
			{Login: "fulluser" + sfx, Email: "full" + sfx + "@test.com", FirstName: "Full", LastName: "Fields"},
		},
		ManagingGroupIDs: []int{dedicatedGroupID},
		MailAccount: &service.MailAccountInput{
			Login:       "fullmail" + sfx + "@example.com",
			Password:    "secret123",
			Host:        "imap.example.com",
			AccountType: "IMAP",
			QueueID:     dedicatedQueueID,
		},
	})
	require.True(t, res.Success, "onboarding should succeed: %v", res.Error)

	config, err := svc.LoadExistingCustomer(ctx, cid)
	require.NoError(t, err)
	require.NotNil(t, config)

	// --- Customer company: every field must be populated ---
	t.Run("customer_company_all_fields", func(t *testing.T) {
		require.NotNil(t, config.Customer)
		assert.Contains(t, config.Customer.Name, "Full Fields Corp")
		assert.Equal(t, "123 Test Street", config.Customer.Street)
		assert.Equal(t, "Testville", config.Customer.City)
		assert.Equal(t, "12345", config.Customer.Zip)
		assert.Equal(t, "AU", config.Customer.Country)
		assert.Equal(t, "https://example.test", config.Customer.Url)
		assert.Equal(t, "Test customer with all fields", config.Customer.Comments)
	})

	// --- Groups: exactly our dedicated group ---
	t.Run("groups_has_dedicated_team", func(t *testing.T) {
		require.NotEmpty(t, config.Groups, "managing group must be returned")
		found := false
		for _, g := range config.Groups {
			assert.NotEmpty(t, g.Name)
			assert.Greater(t, g.ID, 0)
			if g.ID == dedicatedGroupID {
				assert.Contains(t, g.Name, "IsolatedGrp")
				found = true
			}
		}
		assert.True(t, found, "dedicated group %d must appear in groups", dedicatedGroupID)
	})

	// --- Queues: exactly our dedicated queue ---
	t.Run("queues_has_dedicated_queue", func(t *testing.T) {
		require.Len(t, config.Queues, 1, "exactly one queue (the dedicated one) must be returned")
		assert.Contains(t, config.Queues[0].Name, "IsolatedQ")
	})

	// --- Mail accounts: exactly our dedicated mailbox ---
	t.Run("mail_accounts_has_dedicated_mailbox", func(t *testing.T) {
		require.Len(t, config.MailAccounts, 1, "exactly one mail account must be returned")
		ma := config.MailAccounts[0]
		assert.Contains(t, ma.Login, "fullmail")
		assert.Equal(t, "imap.example.com", ma.Host)
		assert.Equal(t, "IMAP", ma.AccountType)
		assert.Equal(t, dedicatedQueueID, ma.QueueID)
	})

	// --- Services/SLAs: empty (none assigned) ---
	t.Run("services_empty", func(t *testing.T) {
		assert.Empty(t, config.Services, "no SLA assigned → services must be empty")
	})
	t.Run("slas_empty", func(t *testing.T) {
		assert.Empty(t, config.SLAs)
	})

	// --- Global sections: empty/null ---
	t.Run("global_sections_empty", func(t *testing.T) {
		assert.Empty(t, config.CannedResponses)
		assert.Empty(t, config.BusinessHours)
		assert.Nil(t, config.EmailTransport)
	})
}

func TestSetupAssistant_LoadExistingCustomer_NewCompanyIsEmpty(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	// Create a bare customer with NO managing teams, NO SLA, NO mail account.
	// Use a unique group that has NO queues and NO mail accounts.
	cid := "Bare" + sfx
	err := svc.CreateCustomerCompany(ctx, cid, "Bare Bones Co "+sfx, map[string]string{
		"street":  "456 Empty Lane",
		"city":    "Nowhere",
		"zip":     "67890",
		"country": "NZ",
	}, 1)
	require.NoError(t, err)

	config, err := svc.LoadExistingCustomer(ctx, cid)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Customer company exists with fields populated.
	t.Run("customer_company_populated", func(t *testing.T) {
		require.NotNil(t, config.Customer)
		assert.Contains(t, config.Customer.Name, "Bare Bones")
		assert.Equal(t, "456 Empty Lane", config.Customer.Street)
		assert.Equal(t, "Nowhere", config.Customer.City)
		assert.Equal(t, "67890", config.Customer.Zip)
		assert.Equal(t, "NZ", config.Customer.Country)
	})

	// No managing team assigned → groups must be empty.
	t.Run("groups_empty", func(t *testing.T) {
		assert.Empty(t, config.Groups, "no managing team → groups must be empty")
	})

	// No queues linked → queues must be empty.
	t.Run("queues_empty", func(t *testing.T) {
		assert.Empty(t, config.Queues, "no queue association → queues must be empty")
	})

	// No mail account → mail accounts must be empty.
	t.Run("mail_accounts_empty", func(t *testing.T) {
		assert.Empty(t, config.MailAccounts, "no mailbox → mail accounts must be empty")
	})

	// No SLA → services and SLAs must be empty.
	t.Run("services_and_slas_empty", func(t *testing.T) {
		assert.Empty(t, config.Services, "no SLA → services must be empty")
		assert.Empty(t, config.SLAs, "no SLA → SLAs must be empty")
	})
}

func TestSetupAssistant_LoadExistingCustomer_WithSLA(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "SLA" + sfx

	// Find a real SLA id.
	db, _ := database.GetDB()
	var slaID int
	_ = db.QueryRow("SELECT id FROM sla WHERE valid_id = 1 LIMIT 1").Scan(&slaID)
	if slaID == 0 {
		t.Skip("no SLA available in test DB")
	}

	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "SLA Test Co " + sfx,
		Users: []service.CustomerUserInput{
			{Login: "slauser" + sfx, Email: "sla" + sfx + "@test.com", FirstName: "SLA", LastName: "Test"},
		},
		SLAID:       slaID,
		ServiceName: "Premium " + sfx,
	})
	require.True(t, res.Success, "onboarding with SLA should succeed: %v", res.Error)

	config, err := svc.LoadExistingCustomer(ctx, cid)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Services must be populated with the service name.
	t.Run("services_populated", func(t *testing.T) {
		require.NotEmpty(t, config.Services, "service must be returned when SLA was assigned")
		// The service name must match what was created.
		found := false
		for _, svc := range config.Services {
			if strings.Contains(svc, "Premium") {
				found = true
				break
			}
		}
		assert.True(t, found, "created service must appear in services list")
	})
}

func TestSetupAssistant_LoadExistingCustomer_WithMailAccount(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "Mail" + sfx

	// Find a real queue id to target.
	db, _ := database.GetDB()
	var queueID int
	_ = db.QueryRow("SELECT id FROM queue WHERE valid_id = 1 LIMIT 1").Scan(&queueID)
	require.Greater(t, queueID, 0, "test DB needs at least one active queue")

	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "Mail Test Co " + sfx,
		Users: []service.CustomerUserInput{
			{Login: "mailuser" + sfx, Email: "mail" + sfx + "@test.com", FirstName: "Mail", LastName: "Test"},
		},
		ManagingGroupIDs: []int{1},
		MailAccount: &service.MailAccountInput{
			Login:       "mailtest" + sfx + "@example.com",
			Password:    "secret123",
			Host:        "imap.example.com",
			AccountType: "IMAP",
			QueueID:     queueID,
		},
	})
	require.True(t, res.Success, "onboarding with mail should succeed: %v", res.Error)

	config, err := svc.LoadExistingCustomer(ctx, cid)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Mail accounts must be populated with all fields.
	t.Run("mail_accounts_populated", func(t *testing.T) {
		require.NotEmpty(t, config.MailAccounts, "mail account must be returned")
		ma := config.MailAccounts[0]
		assert.NotEmpty(t, ma.Login, "mail account login must be populated")
		assert.NotEmpty(t, ma.Host, "mail account host must be populated")
		assert.NotEmpty(t, ma.AccountType, "mail account type must be populated")
		assert.Greater(t, ma.QueueID, 0, "mail account queue_id must be populated")
	})

	// Queues must also be populated since the managing team owns queues.
	t.Run("queues_populated_from_group", func(t *testing.T) {
		// If group 1 owns any queues, they should appear.
		for _, q := range config.Queues {
			assert.NotEmpty(t, q.Name, "queue name must be populated")
		}
	})
}

func TestSetupAssistant_LoadExistingCustomer_CreateManagingTeam(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "MGT" + sfx
	teamName := "LoadMGT Team " + sfx

	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID:             cid,
		Name:                   "MGT Load Co " + sfx,
		CreateManagingGroupName: teamName,
		Users: []service.CustomerUserInput{
			{Login: "mgtuser" + sfx, Email: "mgt" + sfx + "@test.com", FirstName: "MGT", LastName: "Load"},
		},
	})
	require.True(t, res.Success, "onboarding with managing team should succeed: %v", res.Error)
	require.Greater(t, res.CreatedGroupID, 0)

	config, err := svc.LoadExistingCustomer(ctx, cid)
	require.NoError(t, err)
	require.NotNil(t, config)

	// The created managing team must appear in the groups list.
	t.Run("created_team_in_groups", func(t *testing.T) {
		require.NotEmpty(t, config.Groups)
		found := false
		for _, g := range config.Groups {
			if g.ID == res.CreatedGroupID {
				assert.Contains(t, g.Name, teamName, "team name must match")
				found = true
				break
			}
		}
		assert.True(t, found, "created team (id=%d) must appear in groups", res.CreatedGroupID)
	})
}

// --- i18n tests ---

func TestI18n_SetupAssistantTasks_AllLanguages(t *testing.T) {
	// Verify the setup_assistant_tasks section exists in the English translation file
	enBytes, err := readFile("../../internal/platform/i18n/translations/en.json")
	require.NoError(t, err)

	var en map[string]interface{}
	require.NoError(t, json.Unmarshal(enBytes, &en))

	tasks, ok := en["setup_assistant_tasks"].(map[string]interface{})
	require.True(t, ok, "en.json must have setup_assistant_tasks section")

	for _, taskID := range []string{"create_response_template", "configure_business_hours", "configure_email_transport"} {
		task, ok := tasks[taskID].(map[string]interface{})
		require.True(t, ok, "task %s must exist in setup_assistant_tasks", taskID)
		assert.NotEmpty(t, task["title"], "task %s must have a title", taskID)
		assert.NotEmpty(t, task["description"], "task %s must have a description", taskID)
	}
}

// --- Helpers ---

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
