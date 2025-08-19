package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TicketService handles all ticket-related operations as a microservice
type TicketService struct {
	UnimplementedTicketServiceServer
	repository TicketRepository
	cache      CacheService
	events     EventBus
	metrics    MetricsCollector
}

// TicketRepository defines the interface for ticket data access
type TicketRepository interface {
	Create(ctx context.Context, ticket *Ticket) error
	GetByID(ctx context.Context, id string) (*Ticket, error)
	Update(ctx context.Context, ticket *Ticket) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter *ListFilter) ([]*Ticket, error)
	Search(ctx context.Context, query string) ([]*Ticket, error)
}

// CacheService defines the interface for caching
type CacheService interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Invalidate(ctx context.Context, pattern string) error
}

// EventBus defines the interface for event publishing
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
}

// MetricsCollector defines the interface for metrics collection
type MetricsCollector interface {
	IncrementCounter(name string, labels map[string]string)
	RecordDuration(name string, duration time.Duration, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

// Event represents a domain event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Source    string                 `json:"source"`
	Version   string                 `json:"version"`
}

// EventHandler processes events
type EventHandler func(ctx context.Context, event Event) error

// NewTicketService creates a new ticket service instance
func NewTicketService(repo TicketRepository, cache CacheService, events EventBus, metrics MetricsCollector) *TicketService {
	return &TicketService{
		repository: repo,
		cache:      cache,
		events:     events,
		metrics:    metrics,
	}
}

// CreateTicket creates a new ticket
func (s *TicketService) CreateTicket(ctx context.Context, req *CreateTicketRequest) (*CreateTicketResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("ticket.create.duration", time.Since(start), map[string]string{
			"priority": req.Priority,
			"queue":    req.QueueId,
		})
	}()

	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		s.metrics.IncrementCounter("ticket.create.validation_error", nil)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Create ticket entity
	ticket := &Ticket{
		ID:          generateTicketID(),
		Number:      generateTicketNumber(),
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Status:      "open",
		QueueID:     req.QueueId,
		CustomerID:  req.CustomerId,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save to repository
	if err := s.repository.Create(ctx, ticket); err != nil {
		s.metrics.IncrementCounter("ticket.create.error", map[string]string{
			"error": err.Error(),
		})
		return nil, status.Error(codes.Internal, "Failed to create ticket")
	}

	// Invalidate cache
	s.cache.Invalidate(ctx, "tickets:*")

	// Publish event
	event := Event{
		ID:        generateEventID(),
		Type:      "ticket.created",
		Timestamp: time.Now(),
		Source:    "ticket-service",
		Version:   "1.0",
		Data: map[string]interface{}{
			"ticket_id":   ticket.ID,
			"ticket_number": ticket.Number,
			"queue_id":    ticket.QueueID,
			"customer_id": ticket.CustomerID,
			"priority":    ticket.Priority,
		},
	}

	if err := s.events.Publish(ctx, event); err != nil {
		log.Printf("Failed to publish ticket.created event: %v", err)
	}

	s.metrics.IncrementCounter("ticket.created", map[string]string{
		"priority": ticket.Priority,
		"queue":    ticket.QueueID,
	})

	return &CreateTicketResponse{
		Ticket: ticket.ToProto(),
	}, nil
}

// GetTicket retrieves a ticket by ID
func (s *TicketService) GetTicket(ctx context.Context, req *GetTicketRequest) (*GetTicketResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("ticket.get.duration", time.Since(start), nil)
	}()

	// Check cache first
	cacheKey := fmt.Sprintf("ticket:%s", req.Id)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var ticket Ticket
		if err := json.Unmarshal(cached, &ticket); err == nil {
			s.metrics.IncrementCounter("ticket.get.cache_hit", nil)
			return &GetTicketResponse{
				Ticket: ticket.ToProto(),
			}, nil
		}
	}

	s.metrics.IncrementCounter("ticket.get.cache_miss", nil)

	// Get from repository
	ticket, err := s.repository.GetByID(ctx, req.Id)
	if err != nil {
		s.metrics.IncrementCounter("ticket.get.error", nil)
		return nil, status.Error(codes.NotFound, "Ticket not found")
	}

	// Cache the result
	if data, err := json.Marshal(ticket); err == nil {
		s.cache.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return &GetTicketResponse{
		Ticket: ticket.ToProto(),
	}, nil
}

