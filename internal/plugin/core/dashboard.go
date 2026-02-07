// Package core provides core built-in plugins for GoatFlow.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/plugin"
)

// DashboardPlugin provides core dashboard widgets.
type DashboardPlugin struct {
	host plugin.HostAPI
}

// NewDashboardPlugin creates a new dashboard plugin instance.
func NewDashboardPlugin() *DashboardPlugin {
	return &DashboardPlugin{}
}

// GKRegister implements plugin.Plugin.
func (p *DashboardPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{
		Name:        "dashboard-core",
		Version:     "1.0.0",
		Description: "Core dashboard widgets for GoatFlow",
		Author:      "GoatFlow Team",
		License:     "Apache-2.0",
		Homepage:    "https://github.com/goatkit/goatflow",

		Widgets: []plugin.WidgetSpec{
			{
				ID:          "recent_tickets",
				Title:       "Recent Tickets",
				Description: "Shows recently updated tickets",
				Handler:     "widget_recent_tickets",
				Location:    "dashboard",
				Size:        "medium",
				Refreshable: true,
				RefreshSec:  30,
			},
			{
				ID:          "queue_status",
				Title:       "Queue Status",
				Description: "Shows ticket counts per queue",
				Handler:     "widget_queue_status",
				Location:    "dashboard",
				Size:        "medium",
				Refreshable: true,
				RefreshSec:  60,
			},
		},

		MinHostVersion: "0.7.0",
		Permissions:    []string{"db:read"},
	}
}

// Init implements plugin.Plugin.
func (p *DashboardPlugin) Init(ctx context.Context, host plugin.HostAPI) error {
	p.host = host
	p.host.Log(ctx, "info", "Dashboard core plugin initialized", map[string]any{
		"version": "1.0.0",
	})
	return nil
}

// Call implements plugin.Plugin.
func (p *DashboardPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	switch fn {
	case "widget_recent_tickets":
		return p.handleRecentTickets(ctx)
	case "widget_queue_status":
		return p.handleQueueStatus(ctx)
	default:
		return nil, fmt.Errorf("unknown function: %s", fn)
	}
}

// Shutdown implements plugin.Plugin.
func (p *DashboardPlugin) Shutdown(ctx context.Context) error {
	if p.host != nil {
		p.host.Log(ctx, "info", "Dashboard core plugin shutting down", nil)
	}
	return nil
}

