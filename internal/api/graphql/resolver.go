//go:build graphql

package graphql

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/cache"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// Resolver is the root resolver
type Resolver struct {
	ticketService *service.TicketService
	userService   *service.UserService
	queueService  *service.QueueService
	authService   *auth.Service
	cacheManager  *cache.Manager
	repos         *repository.Repositories
}

// NewResolver creates a new GraphQL resolver
func NewResolver(
	ticketService *service.TicketService,
	userService *service.UserService,
	queueService *service.QueueService,
	authService *auth.Service,
	cacheManager *cache.Manager,
	repos *repository.Repositories,
) *Resolver {
	return &Resolver{
		ticketService: ticketService,
		userService:   userService,
		queueService:  queueService,
		authService:   authService,
		cacheManager:  cacheManager,
		repos:         repos,
	}
}

// Query returns the query resolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// Mutation returns the mutation resolver
func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}

// Subscription returns the subscription resolver
func (r *Resolver) Subscription() SubscriptionResolver {
	return &subscriptionResolver{r}
}

// queryResolver handles all query operations
type queryResolver struct{ *Resolver }

// Me returns the current user
func (r *queryResolver) Me(ctx context.Context) (*User, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	
	// Check cache first
	cached, err := r.cacheManager.GetUser(ctx, userID)
	if err == nil && cached != nil {
		return mapCachedUserToGraphQL(cached), nil
	}
	
	// Load from database
	user, err := r.userService.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	r.cacheManager.SetUser(ctx, mapUserToCached(user))
	
	return mapUserToGraphQL(user), nil
}

// User returns a user by ID
func (r *queryResolver) User(ctx context.Context, id string) (*User, error) {
	userID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}
	
	user, err := r.userService.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	return mapUserToGraphQL(user), nil
}

// Users returns a list of users
func (r *queryResolver) Users(ctx context.Context, search *string, role *string, isActive *bool) ([]*User, error) {
	filters := map[string]interface{}{}
	if search != nil {
		filters["search"] = *search
	}
	if role != nil {
		filters["role"] = *role
	}
	if isActive != nil {
		filters["is_active"] = *isActive
	}
	
	users, err := r.userService.ListUsers(ctx, filters)
	if err != nil {
		return nil, err
	}
	
	result := make([]*User, len(users))
	for i, user := range users {
		result[i] = mapUserToGraphQL(user)
	}
	
	return result, nil
}

// Ticket returns a ticket by ID
func (r *queryResolver) Ticket(ctx context.Context, id string) (*Ticket, error) {
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket ID")
	}
	
	// Check cache first
	cached, err := r.cacheManager.GetTicket(ctx, ticketID)
	if err == nil && cached != nil {
		return mapCachedTicketToGraphQL(cached), nil
	}
	
	ticket, err := r.ticketService.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	r.cacheManager.SetTicket(ctx, mapTicketToCached(ticket))
	
	return mapTicketToGraphQL(ticket), nil
}

// TicketByNumber returns a ticket by number
func (r *queryResolver) TicketByNumber(ctx context.Context, number string) (*Ticket, error) {
	ticket, err := r.ticketService.GetTicketByNumber(ctx, number)
	if err != nil {
		return nil, err
	}
	
	return mapTicketToGraphQL(ticket), nil
}

// Tickets returns a paginated list of tickets
func (r *queryResolver) Tickets(ctx context.Context, filter *TicketFilterInput, pagination *PaginationInput) (*TicketConnection, error) {
	// Convert filter
	filters := convertTicketFilter(filter)
	
	// Convert pagination
	page, limit := convertPagination(pagination)
	
	// Get tickets
	tickets, total, err := r.ticketService.ListTickets(ctx, filters, page, limit)
	if err != nil {
		return nil, err
	}
	
	// Build connection
	return buildTicketConnection(tickets, total, page, limit), nil
}

// MyTickets returns tickets for the current user
func (r *queryResolver) MyTickets(ctx context.Context, filter *TicketFilterInput, pagination *PaginationInput) (*TicketConnection, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	
	// Add user filter
	filters := convertTicketFilter(filter)
	filters["assignee_id"] = userID
	
	page, limit := convertPagination(pagination)
	
	tickets, total, err := r.ticketService.ListTickets(ctx, filters, page, limit)
	if err != nil {
		return nil, err
	}
	
	return buildTicketConnection(tickets, total, page, limit), nil
}

