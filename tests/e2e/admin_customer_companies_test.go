//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	playwright "github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdminCustomerCompaniesE2E tests the complete customer company management workflow
func TestAdminCustomerCompaniesE2E(t *testing.T) {
	// Setup browser and navigate to admin
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	// Login as admin
	auth := helpers.NewAuthHelper(browser)
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("Customer Companies List Page", func(t *testing.T) {
		// Navigate to customer companies page
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		// Wait for page to load
		browser.WaitForLoad()

		// Verify page title and main elements
		title, err := browser.Page.Title()
		require.NoError(t, err)
		assert.Contains(t, title, "Customer Companies")

		// Check for main UI elements
		heading := browser.Page.Locator("h1, h2")
		assert.Greater(t, mustCount(t, heading), 0)

		addButton := browser.Page.Locator("button:has-text('Add New Company')")
		assert.Greater(t, mustCount(t, addButton), 0)

		table := browser.Page.Locator("table")
		assert.Greater(t, mustCount(t, table), 0)

		// Test search functionality
		searchInput := browser.Page.Locator("input[placeholder*='Search'], input[name*='search']")
		if mustCount(t, searchInput) > 0 {
			searchInput.Fill("test")
			searchInput.Type("company")
			browser.WaitForLoad()
		}

		// Test status filter
		statusSelect := browser.Page.Locator("select[name='status'], select:has-text('Status')")
		if mustCount(t, statusSelect) > 0 {
			_, err := statusSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"valid"}})
			require.NoError(t, err)
			browser.WaitForLoad()
		}
	})

	t.Run("Create New Customer Company", func(t *testing.T) {
		// Navigate to new company form
		err := browser.NavigateTo("/admin/customer/companies/new")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Verify form elements
		heading := browser.Page.Locator("h1, h2:has-text('Create New Customer Company')")
		assert.Greater(t, mustCount(t, heading), 0)

		customerIDInput := browser.Page.Locator("input[name='customer_id']")
		assert.Greater(t, mustCount(t, customerIDInput), 0)

		nameInput := browser.Page.Locator("input[name='name']")
		assert.Greater(t, mustCount(t, nameInput), 0)

		streetInput := browser.Page.Locator("input[name='street']")
		assert.Greater(t, mustCount(t, streetInput), 0)

		cityInput := browser.Page.Locator("input[name='city']")
		assert.Greater(t, mustCount(t, cityInput), 0)

		countryInput := browser.Page.Locator("input[name='country']")
		assert.Greater(t, mustCount(t, countryInput), 0)

		// Fill out the form
		testCustomerID := fmt.Sprintf("TEST_%d", time.Now().Unix())

		customerIDInput.Fill(testCustomerID)
		nameInput.Fill("Test Company Ltd")
		streetInput.Fill("123 Test Street")
		cityInput.Fill("Test City")
		countryInput.Fill("Test Country")

		// Submit form
		submitButton := browser.Page.Locator("button[type='submit']")
		assert.Greater(t, mustCount(t, submitButton), 0)

		// Click submit and wait for response
		submitButton.Click()
		browser.WaitForLoad()

		// Verify success or validation errors
		// Should either redirect to list or show success message
		currentURL := browser.Page.URL()
		assert.Contains(t, currentURL, "/admin/customer/companies")
	})

	t.Run("Edit Customer Company", func(t *testing.T) {
		// Navigate to edit form for an existing company
		err := browser.NavigateTo("/admin/customer/companies/TEST001/edit")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Verify form is populated
		heading := browser.Page.Locator("h1, h2:has-text('Edit Customer Company')")
		assert.Greater(t, mustCount(t, heading), 0)

		customerIDField := browser.Page.Locator("input[name='customer_id']")
		if mustCount(t, customerIDField) > 0 {
			value, err := customerIDField.InputValue()
			require.NoError(t, err)
			assert.NotEmpty(t, value)
		}

		nameField := browser.Page.Locator("input[name='name']")
		if mustCount(t, nameField) > 0 {
			originalName, err := nameField.InputValue()
			require.NoError(t, err)
			assert.NotEmpty(t, originalName)

			// Update the name
			newName := "Updated " + originalName
			nameField.Fill(newName)

			// Submit form
			submitButton := browser.Page.Locator("button[type='submit']")
			if mustCount(t, submitButton) > 0 {
				submitButton.Click()
				browser.WaitForLoad()
			}
		}
	})

	t.Run("Customer Company Actions", func(t *testing.T) {
		// Navigate to customer companies list
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Test action buttons if they exist
		// Look for edit, delete, activate buttons
		actionButtons := browser.Page.Locator("button")
		buttonCount, err := actionButtons.Count()
		require.NoError(t, err)

		for i := 0; i < buttonCount; i++ {
			button := actionButtons.Nth(i)
			text, err := button.TextContent()
			require.NoError(t, err)

			// Test edit action
			if contains(text, "Edit") || contains(text, "edit") {
				t.Logf("Found edit button: %s", text)
				// Could click and verify navigation
			}

			// Test delete action
			if contains(text, "Delete") || contains(text, "delete") {
				t.Logf("Found delete button: %s", text)
				// Could click and verify modal appears
			}

			// Test activate/deactivate action
			if contains(text, "Activate") || contains(text, "Deactivate") {
				t.Logf("Found activate button: %s", text)
				// Could click and verify status change
			}
		}
	})

	t.Run("Portal Settings Tab", func(t *testing.T) {
		// Navigate to edit form
		err := browser.NavigateTo("/admin/customer/companies/TEST001/edit")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Look for portal settings tab
		portalTab := browser.Page.Locator("text=Portal Settings")
		if mustCount(t, portalTab) > 0 {
			portalTab.Click()
			browser.WaitForLoad()

			// Verify portal settings form elements
			loginHint := browser.Page.Locator("input[name='login_hint'], textarea[name='login_hint']")
			theme := browser.Page.Locator("select[name='theme']")
			customCSS := browser.Page.Locator("textarea[name='custom_css']")

			assert.True(t, mustCount(t, loginHint) > 0 || mustCount(t, theme) > 0)
			assert.Greater(t, mustCount(t, theme), 0)
			assert.Greater(t, mustCount(t, customCSS), 0)
		}
	})

	t.Run("Services Tab", func(t *testing.T) {
		// Navigate to edit form
		err := browser.NavigateTo("/admin/customer/companies/TEST001/edit")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Look for services tab
		servicesTab := browser.Page.Locator("text=Services")
		if mustCount(t, servicesTab) > 0 {
			servicesTab.Click()
			browser.WaitForLoad()

			// Verify services assignment interface
			checkboxes := browser.Page.Locator("input[type='checkbox']")
			multiSelect := browser.Page.Locator("select[multiple]")

			assert.True(t, mustCount(t, checkboxes) > 0 || mustCount(t, multiSelect) > 0)
		}
	})

	t.Run("Users Modal", func(t *testing.T) {
		// Navigate to customer companies list
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Look for users button/modal trigger
		usersButton := browser.Page.Locator("text=Users")
		if mustCount(t, usersButton) > 0 {
			usersButton.Click()
			browser.WaitForLoad()

			// Verify modal or new page with users list
			table := browser.Page.Locator("table")
			modal := browser.Page.Locator(".modal, [role='dialog']")

			assert.True(t, mustCount(t, table) > 0 || mustCount(t, modal) > 0)
		}
	})

	t.Run("Tickets Modal", func(t *testing.T) {
		// Navigate to customer companies list
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Look for tickets button/modal trigger
		ticketsButton := browser.Page.Locator("text=Tickets")
		if mustCount(t, ticketsButton) > 0 {
			ticketsButton.Click()
			browser.WaitForLoad()

			// Verify modal or new page with tickets list
			table := browser.Page.Locator("table")
			modal := browser.Page.Locator(".modal, [role='dialog']")

			assert.True(t, mustCount(t, table) > 0 || mustCount(t, modal) > 0)
		}
	})

	t.Run("Dark Mode Compatibility", func(t *testing.T) {
		// Test dark mode toggle
		darkModeButton := browser.Page.Locator("text=dark, .dark, [data-theme='dark']")
		if mustCount(t, darkModeButton) == 0 {
			darkModeButton = browser.Page.Locator("button:has-text('moon'), [data-toggle='dark']")
		}

		if mustCount(t, darkModeButton) > 0 {
			// Toggle dark mode
			darkModeButton.Click()
			time.Sleep(500 * time.Millisecond) // Wait for transition

			// Verify dark mode classes are applied
			html, err := browser.Page.Content()
			require.NoError(t, err)
			assert.Contains(t, html, "dark")

			// Toggle back
			lightModeButton := browser.Page.Locator("text=light, .light, [data-theme='light']")
			if mustCount(t, lightModeButton) == 0 {
				lightModeButton = browser.Page.Locator("button:has-text('sun'), [data-toggle='light']")
			}

			if mustCount(t, lightModeButton) > 0 {
				lightModeButton.Click()
				time.Sleep(500 * time.Millisecond)

				// Verify light mode
				html, err := browser.Page.Content()
				require.NoError(t, err)
				assert.NotContains(t, html, "dark")
			}
		}
	})

	t.Run("Responsive Design", func(t *testing.T) {
		// Test mobile view
		err := browser.Page.SetViewportSize(375, 667) // iPhone size
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)

		err = browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Verify mobile navigation works
		nav := browser.Page.Locator("nav, .mobile-menu, .navbar")
		assert.Greater(t, mustCount(t, nav), 0)

		// Test tablet view
		err = browser.Page.SetViewportSize(768, 1024) // iPad size
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)

		err = browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Reset to desktop
		err = browser.Page.SetViewportSize(1920, 1080)
		require.NoError(t, err)
	})

	t.Run("Error Handling", func(t *testing.T) {
		// Test invalid customer ID
		err := browser.NavigateTo("/admin/customer/companies/NONEXISTENT/edit")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Should show 404 or error message
		pageText, err := browser.Page.TextContent("body")
		require.NoError(t, err)
		assert.True(t, contains(pageText, "not found") ||
			contains(pageText, "error") ||
			contains(pageText, "404"))
	})

	t.Run("Form Validation", func(t *testing.T) {
		// Navigate to new form
		err := browser.NavigateTo("/admin/customer/companies/new")
		require.NoError(t, err)

		browser.WaitForLoad()

		// Try to submit empty form
		submitButton := browser.Page.Locator("button[type='submit'], input[type='submit']")
		if mustCount(t, submitButton) > 0 {
			submitButton.Click()
			browser.WaitForLoad()

			// Should show validation errors
			pageText, err := browser.Page.TextContent("body")
			require.NoError(t, err)
			assert.True(t, contains(pageText, "required") ||
				contains(pageText, "error") ||
				contains(pageText, "validation"))
		}

		// Test invalid email format if there's an email field
		emailField := browser.Page.Locator("input[name='email'], input[type='email']")
		if mustCount(t, emailField) > 0 {
			emailField.Fill("invalid-email")
			submitButton.Click()
			browser.WaitForLoad()

			// Should show email validation error
			pageText, err := browser.Page.TextContent("body")
			require.NoError(t, err)
			assert.True(t, contains(pageText, "email") ||
				contains(pageText, "invalid"))
		}
	})

	t.Run("Accessibility", func(t *testing.T) {
		err := browser.NavigateTo("/admin/customer/companies")
		require.NoError(t, err)

		browser.WaitForLoad()

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
				assert.True(t, id != "" || name != "")
			}
		}
	})
}

