// Package notificationevents evaluates the ticket notification rules managed
// under Admin -> Ticket Notifications (the OTRS notification_event,
// notification_event_item and notification_event_message tables) and queues
// the resulting email on mail_queue for the email runner to send.
package notificationevents

// TicketEvents are the ticket events a notification rule can subscribe to,
// in the order the admin form lists them. Each one is raised from a row
// GoatFlow writes: TicketCreate from the ticket table, the others from
// ticket_history (see historyEvents). The Notification* names are the OTRS
// legacy events used by notifications imported from OTRS.
var TicketEvents = []string{
	"TicketCreate",
	"TicketTitleUpdate",
	"TicketQueueUpdate",
	"TicketTypeUpdate",
	"TicketCustomerUpdate",
	"TicketPendingTimeUpdate",
	"TicketLockUpdate",
	"TicketStateUpdate",
	"TicketOwnerUpdate",
	"TicketResponsibleUpdate",
	"TicketPriorityUpdate",
	"TicketMerge",
	"EscalationResponseTimeNotifyBefore",
	"EscalationResponseTimeStart",
	"EscalationResponseTimeStop",
	"EscalationUpdateTimeNotifyBefore",
	"EscalationUpdateTimeStart",
	"EscalationUpdateTimeStop",
	"EscalationSolutionTimeNotifyBefore",
	"EscalationSolutionTimeStart",
	"EscalationSolutionTimeStop",
	"NotificationNewTicket",
	"NotificationAddNote",
	"NotificationMove",
	"NotificationOwnerUpdate",
	"NotificationResponsibleUpdate",
	"NotificationEscalation",
	"NotificationEscalationNotifyBefore",
}

// ArticleEvents are the article events a notification rule can subscribe to.
var ArticleEvents = []string{
	"ArticleCreate",
}

// Events raised by a new row in the ticket and article tables.
var (
	ticketRowEvents  = []string{"TicketCreate", "NotificationNewTicket"}
	articleRowEvents = []string{"ArticleCreate"}
)

// historyEvents maps OTRS ticket_history types to the notification events
// they raise. Merged is handled in historyRowEvents.
var historyEvents = map[string][]string{
	"StateUpdate":       {"TicketStateUpdate"},
	"Move":              {"TicketQueueUpdate", "NotificationMove"},
	"OwnerUpdate":       {"TicketOwnerUpdate", "NotificationOwnerUpdate"},
	"ResponsibleUpdate": {"TicketResponsibleUpdate", "NotificationResponsibleUpdate"},
	"PriorityUpdate":    {"TicketPriorityUpdate"},
	"TitleUpdate":       {"TicketTitleUpdate"},
	"TypeUpdate":        {"TicketTypeUpdate"},
	"CustomerUpdate":    {"TicketCustomerUpdate"},
	"SetPendingTime":    {"TicketPendingTimeUpdate"},
	"Lock":              {"TicketLockUpdate"},
	"Unlock":            {"TicketLockUpdate"},
	"AddNote":           {"NotificationAddNote"},

	"EscalationResponseTimeNotifyBefore": {"EscalationResponseTimeNotifyBefore", metaEscalationNotifyBefore},
	"EscalationUpdateTimeNotifyBefore":   {"EscalationUpdateTimeNotifyBefore", metaEscalationNotifyBefore},
	"EscalationSolutionTimeNotifyBefore": {"EscalationSolutionTimeNotifyBefore", metaEscalationNotifyBefore},
	"EscalationResponseTimeStart":        {"EscalationResponseTimeStart", metaEscalation},
	"EscalationUpdateTimeStart":          {"EscalationUpdateTimeStart", metaEscalation},
	"EscalationSolutionTimeStart":        {"EscalationSolutionTimeStart", metaEscalation},
	"EscalationResponseTimeStop":         {"EscalationResponseTimeStop"},
	"EscalationUpdateTimeStop":           {"EscalationUpdateTimeStop"},
	"EscalationSolutionTimeStop":         {"EscalationSolutionTimeStop"},
}

// The OTRS escalation meta-events fire once per ticket for one escalation
// check, however many of its response/update/solution escalations started.
const (
	metaEscalation             = "NotificationEscalation"
	metaEscalationNotifyBefore = "NotificationEscalationNotifyBefore"
)

// historyRowEvents returns the events raised by a ticket_history row.
// stateType is the ticket_state_type of the state recorded on the row.
// A merge writes a Merged row on the surviving ticket and on every merged
// ticket; like OTRS, TicketMerge fires for the merged tickets only.
func historyRowEvents(historyType, stateType string) []string {
	if historyType == "Merged" {
		if stateType == "merged" {
			return []string{"TicketMerge"}
		}
		return nil
	}
	return historyEvents[historyType]
}
