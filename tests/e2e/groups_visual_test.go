//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupsUIVisual(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Visual verification of Groups page", func(t *testing.T) {
		// Login as admin
		err := auth.LoginAsAdmin()
		require.NoError(t, err, "Login should succeed")

		// Navigate to groups page
		err = browser.NavigateTo("/admin/groups")
		require.NoError(t, err, "Should navigate to groups page")
		time.Sleep(2 * time.Second)

		// Take screenshot for visual verification
		screenshot, err := browser.Page.Screenshot()
		if err == nil {
			// Save screenshot for manual review if needed
			t.Logf("Screenshot taken of groups page (length: %d bytes)", len(screenshot))
		}

		// Check that all text is properly translated (no dots in visible text)
		pageText, err := browser.Page.Locator("body").InnerText()
		require.NoError(t, err)

		// These should be translated
		assert.Contains(t, pageText, "Groups", "Should show 'Groups' not 'admin.groups'")
		assert.Contains(t, pageText, "Add Group", "Should show 'Add Group' not 'admin.add_group'")
		assert.Contains(t, pageText, "Group Name", "Should show 'Group Name' not 'admin.group_name'")
		assert.Contains(t, pageText, "Description", "Should show 'Description' not 'admin.description'")
		assert.Contains(t, pageText, "Members", "Should show 'Members' not 'admin.members'")
		assert.Contains(t, pageText, "Status", "Should show 'Status' not 'admin.status'")
		assert.Contains(t, pageText, "Created", "Should show 'Created' not 'admin.created'")

		// Check that translation keys are NOT visible
		assert.NotContains(t, pageText, "admin.groups", "Should not show raw translation key")
		assert.NotContains(t, pageText, "admin.add_group", "Should not show raw translation key")
		assert.NotContains(t, pageText, "admin.group_name", "Should not show raw translation key")
		assert.NotContains(t, pageText, "admin.description", "Should not show raw translation key")
		assert.NotContains(t, pageText, "admin.members", "Should not show raw translation key")
		assert.NotContains(t, pageText, "admin.status", "Should not show raw translation key")
		assert.NotContains(t, pageText, "admin.created", "Should not show raw translation key")
		assert.NotContains(t, pageText, "admin.no_groups_found", "Should not show raw translation key")
	})

	t.Run("Visual verification of Add Group modal", func(t *testing.T) {
		// Make sure we're on the groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Click Add Group button
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		err = addButton.Click()
		assert.NoError(t, err, "Should open modal")
		time.Sleep(1 * time.Second)

		// Get modal text
		modal := browser.Page.Locator("#groupModal")
		modalVisible, _ := modal.IsVisible()
		assert.True(t, modalVisible, "Modal should be visible")

		modalText, err := modal.InnerText()
		if err == nil {
			// Check modal has translated text
			assert.Contains(t, modalText, "Add Group", "Modal should show 'Add Group'")
			assert.Contains(t, modalText, "Group Name", "Modal should show 'Group Name'")
			assert.Contains(t, modalText, "Description", "Modal should show 'Description'")
			assert.Contains(t, modalText, "Status", "Modal should show 'Status'")
			assert.Contains(t, modalText, "Save", "Modal should show 'Save'")
			assert.Contains(t, modalText, "Cancel", "Modal should show 'Cancel'")

			// Check no translation keys visible
			assert.NotContains(t, modalText, "admin.", "Modal should not show translation keys")
			assert.NotContains(t, modalText, "app.", "Modal should not show translation keys")
			assert.NotContains(t, modalText, "common.", "Modal should not show translation keys")
		}

		// Close modal
		cancelButton := browser.Page.Locator("button:has-text('Cancel')")
		err = cancelButton.Click()
		assert.NoError(t, err, "Should close modal")
	})

	t.Run("Visual verification of admin dashboard Groups card", func(t *testing.T) {
		// Navigate to admin dashboard
		err := browser.NavigateTo("/admin")
		require.NoError(t, err, "Should navigate to admin dashboard")
		time.Sleep(2 * time.Second)

		// Check for Groups card
		groupCard := browser.Page.Locator("a[href='/admin/groups']")
		cardVisible, _ := groupCard.IsVisible()
		assert.True(t, cardVisible, "Groups card should be visible")

		cardText, err := groupCard.InnerText()
		if err == nil {
			// Should show translated text
			assert.Contains(t, cardText, "Group Management", "Card should show 'Group Management'")
			assert.NotContains(t, cardText, "admin.group_management", "Should not show translation key")
		}

		// Check Total Groups stat
		statsText, _ := browser.Page.Locator("body").InnerText()
		assert.Contains(t, statsText, "Total Groups", "Should show 'Total Groups' stat")
		assert.NotContains(t, statsText, "admin.total_groups", "Should not show translation key")
	})
}

func TestGroupsPageResponsiveness(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Groups page responsive on mobile", func(t *testing.T) {
		// Set mobile viewport
		err := browser.Page.SetViewportSize(375, 667) // iPhone size
		require.NoError(t, err)

		// Login and navigate
		err = auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Check that page is still functional
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		visible, _ := addButton.IsVisible()
		assert.True(t, visible, "Add button should be visible on mobile")

		// Check search is accessible
		searchInput := browser.Page.Locator("input#groupSearch")
		searchVisible, _ := searchInput.IsVisible()
		assert.True(t, searchVisible, "Search should be visible on mobile")
	})

	t.Run("Groups page responsive on tablet", func(t *testing.T) {
		// Set tablet viewport
		err := browser.Page.SetViewportSize(768, 1024) // iPad size
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Check layout
		table := browser.Page.Locator("table#groupsTable")
		tableVisible, _ := table.IsVisible()
		assert.True(t, tableVisible, "Table should be visible on tablet")
	})

	t.Run("Groups page responsive on desktop", func(t *testing.T) {
		// Set desktop viewport
		err := browser.Page.SetViewportSize(1920, 1080)
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// All elements should be visible
		pageElements := []string{
			"h1:has-text('Groups')",
			"button:has-text('Add Group')",
			"input#groupSearch",
			"table#groupsTable",
			"select#statusFilter",
		}

		for _, selector := range pageElements {
			elem := browser.Page.Locator(selector)
			visible, _ := elem.IsVisible()
			assert.True(t, visible, "Element %s should be visible on desktop", selector)
		}
	})
}
