//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// examplePlugin is the example WASM plugin shipped in config/plugins. It is
// disabled by default (plugin.defaultDisabledPlugins), so tests that need it
// running enable it and restore its previous state afterwards.
const (
	examplePlugin            = "hello-wasm"
	examplePluginDescription = "A simple hello world WASM plugin"
)

var pluginWait = playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible, Timeout: playwright.Float(10000)}

// pluginAPI calls a plugin admin endpoint with the browser session's cookies
// and decodes the JSON response body.
func pluginAPI(t *testing.T, b *helpers.BrowserHelper, method, path string, body any) (int, map[string]any) {
	t.Helper()
	opts := playwright.APIRequestContextFetchOptions{Method: playwright.String(method)}
	if body != nil {
		opts.Data = body
	}
	resp, err := b.Page.Request().Fetch(b.Config.BaseURL+path, opts)
	require.NoError(t, err, "%s %s", method, path)
	defer resp.Dispose()
	var out map[string]any
	require.NoError(t, resp.JSON(&out), "%s %s should return a JSON object", method, path)
	return resp.Status(), out
}

// pluginEnabled reports the enabled flag of a plugin from GET /api/v1/plugins.
func pluginEnabled(t *testing.T, b *helpers.BrowserHelper, name string) bool {
	t.Helper()
	status, body := pluginAPI(t, b, http.MethodGet, "/api/v1/plugins", nil)
	require.Equal(t, http.StatusOK, status, "plugin list: %v", body)
	plugins, ok := body["plugins"].([]any)
	require.True(t, ok, "plugin list should carry a plugins array: %v", body)
	for _, p := range plugins {
		if m, ok := p.(map[string]any); ok && m["name"] == name {
			enabled, ok := m["enabled"].(bool)
			require.True(t, ok, "plugin %s should report enabled as a bool: %v", name, m)
			return enabled
		}
	}
	require.Failf(t, "plugin not listed", "plugin %s missing from /api/v1/plugins", name)
	return false
}

// setPluginEnabled enables or disables a plugin through the admin API.
func setPluginEnabled(t *testing.T, b *helpers.BrowserHelper, name string, enabled bool) {
	t.Helper()
	action, want := "disable", "disabled"
	if enabled {
		action, want = "enable", "enabled"
	}
	status, body := pluginAPI(t, b, http.MethodPost, "/api/v1/plugins/"+name+"/"+action, nil)
	require.Equal(t, http.StatusOK, status, "%s %s: %v", action, name, body)
	require.Equal(t, want, body["status"])
}

// adminPluginBrowser starts a browser logged in as admin. Cleanups registered
// after this run before the browser is torn down.
func adminPluginBrowser(t *testing.T) *helpers.BrowserHelper {
	t.Helper()
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	t.Cleanup(browser.TearDown)
	require.NoError(t, helpers.NewAuthHelper(browser).LoginAsAdmin(), "Failed to login as admin")
	return browser
}

// keepPluginState restores the plugin's current enabled state when the test ends.
func keepPluginState(t *testing.T, b *helpers.BrowserHelper, name string) bool {
	t.Helper()
	initial := pluginEnabled(t, b, name)
	t.Cleanup(func() {
		if pluginEnabled(t, b, name) != initial {
			setPluginEnabled(t, b, name, initial)
		}
	})
	return initial
}

