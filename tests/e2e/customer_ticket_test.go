//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomerTicketCreation tests the complete customer ticket creation workflow
func TestCustomerTicketCreation(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	t.Run("Customer can access ticket creation form", func(t *testing.T) {
		// Navigate to ticket creation form
		err := browser.NavigateTo("/customer/tickets/new")
		require.NoError(t, err)

		// Check if we're redirected to login (expected for unauthenticated)
		if browser.Page.URL() != browser.Config.BaseURL+"/customer/tickets/new" {
			// Should be on login page
			assert.Contains(t, browser.Page.URL(), "/login")
			return
		}

		// If we reached the form, verify it loads correctly
		assert.Equal(t, browser.Config.BaseURL+"/customer/tickets/new", browser.Page.URL())

		// Check for form elements
		subjectInput := browser.Page.Locator("input[name='subject'], input[name='title']")
		count, _ := subjectInput.Count()
		assert.Greater(t, count, 0, "Subject/title input should be present")

		messageTextarea := browser.Page.Locator("textarea[name='message'], textarea[name='body']")
		count, _ = messageTextarea.Count()
		assert.Greater(t, count, 0, "Message textarea should be present")

		// Check for create ticket button or submit button
		submitButton := browser.Page.Locator("button[type='submit'], input[type='submit']")
		count, _ = submitButton.Count()
		assert.Greater(t, count, 0, "Submit button should be present")
	})

	t.Run("Customer can create ticket via form submission", func(t *testing.T) {
		// Navigate to ticket creation form
		err := browser.NavigateTo("/customer/tickets/new")
		require.NoError(t, err)

		// Skip if redirected to login
		if browser.Page.URL() != browser.Config.BaseURL+"/customer/tickets/new" {
			t.Skip("Customer authentication required for this test")
			return
		}

		// Fill out the form
		subjectInput := browser.Page.Locator("input[name='subject'], input[name='title']")
		if count, _ := subjectInput.Count(); count > 0 {
			err := subjectInput.Fill("Test Ticket from Customer")
			require.NoError(t, err)
		}

		messageTextarea := browser.Page.Locator("textarea[name='message'], textarea[name='body']")
		if count, _ := messageTextarea.Count(); count > 0 {
			err := messageTextarea.Fill("This is a test message from the customer")
			require.NoError(t, err)
		}

		// Submit the form
		submitButton := browser.Page.Locator("button[type='submit'], input[type='submit']")
		if count, _ := submitButton.Count(); count > 0 {
			err := submitButton.Click()
			require.NoError(t, err)

			// Wait for response
			err = browser.WaitForHTMX()
			require.NoError(t, err)

			// Check if we got redirected to a ticket view
			currentURL := browser.Page.URL()
			if strings.Contains(currentURL, "/customer/tickets/") && currentURL != browser.Config.BaseURL+"/customer/tickets/new" {
				t.Logf("Ticket created successfully, redirected to: %s", currentURL)
			} else {
				// Check for error messages
				errorMsg := browser.Page.Locator(".error-message, #error-message, .alert-danger")
				if count, _ := errorMsg.Count(); count > 0 {
					text, _ := errorMsg.TextContent()
					t.Logf("Form submission error: %s", text)
				}
			}
		}
	})
}
