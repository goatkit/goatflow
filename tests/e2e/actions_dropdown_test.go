//go:build playwright

package e2e

import (
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActionsDropdownVisibility tests that the Actions dropdown is visible on ticket detail pages
func TestActionsDropdownVisibility(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	// Setup auth helper
	auth := helpers.NewAuthHelper(browser)

	t.Run("Actions dropdown is visible on ticket detail page", func(t *testing.T) {
		// Login as admin (assuming demo credentials are set)
		if browser.Config.AdminEmail != "" && browser.Config.AdminPassword != "" {
			err := auth.Login(browser.Config.AdminEmail, browser.Config.AdminPassword)
			require.NoError(t, err, "Failed to login")
		} else {
			t.Skip("Admin credentials not configured, skipping authenticated test")
		}

		// Navigate to a ticket detail page (assuming ticket ID 1 exists)
		err := browser.NavigateTo("/tickets/1")
		require.NoError(t, err, "Failed to navigate to ticket detail")

		// Check if we're on the ticket detail page
		currentURL := browser.Page.URL()
		assert.Contains(t, currentURL, "/tickets/1", "Should be on ticket detail page")

		// Wait for page to reach network idle (typed API)
		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateNetworkidle})

		// Check for Actions dropdown button
		actionsButton := browser.Page.Locator("button:has-text('Actions')")
		count, err := actionsButton.Count()
		require.NoError(t, err, "Error checking for Actions button")

		if count == 0 {
			t.Log("Actions button not found. Checking page content...")

			// Take a screenshot for debugging
			if browser.Config.Screenshots {
				screenshot, _ := browser.Page.Screenshot()
				t.Logf("Screenshot taken, size: %d bytes", len(screenshot))
			}

			// Check what buttons are actually present
			allButtons := browser.Page.Locator("button")
			buttonCount, _ := allButtons.Count()
			t.Logf("Found %d buttons on page", buttonCount)

			for i := 0; i < buttonCount; i++ {
				buttonText, _ := allButtons.Nth(i).TextContent()
				t.Logf("Button %d: %s", i, buttonText)
			}

			t.Error("Actions dropdown button not found on ticket detail page")
			return
		}

		t.Logf("Found %d Actions button(s)", count)
		assert.Greater(t, count, 0, "Actions button should be present")

		// Check if the dropdown is initially hidden
		actionsDropdown := browser.Page.Locator("#actionsDropdown")
		isVisible, err := actionsDropdown.IsVisible()
		require.NoError(t, err, "Error checking dropdown visibility")
		assert.False(t, isVisible, "Actions dropdown should be initially hidden")

		// Click the Actions button to show dropdown
		err = actionsButton.First().Click()
		require.NoError(t, err, "Failed to click Actions button")

		// Wait a bit for the dropdown to appear
		browser.Page.WaitForTimeout(500)

		// Check if dropdown is now visible
		isVisible, err = actionsDropdown.IsVisible()
		require.NoError(t, err, "Error checking dropdown visibility after click")
		assert.True(t, isVisible, "Actions dropdown should be visible after clicking")

		// Check for Move to Queue option in the dropdown
		moveToQueueLink := browser.Page.Locator("#actionsDropdown a:has-text('Move to Queue')")
		count, err = moveToQueueLink.Count()
		require.NoError(t, err, "Error checking for Move to Queue link")
		assert.Greater(t, count, 0, "Move to Queue option should be present in dropdown")

		t.Log("✅ Actions dropdown is working correctly with Move to Queue option")
	})
}