func TestAdminPluginsPage(t *testing.T) {
	browser := adminPluginBrowser(t)
	page := browser.Page
	exampleRow := page.Locator("#plugin-row-" + examplePlugin)

	t.Run("Navigate to plugins page", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/plugins"))
		text, err := page.Locator("h1").TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "Plugins")
		require.NoError(t, exampleRow.WaitFor(pluginWait), "plugin rows are rendered from /api/v1/plugins")
	})

	t.Run("View plugin list", func(t *testing.T) {
		status, body := pluginAPI(t, browser, http.MethodGet, "/api/v1/plugins", nil)
		require.Equal(t, http.StatusOK, status)
		plugins, ok := body["plugins"].([]any)
		require.True(t, ok, "plugins array: %v", body)

		rows := page.Locator("#plugin-tbody tr[id^='plugin-row-']")
		n, err := rows.Count()
		require.NoError(t, err)
		assert.Equal(t, len(plugins), n, "one table row per registered plugin")
		total, err := page.Locator("#stat-total").TextContent()
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprint(len(plugins)), total)
		for _, p := range plugins {
			name := p.(map[string]any)["name"].(string)
			visible, err := page.Locator("#plugin-row-" + name).IsVisible()
			require.NoError(t, err)
			assert.True(t, visible, "plugin %s should have a row", name)
		}

		rowText, err := exampleRow.TextContent()
		require.NoError(t, err)
		assert.Contains(t, rowText, examplePluginDescription)
		assert.Contains(t, rowText, "1.0.0")
	})

	t.Run("View plugin details modal", func(t *testing.T) {
		require.NoError(t, exampleRow.Locator("a", playwright.LocatorLocatorOptions{HasText: examplePlugin}).Click())
		modal := page.Locator("#plugin-details-modal")
		require.NoError(t, modal.WaitFor(pluginWait), "details modal should open")

		title, err := page.Locator("#modal-plugin-name").TextContent()
		require.NoError(t, err)
		assert.Equal(t, examplePlugin, title)
		content, err := page.Locator("#modal-plugin-content").TextContent()
		require.NoError(t, err)
		assert.Contains(t, content, examplePluginDescription)
		assert.Contains(t, content, "GoatFlow Team", "author")
		assert.Contains(t, content, "/api/plugins/hello-wasm", "declared route")
		assert.Contains(t, content, "Hello WASM", "declared widget")

		require.NoError(t, modal.Locator(".gk-modal-footer button.gk-btn-secondary").Click())
		require.NoError(t, modal.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateHidden, Timeout: playwright.Float(5000)}))
	})

	t.Run("Navigate to plugin logs", func(t *testing.T) {
		require.NoError(t, page.Locator("header a[href='/admin/plugins/logs']").Click())
		require.NoError(t, page.WaitForURL("**/admin/plugins/logs"))
		require.NoError(t, page.Locator("#log-tbody").WaitFor(pluginWait))
		text, err := page.Locator("h1").TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "Plugin Logs")
	})
}

func TestAdminPluginLogs(t *testing.T) {
	browser := adminPluginBrowser(t)
	page := browser.Page
	page.OnDialog(func(d playwright.Dialog) { _ = d.Accept() })

	// A failed enable of an unknown plugin logs an error entry under that
	// plugin's name, so the entry is unique to this run.
	missing := fmt.Sprintf("e2e-missing-%d", time.Now().UnixNano())
	status, body := pluginAPI(t, browser, http.MethodPost, "/api/v1/plugins/"+missing+"/enable", nil)
	require.Equal(t, http.StatusNotFound, status, "enable unknown plugin: %v", body)
	missingRow := page.Locator("#log-tbody tr", playwright.PageLocatorOptions{HasText: missing})

	t.Run("View plugin logs page", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/plugins/logs"))
		text, err := page.Locator("h1").TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "Plugin Logs")
		require.NoError(t, missingRow.WaitFor(pluginWait), "logged error for %s should be listed", missing)
		assert.Contains(t, mustText(t, missingRow), "Failed to enable plugin")
		assert.Contains(t, mustText(t, missingRow), "ERROR")
	})

	// selectLevel picks a level filter and waits until the table shows only
	// badges of that level (or the empty-state row).
	selectLevel := func(t *testing.T, value, badge string) {
		t.Helper()
		resp, err := page.ExpectResponse(func(u string) bool {
			return strings.Contains(u, "/api/v1/plugins/logs?") && strings.Contains(u, "level="+value) == (value != "")
		}, func() error {
			_, err := page.Locator("#filter-level").SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice(value)})
			return err
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Status())
		if badge == "" {
			return
		}
		_, err = page.WaitForFunction(`(badge) => [...document.querySelectorAll('#log-tbody .badge')].every(b => b.textContent.trim() === badge)`,
			badge, playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(10000)})
		require.NoError(t, err, "only %s entries should be listed", badge)
	}
	gone := playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateDetached, Timeout: playwright.Float(10000)}

	t.Run("Filter logs by level", func(t *testing.T) {
		selectLevel(t, "error", "ERROR")
		require.NoError(t, missingRow.WaitFor(pluginWait), "error entry is listed under the error filter")

		selectLevel(t, "info", "INFO")
		require.NoError(t, missingRow.WaitFor(gone), "error entry is filtered out under the info filter")
	})

	t.Run("Clear logs", func(t *testing.T) {
		selectLevel(t, "", "")
		require.NoError(t, missingRow.WaitFor(pluginWait), "entry is listed again without a level filter")

		resp, err := page.ExpectResponse(func(u string) bool {
			return u == browser.Config.BaseURL+"/api/v1/plugins/logs"
		}, func() error {
			return page.Locator("header button", playwright.PageLocatorOptions{HasText: "Clear Logs"}).Click()
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Status(), "DELETE /api/v1/plugins/logs")
		require.NoError(t, missingRow.WaitFor(gone), "cleared entry disappears from the table")

		status, body := pluginAPI(t, browser, http.MethodGet, "/api/v1/plugins/logs?plugin="+missing, nil)
		require.Equal(t, http.StatusOK, status)
		assert.EqualValues(t, 0, body["count"], "cleared buffer has no entries for %s: %v", missing, body)
	})
}

