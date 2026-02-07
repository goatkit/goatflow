//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProfilePreferencesPage tests the agent profile/preferences page functionality.
// This is a TDD test - it documents expected behavior and will fail until the page is fixed.
func TestProfilePreferencesPage(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Login as admin first
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Login should succeed")

	t.Run("Profile page loads without errors", func(t *testing.T) {
		err := browser.NavigateTo("/agent/profile")
		require.NoError(t, err, "Should navigate to profile page")

		// Wait for page to load
		err = browser.WaitForLoad()
		require.NoError(t, err, "Page should load")

		// Check page title contains "Profile"
		title, err := browser.Page.Title()
		require.NoError(t, err)
		assert.Contains(t, title, "Profile", "Page title should contain 'Profile'")

		// Check profile form exists
		profileForm := browser.Page.Locator("#profile-form")
		count, _ := profileForm.Count()
		assert.Equal(t, 1, count, "Profile form should exist")
	})

	t.Run("Language dropdown has multiple options", func(t *testing.T) {
		err := browser.NavigateTo("/agent/profile")
		require.NoError(t, err)

		// Wait for page and JS to initialize
		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Give JavaScript time to populate the dropdown
		time.Sleep(2 * time.Second)

		// Check language dropdown has options beyond just "Default"
		languageSelect := browser.Page.Locator("#language-select")
		count, _ := languageSelect.Count()
		require.Equal(t, 1, count, "Language select should exist")

		// Get all options
		options := browser.Page.Locator("#language-select option")
		optionCount, _ := options.Count()

		// Should have at least 7 options: Default + 6 languages (en, de, es, fr, ar, tlh)
		assert.GreaterOrEqual(t, optionCount, 7,
			"Language dropdown should have Default + 6 language options (en, de, es, fr, ar, tlh), but has %d options", optionCount)

		// Check that we have actual language options, not just "Default"
		if optionCount > 1 {
			secondOption := browser.Page.Locator("#language-select option:nth-child(2)")
			value, _ := secondOption.GetAttribute("value")
			text, _ := secondOption.TextContent()
			t.Logf("Second option: value=%q, text=%q", value, text)
			assert.NotEmpty(t, value, "Language options should have values")
		}
	})

	t.Run("No JavaScript console errors on profile page", func(t *testing.T) {
		// Collect console errors
		var consoleErrors []string
		browser.Page.On("console", func(msg playwright.ConsoleMessage) {
			if msg.Type() == "error" {
				consoleErrors = append(consoleErrors, msg.Text())
			}
		})

		err := browser.NavigateTo("/agent/profile")
		require.NoError(t, err)

		// Wait for all JS to execute
		err = browser.WaitForHTMX()
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Filter out known acceptable errors (if any)
		var relevantErrors []string
		for _, errMsg := range consoleErrors {
			// Skip any acceptable errors here
			if strings.Contains(errMsg, "favicon") {
				continue
			}
			relevantErrors = append(relevantErrors, errMsg)
		}

		assert.Empty(t, relevantErrors,
			"Profile page should have no JavaScript console errors, but found: %v", relevantErrors)
	})

	t.Run("Profile form saves successfully", func(t *testing.T) {
		err := browser.NavigateTo("/agent/profile")
		require.NoError(t, err)

		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Fill in first name
		firstName := browser.Page.Locator("#first-name")
		err = firstName.Fill("TestFirst")
		require.NoError(t, err)

		// Fill in last name
		lastName := browser.Page.Locator("#last-name")
		err = lastName.Fill("TestLast")
		require.NoError(t, err)

		// Track if we get an alert (error case)
		var alertMessage string
		browser.Page.On("dialog", func(dialog playwright.Dialog) {
			alertMessage = dialog.Message()
			dialog.Dismiss()
		})

		// Submit the profile form
		submitBtn := browser.Page.Locator("#profile-form button[type='submit']")
		err = submitBtn.Click()
		require.NoError(t, err)

		// Wait for response
		time.Sleep(2 * time.Second)

		// Check for success feedback
		feedback := browser.Page.Locator("#profile-save-feedback")
		isVisible, _ := feedback.IsVisible()

		// Either the success feedback should be visible OR there should be no error alert
		if !isVisible && alertMessage != "" {
			t.Errorf("Profile save failed with alert: %s", alertMessage)
		}

		// The alert message should NOT contain "Unknown error" or authentication errors
		if alertMessage != "" {
			assert.NotContains(t, alertMessage, "Unknown error",
				"Profile save should not show 'Unknown error'")
			assert.NotContains(t, alertMessage, "not authenticated",
				"Profile save should not show authentication error")
			assert.NotContains(t, alertMessage, "Missing authorization",
				"Profile save should not show authorization error")
		}
	})

	t.Run("Language preference saves successfully", func(t *testing.T) {
		err := browser.NavigateTo("/agent/profile")
		require.NoError(t, err)

		err = browser.WaitForHTMX()
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Check if we have language options
		options := browser.Page.Locator("#language-select option")
		optionCount, _ := options.Count()

		if optionCount < 2 {
			t.Skip("Skipping language save test - no language options available")
		}

		// Track alert messages
		var alertMessage string
		browser.Page.On("dialog", func(dialog playwright.Dialog) {
			alertMessage = dialog.Message()
			dialog.Dismiss()
		})

		// Select a language (German)
		languageSelect := browser.Page.Locator("#language-select")
		_, err = languageSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"de"},
		})
		if err != nil {
			t.Logf("Could not select 'de' option: %v", err)
		}

		// Submit the language form
		submitBtn := browser.Page.Locator("#language-preferences-form button[type='submit']")
		err = submitBtn.Click()
		require.NoError(t, err)

		// Wait for response
		time.Sleep(2 * time.Second)

		// Check for success feedback or error
		feedback := browser.Page.Locator("#language-save-feedback")
		isVisible, _ := feedback.IsVisible()

		if !isVisible && alertMessage != "" {
			t.Errorf("Language preference save failed with alert: %s", alertMessage)
		}

		// The alert message should NOT contain errors
		if alertMessage != "" {
			assert.NotContains(t, alertMessage, "Unknown error",
				"Language save should not show 'Unknown error'")
			assert.NotContains(t, alertMessage, "not authenticated",
				"Language save should not show authentication error")
		}
	})

	t.Run("Session timeout preference saves successfully", func(t *testing.T) {
		err := browser.NavigateTo("/agent/profile")
		require.NoError(t, err)

		err = browser.WaitForHTMX()
		require.NoError(t, err)

		// Track alert messages
		var alertMessage string
		browser.Page.On("dialog", func(dialog playwright.Dialog) {
			alertMessage = dialog.Message()
			dialog.Dismiss()
		})

		// Select a timeout value (1 hour)
		timeoutSelect := browser.Page.Locator("#session-timeout")
		_, err = timeoutSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"3600"},
		})
		require.NoError(t, err)

		// Submit the session preferences form
		submitBtn := browser.Page.Locator("#session-preferences-form button[type='submit']")
		err = submitBtn.Click()
		require.NoError(t, err)

		// Wait for response
		time.Sleep(2 * time.Second)

		// Check for success or error
		feedback := browser.Page.Locator("#save-feedback")
		isVisible, _ := feedback.IsVisible()

		if !isVisible && alertMessage != "" {
			t.Errorf("Session timeout save failed with alert: %s", alertMessage)
		}

		// The alert message should NOT contain errors
		if alertMessage != "" {
			assert.NotContains(t, alertMessage, "Unknown error",
				"Session timeout save should not show 'Unknown error'")
			assert.NotContains(t, alertMessage, "not authenticated",
				"Session timeout save should not show authentication error")
		}
	})
}

