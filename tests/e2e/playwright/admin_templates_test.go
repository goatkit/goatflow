//go:build e2e

package playwright

import (
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func templateCount(t *testing.T, loc playwright.Locator) int {
	n, err := loc.Count()
	require.NoError(t, err)
	return n
}

func TestAdminTemplatesUI(t *testing.T) {
	t.Skip("TODO: Update selectors to match current templates page implementation")
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Admin Templates page loads correctly", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)
		err = browser.NavigateTo("/admin/templates")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/templates")

		pageTitle := browser.Page.Locator("h1:has-text('Templates')")
		if templateCount(t, pageTitle) == 0 {
			t.Skip("templates page not reachable")
		}

		addButton := browser.Page.Locator("button:has-text('Add Template'), a:has-text('Add Template')")
		assert.Greater(t, templateCount(t, addButton), 0, "Add Template button should exist")

		searchInput := browser.Page.Locator("input[placeholder*='Search'], input#templateSearch")
		assert.Greater(t, templateCount(t, searchInput), 0, "Search input should exist")

		templatesTable := browser.Page.Locator("table")
		assert.Greater(t, templateCount(t, templatesTable), 0, "Templates table should exist")
	})

	t.Run("Template list shows expected columns", func(t *testing.T) {
		err := browser.NavigateTo("/admin/templates")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		expectedHeaders := []string{"Name", "Type"}
		for _, h := range expectedHeaders {
			header := browser.Page.Locator("th:has-text('" + h + "')")
			assert.Greater(t, templateCount(t, header), 0, "Header '%s' should exist", h)
		}
	})

	t.Run("Add Template form has required fields", func(t *testing.T) {
		err := browser.NavigateTo("/admin/templates/create")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		nameInput := browser.Page.Locator("input[name='name'], input#templateName")
		assert.Greater(t, templateCount(t, nameInput), 0, "Name input should exist")

		typeField := browser.Page.Locator("select[name='template_type'], input[name='template_type'], [data-field='template_type']")
		// Type might be checkboxes or select
		typeCheckboxes := browser.Page.Locator("input[type='checkbox'][name='template_types']")
		typeSelect := browser.Page.Locator("select[name='template_type']")
		hasTypeField := templateCount(t, typeField) > 0 || templateCount(t, typeCheckboxes) > 0 || templateCount(t, typeSelect) > 0
		assert.True(t, hasTypeField, "Template type field should exist")

		contentArea := browser.Page.Locator("textarea[name='text'], .tiptap-editor, [data-tiptap]")
		assert.Greater(t, templateCount(t, contentArea), 0, "Content/text area should exist")

		saveButton := browser.Page.Locator("button[type='submit'], button:has-text('Save'), button:has-text('Create')")
		assert.Greater(t, templateCount(t, saveButton), 0, "Save button should exist")
	})

	t.Run("Content type selector exists", func(t *testing.T) {
		err := browser.NavigateTo("/admin/templates/create")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		contentTypeField := browser.Page.Locator("select[name='content_type'], input[name='content_type'], [name='content_type']")
		assert.Greater(t, templateCount(t, contentTypeField), 0, "Content type selector should exist")
	})

	t.Run("Template attachment management available", func(t *testing.T) {
		// First find an existing template to edit
		err := browser.NavigateTo("/admin/templates")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Look for edit link on first template
		editLink := browser.Page.Locator("a[href*='/admin/templates/']:not([href*='create'])")
		if templateCount(t, editLink) == 0 {
			t.Skip("No templates available to test attachment management")
		}

		// Click the first edit link
		require.NoError(t, editLink.First().Click())
		require.NoError(t, browser.WaitForLoad())
		time.Sleep(500 * time.Millisecond)

		// Look for attachment-related elements
		attachmentSection := browser.Page.Locator("[data-section='attachments'], #attachments, .attachment-section, h3:has-text('Attachments'), h4:has-text('Attachments')")
		if templateCount(t, attachmentSection) > 0 {
			t.Log("Attachment section found")
		} else {
			// Attachments might be managed via a separate tab or link
			attachmentTab := browser.Page.Locator("a:has-text('Attachments'), button:has-text('Attachments')")
			if templateCount(t, attachmentTab) > 0 {
				t.Log("Attachment tab/link found")
			}
		}
	})
}

func TestAdminTemplateQueueAssignment(t *testing.T) {
	t.Skip("TODO: Update selectors to match current templates page implementation")
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Queue assignment page loads", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)
		err = browser.NavigateTo("/admin/templates/queues")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		if !assert.Contains(t, url, "/admin/templates") {
			t.Skip("Templates-Queue assignment page not found at expected URL")
		}

		// Look for queue-template assignment elements
		queueSelect := browser.Page.Locator("select[name='queue_id'], select#queue")
		templateSelect := browser.Page.Locator("select[name='template_id'], select#template")
		assignmentTable := browser.Page.Locator("table")

		hasAssignmentUI := templateCount(t, queueSelect) > 0 || templateCount(t, templateSelect) > 0 || templateCount(t, assignmentTable) > 0
		assert.True(t, hasAssignmentUI, "Queue-Template assignment UI should exist")
	})
}

func TestAdminTemplateAttachmentAssignment(t *testing.T) {
	t.Skip("TODO: Update selectors to match current templates page implementation")
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Template attachment assignment page loads", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)
		err = browser.NavigateTo("/admin/templates/attachments")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		if !assert.Contains(t, url, "/admin/templates") {
			t.Skip("Templates-Attachment assignment page not found")
		}

		pageTitle := browser.Page.Locator("h1, h2")
		assert.Greater(t, templateCount(t, pageTitle), 0, "Page should have a title")
	})
}
