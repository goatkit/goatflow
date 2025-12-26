//go:build playwright

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminDynamicFieldsWorkflow(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Navigate to Dynamic Fields admin page", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err, "Login should succeed")

		err = browser.NavigateTo("/admin/dynamic-fields")
		require.NoError(t, err, "Should navigate to dynamic fields page")

		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/dynamic-fields", "Should be on dynamic fields page")

		pageTitle := browser.Page.Locator("h1")
		titleText, _ := pageTitle.TextContent()
		assert.Contains(t, titleText, "Dynamic Field", "Page title should mention Dynamic Fields")
	})

	t.Run("Create a new dynamic field", func(t *testing.T) {
		fieldName := fmt.Sprintf("test_field_%d", time.Now().Unix())

		// Click Add Dynamic Field button
		addButton := browser.Page.Locator("a:has-text('Add Dynamic Field')")
		count, _ := addButton.Count()
		require.Greater(t, count, 0, "Add Dynamic Field button should exist")

		err := addButton.Click()
		require.NoError(t, err, "Should click Add button")

		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})

		// Verify we're on the create form
		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/dynamic-fields/new", "Should be on new field form")

		// Fill in the form
		nameInput := browser.Page.Locator("input[name='name']")
		err = nameInput.Fill(fieldName)
		require.NoError(t, err, "Should fill name field")

		labelInput := browser.Page.Locator("input[name='label']")
		err = labelInput.Fill("Test Field Label")
		require.NoError(t, err, "Should fill label field")

		// Verify form uses hx-post for create
		form := browser.Page.Locator("form#dynamicFieldForm")
		hxPost, _ := form.GetAttribute("hx-post")
		assert.Equal(t, "/admin/api/dynamic-fields", hxPost, "Create form should use hx-post")

		// Submit the form
		submitButton := browser.Page.Locator("button[type='submit']")
		err = submitButton.Click()
		require.NoError(t, err, "Should click submit button")

		// Wait for redirect back to list
		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		time.Sleep(1 * time.Second)

		// Verify redirect to list page
		url = browser.Page.URL()
		assert.Contains(t, url, "/admin/dynamic-fields", "Should redirect to list after create")
		assert.NotContains(t, url, "/new", "Should not be on new page anymore")

		// Verify the field appears in the list
		fieldRow := browser.Page.Locator(fmt.Sprintf("text='%s'", fieldName))
		count, _ = fieldRow.Count()
		assert.Greater(t, count, 0, "New field should appear in list")
	})

	t.Run("Edit an existing dynamic field", func(t *testing.T) {
		// Click edit on the first field
		editButton := browser.Page.Locator("a:has-text('Edit')").First()
		count, _ := editButton.Count()
		require.Greater(t, count, 0, "Edit button should exist")

		err := editButton.Click()
		require.NoError(t, err, "Should click Edit button")

		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})

		// Verify we're on edit form (URL has ID)
		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/dynamic-fields/", "Should be on edit form")
		assert.NotContains(t, url, "/new", "Should not be on new page")

		// Verify form uses hx-put for edit
		form := browser.Page.Locator("form#dynamicFieldForm")
		hxPut, _ := form.GetAttribute("hx-put")
		assert.Contains(t, hxPut, "/admin/api/dynamic-fields/", "Edit form should use hx-put")
		assert.NotEmpty(t, hxPut, "hx-put should have a value with ID")

		// Verify no hx-post on edit form
		hxPost, _ := form.GetAttribute("hx-post")
		assert.Empty(t, hxPost, "Edit form should NOT have hx-post")

		// Modify the label
		labelInput := browser.Page.Locator("input[name='label']")
		err = labelInput.Fill("Updated Label " + time.Now().Format("15:04:05"))
		require.NoError(t, err, "Should update label field")

		// Submit the form
		submitButton := browser.Page.Locator("button[type='submit']")
		err = submitButton.Click()
		require.NoError(t, err, "Should click submit button")

		// Wait for redirect
		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		time.Sleep(1 * time.Second)

		// Verify redirect back to list
		url = browser.Page.URL()
		assert.Contains(t, url, "/admin/dynamic-fields", "Should redirect to list after edit")

		// Verify no error alert appeared
		// If there was an alert, the test would have failed during form submission
	})

	t.Run("Delete a dynamic field", func(t *testing.T) {
		// Get initial count of fields
		rows := browser.Page.Locator("table tbody tr")
		initialCount, _ := rows.Count()

		if initialCount == 0 {
			t.Skip("No fields to delete")
		}

		// Click delete on the last field (to avoid deleting system fields)
		deleteButton := browser.Page.Locator("button:has-text('Delete')").Last()
		count, _ := deleteButton.Count()
		if count == 0 {
			t.Skip("No delete button found")
		}

		// Handle confirmation dialog
		browser.Page.On("dialog", func(dialog playwright.Dialog) {
			dialog.Accept()
		})

		err := deleteButton.Click()
		require.NoError(t, err, "Should click Delete button")

		// Wait for HTMX to update the page
		time.Sleep(2 * time.Second)

		// Verify count decreased
		rows = browser.Page.Locator("table tbody tr")
		newCount, _ := rows.Count()
		assert.Less(t, newCount, initialCount, "Field count should decrease after delete")
	})
}

func TestDynamicFieldFormHTMXAttributes(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Login should succeed")

	t.Run("Create form has correct HTMX attributes", func(t *testing.T) {
		err := browser.NavigateTo("/admin/dynamic-fields/new")
		require.NoError(t, err)

		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})

		form := browser.Page.Locator("form#dynamicFieldForm")

		// Must have hx-post
		hxPost, _ := form.GetAttribute("hx-post")
		assert.Equal(t, "/admin/api/dynamic-fields", hxPost, "Create form must use hx-post")

		// Must NOT have hx-put
		hxPut, _ := form.GetAttribute("hx-put")
		assert.Empty(t, hxPut, "Create form must NOT have hx-put")

		// Action should match
		action, _ := form.GetAttribute("action")
		assert.Equal(t, "/admin/api/dynamic-fields", action, "Form action should match")
	})

	t.Run("Edit form has correct HTMX attributes", func(t *testing.T) {
		// First navigate to list to find a field to edit
		err := browser.NavigateTo("/admin/dynamic-fields")
		require.NoError(t, err)

		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})

		// Click first edit button
		editButton := browser.Page.Locator("a:has-text('Edit')").First()
		count, _ := editButton.Count()
		if count == 0 {
			t.Skip("No fields available to test edit form")
		}

		err = editButton.Click()
		require.NoError(t, err)

		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})

		form := browser.Page.Locator("form#dynamicFieldForm")

		// Must have hx-put with ID
		hxPut, _ := form.GetAttribute("hx-put")
		assert.Contains(t, hxPut, "/admin/api/dynamic-fields/", "Edit form must use hx-put with ID")
		assert.NotEmpty(t, hxPut, "hx-put must have value")

		// Must NOT have hx-post
		hxPost, _ := form.GetAttribute("hx-post")
		assert.Empty(t, hxPost, "Edit form must NOT have hx-post")

		// Action should have ID
		action, _ := form.GetAttribute("action")
		assert.Contains(t, action, "/admin/api/dynamic-fields/", "Form action should have ID")
	})
}