// Queue returns a queue by ID
func (r *queryResolver) Queue(ctx context.Context, id string) (*Queue, error) {
	queueID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid queue ID")
	}
	
	queue, err := r.queueService.GetQueueByID(ctx, queueID)
	if err != nil {
		return nil, err
	}
	
	return mapQueueToGraphQL(queue), nil
}

// Queues returns a list of queues
func (r *queryResolver) Queues(ctx context.Context, isActive *bool) ([]*Queue, error) {
	filters := map[string]interface{}{}
	if isActive != nil {
		filters["is_active"] = *isActive
	}
	
	queues, err := r.queueService.ListQueues(ctx, filters)
	if err != nil {
		return nil, err
	}
	
	result := make([]*Queue, len(queues))
	for i, queue := range queues {
		result[i] = mapQueueToGraphQL(queue)
	}
	
	return result, nil
}

// QueueStatistics returns statistics for a queue
func (r *queryResolver) QueueStatistics(ctx context.Context, queueID string) (*QueueStatistics, error) {
	id, err := strconv.Atoi(queueID)
	if err != nil {
		return nil, fmt.Errorf("invalid queue ID")
	}
	
	stats, err := r.queueService.GetQueueStatistics(ctx, id)
	if err != nil {
		return nil, err
	}
	
	return mapQueueStatisticsToGraphQL(stats), nil
}

// Organization returns an organization by ID
func (r *queryResolver) Organization(ctx context.Context, id string) (*Organization, error) {
	orgID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID")
	}
	
	org, err := r.repos.Organization.GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	
	return mapOrganizationToGraphQL(org), nil
}

// Organizations returns a list of organizations
func (r *queryResolver) Organizations(ctx context.Context, search *string) ([]*Organization, error) {
	filters := map[string]interface{}{}
	if search != nil {
		filters["search"] = *search
	}
	
	orgs, err := r.repos.Organization.List(ctx, filters, 1, 100)
	if err != nil {
		return nil, err
	}
	
	result := make([]*Organization, len(orgs))
	for i, org := range orgs {
		result[i] = mapOrganizationToGraphQL(org)
	}
	
	return result, nil
}

// TicketStatistics returns ticket statistics
func (r *queryResolver) TicketStatistics(ctx context.Context, filter *TicketFilterInput) (*TicketStatistics, error) {
	filters := convertTicketFilter(filter)
	stats, err := r.ticketService.GetTicketStatistics(ctx, filters)
	if err != nil {
		return nil, err
	}
	
	return mapTicketStatisticsToGraphQL(stats), nil
}

// DashboardStatistics returns dashboard statistics
func (r *queryResolver) DashboardStatistics(ctx context.Context) (*DashboardStatistics, error) {
	stats, err := r.ticketService.GetDashboardStatistics(ctx)
	if err != nil {
		return nil, err
	}
	
	return mapDashboardStatisticsToGraphQL(stats), nil
}

// Search performs a global search
func (r *queryResolver) Search(ctx context.Context, query string, types []string) (*SearchResult, error) {
	results, err := r.ticketService.Search(ctx, query, types)
	if err != nil {
		return nil, err
	}
	
	return mapSearchResultsToGraphQL(results), nil
}

// mutationResolver handles all mutation operations
type mutationResolver struct{ *Resolver }

// Login authenticates a user
func (r *mutationResolver) Login(ctx context.Context, email string, password string) (*AuthPayload, error) {
	token, refreshToken, user, err := r.authService.Login(ctx, email, password)
	if err != nil {
		return nil, err
	}
	
	return &AuthPayload{
		Token:        token,
		RefreshToken: refreshToken,
		User:         mapUserToGraphQL(user),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}, nil
}

// Logout logs out the current user
func (r *mutationResolver) Logout(ctx context.Context) (bool, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		return false, fmt.Errorf("unauthorized")
	}
	
	err := r.authService.Logout(ctx, userID)
	return err == nil, err
}

// RefreshToken refreshes an authentication token
func (r *mutationResolver) RefreshToken(ctx context.Context, token string) (*AuthPayload, error) {
	newToken, newRefreshToken, user, err := r.authService.RefreshToken(ctx, token)
	if err != nil {
		return nil, err
	}
	
	return &AuthPayload{
		Token:        newToken,
		RefreshToken: newRefreshToken,
		User:         mapUserToGraphQL(user),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}, nil
}

