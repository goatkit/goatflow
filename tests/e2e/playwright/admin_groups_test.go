//go:build e2e

package playwright

import (
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupsUI(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)

	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Admin Groups page loads correctly", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)
		err = browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())
		assert.Contains(t, browser.Page.URL(), "/admin/groups")
		// Heading is t("admin.group_management").
		pageTitle := browser.Page.Locator("h1:has-text('Group Management')")
		require.Equal(t, 1, count(t, pageTitle), "groups page heading missing")
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		assert.Equal(t, 1, count(t, addButton))
		searchInput := browser.Page.Locator("input#groupSearch")
		assert.Equal(t, 1, count(t, searchInput))
		groupsTable := browser.Page.Locator("table#groupsTable")
		require.Equal(t, 1, count(t, groupsTable))
		headers := []string{"Group Name", "Description", "Members", "Status", "Created"}
		for _, h := range headers {
			he := groupsTable.Locator("th:has-text('" + h + "')")
			assert.Equal(t, 1, count(t, he), "column header %q", h)
		}
		// The seeded OTRS "admin" group is always listed.
		adminRow := groupsTable.Locator("tbody tr:has-text('admin')")
		assert.Greater(t, count(t, adminRow), 0, "admin group row missing")
	})

	t.Run("Add Group modal works", func(t *testing.T) {
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		require.NoError(t, addButton.Click())
		modal := browser.Page.Locator("#groupModal")
		require.NoError(t, modal.WaitFor(), "Add Group modal should open")
		for _, sel := range []string{"input#groupName", "textarea#groupComments", "select#groupStatus"} {
			v, err := modal.Locator(sel).IsVisible()
			require.NoError(t, err)
			assert.True(t, v, "%s should be visible in the modal", sel)
		}
		assert.Equal(t, 1, count(t, modal.Locator("button[type='submit']:has-text('Save')")))
		cancelButton := modal.Locator("button:has-text('Cancel')")
		require.NoError(t, cancelButton.Click())
		require.NoError(t, modal.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateHidden}),
			"Cancel should close the modal")
	})
}
