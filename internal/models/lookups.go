package models

// QueueInfo represents queue information for dropdowns.
type QueueInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
}

// TicketFormData contains all the data needed for ticket forms.
type TicketFormData struct {
	Queues     []QueueInfo  `json:"queues"`
	Priorities []LookupItem `json:"priorities"`
	Types      []LookupItem `json:"types"`
	Statuses   []LookupItem `json:"statuses"`
}
