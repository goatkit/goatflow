package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/goatkit/goatflow/sdk/go/types"
)

// TicketsService handles ticket-related API operations
type TicketsService struct {
	client *Client
}

// List retrieves a list of tickets
func (s *TicketsService) List(ctx context.Context, options *types.TicketListOptions) (*types.TicketListResponse, error) {
	path := "/api/v1/tickets"

	if options != nil {
		query := url.Values{}

		if options.Page > 0 {
			query.Set("page", strconv.Itoa(options.Page))
		}
		if options.PageSize > 0 {
			query.Set("page_size", strconv.Itoa(options.PageSize))
		}
		if len(options.Status) > 0 {
			query.Set("status", strings.Join(options.Status, ","))
		}
		if len(options.Priority) > 0 {
			query.Set("priority", strings.Join(options.Priority, ","))
		}
		if len(options.QueueID) > 0 {
			queueIDs := make([]string, len(options.QueueID))
			for i, id := range options.QueueID {
				queueIDs[i] = strconv.FormatUint(uint64(id), 10)
			}
			query.Set("queue_id", strings.Join(queueIDs, ","))
		}
		if options.AssignedTo != nil {
			query.Set("assigned_to", strconv.FormatUint(uint64(*options.AssignedTo), 10))
		}
		if options.CustomerID != nil {
			query.Set("customer_id", strconv.FormatUint(uint64(*options.CustomerID), 10))
		}
		if options.Search != "" {
			query.Set("search", options.Search)
		}
		if len(options.Tags) > 0 {
			query.Set("tags", strings.Join(options.Tags, ","))
		}
		if options.CreatedAfter != nil {
			query.Set("created_after", options.CreatedAfter.Format("2006-01-02T15:04:05Z"))
		}
		if options.CreatedBefore != nil {
			query.Set("created_before", options.CreatedBefore.Format("2006-01-02T15:04:05Z"))
		}
		if options.SortBy != "" {
			query.Set("sort_by", options.SortBy)
		}
		if options.SortOrder != "" {
			query.Set("sort_order", options.SortOrder)
		}

		if len(query) > 0 {
			path += "?" + query.Encode()
		}
	}

	var result types.TicketListResponse
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// Get retrieves a specific ticket by ID
func (s *TicketsService) Get(ctx context.Context, id uint) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d", id)

	var result types.Ticket
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// GetByNumber retrieves a specific ticket by ticket number
func (s *TicketsService) GetByNumber(ctx context.Context, ticketNumber string) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/number/%s", ticketNumber)

	var result types.Ticket
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// Create creates a new ticket
func (s *TicketsService) Create(ctx context.Context, request *types.TicketCreateRequest) (*types.Ticket, error) {
	path := "/api/v1/tickets"

	var result types.Ticket
	err := s.client.Post(ctx, path, request, &result)
	return &result, err
}

// Update updates an existing ticket
func (s *TicketsService) Update(ctx context.Context, id uint, request *types.TicketUpdateRequest) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d", id)

	var result types.Ticket
	err := s.client.Put(ctx, path, request, &result)
	return &result, err
}

// Delete deletes a ticket
func (s *TicketsService) Delete(ctx context.Context, id uint) error {
	path := fmt.Sprintf("/api/v1/tickets/%d", id)
	return s.client.Delete(ctx, path, nil)
}

// Close closes a ticket
func (s *TicketsService) Close(ctx context.Context, id uint, reason string) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/close", id)

	body := map[string]string{"reason": reason}
	var result types.Ticket
	err := s.client.Post(ctx, path, body, &result)
	return &result, err
}

// Reopen reopens a closed ticket
func (s *TicketsService) Reopen(ctx context.Context, id uint, reason string) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/reopen", id)

	body := map[string]string{"reason": reason}
	var result types.Ticket
	err := s.client.Post(ctx, path, body, &result)
	return &result, err
}

// Assign assigns a ticket to a user
func (s *TicketsService) Assign(ctx context.Context, id uint, userID uint) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/assign", id)

	body := map[string]uint{"user_id": userID}
	var result types.Ticket
	err := s.client.Post(ctx, path, body, &result)
	return &result, err
}

// Unassign removes assignment from a ticket
func (s *TicketsService) Unassign(ctx context.Context, id uint) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/unassign", id)

	var result types.Ticket
	err := s.client.Post(ctx, path, nil, &result)
	return &result, err
}

// AddMessage adds a message to a ticket
func (s *TicketsService) AddMessage(ctx context.Context, id uint, request *types.MessageCreateRequest) (*types.TicketMessage, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/messages", id)

	var result types.TicketMessage
	err := s.client.Post(ctx, path, request, &result)
	return &result, err
}

