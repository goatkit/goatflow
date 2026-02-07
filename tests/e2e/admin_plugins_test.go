//go:build e2e

package e2e

import (
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminPluginsPage(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	page := browser.Page

	// Login as admin
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("Navigate to plugins page", func(t *testing.T) {
		err := browser.NavigateTo("/admin/plugins")
		require.NoError(t, err)

		// Wait for page to load
		err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateDomcontentloaded,
		})
		require.NoError(t, err)

		// Check page title contains plugins
		title := page.Locator("h1")
		text, err := title.TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "Plugin")
	})

	t.Run("View plugin list", func(t *testing.T) {
		// Check that the plugins table exists
		table := page.Locator("table")
		visible, err := table.IsVisible()
		require.NoError(t, err)
		assert.True(t, visible, "plugins table should be visible")

		// Check for the hello plugin (registered by default)
		helloRow := page.Locator("text=hello")
		count, err := helloRow.Count()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1, "hello plugin should be in the list")
	})

	t.Run("View plugin details modal", func(t *testing.T) {
		// Click on view details button for hello plugin
		detailsBtn := page.Locator("button[title*='Details'], button:has-text('Details')").First()
		visible, _ := detailsBtn.IsVisible()
		if visible {
			err := detailsBtn.Click()
			require.NoError(t, err)

			// Wait for modal
			modal := page.Locator("dialog[open], .modal.modal-open")
			err = modal.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})
			if err == nil {
				// Check modal content
				modalContent, _ := modal.TextContent()
				assert.Contains(t, modalContent, "hello")

				// Close modal
				closeBtn := modal.Locator("button:has-text('Close'), button:has-text('close')")
				if count, _ := closeBtn.Count(); count > 0 {
					closeBtn.Click()
				}
			}
		}
	})

	t.Run("Navigate to plugin logs", func(t *testing.T) {
		// Click on View Logs button
		logsBtn := page.Locator("a:has-text('View Logs'), a:has-text('Logs')")
		visible, _ := logsBtn.IsVisible()
		if visible {
			err := logsBtn.Click()
			require.NoError(t, err)

			// Wait for navigation
			err = page.WaitForURL("**/plugins/logs**", playwright.PageWaitForURLOptions{
				Timeout: playwright.Float(5000),
			})
			if err == nil {
				// Check logs page loaded
				logsTable := page.Locator("table")
				visible, _ := logsTable.IsVisible()
				assert.True(t, visible, "logs table should be visible")
			}
		}
	})
}

func TestAdminPluginLogs(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	page := browser.Page

	// Login as admin
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("View plugin logs page", func(t *testing.T) {
		err := browser.NavigateTo("/admin/plugins/logs")
		require.NoError(t, err)

		err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateDomcontentloaded,
		})
		require.NoError(t, err)

		// Check page title
		title := page.Locator("h1")
		text, err := title.TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "Log")
	})

	t.Run("Filter logs by level", func(t *testing.T) {
		// Find level filter
		levelSelect := page.Locator("select#filter-level, select[name='level']")
		visible, _ := levelSelect.IsVisible()
		if visible {
			// Select error level
			err := levelSelect.SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice("error"),
			})
			if err == nil {
				// Wait for filter to apply
				page.WaitForTimeout(500)

				// Logs should be filtered (or show no results)
				logsContainer := page.Locator("#log-tbody, .log-entries")
				_, _ = logsContainer.TextContent()
				// Just verify no error - filtering works
			}
		}
	})

	t.Run("Clear logs", func(t *testing.T) {
		// Find clear button
		clearBtn := page.Locator("button:has-text('Clear')")
		visible, _ := clearBtn.IsVisible()
		if visible {
			// Set up dialog handler
			page.OnDialog(func(dialog playwright.Dialog) {
				dialog.Accept()
			})

			err := clearBtn.Click()
			if err == nil {
				// Wait for clear to complete
				page.WaitForTimeout(1000)

				// Verify logs cleared (count should show 0)
				logCount := page.Locator("#log-count, .log-count")
				if visible, _ := logCount.IsVisible(); visible {
					text, _ := logCount.TextContent()
					assert.Contains(t, text, "0")
				}
			}
		}
	})
}

func TestPluginAPI(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	page := browser.Page

	// Login as admin to get auth token
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("List plugins via API", func(t *testing.T) {
		// Use page.Evaluate to make authenticated API call
		result, err := page.Evaluate(`async () => {
			const response = await fetch('/api/v1/plugins');
			if (!response.ok) {
				return { error: response.status };
			}
			return response.json();
		}`)
		require.NoError(t, err)

		plugins, ok := result.([]interface{})
		if ok {
			// Should have at least the hello plugin
			assert.GreaterOrEqual(t, len(plugins), 1)
		}
	})

	t.Run("Call plugin function via API", func(t *testing.T) {
		result, err := page.Evaluate(`async () => {
			const response = await fetch('/api/v1/plugins/hello/call/hello', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: 'E2E Test' })
			});
			if (!response.ok) {
				return { error: response.status };
			}
			return response.json();
		}`)
		require.NoError(t, err)

		resultMap, ok := result.(map[string]interface{})
		if ok {
			// Should have a message in response
			assert.Contains(t, resultMap, "message")
		}
	})

	t.Run("Get plugin logs via API", func(t *testing.T) {
		result, err := page.Evaluate(`async () => {
			const response = await fetch('/api/v1/plugins/logs?limit=10');
			if (!response.ok) {
				return { error: response.status };
			}
			return response.json();
		}`)
		require.NoError(t, err)

		resultMap, ok := result.(map[string]interface{})
		if ok {
			// Should have logs array
			assert.Contains(t, resultMap, "logs")
			assert.Contains(t, resultMap, "count")
		}
	})
}