// CreateTicket creates a new ticket
func (r *mutationResolver) CreateTicket(ctx context.Context, input CreateTicketInput) (*Ticket, error) {
	ticket := &models.Ticket{
		Title:       input.Subject,
		Description: input.Description,
		QueueID:     parseID(input.QueueID),
		PriorityID:  mapPriorityToID(input.Priority),
	}
	
	if input.CustomerID != nil {
		ticket.CustomerUserID = parseID(*input.CustomerID)
	}
	
	created, err := r.ticketService.CreateTicket(ctx, ticket)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, "queue:*")
	
	return mapTicketToGraphQL(created), nil
}

// UpdateTicket updates an existing ticket
func (r *mutationResolver) UpdateTicket(ctx context.Context, id string, input UpdateTicketInput) (*Ticket, error) {
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket ID")
	}
	
	updates := make(map[string]interface{})
	if input.Subject != nil {
		updates["title"] = *input.Subject
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Status != nil {
		updates["ticket_state_id"] = mapStatusToID(*input.Status)
	}
	if input.Priority != nil {
		updates["ticket_priority_id"] = mapPriorityToID(input.Priority)
	}
	if input.QueueID != nil {
		updates["queue_id"] = parseID(*input.QueueID)
	}
	if input.AssigneeID != nil {
		updates["user_id"] = parseID(*input.AssigneeID)
	}
	
	updated, err := r.ticketService.UpdateTicket(ctx, ticketID, updates)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidateTicket(ctx, ticketID)
	
	return mapTicketToGraphQL(updated), nil
}

// AssignTicket assigns a ticket to a user
func (r *mutationResolver) AssignTicket(ctx context.Context, id string, userID string) (*Ticket, error) {
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket ID")
	}
	
	assigneeID := parseID(userID)
	
	updated, err := r.ticketService.AssignTicket(ctx, ticketID, assigneeID)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidateTicket(ctx, ticketID)
	
	return mapTicketToGraphQL(updated), nil
}

// CloseTicket closes a ticket
func (r *mutationResolver) CloseTicket(ctx context.Context, id string, resolution *string) (*Ticket, error) {
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket ID")
	}
	
	updated, err := r.ticketService.CloseTicket(ctx, ticketID, resolution)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidateTicket(ctx, ticketID)
	
	return mapTicketToGraphQL(updated), nil
}

// MergeTickets merges two tickets
func (r *mutationResolver) MergeTickets(ctx context.Context, sourceID string, targetID string) (*Ticket, error) {
	srcID, err := strconv.ParseInt(sourceID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid source ticket ID")
	}
	
	tgtID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid target ticket ID")
	}
	
	merged, err := r.ticketService.MergeTickets(ctx, srcID, tgtID)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache for both tickets
	r.cacheManager.InvalidateTicket(ctx, srcID)
	r.cacheManager.InvalidateTicket(ctx, tgtID)
	
	return mapTicketToGraphQL(merged), nil
}

// SplitTicket splits a ticket
func (r *mutationResolver) SplitTicket(ctx context.Context, id string, articleIDs []string) (*Ticket, error) {
	ticketID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket ID")
	}
	
	artIDs := make([]int64, len(articleIDs))
	for i, aid := range articleIDs {
		artIDs[i], _ = strconv.ParseInt(aid, 10, 64)
	}
	
	newTicket, err := r.ticketService.SplitTicket(ctx, ticketID, artIDs)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidateTicket(ctx, ticketID)
	
	return mapTicketToGraphQL(newTicket), nil
}

// CreateArticle creates a new article
func (r *mutationResolver) CreateArticle(ctx context.Context, input CreateArticleInput) (*Article, error) {
	ticketID, err := strconv.ParseInt(input.TicketID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket ID")
	}
	
	article := &models.Article{
		TicketID: int(ticketID),
		Subject:  input.Subject,
		Body:     input.Body,
	}
	
	if input.Internal != nil {
		article.Internal = *input.Internal
	}
	
	created, err := r.repos.Article.Create(ctx, article)
	if err != nil {
		return nil, err
	}
	
	// Invalidate ticket cache
	r.cacheManager.InvalidateTicket(ctx, ticketID)
	
	return mapArticleToGraphQL(created), nil
}

// UpdateArticle updates an article
func (r *mutationResolver) UpdateArticle(ctx context.Context, id string, body string) (*Article, error) {
	articleID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid article ID")
	}
	
	article, err := r.repos.Article.GetByID(ctx, articleID)
	if err != nil {
		return nil, err
	}
	
	article.Body = body
	updated, err := r.repos.Article.Update(ctx, article)
	if err != nil {
		return nil, err
	}
	
	// Invalidate ticket cache
	r.cacheManager.InvalidateTicket(ctx, int64(article.TicketID))
	
	return mapArticleToGraphQL(updated), nil
}

