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

// TestQueueManagement covers the agent queue list (/queues) and the admin
// queue management page (/admin/queues): create, edit, disable, re-enable and
// filter. Queues are never hard-deleted (OTRS semantics); invalidating is the
// removal path, so the test queue is invalidated on exit.
func TestQueueManagement(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	t.Cleanup(browser.TearDown)
	require.NoError(t, helpers.NewAuthHelper(browser).LoginAsAdmin(), "Failed to login as admin")
	page := browser.Page

	// Confirm the status-toggle confirm() dialog.
	page.OnDialog(func(d playwright.Dialog) { _ = d.Accept() })

	visible := playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible, Timeout: playwright.Float(10000)}
	attached := playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateAttached, Timeout: playwright.Float(10000)}
	row := func(name string) playwright.Locator {
		return page.Locator(fmt.Sprintf("tr.queue-row[data-name='%s']", strings.ToLower(name)))
	}
	rowWithStatus := func(name string, validID int) playwright.Locator {
		return page.Locator(fmt.Sprintf("tr.queue-row[data-name='%s'][data-status='%d']", strings.ToLower(name), validID))
	}
	openAdminQueues := func(t *testing.T) {
		t.Helper()
		require.NoError(t, browser.NavigateTo("/admin/queues"))
		require.NoError(t, browser.WaitForLoad())
	}
	// saveQueueForm submits the queue modal and waits for the API response and
	// the page reload that follows a successful save (the marker set on the
	// old document disappears with it).
	saveQueueForm := func(t *testing.T, apiURL string, wantStatus int) {
		t.Helper()
		_, err := page.Evaluate(`() => { window.__e2eBeforeSave = true }`)
		require.NoError(t, err)
		resp, err := page.ExpectResponse(apiURL, func() error {
			return page.Locator("#queueForm button[type='submit']").Click()
		})
		require.NoError(t, err)
		require.Equal(t, wantStatus, resp.Status())
		_, err = page.WaitForFunction(`() => window.__e2eBeforeSave === undefined && document.readyState === 'complete'`, nil,
			playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(10000)})
		require.NoError(t, err, "admin queues page should reload after saving")
	}

	// Shared across the sequential subtests below.
	queueName := fmt.Sprintf("E2EQueue_%d", time.Now().UnixNano())
	renamed := queueName + "_Updated"
	queueID := ""
	// Registered after browser.TearDown, so it runs first.
	t.Cleanup(func() {
		if queueID == "" {
			return
		}
		resp, err := page.Request().Delete(browser.Config.BaseURL + "/api/v1/queues/" + queueID)
		if assert.NoError(t, err) {
			assert.Equal(t, http.StatusOK, resp.Status(), "invalidate test queue %s", queueID)
		}
	})

	t.Run("Queue list page loads", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/queues"))
		require.NoError(t, browser.WaitForHTMX())

		text, err := page.Locator("h1").First().TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "Queue", "Page should have queue-related title")

		seeded := page.Locator("a[href='/queues/5']", playwright.PageLocatorOptions{HasText: helpers.SeedQueueName})
		require.NoError(t, seeded.First().WaitFor(visible), "seeded queue %s should be listed", helpers.SeedQueueName)
	})

	t.Run("Create new queue", func(t *testing.T) {
		openAdminQueues(t)
		require.NoError(t, page.Locator("header button[onclick='openQueueModal()']").Click(), "Add Queue button")
		require.NoError(t, page.Locator("#queueModal #queueName").WaitFor(visible), "queue modal should open")

		require.NoError(t, page.Locator("#queueName").Fill(queueName))
		_, err := page.Locator("#queueGroup").SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		require.NoError(t, err, "group 'users' (id 1) must be selectable")
		require.NoError(t, page.Locator("#queueComments").Fill("Created by E2E test"))

		// Save reloads the page; the new row is rendered server-side.
		saveQueueForm(t, browser.Config.BaseURL+"/api/v1/queues", http.StatusCreated)

		created := row(queueName)
		require.NoError(t, created.WaitFor(attached), "created queue %s should be listed", queueName)
		queueID, err = created.GetAttribute("data-id")
		require.NoError(t, err)
		require.NotEmpty(t, queueID)

		text, err := created.TextContent()
		require.NoError(t, err)
		assert.Contains(t, text, "Created by E2E test", "queue comment is shown under its name")
		assert.Contains(t, text, "users", "queue group is shown")
	})

	t.Run("Edit queue", func(t *testing.T) {
		require.NotEmpty(t, queueID, "queue creation failed; this flow is sequential")
		openAdminQueues(t)

		require.NoError(t, row(queueName).Locator("button[onclick^='editQueue']").Click())
		nameInput := page.Locator("#queueName")
		require.NoError(t, nameInput.WaitFor(visible), "edit modal should open")
		// editQueue fills the form from /api/queues/:id before showing it.
		_, err := page.WaitForFunction(`(id) => document.getElementById('queueId').value === id`, queueID,
			playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(10000)})
		require.NoError(t, err, "edit form should be loaded for queue %s", queueID)
		value, err := nameInput.InputValue()
		require.NoError(t, err)
		require.Equal(t, queueName, value, "edit form should be populated with the queue name")
		comments, err := page.Locator("#queueComments").InputValue()
		require.NoError(t, err)
		assert.Equal(t, "Created by E2E test", comments)

		require.NoError(t, nameInput.Fill(renamed))
		saveQueueForm(t, browser.Config.BaseURL+"/api/v1/queues/"+queueID, http.StatusOK)
		require.NoError(t, rowWithStatus(renamed, 1).WaitFor(attached), "renamed queue %s should be listed", renamed)
	})

	toggle := func(t *testing.T, from, to int) {
		t.Helper()
		require.NotEmpty(t, queueID, "queue creation failed; this flow is sequential")
		openAdminQueues(t)
		resp, err := page.ExpectResponse(browser.Config.BaseURL+"/api/queues/"+queueID+"/status", func() error {
			return rowWithStatus(renamed, from).Locator("button[onclick^='toggleQueueStatus']").Click()
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Status())
		// The page reloads after the toggle; inactive queues stay listed.
		require.NoError(t, rowWithStatus(renamed, to).WaitFor(attached), "queue %s should have valid_id %d after the toggle", renamed, to)
	}

	t.Run("Disable queue", func(t *testing.T) {
		toggle(t, 1, 2)
		badge, err := rowWithStatus(renamed, 2).Locator(".gk-badge-muted").TextContent()
		require.NoError(t, err)
		assert.Contains(t, badge, "Inactive")
	})

	t.Run("Re-enable queue", func(t *testing.T) {
		toggle(t, 2, 1)
		badge, err := rowWithStatus(renamed, 1).Locator(".gk-badge-success").TextContent()
		require.NoError(t, err)
		assert.Contains(t, badge, "Active")
	})

	t.Run("Search queues", func(t *testing.T) {
		openAdminQueues(t)

		// filterQueues runs on keyup, so type rather than fill.
		search := page.Locator("#searchQueue")
		require.NoError(t, search.Fill(""))
		require.NoError(t, search.PressSequentially(strings.ToLower(helpers.SeedQueueName)))

		supportVisible, err := row(helpers.SeedQueueName).IsVisible()
		require.NoError(t, err)
		assert.True(t, supportVisible, "seeded queue %q should match the search", helpers.SeedQueueName)

		rawHidden, err := row("Raw").IsHidden()
		require.NoError(t, err)
		assert.True(t, rawHidden, "queue Raw should be filtered out")
	})
}
