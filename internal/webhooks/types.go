// Package webhooks defines the ticket and article events GoatFlow publishes to
// outbound webhooks and turns database changes into those events.
package webhooks

// Event types a webhook can subscribe to.
const (
	EventTicketCreated         = "ticket.created"
	EventTicketUpdated         = "ticket.updated"
	EventTicketStateChanged    = "ticket.state_changed"
	EventTicketClosed          = "ticket.closed"
	EventTicketQueueMoved      = "ticket.queue_moved"
	EventTicketAssigned        = "ticket.assigned"
	EventTicketPriorityChanged = "ticket.priority_changed"
	EventTicketMerged          = "ticket.merged"
	EventTicketEscalated       = "ticket.escalated"
	EventArticleCreated        = "article.created"
)

// EventInfo describes one subscribable event.
type EventInfo struct {
	Event       string `json:"event"`
	Description string `json:"description"`
}

// Events is the catalogue of subscribable events, in display order.
var Events = []EventInfo{
	{EventTicketCreated, "A ticket was created (any channel)."},
	{EventTicketUpdated, "Ticket title, customer, type, service, SLA, responsible, lock, pending time or a dynamic field changed."},
	{EventTicketStateChanged, "The ticket state changed to a state that is not closed."},
	{EventTicketClosed, "The ticket state changed to a closed state."},
	{EventTicketQueueMoved, "The ticket moved to another queue."},
	{EventTicketAssigned, "The ticket owner changed."},
	{EventTicketPriorityChanged, "The ticket priority changed."},
	{EventTicketMerged, "The ticket was merged."},
	{EventTicketEscalated, "A response, update or solution time escalation started."},
	{EventArticleCreated, "An article (note, reply, email, phone call, web request) was added to a ticket."},
}

// IsEvent reports whether name is a subscribable event.
func IsEvent(name string) bool {
	for _, e := range Events {
		if e.Event == name {
			return true
		}
	}
	return false
}

// historyEvents maps OTRS ticket_history types to events. History types not
// listed here (article history, notifications, time accounting, ...) do not
// produce events; articles are published from the article table instead.
var historyEvents = map[string]string{
	"Move":                        EventTicketQueueMoved,
	"OwnerUpdate":                 EventTicketAssigned,
	"PriorityUpdate":              EventTicketPriorityChanged,
	"Merged":                      EventTicketMerged,
	"EscalationResponseTimeStart": EventTicketEscalated,
	"EscalationUpdateTimeStart":   EventTicketEscalated,
	"EscalationSolutionTimeStart": EventTicketEscalated,
	"TitleUpdate":                 EventTicketUpdated,
	"CustomerUpdate":              EventTicketUpdated,
	"TypeUpdate":                  EventTicketUpdated,
	"ServiceUpdate":               EventTicketUpdated,
	"SLAUpdate":                   EventTicketUpdated,
	"ResponsibleUpdate":           EventTicketUpdated,
	"Lock":                        EventTicketUpdated,
	"Unlock":                      EventTicketUpdated,
	"SetPendingTime":              EventTicketUpdated,
	"TicketDynamicFieldUpdate":    EventTicketUpdated,
	"ArchiveFlagUpdate":           EventTicketUpdated,
}

// historyEvent returns the event for a ticket_history row; stateType is the
// ticket_state_type name of the state recorded on that row.
func historyEvent(historyType, stateType string) (string, bool) {
	if historyType == "StateUpdate" {
		if stateType == "closed" {
			return EventTicketClosed, true
		}
		return EventTicketStateChanged, true
	}
	e, ok := historyEvents[historyType]
	return e, ok
}
