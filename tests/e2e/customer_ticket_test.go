//go:build e2e

package e2e

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomerTicketCreation creates a ticket through the customer portal
// form and checks the ticket, its number and its first article.
func TestCustomerTicketCreation(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	defer browser.TearDown()
	page := browser.Page

	t.Run("Unauthenticated visitor is sent to the customer login", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/customer/tickets/new"))
		assert.Equal(t, browser.Config.BaseURL+"/customer/login", page.URL())
	})

	t.Run("Customer creates a ticket through the form", func(t *testing.T) {
		require.NoError(t, helpers.NewAuthHelper(browser).LoginAsCustomer(),
			"seeded customer %q (schema/seed/test_integration*.sql) must be able to log in", helpers.SeedCustomerLogin)

		title := fmt.Sprintf("Customer printer issue %d", time.Now().UnixNano())
		message := "The printer on floor 2 shows error E42."
		ticketURL, tn := createCustomerTicket(t, browser, title, message, "")

		// Ticket view: title, ticket number from the configured generator
		// (DateChecksum on the test stack) and the message as the first,
		// customer-authored article.
		heading, err := page.Locator("h1.gk-heading").TextContent()
		require.NoError(t, err)
		assert.Equal(t, title, strings.TrimSpace(heading))
		assertDateChecksumTN(t, tn)

		articles := page.Locator("[data-article-body]")
		count, err := articles.Count()
		require.NoError(t, err)
		require.Equal(t, 1, count, "a new ticket has exactly its initial article")
		body, err := articles.First().TextContent()
		require.NoError(t, err)
		assert.Contains(t, body, message)
		assert.NoError(t, page.GetByText("(Customer)").First().WaitFor(), "initial article should be authored by the customer")

		// My Tickets lists the new ticket (newest first) under the same number.
		require.NoError(t, browser.NavigateTo("/customer/tickets"))
		link := page.Locator(fmt.Sprintf("a[href='%s']", strings.TrimPrefix(ticketURL, browser.Config.BaseURL)))
		require.NoError(t, link.Filter(playwright.LocatorFilterOptions{HasText: title}).WaitFor())
		assert.NoError(t, link.Filter(playwright.LocatorFilterOptions{HasText: "#" + tn}).WaitFor())
	})
}

var customerTicketURL = regexp.MustCompile(`/customer/tickets/\d+$`)

// createCustomerTicket fills and submits the customer new-ticket form (title,
// rich text message and, when priority is not empty, the priority by label)
// as the signed-in customer and returns the ticket view URL and the ticket
// number it shows.
func createCustomerTicket(t *testing.T, browser *helpers.BrowserHelper, title, message, priority string) (string, string) {
	t.Helper()
	page := browser.Page
	require.NoError(t, browser.NavigateTo("/customer/tickets/new"))
	require.Equal(t, browser.Config.BaseURL+"/customer/tickets/new", page.URL(),
		"logged-in customer must reach the ticket creation form")

	require.NoError(t, page.Locator("#title").Fill(title))
	if priority != "" {
		_, err := page.Locator("#priority_id").SelectOption(playwright.SelectOptionValues{Labels: &[]string{priority}})
		require.NoError(t, err)
	}
	editor := page.Locator("#messageEditor .ProseMirror")
	require.NoError(t, editor.Click())
	require.NoError(t, editor.PressSequentially(message))
	require.NoError(t, page.Locator("form[action='/customer/tickets/create'] button[type='submit']").Click())

	require.NoError(t, page.WaitForURL(customerTicketURL), "creating a ticket should open its ticket view")
	tnText, err := page.Locator("dd.font-mono").First().TextContent()
	require.NoError(t, err)
	tn := strings.TrimPrefix(strings.TrimSpace(tnText), "#")
	return page.URL(), tn
}

// assertDateChecksumTN checks a DateChecksum ticket number: yyyymmdd (the
// creation date) + SystemID + counter + check digit (multipliers 1,2,1,2...).
func assertDateChecksumTN(t *testing.T, tn string) {
	t.Helper()
	require.Regexp(t, `^\d{16,}$`, tn, "ticket number should come from the DateChecksum generator")
	now := time.Now().UTC()
	dates := []string{now.AddDate(0, 0, -1).Format("20060102"), now.Format("20060102"), now.AddDate(0, 0, 1).Format("20060102")}
	assert.Contains(t, dates, tn[:8], "ticket number should start with the creation date")

	sum := 0
	for i, d := range tn[:len(tn)-1] {
		sum += (i%2 + 1) * int(d-'0')
	}
	check := 10 - sum%10
	if check == 10 {
		check = 1
	}
	assert.Equal(t, fmt.Sprint(check), tn[len(tn)-1:], "ticket number check digit")
}
