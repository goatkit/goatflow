//go:build playwright

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

func TestTranslationKeys(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	// Skip if admin credentials not provided (these pages require auth)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured; set DEMO_ADMIN_EMAIL and DEMO_ADMIN_PASSWORD to run translation key checks")
	}
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Check for untranslated keys on Groups page", func(t *testing.T) {
		// Login as admin
		err := auth.LoginAsAdmin()
		require.NoError(t, err, "Login should succeed")

		// Navigate to admin dashboard first
		err = browser.NavigateTo("/admin")
		require.NoError(t, err, "Should navigate to admin dashboard")
		time.Sleep(2 * time.Second)

		// Get all text content from the page
		dashboardText, err := browser.Page.Locator("body").InnerText()
		require.NoError(t, err, "Should get dashboard text")

		// Check for translation keys pattern (word.word or word.word.word)
		untranslatedKeys := findTranslationKeys(dashboardText)
		if len(untranslatedKeys) > 0 {
			t.Errorf("Found untranslated keys on admin dashboard: %v", untranslatedKeys)
		}

		// Navigate to groups page
		err = browser.NavigateTo("/admin/groups")
		require.NoError(t, err, "Should navigate to groups page")
		time.Sleep(2 * time.Second)

		// Get all text content from the groups page
		groupsText, err := browser.Page.Locator("body").InnerText()
		require.NoError(t, err, "Should get groups page text")

		// Check for translation keys
		groupsUntranslated := findTranslationKeys(groupsText)
		if len(groupsUntranslated) > 0 {
			t.Errorf("Found untranslated keys on groups page: %v", groupsUntranslated)
		}

		// Also check specific elements that commonly have translation issues
		checkElementsForTranslationKeys(t, browser)
	})

	t.Run("Check Groups modal for untranslated keys", func(t *testing.T) {
		// Ensure we're logged in (in case prior subtest failed early)
		if !auth.IsLoggedIn() {
			err := auth.LoginAsAdmin()
			require.NoError(t, err, "Login should succeed for modal test")
		}
		// Make sure we're on the groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Click Add Group button to open modal
		addButton := browser.Page.Locator("button").Filter(
			playwright.LocatorFilterOptions{
				HasText: "Add Group",
			},
		)

		// Check if button text itself is a translation key
		buttonText, _ := addButton.InnerText()
		if strings.Contains(buttonText, "admin.") || strings.Contains(buttonText, "app.") {
			t.Errorf("Add button has untranslated key: %s", buttonText)
		}

		err = addButton.Click()
		assert.NoError(t, err, "Should open add group modal")
		time.Sleep(1 * time.Second)

		// Get modal content
		modal := browser.Page.Locator("#groupModal")
		modalText, err := modal.InnerText()
		if err == nil {
			modalUntranslated := findTranslationKeys(modalText)
			if len(modalUntranslated) > 0 {
				t.Errorf("Found untranslated keys in modal: %v", modalUntranslated)
			}
		}
	})
}

// findTranslationKeys looks for patterns like "admin.something" or "app.something"
func findTranslationKeys(text string) []string {
	var keys []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		// Look for translation key patterns
		words := strings.Fields(line)
		for _, word := range words {
			// Check if word matches translation key pattern
			if isTranslationKey(word) {
				keys = append(keys, word)
			}
		}
	}

	return unique(keys)
}

// isTranslationKey checks if a string looks like a translation key
func isTranslationKey(s string) bool {
	// Remove trailing punctuation
	s = strings.TrimSuffix(s, ".")
	s = strings.TrimSuffix(s, ",")
	s = strings.TrimSuffix(s, ":")
	s = strings.TrimSuffix(s, ";")
	s = strings.TrimSuffix(s, "!")
	s = strings.TrimSuffix(s, "?")

	// Common translation key prefixes
	prefixes := []string{
		"admin.",
		"app.",
		"common.",
		"user.",
		"dashboard.",
		"tickets.",
		"queue.",
		"error.",
		"success.",
		"warning.",
		"info.",
		"time.",
		"status.",
		"priority.",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			// Make sure it's not a URL or file path
			if !strings.Contains(s, "/") && !strings.Contains(s, "http") {
				return true
			}
		}
	}

	return false
}

// checkElementsForTranslationKeys checks specific UI elements
func checkElementsForTranslationKeys(t *testing.T, browser *helpers.BrowserHelper) {
	// Check page title
	title := browser.Page.Locator("h1")
	titleText, err := title.InnerText()
	if err == nil && isTranslationKey(titleText) {
		t.Errorf("Page title is untranslated: %s", titleText)
	}

	// Check all buttons
	buttons := browser.Page.Locator("button")
	count, _ := buttons.Count()
	for i := 0; i < count; i++ {
		btn := buttons.Nth(i)
		btnText, err := btn.InnerText()
		if err == nil && btnText != "" && isTranslationKey(btnText) {
			t.Errorf("Button %d has untranslated text: %s", i, btnText)
		}
	}

	// Check all links
	links := browser.Page.Locator("a")
	linkCount, _ := links.Count()
	for i := 0; i < linkCount; i++ {
		link := links.Nth(i)
		linkText, err := link.InnerText()
		if err == nil && linkText != "" && isTranslationKey(linkText) {
			t.Errorf("Link %d has untranslated text: %s", i, linkText)
		}
	}

	// Check table headers
	headers := browser.Page.Locator("th")
	headerCount, _ := headers.Count()
	for i := 0; i < headerCount; i++ {
		header := headers.Nth(i)
		headerText, err := header.InnerText()
		if err == nil && headerText != "" && isTranslationKey(headerText) {
			t.Errorf("Table header %d has untranslated text: %s", i, headerText)
		}
	}

	// Check labels
	labels := browser.Page.Locator("label")
	labelCount, _ := labels.Count()
	for i := 0; i < labelCount; i++ {
		label := labels.Nth(i)
		labelText, err := label.InnerText()
		if err == nil && labelText != "" && isTranslationKey(labelText) {
			t.Errorf("Label %d has untranslated text: %s", i, labelText)
		}
	}

	// Check for any visible text containing dots (potential translation keys)
	allText := browser.Page.Locator("*:visible")
	textCount, _ := allText.Count()
	for i := 0; i < textCount && i < 100; i++ { // Limit to first 100 elements for performance
		elem := allText.Nth(i)
		elemText, err := elem.InnerText()
		if err == nil && elemText != "" {
			// Check if this element's direct text (not children) contains translation keys
			if strings.Count(elemText, ".") > 0 && len(elemText) < 50 {
				possibleKeys := findTranslationKeys(elemText)
				if len(possibleKeys) > 0 {
					tagName, _ := elem.Evaluate("el => el.tagName", nil)
					t.Logf("Warning: Element <%s> might have untranslated keys: %v", tagName, possibleKeys)
				}
			}
		}
	}
}

// unique returns unique strings from a slice
func unique(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
