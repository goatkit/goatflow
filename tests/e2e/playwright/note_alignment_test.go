//go:build e2e

package playwright

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ticketIdentifiers struct {
	ID        string
	TicketNum string
}

func TestAgentNoteBlockquoteAlignment(t *testing.T) {
	t.Skip("TODO: Update test to work with current ticket note implementation")
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	require.NoError(t, browser.Setup())
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	require.NoError(t, auth.Login(browser.Config.AdminEmail, browser.Config.AdminPassword))

	ids := createTicketForNoteTest(t, browser)
	require.NotEmpty(t, ids.TicketNum, "ticket number should not be empty")

	require.NoError(t, browser.NavigateTo("/ticket/"+ids.TicketNum))

	noteForm := browser.Page.Locator("#noteForm")
	require.NoError(t, noteForm.WaitFor())

	noteInput := browser.Page.Locator("#note_content")
	count, err := noteInput.Count()
	require.NoError(t, err)
	require.Greater(t, count, 0, "note content input missing")

	noteText := fmt.Sprintf("Alignment blockquote note %d", time.Now().UnixNano())
	noteHTML := fmt.Sprintf("<blockquote>%s</blockquote>", noteText)
	require.NoError(t, postNote(t, browser, ids.ID, noteHTML, true))

	_, err = browser.Page.Reload()
	require.NoError(t, err)
	require.NoError(t, browser.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateNetworkidle}))
	pageHTML, err := browser.Page.Content()
	require.NoError(t, err)
	require.Contains(t, pageHTML, noteText)
	noteContainer := browser.Page.Locator("div[id^='note-content-']").Filter(playwright.LocatorFilterOptions{HasText: noteText})
	containerCount, err := noteContainer.Count()
	require.NoError(t, err)
	if containerCount == 0 {
		idx := strings.Index(pageHTML, noteText)
		if idx > -1 {
			lo := idx - 500
			if lo < 0 {
				lo = 0
			}
			hi := idx + 500
			if hi > len(pageHTML) {
				hi = len(pageHTML)
			}
			t.Logf("note snippet: %s", pageHTML[lo:hi])
		} else {
			t.Log("note text not found in html content")
		}
		require.FailNow(t, "note container not found in DOM")
	}
	require.Contains(t, pageHTML, "note-content-", "notes section missing note containers")
	innerHTML, err := noteContainer.First().InnerHTML()
	require.NoError(t, err)
	t.Logf("note innerHTML: %s", innerHTML)
	blockQuote := noteContainer.Locator("blockquote").Filter(playwright.LocatorFilterOptions{HasText: noteText})
	bqCount, err := blockQuote.Count()
	require.NoError(t, err)
	require.Greater(t, bqCount, 0, "blockquote element not rendered")
	marginLeftRaw, err := blockQuote.First().Evaluate(`node => window.getComputedStyle(node).marginLeft`, nil)
	require.NoError(t, err)
	paddingLeftRaw, err := blockQuote.First().Evaluate(`node => window.getComputedStyle(node).paddingLeft`, nil)
	require.NoError(t, err)

	marginLeft := cssPixels(t, marginLeftRaw)
	paddingLeft := cssPixels(t, paddingLeftRaw)

	assert.Greater(t, marginLeft, 0.0, "blockquote margin-left should preserve indent")
	assert.Greater(t, paddingLeft, 0.0, "blockquote padding-left should preserve indent")
}

func createTicketForNoteTest(t *testing.T, browser *helpers.BrowserHelper) ticketIdentifiers {
	t.Helper()

	for attempt := 0; attempt < 3; attempt++ {
		ids, err := attemptTicketCreation(browser)
		if err == nil {
			require.NotEmpty(t, ids.ID, "ticket id missing")
			require.NotEmpty(t, ids.TicketNum, "ticket number missing")
			return ids
		}
		if errors.Is(err, errDuplicateTicket) {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		require.NoError(t, err)
	}

	t.Fatalf("ticket creation failed after retries")
	return ticketIdentifiers{}
}

func attemptTicketCreation(browser *helpers.BrowserHelper) (ticketIdentifiers, error) {
	payload := newTicketPayload()

	result, err := browser.Page.Evaluate(createTicketScript, payload)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ticketIdentifiers{}, errDuplicateTicket
		}
		return ticketIdentifiers{}, err
	}

	return parseTicketIdentifiers(result)
}

