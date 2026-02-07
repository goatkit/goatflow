//go:build e2e

package playwright

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tarElementCount(t *testing.T, loc playwright.Locator) int {
	n, err := loc.Count()
	require.NoError(t, err)
	return n
}

// readTestDataFile reads a test data CSV file from testdata directory
func readTestDataFile(filename string) ([]byte, error) {
	// Try relative path from project root
	paths := []string{
		"testdata/ticket_attribute_relations/" + filename,
		"../../../testdata/ticket_attribute_relations/" + filename,
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("could not find test data file: %s", filename)
}

func TestAdminTicketAttributeRelationsUI(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Ticket Attribute Relations page loads correctly", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)
		err = browser.NavigateTo("/admin/ticket-attribute-relations")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/ticket-attribute-relations")

		// Page title should exist
		pageTitle := browser.Page.Locator("h1")
		assert.Greater(t, tarElementCount(t, pageTitle), 0, "Page title should exist")

		// Import button should exist
		importButton := browser.Page.Locator("a[href='/admin/ticket-attribute-relations/new']")
		assert.Greater(t, tarElementCount(t, importButton), 0, "Import CSV/Excel button should exist")

		// Table should exist (even if empty)
		relationsTable := browser.Page.Locator("table")
		assert.Greater(t, tarElementCount(t, relationsTable), 0, "Relations table should exist")

		// Check for expected column headers
		expectedHeaders := []string{"Priority", "Attribute", "Filename"}
		for _, h := range expectedHeaders {
			header := browser.Page.Locator(fmt.Sprintf("th:has-text('%s')", h))
			assert.Greater(t, tarElementCount(t, header), 0, "Header '%s' should exist", h)
		}
	})

	t.Run("New relation form has required fields", func(t *testing.T) {
		err := browser.NavigateTo("/admin/ticket-attribute-relations/new")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Check page loaded
		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/ticket-attribute-relations/new")

		// File input should exist
		fileInput := browser.Page.Locator("input[type='file'][name='file']")
		assert.Greater(t, tarElementCount(t, fileInput), 0, "File input should exist")

		// Priority selector should exist
		prioritySelect := browser.Page.Locator("select[name='priority']")
		assert.Greater(t, tarElementCount(t, prioritySelect), 0, "Priority selector should exist")

		// Checkbox for adding missing values should exist
		addMissingCheckbox := browser.Page.Locator("input[name='dynamic_field_config_update']")
		assert.Greater(t, tarElementCount(t, addMissingCheckbox), 0, "Add missing values checkbox should exist")

		// Save button should exist
		saveButton := browser.Page.Locator("button[type='submit']")
		assert.Greater(t, tarElementCount(t, saveButton), 0, "Save button should exist")

		// Cancel link should exist
		cancelLink := browser.Page.Locator("a[href='/admin/ticket-attribute-relations']")
		assert.Greater(t, tarElementCount(t, cancelLink), 0, "Cancel link should exist")
	})
}

