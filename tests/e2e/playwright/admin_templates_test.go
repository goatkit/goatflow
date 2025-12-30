//go:build e2e

package playwright

import (
	"fmt"
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

		// Use data-testid selectors for stability
		pageTitle := browser.Page.Locator("[data-testid='page-title']")
		if templateCount(t, pageTitle) == 0 {
			t.Skip("templates page not reachable")
		}

		addButton := browser.Page.Locator("[data-testid='add-template-btn']")
		assert.Greater(t, templateCount(t, addButton), 0, "Add Template button should exist")

		searchInput := browser.Page.Locator("[data-testid='search-input']")
		assert.Greater(t, templateCount(t, searchInput), 0, "Search input should exist")

		// Table is only rendered when templates exist
		templatesTable := browser.Page.Locator("[data-testid='templates-table']")
		if templateCount(t, templatesTable) == 0 {
			t.Log("Templates table not rendered (no templates in test database)")
		}
	})

	t.Run("Template list shows expected columns", func(t *testing.T) {
		err := browser.NavigateTo("/admin/templates")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Only check headers if table exists (templates present)
		templatesTable := browser.Page.Locator("[data-testid='templates-table']")
		if templateCount(t, templatesTable) == 0 {
			t.Skip("No templates in test database - table not rendered")
		}

		expectedHeaders := []string{"Name", "Type"}
		for _, h := range expectedHeaders {
			header := browser.Page.Locator("th:has-text('" + h + "')")
			assert.Greater(t, templateCount(t, header), 0, "Header '%s' should exist", h)
		}
	})

	t.Run("Add Template form has required fields", func(t *testing.T) {
		err := browser.NavigateTo("/admin/templates/new")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Check page loaded correctly
		pageTitle := browser.Page.Locator("[data-testid='page-title']")
		if templateCount(t, pageTitle) == 0 {
			t.Skip("Template create page not reachable")
		}

		nameInput := browser.Page.Locator("[data-testid='name-input'], input[name='name']")
		assert.Greater(t, templateCount(t, nameInput), 0, "Name input should exist")

		// Template type checkboxes container
		typeContainer := browser.Page.Locator("[data-testid='template-types-container'], input[name='template_type']")
		assert.Greater(t, templateCount(t, typeContainer), 0, "Template type field should exist")

		// Content area (rich text editor or textarea)
		contentArea := browser.Page.Locator("textarea[name='text'], .tiptap-editor, [data-tiptap], .ProseMirror")
		assert.Greater(t, templateCount(t, contentArea), 0, "Content/text area should exist")

		saveButton := browser.Page.Locator("[data-testid='save-button'], button[type='submit']")
		assert.Greater(t, templateCount(t, saveButton), 0, "Save button should exist")
	})

	t.Run("Content type selector exists", func(t *testing.T) {
		err := browser.NavigateTo("/admin/templates/new")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		contentTypeField := browser.Page.Locator("[data-testid='content-type-select'], select[name='content_type']")
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

		// First go to templates list to find a template
		err = browser.NavigateTo("/admin/templates")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Look for a "Queues" action link on any template row
		queueLink := browser.Page.Locator("a[href*='/queues']")
		if templateCount(t, queueLink) == 0 {
			t.Skip("No queue assignment links found - templates may not exist or feature not enabled")
		}

		// Click the first queue assignment link
		require.NoError(t, queueLink.First().Click())
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		if !assert.Contains(t, url, "/queues") {
			t.Skip("Queue assignment page not found at expected URL")
		}

		// Look for queue-template assignment elements
		pageTitle := browser.Page.Locator("[data-testid='page-title']")
		queueCheckboxes := browser.Page.Locator("input[type='checkbox'][name='queue_ids']")
		assignmentForm := browser.Page.Locator("form")

		hasAssignmentUI := templateCount(t, pageTitle) > 0 || templateCount(t, queueCheckboxes) > 0 || templateCount(t, assignmentForm) > 0
		assert.True(t, hasAssignmentUI, "Queue-Template assignment UI should exist")
	})
}

