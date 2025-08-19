package ticket

import (
	"context"
)

// Proto stub definitions until we generate from .proto files
// This allows the service to compile without protoc

// TicketProto represents a ticket in protobuf format
type TicketProto struct {
	Id              string   `json:"id"`
	Number          string   `json:"number"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Priority        string   `json:"priority"`
	Status          string   `json:"status"`
	QueueId         string   `json:"queue_id"`
	CustomerId      string   `json:"customer_id"`
	AssignedTo      string   `json:"assigned_to"`
	Tags            []string `json:"tags"`
	CreatedAt       int64    `json:"created_at"`
	UpdatedAt       int64    `json:"updated_at"`
	ResolvedAt      int64    `json:"resolved_at"`
	ClosedAt        int64    `json:"closed_at"`
	DueAt           int64    `json:"due_at"`
	FirstResponseAt int64    `json:"first_response_at"`
}

// Request/Response types for gRPC

type CreateTicketRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Priority    string            `json:"priority"`
	QueueId     string            `json:"queue_id"`
	CustomerId  string            `json:"customer_id"`
	Tags        []string          `json:"tags"`
	CustomFields map[string]string `json:"custom_fields"`
}

type CreateTicketResponse struct {
	Ticket *TicketProto `json:"ticket"`
}

type GetTicketRequest struct {
	Id string `json:"id"`
}

type GetTicketResponse struct {
	Ticket *TicketProto `json:"ticket"`
}

type UpdateTicketRequest struct {
	Id          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Status      string   `json:"status"`
	AssignedTo  string   `json:"assigned_to"`
	Tags        []string `json:"tags"`
}

type UpdateTicketResponse struct {
	Ticket *TicketProto `json:"ticket"`
}

type DeleteTicketRequest struct {
	Id string `json:"id"`
}

type DeleteTicketResponse struct {
	Success bool `json:"success"`
}

type ListTicketsRequest struct {
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	QueueId    string `json:"queue_id"`
	AssignedTo string `json:"assigned_to"`
	CustomerId string `json:"customer_id"`
	Limit      int32  `json:"limit"`
	Offset     int32  `json:"offset"`
}

type ListTicketsResponse struct {
	Tickets []*TicketProto `json:"tickets"`
	Total   int32          `json:"total"`
}

type SearchTicketsRequest struct {
	Query string `json:"query"`
	Limit int32  `json:"limit"`
}

type SearchTicketsResponse struct {
	Tickets []*TicketProto `json:"tickets"`
	Total   int32          `json:"total"`
}

// gRPC Server interface
type TicketServiceServer interface {
	CreateTicket(context.Context, *CreateTicketRequest) (*CreateTicketResponse, error)
	GetTicket(context.Context, *GetTicketRequest) (*GetTicketResponse, error)
	UpdateTicket(context.Context, *UpdateTicketRequest) (*UpdateTicketResponse, error)
	DeleteTicket(context.Context, *DeleteTicketRequest) (*DeleteTicketResponse, error)
	ListTickets(context.Context, *ListTicketsRequest) (*ListTicketsResponse, error)
	SearchTickets(context.Context, *SearchTicketsRequest) (*SearchTicketsResponse, error)
}

// UnimplementedTicketServiceServer can be embedded to have forward compatible implementations
type UnimplementedTicketServiceServer struct{}

func (UnimplementedTicketServiceServer) CreateTicket(context.Context, *CreateTicketRequest) (*CreateTicketResponse, error) {
	return nil, nil
}

func (UnimplementedTicketServiceServer) GetTicket(context.Context, *GetTicketRequest) (*GetTicketResponse, error) {
	return nil, nil
}

func (UnimplementedTicketServiceServer) UpdateTicket(context.Context, *UpdateTicketRequest) (*UpdateTicketResponse, error) {
	return nil, nil
}

func (UnimplementedTicketServiceServer) DeleteTicket(context.Context, *DeleteTicketRequest) (*DeleteTicketResponse, error) {
	return nil, nil
}

func (UnimplementedTicketServiceServer) ListTickets(context.Context, *ListTicketsRequest) (*ListTicketsResponse, error) {
	return nil, nil
}

func (UnimplementedTicketServiceServer) SearchTickets(context.Context, *SearchTicketsRequest) (*SearchTicketsResponse, error) {
	return nil, nil
}

// RegisterTicketServiceServer registers the service with gRPC server
func RegisterTicketServiceServer(s interface{}, srv TicketServiceServer) {
	// Stub implementation for compilation
}