//go:build e2e

package playwright

import (
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminCustomerGroupsUI(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Customer groups page loads correctly", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/customer-groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/customer-groups")

		// Check page title
		pageTitle := browser.Page.Locator("h1:has-text('Customer Group Permissions')")
		assert.Greater(t, count(t, pageTitle), 0, "Page title should exist")

		// Check for two columns - Customers and Groups
		customersHeading := browser.Page.Locator("h3:has-text('Customer Companies')")
		assert.Greater(t, count(t, customersHeading), 0, "Customer Companies heading should exist")

		groupsHeading := browser.Page.Locator("h3:has-text('Groups')")
		assert.Greater(t, count(t, groupsHeading), 0, "Groups heading should exist")

		// Check for search inputs
		customerSearch := browser.Page.Locator("input#search")
		assert.Greater(t, count(t, customerSearch), 0, "Customer search input should exist")

		groupSearch := browser.Page.Locator("input#group-search")
		assert.Greater(t, count(t, groupSearch), 0, "Group search/filter input should exist")
	})

	t.Run("Customer search works", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/customer-groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Type in search and submit
		searchInput := browser.Page.Locator("input#search")
		err = searchInput.Fill("*")
		require.NoError(t, err)

		// Submit the form
		searchButton := browser.Page.Locator("button:has-text('Search')")
		err = searchButton.Click()
		require.NoError(t, err)

		require.NoError(t, browser.WaitForLoad())

		// URL should contain the search parameter
		url := browser.Page.URL()
		assert.Contains(t, url, "search=")
	})

	t.Run("Group filter works client-side", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/customer-groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Get initial count of visible groups
		groupsList := browser.Page.Locator("#groups-list li[data-group-name]")
		initialCount, _ := groupsList.Count()

		if initialCount == 0 {
			t.Skip("No groups available to test filter")
		}

		// Type in the filter - use a string that likely won't match
		filterInput := browser.Page.Locator("input#group-search")
		err = filterInput.Fill("zzzznonexistent")
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// Check that groups are filtered (hidden)
		visibleGroups := browser.Page.Locator("#groups-list li[data-group-name]:visible")
		visibleCount, _ := visibleGroups.Count()
		assert.Less(t, visibleCount, initialCount, "Filter should hide non-matching groups")

		// Clear the filter
		clearButton := browser.Page.Locator("button#clear-group-filter")
		err = clearButton.Click()
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// All groups should be visible again
		afterClearCount, _ := groupsList.Count()
		assert.Equal(t, initialCount, afterClearCount, "All groups should be visible after clearing filter")
	})

	t.Run("Customer edit page loads and saves with POST", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		// First, search for customers
		err = browser.NavigateTo("/admin/customer-groups?search=*")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Find first customer link
		customerLink := browser.Page.Locator("a[href^='/admin/customer-groups/customer/']").First()
		linkCount, _ := customerLink.Count()
		if linkCount == 0 {
			t.Skip("No customers available to test")
		}

		// Click to navigate to customer edit page
		err = customerLink.Click()
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/customer-groups/customer/")

		// Check for Save button
		saveButton := browser.Page.Locator("button:has-text('Save')")
		assert.Greater(t, count(t, saveButton), 0, "Save button should exist")

		// Check for permission checkboxes
		checkboxes := browser.Page.Locator("input[type='checkbox'][name^='permissions']")
		assert.Greater(t, count(t, checkboxes), 0, "Permission checkboxes should exist")

		// Track network requests to verify POST is used
		var requestMethod string
		browser.Page.OnRequest(func(request playwright.Request) {
			url := request.URL()
			if contains(url, "/admin/customer-groups/customer/") && (request.Method() == "POST" || request.Method() == "PUT") {
				requestMethod = request.Method()
			}
		})

		// Click Save
		err = saveButton.Click()
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		// Verify POST was used (the form should submit with POST)
		// Note: The form uses standard form submission which is POST by default
		assert.Equal(t, "POST", requestMethod, "Save should use POST method")
	})

	t.Run("Group edit page loads correctly", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/customer-groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Find first group link
		groupLink := browser.Page.Locator("a[href^='/admin/customer-groups/group/']").First()
		linkCount, _ := groupLink.Count()
		if linkCount == 0 {
			t.Skip("No groups available to test")
		}

		// Click to navigate to group edit page
		err = groupLink.Click()
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/customer-groups/group/")

		// Check for Save button
		saveButton := browser.Page.Locator("button:has-text('Save')")
		assert.Greater(t, count(t, saveButton), 0, "Save button should exist")

		// Check for Back link
		backLink := browser.Page.Locator("a[href='/admin/customer-groups']")
		assert.Greater(t, count(t, backLink), 0, "Back link should exist")
	})

	t.Run("Help section is visible", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/customer-groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Check for help section
		helpSection := browser.Page.Locator("h3:has-text('About Customer Group Permissions')")
		assert.Greater(t, count(t, helpSection), 0, "Help section should exist")

		// Check for permission explanations
		roExplanation := browser.Page.Locator("text=Read-only access")
		assert.Greater(t, count(t, roExplanation), 0, "RO permission explanation should exist")

		rwExplanation := browser.Page.Locator("text=Read-write access")
		assert.Greater(t, count(t, rwExplanation), 0, "RW permission explanation should exist")
	})
}
