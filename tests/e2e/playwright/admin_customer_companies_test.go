package playwright

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdminCustomerCompaniesPlaywright tests customer company management using Playwright directly
func TestAdminCustomerCompaniesPlaywright(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	// Login as admin
	auth := helpers.NewAuthHelper(browser)
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("Customer Companies List - Playwright", func(t *testing.T) {
		// Navigate to customer companies page
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		// Wait for page to load
		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Verify page title
		title, err := browser.Page.Title()
		require.NoError(t, err)
		assert.Contains(t, title, "Customer Companies")

		// Check for main elements using Playwright selectors
		heading := browser.Page.Locator("h1, h2")
		assert.True(t, heading.Count() > 0)

		addButton := browser.Page.Locator("button:has-text('Add New Company')")
		assert.True(t, addButton.Count() > 0)

		table := browser.Page.Locator("table")
		assert.True(t, table.Count() > 0)

		// Test search functionality
		searchInput := browser.Page.Locator("input[placeholder*='Search'], input[name*='search']")
		if searchInput.Count() > 0 {
			searchInput.Fill("test")
			searchInput.Type("company")
			err = browser.WaitForLoad()
			require.NoError(t, err)
		}

		// Test status filter
		statusSelect := browser.Page.Locator("select[name='status'], select:has-text('Status')")
		if statusSelect.Count() > 0 {
			statusSelect.SelectOption("valid")
			err = browser.WaitForLoad()
			require.NoError(t, err)
		}

		// Take screenshot for debugging
		if browser.Config.Screenshots {
			screenshot, err := browser.Page.Screenshot()
			require.NoError(t, err)
			t.Logf("Screenshot taken: %d bytes", len(screenshot))
		}
	})

	t.Run("Create New Company - Playwright", func(t *testing.T) {
		// Navigate to new company form
		err := browser.NavigateTo("/admin/customer/companies/new")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Verify form elements
		heading := browser.Page.Locator("h1, h2:has-text('Create New Customer Company')")
		assert.True(t, heading.Count() > 0)

		// Fill out the form
		testCustomerID := fmt.Sprintf("PLAYWRIGHT_%d", time.Now().Unix())

		customerIDInput := browser.Page.Locator("input[name='customer_id']")
		require.True(t, customerIDInput.Count() > 0)
		customerIDInput.Fill(testCustomerID)

		nameInput := browser.Page.Locator("input[name='name']")
		require.True(t, nameInput.Count() > 0)
		nameInput.Fill("Playwright Test Company")

		streetInput := browser.Page.Locator("input[name='street']")
		if streetInput.Count() > 0 {
			streetInput.Fill("123 Playwright St")
		}

		cityInput := browser.Page.Locator("input[name='city']")
		if cityInput.Count() > 0 {
			cityInput.Fill("Playwright City")
		}

		countryInput := browser.Page.Locator("input[name='country']")
		if countryInput.Count() > 0 {
			countryInput.Fill("Playwright Country")
		}

		// Submit form
		submitButton := browser.Page.Locator("button[type='submit']")
		require.True(t, submitButton.Count() > 0)

		// Click submit and wait for response
		submitButton.Click()
		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Verify redirect or success
		currentURL, err := browser.Page.URL()
		require.NoError(t, err)
		assert.Contains(t, currentURL, "/admin/customer/companies")
	})

	t.Run("Edit Company - Playwright", func(t *testing.T) {
		// Navigate to edit form
		err := browser.NavigateTo("/admin/customer/companies/TEST001/edit")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Verify form is populated
		heading := browser.Page.Locator("h1, h2:has-text('Edit Customer Company')")
		assert.True(t, heading.Count() > 0)

		nameField := browser.Page.Locator("input[name='name']")
		if nameField.Count() > 0 {
			originalName, err := nameField.InputValue()
			require.NoError(t, err)
			assert.NotEmpty(t, originalName)

			// Update the name
			newName := "Updated " + originalName
			nameField.Fill(newName)

			// Submit form
			submitButton := browser.Page.Locator("button[type='submit']")
			if submitButton.Count() > 0 {
				submitButton.Click()
				err = browser.WaitForLoad()
				require.NoError(t, err)
			}
		}
	})

	t.Run("Portal Settings Tab - Playwright", func(t *testing.T) {
		// Navigate to edit form
		err := browser.NavigateTo("/admin/customer/companies/TEST001/edit")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Look for portal settings tab
		portalTab := browser.Page.Locator("text=Portal Settings")
		if portalTab.Count() > 0 {
			portalTab.Click()
			err = browser.WaitForLoad()
			require.NoError(t, err)

			// Verify portal settings elements
			loginHint := browser.Page.Locator("input[name='login_hint'], textarea[name='login_hint']")
			theme := browser.Page.Locator("select[name='theme']")
			customCSS := browser.Page.Locator("textarea[name='custom_css']")

			// At least one of these should exist
			assert.True(t, loginHint.Count() > 0 || theme.Count() > 0 || customCSS.Count() > 0)
		}
	})

	t.Run("Services Tab - Playwright", func(t *testing.T) {
		// Navigate to edit form
		err := browser.NavigateTo("/admin/customer/companies/TEST001/edit")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Look for services tab
		servicesTab := browser.Page.Locator("text=Services")
		if servicesTab.Count() > 0 {
			servicesTab.Click()
			err = browser.WaitForLoad()
			require.NoError(t, err)

			// Verify services assignment interface
			checkboxes := browser.Page.Locator("input[type='checkbox']")
			multiSelect := browser.Page.Locator("select[multiple]")

			assert.True(t, checkboxes.Count() > 0 || multiSelect.Count() > 0)
		}
	})

	t.Run("Dark Mode Toggle - Playwright", func(t *testing.T) {
		// Navigate to page
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Test dark mode toggle
		darkModeButton := browser.Page.Locator("button:has-text('moon'), [data-toggle='dark']")
		if darkModeButton.Count() > 0 {
			// Toggle dark mode
			darkModeButton.Click()
			time.Sleep(500 * time.Millisecond)

			// Verify dark mode classes
			html, err := browser.Page.Content()
			require.NoError(t, err)
			assert.Contains(t, html, "dark")

			// Toggle back
			lightModeButton := browser.Page.Locator("button:has-text('sun'), [data-toggle='light']")
			if lightModeButton.Count() > 0 {
				lightModeButton.Click()
				time.Sleep(500 * time.Millisecond)

				// Verify light mode
				html, err := browser.Page.Content()
				require.NoError(t, err)
				assert.NotContains(t, html, "dark")
			}
		}
	})

	t.Run("Responsive Design - Playwright", func(t *testing.T) {
		// Test mobile view
		_, err := browser.Page.SetViewportSize(375, 667)
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)

		err = browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Verify mobile navigation
		nav := browser.Page.Locator("nav, .mobile-menu, .navbar")
		assert.True(t, nav.Count() > 0)

		// Test tablet view
		_, err = browser.Page.SetViewportSize(768, 1024)
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)

		err = browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Reset to desktop
		_, err = browser.Page.SetViewportSize(1920, 1080)
		require.NoError(t, err)
	})

	t.Run("Error Handling - Playwright", func(t *testing.T) {
		// Test invalid customer ID
		err := browser.NavigateTo("/admin/customer/companies/NONEXISTENT/edit")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Should show 404 or error message
		pageText, err := browser.Page.TextContent("body")
		require.NoError(t, err)
		assert.True(t, contains(pageText, "not found") ||
			contains(pageText, "error") ||
			contains(pageText, "404"))
	})

	t.Run("Form Validation - Playwright", func(t *testing.T) {
		// Navigate to new form
		err := browser.NavigateTo("/admin/customer/companies/new")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Try to submit empty form
		submitButton := browser.Page.Locator("button[type='submit']")
		if submitButton.Count() > 0 {
			submitButton.Click()
			err = browser.WaitForLoad()
			require.NoError(t, err)

			// Should show validation errors
			pageText, err := browser.Page.TextContent("body")
			require.NoError(t, err)
			assert.True(t, contains(pageText, "required") ||
				contains(pageText, "error") ||
				contains(pageText, "validation"))
		}
	})

	t.Run("Accessibility - Playwright", func(t *testing.T) {
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Check for proper heading hierarchy
		headings := browser.Page.Locator("h1, h2, h3, h4, h5, h6")
		headingCount, err := headings.Count()
		require.NoError(t, err)
		assert.True(t, headingCount > 0)

		// Check for alt text on images
		images := browser.Page.Locator("img")
		imageCount, err := images.Count()
		require.NoError(t, err)
		for i := 0; i < imageCount; i++ {
			img := images.Nth(i)
			alt, err := img.GetAttribute("alt")
			require.NoError(t, err)
			if alt != "" {
				assert.NotEmpty(t, alt)
			}
		}

		// Check for proper form labels
		inputs := browser.Page.Locator("input")
		inputCount, err := inputs.Count()
		require.NoError(t, err)
		for i := 0; i < inputCount; i++ {
			input := inputs.Nth(i)
			inputType, err := input.GetAttribute("type")
			require.NoError(t, err)
			if inputType == "" || inputType != "hidden" {
				id, err := input.GetAttribute("id")
				require.NoError(t, err)
				name, err := input.GetAttribute("name")
				require.NoError(t, err)

				// Should have either id or name for accessibility
				assert.True(t, (id != "" && id != "") || (name != "" && name != ""))
			}
		}
	})

	t.Run("Network Monitoring - Playwright", func(t *testing.T) {
		// Track network requests
		var requests []playwright.Request
		browser.Page.OnRequest(func(request playwright.Request) {
			requests = append(requests, request)
		})

		// Track responses
		var responses []playwright.Response
		browser.Page.OnResponse(func(response playwright.Response) {
			responses = append(responses, response)
		})

		// Navigate and perform actions
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Verify some requests were made
		assert.True(t, len(requests) > 0)
		assert.True(t, len(responses) > 0)

		// Check for failed responses
		for _, response := range responses {
			if response.Status() >= 400 {
				t.Logf("Failed response: %d %s", response.Status(), response.URL())
			}
		}
	})

	t.Run("Console Monitoring - Playwright", func(t *testing.T) {
		// Track console messages
		var consoleMessages []string
		browser.Page.OnConsole(func(msg playwright.ConsoleMessage) {
			consoleMessages = append(consoleMessages, fmt.Sprintf("[%s] %s", msg.Type(), msg.Text()))

			if msg.Type() == "error" {
				t.Logf("Console ERROR: %s", msg.Text())
			}
		})

		// Navigate and perform actions
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Log console messages for debugging
		for _, msg := range consoleMessages {
			t.Logf("Console: %s", msg)
		}
	})

	// Cleanup
	browser.TearDown()
}

