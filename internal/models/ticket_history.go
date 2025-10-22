package models

import "time"

// TicketHistoryEntry represents a single history change for a ticket.
type TicketHistoryEntry struct {
	ID              uint
	HistoryType     string
	Name            string
	CreatorLogin    string
	CreatorFullName string
	CreatedAt       time.Time
	ArticleSubject  string
	QueueName       string
	StateName       string
	PriorityName    string
}

// TicketLink represents a linked ticket relationship.
type TicketLink struct {
	RelatedTicketID    uint
	RelatedTicketTN    string
	RelatedTicketTitle string
	LinkType           string
	LinkState          string
	Direction          string
	CreatorLogin       string
	CreatorFullName    string
	CreatedAt          time.Time
}

// TicketHistoryInsert captures the data required to persist a history entry.
type TicketHistoryInsert struct {
	TicketID    int
	ArticleID   *int
	TypeID      int
	QueueID     int
	OwnerID     int
	PriorityID  int
	StateID     int
	CreatedBy   int
	HistoryType string
	Name        string
	CreatedAt   time.Time
}
