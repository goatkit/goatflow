//go:build e2e

package e2e

import (
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationFlow(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Login page loads correctly", func(t *testing.T) {
		err := browser.NavigateTo("/login")
		require.NoError(t, err)

		// Check for logo
		logo := browser.Page.Locator("img[alt='GoatFlow Logo']")
		count, _ := logo.Count()
		assert.Greater(t, count, 0, "Logo should be visible")

		// Check for form fields
		emailInput := browser.Page.Locator("input#email")
		count, _ = emailInput.Count()
		assert.Greater(t, count, 0, "Email input should be present")

		passwordInput := browser.Page.Locator("input#password")
		count, _ = passwordInput.Count()
		assert.Greater(t, count, 0, "Password input should be present")

		// Theme/appearance selector (replaced the old toggleDarkMode() button)
		themeButton := browser.Page.Locator("button#login-theme-button")
		count, _ = themeButton.Count()
		assert.Equal(t, 1, count, "Theme selector should be present")
	})

	t.Run("Login with valid credentials", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err, "Login should succeed with valid credentials")

		// Verify we're on dashboard
		url := browser.Page.URL()
		assert.Contains(t, url, "/dashboard", "Should redirect to dashboard after login")

		// Verify user is logged in
		assert.True(t, auth.IsLoggedIn(), "User should be logged in")
	})

	t.Run("Logout functionality", func(t *testing.T) {
		// Ensure we're logged in first
		if !auth.IsLoggedIn() {
			err := auth.LoginAsAdmin()
			require.NoError(t, err)
		}

		// Perform logout
		err := auth.Logout()
		require.NoError(t, err, "Logout should succeed")

		// Verify we're back at login
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should redirect to login after logout")

		// Verify user is logged out
		assert.False(t, auth.IsLoggedIn(), "User should be logged out")
	})

	t.Run("Login with invalid credentials", func(t *testing.T) {
		err := auth.Login("invalid@example.com", "wrongpassword")
		assert.Error(t, err, "Login should fail with invalid credentials")

		// Should still be on login page
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should remain on login page after failed login")
	})

	t.Run("Appearance switch works", func(t *testing.T) {
		err := browser.NavigateTo("/login")
		require.NoError(t, err)

		html := browser.Page.Locator("html")
		initialDark, err := htmlHasClass(html, "dark")
		require.NoError(t, err)
		target, restore := "dark", "light"
		if initialDark {
			target, restore = "light", "dark"
		}

		selectMode := func(mode string) {
			t.Helper()
			require.NoError(t, browser.Page.Locator("button#login-theme-button").Click())
			modeButton := browser.Page.Locator("button#login-mode-btn-" + mode)
			require.NoError(t, modeButton.WaitFor())
			// selectLoginMode() persists the choice via POST /api/themes, then applies it.
			resp, err := browser.Page.ExpectResponse("**/api/themes", func() error { return modeButton.Click() })
			require.NoError(t, err)
			assert.Equal(t, 200, resp.Status())
			_, err = browser.Page.WaitForFunction(
				`(mode) => document.documentElement.classList.contains(mode)`, mode)
			require.NoError(t, err, "html should switch to %s mode", mode)
		}

		selectMode(target)
		// The choice survives a reload.
		_, err = browser.Page.Reload()
		require.NoError(t, err)
		has, err := htmlHasClass(html, target)
		require.NoError(t, err)
		assert.True(t, has, "%s mode should persist across reload", target)

		selectMode(restore)
	})

	t.Run("Form field padding is correct", func(t *testing.T) {
		err := browser.NavigateTo("/login")
		require.NoError(t, err)

		// Text must not sit against the input border: both login inputs get
		// the theme input padding (gk-input-neon), with matching left inset.
		padding := func(sel string) (left, top float64) {
			t.Helper()
			v, err := browser.Page.Locator(sel).Evaluate(
				`el => { const s = getComputedStyle(el); return [parseFloat(s.paddingLeft), parseFloat(s.paddingTop)]; }`, nil)
			require.NoError(t, err)
			vals := v.([]interface{})
			return toFloat(vals[0]), toFloat(vals[1])
		}
		emailLeft, emailTop := padding("input#email")
		passwordLeft, passwordTop := padding("input#password")
		assert.GreaterOrEqual(t, emailLeft, 12.0, "email input left padding")
		assert.GreaterOrEqual(t, emailTop, 8.0, "email input top padding")
		assert.Equal(t, emailLeft, passwordLeft, "email and password inputs share the left inset")
		assert.Equal(t, emailTop, passwordTop, "email and password inputs share the top inset")
	})
}

func htmlHasClass(html playwright.Locator, class string) (bool, error) {
	v, err := html.Evaluate(`(el, c) => el.classList.contains(c)`, class)
	if err != nil {
		return false, err
	}
	b, _ := v.(bool)
	return b, nil
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return -1
}
