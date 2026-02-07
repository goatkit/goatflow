//go:build e2e

package e2e

import (
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
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

		// Check for dark mode toggle
		darkModeToggle := browser.Page.Locator("button[onclick='toggleDarkMode()']")
		count, _ = darkModeToggle.Count()
		assert.Greater(t, count, 0, "Dark mode toggle should be present")
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

	t.Run("Dark mode toggle works", func(t *testing.T) {
		err := browser.NavigateTo("/login")
		require.NoError(t, err)

		// Get initial dark mode state
		html := browser.Page.Locator("html")
		initialClass, _ := html.GetAttribute("class")

		// Click dark mode toggle
		darkModeToggle := browser.Page.Locator("button[onclick='toggleDarkMode()']")
		err = darkModeToggle.Click()
		require.NoError(t, err)

		// Check class changed
		newClass, _ := html.GetAttribute("class")
		assert.NotEqual(t, initialClass, newClass, "HTML class should change after toggle")

		// Toggle back
		err = darkModeToggle.Click()
		require.NoError(t, err)

		// Check class reverted
		finalClass, _ := html.GetAttribute("class")
		assert.Equal(t, initialClass, finalClass, "HTML class should revert after second toggle")
	})

	t.Run("Form field padding is correct", func(t *testing.T) {
		err := browser.NavigateTo("/login")
		require.NoError(t, err)

		// Check email input has proper padding classes
		emailInput := browser.Page.Locator("input#email")
		classes, _ := emailInput.GetAttribute("class")
		assert.Contains(t, classes, "px-3", "Email input should have px-3 padding")
		assert.Contains(t, classes, "py-2.5", "Email input should have py-2.5 padding")

		// Check password input has proper padding classes
		passwordInput := browser.Page.Locator("input#password")
		classes, _ = passwordInput.GetAttribute("class")
		assert.Contains(t, classes, "px-3", "Password input should have px-3 padding")
		assert.Contains(t, classes, "py-2.5", "Password input should have py-2.5 padding")
	})
}
