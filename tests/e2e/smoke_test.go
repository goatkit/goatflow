//go:build e2e

package e2e

import (
	"testing"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
)

// TestSmokeTest is a basic test to verify E2E setup works
func TestSmokeTest(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	// Navigate to login page
	err = browser.NavigateTo("/login")
	require.NoError(t, err, "Failed to navigate to login page")

	// Check page loaded
	title, err := browser.Page.Title()
	require.NoError(t, err, "Failed to get page title")
	t.Logf("Page title: %s", title)

	// Take a screenshot
	if browser.Config.Screenshots {
		browser.Page.Screenshot()
		t.Log("Screenshot captured")
	}
}
