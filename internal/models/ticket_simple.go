package models

import "time"

// It maps to the OTRS-compatible Ticket model but with more intuitive field names.
type SimpleTicket struct {
	ID            uint      `json:"id"`
	TicketNumber  string    `json:"ticket_number"`
	Subject       string    `json:"subject"`
	QueueID       uint      `json:"queue_id"`
	TypeID        uint      `json:"type_id"`
	Priority      string    `json:"priority"` // "low", "normal", "high", "urgent"
	Status        string    `json:"status"`   // "new", "open", "pending", "closed"
	CustomerEmail string    `json:"customer_email"`
	CustomerName  string    `json:"customer_name"`
	AssignedTo    uint      `json:"assigned_to"`
	CreatedBy     uint      `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ToORTSTicket converts SimpleTicket to OTRS-compatible Ticket model.
func (st *SimpleTicket) ToORTSTicket() *Ticket {
	// Map priority string to ID
	priorityMap := map[string]int{
		"low":    1,
		"normal": 2,
		"high":   3,
		"urgent": 4,
	}
	priorityID := priorityMap[st.Priority]
	if priorityID == 0 {
		priorityID = 2 // Default to normal
	}

	// Map status string to state ID
	stateMap := map[string]int{
		"new":     1,
		"open":    2,
		"pending": 3,
		"closed":  4,
	}
	stateID := stateMap[st.Status]
	if stateID == 0 {
		stateID = 1 // Default to new
	}

	var userID *int
	if st.AssignedTo > 0 {
		uid := int(st.AssignedTo)
		userID = &uid
	}

	var customerID *string
	if st.CustomerEmail != "" {
		customerID = &st.CustomerEmail
	}

	var typeID *int
	if st.TypeID > 0 {
		tid := int(st.TypeID)
		typeID = &tid
	}

	return &Ticket{
		ID:               int(st.ID),
		TicketNumber:     st.TicketNumber,
		Title:            st.Subject,
		QueueID:          int(st.QueueID),
		TypeID:           typeID,
		TicketPriorityID: priorityID,
		TicketStateID:    stateID,
		TicketLockID:     1, // Unlocked by default
		UserID:           userID,
		CustomerUserID:   customerID,
		CreateTime:       st.CreatedAt,
		CreateBy:         int(st.CreatedBy),
		ChangeTime:       st.UpdatedAt,
		ChangeBy:         int(st.CreatedBy),
	}
}

// FromORTSTicket creates a SimpleTicket from OTRS Ticket model.
func FromORTSTicket(t *Ticket) *SimpleTicket {
	// Map priority ID to string
	priorityMap := map[int]string{
		1: "low",
		2: "normal",
		3: "high",
		4: "urgent",
	}
	priority := priorityMap[t.TicketPriorityID]
	if priority == "" {
		priority = "normal"
	}

	// Map state ID to status string
	statusMap := map[int]string{
		1: "new",
		2: "open",
		3: "pending",
		4: "closed",
	}
	status := statusMap[t.TicketStateID]
	if status == "" {
		status = "new"
	}

	var assignedTo uint
	if t.UserID != nil {
		assignedTo = uint(*t.UserID)
	}

	var customerEmail string
	if t.CustomerUserID != nil {
		customerEmail = *t.CustomerUserID
	}

	var typeID uint
	if t.TypeID != nil {
		typeID = uint(*t.TypeID)
	}

	return &SimpleTicket{
		ID:            uint(t.ID),
		TicketNumber:  t.TicketNumber,
		Subject:       t.Title,
		QueueID:       uint(t.QueueID),
		TypeID:        typeID,
		Priority:      priority,
		Status:        status,
		CustomerEmail: customerEmail,
		AssignedTo:    assignedTo,
		CreatedBy:     uint(t.CreateBy),
		CreatedAt:     t.CreateTime,
		UpdatedAt:     t.ChangeTime,
	}
}
