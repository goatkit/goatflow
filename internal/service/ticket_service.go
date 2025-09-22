package service

import (
	"database/sql"
	"fmt"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// TicketService handles business logic for tickets
type TicketService struct {
	ticketRepo  *repository.TicketRepository
	articleRepo *repository.ArticleRepository
	db          *sql.DB
}

// NewTicketService creates a new ticket service
func NewTicketService(db *sql.DB) *TicketService {
	return &TicketService{
		ticketRepo:  repository.NewTicketRepository(db),
		articleRepo: repository.NewArticleRepository(db),
		db:          db,
	}
}

// CreateTicketRequest represents a simplified ticket creation request from the form
type CreateTicketRequest struct {
	Subject       string `json:"subject" form:"subject" binding:"required"`
	CustomerEmail string `json:"customer_email" form:"customer_email" binding:"required,email"`
	CustomerName  string `json:"customer_name" form:"customer_name"`
	Priority      string `json:"priority" form:"priority"`
	QueueID       int    `json:"queue_id" form:"queue_id"`
	TypeID        int    `json:"type_id" form:"type_id"`
	Body          string `json:"body" form:"body" binding:"required"`
}

// CreateTicketResponse represents the response after creating a ticket
type CreateTicketResponse struct {
	ID           int    `json:"id"`
	TicketNumber string `json:"ticket_number"`
	Message      string `json:"message"`
}

// CreateTicket creates a new ticket from a simplified request
func (s *TicketService) CreateTicket(req *CreateTicketRequest, createBy int) (*CreateTicketResponse, error) {
	// Parse priority (format: "3 normal" -> priority_id = 3)
	priorityID := s.parsePriorityID(req.Priority)

	// Set defaults
	if req.QueueID == 0 {
		req.QueueID = 1 // Default to General Support
	}
	if req.TypeID == 0 {
		req.TypeID = 1 // Default to Incident
	}

	// Create ticket
	typeID := req.TypeID
	ticket := &models.Ticket{
		Title:                  req.Subject,
		QueueID:                req.QueueID,
		TicketLockID:           models.TicketUnlocked,
		TypeID:                 &typeID,
		TicketStateID:          models.TicketStateNew,
		TicketPriorityID:       priorityID,
		UntilTime:              0,
		EscalationTime:         0,
		EscalationUpdateTime:   0,
		EscalationResponseTime: 0,
		EscalationSolutionTime: 0,
		ArchiveFlag:            0,
		CreateBy:               createBy,
		ChangeBy:               createBy,
	}

	// For demo purposes, we'll store customer info in the customer fields
	// In a real system, we'd create/lookup customer records
	if req.CustomerEmail != "" {
		ticket.CustomerUserID = &req.CustomerEmail
	}

	// Create the ticket
	if err := s.ticketRepo.Create(ticket); err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	// Create the initial article (the customer's message)
	article := &models.Article{
		TicketID:               ticket.ID,
		ArticleTypeID:          models.ArticleTypeWebRequest,
		SenderTypeID:           models.SenderTypeCustomer,
		CommunicationChannelID: 1, // Web
		IsVisibleForCustomer:   1, // Visible to customer
		Subject:                req.Subject,
		Body:                   req.Body,
		BodyType:               "text/plain",
		Charset:                "utf-8",
		MimeType:               "text/plain",
		ValidID:                1,
		CreateBy:               createBy,
		ChangeBy:               createBy,
	}

	// Insert article via repository (handles schema + tx)
	if err := s.articleRepo.Create(article); err != nil {
		return nil, fmt.Errorf("failed to create article: %w", err)
	}

	return &CreateTicketResponse{
		ID:           ticket.ID,
		TicketNumber: ticket.TicketNumber,
		Message:      "Ticket created successfully",
	}, nil
}

// parsePriorityID extracts the priority ID from the priority string
func (s *TicketService) parsePriorityID(priority string) int {
	switch priority {
	case "1 very low":
		return 1
	case "2 low":
		return 2
	case "3 normal":
		return 3
	case "4 high":
		return 4
	case "5 very high":
		return 5
	default:
		return 3 // Default to normal
	}
}

// GetTicket retrieves a ticket by ID
func (s *TicketService) GetTicket(ticketID int) (*models.Ticket, error) {
	return s.ticketRepo.GetByID(uint(ticketID))
}

// ListTickets retrieves a paginated list of tickets
func (s *TicketService) ListTickets(req *models.TicketListRequest) (*models.TicketListResponse, error) {
	return s.ticketRepo.List(req)
}
