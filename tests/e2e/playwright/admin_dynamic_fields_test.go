//go:build e2e

package playwright

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
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

	// OTRS-compatible dynamic field names are alphanumeric only
	// (internal/api/dynamic_field_types.go); underscores are rejected with 400.
	fieldName := fmt.Sprintf("E2EField%d", time.Now().UnixNano())
	fieldRow := browser.Page.Locator(fmt.Sprintf("tr:has(td span:text-is('%s'))", fieldName))

	t.Run("Create a new dynamic field", func(t *testing.T) {
		addButton := browser.Page.Locator("header a[href='/admin/dynamic-fields/new']")
		require.NoError(t, addButton.Click(), "Should click Add Dynamic Field")
		require.NoError(t, browser.Page.WaitForURL("**/admin/dynamic-fields/new"))

		require.NoError(t, browser.Page.Locator("input[name='name']").Fill(fieldName))
		require.NoError(t, browser.Page.Locator("input[name='label']").Fill("E2E Field Label"))

		form := browser.Page.Locator("form#dynamicFieldForm")
		hxPost, err := form.GetAttribute("hx-post")
		require.NoError(t, err)
		assert.Equal(t, "/admin/api/dynamic-fields", hxPost, "Create form should use hx-post")

		// The create endpoint answers with HX-Redirect back to the list.
		require.NoError(t, browser.Page.Locator("form#dynamicFieldForm button[type='submit']").Click())
		require.NoError(t, browser.Page.WaitForURL("**/admin/dynamic-fields"), "Should redirect to list after create")

		require.NoError(t, fieldRow.WaitFor(), "New field should appear in list")
		label, err := fieldRow.Locator("td").Nth(1).TextContent()
		require.NoError(t, err)
		assert.Equal(t, "E2E Field Label", strings.TrimSpace(label))
	})

	t.Run("Edit an existing dynamic field", func(t *testing.T) {
		require.NoError(t, fieldRow.Locator("a[href^='/admin/dynamic-fields/']").Click(), "Should click the field's edit link")
		require.NoError(t, browser.Page.WaitForURL(regexp.MustCompile(`/admin/dynamic-fields/\d+$`)), "Should be on edit form")

		form := browser.Page.Locator("form#dynamicFieldForm")
		hxPut, err := form.GetAttribute("hx-put")
		require.NoError(t, err)
		assert.Regexp(t, `^/admin/api/dynamic-fields/\d+$`, hxPut, "Edit form should use hx-put with the field ID")
		hxPost, err := form.GetAttribute("hx-post")
		require.NoError(t, err)
		assert.Empty(t, hxPost, "Edit form should NOT have hx-post")

		newLabel := "Updated Label " + time.Now().Format("150405")
		require.NoError(t, browser.Page.Locator("input[name='label']").Fill(newLabel))
		require.NoError(t, form.Locator("button[type='submit']").Click())
		require.NoError(t, browser.Page.WaitForURL("**/admin/dynamic-fields"), "Should redirect to list after edit")

		require.NoError(t, fieldRow.WaitFor())
		label, err := fieldRow.Locator("td").Nth(1).TextContent()
		require.NoError(t, err)
		assert.Equal(t, newLabel, strings.TrimSpace(label), "list should show the updated label")
	})

	t.Run("Delete a dynamic field", func(t *testing.T) {
		browser.Page.OnDialog(func(dialog playwright.Dialog) {
			_ = dialog.Accept()
		})

		// deleteField() calls DELETE /admin/api/dynamic-fields/:id and reloads the list.
		deleted, err := browser.Page.ExpectResponse(
			regexp.MustCompile(`/admin/api/dynamic-fields/\d+$`),
			func() error { return fieldRow.Locator("button[onclick^='deleteField(']").Click() },
		)
		require.NoError(t, err, "Should click the field's delete button")
		assert.Equal(t, 200, deleted.Status())
		require.NoError(t, browser.WaitForLoad())

		require.NoError(t, fieldRow.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateDetached}),
			"deleted field should disappear from the list")
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

		// Edit link (icon-only anchor) on the seeded field's row.
		editButton := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s') a[href^='/admin/dynamic-fields/']", helpers.SeedDynamicFieldName)).First()
		count, err := editButton.Count()
		require.NoError(t, err)
		require.Greater(t, count, 0, "edit link for seeded dynamic field %q missing", helpers.SeedDynamicFieldName)

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
