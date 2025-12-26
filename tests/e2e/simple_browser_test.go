//go:build e2e

package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestSimpleBrowser tests basic browser automation
func TestSimpleBrowser(t *testing.T) {
	// Skip if not in browser mode
	if os.Getenv("SKIP_BROWSER") == "true" {
		t.Skip("Skipping browser test")
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://backend:8080"
	}

	// Run Playwright
	err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
	})
	if err != nil {
		t.Logf("Note: Playwright install returned: %v (this is OK if browsers are pre-installed)", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		t.Skipf("Could not start Playwright: %v (browsers may not be installed)", err)
		return
	}
	defer pw.Stop()

	// Launch browser
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	require.NoError(t, err, "Failed to launch browser")
	defer browser.Close()

	// Create page
	page, err := browser.NewPage()
	require.NoError(t, err, "Failed to create page")
	defer page.Close()

	// Navigate to login
	_, err = page.Goto(baseURL + "/login")
	require.NoError(t, err, "Failed to navigate to login")

	// Take screenshot
	screenshotPath := "/tmp/login-page.png"
	_, err = page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(screenshotPath),
	})
	if err == nil {
		t.Logf("✅ Screenshot saved to %s", screenshotPath)
	}

	// Check for login form
	emailInput, err := page.Locator("input#email").Count()
	require.NoError(t, err)
	require.Greater(t, emailInput, 0, "Email input should exist")

	// Fill login form
	err = page.Fill("input#email", os.Getenv("DEMO_ADMIN_EMAIL"))
	require.NoError(t, err, "Failed to fill email")

	err = page.Fill("input#password", os.Getenv("DEMO_ADMIN_PASSWORD"))
	require.NoError(t, err, "Failed to fill password")

	// Click submit
	err = page.Click("button[type='submit']")
	require.NoError(t, err, "Failed to click submit")

	// Wait for navigation
	time.Sleep(2 * time.Second)

	// Check if we're logged in
	url := page.URL()
	t.Logf("After login URL: %s", url)

	if url == baseURL+"/dashboard" {
		t.Log("✅ Successfully logged in and redirected to dashboard")

		// Navigate to queues
		_, err = page.Goto(baseURL + "/queues")
		if err == nil {
			t.Log("✅ Successfully navigated to queues page")

			// Try to find edit button
			editButtons, _ := page.Locator("a[href*='edit'], button:has-text('Edit')").Count()
			t.Logf("Found %d edit buttons on queues page", editButtons)

			if editButtons > 0 {
				// Click first edit button
				err = page.Locator("a[href*='edit'], button:has-text('Edit')").First().Click()
				if err == nil {
					time.Sleep(1 * time.Second)

					// Check for populated form fields
					nameInput, _ := page.Locator("input[name='name'], input#name").First().InputValue()
					if nameInput != "" {
						t.Logf("✅ Edit form name field populated with: %s", nameInput)
					}

					commentTextarea, _ := page.Locator("textarea[name='comment'], textarea#comment").First().InputValue()
					if commentTextarea != "" {
						t.Logf("✅ Edit form comment field populated with: %s", commentTextarea)
					}
				}
			}
		}
	}
}