// handleRecentTickets returns recent tickets widget HTML.
func (p *DashboardPlugin) handleRecentTickets(ctx context.Context) (json.RawMessage, error) {
	// Query recent tickets
	rows, err := p.host.DBQuery(ctx, `
		SELECT 
			t.tn as ticket_number,
			t.title,
			ts.name as status,
			tp.name as priority,
			t.customer_user_id,
			t.change_time
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
		ORDER BY t.change_time DESC
		LIMIT 5
	`)

	var html strings.Builder
	html.WriteString(`<ul role="list" class="-my-5 divide-y" style="border-color: var(--gk-border-default);">`)

	if err != nil || len(rows) == 0 {
		noTickets := p.host.Translate(ctx, "dashboard.no_recent_tickets")
		if noTickets == "" {
			noTickets = "No recent tickets"
		}
		html.WriteString(fmt.Sprintf(`
			<li class="py-4">
				<div class="flex items-center space-x-4">
					<div class="min-w-0 flex-1">
						<p class="truncate text-sm" style="color: var(--gk-text-muted);">%s</p>
					</div>
				</div>
			</li>`, noTickets))
	} else {
		for _, row := range rows {
			ticketNum := toString(row["ticket_number"])
			title := toString(row["title"])
			status := toString(row["status"])
			priority := toString(row["priority"])
			customer := toString(row["customer_user_id"])

			// Truncate title if too long
			if len(title) > 50 {
				title = title[:47] + "..."
			}

			// Priority badge style (matches static Recent Tickets)
			priorityStyle := "background: var(--gk-success-subtle); color: var(--gk-success);"
			switch strings.ToLower(priority) {
			case "1 very low":
				priorityStyle = "background: var(--gk-info-subtle); color: var(--gk-info);"
			case "2 low":
				priorityStyle = "background: var(--gk-info-subtle); color: var(--gk-info);"
			case "3 normal":
				priorityStyle = "background: var(--gk-success-subtle); color: var(--gk-success);"
			case "4 high":
				priorityStyle = "background: var(--gk-warning-subtle); color: var(--gk-warning);"
			case "5 very high":
				priorityStyle = "background: var(--gk-error-subtle); color: var(--gk-error);"
			}

			// Status badge style (matches static Recent Tickets)
			statusStyle := "background: var(--gk-primary-subtle); color: var(--gk-primary);"
			statusLower := strings.ToLower(status)
			switch {
			case statusLower == "new":
				statusStyle = "background: var(--gk-secondary-subtle); color: var(--gk-secondary);"
			case statusLower == "open":
				statusStyle = "background: var(--gk-success-subtle); color: var(--gk-success);"
			case strings.HasPrefix(statusLower, "pending"):
				statusStyle = "background: var(--gk-warning-subtle); color: var(--gk-warning);"
			case strings.HasPrefix(statusLower, "closed"), strings.HasPrefix(statusLower, "merged"), strings.HasPrefix(statusLower, "removed"):
				statusStyle = "background: var(--gk-bg-elevated); color: var(--gk-text-secondary);"
			}

			html.WriteString(fmt.Sprintf(`
				<li class="py-4 transition-all duration-200 hover:translate-x-1" style="border-color: var(--gk-border-default);">
					<div class="flex items-start space-x-4">
						<div class="min-w-0 flex-1">
							<a href="/tickets/%s" class="gk-link-neon text-sm font-medium">
								%s: %s
							</a>
							<div class="mt-2 flex flex-wrap gap-1">
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium" style="%s">
									%s
								</span>
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium" style="%s">
									%s
								</span>
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium" style="background: var(--gk-bg-elevated); color: var(--gk-text-secondary);">
									Customer: %s
								</span>
							</div>
						</div>
					</div>
				</li>`,
				ticketNum,
				escapeHTML(ticketNum), escapeHTML(title),
				priorityStyle, escapeHTML(priority),
				statusStyle, escapeHTML(status),
				escapeHTML(customer),
			))
		}
	}

	html.WriteString(`</ul>`)

	// View All link
	viewAll := p.host.Translate(ctx, "common.view_all")
	if viewAll == "" {
		viewAll = "View All"
	}
	html.WriteString(fmt.Sprintf(`
		<div class="mt-6">
			<a href="/tickets" class="gk-link-neon flex w-full items-center justify-center rounded-md px-3 py-2 text-sm font-semibold" style="border: 1px solid var(--gk-border-default);">
				%s
			</a>
		</div>`, escapeHTML(viewAll)))

	return json.Marshal(map[string]string{
		"html": html.String(),
	})
}

