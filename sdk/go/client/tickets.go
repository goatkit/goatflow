package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/goatkit/goatflow/sdk/go/types"
)

// TicketsService covers /api/v1/tickets.
type TicketsService struct {
	client *Client
}

// List returns one page of tickets the caller can read.
func (s *TicketsService) List(ctx context.Context, options *types.TicketListOptions) (*types.TicketList, error) {
	query := url.Values{}
	if o := options; o != nil {
		setInt(query, "page", o.Page)
		setInt(query, "per_page", o.PerPage)
		setString(query, "status", o.Status)
		setUint(query, "queue_id", o.QueueID)
		setUint(query, "priority_id", o.PriorityID)
		setString(query, "customer_user_id", o.CustomerUserID)
		setUint(query, "assigned_user_id", o.AssignedUserID)
		setString(query, "search", o.Search)
		setString(query, "sort", o.Sort)
		setString(query, "order", o.Order)
		setString(query, "include", strings.Join(o.Include, ","))
	}
	list := &types.TicketList{}
	pagination, err := s.client.do(ctx, http.MethodGet, "/api/v1/tickets", query, nil, &list.Tickets)
	if err != nil {
		return nil, err
	}
	if pagination != nil {
		list.Pagination = *pagination
	}
	return list, nil
}

// Get returns one ticket.
func (s *TicketsService) Get(ctx context.Context, id uint) (*types.Ticket, error) {
	var ticket types.Ticket
	if _, err := s.client.do(ctx, http.MethodGet, ticketPath(id), nil, nil, &ticket); err != nil {
		return nil, err
	}
	return &ticket, nil
}

// Create creates a ticket.
func (s *TicketsService) Create(ctx context.Context, request *types.TicketCreateRequest) (*types.CreatedTicket, error) {
	var created types.CreatedTicket
	if _, err := s.client.do(ctx, http.MethodPost, "/api/v1/tickets", nil, request, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// Update changes the non-nil fields of request and returns the updated row.
func (s *TicketsService) Update(ctx context.Context, id uint, request *types.TicketUpdateRequest) (*types.TicketRecord, error) {
	var record types.TicketRecord
	if _, err := s.client.do(ctx, http.MethodPut, ticketPath(id), nil, request, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// Delete deletes a ticket.
func (s *TicketsService) Delete(ctx context.Context, id uint) error {
	_, err := s.client.do(ctx, http.MethodDelete, ticketPath(id), nil, nil, nil)
	return err
}

// Reopen moves a closed ticket back to "open" and records the reason as an
// article.
func (s *TicketsService) Reopen(ctx context.Context, id uint, reason string) (*types.ReopenResult, error) {
	var result types.ReopenResult
	body := map[string]string{"reason": reason}
	if _, err := s.client.do(ctx, http.MethodPost, ticketPath(id)+"/reopen", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ArticlesService covers /api/v1/tickets/:id/articles.
type ArticlesService struct {
	client *Client
}

// List returns a ticket's articles, newest first. includeAttachments adds
// each article's attachment list.
func (s *ArticlesService) List(ctx context.Context, ticketID uint, includeAttachments bool) (*types.ArticleList, error) {
	var list types.ArticleList
	if _, err := s.client.do(ctx, http.MethodGet, ticketPath(ticketID)+"/articles", attachmentsQuery(includeAttachments), nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// Get returns one article of a ticket.
func (s *ArticlesService) Get(ctx context.Context, ticketID, articleID uint, includeAttachments bool) (*types.Article, error) {
	var article types.Article
	if _, err := s.client.do(ctx, http.MethodGet, articlePath(ticketID, articleID), attachmentsQuery(includeAttachments), nil, &article); err != nil {
		return nil, err
	}
	return &article, nil
}

// Create adds an article to a ticket.
func (s *ArticlesService) Create(ctx context.Context, ticketID uint, request *types.ArticleCreateRequest) (*types.Article, error) {
	var article types.Article
	if _, err := s.client.do(ctx, http.MethodPost, ticketPath(ticketID)+"/articles", nil, request, &article); err != nil {
		return nil, err
	}
	return &article, nil
}

// Update changes an article's subject and/or body.
func (s *ArticlesService) Update(ctx context.Context, ticketID, articleID uint, request *types.ArticleUpdateRequest) (*types.ArticleUpdate, error) {
	var updated types.ArticleUpdate
	if _, err := s.client.do(ctx, http.MethodPut, articlePath(ticketID, articleID), nil, request, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// Delete deletes an article and its attachments.
func (s *ArticlesService) Delete(ctx context.Context, ticketID, articleID uint) error {
	_, err := s.client.do(ctx, http.MethodDelete, articlePath(ticketID, articleID), nil, nil, nil)
	return err
}

func ticketPath(id uint) string { return fmt.Sprintf("/api/v1/tickets/%d", id) }

func articlePath(ticketID, articleID uint) string {
	return fmt.Sprintf("/api/v1/tickets/%d/articles/%d", ticketID, articleID)
}

func attachmentsQuery(include bool) url.Values {
	if !include {
		return nil
	}
	return url.Values{"include_attachments": {"true"}}
}

func setString(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setInt(q url.Values, key string, value int) {
	if value != 0 {
		q.Set(key, strconv.Itoa(value))
	}
}

func setUint(q url.Values, key string, value uint) {
	if value != 0 {
		q.Set(key, strconv.FormatUint(uint64(value), 10))
	}
}
