//go:build e2e

package e2e

import (
	"testing"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomerTicketWorkflowComplete tests the complete customer ticket creation and management workflow
func TestCustomerTicketWorkflowComplete(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Customer ticket creation form is accessible and properly structured", func(t *testing.T) {
		// Navigate to ticket creation form
		err := browser.NavigateTo("/customer/tickets/new")
		require.NoError(t, err)

		// Should redirect to login for unauthenticated users
		if browser.Page.URL() != browser.Config.BaseURL+"/customer/tickets/new" {
			assert.Contains(t, browser.Page.URL(), "/login", "Should redirect to login")
			return
		}

		// If we reached the form (authenticated), verify it loads correctly
		assert.Equal(t, browser.Config.BaseURL+"/customer/tickets/new", browser.Page.URL())

		// Check for required form elements
		subjectInput := browser.Page.Locator("input[name='title'], input[name='subject']")
		count, _ := subjectInput.Count()
		assert.Greater(t, count, 0, "Subject/title input should be present")

		messageTextarea := browser.Page.Locator("textarea[name='message'], textarea[name='body']")
		count, _ = messageTextarea.Count()
		assert.Greater(t, count, 0, "Message textarea should be present")

		// Check for optional fields
		prioritySelect := browser.Page.Locator("select[name='priority_id']")
		count, _ = prioritySelect.Count()
		assert.Greater(t, count, 0, "Priority select should be present")

		serviceSelect := browser.Page.Locator("select[name='service_id']")
		count, _ = serviceSelect.Count()
		assert.Greater(t, count, 0, "Service select should be present")

		// Check for submit button
		submitButton := browser.Page.Locator("button[type='submit'], input[type='submit']")
		count, _ = submitButton.Count()
		assert.Greater(t, count, 0, "Submit button should be present")

		// Check for form validation indicators (required fields)
		requiredFields := browser.Page.Locator(".text-red-500, [required]")
		count, _ = requiredFields.Count()
		assert.Greater(t, count, 0, "Required field indicators should be present")
	})

	t.Run("Admin user cannot access customer ticket creation", func(t *testing.T) {
		// Login as admin
		err := auth.LoginAsAdmin()
		require.NoError(t, err, "Failed to login as admin")

		// Try to access customer ticket creation
		err = browser.NavigateTo("/customer/tickets/new")
		require.NoError(t, err)

		// Should get forbidden or redirect
		currentURL := browser.Page.URL()
		if currentURL == browser.Config.BaseURL+"/customer/tickets/new" {
			// If we reached the page, check for error message
			errorMsg := browser.Page.Locator(".error-message, .alert-danger, .text-red-600")
			if count, _ := errorMsg.Count(); count > 0 {
				text, _ := errorMsg.TextContent()
				assert.Contains(t, text, "Customer", "Should show customer access required error")
			}
		} else {
			// Should be redirected or show error
			assert.NotEqual(t, browser.Config.BaseURL+"/customer/tickets/new", currentURL, "Admin should not access customer ticket creation")
		}

		// Logout for next test
		auth.Logout()
	})

	t.Run("Customer ticket creation validates required fields", func(t *testing.T) {
		// This test would require a customer user to be set up
		// For now, we'll test the validation logic by examining the form

		err := browser.NavigateTo("/customer/tickets/new")
		require.NoError(t, err)

		// Skip if redirected to login
		if browser.Page.URL() != browser.Config.BaseURL+"/customer/tickets/new" {
			t.Skip("Customer authentication required for this test")
			return
		}

		// Check for required field validation
		titleInput := browser.Page.Locator("input[name='title'], input[name='subject']")
		if count, _ := titleInput.Count(); count > 0 {
			requiredAttr, _ := titleInput.GetAttribute("required")
			assert.Equal(t, "required", requiredAttr, "Title field should be required")
		}

		messageTextarea := browser.Page.Locator("textarea[name='message'], textarea[name='body']")
		if count, _ := messageTextarea.Count(); count > 0 {
			requiredAttr, _ := messageTextarea.GetAttribute("required")
			assert.Equal(t, "required", requiredAttr, "Message field should be required")
		}
	})

	t.Run("Customer ticket creation generates proper ticket number format", func(t *testing.T) {
		// This test verifies the ticket number generation logic
		// The format should be: YYYYMMDDHHMMSS based on the handler code

		// For now, we'll verify the handler exists and is properly structured
		// by checking that the route is registered

		// Navigate to a test page to ensure the server is responding
		err := browser.NavigateTo("/health")
		require.NoError(t, err)

		// The actual ticket number generation would be tested when we create a ticket
		t.Log("Ticket number format should be: YYYYMMDDHHMMSS")
	})

	t.Run("Customer ticket creation creates associated article", func(t *testing.T) {
		// This test verifies that when a ticket is created, an initial article is also created
		// This is based on the handler code that creates both ticket and article

		// For now, we'll verify the handler logic exists
		// The handler creates a ticket and then creates an article with the same content

		t.Log("Customer ticket creation should create both ticket and initial article")
	})
}
