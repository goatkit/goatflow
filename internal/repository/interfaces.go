package repository

import "github.com/gotrs-io/gotrs-ce/internal/models"

// ITicketRepository defines the interface for ticket data operations.
type ITicketRepository interface {
	Create(ticket *models.Ticket) error
	GetByID(id uint) (*models.Ticket, error)
	Update(ticket *models.Ticket) error
	Delete(id uint) error
	List(req *models.TicketListRequest) (*models.TicketListResponse, error)
	GetByTicketNumber(ticketNumber string) (*models.Ticket, error)
	Count() (int, error)
	CountByStatus(status string) (int, error)
	// Dashboard statistics methods
	CountByStateID(stateID int) (int, error)
	CountClosedToday() (int, error)
}
