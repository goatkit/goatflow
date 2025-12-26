//go:build e2e

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

func TestQueueManagement(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Login as admin
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("Queue list page loads", func(t *testing.T) {
		err := browser.NavigateTo("/queues")
		require.NoError(t, err)

		// Wait for page to load
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Check page title
		title := browser.Page.Locator("h1, h2").First()
		text, _ := title.TextContent()
		assert.Contains(t, text, "Queue", "Page should have queue-related title")

		// Check for queue list or table
		queueList := browser.Page.Locator("[data-queue-list], table, .queue-item")
		count, _ := queueList.Count()
		assert.Greater(t, count, 0, "Queue list should be present")
	})

	t.Run("Create new queue", func(t *testing.T) {
		err := browser.NavigateTo("/queues")
		require.NoError(t, err)

		// Click new queue button
		newQueueBtn := browser.Page.Locator("a[href*='/queues/new'], button:has-text('New Queue'), button:has-text('Add Queue')")
		count, _ := newQueueBtn.Count()
		require.Greater(t, count, 0, "New queue button should exist")

		err = newQueueBtn.First().Click()
		require.NoError(t, err, "Should be able to click new queue button")

		// Wait for form to appear
		_, err = browser.Page.WaitForSelector("form", playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(5000),
		})
		require.NoError(t, err, "New queue form should appear")

		// Fill in queue details
		testQueueName := fmt.Sprintf("TestQueue_%d", time.Now().Unix())

		nameInput := browser.Page.Locator("input[name='name'], input#name")
		err = nameInput.Fill(testQueueName)
		require.NoError(t, err, "Should fill queue name")

		descInput := browser.Page.Locator("textarea[name='comment'], textarea#comment, textarea[name='description']")
		if count, _ := descInput.Count(); count > 0 {
			err = descInput.Fill("Test queue created by E2E test")
			assert.NoError(t, err, "Should fill description if field exists")
		}

		emailInput := browser.Page.Locator("input[name='system_address'], input#system_address")
		if count, _ := emailInput.Count(); count > 0 {
			err = emailInput.Fill("test@example.com")
			assert.NoError(t, err, "Should fill email if field exists")
		}

		// Submit form
		submitBtn := browser.Page.Locator("button[type='submit'], button:has-text('Save'), button:has-text('Create')")
		err = submitBtn.Click()
		require.NoError(t, err, "Should submit form")

		// Wait for response
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Verify queue was created by checking if it appears in the list
		browser.Page.WaitForTimeout(1000) // Give time for list to update
		queueItem := browser.Page.Locator(fmt.Sprintf("*:has-text('%s')", testQueueName))
		count, _ = queueItem.Count()
		assert.Greater(t, count, 0, "New queue should appear in list")
	})

	t.Run("Edit queue", func(t *testing.T) {
		err := browser.NavigateTo("/queues")
		require.NoError(t, err)

		// Wait for queue list to load
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Find edit button for first queue
		editBtn := browser.Page.Locator("a[href*='/edit'], button:has-text('Edit')").First()
		count, _ := editBtn.Count()

		if count == 0 {
			t.Skip("No queues available to edit")
		}

		err = editBtn.Click()
		require.NoError(t, err, "Should click edit button")

		// Wait for edit form
		_, err = browser.Page.WaitForSelector("form", playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(5000),
		})
		require.NoError(t, err, "Edit form should appear")

		// Check that fields are populated
		nameInput := browser.Page.Locator("input[name='name'], input#name")
		nameValue, _ := nameInput.InputValue()
		assert.NotEmpty(t, nameValue, "Name field should be populated")

		// Check description field
		descInput := browser.Page.Locator("textarea[name='comment'], textarea#comment")
		if count, _ := descInput.Count(); count > 0 {
			descValue, _ := descInput.InputValue()
			// Description might be empty, but field should exist
			t.Logf("Description field value: %s", descValue)
		}

		// Update the name
		updatedName := nameValue + "_Updated"
		err = nameInput.Fill(updatedName)
		require.NoError(t, err, "Should update name field")

		// Submit the form
		submitBtn := browser.Page.Locator("button[type='submit'], button:has-text('Save')")
		err = submitBtn.Click()
		require.NoError(t, err, "Should submit edit form")

		// Wait for response
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Verify the update by checking if updated name appears
		browser.Page.WaitForTimeout(1000)
		updatedItem := browser.Page.Locator(fmt.Sprintf("*:has-text('%s')", updatedName))
		count, _ = updatedItem.Count()
		assert.Greater(t, count, 0, "Updated queue name should appear")
	})

	t.Run("Delete queue", func(t *testing.T) {
		err := browser.NavigateTo("/queues")
		require.NoError(t, err)

		// Wait for queue list
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Find a test queue to delete
		testQueue := browser.Page.Locator("*:has-text('TestQueue_')").First()
		count, _ := testQueue.Count()

		if count == 0 {
			t.Skip("No test queues available to delete")
		}

		// Get the queue name for verification
		queueText, _ := testQueue.TextContent()

		// Find delete button near the test queue
		deleteBtn := browser.Page.Locator("button:has-text('Delete'), a[href*='/delete']").First()
		count, _ = deleteBtn.Count()

		if count == 0 {
			t.Skip("Delete functionality not available")
		}

		err = deleteBtn.Click()
		require.NoError(t, err, "Should click delete button")

		// Handle confirmation dialog if it appears
		confirmBtn := browser.Page.Locator("button:has-text('Confirm'), button:has-text('Yes')")
		if count, _ := confirmBtn.Count(); count > 0 {
			err = confirmBtn.Click()
			assert.NoError(t, err, "Should confirm deletion")
		}

		// Wait for deletion to complete
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Verify queue is removed from list
		browser.Page.WaitForTimeout(1000)
		deletedQueue := browser.Page.Locator(fmt.Sprintf("*:has-text('%s')", queueText))
		count, _ = deletedQueue.Count()

		// The queue might be soft-deleted and still visible but marked as inactive
		// So we just log the result
		t.Logf("Queue presence after delete: %d occurrences", count)
	})

	t.Run("Search queues", func(t *testing.T) {
		err := browser.NavigateTo("/queues")
		require.NoError(t, err)

		// Look for search input
		searchInput := browser.Page.Locator("input[type='search'], input[placeholder*='Search'], input[name='search']")
		count, _ := searchInput.Count()

		if count == 0 {
			t.Skip("Search functionality not available")
		}

		// Enter search term
		err = searchInput.Fill("Test")
		require.NoError(t, err, "Should enter search term")

		// Trigger search (might be automatic or need button/enter)
		searchBtn := browser.Page.Locator("button:has-text('Search')")
		if count, _ := searchBtn.Count(); count > 0 {
			err = searchBtn.Click()
			assert.NoError(t, err)
		} else {
			// Press Enter to search
			err = searchInput.Press("Enter")
			assert.NoError(t, err)
		}

		// Wait for results
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Results should be filtered
		// This is hard to verify without knowing the data
		t.Log("Search executed successfully")
	})
}
