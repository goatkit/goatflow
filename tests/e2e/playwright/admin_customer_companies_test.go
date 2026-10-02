//go:build e2e

package playwright

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func count(t *testing.T, loc playwright.Locator) int {
	n, err := loc.Count()
	require.NoError(t, err)
	return n
}

// contains reports whether substr occurs in text, ignoring case.
func contains(text, substr string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substr))
}

// companyRequest calls an admin endpoint with the browser session's cookies
// and returns the status and body. Redirects are not followed.
func companyRequest(t *testing.T, b *helpers.BrowserHelper, method, path string, form map[string]string) (int, string) {
	t.Helper()
	opts := playwright.APIRequestContextFetchOptions{
		Method:       playwright.String(method),
		MaxRedirects: playwright.Int(0),
		Headers:      map[string]string{"Accept": "application/json"},
	}
	if form != nil {
		f := make(map[string]any, len(form))
		for k, v := range form {
			f[k] = v
		}
		opts.Form = f
	}
	resp, err := b.Page.Request().Fetch(b.Config.BaseURL+path, opts)
	require.NoError(t, err, "%s %s", method, path)
	defer resp.Dispose()
	body, err := resp.Text()
	require.NoError(t, err)
	return resp.Status(), body
}

// effectivePortalSettings returns the portal settings the company resolves to
// (its overrides merged onto the defaults) from GET .../portal-settings.
func effectivePortalSettings(t *testing.T, b *helpers.BrowserHelper, customerID string) map[string]any {
	t.Helper()
	resp, err := b.Page.Request().Get(b.Config.BaseURL + "/admin/customer/companies/" + url.PathEscape(customerID) + "/portal-settings")
	require.NoError(t, err)
	defer resp.Dispose()
	require.Equal(t, http.StatusOK, resp.Status())
	var body struct {
		Settings map[string]any `json:"settings"`
	}
	require.NoError(t, resp.JSON(&body))
	require.NotNil(t, body.Settings, "portal-settings should return a settings object")
	return body.Settings
}

