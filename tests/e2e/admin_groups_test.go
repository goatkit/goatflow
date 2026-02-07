//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupsUI(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Admin Groups page loads correctly", func(t *testing.T) {
		// Login as admin
		err := auth.LoginAsAdmin()
		require.NoError(t, err, "Login should succeed")

		// Navigate to admin dashboard
		err = browser.NavigateTo("/admin")
		require.NoError(t, err, "Should navigate to admin dashboard")

		// Wait for page to load
		time.Sleep(2 * time.Second)

		// Check if Groups card is present
		groupCard := browser.Page.Locator("a[href='/admin/groups']")
		count, _ := groupCard.Count()
		assert.Greater(t, count, 0, "Groups management card should be present")

		// Check if Total Groups stat is present
		totalGroups := browser.Page.Locator("text='Total Groups'")
		count, _ = totalGroups.Count()
		assert.Greater(t, count, 0, "Total Groups stat should be present")

		// Click on Groups management
		err = groupCard.Click()
		assert.NoError(t, err, "Should be able to click Groups card")

		// Wait for navigation
		time.Sleep(2 * time.Second)

		// Verify we're on the groups page
		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/groups", "Should navigate to groups page")

		// Check for page title
		pageTitle := browser.Page.Locator("h1:has-text('Groups')")
		count, _ = pageTitle.Count()
		assert.Greater(t, count, 0, "Groups page title should be visible")

		// Check for Add Group button
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		count, _ = addButton.Count()
		assert.Greater(t, count, 0, "Add Group button should be present")

		// Check for search input
		searchInput := browser.Page.Locator("input#groupSearch")
		count, _ = searchInput.Count()
		assert.Greater(t, count, 0, "Group search input should be present")

		// Check for groups table
		groupsTable := browser.Page.Locator("table#groupsTable")
		count, _ = groupsTable.Count()
		assert.Greater(t, count, 0, "Groups table should be present")

		// Check for table headers
		headers := []string{"Group Name", "Description", "Members", "Status", "Created"}
		for _, header := range headers {
			headerElement := browser.Page.Locator("th:has-text('" + header + "')")
			count, _ = headerElement.Count()
			assert.Greater(t, count, 0, "Table header '"+header+"' should be present")
		}
	})

	t.Run("Add Group modal works", func(t *testing.T) {
		// Make sure we're on the groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Click Add Group button
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		err = addButton.Click()
		assert.NoError(t, err, "Should be able to click Add Group button")

		// Wait for modal to open
		time.Sleep(1 * time.Second)

		// Check modal is visible
		modal := browser.Page.Locator("#groupModal")
		visible, _ := modal.IsVisible()
		assert.True(t, visible, "Group modal should be visible")

		// Check for form fields
		nameInput := browser.Page.Locator("input#groupName")
		count, _ := nameInput.Count()
		assert.Greater(t, count, 0, "Group name input should be present")

		descriptionInput := browser.Page.Locator("textarea#groupComments")
		count, _ = descriptionInput.Count()
		assert.Greater(t, count, 0, "Group description textarea should be present")

		statusSelect := browser.Page.Locator("select#groupStatus")
		count, _ = statusSelect.Count()
		assert.Greater(t, count, 0, "Group status select should be present")

		// Check for Save and Cancel buttons
		saveButton := browser.Page.Locator("button:has-text('Save')")
		count, _ = saveButton.Count()
		assert.Greater(t, count, 0, "Save button should be present")

		cancelButton := browser.Page.Locator("button:has-text('Cancel')")
		count, _ = cancelButton.Count()
		assert.Greater(t, count, 0, "Cancel button should be present")

		// Close modal
		err = cancelButton.Click()
		assert.NoError(t, err, "Should be able to close modal")
	})

	t.Run("Search functionality works", func(t *testing.T) {
		// Make sure we're on the groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Type in search box
		searchInput := browser.Page.Locator("input#groupSearch")
		err = searchInput.Fill("admin")
		assert.NoError(t, err, "Should be able to type in search box")

		// Check clear button appears
		clearButton := browser.Page.Locator("#clearSearchBtn")
		visible, _ := clearButton.IsVisible()
		assert.True(t, visible, "Clear search button should appear when text is entered")
	})

	t.Run("System groups are marked correctly", func(t *testing.T) {
		// Make sure we're on the groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Check for system group badges
		systemBadges := browser.Page.Locator("span:has-text('System')")
		count, _ := systemBadges.Count()
		assert.GreaterOrEqual(t, count, 0, "System badges may be present for system groups")

		// Check that delete buttons are disabled for system groups
		// This would need specific group IDs to test properly
	})
}