// handleQueueStatus returns queue status widget HTML.
func (p *DashboardPlugin) handleQueueStatus(ctx context.Context) (json.RawMessage, error) {
	// Query queue statistics with full status breakdown
	// Uses ticket_state_type to properly categorize states regardless of state IDs
	rows, err := p.host.DBQuery(ctx, `
		SELECT 
			q.name as queue_name,
			COUNT(t.id) as total,
			SUM(CASE WHEN tst.name = 'new' THEN 1 ELSE 0 END) as new_count,
			SUM(CASE WHEN tst.name = 'open' THEN 1 ELSE 0 END) as open_count,
			SUM(CASE WHEN tst.name LIKE 'pending%' THEN 1 ELSE 0 END) as pending_count,
			SUM(CASE WHEN tst.name = 'closed' THEN 1 ELSE 0 END) as closed_count
		FROM queue q
		LEFT JOIN ticket t ON t.queue_id = q.id
		LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
		LEFT JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE q.valid_id = 1
		GROUP BY q.id, q.name
		ORDER BY q.name
		LIMIT 10
	`)

	var html strings.Builder
	
	if err != nil || len(rows) == 0 {
		noQueues := p.host.Translate(ctx, "dashboard.no_queues_assigned")
		if noQueues == "" {
			noQueues = "No queues available"
		}
		html.WriteString(fmt.Sprintf(`<p class="text-sm" style="color: var(--gk-text-muted);">%s</p>`, noQueues))
	} else {
		// Table header
		html.WriteString(`<div class="overflow-x-auto">
			<table class="gk-table w-full text-sm">
				<thead>
					<tr>
						<th class="text-left py-2 px-3" style="color: var(--gk-text-secondary);">Queue</th>
						<th class="text-center py-2 px-2" style="color: var(--gk-text-secondary);">New</th>
						<th class="text-center py-2 px-2" style="color: var(--gk-text-secondary);">Open</th>
						<th class="text-center py-2 px-2" style="color: var(--gk-text-secondary);">Pending</th>
						<th class="text-center py-2 px-2" style="color: var(--gk-text-secondary);">Closed</th>
						<th class="text-center py-2 px-2" style="color: var(--gk-text-secondary);">Total</th>
					</tr>
				</thead>
				<tbody>`)

		for _, row := range rows {
			queueName := toString(row["queue_name"])
			total := toInt(row["total"])
			newCount := toInt(row["new_count"])
			openCount := toInt(row["open_count"])
			pendingCount := toInt(row["pending_count"])
			closedCount := toInt(row["closed_count"])

			// Truncate long queue names
			displayName := queueName
			if len(displayName) > 35 {
				displayName = displayName[:32] + "..."
			}

			html.WriteString(fmt.Sprintf(`
				<tr>
					<td class="py-2 px-3 truncate" style="color: var(--gk-text-primary); max-width: 200px;" title="%s">%s</td>
					<td class="py-2 px-2 text-center">%s</td>
					<td class="py-2 px-2 text-center">%s</td>
					<td class="py-2 px-2 text-center">%s</td>
					<td class="py-2 px-2 text-center">%s</td>
					<td class="py-2 px-2 text-center font-semibold" style="color: var(--gk-text-primary);">%d</td>
				</tr>`,
				escapeHTML(queueName), escapeHTML(displayName),
				statusBadge(newCount, "new"),
				statusBadge(openCount, "open"),
				statusBadge(pendingCount, "pending"),
				statusBadge(closedCount, "closed"),
				total,
			))
		}

		html.WriteString(`</tbody></table></div>`)
	}

	return json.Marshal(map[string]string{
		"html": html.String(),
	})
}

// statusBadge returns HTML for a colored badge with the count.
// Always shows a badge (even for zero) to match the built-in widget style.
func statusBadge(count int, statusType string) string {
	var bgColor, textColor string
	switch statusType {
	case "new":
		bgColor = "var(--gk-info-subtle)"
		textColor = "var(--gk-info)"
	case "open":
		bgColor = "var(--gk-success-subtle)"
		textColor = "var(--gk-success)"
	case "pending":
		bgColor = "var(--gk-warning-subtle)"
		textColor = "var(--gk-warning)"
	case "closed":
		bgColor = "var(--gk-bg-elevated)"
		textColor = "var(--gk-text-muted)"
	default:
		bgColor = "var(--gk-bg-elevated)"
		textColor = "var(--gk-text-muted)"
	}
	
	return fmt.Sprintf(`<span class="inline-flex items-center justify-center px-2 py-0.5 rounded-full text-xs font-medium" style="background: %s; color: %s; min-width: 2rem;">%d</span>`,
		bgColor, textColor, count)
}

// Helper functions

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case []byte:
		// MariaDB/MySQL drivers often return numeric values as []byte
		i, _ := strconv.Atoi(string(val))
		return i
	case string:
		i, _ := strconv.Atoi(val)
		return i
	default:
		return 0
	}
}

func toTime(v any) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch val := v.(type) {
	case time.Time:
		return val
	case string:
		t, _ := time.Parse(time.RFC3339, val)
		return t
	default:
		return time.Time{}
	}
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
