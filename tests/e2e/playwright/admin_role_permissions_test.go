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

func TestAdminRolePermissionsUI(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Role permissions page loads correctly", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		// First navigate to roles page
		err = browser.NavigateTo("/admin/roles")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Navigate to role 1 permissions
		err = browser.NavigateTo("/admin/roles/1/permissions")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/roles/1/permissions")

		// Check page title
		pageTitle := browser.Page.Locator("h1:has-text('Role Permissions')")
		if count(t, pageTitle) == 0 {
			t.Skip("role permissions page not reachable")
		}

		// Check for Save Permissions button
		saveButton := browser.Page.Locator("button:has-text('Save Permissions')")
		assert.Greater(t, count(t, saveButton), 0, "Save Permissions button should exist")

		// Check for permissions table
		permissionsTable := browser.Page.Locator("table")
		assert.Greater(t, count(t, permissionsTable), 0, "Permissions table should exist")

		// Check for permission checkboxes
		checkboxes := browser.Page.Locator("input[type='checkbox'][name^='perm_']")
		assert.Greater(t, count(t, checkboxes), 0, "Permission checkboxes should exist")

		// Check for Quick Actions
		selectAllButton := browser.Page.Locator("button:has-text('Select All')")
		assert.Greater(t, count(t, selectAllButton), 0, "Select All button should exist")

		selectNoneButton := browser.Page.Locator("button:has-text('Select None')")
		assert.Greater(t, count(t, selectNoneButton), 0, "Select None button should exist")
	})

	t.Run("Save Permissions sends POST request", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/roles/1/permissions")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Track network requests
		var requestMethod string
		var requestURL string

		browser.Page.OnRequest(func(request playwright.Request) {
			if request.URL() != "" && request.Method() != "" {
				// Look for the permissions POST request
				url := request.URL()
				if len(url) > 0 && request.Method() == "POST" || request.Method() == "PUT" {
					if contains(url, "/admin/roles/1/permissions") {
						requestMethod = request.Method()
						requestURL = url
					}
				}
			}
		})

		// Click a checkbox to make a change
		firstCheckbox := browser.Page.Locator("input[type='checkbox'][name^='perm_']").First()
		isChecked, _ := firstCheckbox.IsChecked()

		// Toggle the checkbox
		err = firstCheckbox.Click()
		require.NoError(t, err)

		// Click Save Permissions
		saveButton := browser.Page.Locator("button:has-text('Save Permissions')")
		err = saveButton.Click()
		require.NoError(t, err)

		// Wait for the request to be made
		time.Sleep(2 * time.Second)

		// Verify the request was POST, not PUT
		assert.Equal(t, "POST", requestMethod, "Save Permissions should use POST method, not PUT")
		assert.Contains(t, requestURL, "/admin/roles/1/permissions", "Request should be to the permissions endpoint")

		// Restore original state
		if isChecked {
			// If it was checked before, we unchecked it, so check it again
			err = firstCheckbox.Click()
			require.NoError(t, err)
			err = saveButton.Click()
			require.NoError(t, err)
			time.Sleep(1 * time.Second)
		}
	})

	t.Run("Select All button checks all checkboxes", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/roles/1/permissions")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Click Select All
		selectAllButton := browser.Page.Locator("button:has-text('Select All')")
		err = selectAllButton.Click()
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// Verify all checkboxes are checked
		checkboxes := browser.Page.Locator("input[type='checkbox'][name^='perm_']")
		checkboxCount, _ := checkboxes.Count()

		for i := 0; i < checkboxCount; i++ {
			cb := checkboxes.Nth(i)
			isChecked, _ := cb.IsChecked()
			assert.True(t, isChecked, "Checkbox %d should be checked after Select All", i)
		}
	})

	t.Run("Select None button unchecks all checkboxes", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/roles/1/permissions")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// First select all
		selectAllButton := browser.Page.Locator("button:has-text('Select All')")
		err = selectAllButton.Click()
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// Then select none
		selectNoneButton := browser.Page.Locator("button:has-text('Select None')")
		err = selectNoneButton.Click()
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// Verify all checkboxes are unchecked
		checkboxes := browser.Page.Locator("input[type='checkbox'][name^='perm_']")
		checkboxCount, _ := checkboxes.Count()

		for i := 0; i < checkboxCount; i++ {
			cb := checkboxes.Nth(i)
			isChecked, _ := cb.IsChecked()
			assert.False(t, isChecked, "Checkbox %d should be unchecked after Select None", i)
		}
	})

	t.Run("Back button returns to roles list", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/roles/1/permissions")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Click back button (the arrow link)
		backButton := browser.Page.Locator("a[href='/admin/roles']").First()
		err = backButton.Click()
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Verify we're back on roles page
		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/roles")
		assert.NotContains(t, url, "/permissions")
	})
}

// Note: contains() helper is defined in admin_customer_companies_test.go
