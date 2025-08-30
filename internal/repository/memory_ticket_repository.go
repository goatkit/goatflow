package repository

import (
	"fmt"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// MemoryTicketRepository implements TicketRepository with in-memory storage
// This is for development/testing. Production should use PostgreSQL implementation.
type MemoryTicketRepository struct {
	mu      sync.RWMutex
	tickets map[uint]*models.SimpleTicket
	nextID  uint
}

// NewMemoryTicketRepository creates a new in-memory ticket repository
func NewMemoryTicketRepository() *MemoryTicketRepository {
	return &MemoryTicketRepository{
		tickets: make(map[uint]*models.SimpleTicket),
		nextID:  1001, // Start from 1001 to avoid conflicts with mock data
	}
}

// Create saves a new ticket to memory
func (r *MemoryTicketRepository) Create(ticket *models.Ticket) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Convert to SimpleTicket for internal storage
	simpleTicket := models.FromORTSTicket(ticket)
	
	// Generate ID and ticket number
	simpleTicket.ID = r.nextID
	simpleTicket.TicketNumber = fmt.Sprintf("TICKET-%06d", r.nextID)
	ticket.ID = int(r.nextID)
	ticket.TicketNumber = simpleTicket.TicketNumber
	r.nextID++

	// Set timestamps
	now := time.Now()
	simpleTicket.CreatedAt = now
	simpleTicket.UpdatedAt = now
	ticket.CreateTime = now
	ticket.ChangeTime = now

	// Set defaults if not provided
	if simpleTicket.Status == "" {
		simpleTicket.Status = "new"
		ticket.TicketStateID = 1
	}
	if simpleTicket.Priority == "" {
		simpleTicket.Priority = "normal"
		ticket.TicketPriorityID = 2
	}
	if simpleTicket.QueueID == 0 {
		simpleTicket.QueueID = 1
		ticket.QueueID = 1
	}
	if simpleTicket.TypeID == 0 {
		simpleTicket.TypeID = 1
		typeID := 1
		ticket.TypeID = &typeID
	}

	// Store the ticket
	r.tickets[simpleTicket.ID] = simpleTicket

	return nil
}

// GetByID retrieves a ticket by its ID
func (r *MemoryTicketRepository) GetByID(id uint) (*models.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	simpleTicket, exists := r.tickets[id]
	if !exists {
		return nil, fmt.Errorf("ticket not found: %d", id)
	}

	// Convert to ORTS ticket and return a copy
	ortsTicket := simpleTicket.ToORTSTicket()
	return ortsTicket, nil
}

// Update modifies an existing ticket
func (r *MemoryTicketRepository) Update(ticket *models.Ticket) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tickets[uint(ticket.ID)]; !exists {
		return fmt.Errorf("ticket not found: %d", ticket.ID)
	}

	// Convert to SimpleTicket and update
	simpleTicket := models.FromORTSTicket(ticket)
	simpleTicket.UpdatedAt = time.Now()
	ticket.ChangeTime = simpleTicket.UpdatedAt
	r.tickets[uint(ticket.ID)] = simpleTicket

	return nil
}

// Delete removes a ticket from memory
func (r *MemoryTicketRepository) Delete(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tickets[id]; !exists {
		return fmt.Errorf("ticket not found: %d", id)
	}

	delete(r.tickets, id)
	return nil
}

// List returns a paginated list of tickets
func (r *MemoryTicketRepository) List(req *models.TicketListRequest) (*models.TicketListResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Convert map to slice of ORTS tickets
	allTickets := make([]models.Ticket, 0, len(r.tickets))
	for _, simpleTicket := range r.tickets {
		ortsTicket := simpleTicket.ToORTSTicket()
		allTickets = append(allTickets, *ortsTicket)
	}

	// Apply filters (simplified for now)
	// TODO: Implement full filtering based on TicketListRequest fields

	// Apply pagination
	page := req.Page
	if page < 1 {
		page = 1
	}
	perPage := req.PerPage
	if perPage < 1 {
		perPage = 20 // Default page size
	}
	
	start := (page - 1) * perPage
	end := start + perPage
	
	if start > len(allTickets) {
		start = len(allTickets)
	}
	if end > len(allTickets) {
		end = len(allTickets)
	}

	paginatedTickets := allTickets[start:end]
	totalPages := (len(allTickets) + perPage - 1) / perPage

	return &models.TicketListResponse{
		Tickets:    paginatedTickets,
		Total:      len(allTickets),
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}


// containsIgnoreCase checks if a string contains another string (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		   (s == substr || len(s) > len(substr) && 
		   (s[:len(substr)] == substr || containsIgnoreCase(s[1:], substr)))
}

// GetByTicketNumber retrieves a ticket by its ticket number
func (r *MemoryTicketRepository) GetByTicketNumber(ticketNumber string) (*models.Ticket, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, simpleTicket := range r.tickets {
		if simpleTicket.TicketNumber == ticketNumber {
			ortsTicket := simpleTicket.ToORTSTicket()
			return ortsTicket, nil
		}
	}

	return nil, fmt.Errorf("ticket not found: %s", ticketNumber)
}

// Count returns the total number of tickets
func (r *MemoryTicketRepository) Count() (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tickets), nil
}

// CountByStatus returns the count of tickets by status
func (r *MemoryTicketRepository) CountByStatus(status string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, simpleTicket := range r.tickets {
		if simpleTicket.Status == status {
			count++
		}
	}

	return count, nil
}