// UpdateTicket updates an existing ticket
func (s *TicketService) UpdateTicket(ctx context.Context, req *UpdateTicketRequest) (*UpdateTicketResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("ticket.update.duration", time.Since(start), nil)
	}()

	// Get existing ticket
	existing, err := s.repository.GetByID(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Ticket not found")
	}

	// Store previous state for event
	previousState := existing.Clone()

	// Apply updates
	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Priority != "" {
		existing.Priority = req.Priority
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	existing.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repository.Update(ctx, existing); err != nil {
		s.metrics.IncrementCounter("ticket.update.error", nil)
		return nil, status.Error(codes.Internal, "Failed to update ticket")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("ticket:%s", req.Id)
	s.cache.Delete(ctx, cacheKey)
	s.cache.Invalidate(ctx, "tickets:*")

	// Publish event
	event := Event{
		ID:        generateEventID(),
		Type:      "ticket.updated",
		Timestamp: time.Now(),
		Source:    "ticket-service",
		Version:   "1.0",
		Data: map[string]interface{}{
			"ticket_id":      existing.ID,
			"ticket_number":  existing.Number,
			"previous_state": previousState,
			"current_state":  existing,
			"changes":        s.detectChanges(previousState, existing),
		},
	}

	if err := s.events.Publish(ctx, event); err != nil {
		log.Printf("Failed to publish ticket.updated event: %v", err)
	}

	s.metrics.IncrementCounter("ticket.updated", map[string]string{
		"status": existing.Status,
	})

	return &UpdateTicketResponse{
		Ticket: existing.ToProto(),
	}, nil
}

// ListTickets lists tickets with filtering
func (s *TicketService) ListTickets(ctx context.Context, req *ListTicketsRequest) (*ListTicketsResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("ticket.list.duration", time.Since(start), nil)
	}()

	// Build filter
	filter := &ListFilter{
		Status:   req.Status,
		Priority: req.Priority,
		QueueID:  req.QueueId,
		Limit:    int(req.Limit),
		Offset:   int(req.Offset),
	}

	// Check cache
	cacheKey := fmt.Sprintf("tickets:list:%s", filter.Hash())
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var tickets []*Ticket
		if err := json.Unmarshal(cached, &tickets); err == nil {
			s.metrics.IncrementCounter("ticket.list.cache_hit", nil)
			return s.buildListResponse(tickets), nil
		}
	}

	// Get from repository
	tickets, err := s.repository.List(ctx, filter)
	if err != nil {
		s.metrics.IncrementCounter("ticket.list.error", nil)
		return nil, status.Error(codes.Internal, "Failed to list tickets")
	}

	// Cache the result
	if data, err := json.Marshal(tickets); err == nil {
		s.cache.Set(ctx, cacheKey, data, 1*time.Minute)
	}

	s.metrics.IncrementCounter("ticket.list.success", map[string]string{
		"count": fmt.Sprintf("%d", len(tickets)),
	})

	return s.buildListResponse(tickets), nil
}

// SearchTickets searches tickets
func (s *TicketService) SearchTickets(ctx context.Context, req *SearchTicketsRequest) (*SearchTicketsResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.RecordDuration("ticket.search.duration", time.Since(start), nil)
	}()

	tickets, err := s.repository.Search(ctx, req.Query)
	if err != nil {
		s.metrics.IncrementCounter("ticket.search.error", nil)
		return nil, status.Error(codes.Internal, "Search failed")
	}

	s.metrics.IncrementCounter("ticket.search.success", map[string]string{
		"results": fmt.Sprintf("%d", len(tickets)),
	})

	return &SearchTicketsResponse{
		Tickets: s.ticketsToProto(tickets),
		Total:   int32(len(tickets)),
	}, nil
}

// Start starts the gRPC server
func (s *TicketService) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(s.unaryInterceptor),
	)

	RegisterTicketServiceServer(grpcServer, s)

	log.Printf("Ticket service starting on port %d", port)
	return grpcServer.Serve(lis)
}

// unaryInterceptor adds logging and metrics to all unary RPC calls
func (s *TicketService) unaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	
	// Call the handler
	resp, err := handler(ctx, req)
	
	// Record metrics
	duration := time.Since(start)
	s.metrics.RecordDuration("grpc.request.duration", duration, map[string]string{
		"method": info.FullMethod,
		"status": grpcStatusCode(err),
	})
	
	if err != nil {
		log.Printf("gRPC error: %s: %v", info.FullMethod, err)
		s.metrics.IncrementCounter("grpc.request.error", map[string]string{
			"method": info.FullMethod,
		})
	}
	
	return resp, err
}

// Helper functions

func (s *TicketService) validateCreateRequest(req *CreateTicketRequest) error {
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	if req.Description == "" {
		return fmt.Errorf("description is required")
	}
	if req.QueueId == "" {
		return fmt.Errorf("queue_id is required")
	}
	return nil
}

func (s *TicketService) detectChanges(old, new *Ticket) map[string]interface{} {
	changes := make(map[string]interface{})
	
	if old.Title != new.Title {
		changes["title"] = map[string]string{"old": old.Title, "new": new.Title}
	}
	if old.Description != new.Description {
		changes["description"] = map[string]string{"old": old.Description, "new": new.Description}
	}
	if old.Priority != new.Priority {
		changes["priority"] = map[string]string{"old": old.Priority, "new": new.Priority}
	}
	if old.Status != new.Status {
		changes["status"] = map[string]string{"old": old.Status, "new": new.Status}
	}
	
	return changes
}

func (s *TicketService) buildListResponse(tickets []*Ticket) *ListTicketsResponse {
	return &ListTicketsResponse{
		Tickets: s.ticketsToProto(tickets),
		Total:   int32(len(tickets)),
	}
}

func (s *TicketService) ticketsToProto(tickets []*Ticket) []*TicketProto {
	result := make([]*TicketProto, len(tickets))
	for i, ticket := range tickets {
		result[i] = ticket.ToProto()
	}
	return result
}

func grpcStatusCode(err error) string {
	if err == nil {
		return "OK"
	}
	if st, ok := status.FromError(err); ok {
		return st.Code().String()
	}
	return "UNKNOWN"
}

func generateTicketID() string {
	return fmt.Sprintf("TKT-%d", time.Now().UnixNano())
}

func generateTicketNumber() string {
	return fmt.Sprintf("T-%d-%06d", time.Now().Year(), time.Now().UnixNano()%1000000)
}

func generateEventID() string {
	return fmt.Sprintf("EVT-%d", time.Now().UnixNano())
}