func TestAdminTicketAttributeRelationsCSVImport(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	// Generate unique filename to avoid conflicts
	testFilename := fmt.Sprintf("e2e_queue_category_%d.csv", time.Now().UnixNano())

	t.Run("Can upload queue_category CSV file", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		// Navigate to new relation form
		err = browser.NavigateTo("/admin/ticket-attribute-relations/new")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Read the test data file
		csvContent, err := readTestDataFile("queue_category.csv")
		if err != nil {
			t.Skipf("Test data file not found: %v", err)
		}

		// Find and fill the file input
		fileInput := browser.Page.Locator("input[type='file'][name='file']")
		require.Greater(t, tarElementCount(t, fileInput), 0, "File input should exist")

		// Upload file using bytes
		err = fileInput.SetInputFiles(playwright.InputFile{
			Name:     testFilename,
			MimeType: "text/csv",
			Buffer:   csvContent,
		})
		require.NoError(t, err, "Should be able to set file input")

		// Select priority (use first available)
		prioritySelect := browser.Page.Locator("select[name='priority']")
		if tarElementCount(t, prioritySelect) > 0 {
			_, err = prioritySelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"1"},
			})
			if err != nil {
				t.Logf("Could not select priority 1, using default: %v", err)
			}
		}

		// Submit the form
		saveButton := browser.Page.Locator("button[type='submit']")
		require.Greater(t, tarElementCount(t, saveButton), 0, "Save button should exist")
		err = saveButton.Click()
		require.NoError(t, err)

		// Wait for response
		require.NoError(t, browser.WaitForHTMX())
		time.Sleep(1 * time.Second) // Allow for redirect

		// Check if we're back on list page or edit page (both are valid outcomes)
		url := browser.Page.URL()
		isSuccess := assert.True(t,
			url == browser.Config.BaseURL+"/admin/ticket-attribute-relations" ||
				url != browser.Config.BaseURL+"/admin/ticket-attribute-relations/new",
			"Should redirect after successful upload, got URL: %s", url)

		if !isSuccess {
			// Check for error messages
			errorMsg := browser.Page.Locator(".error, .alert-error, [class*='error']")
			if errCount := tarElementCount(t, errorMsg); errCount > 0 {
				text, _ := errorMsg.First().TextContent()
				t.Logf("Error message found: %s", text)
			}
		}
	})

	t.Run("Uploaded relation appears in list", func(t *testing.T) {
		// Navigate to list page
		err := browser.NavigateTo("/admin/ticket-attribute-relations")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Look for our uploaded file in the table
		// The filename should appear in the table
		filenameCell := browser.Page.Locator(fmt.Sprintf("td:has-text('%s')", testFilename))
		count := tarElementCount(t, filenameCell)

		if count == 0 {
			// Check if there are any relations at all
			anyRows := browser.Page.Locator("tbody tr:not(:has-text('No relations'))")
			rowCount := tarElementCount(t, anyRows)
			t.Logf("Found %d relation rows in table", rowCount)

			if rowCount > 0 {
				// Log first row content for debugging
				firstRow := anyRows.First()
				content, _ := firstRow.TextContent()
				t.Logf("First row content: %s", content)
			}
		}

		assert.Greater(t, count, 0, "Uploaded relation should appear in the list with filename: %s", testFilename)
	})

	t.Run("Relation shows correct attributes", func(t *testing.T) {
		// Find the row with our filename
		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			t.Skip("Relation row not found - may have been cleaned up")
		}

		// Check that Queue attribute is shown
		queueAttr := relationRow.Locator("td:has-text('Queue')")
		assert.Greater(t, tarElementCount(t, queueAttr), 0, "Should show Queue as Attribute 1")

		// Check that DynamicField_Category attribute is shown
		categoryAttr := relationRow.Locator("td:has-text('DynamicField_Category')")
		assert.Greater(t, tarElementCount(t, categoryAttr), 0, "Should show DynamicField_Category as Attribute 2")
	})

	t.Run("Can edit uploaded relation", func(t *testing.T) {
		// Find the row with our filename
		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			t.Skip("Relation row not found - may have been cleaned up")
		}

		// Click edit link
		editLink := relationRow.Locator("a[href*='/admin/ticket-attribute-relations/']")
		if tarElementCount(t, editLink) == 0 {
			t.Skip("Edit link not found")
		}

		err := editLink.First().Click()
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Should be on edit page
		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/ticket-attribute-relations/")
		assert.NotContains(t, url, "/new")

		// Should see the attribute values table
		attributeTable := browser.Page.Locator("table:has(th:has-text('Queue'))")
		assert.Greater(t, tarElementCount(t, attributeTable), 0, "Attribute values table should exist")

		// Should see some of the values from our CSV
		salesValue := browser.Page.Locator("td:has-text('Sales')")
		assert.Greater(t, tarElementCount(t, salesValue), 0, "Should show 'Sales' value from CSV")

		// Download link should exist
		downloadLink := browser.Page.Locator("a[href*='/download']")
		assert.Greater(t, tarElementCount(t, downloadLink), 0, "Download link should exist")
	})

	t.Run("Can submit edit form via PUT", func(t *testing.T) {
		// Navigate to list to find our relation
		err := browser.NavigateTo("/admin/ticket-attribute-relations")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Find the row with our filename
		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			t.Skip("Relation row not found - may have been cleaned up")
		}

		// Click edit link
		editLink := relationRow.Locator("a[href*='/admin/ticket-attribute-relations/']")
		if tarElementCount(t, editLink) == 0 {
			t.Skip("Edit link not found")
		}

		err = editLink.First().Click()
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Capture current URL to verify we're on edit page
		editURL := browser.Page.URL()
		assert.Contains(t, editURL, "/admin/ticket-attribute-relations/")
		assert.NotContains(t, editURL, "/new")

		// Change the priority if possible
		prioritySelect := browser.Page.Locator("select[name='priority']")
		if tarElementCount(t, prioritySelect) > 0 {
			// Try to change priority to a different value
			_, err = prioritySelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"2"},
			})
			if err != nil {
				t.Logf("Could not select priority 2, trying priority 1: %v", err)
				_, _ = prioritySelect.SelectOption(playwright.SelectOptionValues{
					Values: &[]string{"1"},
				})
			}
		}

		// Submit the form (this uses hx-put)
		// Click "Save and Finish" button (value="0") to redirect back to list
		saveButton := browser.Page.Locator("button[type='submit'][value='0']")
		require.Greater(t, tarElementCount(t, saveButton), 0, "Save and Finish button should exist")
		err = saveButton.Click()
		require.NoError(t, err)

		// Wait for HTMX response
		require.NoError(t, browser.WaitForHTMX())
		time.Sleep(1 * time.Second) // Allow for redirect

		// Check that we're redirected back to list (successful PUT)
		// or still on edit page without errors (also success)
		url := browser.Page.URL()

		// Either redirected to list or stayed on edit page is okay
		// The key is we should NOT get a 404 or error page
		isValidOutcome := url == browser.Config.BaseURL+"/admin/ticket-attribute-relations" ||
			url == editURL // Stayed on edit page

		if !isValidOutcome {
			// Check for error indicators
			errorIndicators := browser.Page.Locator(".error, .alert-error, [class*='error'], .text-red")
			if errCount := tarElementCount(t, errorIndicators); errCount > 0 {
				text, _ := errorIndicators.First().TextContent()
				t.Errorf("Error found after PUT submission: %s", text)
			}

			// Check for 404 page
			notFoundIndicator := browser.Page.Locator("body:has-text('404'), body:has-text('Not Found')")
			if tarElementCount(t, notFoundIndicator) > 0 {
				t.Error("PUT request resulted in 404 Not Found - check that hx-put is being used instead of hx-post with _method")
			}
		}

		assert.True(t, isValidOutcome, "PUT form submission should succeed, got URL: %s", url)
	})

	t.Run("Can delete uploaded relation", func(t *testing.T) {
		// Navigate back to list
		err := browser.NavigateTo("/admin/ticket-attribute-relations")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		// Find the row with our filename
		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			t.Skip("Relation row not found - nothing to delete")
		}

		// Get the relation ID from the row
		rowID, err := relationRow.GetAttribute("id")
		if err != nil || rowID == "" {
			t.Skip("Could not get row ID")
		}

		// Click delete button
		deleteButton := relationRow.Locator("button[onclick*='confirmDelete']")
		if tarElementCount(t, deleteButton) == 0 {
			t.Skip("Delete button not found")
		}

		err = deleteButton.Click()
		require.NoError(t, err)

		// Modal should appear
		modal := browser.Page.Locator("#deleteModal")
		err = modal.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5000),
		})
		require.NoError(t, err, "Delete modal should appear")

		// Verify filename is shown in modal
		filenameInModal := browser.Page.Locator("#deleteFilename")
		text, err := filenameInModal.TextContent()
		require.NoError(t, err)
		assert.Equal(t, testFilename, text, "Modal should show the filename being deleted")

		// Click confirm delete
		confirmButton := modal.Locator("button:has-text('Delete'):not(:has-text('Cancel'))")
		err = confirmButton.Click()
		require.NoError(t, err)

		// Wait for modal to close and row to be removed
		time.Sleep(1 * time.Second)

		// Modal should be hidden
		isHidden, _ := modal.IsHidden()
		assert.True(t, isHidden, "Modal should be hidden after delete")

		// Row should be gone
		relationRowAfter := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))
		assert.Equal(t, 0, tarElementCount(t, relationRowAfter), "Relation row should be removed after delete")
	})
}