// TestProfileAPIEndpoints directly tests the API endpoints used by the profile page.
func TestProfileAPIEndpoints(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Login first to get a session
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Login should succeed")

	t.Run("GET /agent/api/preferences/language returns available languages", func(t *testing.T) {
		// Navigate to any page first to ensure we have a session
		err := browser.NavigateTo("/agent/dashboard")
		require.NoError(t, err)

		// Make API call via JavaScript and capture result
		result, err := browser.Page.Evaluate(`async () => {
			try {
				const response = await fetch('/agent/api/preferences/language', {
					credentials: 'include',
					headers: { 'Accept': 'application/json' }
				});
				const data = await response.json();
				return { status: response.status, data: data };
			} catch (e) {
				return { error: e.message };
			}
		}`)
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})

		// Check for error
		if errMsg, hasErr := resultMap["error"]; hasErr {
			t.Fatalf("API call failed: %v", errMsg)
		}

		status := resultMap["status"].(float64)
		data := resultMap["data"].(map[string]interface{})

		// Should return 200 OK
		assert.Equal(t, float64(200), status, "GET /agent/api/preferences/language should return 200")

		// Should have success: true
		assert.True(t, data["success"].(bool), "Response should have success: true")

		// Should have available languages
		available, hasAvailable := data["available"]
		assert.True(t, hasAvailable, "Response should have 'available' field")

		if hasAvailable {
			languages := available.([]interface{})
			assert.GreaterOrEqual(t, len(languages), 6,
				"Should have at least 6 available languages, got %d", len(languages))
			t.Logf("Available languages: %v", languages)
		}
	})

	t.Run("GET /agent/api/preferences/session-timeout returns value", func(t *testing.T) {
		err := browser.NavigateTo("/agent/dashboard")
		require.NoError(t, err)

		result, err := browser.Page.Evaluate(`async () => {
			try {
				const response = await fetch('/agent/api/preferences/session-timeout', {
					credentials: 'include',
					headers: { 'Accept': 'application/json' }
				});
				const data = await response.json();
				return { status: response.status, data: data };
			} catch (e) {
				return { error: e.message };
			}
		}`)
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})

		if errMsg, hasErr := resultMap["error"]; hasErr {
			t.Fatalf("API call failed: %v", errMsg)
		}

		status := resultMap["status"].(float64)
		data := resultMap["data"].(map[string]interface{})

		assert.Equal(t, float64(200), status, "GET /agent/api/preferences/session-timeout should return 200")
		assert.True(t, data["success"].(bool), "Response should have success: true")
	})

	t.Run("GET /agent/api/profile returns user data", func(t *testing.T) {
		err := browser.NavigateTo("/agent/dashboard")
		require.NoError(t, err)

		result, err := browser.Page.Evaluate(`async () => {
			try {
				const response = await fetch('/agent/api/profile', {
					credentials: 'include',
					headers: { 'Accept': 'application/json' }
				});
				const data = await response.json();
				return { status: response.status, data: data };
			} catch (e) {
				return { error: e.message };
			}
		}`)
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})

		if errMsg, hasErr := resultMap["error"]; hasErr {
			t.Fatalf("API call failed: %v", errMsg)
		}

		status := resultMap["status"].(float64)
		data := resultMap["data"].(map[string]interface{})

		assert.Equal(t, float64(200), status, "GET /agent/api/profile should return 200")
		assert.True(t, data["success"].(bool), "Response should have success: true")

		// Should have profile data
		_, hasProfile := data["profile"]
		assert.True(t, hasProfile, "Response should have 'profile' field")
	})

	t.Run("POST /agent/api/profile saves profile data", func(t *testing.T) {
		err := browser.NavigateTo("/agent/dashboard")
		require.NoError(t, err)

		result, err := browser.Page.Evaluate(`async () => {
			try {
				const response = await fetch('/agent/api/profile', {
					method: 'POST',
					credentials: 'include',
					headers: {
						'Accept': 'application/json',
						'Content-Type': 'application/json'
					},
					body: JSON.stringify({
						first_name: 'Test',
						last_name: 'User',
						title: 'Mr.'
					})
				});
				const data = await response.json();
				return { status: response.status, data: data };
			} catch (e) {
				return { error: e.message };
			}
		}`)
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})

		if errMsg, hasErr := resultMap["error"]; hasErr {
			t.Fatalf("API call failed: %v", errMsg)
		}

		status := resultMap["status"].(float64)
		data := resultMap["data"].(map[string]interface{})

		assert.Equal(t, float64(200), status, "POST /agent/api/profile should return 200")
		assert.True(t, data["success"].(bool), "Response should have success: true")
	})
}