// GetMessages retrieves messages for a ticket
func (s *TicketsService) GetMessages(ctx context.Context, id uint) ([]types.TicketMessage, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/messages", id)

	var result []types.TicketMessage
	err := s.client.Get(ctx, path, &result)
	return result, err
}

// GetMessage retrieves a specific message
func (s *TicketsService) GetMessage(ctx context.Context, ticketID, messageID uint) (*types.TicketMessage, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/messages/%d", ticketID, messageID)

	var result types.TicketMessage
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// UpdateMessage updates a ticket message
func (s *TicketsService) UpdateMessage(ctx context.Context, ticketID, messageID uint, content string) (*types.TicketMessage, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/messages/%d", ticketID, messageID)

	body := map[string]string{"content": content}
	var result types.TicketMessage
	err := s.client.Put(ctx, path, body, &result)
	return &result, err
}

// DeleteMessage deletes a ticket message
func (s *TicketsService) DeleteMessage(ctx context.Context, ticketID, messageID uint) error {
	path := fmt.Sprintf("/api/v1/tickets/%d/messages/%d", ticketID, messageID)
	return s.client.Delete(ctx, path, nil)
}

// GetAttachments retrieves attachments for a ticket
func (s *TicketsService) GetAttachments(ctx context.Context, id uint) ([]types.Attachment, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/attachments", id)

	var result []types.Attachment
	err := s.client.Get(ctx, path, &result)
	return result, err
}

// GetAttachment retrieves a specific attachment
func (s *TicketsService) GetAttachment(ctx context.Context, ticketID, attachmentID uint) (*types.Attachment, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/attachments/%d", ticketID, attachmentID)

	var result types.Attachment
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// DownloadAttachment downloads an attachment's content
func (s *TicketsService) DownloadAttachment(ctx context.Context, ticketID, attachmentID uint) ([]byte, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/attachments/%d/download", ticketID, attachmentID)

	resp, err := s.client.httpClient.R().
		SetContext(ctx).
		Get(path)

	if err != nil {
		return nil, err
	}

	return resp.Body(), nil
}

// DeleteAttachment deletes an attachment
func (s *TicketsService) DeleteAttachment(ctx context.Context, ticketID, attachmentID uint) error {
	path := fmt.Sprintf("/api/v1/tickets/%d/attachments/%d", ticketID, attachmentID)
	return s.client.Delete(ctx, path, nil)
}

// Search searches tickets with advanced options
func (s *TicketsService) Search(ctx context.Context, query string, options *types.TicketListOptions) (*types.SearchResult, error) {
	path := "/api/v1/tickets/search"

	params := url.Values{}
	params.Set("q", query)

	if options != nil {
		if options.Page > 0 {
			params.Set("page", strconv.Itoa(options.Page))
		}
		if options.PageSize > 0 {
			params.Set("page_size", strconv.Itoa(options.PageSize))
		}
		if len(options.Status) > 0 {
			params.Set("status", strings.Join(options.Status, ","))
		}
		if len(options.Priority) > 0 {
			params.Set("priority", strings.Join(options.Priority, ","))
		}
		if len(options.QueueID) > 0 {
			queueIDs := make([]string, len(options.QueueID))
			for i, id := range options.QueueID {
				queueIDs[i] = strconv.FormatUint(uint64(id), 10)
			}
			params.Set("queue_id", strings.Join(queueIDs, ","))
		}
	}

	path += "?" + params.Encode()

	var result types.SearchResult
	err := s.client.Get(ctx, path, &result)
	return &result, err
}

// GetHistory retrieves ticket history
func (s *TicketsService) GetHistory(ctx context.Context, id uint) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/history", id)

	var result []map[string]interface{}
	err := s.client.Get(ctx, path, &result)
	return result, err
}

// AddTags adds tags to a ticket
func (s *TicketsService) AddTags(ctx context.Context, id uint, tags []string) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/tags", id)

	body := map[string][]string{"tags": tags}
	var result types.Ticket
	err := s.client.Post(ctx, path, body, &result)
	return &result, err
}

// RemoveTags removes tags from a ticket
func (s *TicketsService) RemoveTags(ctx context.Context, id uint, tags []string) (*types.Ticket, error) {
	path := fmt.Sprintf("/api/v1/tickets/%d/tags", id)

	body := map[string][]string{"tags": tags}
	err := s.client.Delete(ctx, path, body)
	if err != nil {
		return nil, err
	}

	// Get updated ticket
	return s.Get(ctx, id)
}