// DeleteArticle deletes an article
func (r *mutationResolver) DeleteArticle(ctx context.Context, id string) (bool, error) {
	articleID, err := strconv.Atoi(id)
	if err != nil {
		return false, fmt.Errorf("invalid article ID")
	}
	
	article, err := r.repos.Article.GetByID(ctx, articleID)
	if err != nil {
		return false, err
	}
	
	err = r.repos.Article.Delete(ctx, articleID)
	if err != nil {
		return false, err
	}
	
	// Invalidate ticket cache
	r.cacheManager.InvalidateTicket(ctx, int64(article.TicketID))
	
	return true, nil
}

// UploadAttachment uploads a file attachment
func (r *mutationResolver) UploadAttachment(ctx context.Context, ticketID string, file graphql.Upload) (*Attachment, error) {
	// Implementation would handle file upload
	// For now, return a placeholder
	return &Attachment{
		ID:          "1",
		Filename:    file.Filename,
		ContentType: file.ContentType,
		Size:        int(file.Size),
		URL:         fmt.Sprintf("/attachments/%s", file.Filename),
		CreatedAt:   time.Now(),
	}, nil
}

// DeleteAttachment deletes an attachment
func (r *mutationResolver) DeleteAttachment(ctx context.Context, id string) (bool, error) {
	// Implementation would delete the attachment
	return true, nil
}

// CreateUser creates a new user
func (r *mutationResolver) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	user := &models.User{
		Login:    input.Email,
		Email:    input.Email,
		Realname: input.Name,
	}
	
	created, err := r.userService.CreateUser(ctx, user, input.Password)
	if err != nil {
		return nil, err
	}
	
	return mapUserToGraphQL(created), nil
}

// UpdateUser updates a user
func (r *mutationResolver) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error) {
	userID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}
	
	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["realname"] = *input.Name
	}
	if input.Email != nil {
		updates["email"] = *input.Email
	}
	if input.IsActive != nil {
		updates["valid_id"] = 1
		if !*input.IsActive {
			updates["valid_id"] = 2
		}
	}
	
	updated, err := r.userService.UpdateUser(ctx, userID, updates)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, fmt.Sprintf("user:%d", userID))
	
	return mapUserToGraphQL(updated), nil
}

// DeleteUser deletes a user
func (r *mutationResolver) DeleteUser(ctx context.Context, id string) (bool, error) {
	userID, err := strconv.Atoi(id)
	if err != nil {
		return false, fmt.Errorf("invalid user ID")
	}
	
	err = r.userService.DeleteUser(ctx, userID)
	if err != nil {
		return false, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, fmt.Sprintf("user:%d", userID))
	
	return true, nil
}

// UpdateProfile updates the current user's profile
func (r *mutationResolver) UpdateProfile(ctx context.Context, input UpdateUserInput) (*User, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	
	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["realname"] = *input.Name
	}
	if input.Email != nil {
		updates["email"] = *input.Email
	}
	
	updated, err := r.userService.UpdateUser(ctx, userID, updates)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, fmt.Sprintf("user:%d", userID))
	
	return mapUserToGraphQL(updated), nil
}

// UpdatePreferences updates user preferences
func (r *mutationResolver) UpdatePreferences(ctx context.Context, preferences map[string]interface{}) (*User, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	
	err := r.userService.UpdatePreferences(ctx, userID, preferences)
	if err != nil {
		return nil, err
	}
	
	user, err := r.userService.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, fmt.Sprintf("user:%d", userID))
	
	return mapUserToGraphQL(user), nil
}

// CreateQueue creates a new queue
func (r *mutationResolver) CreateQueue(ctx context.Context, name string, description *string) (*Queue, error) {
	queue := &models.Queue{
		Name: name,
	}
	
	if description != nil {
		queue.Comments = description
	}
	
	created, err := r.queueService.CreateQueue(ctx, queue)
	if err != nil {
		return nil, err
	}
	
	return mapQueueToGraphQL(created), nil
}

