//go:build e2e

package playwright

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTicketSearchFiltersResults(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	require.NoError(t, browser.Setup())
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	require.NoError(t, auth.Login(browser.Config.AdminEmail, browser.Config.AdminPassword))

	subjectA := fmt.Sprintf("Search Regression Alpha %d", time.Now().UnixNano())
	subjectB := fmt.Sprintf("Search Regression Bravo %d", time.Now().UnixNano())

	idA := createTicketWithSubject(t, browser, subjectA)
	createTicketWithSubject(t, browser, subjectB)

	_, err := browser.Page.Goto(browser.Config.BaseURL+"/tickets", playwright.PageGotoOptions{
		Timeout:   playwright.Float(60000),
		WaitUntil: playwright.WaitUntilStateCommit,
	})
	require.NoError(t, err)
	currentURL := browser.Page.URL()
	t.Logf("landed on %s", currentURL)

	rowsInitialA, err := browser.Page.Locator("tbody tr").Filter(playwright.LocatorFilterOptions{HasText: subjectA}).Count()
	require.NoError(t, err)
	rowsInitialB, err := browser.Page.Locator("tbody tr").Filter(playwright.LocatorFilterOptions{HasText: subjectB}).Count()
	require.NoError(t, err)
	require.Greater(t, rowsInitialA, 0, "precondition failed: ticket %s missing before search", subjectA)
	require.Greater(t, rowsInitialB, 0, "precondition failed: ticket %s missing before search", subjectB)

	searchInput := browser.Page.Locator("#search-input")
	count, err := searchInput.Count()
	require.NoError(t, err)
	if count == 0 {
		html, contentErr := browser.Page.Content()
		if contentErr == nil {
			lower := strings.ToLower(html)
			if idx := strings.Index(lower, "<body"); idx >= 0 {
				body := strings.TrimSpace(html[idx:])
				t.Fatalf("search input missing; body snippet: %.4000s", body)
			}
			t.Fatalf("search input missing; page content snippet: %.3200s", strings.TrimSpace(html))
		}
		t.Fatalf("search input missing and content unavailable: %v", contentErr)
	}
	require.NoError(t, searchInput.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}))
	require.NoError(t, searchInput.Fill(subjectB))

	searchBtn := browser.Page.Locator("#search-btn")
	visible, err := searchBtn.IsVisible()
	require.NoError(t, err)
	require.True(t, visible, "search button not visible")
	enabled, err := searchBtn.IsEnabled()
	require.NoError(t, err)
	require.True(t, enabled, "search button disabled")
	escaped := url.QueryEscape(subjectB)
	require.NoError(t, searchBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)}))

	require.Eventually(t, func() bool {
		current := browser.Page.URL()
		if !strings.Contains(current, "search="+escaped) {
			t.Logf("waiting for search param, current URL %s", current)
			return false
		}
		return true
	}, 30*time.Second, 500*time.Millisecond, "search URL should update with query")

	rowsMatchingB, err := browser.Page.Locator("tbody tr").Filter(playwright.LocatorFilterOptions{HasText: subjectB}).Count()
	require.NoError(t, err)
	if rowsMatchingB == 0 {
		table := browser.Page.Locator("tbody")
		if tableHTML, contentErr := table.InnerHTML(); contentErr == nil {
			trimmed := tableHTML
			if len(trimmed) > 1200 {
				trimmed = trimmed[:1200] + "…"
			}
			t.Logf("tbody after search HTML:\n%s", trimmed)
		} else if pageHTML, pageErr := browser.Page.Content(); pageErr == nil {
			trimmed := pageHTML
			if len(trimmed) > 1600 {
				trimmed = trimmed[:1600] + "…"
			}
			t.Logf("page content fallback:\n%s", trimmed)
		} else {
			t.Logf("failed to capture table content: %v", contentErr)
		}
	}
	assert.Greater(t, rowsMatchingB, 0, "search results should contain matching ticket")

	rowsContainingA, err := browser.Page.Locator("tbody tr").Filter(playwright.LocatorFilterOptions{HasText: subjectA}).Count()
	require.NoError(t, err)
	assert.Equal(t, 0, rowsContainingA, "search results should exclude non-matching ticket %s (id %s)", subjectA, idA.TicketNum)
}

func createTicketWithSubject(t *testing.T, browser *helpers.BrowserHelper, subject string) ticketIdentifiers {
	t.Helper()
	payload := newTicketPayload()
	payload["subject"] = subject
	payload["body"] = fmt.Sprintf("<p>%s body</p>", subject)
	payload["customerEmail"] = fmt.Sprintf("search-%d@example.com", time.Now().UnixNano())
	result, err := browser.Page.Evaluate(createTicketScript, payload)
	require.NoError(t, err)
	info, err := parseTicketIdentifiers(result)
	require.NoError(t, err)
	return info
}
