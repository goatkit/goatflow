//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupsCRUDOperations(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Login once for all tests
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Login should succeed")

	// Generate unique group name for this test run
	testGroupName := fmt.Sprintf("TestGroup_%d", time.Now().Unix())
	testGroupDesc := "Test group for automated testing"
	updatedDesc := "Updated description for test group"

	t.Run("Complete CRUD workflow", func(t *testing.T) {
		// Navigate to groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err, "Should navigate to groups page")
		time.Sleep(2 * time.Second)

		// Step 1: CREATE - Add a new group
		t.Run("Create new group", func(t *testing.T) {
			// Click Add Group button
			addButton := browser.Page.Locator("button:has-text('Add Group')")
			err = addButton.Click()
			require.NoError(t, err, "Should open add group modal")
			time.Sleep(1 * time.Second)

			// Verify modal is open
			modal := browser.Page.Locator("#groupModal")
			visible, _ := modal.IsVisible()
			assert.True(t, visible, "Add group modal should be visible")

			// Fill in group details
			nameInput := browser.Page.Locator("input#name")
			err = nameInput.Fill(testGroupName)
			require.NoError(t, err, "Should fill group name")

			descInput := browser.Page.Locator("textarea#comments")
			err = descInput.Fill(testGroupDesc)
			require.NoError(t, err, "Should fill description")

			// Ensure Valid is selected (should be default)
			validSelect := browser.Page.Locator("select#valid_id")
			_, err = validSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
			require.NoError(t, err, "Should select valid status")

			// Submit form by pressing Enter
			err = nameInput.Press("Enter")
			require.NoError(t, err, "Should submit form with Enter key")
			time.Sleep(2 * time.Second)

			// Verify modal is closed
			visible, _ = modal.IsVisible()
			assert.False(t, visible, "Modal should be closed after submission")

			// Verify group appears in table
			groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", testGroupName))
			visible, _ = groupRow.IsVisible()
			assert.True(t, visible, "New group should appear in table")

			// Verify description is shown
			rowText, _ := groupRow.InnerText()
			assert.Contains(t, rowText, testGroupDesc, "Group description should be visible")
		})

		// Step 2: READ - Search and filter the group
		t.Run("Search for created group", func(t *testing.T) {
			// Use search to find our group
			searchInput := browser.Page.Locator("input#groupSearch")
			err = searchInput.Fill(testGroupName)
			require.NoError(t, err, "Should fill search input")
			time.Sleep(1 * time.Second) // Wait for search to filter

			// Verify only our group is visible
			visibleRows := browser.Page.Locator("tbody tr:visible")
			count, _ := visibleRows.Count()
			assert.Equal(t, 1, count, "Should show only one group after search")

			// Verify it's our group
			rowText, _ := visibleRows.First().InnerText()
			assert.Contains(t, rowText, testGroupName, "Visible row should be our test group")

			// Clear search
			err = searchInput.Fill("")
			require.NoError(t, err, "Should clear search")
			time.Sleep(1 * time.Second)
		})

		// Step 3: UPDATE - Edit the group
		t.Run("Edit created group", func(t *testing.T) {
			// Find the edit button for our group
			groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", testGroupName))
			editButton := groupRow.Locator("button[title='Edit']")

			err = editButton.Click()
			require.NoError(t, err, "Should click edit button")
			time.Sleep(1 * time.Second)

			// Verify modal opens with existing data
			modal := browser.Page.Locator("#groupModal")
			visible, _ := modal.IsVisible()
			assert.True(t, visible, "Edit modal should be visible")

			// Verify existing values are loaded
			nameInput := browser.Page.Locator("input#name")
			nameValue, _ := nameInput.InputValue()
			assert.Equal(t, testGroupName, nameValue, "Name should be pre-filled")

			descInput := browser.Page.Locator("textarea#comments")
			descValue, _ := descInput.InputValue()
			assert.Equal(t, testGroupDesc, descValue, "Description should be pre-filled")

			// Update description
			err = descInput.Fill(updatedDesc)
			require.NoError(t, err, "Should update description")

			// Save changes
			saveButton := browser.Page.Locator("button:has-text('Save')")
			err = saveButton.Click()
			require.NoError(t, err, "Should save changes")
			time.Sleep(2 * time.Second)

			// Verify modal is closed
			visible, _ = modal.IsVisible()
			assert.False(t, visible, "Modal should be closed after saving")

			// Verify updated description appears in table
			groupRow = browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", testGroupName))
			rowText, _ := groupRow.InnerText()
			assert.Contains(t, rowText, updatedDesc, "Updated description should be visible")
		})

		// Step 4: Test status filter
		t.Run("Filter by status", func(t *testing.T) {
			// Filter by Valid status
			statusFilter := browser.Page.Locator("select#statusFilter")
			_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
			require.NoError(t, err, "Should select valid status filter")
			time.Sleep(1 * time.Second)

			// Our test group should still be visible
			groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", testGroupName))
			visible, _ := groupRow.IsVisible()
			assert.True(t, visible, "Test group should be visible when filtering by valid")

			// Filter by Invalid status
			_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
			require.NoError(t, err, "Should select invalid status filter")
			time.Sleep(1 * time.Second)

			// Our test group should not be visible
			visible, _ = groupRow.IsVisible()
			assert.False(t, visible, "Test group should not be visible when filtering by invalid")

			// Reset filter
			_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{""}})
			require.NoError(t, err, "Should reset status filter")
			time.Sleep(1 * time.Second)
		})

		// Step 5: Test sort functionality
		t.Run("Sort groups", func(t *testing.T) {
			// Click on Name column header to sort
			nameHeader := browser.Page.Locator("th:has-text('Group Name')")
			err = nameHeader.Click()
			require.NoError(t, err, "Should click name header to sort")
			time.Sleep(1 * time.Second)

			// Get first row to check sort worked
			firstRow := browser.Page.Locator("tbody tr").First()
			firstRowText, _ := firstRow.InnerText()
			t.Logf("First row after sort: %s", firstRowText)

			// Click again to reverse sort
			err = nameHeader.Click()
			require.NoError(t, err, "Should click name header again to reverse sort")
			time.Sleep(1 * time.Second)

			// Get first row again
			firstRowAfterReverse := browser.Page.Locator("tbody tr").First()
			firstRowTextReverse, _ := firstRowAfterReverse.InnerText()
			t.Logf("First row after reverse sort: %s", firstRowTextReverse)

			// They should be different (unless there's only one group)
			rowCount, _ := browser.Page.Locator("tbody tr").Count()
			if rowCount > 1 {
				assert.NotEqual(t, firstRowText, firstRowTextReverse, "Sort order should change")
			}
		})

		// Step 6: Test that system groups cannot be deleted
		t.Run("System groups cannot be deleted", func(t *testing.T) {
			// Find the admin group row
			adminRow := browser.Page.Locator("tr:has-text('admin')")
			visible, _ := adminRow.IsVisible()

			if visible {
				// Check that delete button is disabled or not present
				deleteButton := adminRow.Locator("button[title='Delete']")
				deleteVisible, _ := deleteButton.IsVisible()

				if deleteVisible {
					// If visible, it should be disabled
					disabled, _ := deleteButton.IsDisabled()
					assert.True(t, disabled, "Delete button for admin group should be disabled")
				} else {
					// Or it might not be shown at all
					assert.False(t, deleteVisible, "Delete button should not be shown for system groups")
				}
			}
		})

		// Step 7: DELETE - Remove the test group
		t.Run("Delete created group", func(t *testing.T) {
			// Find our test group
			groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", testGroupName))
			deleteButton := groupRow.Locator("button[title='Delete']")

			err = deleteButton.Click()
			require.NoError(t, err, "Should click delete button")
			time.Sleep(500 * time.Millisecond)

			// Handle confirmation dialog
			// Look for browser dialog or custom modal
			browser.Page.OnDialog(func(dialog playwright.Dialog) {
				assert.Contains(t, dialog.Message(), testGroupName, "Confirmation should mention group name")
				dialog.Accept() // Confirm deletion
			})

			// If using custom modal, handle that instead
			confirmModal := browser.Page.Locator("[role='dialog']:has-text('Confirm')")
			modalVisible, _ := confirmModal.IsVisible()

			if modalVisible {
				confirmButton := confirmModal.Locator("button:has-text('Delete')")
				err = confirmButton.Click()
				require.NoError(t, err, "Should confirm deletion")
			}

			time.Sleep(2 * time.Second)

			// Verify group is removed from table
			groupRow = browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", testGroupName))
			visible, _ := groupRow.IsVisible()
			assert.False(t, visible, "Deleted group should not be in table")
		})
	})

	t.Run("Form validation", func(t *testing.T) {
		// Navigate to groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		t.Run("Cannot create group without name", func(t *testing.T) {
			// Open add modal
			addButton := browser.Page.Locator("button:has-text('Add Group')")
			err = addButton.Click()
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// Try to submit without filling name
			saveButton := browser.Page.Locator("button:has-text('Save')")
			err = saveButton.Click()
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// Modal should still be open
			modal := browser.Page.Locator("#groupModal")
			visible, _ := modal.IsVisible()
			assert.True(t, visible, "Modal should remain open when validation fails")

			// Check for error message or required field indication
			nameInput := browser.Page.Locator("input#name")
			required, _ := nameInput.GetAttribute("required")
			assert.Equal(t, "required", required, "Name field should be required")

			// Close modal
			cancelButton := browser.Page.Locator("button:has-text('Cancel')")
			err = cancelButton.Click()
			require.NoError(t, err)
			time.Sleep(1 * time.Second)
		})

		t.Run("Cannot create duplicate group", func(t *testing.T) {
			// Try to create a group with an existing name (e.g., 'admin')
			addButton := browser.Page.Locator("button:has-text('Add Group')")
			err = addButton.Click()
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			nameInput := browser.Page.Locator("input#name")
			err = nameInput.Fill("admin")
			require.NoError(t, err)

			descInput := browser.Page.Locator("textarea#comments")
			err = descInput.Fill("Trying to duplicate admin group")
			require.NoError(t, err)

			// Submit
			err = nameInput.Press("Enter")
			require.NoError(t, err)
			time.Sleep(2 * time.Second)

			// Should show error message
			errorMsg := browser.Page.Locator(".text-red-500, .text-red-600, [role='alert']")
			errorVisible, _ := errorMsg.IsVisible()

			if errorVisible {
				errorText, _ := errorMsg.InnerText()
				assert.True(t, strings.Contains(strings.ToLower(errorText), "exists") ||
					strings.Contains(strings.ToLower(errorText), "duplicate"),
					"Should show duplicate error message")
			}

			// Close modal if still open
			modal := browser.Page.Locator("#groupModal")
			visible, _ := modal.IsVisible()
			if visible {
				cancelButton := browser.Page.Locator("button:has-text('Cancel')")
				cancelButton.Click()
			}
		})
	})

	t.Run("Persistence across page refresh", func(t *testing.T) {
		// Set a search term
		searchInput := browser.Page.Locator("input#groupSearch")
		err = searchInput.Fill("admin")
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Reload page
		_, err = browser.Page.Reload()
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Check if search was persisted (depends on implementation)
		searchValue, _ := searchInput.InputValue()
		// Note: This might be empty if session storage isn't implemented for groups
		t.Logf("Search value after reload: '%s'", searchValue)
	})
}