func TestAdminTicketAttributeRelationsStatePriorityCSV(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	// Generate unique filename
	testFilename := fmt.Sprintf("e2e_state_priority_%d.csv", time.Now().UnixNano())

	t.Run("Can upload state_priority CSV file", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/ticket-attribute-relations/new")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		csvContent, err := readTestDataFile("state_priority.csv")
		if err != nil {
			t.Skipf("Test data file not found: %v", err)
		}

		fileInput := browser.Page.Locator("input[type='file'][name='file']")
		require.Greater(t, tarElementCount(t, fileInput), 0, "File input should exist")

		err = fileInput.SetInputFiles(playwright.InputFile{
			Name:     testFilename,
			MimeType: "text/csv",
			Buffer:   csvContent,
		})
		require.NoError(t, err)

		saveButton := browser.Page.Locator("button[type='submit']")
		err = saveButton.Click()
		require.NoError(t, err)

		require.NoError(t, browser.WaitForHTMX())
		time.Sleep(1 * time.Second)

		url := browser.Page.URL()
		assert.NotEqual(t, browser.Config.BaseURL+"/admin/ticket-attribute-relations/new", url,
			"Should redirect after upload")
	})

	t.Run("State-Priority relation shows correct attributes", func(t *testing.T) {
		err := browser.NavigateTo("/admin/ticket-attribute-relations")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			t.Skip("Relation row not found")
		}

		// Check that State attribute is shown
		stateAttr := relationRow.Locator("td:has-text('State')")
		assert.Greater(t, tarElementCount(t, stateAttr), 0, "Should show State as Attribute 1")

		// Check that Priority attribute is shown
		priorityAttr := relationRow.Locator("td:has-text('Priority')")
		assert.Greater(t, tarElementCount(t, priorityAttr), 0, "Should show Priority as Attribute 2")
	})

	// Cleanup: delete the test relation
	t.Run("Cleanup: delete state_priority relation", func(t *testing.T) {
		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			return // Nothing to clean up
		}

		deleteButton := relationRow.Locator("button[onclick*='confirmDelete']")
		if tarElementCount(t, deleteButton) > 0 {
			deleteButton.Click()
			modal := browser.Page.Locator("#deleteModal")
			modal.WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(5000),
			})
			confirmButton := modal.Locator("button:has-text('Delete'):not(:has-text('Cancel'))")
			confirmButton.Click()
			time.Sleep(500 * time.Millisecond)
		}
	})
}

