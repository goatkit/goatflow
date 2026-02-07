//go:build e2e

package playwright

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslationKeys(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	t.Logf("Using BASE_URL: %s", browser.Config.BaseURL)
	// Quick reachability check
	client := &http.Client{Timeout: 800 * time.Millisecond}
	if resp, err := client.Get(browser.Config.BaseURL + "/login"); err != nil {
		t.Skipf("Backend not reachable at %s/login: %v", browser.Config.BaseURL, err)
	} else {
		_ = resp.Body.Close()
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Check for untranslated keys on Groups page", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)
		err = browser.NavigateTo("/admin")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)
		dashboardText, err := browser.Page.Locator("body").InnerText()
		require.NoError(t, err)
		untranslated := findTranslationKeys(dashboardText)
		if len(untranslated) > 0 {
			t.Errorf("Found untranslated keys on admin dashboard: %v", untranslated)
		}
		err = browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)
		groupsText, err := browser.Page.Locator("body").InnerText()
		require.NoError(t, err)
		groupsUn := findTranslationKeys(groupsText)
		if len(groupsUn) > 0 {
			t.Errorf("Found untranslated keys on groups page: %v", groupsUn)
		}
		checkElementsForTranslationKeys(t, browser)
	})

	t.Run("Check Groups modal for untranslated keys", func(t *testing.T) {
		if !auth.IsLoggedIn() {
			err := auth.LoginAsAdmin()
			require.NoError(t, err)
		}
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)
		addButton := browser.Page.Locator("button").Filter(playwright.LocatorFilterOptions{HasText: "Add Group"})
		buttonText, _ := addButton.InnerText()
		if strings.Contains(buttonText, "admin.") || strings.Contains(buttonText, "app.") {
			t.Errorf("Add button has untranslated key: %s", buttonText)
		}
		err = addButton.Click()
		assert.NoError(t, err)
		time.Sleep(1 * time.Second)
		modal := browser.Page.Locator("#groupModal")
		modalText, err := modal.InnerText()
		if err == nil {
			u := findTranslationKeys(modalText)
			if len(u) > 0 {
				t.Errorf("Found untranslated keys in modal: %v", u)
			}
		}
	})
}

func findTranslationKeys(text string) []string {
	var keys []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		words := strings.Fields(line)
		for _, w := range words {
			if isTranslationKey(w) {
				keys = append(keys, w)
			}
		}
	}
	return unique(keys)
}

func isTranslationKey(s string) bool {
	for _, suf := range []string{".", ",", ":", ";", "!", "?"} {
		s = strings.TrimSuffix(s, suf)
	}
	prefixes := []string{"admin.", "app.", "common.", "user.", "dashboard.", "tickets.", "queue.", "error.", "success.", "warning.", "info.", "time.", "status.", "priority."}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) && !strings.Contains(s, "/") && !strings.Contains(s, "http") {
			return true
		}
	}
	return false
}

func checkElementsForTranslationKeys(t *testing.T, browser *helpers.BrowserHelper) {
	title := browser.Page.Locator("h1")
	titleText, err := title.InnerText()
	if err == nil && isTranslationKey(titleText) {
		t.Errorf("Page title is untranslated: %s", titleText)
	}
	buttons := browser.Page.Locator("button")
	if c, _ := buttons.Count(); c > 0 {
		for i := 0; i < c; i++ {
			txt, err := buttons.Nth(i).InnerText()
			if err == nil && txt != "" && isTranslationKey(txt) {
				t.Errorf("Button %d has untranslated text: %s", i, txt)
			}
		}
	}
	links := browser.Page.Locator("a")
	if c, _ := links.Count(); c > 0 {
		for i := 0; i < c; i++ {
			txt, err := links.Nth(i).InnerText()
			if err == nil && txt != "" && isTranslationKey(txt) {
				t.Errorf("Link %d has untranslated text: %s", i, txt)
			}
		}
	}
	headers := browser.Page.Locator("th")
	if c, _ := headers.Count(); c > 0 {
		for i := 0; i < c; i++ {
			txt, err := headers.Nth(i).InnerText()
			if err == nil && txt != "" && isTranslationKey(txt) {
				t.Errorf("Table header %d has untranslated text: %s", i, txt)
			}
		}
	}
	labels := browser.Page.Locator("label")
	if c, _ := labels.Count(); c > 0 {
		for i := 0; i < c; i++ {
			txt, err := labels.Nth(i).InnerText()
			if err == nil && txt != "" && isTranslationKey(txt) {
				t.Errorf("Label %d has untranslated text: %s", i, txt)
			}
		}
	}
	all := browser.Page.Locator("*:visible")
	if c, _ := all.Count(); c > 0 {
		for i := 0; i < c && i < 100; i++ {
			txt, err := all.Nth(i).InnerText()
			if err == nil && txt != "" && strings.Count(txt, ".") > 0 && len(txt) < 50 {
				poss := findTranslationKeys(txt)
				if len(poss) > 0 {
					_, _ = all.Nth(i).Evaluate("el => el.tagName", nil)
				}
			}
		}
	}
}

func unique(slice []string) []string {
	m := map[string]bool{}
	out := []string{}
	for _, e := range slice {
		if !m[e] {
			m[e] = true
			out = append(out, e)
		}
	}
	return out
}