func mustText(t *testing.T, l playwright.Locator) string {
	t.Helper()
	text, err := l.TextContent()
	require.NoError(t, err)
	return text
}

func TestPluginAPI(t *testing.T) {
	browser := adminPluginBrowser(t)
	keepPluginState(t, browser, examplePlugin)

	t.Run("List plugins via API", func(t *testing.T) {
		status, body := pluginAPI(t, browser, http.MethodGet, "/api/v1/plugins", nil)
		require.Equal(t, http.StatusOK, status)
		plugins, ok := body["plugins"].([]any)
		require.True(t, ok, "plugins array: %v", body)
		var example map[string]any
		for _, p := range plugins {
			if m := p.(map[string]any); m["name"] == examplePlugin {
				example = m
			}
		}
		require.NotNil(t, example, "%s should be listed: %v", examplePlugin, body)
		assert.Equal(t, "1.0.0", example["version"])
		assert.Equal(t, examplePluginDescription, example["description"])
		assert.Equal(t, true, example["loaded"])
	})

	t.Run("Call plugin function via API", func(t *testing.T) {
		setPluginEnabled(t, browser, examplePlugin, true)
		status, body := pluginAPI(t, browser, http.MethodPost, "/api/v1/plugins/"+examplePlugin+"/call/hello", map[string]any{"name": "E2E Test"})
		require.Equal(t, http.StatusOK, status, "%v", body)
		assert.Equal(t, "Hello from WASM, E2E Test!", body["message"])
		assert.Equal(t, "tinygo-wasm", body["runtime"])
	})

	t.Run("Call on disabled plugin is refused", func(t *testing.T) {
		setPluginEnabled(t, browser, examplePlugin, false)
		status, body := pluginAPI(t, browser, http.MethodPost, "/api/v1/plugins/"+examplePlugin+"/call/hello", map[string]any{"name": "E2E Test"})
		assert.Equal(t, http.StatusForbidden, status)
		assert.Contains(t, body["error"], "disabled")
	})

	t.Run("Call on unknown plugin is 404", func(t *testing.T) {
		status, body := pluginAPI(t, browser, http.MethodPost, "/api/v1/plugins/e2e-no-such-plugin/call/hello", map[string]any{})
		assert.Equal(t, http.StatusNotFound, status)
		assert.Contains(t, body["error"], "not found")
	})

	t.Run("Get plugin logs via API", func(t *testing.T) {
		// The enable/disable above were logged under the plugin's name.
		status, body := pluginAPI(t, browser, http.MethodGet, "/api/v1/plugins/logs?plugin="+examplePlugin+"&level=info&limit=10", nil)
		require.Equal(t, http.StatusOK, status)
		logs, ok := body["logs"].([]any)
		require.True(t, ok, "logs array: %v", body)
		require.NotEmpty(t, logs)
		assert.LessOrEqual(t, len(logs), 10)
		assert.EqualValues(t, len(logs), body["count"])
		newest := logs[0].(map[string]any)
		assert.Equal(t, examplePlugin, newest["plugin"])
		assert.Equal(t, "info", newest["level"])
		assert.Equal(t, "Plugin disabled: "+examplePlugin, newest["message"])
	})
}