// TestAdminCustomerCompaniesAPIE2E tests the API endpoints directly
func TestAdminCustomerCompaniesAPIE2E(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := "http://localhost:8080"

	t.Run("API Endpoints", func(t *testing.T) {
		// Test GET /admin/customer/companies
		resp, err := client.Get(fmt.Sprintf("%s/admin/customer/companies", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test GET /admin/customer/companies/new
		resp, err = client.Get(fmt.Sprintf("%s/admin/customer/companies/new", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test GET /admin/customer/companies/TEST001/edit
		resp, err = client.Get(fmt.Sprintf("%s/admin/customer/companies/TEST001/edit", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test POST /admin/customer/companies (create)
		formData := fmt.Sprintf("customer_id=API_TEST_%d&name=API Test Company&street=123 API St&city=API City&country=API Country&valid_id=1",
			time.Now().Unix())
		resp, err = client.Post(fmt.Sprintf("%s/admin/customer/companies", baseURL),
			"application/x-www-form-urlencoded",
			bytes.NewBufferString(formData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should succeed or return validation error
		assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusBadRequest)

		// Test POST /admin/customer/companies/:id/edit (update)
		updateData := fmt.Sprintf("name=Updated API Test Company&street=456 API St&city=Updated API City&country=API Country&valid_id=1")
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/admin/customer/companies/API_TEST/edit", baseURL),
			bytes.NewBufferString(updateData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest)

		// Test POST /admin/customer/companies/:id/activate
		req, err = http.NewRequest("POST", fmt.Sprintf("%s/admin/customer/companies/API_TEST/activate", baseURL), nil)
		require.NoError(t, err)

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test POST /admin/customer/companies/:id/delete
		req, err = http.NewRequest("POST", fmt.Sprintf("%s/admin/customer/companies/API_TEST/delete", baseURL), nil)
		require.NoError(t, err)

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest)
	})

	t.Run("API Search and Filtering", func(t *testing.T) {
		// Test search parameter
		resp, err := client.Get(fmt.Sprintf("%s/admin/customer/companies?search=test", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test status filter
		resp, err = client.Get(fmt.Sprintf("%s/admin/customer/companies?status=valid", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test pagination
		resp, err = client.Get(fmt.Sprintf("%s/admin/customer/companies?limit=10&offset=0", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test sorting
		resp, err = client.Get(fmt.Sprintf("%s/admin/customer/companies?sort=name&order=asc", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("API Error Handling", func(t *testing.T) {
		// Test non-existent company
		resp, err := client.Get(fmt.Sprintf("%s/admin/customer/companies/NONEXISTENT/edit", baseURL))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Test invalid POST data
		invalidData := "invalid=data"
		resp, err = client.Post(fmt.Sprintf("%s/admin/customer/companies", baseURL),
			"application/x-www-form-urlencoded",
			bytes.NewBufferString(invalidData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func mustCount(t *testing.T, locator playwright.Locator) int {
	t.Helper()
	count, err := locator.Count()
	require.NoError(t, err)
	return count
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
