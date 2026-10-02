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

// LoginAsCustomer signs in through the customer portal login form with the
// seeded customer (SeedCustomerLogin) and waits until the portal redirect
// leaves /customer/login.
func (a *AuthHelper) LoginAsCustomer() error {
	if err := a.browser.NavigateTo("/customer/login"); err != nil {
		return fmt.Errorf("failed to navigate to customer login: %w", err)
	}
	loginInput := a.browser.Page.Locator("input#login")
	if err := loginInput.WaitFor(); err != nil {
		return fmt.Errorf("customer login input not found: %w", err)
	}
	if err := loginInput.Fill(SeedCustomerLogin); err != nil {
		return fmt.Errorf("failed to fill customer login: %w", err)
	}
	if err := a.browser.Page.Locator("input#password").Fill(SeedCustomerPassword); err != nil {
		return fmt.Errorf("failed to fill customer password: %w", err)
	}
	if err := a.browser.Page.Locator("form[action='/api/auth/customer/login'] button[type='submit']").Click(); err != nil {
		return fmt.Errorf("failed to submit customer login: %w", err)
	}
	base := a.browser.Config.BaseURL
	err := a.browser.Page.WaitForURL(func(url string) bool {
		return strings.HasPrefix(url, base+"/customer") && !strings.Contains(url, "/customer/login")
	}, playwright.PageWaitForURLOptions{Timeout: playwright.Float(10000)})
	if err == nil {
		return nil
	}
	body, _ := a.browser.Page.Locator("body").TextContent()
	if len(body) > 400 {
		body = body[:400]
	}
	return fmt.Errorf("customer login as %q did not leave the login page (url %s): %s",
		SeedCustomerLogin, a.browser.Page.URL(), strings.TrimSpace(body))
}

// Logout signs out through the user menu (avatar button -> Logout), the way a
// user does, and waits for the redirect to the login page. On pages without
// the app chrome it navigates to /logout directly.
func (a *AuthHelper) Logout() error {
	page := a.browser.Page
	avatar := page.Locator("nav button.gk-avatar")
	if n, err := avatar.Count(); err != nil {
		return err
	} else if n > 0 {
		if err := avatar.First().Click(); err != nil {
			return fmt.Errorf("failed to open user menu: %w", err)
		}
		if err := page.Locator(".gk-user-menu a[href$='/logout']").Click(); err != nil {
			return fmt.Errorf("failed to click Logout: %w", err)
		}
	} else if _, err := page.Goto(a.browser.Config.BaseURL + "/logout"); err != nil {
		return fmt.Errorf("failed to navigate to /logout: %w", err)
	}
	if err := page.WaitForURL("**/login"); err != nil {
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
