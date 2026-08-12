//go:build e2e

package playwright

// Temporary screenshot capture for the v0.9.0 release blog. Remove after use.

import (
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

func sptr(s string) *string { return &s }
func bptr(b bool) *bool      { return &b }

func capture(t *testing.T, browser *helpers.BrowserHelper, path string, full bool) {
	t.Helper()
	_ = browser.WaitForLoad()
	time.Sleep(700 * time.Millisecond)
	_, err := browser.Page.Screenshot(playwright.PageScreenshotOptions{
		Path:     sptr(path),
		FullPage: bptr(full),
	})
	require.NoError(t, err, "screenshot %s", path)
}

func TestCaptureV09Screens(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NotEmpty(t, browser.Config.AdminEmail, "admin email not configured")
	require.NotEmpty(t, browser.Config.AdminPassword, "admin password not configured")
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	// Setup Assistant wizard
	err = auth.LoginAsAdmin()
	require.NoError(t, err)
	require.NoError(t, browser.NavigateTo("/admin/setup"))
	capture(t, browser, "/workspace/tmp/setup_assistant_wizard.png", true)

	// Setup Assistant task catalog
	require.NoError(t, browser.NavigateTo("/admin/setup/assistant"))
	capture(t, browser, "/workspace/tmp/setup_assistant_catalog.png", true)

	// Customer onboarding wizard (step 1: company)
	require.NoError(t, browser.NavigateTo("/admin/setup/task/core/create_customer"))
	capture(t, browser, "/workspace/tmp/onboard_customer.png", true)

	// Identity providers (SAML) list + new form
	require.NoError(t, browser.NavigateTo("/admin/identity-providers"))
	capture(t, browser, "/workspace/tmp/identity_providers.png", true)
	require.NoError(t, browser.NavigateTo("/admin/identity-providers/new"))
	capture(t, browser, "/workspace/tmp/identity_provider_form.png", true)

	// Admin Users group multi-select
	require.NoError(t, browser.NavigateTo("/admin/users"))
	_ = browser.WaitForLoad()
	time.Sleep(500 * time.Millisecond)
	_, err = browser.Page.Click("button[onclick*='showAddUserModal']")
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	_, err = browser.Page.Fill("#groupSearch", "admin")
	require.NoError(t, err)
	_, err = browser.Page.Press("#groupSearch", "Enter")
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	_, err = browser.Page.Screenshot(playwright.PageScreenshotOptions{
		Path:     sptr("/workspace/tmp/users_group_multiselect.png"),
		FullPage: bptr(false),
	})
	require.NoError(t, err)
}
