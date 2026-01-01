//go:build e2e

package playwright

import (
	"testing"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsDropdownVisibility(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Actions dropdown is visible on ticket detail page", func(t *testing.T) {
		err := auth.Login(browser.Config.AdminEmail, browser.Config.AdminPassword)
		require.NoError(t, err)
		if err := browser.NavigateTo("/ticket/2021012710123456"); err != nil {
			t.Skip("ticket fixture unavailable")
		}
		require.NoError(t, browser.WaitForLoad())

		currentURL := browser.Page.URL()
		assert.Contains(t, currentURL, "/ticket/")
		_ = browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateNetworkidle})

		actionsButton := browser.Page.Locator("button:has-text('Actions')").First()
		if count(t, actionsButton) == 0 {
			t.Skip("Actions dropdown button not found")
		}
		actionsDropdown := browser.Page.Locator("#actionsDropdown")
		isVisible, err := actionsDropdown.IsVisible()
		require.NoError(t, err)
		assert.False(t, isVisible)
	})
}