func parseTicketIdentifiers(result interface{}) (ticketIdentifiers, error) {
	resMap, ok := result.(map[string]interface{})
	if !ok {
		return ticketIdentifiers{}, fmt.Errorf("unexpected response type %T", result)
	}

	id, _ := resMap["id"].(string)
	ticketNumber, _ := resMap["ticketNumber"].(string)
	if id == "" || ticketNumber == "" {
		return ticketIdentifiers{}, fmt.Errorf("ticket identifiers missing")
	}

	return ticketIdentifiers{
		ID:        id,
		TicketNum: ticketNumber,
	}, nil
}

func cssPixels(t *testing.T, raw interface{}) float64 {
	str, ok := raw.(string)
	require.True(t, ok, "expected CSS pixel string, got %T", raw)
	val := strings.TrimSpace(str)
	val = strings.TrimSuffix(val, "px")
	if val == "" {
		return 0
	}
	num, err := strconv.ParseFloat(val, 64)
	require.NoError(t, err, "invalid CSS pixel value %q", str)
	return num
}

func newTicketPayload() map[string]interface{} {
	timestamp := time.Now().UnixNano()
	return map[string]interface{}{
		"subject":       fmt.Sprintf("Alignment Ticket %d", timestamp),
		"body":          "<p>Initial content for note alignment test</p>",
		"queueID":       1,
		"typeID":        1,
		"customerEmail": fmt.Sprintf("note-test-%d@example.com", timestamp),
	}
}

var errDuplicateTicket = errors.New("duplicate ticket")

func postNote(t *testing.T, browser *helpers.BrowserHelper, ticketID, html string, visible bool) error {
	resp, err := browser.Page.Evaluate(`async ({ticketId, html, visible}) => {
		const formData = new FormData();
		formData.append('body', html);
		formData.append('communication_channel_id', '3');
		if (visible) {
			formData.append('is_visible_for_customer', '1');
		}
		const response = await fetch('/agent/tickets/' + ticketId + '/note', {
			method: 'POST',
			headers: {
				'Accept': 'application/json',
				'X-Test-Mode': 'true'
			},
			body: formData
		});
		if (!response.ok) {
			const text = await response.text();
			throw new Error('create note failed: ' + response.status + ' ' + text);
		}
		return true;
	}`, map[string]interface{}{"ticketId": ticketID, "html": html, "visible": visible})
	if err != nil {
		return err
	}
	if respBool, ok := resp.(bool); !ok || !respBool {
		return fmt.Errorf("note creation returned unexpected result: %v", resp)
	}
	return nil
}

const createTicketScript = `async ({subject, body, queueID, typeID, customerEmail}) => {
	const requestBody = {
		subject,
		title: subject,
		queue_id: queueID,
		priority_id: 3,
		state_id: 1,
		body,
		customer_email: customerEmail,
		type_id: typeID
	};
	const response = await fetch('/api/tickets', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			'Accept': 'application/json',
			'X-Test-Mode': 'true'
		},
		body: JSON.stringify(requestBody)
	});
	if (!response.ok) {
		const text = await response.text();
		throw new Error('create ticket failed: ' + response.status + ' ' + text);
	}
	const payload = await response.json();
	const envelope = payload && typeof payload === 'object' ? payload : {};
	const data = typeof envelope === 'object' ? (envelope.data ? envelope.data : envelope) : {};
	const pickValue = (source, keys) => {
		if (!source || typeof source !== 'object') {
			return undefined;
		}
		for (const key of keys) {
			const value = source[key];
			if (value !== undefined && value !== null && value !== '') {
				return value;
			}
		}
		return undefined;
	};
	const idSource = pickValue(data, ['id']);
	const idFallback = pickValue(envelope, ['id']);
	const idValue = idSource != null ? idSource : idFallback;
	const id = idValue != null ? String(idValue) : '';
	const ticketCandidate = pickValue(data, ['tn', 'ticket_number', 'ticketNumber', 'ticket_id', 'ticketId']);
	const ticketFallback = pickValue(envelope, ['tn', 'ticket_number', 'ticketNumber']);
	const ticketNumberValue = ticketCandidate != null ? ticketCandidate : ticketFallback;
	const ticketNumber = ticketNumberValue != null ? String(ticketNumberValue) : '';
	return { id, ticketNumber };
}`