func TestAdminTicketAttributeRelationsQueueServiceCSV(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	// Generate unique filename
	testFilename := fmt.Sprintf("e2e_queue_service_%d.csv", time.Now().UnixNano())

	t.Run("Can upload queue_service CSV file", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		err = browser.NavigateTo("/admin/ticket-attribute-relations/new")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		csvContent, err := readTestDataFile("queue_service.csv")
		if err != nil {
			t.Skipf("Test data file not found: %v", err)
		}

		fileInput := browser.Page.Locator("input[type='file'][name='file']")
		require.Greater(t, tarElementCount(t, fileInput), 0, "File input should exist")

		err = fileInput.SetInputFiles(playwright.InputFile{
			Name:     testFilename,
			MimeType: "text/csv",
			Buffer:   csvContent,
		})
		require.NoError(t, err)

		saveButton := browser.Page.Locator("button[type='submit']")
		err = saveButton.Click()
		require.NoError(t, err)

		require.NoError(t, browser.WaitForHTMX())
		time.Sleep(1 * time.Second)

		url := browser.Page.URL()
		assert.NotEqual(t, browser.Config.BaseURL+"/admin/ticket-attribute-relations/new", url,
			"Should redirect after upload")
	})

	t.Run("Queue-Service relation shows correct attributes", func(t *testing.T) {
		err := browser.NavigateTo("/admin/ticket-attribute-relations")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			t.Skip("Relation row not found")
		}

		// Check that Queue attribute is shown
		queueAttr := relationRow.Locator("td:has-text('Queue')")
		assert.Greater(t, tarElementCount(t, queueAttr), 0, "Should show Queue as Attribute 1")

		// Check that Service attribute is shown
		serviceAttr := relationRow.Locator("td:has-text('Service')")
		assert.Greater(t, tarElementCount(t, serviceAttr), 0, "Should show Service as Attribute 2")
	})

	// Cleanup: delete the test relation
	t.Run("Cleanup: delete queue_service relation", func(t *testing.T) {
		relationRow := browser.Page.Locator(fmt.Sprintf("tr:has(td:has-text('%s'))", testFilename))

		if tarElementCount(t, relationRow) == 0 {
			return // Nothing to clean up
		}

		deleteButton := relationRow.Locator("button[onclick*='confirmDelete']")
		if tarElementCount(t, deleteButton) > 0 {
			deleteButton.Click()
			modal := browser.Page.Locator("#deleteModal")
			modal.WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateVisible,
				Timeout: playwright.Float(5000),
			})
			confirmButton := modal.Locator("button:has-text('Delete'):not(:has-text('Cancel'))")
			confirmButton.Click()
			time.Sleep(500 * time.Millisecond)
		}
	})
}
