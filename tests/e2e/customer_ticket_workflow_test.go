//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomerTicketWorkflowComplete covers access control, defaults and
// validation of the customer new-ticket form and a ticket with a chosen
// priority. Ticket number and initial article: TestCustomerTicketCreation.
func TestCustomerTicketWorkflowComplete(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	page := browser.Page
	newTicketURL := browser.Config.BaseURL + "/customer/tickets/new"

	t.Run("Agent session cannot open the customer ticket form", func(t *testing.T) {
		require.NoError(t, auth.LoginAsAdmin(), "Failed to login as admin")
		require.NoError(t, browser.NavigateTo("/customer/tickets/new"))
		assert.Equal(t, browser.Config.BaseURL+"/customer/login", page.URL(),
			"an agent session must be sent to the customer login")
		require.NoError(t, browser.Context.ClearCookies())
	})

	t.Run("Form offers the ticket fields with sensible defaults", func(t *testing.T) {
		require.NoError(t, auth.LoginAsCustomer(),
			"seeded customer %q (schema/seed/test_integration*.sql) must be able to log in", helpers.SeedCustomerLogin)
		require.NoError(t, browser.NavigateTo("/customer/tickets/new"))
		require.Equal(t, newTicketURL, page.URL(), "logged-in customer must reach the ticket creation form")

		required, err := page.Locator("#title").GetAttribute("required")
		require.NoError(t, err)
		assert.NotNil(t, required, "subject must be required")

		priority, err := page.Locator("#priority_id option:checked").TextContent()
		require.NoError(t, err)
		assert.Equal(t, "3 normal", strings.TrimSpace(priority), "priority defaults to 3 normal")

		// The message is a rich text editor that writes into the hidden
		// textarea[name=message] the form submits.
		require.NoError(t, page.Locator("#messageEditor .ProseMirror").WaitFor())
		messageFields, err := page.Locator("form[action='/customer/tickets/create'] textarea[name='message']").Count()
		require.NoError(t, err)
		assert.Equal(t, 1, messageFields)

		uploads, err := page.Locator("input[type='file'][name='attachments']").Count()
		require.NoError(t, err)
		assert.Equal(t, 1, uploads, "form accepts attachments")
	})

	t.Run("Missing subject blocks submission", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/customer/tickets/new"))
		require.NoError(t, page.Locator("form[action='/customer/tickets/create'] button[type='submit']").Click())

		missing, err := page.Locator("#title").Evaluate(`el => el.validity.valueMissing`, nil)
		require.NoError(t, err)
		assert.Equal(t, true, missing, "empty subject should fail browser validation")
		assert.Equal(t, newTicketURL, page.URL(), "the form must not be submitted")
	})

	t.Run("Ticket keeps the priority chosen on the form", func(t *testing.T) {
		title := fmt.Sprintf("Customer urgent outage %d", time.Now().UnixNano())
		createCustomerTicket(t, browser, title, "The whole office is offline.", "4 high")

		heading, err := page.Locator("h1.gk-heading").TextContent()
		require.NoError(t, err)
		assert.Equal(t, title, strings.TrimSpace(heading))
		assert.NoError(t, page.Locator("dd").Filter(playwright.LocatorFilterOptions{HasText: "4 high"}).WaitFor(),
			"ticket view should show the chosen priority")
	})
}
