//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTOTP2FAFlow enables an authenticator app on the profile page, signs in
// with it (wrong code refused, right code accepted) and turns it off again.
func TestTOTP2FAFlow(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	t.Cleanup(browser.TearDown)

	// Dedicated seeded agent, so enabling 2FA never locks out the admin. It
	// must start and end with 2FA off: reset it before the flow (a previous
	// aborted run may have left it on) and again after, whatever happened.
	reset2FAAgent(t, browser)
	t.Cleanup(func() { reset2FAAgent(t, browser) })

	auth := helpers.NewAuthHelper(browser)
	page := browser.Page
	login := helpers.Seed2FAAgentLogin
	password := helpers.Seed2FAAgentPassword
	byID := func(id string) playwright.Locator { return page.Locator(fmt.Sprintf("[id='%s']", id)) }
	var secret string

	t.Run("Login without 2FA lands on the dashboard", func(t *testing.T) {
		require.NoError(t, auth.Login(login, password))
		require.NoError(t, page.WaitForURL("**/dashboard"))
	})

	t.Run("Enable 2FA from profile page", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/profile"))
		require.NoError(t, byID("2fa-status-disabled").WaitFor(), "seeded agent %q must start with 2FA off", login)
		require.NoError(t, byID("2fa-setup-btn").Click())

		require.NoError(t, byID("2fa-setup-password").Fill(password))
		require.NoError(t, byID("2fa-setup-continue-btn").Click())

		secretEl := byID("2fa-secret")
		require.NoError(t, secretEl.WaitFor(), "QR step should show the manual-entry secret")
		var err error
		secret, err = secretEl.TextContent()
		require.NoError(t, err)
		require.Regexp(t, `^[A-Z2-7]{16,}$`, secret, "secret should be base32")
		recoveryCodes, err := byID("2fa-recovery-codes").Locator("div").Count()
		require.NoError(t, err)
		assert.Positive(t, recoveryCodes, "setup should show recovery codes")

		code, err := totp.GenerateCode(secret, time.Now())
		require.NoError(t, err)
		require.NoError(t, byID("2fa-confirm-code").Fill(code))
		require.NoError(t, byID("2fa-confirm-password").Fill(password))
		require.NoError(t, byID("2fa-confirm-btn").Click())

		// Confirming reloads the page; the status badge then reads enabled.
		require.NoError(t, byID("2fa-status-enabled").WaitFor(), "2FA should now be enabled")
		require.NoError(t, byID("2fa-disable-btn").WaitFor())
		assert.NoError(t, byID("2fa-setup-btn").WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateHidden}),
			"enable button should disappear once an authenticator app is set up")
	})

	t.Run("Password login now asks for the code", func(t *testing.T) {
		require.NotEmpty(t, secret, "2FA was not enabled by the previous subtest; this flow is sequential")
		require.NoError(t, auth.Logout())
		submitPasswordLogin(t, browser, login, password)
		require.NoError(t, page.WaitForURL("**/login/2fa"), "password login should continue to the 2FA step")
		// The login form is hx-boosted: the 2FA page is swapped in and its
		// script (auto-submit on six digits) runs after the URL changes.
		_, err := page.WaitForFunction(`() => typeof verifyCode === 'function'`, nil)
		require.NoError(t, err, "2FA page script should load")
	})

	t.Run("Invalid code is refused", func(t *testing.T) {
		require.NotEmpty(t, secret, "2FA was not enabled by the previous subtest; this flow is sequential")
		// The form auto-submits six digits; use a code outside the accepted
		// window around now.
		resp, err := page.ExpectResponse("**/api/auth/2fa/verify", func() error {
			return page.Locator("input#code").Fill(wrongTOTPCode(t, secret))
		})
		require.NoError(t, err)
		body, _ := resp.Text()
		require.Equal(t, 401, resp.Status(), "invalid code must be refused: %s", body)

		errorBox := page.Locator("#error-message")
		require.NoError(t, errorBox.WaitFor(), "an invalid code should show an error")
		text, err := page.Locator("#error-text").TextContent()
		require.NoError(t, err)
		assert.NotEmpty(t, strings.TrimSpace(text))
		assert.Contains(t, page.URL(), "/login/2fa", "should stay on the 2FA step")
	})

	t.Run("Valid code completes login", func(t *testing.T) {
		require.NotEmpty(t, secret, "2FA was not enabled by the previous subtest; this flow is sequential")
		code, err := totp.GenerateCode(secret, time.Now())
		require.NoError(t, err)
		require.NoError(t, page.Locator("input#code").Fill(code))
		require.NoError(t, page.WaitForURL("**/dashboard"), "a valid code should finish signing in")
	})

	t.Run("Disable 2FA from profile page", func(t *testing.T) {
		require.NotEmpty(t, secret, "2FA was not enabled by the previous subtest; this flow is sequential")
		require.NoError(t, browser.NavigateTo("/profile"))
		require.NoError(t, byID("2fa-disable-btn").Click())

		code, err := totp.GenerateCode(secret, time.Now())
		require.NoError(t, err)
		require.NoError(t, byID("2fa-disable-code").Fill(code))
		require.NoError(t, byID("2fa-disable-password").Fill(password))
		resp, err := page.ExpectResponse("**/api/preferences/2fa/disable", func() error {
			return byID("2fa-disable-confirm-btn").Click()
		})
		require.NoError(t, err)
		body, _ := resp.Text()
		require.Equal(t, 200, resp.Status(), "disable 2FA: %s", body)

		require.NoError(t, byID("2fa-status-disabled").WaitFor(), "2FA should now be disabled")
		require.NoError(t, byID("2fa-setup-btn").WaitFor())

		// Without 2FA the password alone signs in again.
		require.NoError(t, auth.Logout())
		submitPasswordLogin(t, browser, login, password)
		require.NoError(t, page.WaitForURL("**/dashboard"), "password login should no longer ask for a code")
	})
}