func TestAdminTemplateAttachmentAssignment(t *testing.T) {
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

		// First go to templates list to find a template
		err = browser.NavigateTo("/admin/templates")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Look for an "Attachments" action link on any template row
		attachmentLink := browser.Page.Locator("a[href*='/attachments']")
		if templateCount(t, attachmentLink) == 0 {
			t.Skip("No attachment assignment links found - templates may not exist or feature not enabled")
		}

		// Click the first attachment assignment link
		require.NoError(t, attachmentLink.First().Click())
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		if !assert.Contains(t, url, "/attachments") {
			t.Skip("Attachment assignment page not found at expected URL")
		}

		pageTitle := browser.Page.Locator("h1, h2")
		assert.Greater(t, templateCount(t, pageTitle), 0, "Page should have a title")

		// Look for attachment-related elements
		attachmentCheckboxes := browser.Page.Locator("input[type='checkbox'][name='attachment_ids']")
		attachmentForm := browser.Page.Locator("form#attachmentAssignmentForm, form")
		uploadButton := browser.Page.Locator("button:has-text('Add'), button:has-text('Upload')")

		hasAttachmentUI := templateCount(t, attachmentCheckboxes) > 0 || templateCount(t, attachmentForm) > 0 || templateCount(t, uploadButton) > 0
		assert.True(t, hasAttachmentUI, "Attachment assignment UI should exist")
	})

	t.Run("Can upload attachment using bytes", func(t *testing.T) {
		// Navigate to attachments management page
		err := browser.NavigateTo("/admin/attachments")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Check if we have the add button (text is "Add Attachment")
		addButton := browser.Page.Locator("button:has-text('Add Attachment'), button:has-text('Add'), button[onclick*='openUploadModal']")
		if templateCount(t, addButton) == 0 {
			t.Skip("Add attachment button not found")
		}

		// Click add button to show modal
		require.NoError(t, addButton.First().Click())

		// Wait for modal to appear
		modal := browser.Page.Locator("#uploadModal")
		require.NoError(t, modal.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		}))

		// Fill in the name field
		nameInput := browser.Page.Locator("#uploadModal input[name='name'], #uploadModal input#name")
		require.Greater(t, templateCount(t, nameInput), 0, "Name input should exist in modal")
		testFileName := fmt.Sprintf("test-attachment-%d", time.Now().UnixNano())
		require.NoError(t, nameInput.First().Fill(testFileName))

		// Find the file input
		fileInput := browser.Page.Locator("#uploadModal input[type='file']")
		require.Greater(t, templateCount(t, fileInput), 0, "File input should exist in modal")

		// Upload file using bytes (not a static file)
		testContent := []byte("This is test content for E2E attachment upload test.\nGenerated at: " + time.Now().Format(time.RFC3339))
		require.NoError(t, fileInput.First().SetInputFiles(playwright.InputFile{
			Name:     "test-upload.txt",
			MimeType: "text/plain",
			Buffer:   testContent,
		}))

		// Submit the form
		submitButton := browser.Page.Locator("#uploadModal button[type='submit']")
		require.Greater(t, templateCount(t, submitButton), 0, "Submit button should exist in modal")
		require.NoError(t, submitButton.First().Click())

		// Wait for the modal to close and page to update
		require.NoError(t, modal.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateHidden,
			Timeout: playwright.Float(10000),
		}))

		// Refresh the page to see the new attachment
		_, err = browser.Page.Reload()
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Verify the attachment appears in the list
		attachmentRow := browser.Page.Locator(fmt.Sprintf("td:has-text('%s')", testFileName))

		// Check if upload was successful by looking for the name in the page
		count := templateCount(t, attachmentRow)
		if count == 0 {
			// Check for error messages
			errorMsg := browser.Page.Locator(".error, .alert-error, [class*='error']")
			if errCount := templateCount(t, errorMsg); errCount > 0 {
				text, _ := errorMsg.First().TextContent()
				t.Logf("Error message found: %s", text)
			}
			// Log page content for debugging
			content, _ := browser.Page.Content()
			if len(content) > 2000 {
				content = content[:2000]
			}
			t.Logf("Page content snippet: %s", content)
		}
		assert.Greater(t, count, 0, "Uploaded attachment should appear in the list")
	})
}