// TestAdminCustomerCompaniesPlaywright exercises the customer company admin
// pages end to end on a company it creates, and deactivates that company
// again afterwards (customer companies are soft-deleted, as in OTRS).
func TestAdminCustomerCompaniesPlaywright(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	// Registered first so it runs last, after the cleanups that use the session.
	t.Cleanup(browser.TearDown)

	auth := helpers.NewAuthHelper(browser)
	require.NoError(t, auth.LoginAsAdmin(), "Failed to login as admin")

	page := browser.Page
	page.OnDialog(func(d playwright.Dialog) { _ = d.Accept() })

	customerID := fmt.Sprintf("PLAYWRIGHT_%d", time.Now().UnixNano())
	companyName := "Playwright Company " + customerID[len(customerID)-6:]
	editPath := "/admin/customer/companies/" + customerID + "/edit"
	companyRow := page.Locator(fmt.Sprintf("table.gk-table tbody tr:has(td:first-child div:text-is('%s'))", customerID))
	created := false
	t.Cleanup(func() {
		if !created {
			return
		}
		status, body := companyRequest(t, browser, http.MethodPost, "/admin/customer/companies/"+customerID+"/delete", nil)
		assert.Equal(t, http.StatusOK, status, "deactivating %s: %s", customerID, body)
	})

	t.Run("List page", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/customer/companies"))
		title, err := page.Title()
		require.NoError(t, err)
		assert.Contains(t, title, "Customer Companies")
		assert.Equal(t, 1, count(t, page.Locator("a[href='/admin/customer/companies/new']:has-text('Add New Company')")))
		for _, seeded := range []string{"COMP1", "COMP2", "TEST001"} {
			assert.Equal(t, 1, count(t, page.Locator(fmt.Sprintf("table.gk-table tbody td:first-child div:text-is('%s')", seeded))),
				"seeded company %s should be listed", seeded)
		}
	})

	t.Run("Search filters the list", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/customer/companies"))
		require.NoError(t, page.Locator("input[name='search']").Fill("COMP1"))
		require.NoError(t, page.Locator("form[action='/admin/customer/companies'] button[type='submit']").Click())
		require.NoError(t, page.WaitForURL("**/admin/customer/companies?search=COMP1*"))

		ids, err := page.Locator("table.gk-table tbody td:first-child div").AllTextContents()
		require.NoError(t, err)
		require.NotEmpty(t, ids, "search for COMP1 should match the seeded company")
		for _, id := range ids {
			assert.Contains(t, strings.TrimSpace(id), "COMP1")
		}
		value, err := page.Locator("input[name='search']").InputValue()
		require.NoError(t, err)
		assert.Equal(t, "COMP1", value, "the search box should keep the query")
	})

	t.Run("Create form requires ID and name", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/customer/companies/new"))
		require.NoError(t, page.Locator("button[type='submit']:has-text('Create Company')").Click())
		// HTML5 validation blocks the submit, so the browser stays on the form.
		assert.True(t, strings.HasSuffix(page.URL(), "/admin/customer/companies/new"), "empty form must not submit: %s", page.URL())
		missing, err := page.Locator("#customer_id").Evaluate("el => el.validity.valueMissing", nil)
		require.NoError(t, err)
		assert.Equal(t, true, missing, "customer_id should be reported as missing")

		// Every visible form control is labelled.
		for _, id := range []string{"customer_id", "name", "street", "zip", "city", "country", "url", "valid_id", "comments"} {
			assert.Equal(t, 1, count(t, page.Locator(fmt.Sprintf("#content-general label[for='%s']", id))), "control %s should have a label", id)
		}
	})

	t.Run("Create company", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/customer/companies/new"))
		require.NoError(t, page.Locator("#customer_id").Fill(customerID))
		require.NoError(t, page.Locator("#name").Fill(companyName))
		require.NoError(t, page.Locator("#street").Fill("123 Playwright St"))
		require.NoError(t, page.Locator("#city").Fill("Playwright City"))
		require.NoError(t, page.Locator("button[type='submit']:has-text('Create Company')").Click())
		require.NoError(t, page.WaitForURL("**"+editPath+"?success=1"), "create should open the new company's edit form")
		created = true

		for sel, want := range map[string]string{"#customer_id": customerID, "#name": companyName, "#street": "123 Playwright St", "#city": "Playwright City"} {
			got, err := page.Locator(sel).InputValue()
			require.NoError(t, err)
			assert.Equal(t, want, got, sel)
		}
		readonly, err := page.Locator("#customer_id").GetAttribute("readonly")
		require.NoError(t, err)
		assert.NotNil(t, readonly, "customer_id must not be editable after create")

		status, body := companyRequest(t, browser, http.MethodPost, "/admin/customer/companies",
			map[string]string{"customer_id": customerID, "name": "Duplicate"})
		assert.Equal(t, http.StatusBadRequest, status, "duplicate customer_id must be rejected: %s", body)
		assert.Contains(t, body, "already exists")
	})

	t.Run("Edit company", func(t *testing.T) {
		require.True(t, created, "company was not created")
		require.NoError(t, browser.NavigateTo(editPath))
		require.NoError(t, page.Locator("#city").Fill("Edited City"))
		require.NoError(t, page.Locator("#zip").Fill("PW-123"))
		require.NoError(t, page.Locator("button[type='submit']:has-text('Update Company')").Click())
		require.NoError(t, page.WaitForURL("**"+editPath+"?success=1"))
		require.NoError(t, page.Locator("#toast:has-text('Customer company updated successfully')").WaitFor())
		require.NoError(t, browser.NavigateTo(editPath))
		city, err := page.Locator("#city").InputValue()
		require.NoError(t, err)
		assert.Equal(t, "Edited City", city)
		zip, err := page.Locator("#zip").InputValue()
		require.NoError(t, err)
		assert.Equal(t, "PW-123", zip)

		status, body := companyRequest(t, browser, http.MethodPost, editPath, map[string]string{"name": "", "valid_id": "1"})
		assert.Equal(t, http.StatusBadRequest, status, "empty name must be rejected: %s", body)
		require.NoError(t, browser.NavigateTo(editPath))
		name, err := page.Locator("#name").InputValue()
		require.NoError(t, err)
		assert.Equal(t, companyName, name, "rejected update must not change the name")
	})

	t.Run("Portal Settings override and inheritance", func(t *testing.T) {
		require.True(t, created, "company was not created")
		portalPath := editPath + "?tab=portal"
		require.NoError(t, browser.NavigateTo(portalPath))
		portal := page.Locator("#content-portal")
		require.NoError(t, portal.WaitFor(), "?tab=portal should open the Portal Settings tab")

		overrideTitle := portal.Locator("input[type='checkbox'][name='override_title']")
		titleField := portal.Locator("#fld-title")
		defaultTitle, err := titleField.GetAttribute("placeholder")
		require.NoError(t, err)
		defaults := effectivePortalSettings(t, browser, customerID)
		require.Equal(t, defaultTitle, defaults["Title"], "a new company inherits the default title")

		checked, err := overrideTitle.IsChecked()
		require.NoError(t, err)
		require.False(t, checked, "a new company has no title override")
		disabled, err := titleField.IsDisabled()
		require.NoError(t, err)
		require.True(t, disabled, "inherited fields are read-only")

		// Tick the override, set a value and save.
		customTitle := "Portal " + customerID
		require.NoError(t, overrideTitle.Check())
		require.NoError(t, titleField.Fill(customTitle))
		require.NoError(t, portal.Locator("button[type='submit']:has-text('Save Portal Settings')").Click())
		require.NoError(t, page.WaitForURL("**"+editPath+"?tab=portal&success=1"), "saving should return to the Portal Settings tab")
		visible, err := portal.IsVisible()
		require.NoError(t, err)
		assert.True(t, visible, "the Portal Settings tab should be active after saving")

		checked, err = overrideTitle.IsChecked()
		require.NoError(t, err)
		assert.True(t, checked, "the title override should persist")
		value, err := titleField.InputValue()
		require.NoError(t, err)
		assert.Equal(t, customTitle, value)
		for _, other := range []string{"override_enabled", "override_login_required", "override_footer_text", "override_landing_page"} {
			c, err := portal.Locator(fmt.Sprintf("input[type='checkbox'][name='%s']", other)).IsChecked()
			require.NoError(t, err)
			assert.False(t, c, "%s was not ticked and must stay inherited", other)
		}
		settings := effectivePortalSettings(t, browser, customerID)
		assert.Equal(t, customTitle, settings["Title"], "the company resolves to its own title")
		for _, key := range []string{"Enabled", "LoginRequired", "FooterText", "LandingPage"} {
			assert.Equal(t, defaults[key], settings[key], "%s should still be inherited", key)
		}

		// Untick the override: the company inherits the default again.
		require.NoError(t, overrideTitle.Uncheck())
		disabled, err = titleField.IsDisabled()
		require.NoError(t, err)
		assert.True(t, disabled, "unticking the override makes the field read-only")
		require.NoError(t, portal.Locator("button[type='submit']:has-text('Save Portal Settings')").Click())
		require.NoError(t, page.WaitForURL("**"+editPath+"?tab=portal&success=1"))

		checked, err = overrideTitle.IsChecked()
		require.NoError(t, err)
		assert.False(t, checked, "the title override should be removed")
		value, err = titleField.InputValue()
		require.NoError(t, err)
		assert.Empty(t, value, "an inherited title has no per-company value")
		assert.Equal(t, defaults, effectivePortalSettings(t, browser, customerID), "every setting is inherited again")
	})

	t.Run("Users and tickets pages match the list counts", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/customer/companies?search=COMP1"))
		row := page.Locator("table.gk-table tbody tr:has(td:first-child div:text-is('COMP1'))")
		usersLink := row.Locator("a[href='/admin/customer/companies/COMP1/users']")
		usersText, err := usersLink.TextContent()
		require.NoError(t, err)
		listedUsers, err := strconv.Atoi(strings.Fields(usersText)[0])
		require.NoError(t, err, "users link text %q", usersText)
		ticketsText, err := row.Locator("a[href='/admin/customer/companies/COMP1/tickets']").TextContent()
		require.NoError(t, err)
		listedTickets, err := strconv.Atoi(strings.Fields(ticketsText)[0])
		require.NoError(t, err, "tickets link text %q", ticketsText)

		require.NoError(t, usersLink.Click())
		require.NoError(t, page.WaitForURL("**/admin/customer/companies/COMP1/users"))
		assert.Equal(t, listedUsers, count(t, page.Locator("tr[data-login]")), "users page rows")
		assert.Equal(t, 1, count(t, page.Locator("tr[data-login='"+helpers.SeedCustomerLogin+"']")), "seeded COMP1 customer user")

		require.NoError(t, browser.NavigateTo("/admin/customer/companies/COMP1/tickets"))
		total, err := page.Locator("#company-ticket-total").TextContent()
		require.NoError(t, err)
		assert.Equal(t, strconv.Itoa(listedTickets), strings.TrimSpace(total), "tickets page total")
	})

	t.Run("Services tab assigns a service to a customer user", func(t *testing.T) {
		serviceName := fmt.Sprintf("PW Service %d", time.Now().UnixNano())
		status, body := companyRequest(t, browser, http.MethodPost, "/admin/services/create", map[string]string{"name": serviceName})
		require.Equal(t, http.StatusOK, status, "create service: %s", body)

		require.NoError(t, browser.NavigateTo("/admin/customer/companies/COMP1/edit"))
		require.NoError(t, page.Locator("nav[aria-label='Tabs'] a[href='/admin/customer/companies/COMP1/services']").Click())
		require.NoError(t, page.WaitForURL("**/admin/customer/companies/COMP1/services"))

		serviceRow := page.Locator(fmt.Sprintf("tr[data-service-id]:has(div:text-is('%s'))", serviceName))
		require.NoError(t, serviceRow.WaitFor(), "the new service should be listed")
		serviceID, err := serviceRow.GetAttribute("data-service-id")
		require.NoError(t, err)
		t.Cleanup(func() {
			status, body := companyRequest(t, browser, http.MethodDelete, "/admin/services/"+serviceID+"/delete", nil)
			assert.Equal(t, http.StatusOK, status, "delete service %s: %s", serviceID, body)
		})

		assign := serviceRow.Locator(fmt.Sprintf("label:has-text('%s') input[name='assign']", helpers.SeedCustomerLogin))
		checked, err := assign.IsChecked()
		require.NoError(t, err)
		require.False(t, checked, "a new service is assigned to nobody")
		require.NoError(t, assign.Check())
		require.NoError(t, page.Locator("button[type='submit']:has-text('Save assignments')").Click())
		require.NoError(t, page.WaitForURL("**/admin/customer/companies/COMP1/services?saved=1"))
		require.NoError(t, page.Locator("#services-saved").WaitFor())

		checked, err = assign.IsChecked()
		require.NoError(t, err)
		assert.True(t, checked, "the assignment should persist")
		assigned, err := serviceRow.Locator("td").Nth(1).TextContent()
		require.NoError(t, err)
		assert.Regexp(t, regexp.MustCompile(`^1 / \d+$`), strings.TrimSpace(assigned))

		require.NoError(t, assign.Uncheck())
		require.NoError(t, page.Locator("button[type='submit']:has-text('Save assignments')").Click())
		require.NoError(t, page.WaitForURL("**/admin/customer/companies/COMP1/services?saved=1"))
		checked, err = assign.IsChecked()
		require.NoError(t, err)
		assert.False(t, checked, "the assignment should be removed")
	})

	t.Run("Deactivate and activate from the list", func(t *testing.T) {
		require.True(t, created, "company was not created")
		require.NoError(t, browser.NavigateTo("/admin/customer/companies?search="+customerID))
		require.NoError(t, companyRow.Locator(".gk-badge-success").WaitFor(), "a new company is valid")

		resp, err := page.ExpectResponse("**/admin/customer/companies/"+customerID+"/delete", func() error {
			return companyRow.Locator("button[onclick^='deleteCompany(']").Click()
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.Status())
		require.NoError(t, companyRow.Locator(".gk-badge-error").WaitFor(), "the company should be shown as invalid after deactivating")

		require.NoError(t, browser.NavigateTo("/admin/customer/companies?valid=valid&search="+customerID))
		assert.Equal(t, 0, count(t, companyRow), "an invalid company is hidden by the Valid Only filter")
		require.NoError(t, browser.NavigateTo("/admin/customer/companies?valid=invalid&search="+customerID))
		assert.Equal(t, 1, count(t, companyRow), "an invalid company is listed by the Invalid Only filter")

		resp, err = page.ExpectResponse("**/admin/customer/companies/"+customerID+"/activate", func() error {
			return companyRow.Locator("button[onclick^='activateCompany(']").Click()
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.Status())
		// activateCompany() reloads the Invalid Only list, which no longer shows the company.
		require.NoError(t, companyRow.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateDetached}))
		require.NoError(t, browser.NavigateTo("/admin/customer/companies?valid=valid&search="+customerID))
		require.NoError(t, companyRow.Locator(".gk-badge-success").WaitFor(), "the company should be valid again")
	})

	t.Run("Unknown company", func(t *testing.T) {
		resp, err := page.Goto(browser.Config.BaseURL + "/admin/customer/companies/NONEXISTENT/edit")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.Status())
		for _, action := range []string{"delete", "activate"} {
			status, body := companyRequest(t, browser, http.MethodPost, "/admin/customer/companies/NONEXISTENT/"+action, nil)
			assert.Equal(t, http.StatusNotFound, status, "%s of an unknown company: %s", action, body)
		}
	})
}