// TestTOTP2FASecurityConstraints checks the 2FA step cannot be bypassed.
func TestTOTP2FASecurityConstraints(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup())
	defer browser.TearDown()

	t.Run("Cannot access dashboard with only 2fa_pending cookie", func(t *testing.T) {
		require.NoError(t, browser.Context.AddCookies([]playwright.OptionalCookie{{
			Name:  "2fa_pending",
			Value: "fake_token_12345",
			URL:   playwright.String(browser.Config.BaseURL + "/"),
		}}))

		require.NoError(t, browser.NavigateTo("/dashboard"))
		assert.Contains(t, browser.Page.URL(), "/login", "Should redirect to login, not dashboard")
		assert.NotContains(t, browser.Page.URL(), "/dashboard")
	})

	t.Run("2FA page requires valid pending session", func(t *testing.T) {
		require.NoError(t, browser.Context.ClearCookies())

		require.NoError(t, browser.NavigateTo("/login/2fa"))
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should redirect to login without pending session")
		assert.NotContains(t, url, "/2fa", "Should not stay on 2FA page")
	})
}

// submitPasswordLogin fills and submits the agent login form.
func submitPasswordLogin(t *testing.T, browser *helpers.BrowserHelper, login, password string) {
	t.Helper()
	require.NoError(t, browser.NavigateTo("/login"))
	require.NoError(t, browser.Page.Locator("input#email").Fill(login))
	require.NoError(t, browser.Page.Locator("input#password").Fill(password))
	require.NoError(t, browser.Page.Locator("button[type='submit']").Click())
}

// wrongTOTPCode returns a six-digit code that is not valid for secret in the
// time steps around now.
func wrongTOTPCode(t *testing.T, secret string) string {
	t.Helper()
	valid := map[string]bool{}
	for _, offset := range []time.Duration{-60 * time.Second, -30 * time.Second, 0, 30 * time.Second, 60 * time.Second} {
		code, err := totp.GenerateCode(secret, time.Now().Add(offset))
		require.NoError(t, err)
		valid[code] = true
	}
	for n := 0; ; n++ {
		code := fmt.Sprintf("%06d", n)
		if !valid[code] {
			return code
		}
	}
}

// reset2FAAgent turns 2FA off for the seeded 2FA agent through the admin
// override (POST /admin/api/users/:id/2fa/disable) in a separate API session.
// "not enabled" counts as success.
func reset2FAAgent(t *testing.T, browser *helpers.BrowserHelper) {
	t.Helper()
	cfg := browser.Config
	api, err := browser.Playwright.Request.NewContext(playwright.APIRequestNewContextOptions{
		BaseURL: playwright.String(cfg.BaseURL),
	})
	require.NoError(t, err)
	defer api.Dispose()

	resp, err := api.Post("/api/auth/login", playwright.APIRequestContextPostOptions{
		Form: map[string]interface{}{"username": cfg.AdminEmail, "password": cfg.AdminPassword},
	})
	require.NoError(t, err)
	require.True(t, resp.Ok(), "admin API login failed: %d", resp.Status())

	resp, err = api.Get("/admin/users/list")
	require.NoError(t, err)
	var list struct {
		Users []struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
		} `json:"users"`
	}
	require.NoError(t, resp.JSON(&list), "admin users list (status %d)", resp.Status())
	userID := 0
	for _, u := range list.Users {
		if u.Login == helpers.Seed2FAAgentLogin {
			userID = u.ID
		}
	}
	require.NotZero(t, userID, "seeded agent %q missing from /admin/users/list", helpers.Seed2FAAgentLogin)

	resp, err = api.Post(fmt.Sprintf("/admin/api/users/%d/2fa/disable", userID), playwright.APIRequestContextPostOptions{
		Data: map[string]string{"reason": "e2e TestTOTP2FAFlow reset"},
	})
	require.NoError(t, err)
	body, err := resp.Text()
	require.NoError(t, err)
	if resp.Status() == 400 && strings.Contains(body, "2FA is not enabled") {
		return
	}
	require.Equal(t, 200, resp.Status(), "admin 2FA override failed: %s", body)
}