// UpdateQueue updates a queue
func (r *mutationResolver) UpdateQueue(ctx context.Context, id string, name *string, description *string, isActive *bool) (*Queue, error) {
	queueID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid queue ID")
	}
	
	updates := make(map[string]interface{})
	if name != nil {
		updates["name"] = *name
	}
	if description != nil {
		updates["comments"] = *description
	}
	if isActive != nil {
		updates["valid_id"] = 1
		if !*isActive {
			updates["valid_id"] = 2
		}
	}
	
	updated, err := r.queueService.UpdateQueue(ctx, queueID, updates)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, "queue:*")
	
	return mapQueueToGraphQL(updated), nil
}

// AssignAgentToQueue assigns an agent to a queue
func (r *mutationResolver) AssignAgentToQueue(ctx context.Context, queueID string, userID string) (*Queue, error) {
	qID, err := strconv.Atoi(queueID)
	if err != nil {
		return nil, fmt.Errorf("invalid queue ID")
	}
	
	uID, err := strconv.Atoi(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}
	
	err = r.queueService.AssignAgent(ctx, qID, uID)
	if err != nil {
		return nil, err
	}
	
	queue, err := r.queueService.GetQueueByID(ctx, qID)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, fmt.Sprintf("queue:%d:*", qID))
	
	return mapQueueToGraphQL(queue), nil
}

// RemoveAgentFromQueue removes an agent from a queue
func (r *mutationResolver) RemoveAgentFromQueue(ctx context.Context, queueID string, userID string) (*Queue, error) {
	qID, err := strconv.Atoi(queueID)
	if err != nil {
		return nil, fmt.Errorf("invalid queue ID")
	}
	
	uID, err := strconv.Atoi(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}
	
	err = r.queueService.RemoveAgent(ctx, qID, uID)
	if err != nil {
		return nil, err
	}
	
	queue, err := r.queueService.GetQueueByID(ctx, qID)
	if err != nil {
		return nil, err
	}
	
	// Invalidate cache
	r.cacheManager.InvalidatePattern(ctx, fmt.Sprintf("queue:%d:*", qID))
	
	return mapQueueToGraphQL(queue), nil
}

// CreateOrganization creates a new organization
func (r *mutationResolver) CreateOrganization(ctx context.Context, name string, domain *string) (*Organization, error) {
	org := &models.Organization{
		Name: name,
	}
	
	if domain != nil {
		org.Domain = *domain
	}
	
	created, err := r.repos.Organization.Create(ctx, org)
	if err != nil {
		return nil, err
	}
	
	return mapOrganizationToGraphQL(created), nil
}

// UpdateOrganization updates an organization
func (r *mutationResolver) UpdateOrganization(ctx context.Context, id string, name *string, domain *string) (*Organization, error) {
	orgID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID")
	}
	
	org, err := r.repos.Organization.GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	
	if name != nil {
		org.Name = *name
	}
	if domain != nil {
		org.Domain = *domain
	}
	
	updated, err := r.repos.Organization.Update(ctx, org)
	if err != nil {
		return nil, err
	}
	
	return mapOrganizationToGraphQL(updated), nil
}

// DeleteOrganization deletes an organization
func (r *mutationResolver) DeleteOrganization(ctx context.Context, id string) (bool, error) {
	orgID, err := strconv.Atoi(id)
	if err != nil {
		return false, fmt.Errorf("invalid organization ID")
	}
	
	err = r.repos.Organization.Delete(ctx, orgID)
	return err == nil, err
}

// subscriptionResolver handles all subscription operations
type subscriptionResolver struct{ *Resolver }

// TicketUpdated subscribes to ticket updates
func (r *subscriptionResolver) TicketUpdated(ctx context.Context, ticketID *string) (<-chan *TicketUpdate, error) {
	// Implementation would use WebSocket or SSE
	ch := make(chan *TicketUpdate)
	
	// Start goroutine to send updates
	go func() {
		// Subscribe to events
		// Send updates to channel
	}()
	
	return ch, nil
}

// TicketCreated subscribes to new tickets
func (r *subscriptionResolver) TicketCreated(ctx context.Context, queueID *string) (<-chan *Ticket, error) {
	ch := make(chan *Ticket)
	
	// Implementation would subscribe to ticket creation events
	
	return ch, nil
}

// TicketAssigned subscribes to ticket assignments
func (r *subscriptionResolver) TicketAssigned(ctx context.Context, userID *string) (<-chan *Ticket, error) {
	ch := make(chan *Ticket)
	
	// Implementation would subscribe to assignment events
	
	return ch, nil
}