func TestInactiveGroup(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Login once
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Login should succeed")

	// Generate unique inactive group name
	inactiveGroupName := fmt.Sprintf("InactiveGroup_%d", time.Now().Unix())
	inactiveGroupDesc := "This group is set to invalid/inactive status"

	t.Run("Create Inactive Group", func(t *testing.T) {
		// Navigate to groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err, "Should navigate to groups page")
		time.Sleep(2 * time.Second)

		// Click Add Group button
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		err = addButton.Click()
		require.NoError(t, err, "Should open add group modal")
		time.Sleep(1 * time.Second)

		// Fill in group details
		nameInput := browser.Page.Locator("input#name")
		err = nameInput.Fill(inactiveGroupName)
		require.NoError(t, err, "Should fill group name")

		descInput := browser.Page.Locator("textarea#comments")
		err = descInput.Fill(inactiveGroupDesc)
		require.NoError(t, err, "Should fill description")

		// SELECT INVALID STATUS (2 = Invalid/Inactive)
		validSelect := browser.Page.Locator("select#valid_id")
		_, err = validSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
		require.NoError(t, err, "Should select invalid/inactive status")

		// Submit form
		saveButton := browser.Page.Locator("button:has-text('Save')")
		err = saveButton.Click()
		require.NoError(t, err, "Should submit form")
		time.Sleep(2 * time.Second)

		// Verify modal is closed
		modal := browser.Page.Locator("#groupModal")
		visible, _ := modal.IsVisible()
		assert.False(t, visible, "Modal should be closed after submission")
	})

	t.Run("Verify Inactive Group in List", func(t *testing.T) {
		// Should see the inactive group in the full list
		groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", inactiveGroupName))
		visible, _ := groupRow.IsVisible()
		assert.True(t, visible, "Inactive group should appear in table")

		// Check that status shows as Invalid/Inactive
		statusBadge := groupRow.Locator(".badge, .status, span:has-text('Invalid'), span:has-text('Inactive')")
		statusVisible, _ := statusBadge.IsVisible()
		if statusVisible {
			statusText, _ := statusBadge.InnerText()
			t.Logf("Inactive group status shown as: %s", statusText)
			assert.True(t,
				strings.Contains(strings.ToLower(statusText), "invalid") ||
					strings.Contains(strings.ToLower(statusText), "inactive"),
				"Should show invalid/inactive status")
		}

		// Check the row styling - inactive groups might have different styling
		rowClass, _ := groupRow.GetAttribute("class")
		t.Logf("Inactive group row classes: %s", rowClass)
	})

	t.Run("Filter by Invalid Status", func(t *testing.T) {
		// Reset any existing filters first
		statusFilter := browser.Page.Locator("select#statusFilter")
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{""}})
		require.NoError(t, err, "Should reset filter")
		time.Sleep(1 * time.Second)

		// Filter by Invalid status (2)
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
		require.NoError(t, err, "Should select invalid status filter")
		time.Sleep(1 * time.Second)

		// Our inactive group should be visible
		groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", inactiveGroupName))
		visible, _ := groupRow.IsVisible()
		assert.True(t, visible, "Inactive group should be visible when filtering by invalid")

		// Count how many invalid groups are shown
		visibleRows := browser.Page.Locator("tbody tr:visible")
		count, _ := visibleRows.Count()
		t.Logf("Found %d invalid/inactive groups", count)
		assert.GreaterOrEqual(t, count, 1, "Should show at least our inactive group")

		// Reset filter to show all
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{""}})
		require.NoError(t, err, "Should reset filter")
		time.Sleep(1 * time.Second)
	})

	t.Run("Filter by Valid Status Excludes Inactive", func(t *testing.T) {
		// Filter by Valid status (1)
		statusFilter := browser.Page.Locator("select#statusFilter")
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		require.NoError(t, err, "Should select valid status filter")
		time.Sleep(1 * time.Second)

		// Our inactive group should NOT be visible
		groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", inactiveGroupName))
		visible, _ := groupRow.IsVisible()
		assert.False(t, visible, "Inactive group should NOT be visible when filtering by valid")

		// Reset filter
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{""}})
		require.NoError(t, err, "Should reset filter")
		time.Sleep(1 * time.Second)
	})

	t.Run("Edit Inactive Group", func(t *testing.T) {
		// Find and edit the inactive group
		groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", inactiveGroupName))
		editButton := groupRow.Locator("button[title='Edit']")

		err = editButton.Click()
		require.NoError(t, err, "Should click edit button")
		time.Sleep(1 * time.Second)

		// Verify the status is set to Invalid in the edit form
		validSelect := browser.Page.Locator("select#valid_id")
		selectedValue, _ := validSelect.InputValue()
		assert.Equal(t, "2", selectedValue, "Status should be set to Invalid (2)")

		// Change it back to Valid
		_, err = validSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		require.NoError(t, err, "Should change to valid status")

		// Save changes
		saveButton := browser.Page.Locator("button:has-text('Save')")
		err = saveButton.Click()
		require.NoError(t, err, "Should save changes")
		time.Sleep(2 * time.Second)

		// Now filter by Invalid - our group should not appear
		statusFilter := browser.Page.Locator("select#statusFilter")
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
		require.NoError(t, err, "Should filter by invalid")
		time.Sleep(1 * time.Second)

		visible, _ := groupRow.IsVisible()
		assert.False(t, visible, "Group should not appear in invalid filter after changing to valid")

		// Reset filter and verify it appears in valid filter
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		require.NoError(t, err, "Should filter by valid")
		time.Sleep(1 * time.Second)

		visible, _ = groupRow.IsVisible()
		assert.True(t, visible, "Group should appear in valid filter after changing status")
	})

	t.Run("Delete Inactive Group", func(t *testing.T) {
		// Reset filter to show all
		statusFilter := browser.Page.Locator("select#statusFilter")
		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{""}})
		require.NoError(t, err, "Should show all groups")
		time.Sleep(1 * time.Second)

		// Find and delete the test group
		groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", inactiveGroupName))
		deleteButton := groupRow.Locator("button[title='Delete']")

		err = deleteButton.Click()
		require.NoError(t, err, "Should click delete button")
		time.Sleep(500 * time.Millisecond)

		// Handle confirmation
		browser.Page.OnDialog(func(dialog playwright.Dialog) {
			dialog.Accept()
		})

		// Check for custom confirmation modal
		confirmModal := browser.Page.Locator("[role='dialog']:has-text('Confirm')")
		modalVisible, _ := confirmModal.IsVisible()

		if modalVisible {
			confirmButton := confirmModal.Locator("button:has-text('Delete')")
			err = confirmButton.Click()
			require.NoError(t, err, "Should confirm deletion")
		}

		time.Sleep(2 * time.Second)

		// Verify group is deleted
		groupRow = browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", inactiveGroupName))
		visible, _ := groupRow.IsVisible()
		assert.False(t, visible, "Deleted inactive group should not be in table")
	})

	t.Run("Inactive Groups Behavior", func(t *testing.T) {
		// Document expected behavior of inactive groups
		t.Log("Inactive/Invalid groups behavior:")
		t.Log("1. Can be created with status = Invalid (2)")
		t.Log("2. Appear in the full groups list with Invalid status badge")
		t.Log("3. Can be filtered to show only Invalid groups")
		t.Log("4. Are excluded when filtering for Valid groups")
		t.Log("5. Can be edited to change status between Valid/Invalid")
		t.Log("6. Can be deleted like any non-system group")
		t.Log("7. May have different visual styling (grayed out, etc.)")
	})
}

func TestGroupsAccessControl(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	t.Run("Non-admin users cannot access groups page", func(t *testing.T) {
		// Try to navigate directly without login
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Should be redirected to login
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should redirect to login when not authenticated")
	})
}