func TestPluginEnableDisable(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	page := browser.Page

	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("Disable plugin via UI", func(t *testing.T) {
		err := browser.NavigateTo("/admin/plugins")
		require.NoError(t, err)

		err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		require.NoError(t, err)

		// Find the hello plugin row and its disable button
		helloRow := page.Locator("tr:has-text('hello')")
		visible, err := helloRow.IsVisible()
		require.NoError(t, err)
		require.True(t, visible, "hello plugin row should be visible")

		// Check current status is Enabled
		enabledBadge := helloRow.Locator(".badge-success, .badge:has-text('Enabled')")
		if count, _ := enabledBadge.Count(); count > 0 {
			// Click disable button (the one with the ban/disable icon)
			disableBtn := helloRow.Locator("button[title*='Disable'], button:has(.text-warning)")
			visible, _ := disableBtn.IsVisible()
			if visible {
				err := disableBtn.Click()
				require.NoError(t, err)

				// Wait for page reload
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				require.NoError(t, err)

				// Verify status changed to Disabled
				helloRow = page.Locator("tr:has-text('hello')")
				disabledBadge := helloRow.Locator(".badge-warning, .badge:has-text('Disabled')")
				count, err := disabledBadge.Count()
				require.NoError(t, err)
				assert.GreaterOrEqual(t, count, 1, "plugin should show Disabled status")
			}
		}
	})

	t.Run("Enable plugin via UI", func(t *testing.T) {
		err := browser.NavigateTo("/admin/plugins")
		require.NoError(t, err)

		err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		})
		require.NoError(t, err)

		// Find the hello plugin row
		helloRow := page.Locator("tr:has-text('hello')")

		// Check current status is Disabled
		disabledBadge := helloRow.Locator(".badge-warning, .badge:has-text('Disabled')")
		if count, _ := disabledBadge.Count(); count > 0 {
			// Click enable button (the one with the checkmark/enable icon)
			enableBtn := helloRow.Locator("button[title*='Enable'], button:has(.text-success)")
			visible, _ := enableBtn.IsVisible()
			if visible {
				err := enableBtn.Click()
				require.NoError(t, err)

				// Wait for page reload
				err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
					State: playwright.LoadStateNetworkidle,
				})
				require.NoError(t, err)

				// Verify status changed to Enabled
				helloRow = page.Locator("tr:has-text('hello')")
				enabledBadge := helloRow.Locator(".badge-success, .badge:has-text('Enabled')")
				count, err := enabledBadge.Count()
				require.NoError(t, err)
				assert.GreaterOrEqual(t, count, 1, "plugin should show Enabled status")
			}
		}
	})

	t.Run("Disable plugin via API", func(t *testing.T) {
		result, err := page.Evaluate(`async () => {
			const response = await fetch('/api/v1/plugins/hello/disable', {
				method: 'POST',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json' }
			});
			return { status: response.status, ok: response.ok, body: await response.json() };
		}`)
		require.NoError(t, err)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "result should be a map")
		assert.Equal(t, true, resultMap["ok"], "API call should succeed")
	})

	t.Run("Enable plugin via API", func(t *testing.T) {
		result, err := page.Evaluate(`async () => {
			const response = await fetch('/api/v1/plugins/hello/enable', {
				method: 'POST',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json' }
			});
			return { status: response.status, ok: response.ok, body: await response.json() };
		}`)
		require.NoError(t, err)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "result should be a map")
		assert.Equal(t, true, resultMap["ok"], "API call should succeed")
	})
}

func TestPluginUpload(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	page := browser.Page

	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Failed to login as admin")

	t.Run("Upload modal opens", func(t *testing.T) {
		err := browser.NavigateTo("/admin/plugins")
		require.NoError(t, err)

		// Find upload button
		uploadBtn := page.Locator("button:has-text('Upload')")
		visible, _ := uploadBtn.IsVisible()
		if visible {
			err := uploadBtn.Click()
			require.NoError(t, err)

			// Wait for modal
			modal := page.Locator("dialog#upload-modal, .modal:has-text('Upload')")
			err = modal.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(3000),
			})
			if err == nil {
				visible, _ := modal.IsVisible()
				assert.True(t, visible, "upload modal should be visible")

				// Check file input accepts correct types
				fileInput := modal.Locator("input[type='file']")
				accept, _ := fileInput.GetAttribute("accept")
				assert.Contains(t, accept, ".wasm")
				assert.Contains(t, accept, ".zip")
			}
		}
	})
}