// TestAdminCustomerCompaniesHTTP checks the status codes of the customer
// company endpoints for an authenticated admin and for an anonymous client.
func TestAdminCustomerCompaniesHTTP(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	t.Cleanup(browser.TearDown)

	t.Run("Anonymous requests are sent to login", func(t *testing.T) {
		for _, path := range []string{"/admin/customer/companies", "/admin/customer/companies/TEST001/edit"} {
			status, _ := companyRequest(t, browser, http.MethodGet, path, nil)
			assert.Contains(t, []int{http.StatusFound, http.StatusSeeOther, http.StatusUnauthorized}, status, "GET %s without a session", path)
		}
		status, _ := companyRequest(t, browser, http.MethodPost, "/admin/customer/companies",
			map[string]string{"customer_id": "ANON_" + strconv.FormatInt(time.Now().UnixNano(), 10), "name": "Anonymous"})
		assert.Contains(t, []int{http.StatusFound, http.StatusSeeOther, http.StatusUnauthorized}, status, "anonymous create")
	})

	require.NoError(t, helpers.NewAuthHelper(browser).LoginAsAdmin(), "Failed to login as admin")

	t.Run("Admin pages", func(t *testing.T) {
		for _, path := range []string{
			"/admin/customer/companies",
			"/admin/customer/companies?search=test",
			"/admin/customer/companies?valid=valid",
			"/admin/customer/companies?valid=invalid",
			"/admin/customer/companies/new",
			"/admin/customer/companies/TEST001/edit",
			"/admin/customer/companies/TEST001/users",
			"/admin/customer/companies/TEST001/tickets",
			"/admin/customer/companies/TEST001/services",
		} {
			status, body := companyRequest(t, browser, http.MethodGet, path, nil)
			assert.Equal(t, http.StatusOK, status, "GET %s: %.200s", path, body)
		}
	})

	t.Run("Create validation", func(t *testing.T) {
		for _, form := range []map[string]string{
			{"invalid": "data"},
			{"customer_id": "ONLY_ID"},
			{"name": "Only Name"},
		} {
			status, body := companyRequest(t, browser, http.MethodPost, "/admin/customer/companies", form)
			assert.Equal(t, http.StatusBadRequest, status, "create %v: %s", form, body)
			assert.Contains(t, body, "required")
		}
		require.NoError(t, browser.NavigateTo("/admin/customer/companies?search=ONLY_ID"))
		assert.Equal(t, 0, count(t, browser.Page.Locator("table.gk-table tbody td:first-child div:text-is('ONLY_ID')")),
			"a rejected create must not insert a row")
	})
}
