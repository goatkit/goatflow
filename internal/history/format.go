// Package history provides ticket history formatting and display utilities.
package history

import (
	"fmt"
	"strings"

	"github.com/goatkit/goatflow/internal/models"
)

// NormalizeHistoryName converts the raw ticket_history.name field into a human readable label.
// OTRS stores structured payloads using double-percent (%%) delimiters; when those
// appear we decode the payload based on the reported history type and fall back to a
// reasonable string otherwise. This allows imported tickets to render meaningful events.
func NormalizeHistoryName(entry models.TicketHistoryEntry) string {
	raw := strings.TrimSpace(entry.Name)
	if raw == "" {
		return ""
	}

	if !strings.Contains(raw, "%%") {
		return raw
	}

	parts := splitLegacyPayload(raw)
	if len(parts) == 0 {
		return ""
	}

	typeName := strings.TrimSpace(entry.HistoryType)
	switch typeName {
	case "NewTicket":
		return formatNewTicket(entry, parts)
	case "EmailAgent":
		return formatEmailAgent(parts)
	case "SendAgentNotification":
		return formatAgentNotification(parts)
	default:
		return strings.Join(parts, " • ")
	}
}

func splitLegacyPayload(raw string) []string {
	tokens := strings.Split(raw, "%%")
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		token = strings.Trim(token, ", ")
		if token == "" {
			continue
		}
		parts = append(parts, token)
	}
	return parts
}

func formatNewTicket(entry models.TicketHistoryEntry, parts []string) string {
	var tn string
	if len(parts) > 0 {
		tn = parts[0]
	}

	queue := strings.TrimSpace(entry.QueueName)
	if queue == "" && len(parts) > 1 {
		queue = parts[1]
	}

	priority := strings.TrimSpace(entry.PriorityName)
	if priority == "" && len(parts) > 2 {
		priority = parts[2]
	}

	state := strings.TrimSpace(entry.StateName)
	if state == "" && len(parts) > 3 {
		state = parts[3]
	}

	builder := strings.Builder{}
	builder.WriteString("Ticket created")
	if tn != "" {
		builder.WriteString(fmt.Sprintf(" (#%s)", tn))
	}

	details := make([]string, 0, 3)
	if queue != "" {
		details = append(details, queue)
	}
	if state != "" {
		details = append(details, state)
	}
	if priority != "" {
		if !strings.HasPrefix(strings.ToLower(priority), "priority") {
			details = append(details, fmt.Sprintf("Priority %s", priority))
		} else {
			details = append(details, priority)
		}
	}

	if len(details) > 0 {
		builder.WriteString(" • ")
		builder.WriteString(strings.Join(details, " • "))
	}

	return builder.String()
}

func formatEmailAgent(parts []string) string {
	recipient := ""
	if len(parts) > 0 {
		recipient = strings.Trim(parts[0], ", ")
	}
	if recipient == "" && len(parts) > 1 {
		recipient = strings.Trim(parts[1], ", ")
	}

	if recipient == "" {
		return "Agent email sent"
	}
	return fmt.Sprintf("Agent email sent to %s", recipient)
}

func formatAgentNotification(parts []string) string {
	subject := ""
	if len(parts) > 0 {
		subject = parts[0]
	}
	target := ""
	if len(parts) > 1 {
		target = parts[1]
	}
	channel := ""
	if len(parts) > 2 {
		channel = parts[2]
	}

	builder := strings.Builder{}
	builder.WriteString("Agent notification")
	if subject != "" {
		builder.WriteString(fmt.Sprintf(" “%s”", subject))
	}

	details := make([]string, 0, 2)
	if target != "" {
		details = append(details, fmt.Sprintf("recipient %s", target))
	}
	if channel != "" {
		details = append(details, fmt.Sprintf("via %s", channel))
	}

	if len(details) > 0 {
		builder.WriteString(" (")
		builder.WriteString(strings.Join(details, ", "))
		builder.WriteString(")")
	}

	return builder.String()
}