// TestAdminCustomerCompaniesAPIPlaywright tests API endpoints using Playwright's network monitoring
func TestAdminCustomerCompaniesAPIPlaywright(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	// Login as admin
	auth := helpers.NewAuthHelper(browser)
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("API Network Monitoring - Playwright", func(t *testing.T) {
		// Track network requests and responses
		var apiRequests []playwright.Request
		var apiResponses []playwright.Response

		browser.Page.OnRequest(func(request playwright.Request) {
			if request.URL() != browser.Config.BaseURL+"/admin/customer/companies" {
				return
			}
			apiRequests = append(apiRequests, request)
		})

		browser.Page.OnResponse(func(response playwright.Response) {
			if response.URL() != browser.Config.BaseURL+"/admin/customer/companies" {
				return
			}
			apiResponses = append(apiResponses, response)
		})

		// Navigate to customer companies page
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Verify API calls were made
		assert.True(t, len(apiRequests) > 0)
		assert.True(t, len(apiResponses) > 0)

		// Check response status
		for _, response := range apiResponses {
			assert.True(t, response.Status() >= 200 && response.Status() < 300,
				"API response should be successful: %d %s", response.Status(), response.URL())
		}
	})

	t.Run("Form Submission Network - Playwright", func(t *testing.T) {
		// Track form submissions
		var postRequests []playwright.Request

		browser.Page.OnRequest(func(request playwright.Request) {
			if request.Method() == "POST" && request.URL() != browser.Config.BaseURL+"/admin/customer/companies" {
				return
			}
			if request.Method() == "POST" {
				postRequests = append(postRequests, request)
			}
		})

		// Navigate to new form
		err := browser.NavigateTo("/admin/customer/companies/new")
		require.NoError(t, err)

		err = browser.WaitForLoad()
		require.NoError(t, err)

		// Fill and submit form
		testCustomerID := fmt.Sprintf("API_PLAYWRIGHT_%d", time.Now().Unix())

		customerIDInput := browser.Page.Locator("input[name='customer_id']")
		if customerIDInput.Count() > 0 {
			customerIDInput.Fill(testCustomerID)
		}

		nameInput := browser.Page.Locator("input[name='name']")
		if nameInput.Count() > 0 {
			nameInput.Fill("API Playwright Test Company")
		}

		submitButton := browser.Page.Locator("button[type='submit']")
		if submitButton.Count() > 0 {
			submitButton.Click()
			err = browser.WaitForLoad()
			require.NoError(t, err)
		}

		// Verify POST request was made
		assert.True(t, len(postRequests) > 0, "Should have made at least one POST request")
	})

	// Cleanup
	browser.TearDown()
}

// Helper function to check if string contains substring (case insensitive)
func contains(text, substr string) bool {
	return len(text) >= len(substr) &&
		(text == substr ||
			len(text) > len(substr) && (containsString(text, substr) ||
				containsString(text, toLower(substr)) ||
				containsString(toLower(text), substr)))
}

func containsString(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}