// QueueUpdated subscribes to queue updates
func (r *subscriptionResolver) QueueUpdated(ctx context.Context, queueID string) (<-chan *Queue, error) {
	ch := make(chan *Queue)
	
	// Implementation would subscribe to queue events
	
	return ch, nil
}

// UserStatusChanged subscribes to user status changes
func (r *subscriptionResolver) UserStatusChanged(ctx context.Context, userID string) (<-chan *User, error) {
	ch := make(chan *User)
	
	// Implementation would subscribe to user events
	
	return ch, nil
}

// SystemNotification subscribes to system notifications
func (r *subscriptionResolver) SystemNotification(ctx context.Context) (<-chan *SystemNotification, error) {
	ch := make(chan *SystemNotification)
	
	// Implementation would subscribe to system events
	
	return ch, nil
}

// Helper functions

func parseID(id string) int {
	val, _ := strconv.Atoi(id)
	return val
}

func mapPriorityToID(priority *string) int {
	if priority == nil {
		return 3 // Default to MEDIUM
	}
	
	switch *priority {
	case "LOW":
		return 1
	case "MEDIUM":
		return 3
	case "HIGH":
		return 4
	case "URGENT":
		return 5
	case "CRITICAL":
		return 5
	default:
		return 3
	}
}

func mapStatusToID(status string) int {
	switch status {
	case "NEW":
		return 1
	case "OPEN":
		return 2
	case "PENDING":
		return 3
	case "RESOLVED":
		return 4
	case "CLOSED":
		return 5
	case "MERGED":
		return 6
	default:
		return 1
	}
}

func convertTicketFilter(filter *TicketFilterInput) map[string]interface{} {
	if filter == nil {
		return map[string]interface{}{}
	}
	
	result := make(map[string]interface{})
	
	if filter.Status != nil && len(filter.Status) > 0 {
		statuses := make([]int, len(filter.Status))
		for i, s := range filter.Status {
			statuses[i] = mapStatusToID(s)
		}
		result["status"] = statuses
	}
	
	if filter.Priority != nil && len(filter.Priority) > 0 {
		priorities := make([]int, len(filter.Priority))
		for i, p := range filter.Priority {
			priorities[i] = mapPriorityToID(&p)
		}
		result["priority"] = priorities
	}
	
	if filter.QueueID != nil && len(filter.QueueID) > 0 {
		queueIDs := make([]int, len(filter.QueueID))
		for i, id := range filter.QueueID {
			queueIDs[i] = parseID(id)
		}
		result["queue_id"] = queueIDs
	}
	
	if filter.AssigneeID != nil {
		result["assignee_id"] = parseID(*filter.AssigneeID)
	}
	
	if filter.CustomerID != nil {
		result["customer_id"] = parseID(*filter.CustomerID)
	}
	
	if filter.OrganizationID != nil {
		result["organization_id"] = parseID(*filter.OrganizationID)
	}
	
	if filter.Tags != nil && len(filter.Tags) > 0 {
		result["tags"] = filter.Tags
	}
	
	if filter.Search != nil {
		result["search"] = *filter.Search
	}
	
	if filter.CreatedAfter != nil {
		result["created_after"] = *filter.CreatedAfter
	}
	
	if filter.CreatedBefore != nil {
		result["created_before"] = *filter.CreatedBefore
	}
	
	return result
}

func convertPagination(pagination *PaginationInput) (int, int) {
	page := 1
	limit := 20
	
	if pagination != nil {
		if pagination.Page != nil {
			page = *pagination.Page
		}
		if pagination.Limit != nil {
			limit = *pagination.Limit
		}
	}
	
	return page, limit
}

func buildTicketConnection(tickets []*models.Ticket, total int, page, limit int) *TicketConnection {
	edges := make([]*TicketEdge, len(tickets))
	for i, ticket := range tickets {
		edges[i] = &TicketEdge{
			Node:   mapTicketToGraphQL(ticket),
			Cursor: fmt.Sprintf("%d", ticket.ID),
		}
	}
	
	hasNext := page*limit < total
	hasPrev := page > 1
	
	var startCursor, endCursor *string
	if len(edges) > 0 {
		sc := edges[0].Cursor
		startCursor = &sc
		ec := edges[len(edges)-1].Cursor
		endCursor = &ec
	}
	
	return &TicketConnection{
		Edges: edges,
		PageInfo: &PageInfo{
			HasNextPage:     hasNext,
			HasPreviousPage: hasPrev,
			StartCursor:     startCursor,
			EndCursor:       endCursor,
		},
		TotalCount: total,
	}
}