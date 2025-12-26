//go:build db

package api

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/stretchr/testify/require"
)

func TestTicketDetailTemplatePendingReminderMessage(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	loader := pongo2.MustNewLocalFileSystemLoader(filepath.Join(baseDir, "..", "..", "templates"))
	set := pongo2.NewSet("ticket-detail-test", loader)
	tmpl, err := set.FromFile("pages/ticket_detail.pongo2")
	require.NoError(t, err)

	ticket := pongo2.Context{
		"id":                                 123,
		"tn":                                 "202510161000007",
		"subject":                            "Pending reminder ticket",
		"status":                             "pending reminder",
		"state_type":                         "pending",
		"auto_close_pending":                 false,
		"pending_reminder":                   true,
		"pending_reminder_has_time":          true,
		"pending_reminder_at":                "2025-10-18 13:30:00 UTC",
		"pending_reminder_overdue":           false,
		"pending_reminder_relative":          "2 hours",
		"is_closed":                          false,
		"priority":                           "normal",
		"priority_id":                        3,
		"queue":                              "Raw",
		"queue_id":                           1,
		"customer_name":                      "ACME",
		"customer_user_id":                   "",
		"customer_id":                        "ACME",
		"customer":                           pongo2.Context{"name": "ACME", "email": "", "phone": ""},
		"agent":                              nil,
		"assigned_to":                        "Unassigned",
		"owner":                              "Unassigned",
		"type":                               "Incident",
		"service":                            "-",
		"sla":                                "-",
		"created":                            "2025-10-16 09:00",
		"updated":                            "2025-10-16 10:00",
		"description":                        "Ticket body",
		"description_json":                   "\"Ticket body\"",
		"notes":                              []interface{}{},
		"note_bodies_json":                   "{}",
		"description_is_html":                false,
		"time_total_minutes":                 0,
		"time_total_hours":                   0,
		"time_total_remaining_minutes":       0,
		"time_total_has_hours":               false,
		"time_entries":                       []interface{}{},
		"time_by_article":                    map[string]int{},
		"first_article_id":                   nil,
		"first_article_visible_for_customer": false,
		"age":                                "1 day",
		"status_id":                          4,
	}

	ctx := pongo2.Context{
		"Ticket":               ticket,
		"PendingStateIDs":      []int{4, 5},
		"TicketStates":         []pongo2.Context{},
		"RequireNoteTimeUnits": false,
		"t": func(key string, _ ...interface{}) string {
			return key
		},
	}

	output, err := tmpl.Execute(ctx)
	require.NoError(t, err)

	require.Contains(t, output, "Reminder scheduled")
	require.Contains(t, output, "Ticket will re-open at <span class=\"font-semibold\">2025-10-18 13:30:00 UTC</span>")
}

func TestTicketDetailTemplateBlockquoteIndent(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	loader := pongo2.MustNewLocalFileSystemLoader(filepath.Join(baseDir, "..", "..", "templates"))
	set := pongo2.NewSet("ticket-detail-test", loader)
	tmpl, err := set.FromFile("pages/ticket_detail.pongo2")
	require.NoError(t, err)

	noteBody := `<blockquote>Indented note</blockquote>`

	ticket := pongo2.Context{
		"id":      1,
		"tn":      "123",
		"subject": "Test",
		"notes": []interface{}{
			pongo2.Context{
				"id":                      1,
				"author":                  "Agent",
				"time":                    "now",
				"body":                    noteBody,
				"has_html":                true,
				"is_visible_for_customer": false,
			},
		},
	}

	ctx := pongo2.Context{
		"Ticket":               ticket,
		"PendingStateIDs":      []int{},
		"TicketStates":         []pongo2.Context{},
		"RequireNoteTimeUnits": false,
		"t": func(key string, _ ...interface{}) string {
			return key
		},
	}

	output, err := tmpl.Execute(ctx)
	require.NoError(t, err)

	require.Contains(t, output, "div[id^=\"note-content-\"] blockquote")
	require.Contains(t, output, "margin: 1rem 0 1rem 1.5rem")
	require.Contains(t, output, "padding: 0.75rem 0 0.75rem 1.25rem")
}

func TestTicketDetailTemplatePlainNoteTrimmed(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	loader := pongo2.MustNewLocalFileSystemLoader(filepath.Join(baseDir, "..", "..", "templates"))
	set := pongo2.NewSet("ticket-detail-test", loader)
	tmpl, err := set.FromFile("pages/ticket_detail.pongo2")
	require.NoError(t, err)

	ticket := pongo2.Context{
		"id":      1,
		"tn":      "456",
		"subject": "Test",
		"notes": []interface{}{
			pongo2.Context{
				"id":                      1,
				"author":                  "Agent",
				"time":                    "now",
				"body":                    "Plain text note",
				"has_html":                false,
				"is_visible_for_customer": false,
			},
		},
	}

	ctx := pongo2.Context{
		"Ticket":               ticket,
		"PendingStateIDs":      []int{},
		"TicketStates":         []pongo2.Context{},
		"RequireNoteTimeUnits": false,
		"t": func(key string, _ ...interface{}) string {
			return key
		},
	}

	output, err := tmpl.Execute(ctx)
	require.NoError(t, err)

	require.Contains(t, output, `>Plain text note</div>`)
	require.NotContains(t, output, `> Plain text note`)
}
