//go:build e2e

package playwright

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminJSON calls an admin JSON endpoint with the browser session's cookies.
func adminJSON(t *testing.T, b *helpers.BrowserHelper, method, path string, body any) (int, map[string]any) {
	t.Helper()
	opts := playwright.APIRequestContextFetchOptions{
		Method:  playwright.String(method),
		Headers: map[string]string{"Accept": "application/json"},
	}
	if body != nil {
		opts.Data = body
	}
	resp, err := b.Page.Request().Fetch(b.Config.BaseURL+path, opts)
	require.NoError(t, err, "%s %s", method, path)
	defer resp.Dispose()
	var out map[string]any
	require.NoError(t, resp.JSON(&out), "%s %s should return JSON", method, path)
	return resp.Status(), out
}

// createRESTWebservice creates an HTTP::REST web service whose Search invoker
// sends GET <host>/health, and deletes it when the test ends.
func createRESTWebservice(t *testing.T, b *helpers.BrowserHelper, name, host string) {
	t.Helper()
	status, body := adminJSON(t, b, http.MethodPost, "/admin/api/webservices", map[string]any{
		"name": name,
		"config": map[string]any{
			"description": "e2e Test Connection",
			"requester": map[string]any{
				"invoker": map[string]any{"Search": map[string]any{"type": "Generic::PassThrough"}},
				"transport": map[string]any{
					"type": "HTTP::REST",
					"config": map[string]any{
						"host":    host,
						"timeout": "5",
						"invoker_controller_mapping": map[string]any{
							"Search": map[string]any{"controller": "/health", "command": "GET"},
						},
					},
				},
			},
		},
	})
	require.Equal(t, http.StatusOK, status, "create web service %s: %v", name, body)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "create web service response: %v", body)
	id := fmt.Sprint(data["id"])
	t.Cleanup(func() {
		status, body := adminJSON(t, b, http.MethodDelete, "/admin/api/webservices/"+id, nil)
		assert.Equal(t, http.StatusOK, status, "delete web service %s: %v", name, body)
	})
}

// createWebserviceField creates a WebserviceDropdown field through the admin
// form and returns its edit path; the field is deleted when the test ends.
func createWebserviceField(t *testing.T, b *helpers.BrowserHelper, name, webservice string) string {
	t.Helper()
	page := b.Page
	require.NoError(t, b.NavigateTo("/admin/dynamic-fields/new"))
	_, err := page.Locator("#field_type").SelectOption(playwright.SelectOptionValues{Values: &[]string{"WebserviceDropdown"}})
	require.NoError(t, err)
	require.NoError(t, page.Locator("input[name='name']").Fill(name))
	require.NoError(t, page.Locator("input[name='label']").Fill(name))
	_, err = page.Locator("#webservice").SelectOption(playwright.SelectOptionValues{Values: &[]string{webservice}})
	require.NoError(t, err, "the new web service should be offered in the Webservice select")
	require.NoError(t, page.Locator("#invoker_search").Fill("Search"))
	require.NoError(t, page.Locator("#stored_value").Fill("status"))
	require.NoError(t, page.Locator("#displayed_values").Fill("status"))
	require.NoError(t, page.Locator("form#dynamicFieldForm button[type='submit']").Click())
	require.NoError(t, page.WaitForURL("**/admin/dynamic-fields"), "create should return to the list")

	link := page.Locator(fmt.Sprintf("tr:has(td span:text-is('%s')) a[href^='/admin/dynamic-fields/']", name))
	href, err := link.First().GetAttribute("href")
	require.NoError(t, err, "the new field should be listed")
	require.Regexp(t, `^/admin/dynamic-fields/\d+$`, href)
	id := strings.TrimPrefix(href, "/admin/dynamic-fields/")
	t.Cleanup(func() {
		status, body := adminJSON(t, b, http.MethodDelete, "/admin/api/dynamic-fields/"+id, nil)
		assert.Equal(t, http.StatusOK, status, "delete field %s: %v", name, body)
	})
	return href
}

// TestDynamicFieldWebserviceTestConnection exercises the Test Configuration
// button of a WebserviceDropdown field against a reachable and an unreachable
// HTTP::REST host.
func TestDynamicFieldWebserviceTestConnection(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	// Registered first so it runs last, after the cleanups that use the session.
	t.Cleanup(browser.TearDown)
	require.NoError(t, helpers.NewAuthHelper(browser).LoginAsAdmin(), "Failed to login as admin")
	page := browser.Page

	suffix := fmt.Sprint(time.Now().UnixNano())
	okWS := "E2EWSOk" + suffix
	downWS := "E2EWSDown" + suffix
	// The backend under test calls itself: /health on its own port answers 200.
	createRESTWebservice(t, browser, okWS, "http://127.0.0.1:8080")
	// Nothing listens on port 1.
	createRESTWebservice(t, browser, downWS, "http://127.0.0.1:1")

	t.Run("unsaved field asks to save first", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/dynamic-fields/new"))
		_, err := page.Locator("#field_type").SelectOption(playwright.SelectOptionValues{Values: &[]string{"WebserviceDropdown"}})
		require.NoError(t, err)
		note := page.Locator("#webservice-test-unsaved")
		require.NoError(t, note.WaitFor(), "the save-first note should be shown for a new web service field")
		text, err := note.TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Save the field first to test its web service configuration.", strings.TrimSpace(text))
		assert.Equal(t, 0, count(t, page.Locator("#webservice-test-btn")), "an unsaved field has nothing to test")
	})

	runTest := func(t *testing.T, editPath string) (string, string) {
		t.Helper()
		require.NoError(t, browser.NavigateTo(editPath))
		result := page.Locator("#webservice-test-result")
		hidden, err := result.IsHidden()
		require.NoError(t, err)
		require.True(t, hidden, "no result before testing")

		resp, err := page.ExpectResponse(regexp.MustCompile(`/admin/api/dynamic-fields/\d+/webservice-test$`), func() error {
			return page.Locator("#webservice-test-btn").Click()
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.Status())
		// The button is re-enabled once the result is shown.
		require.NoError(t, page.Locator("#webservice-test-btn:enabled").WaitFor())
		text, err := result.TextContent()
		require.NoError(t, err)
		color, err := result.Evaluate("el => el.style.color", nil)
		require.NoError(t, err)
		return strings.TrimSpace(text), fmt.Sprint(color)
	}

	t.Run("reachable web service reports success", func(t *testing.T) {
		editPath := createWebserviceField(t, browser, "E2EWSFieldOk"+suffix, okWS)
		text, color := runTest(t, editPath)
		assert.Equal(t, "Configuration test successful", text)
		assert.Equal(t, "var(--gk-success)", color)
	})

	t.Run("unreachable web service reports the error", func(t *testing.T) {
		editPath := createWebserviceField(t, browser, "E2EWSFieldDown"+suffix, downWS)
		text, color := runTest(t, editPath)
		assert.True(t, strings.HasPrefix(text, "Configuration test failed: "), "result: %q", text)
		assert.Contains(t, text, "connection refused")
		assert.Equal(t, "var(--gk-error)", color)
	})
}
