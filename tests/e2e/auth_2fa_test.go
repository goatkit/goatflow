//go:build e2e

package e2e

import (
	"net/url"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTOTP2FAFlow tests the complete 2FA setup, login, and disable flow.
func TestTOTP2FAFlow(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Use a test user (not admin to avoid locking admin out)
	testEmail := browser.Config.AdminEmail // In real tests, use a dedicated test user
	testPassword := browser.Config.AdminPassword
	var totpSecret string

	t.Run("Setup: Login without 2FA", func(t *testing.T) {
		err := auth.Login(testEmail, testPassword)
		require.NoError(t, err, "Initial login should succeed without 2FA")
		assert.True(t, auth.IsLoggedIn(), "User should be logged in")
	})

	t.Run("Enable 2FA from profile page", func(t *testing.T) {
		// Navigate to profile
		err := browser.NavigateTo("/profile")
		require.NoError(t, err, "Should navigate to profile")

		// Wait for page load
		time.Sleep(500 * time.Millisecond)

		// Click Enable 2FA button
		enable2FABtn := browser.Page.Locator("#2fa-setup-btn")
		count, _ := enable2FABtn.Count()
		if count == 0 {
			t.Skip("2FA setup button not visible - 2FA may already be enabled")
		}

		err = enable2FABtn.Click()
		require.NoError(t, err, "Should click Enable 2FA button")

		// Wait for modal to appear
		time.Sleep(500 * time.Millisecond)

		// Extract the secret from the modal (for test purposes)
		secretEl := browser.Page.Locator("#2fa-secret")
		err = secretEl.WaitFor()
		require.NoError(t, err, "Secret should be displayed")

		totpSecret, err = secretEl.TextContent()
		require.NoError(t, err, "Should get secret text")
		require.NotEmpty(t, totpSecret, "Secret should not be empty")

		t.Logf("TOTP Secret: %s", totpSecret)

		// Generate a valid TOTP code
		code, err := totp.GenerateCode(totpSecret, time.Now())
		require.NoError(t, err, "Should generate TOTP code")

		// Enter the code
		codeInput := browser.Page.Locator("#2fa-confirm-code")
		err = codeInput.Fill(code)
		require.NoError(t, err, "Should fill verification code")

		// Click verify button
		verifyBtn := browser.Page.Locator("#2fa-confirm-btn")
		err = verifyBtn.Click()
		require.NoError(t, err, "Should click verify button")

		// Wait for response
		time.Sleep(1 * time.Second)

		// Check for success - modal should close and status should show "Enabled"
		enabledBadge := browser.Page.Locator("#2fa-status-enabled")
		err = enabledBadge.WaitFor()
		assert.NoError(t, err, "2FA should now be enabled")
	})

	t.Run("Logout and verify 2FA required", func(t *testing.T) {
		if totpSecret == "" {
			t.Skip("No TOTP secret - 2FA setup may have been skipped")
		}

		// Logout
		err := auth.Logout()
		require.NoError(t, err, "Logout should succeed")

		// Login again - should redirect to 2FA page
		err = browser.NavigateTo("/login")
		require.NoError(t, err)

		// Fill login credentials
		emailInput := browser.Page.Locator("input#email")
		err = emailInput.Fill(testEmail)
		require.NoError(t, err)

		passwordInput := browser.Page.Locator("input#password")
		err = passwordInput.Fill(testPassword)
		require.NoError(t, err)

		submitButton := browser.Page.Locator("button[type='submit']")
		err = submitButton.Click()
		require.NoError(t, err)

		// Wait for redirect to 2FA page
		time.Sleep(1 * time.Second)

		url := browser.Page.URL()
		assert.Contains(t, url, "/login/2fa", "Should redirect to 2FA verification page")
	})

	t.Run("Complete 2FA verification", func(t *testing.T) {
		if totpSecret == "" {
			t.Skip("No TOTP secret - 2FA setup may have been skipped")
		}

		// Generate a valid TOTP code
		code, err := totp.GenerateCode(totpSecret, time.Now())
		require.NoError(t, err, "Should generate TOTP code")

		// Enter the code
		codeInput := browser.Page.Locator("input[name='code'], input#code")
		err = codeInput.Fill(code)
		require.NoError(t, err, "Should fill 2FA code")

		// Submit
		verifyBtn := browser.Page.Locator("button[type='submit']")
		err = verifyBtn.Click()
		require.NoError(t, err, "Should click verify button")

		// Wait for navigation
		time.Sleep(1 * time.Second)

		// Should now be on dashboard
		url := browser.Page.URL()
		assert.Contains(t, url, "/dashboard", "Should redirect to dashboard after 2FA")
		assert.True(t, auth.IsLoggedIn(), "User should be logged in after 2FA")
	})

	t.Run("Invalid 2FA code should fail", func(t *testing.T) {
		if totpSecret == "" {
			t.Skip("No TOTP secret - 2FA setup may have been skipped")
		}

		// Logout first
		auth.Logout()

		// Login to get to 2FA page
		browser.NavigateTo("/login")
		emailInput := browser.Page.Locator("input#email")
		emailInput.Fill(testEmail)
		passwordInput := browser.Page.Locator("input#password")
		passwordInput.Fill(testPassword)
		submitButton := browser.Page.Locator("button[type='submit']")
		submitButton.Click()
		time.Sleep(1 * time.Second)

		// Enter wrong code
		codeInput := browser.Page.Locator("input[name='code'], input#code")
		err := codeInput.Fill("000000")
		require.NoError(t, err)

		verifyBtn := browser.Page.Locator("button[type='submit']")
		err = verifyBtn.Click()
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// Should still be on 2FA page with error
		url := browser.Page.URL()
		assert.Contains(t, url, "/login/2fa", "Should stay on 2FA page after invalid code")

		// Check for error message
		errorEl := browser.Page.Locator(".text-red-500, .error, [class*='error']")
		count, _ := errorEl.Count()
		assert.Greater(t, count, 0, "Should show error message for invalid code")
	})

	t.Run("Disable 2FA", func(t *testing.T) {
		if totpSecret == "" {
			t.Skip("No TOTP secret - 2FA setup may have been skipped")
		}

		// Login with valid 2FA
		browser.NavigateTo("/login")
		emailInput := browser.Page.Locator("input#email")
		emailInput.Fill(testEmail)
		passwordInput := browser.Page.Locator("input#password")
		passwordInput.Fill(testPassword)
		submitButton := browser.Page.Locator("button[type='submit']")
		submitButton.Click()
		time.Sleep(1 * time.Second)

		code, _ := totp.GenerateCode(totpSecret, time.Now())
		codeInput := browser.Page.Locator("input[name='code'], input#code")
		codeInput.Fill(code)
		verifyBtn := browser.Page.Locator("button[type='submit']")
		verifyBtn.Click()
		time.Sleep(1 * time.Second)

		// Navigate to profile
		err := browser.NavigateTo("/profile")
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)

		// Click disable button
		disableBtn := browser.Page.Locator("#2fa-disable-btn")
		err = disableBtn.Click()
		require.NoError(t, err, "Should click Disable 2FA button")

		time.Sleep(500 * time.Millisecond)

		// Generate new code for disable confirmation
		code, _ = totp.GenerateCode(totpSecret, time.Now())

		// Enter code in disable modal
		disableCodeInput := browser.Page.Locator("#2fa-disable-code")
		err = disableCodeInput.Fill(code)
		require.NoError(t, err)

		// Confirm disable
		confirmDisableBtn := browser.Page.Locator("#2fa-disable-confirm-btn")
		err = confirmDisableBtn.Click()
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		// Verify 2FA is now disabled
		disabledBadge := browser.Page.Locator("#2fa-status-disabled")
		err = disabledBadge.WaitFor()
		assert.NoError(t, err, "2FA should now be disabled")
	})
}

// TestTOTP2FASecurityConstraints tests security-specific behaviours.
func TestTOTP2FASecurityConstraints(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()

	t.Run("Cannot access dashboard with only 2fa_pending cookie", func(t *testing.T) {
		// Try to access dashboard with just a fake pending cookie
		// Extract domain from base URL
		baseURL := browser.Config.BaseURL
		domain := "localhost" // Default
		if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
			domain = u.Hostname()
		}
		path := "/"
		browser.Page.Context().AddCookies([]playwright.OptionalCookie{{
			Name:   "2fa_pending",
			Value:  "fake_token_12345",
			Domain: &domain,
			Path:   &path,
		}})

		err := browser.NavigateTo("/dashboard")
		require.NoError(t, err)

		// Should redirect to login, not show dashboard
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should redirect to login, not dashboard")
	})

	t.Run("2FA page requires valid pending session", func(t *testing.T) {
		// Clear all cookies
		browser.Page.Context().ClearCookies()

		// Try to access 2FA page directly
		err := browser.NavigateTo("/login/2fa")
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// Should redirect to login
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should redirect to login without pending session")
		assert.NotContains(t, url, "/2fa", "Should not stay on 2FA page")
	})
}
