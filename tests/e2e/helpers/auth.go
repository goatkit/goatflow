package helpers

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// AuthHelper provides authentication utilities for tests
type AuthHelper struct {
	browser *BrowserHelper
}

// NewAuthHelper creates a new authentication helper
func NewAuthHelper(browser *BrowserHelper) *AuthHelper {
	return &AuthHelper{
		browser: browser,
	}
}

// Login performs login with the given credentials
func (a *AuthHelper) Login(email, password string) error {
	// Check if already logged in - skip re-login to avoid timeout waiting for login form
	if a.IsLoggedIn() {
		return nil
	}

	// Navigate to login page
	if err := a.browser.NavigateTo("/login"); err != nil {
		return fmt.Errorf("failed to navigate to login: %w", err)
	}

	// Check again after navigation - /login may redirect if already authenticated
	currentURL := a.browser.Page.URL()
	if strings.Contains(currentURL, "/dashboard") || strings.Contains(currentURL, "/tickets") {
		// Already logged in and redirected
		return nil
	}

	// Wait for login form
	emailInput := a.browser.Page.Locator("input#email")
	if err := emailInput.WaitFor(); err != nil {
		return fmt.Errorf("email input not found: %w", err)
	}

	// Fill in credentials
	if err := emailInput.Fill(email); err != nil {
		return fmt.Errorf("failed to fill email: %w", err)
	}

	passwordInput := a.browser.Page.Locator("input#password")
	if err := passwordInput.Fill(password); err != nil {
		return fmt.Errorf("failed to fill password: %w", err)
	}

	// Submit form
	submitButton := a.browser.Page.Locator("button[type='submit']")
	if err := submitButton.Click(); err != nil {
		return fmt.Errorf("failed to click submit: %w", err)
	}

	// Wait for navigation or HTMX response
	if err := a.browser.WaitForHTMX(); err != nil {
		return fmt.Errorf("failed waiting for login response: %w", err)
	}

	// Check if we're redirected to dashboard
	url := a.browser.Page.URL()
	if url == a.browser.Config.BaseURL+"/dashboard" {
		return nil
	}

	// Check for error message
	errorMsg := a.browser.Page.Locator("#error-message")
	if count, _ := errorMsg.Count(); count > 0 {
		text, _ := errorMsg.TextContent()
		html, _ := a.browser.Page.Content()
		snippet := strings.TrimSpace(html)
		if len(snippet) > 400 {
			snippet = snippet[:400]
		}
		return fmt.Errorf("login failed: %s; snippet: %s", strings.TrimSpace(text), snippet)
	}

	return nil
}

// LoginAsAdmin logs in with admin credentials from config
func (a *AuthHelper) LoginAsAdmin() error {
	if a.browser.Config.AdminEmail == "" || a.browser.Config.AdminPassword == "" {
		return fmt.Errorf("admin credentials not configured")
	}
	return a.Login(a.browser.Config.AdminEmail, a.browser.Config.AdminPassword)
}

// Logout performs logout
func (a *AuthHelper) Logout() error {
	// Prefer clicking a visible logout control if present
	logoutLink := a.browser.Page.Locator("a[href='/logout'], button:has-text('Logout')")
	if count, _ := logoutLink.Count(); count > 0 {
		if err := logoutLink.First().Click(); err == nil {
			if err := a.browser.Page.WaitForURL("**/login", playwright.PageWaitForURLOptions{Timeout: playwright.Float(5000)}); err == nil {
				return nil
			}
		}
		// Fall through to direct navigation if click or wait fails
	}

	// Fallback: navigate directly to the logout endpoint
	if _, err := a.browser.Page.Goto(a.browser.Config.BaseURL + "/logout"); err != nil {
		return fmt.Errorf("failed to navigate to /logout: %w", err)
	}
	if err := a.browser.Page.WaitForURL("**/login", playwright.PageWaitForURLOptions{Timeout: playwright.Float(5000)}); err != nil {
		return fmt.Errorf("logout redirect failed: %w", err)
	}
	return nil
}

// IsLoggedIn checks if the user is currently logged in
func (a *AuthHelper) IsLoggedIn() bool {
	// Check if we have a session by looking for dashboard elements
	dashboard := a.browser.Page.Locator("[data-page='dashboard']")
	if count, _ := dashboard.Count(); count > 0 {
		return true
	}

	// Or check URL
	url := a.browser.Page.URL()
	// Treat blank pages or non-app URLs as not logged in
	if url == "" || strings.HasPrefix(url, "about:") || !strings.HasPrefix(url, a.browser.Config.BaseURL) {
		return false
	}
	return url != a.browser.Config.BaseURL+"/login" &&
		url != a.browser.Config.BaseURL+"/"
}
