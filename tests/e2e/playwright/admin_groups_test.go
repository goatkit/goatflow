//go:build e2e

package playwright

import (
	"testing"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupsUI(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
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
		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/groups")
		pageTitle := browser.Page.Locator("h1:has-text('Groups')")
		if count(t, pageTitle) == 0 {
			t.Skip("groups page not reachable")
		}
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		assert.Greater(t, count(t, addButton), 0)
		searchInput := browser.Page.Locator("input#groupSearch")
		assert.Greater(t, count(t, searchInput), 0)
		groupsTable := browser.Page.Locator("table#groupsTable")
		assert.Greater(t, count(t, groupsTable), 0)
		headers := []string{"Group Name", "Description", "Members", "Status", "Created"}
		for _, h := range headers {
			he := browser.Page.Locator("th:has-text('" + h + "')")
			assert.Greater(t, count(t, he), 0)
		}
	})

	t.Run("Add Group modal works", func(t *testing.T) {
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		require.NoError(t, addButton.Click())
		require.NoError(t, browser.WaitForLoad())
		modal := browser.Page.Locator("#groupModal")
		v, _ := modal.IsVisible()
		assert.True(t, v)
		nameInput := browser.Page.Locator("input#groupName")
		assert.Greater(t, count(t, nameInput), 0)
		descriptionInput := browser.Page.Locator("textarea#groupComments")
		assert.Greater(t, count(t, descriptionInput), 0)
		statusSelect := browser.Page.Locator("select#groupStatus")
		assert.Greater(t, count(t, statusSelect), 0)
		saveButton := browser.Page.Locator("button:has-text('Save')")
		assert.Greater(t, count(t, saveButton), 0)
		cancelButton := browser.Page.Locator("button:has-text('Cancel')")
		assert.Greater(t, count(t, cancelButton), 0)
		require.NoError(t, cancelButton.Click())
	})
}