func TestPluginEnableDisable(t *testing.T) {
	browser := adminPluginBrowser(t)
	page := browser.Page
	if keepPluginState(t, browser, examplePlugin) {
		setPluginEnabled(t, browser, examplePlugin, false)
	}
	row := page.Locator("#plugin-row-" + examplePlugin)

	toggleViaUI := func(t *testing.T, enable bool) {
		t.Helper()
		action := "disable"
		if enable {
			action = "enable"
		}
		require.NoError(t, browser.NavigateTo("/admin/plugins"))
		require.NoError(t, row.WaitFor(pluginWait))
		resp, err := page.ExpectResponse("**/api/v1/plugins/"+examplePlugin+"/"+action, func() error {
			return row.Locator(fmt.Sprintf("button[onclick=\"togglePlugin('%s', %t)\"]", examplePlugin, enable)).Click()
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Status())
	}

	t.Run("Enable plugin via UI", func(t *testing.T) {
		toggleViaUI(t, true)
		require.NoError(t, row.Locator(".gk-badge-success", playwright.LocatorLocatorOptions{HasText: "Enabled"}).WaitFor(pluginWait))
		assert.True(t, pluginEnabled(t, browser, examplePlugin))
	})

	t.Run("Disable plugin via UI", func(t *testing.T) {
		toggleViaUI(t, false)
		require.NoError(t, row.Locator(".gk-badge-muted", playwright.LocatorLocatorOptions{HasText: "Disabled"}).WaitFor(pluginWait))
		assert.False(t, pluginEnabled(t, browser, examplePlugin))
	})

	t.Run("Enable plugin via API", func(t *testing.T) {
		setPluginEnabled(t, browser, examplePlugin, true)
		assert.True(t, pluginEnabled(t, browser, examplePlugin))
	})

	t.Run("Disable plugin via API", func(t *testing.T) {
		setPluginEnabled(t, browser, examplePlugin, false)
		assert.False(t, pluginEnabled(t, browser, examplePlugin))
	})

	t.Run("Enable unknown plugin is 404", func(t *testing.T) {
		status, body := pluginAPI(t, browser, http.MethodPost, "/api/v1/plugins/e2e-no-such-plugin/enable", nil)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Contains(t, body["error"], "not found")
	})
}

func TestPluginUpload(t *testing.T) {
	browser := adminPluginBrowser(t)
	page := browser.Page
	modal := page.Locator("#upload-modal")
	uploadError := page.Locator("#uploadError")

	t.Run("Upload modal opens", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/plugins"))
		require.NoError(t, page.Locator("header button[onclick='showUploadModal()']").Click())
		require.NoError(t, modal.WaitFor(pluginWait), "upload modal should open")

		accept, err := modal.Locator("input#plugin-file").GetAttribute("accept")
		require.NoError(t, err)
		assert.Equal(t, ".wasm,.zip", accept)
		hidden, err := uploadError.IsHidden()
		require.NoError(t, err)
		assert.True(t, hidden, "no error before an upload attempt")
	})

	t.Run("Upload without file is rejected client-side", func(t *testing.T) {
		require.NoError(t, page.Locator("#upload-btn").Click())
		require.NoError(t, uploadError.WaitFor(pluginWait))
		msg, err := page.Locator("#uploadErrorMessage").TextContent()
		require.NoError(t, err)
		assert.NotEmpty(t, msg)
	})

	t.Run("Upload of non-plugin file is rejected by the server", func(t *testing.T) {
		require.NoError(t, page.Locator("#plugin-file").SetInputFiles([]playwright.InputFile{{
			Name: "not-a-plugin.txt", MimeType: "text/plain", Buffer: []byte("plain text"),
		}}))
		resp, err := page.ExpectResponse("**/api/v1/plugins/upload", func() error {
			return page.Locator("#upload-btn").Click()
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.Status())
		msg := page.Locator("#uploadErrorMessage", playwright.PageLocatorOptions{HasText: "Only .wasm and .zip files are allowed"})
		require.NoError(t, msg.WaitFor(pluginWait))
		visible, err := modal.IsVisible()
		require.NoError(t, err)
		assert.True(t, visible, "modal stays open on a rejected upload")
	})
}
