package graphql

import (
	"context"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// Resolver is the root GraphQL resolver
type Resolver struct {
	userService     *service.UserService
	ticketService   *service.TicketService
	queueService    *service.QueueService
	workflowService *service.WorkflowService
	authService     *service.AuthService
	searchService   *service.SearchService
	reportService   *service.ReportService
}

// NewResolver creates a new GraphQL resolver
func NewResolver(
	userService *service.UserService,
	ticketService *service.TicketService,
	queueService *service.QueueService,
	workflowService *service.WorkflowService,
	authService *service.AuthService,
	searchService *service.SearchService,
	reportService *service.ReportService,
) *Resolver {
	return &Resolver{
		userService:     userService,
		ticketService:   ticketService,
		queueService:    queueService,
		workflowService: workflowService,
		authService:     authService,
		searchService:   searchService,
		reportService:   reportService,
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

type queryResolver struct{ *Resolver }

// Me returns the current user
func (r *queryResolver) Me(ctx context.Context) (*User, error) {
	userID := getUserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	
	user, err := r.userService.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	
	return modelToGraphQLUser(user), nil
}

// User returns a user by ID
func (r *queryResolver) User(ctx context.Context, id string) (*User, error) {
	userID := parseID(id)
	user, err := r.userService.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	
	return modelToGraphQLUser(user), nil
}

// Users returns a list of users
func (r *queryResolver) Users(ctx context.Context, filter *UserFilter, pagination *Pagination) (*UserConnection, error) {
	// Apply filters and pagination
	users, total, err := r.userService.ListUsers(convertUserFilter(filter), convertPagination(pagination))
	if err != nil {
		return nil, err
	}
	
	return &UserConnection{
		Edges:      convertUserEdges(users),
		PageInfo:   createPageInfo(pagination, total),
		TotalCount: total,
	}, nil
}

// Ticket returns a ticket by ID
func (r *queryResolver) Ticket(ctx context.Context, id string) (*Ticket, error) {
	ticketID := parseID(id)
	ticket, err := r.ticketService.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	
	return modelToGraphQLTicket(ticket), nil
}

// Tickets returns a list of tickets
func (r *queryResolver) Tickets(ctx context.Context, filter *TicketFilter, pagination *Pagination) (*TicketConnection, error) {
	tickets, total, err := r.ticketService.ListTickets(convertTicketFilter(filter), convertPagination(pagination))
	if err != nil {
		return nil, err
	}
	
	return &TicketConnection{
		Edges:      convertTicketEdges(tickets),
		PageInfo:   createPageInfo(pagination, total),
		TotalCount: total,
	}, nil
}

// MyTickets returns tickets for the current user
func (r *queryResolver) MyTickets(ctx context.Context, status *TicketStatus, pagination *Pagination) (*TicketConnection, error) {
	userID := getUserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	
	filter := &TicketFilter{
		AssigneeID: &userID,
		Status:     status,
	}
	
	return r.Tickets(ctx, filter, pagination)
}

// Queue returns a queue by ID
func (r *queryResolver) Queue(ctx context.Context, id string) (*Queue, error) {
	queueID := parseID(id)
	queue, err := r.queueService.GetQueueByID(queueID)
	if err != nil {
		return nil, err
	}
	
	return modelToGraphQLQueue(queue), nil
}

// Queues returns a list of queues
func (r *queryResolver) Queues(ctx context.Context, filter *QueueFilter, pagination *Pagination) (*QueueConnection, error) {
	queues, total, err := r.queueService.ListQueues(convertQueueFilter(filter), convertPagination(pagination))
	if err != nil {
		return nil, err
	}
	
	return &QueueConnection{
		Edges:      convertQueueEdges(queues),
		PageInfo:   createPageInfo(pagination, total),
		TotalCount: total,
	}, nil
}

// Workflow returns a workflow by ID
func (r *queryResolver) Workflow(ctx context.Context, id string) (*Workflow, error) {
	workflowID := parseID(id)
	workflow, err := r.workflowService.GetWorkflowByID(workflowID)
	if err != nil {
		return nil, err
	}
	
	return modelToGraphQLWorkflow(workflow), nil
}

// Workflows returns a list of workflows
func (r *queryResolver) Workflows(ctx context.Context, filter *WorkflowFilter, pagination *Pagination) (*WorkflowConnection, error) {
	workflows, total, err := r.workflowService.ListWorkflows(convertWorkflowFilter(filter), convertPagination(pagination))
	if err != nil {
		return nil, err
	}
	
	return &WorkflowConnection{
		Edges:      convertWorkflowEdges(workflows),
		PageInfo:   createPageInfo(pagination, total),
		TotalCount: total,
	}, nil
}

// Search performs a search across resources
func (r *queryResolver) Search(ctx context.Context, query string, searchType *SearchType) (*SearchResults, error) {
	results, err := r.searchService.Search(query, convertSearchType(searchType))
	if err != nil {
		return nil, err
	}
	
	return convertSearchResults(results), nil
}

// DashboardStats returns dashboard statistics
func (r *queryResolver) DashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats, err := r.reportService.GetDashboardStats()
	if err != nil {
		return nil, err
	}
	
	return convertDashboardStats(stats), nil
}

type mutationResolver struct{ *Resolver }

// Login authenticates a user
func (r *mutationResolver) Login(ctx context.Context, email string, password string) (*AuthPayload, error) {
	token, refreshToken, user, err := r.authService.Login(email, password)
	if err != nil {
		return nil, err
	}
	
	return &AuthPayload{
		Token:        token,
		RefreshToken: refreshToken,
		User:         modelToGraphQLUser(user),
	}, nil
}

// CreateTicket creates a new ticket
func (r *mutationResolver) CreateTicket(ctx context.Context, input CreateTicketInput) (*Ticket, error) {
	userID := getUserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	
	ticket := &models.Ticket{
		Subject:          input.Subject,
		Description:      input.Description,
		TicketPriorityID: convertPriority(input.Priority),
		QueueID:          parseID(input.QueueID),
		CustomerID:       parseID(input.CustomerID),
		CreatedBy:        userID,
	}
	
	if err := r.ticketService.CreateTicket(ticket); err != nil {
		return nil, err
	}
	
	// Handle attachments if provided
	if len(input.Attachments) > 0 {
		for _, upload := range input.Attachments {
			if err := r.handleFileUpload(ctx, ticket.ID, upload); err != nil {
				return nil, err
			}
		}
	}
	
	return modelToGraphQLTicket(ticket), nil
}

// UpdateTicket updates an existing ticket
func (r *mutationResolver) UpdateTicket(ctx context.Context, id string, input UpdateTicketInput) (*Ticket, error) {
	ticketID := parseID(id)
	
	ticket, err := r.ticketService.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	
	// Apply updates
	if input.Subject != nil {
		ticket.Subject = *input.Subject
	}
	if input.Description != nil {
		ticket.Description = *input.Description
	}
	if input.Status != nil {
		ticket.TicketStateID = convertStatus(*input.Status)
	}
	if input.Priority != nil {
		ticket.TicketPriorityID = convertPriority(input.Priority)
	}
	if input.QueueID != nil {
		ticket.QueueID = parseID(*input.QueueID)
	}
	if input.AssigneeID != nil {
		ticket.UserID = parseID(*input.AssigneeID)
	}
	
	if err := r.ticketService.UpdateTicket(ticket); err != nil {
		return nil, err
	}
	
	return modelToGraphQLTicket(ticket), nil
}

// AssignTicket assigns a ticket to a user
func (r *mutationResolver) AssignTicket(ctx context.Context, id string, userID string) (*Ticket, error) {
	ticketID := parseID(id)
	assigneeID := parseID(userID)
	
	if err := r.ticketService.AssignTicket(ticketID, assigneeID); err != nil {
		return nil, err
	}
	
	ticket, err := r.ticketService.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	
	return modelToGraphQLTicket(ticket), nil
}

// CloseTicket closes a ticket
func (r *mutationResolver) CloseTicket(ctx context.Context, id string, resolution string) (*Ticket, error) {
	ticketID := parseID(id)
	
	if err := r.ticketService.CloseTicket(ticketID, resolution); err != nil {
		return nil, err
	}
	
	ticket, err := r.ticketService.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	
	return modelToGraphQLTicket(ticket), nil
}

// CreateWorkflow creates a new workflow
func (r *mutationResolver) CreateWorkflow(ctx context.Context, input CreateWorkflowInput) (*Workflow, error) {
	workflow := &models.Workflow{
		Name:        input.Name,
		Description: input.Description,
		Priority:    input.Priority,
		Triggers:    convertTriggerInputs(input.Triggers),
		Conditions:  convertConditionInputs(input.Conditions),
		Actions:     convertActionInputs(input.Actions),
	}
	
	if err := r.workflowService.CreateWorkflow(workflow); err != nil {
		return nil, err
	}
	
	return modelToGraphQLWorkflow(workflow), nil
}

type subscriptionResolver struct{ *Resolver }

// TicketUpdated subscribes to ticket updates
func (r *subscriptionResolver) TicketUpdated(ctx context.Context, id string) (<-chan *Ticket, error) {
	ticketID := parseID(id)
	ch := make(chan *Ticket, 1)
	
	// Set up subscription to ticket updates
	go func() {
		// This would connect to an event bus or message queue
		// For now, we'll simulate with a ticker
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ticket, err := r.ticketService.GetTicketByID(ticketID)
				if err == nil {
					ch <- modelToGraphQLTicket(ticket)
				}
			}
		}
	}()
	
	return ch, nil
}

// TicketCreated subscribes to new tickets
func (r *subscriptionResolver) TicketCreated(ctx context.Context, queueID *string) (<-chan *Ticket, error) {
	ch := make(chan *Ticket, 1)
	
	// Set up subscription to new tickets
	go func() {
		// This would connect to an event bus
		// Simulated implementation
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Check for new tickets
				// Send to channel if found
			}
		}
	}()
	
	return ch, nil
}

// Helper functions

func getUserIDFromContext(ctx context.Context) int {
	// Extract user ID from context (set by auth middleware)
	if userID, ok := ctx.Value("userID").(int); ok {
		return userID
	}
	return 0
}

func parseID(id string) int {
	// Parse GraphQL ID to int
	var intID int
	fmt.Sscanf(id, "%d", &intID)
	return intID
}

func formatID(id int) string {
	return fmt.Sprintf("%d", id)
}

// Model conversion functions

func modelToGraphQLUser(user *models.User) *User {
	return &User{
		ID:        formatID(user.ID),
		Email:     user.Email,
		Name:      user.Name,
		IsActive:  user.IsActive,
		CreatedAt: user.CreateTime,
		UpdatedAt: user.ChangeTime,
	}
}

func modelToGraphQLTicket(ticket *models.Ticket) *Ticket {
	return &Ticket{
		ID:          formatID(ticket.ID),
		Number:      ticket.TicketNumber,
		Subject:     ticket.Subject,
		Description: ticket.Description,
		Status:      convertTicketStatus(ticket.TicketStateID),
		Priority:    convertTicketPriority(ticket.TicketPriorityID),
		CreatedAt:   ticket.CreateTime,
		UpdatedAt:   ticket.ChangeTime,
	}
}

func modelToGraphQLQueue(queue *models.Queue) *Queue {
	return &Queue{
		ID:          formatID(queue.ID),
		Name:        queue.Name,
		Description: queue.Comment,
		Email:       queue.Email,
		IsActive:    queue.ValidID == 1,
		CreatedAt:   queue.CreateTime,
		UpdatedAt:   queue.ChangeTime,
	}
}

func modelToGraphQLWorkflow(workflow *models.Workflow) *Workflow {
	return &Workflow{
		ID:          formatID(workflow.ID),
		Name:        workflow.Name,
		Description: workflow.Description,
		Status:      convertWorkflowStatus(workflow.Status),
		Priority:    workflow.Priority,
		IsSystem:    workflow.IsSystem,
		RunCount:    workflow.RunCount,
		ErrorCount:  workflow.ErrorCount,
		CreatedAt:   workflow.CreatedAt,
		UpdatedAt:   workflow.UpdatedAt,
	}
}

// Status converters

func convertTicketStatus(stateID int) TicketStatus {
	switch stateID {
	case 1:
		return TicketStatusNew
	case 2:
		return TicketStatusOpen
	case 3:
		return TicketStatusPending
	case 4:
		return TicketStatusResolved
	case 5:
		return TicketStatusClosed
	default:
		return TicketStatusNew
	}
}

func convertTicketPriority(priorityID int) TicketPriority {
	switch priorityID {
	case 1:
		return TicketPriorityLow
	case 2:
		return TicketPriorityNormal
	case 3:
		return TicketPriorityHigh
	case 4:
		return TicketPriorityUrgent
	case 5:
		return TicketPriorityCritical
	default:
		return TicketPriorityNormal
	}
}

func convertWorkflowStatus(status models.WorkflowStatus) WorkflowStatus {
	switch status {
	case models.WorkflowStatusDraft:
		return WorkflowStatusDraft
	case models.WorkflowStatusActive:
		return WorkflowStatusActive
	case models.WorkflowStatusInactive:
		return WorkflowStatusInactive
	case models.WorkflowStatusArchived:
		return WorkflowStatusArchived
	default:
		return WorkflowStatusDraft
	}
}

// Filter converters

func convertUserFilter(filter *UserFilter) *service.UserFilter {
	if filter == nil {
		return &service.UserFilter{}
	}
	// Convert GraphQL filter to service filter
	return &service.UserFilter{
		// Map fields
	}
}

func convertTicketFilter(filter *TicketFilter) *service.TicketFilter {
	if filter == nil {
		return &service.TicketFilter{}
	}
	// Convert GraphQL filter to service filter
	return &service.TicketFilter{
		// Map fields
	}
}

func convertQueueFilter(filter *QueueFilter) *service.QueueFilter {
	if filter == nil {
		return &service.QueueFilter{}
	}
	// Convert GraphQL filter to service filter
	return &service.QueueFilter{
		// Map fields
	}
}

func convertWorkflowFilter(filter *WorkflowFilter) *service.WorkflowFilter {
	if filter == nil {
		return &service.WorkflowFilter{}
	}
	// Convert GraphQL filter to service filter
	return &service.WorkflowFilter{
		// Map fields
	}
}

func convertPagination(pagination *Pagination) *service.Pagination {
	if pagination == nil {
		return &service.Pagination{
			Page:  1,
			Limit: 20,
		}
	}
	return &service.Pagination{
		Page:      pagination.Page,
		Limit:     pagination.Limit,
		SortBy:    pagination.SortBy,
		SortOrder: string(pagination.SortOrder),
	}
}

// Edge converters

func convertUserEdges(users []*models.User) []*UserEdge {
	edges := make([]*UserEdge, len(users))
	for i, user := range users {
		edges[i] = &UserEdge{
			Node:   modelToGraphQLUser(user),
			Cursor: encodeCursor(user.ID),
		}
	}
	return edges
}

func convertTicketEdges(tickets []*models.Ticket) []*TicketEdge {
	edges := make([]*TicketEdge, len(tickets))
	for i, ticket := range tickets {
		edges[i] = &TicketEdge{
			Node:   modelToGraphQLTicket(ticket),
			Cursor: encodeCursor(ticket.ID),
		}
	}
	return edges
}

func convertQueueEdges(queues []*models.Queue) []*QueueEdge {
	edges := make([]*QueueEdge, len(queues))
	for i, queue := range queues {
		edges[i] = &QueueEdge{
			Node:   modelToGraphQLQueue(queue),
			Cursor: encodeCursor(queue.ID),
		}
	}
	return edges
}

func convertWorkflowEdges(workflows []*models.Workflow) []*WorkflowEdge {
	edges := make([]*WorkflowEdge, len(workflows))
	for i, workflow := range workflows {
		edges[i] = &WorkflowEdge{
			Node:   modelToGraphQLWorkflow(workflow),
			Cursor: encodeCursor(workflow.ID),
		}
	}
	return edges
}

func createPageInfo(pagination *Pagination, total int) *PageInfo {
	if pagination == nil {
		pagination = &Pagination{Page: 1, Limit: 20}
	}
	
	hasNextPage := (pagination.Page * pagination.Limit) < total
	hasPreviousPage := pagination.Page > 1
	
	return &PageInfo{
		HasNextPage:     hasNextPage,
		HasPreviousPage: hasPreviousPage,
	}
}

func encodeCursor(id int) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("cursor:%d", id)))
}

func decodeCursor(cursor string) int {
	data, _ := base64.StdEncoding.DecodeString(cursor)
	var id int
	fmt.Sscanf(string(data), "cursor:%d", &id)
	return id
}

// File upload handler
func (r *mutationResolver) handleFileUpload(ctx context.Context, ticketID int, upload graphql.Upload) error {
	// Handle file upload
	// This would save the file and create an attachment record
	return nil
}

// Additional converter helpers

func convertTicketStatusToInt(status *TicketStatus) int {
	if status == nil {
		return 0
	}
	switch *status {
	case TicketStatusNew:
		return 1
	case TicketStatusOpen:
		return 2
	case TicketStatusPending:
		return 3
	case TicketStatusResolved:
		return 4
	case TicketStatusClosed:
		return 5
	default:
		return 0
	}
}

func convertTicketPriorityToInt(priority *TicketPriority) int {
	if priority == nil {
		return 0
	}
	switch *priority {
	case TicketPriorityLow:
		return 1
	case TicketPriorityNormal:
		return 2
	case TicketPriorityHigh:
		return 3
	case TicketPriorityUrgent:
		return 4
	case TicketPriorityCritical:
		return 5
	default:
		return 0
	}
}

func convertWorkflowStatusToModel(status *WorkflowStatus) models.WorkflowStatus {
	if status == nil {
		return models.WorkflowStatusDraft
	}
	switch *status {
	case WorkflowStatusDraft:
		return models.WorkflowStatusDraft
	case WorkflowStatusActive:
		return models.WorkflowStatusActive
	case WorkflowStatusInactive:
		return models.WorkflowStatusInactive
	case WorkflowStatusArchived:
		return models.WorkflowStatusArchived
	default:
		return models.WorkflowStatusDraft
	}
}

func convertStatus(status TicketStatus) int {
	switch status {
	case TicketStatusNew:
		return 1
	case TicketStatusOpen:
		return 2
	case TicketStatusPending:
		return 3
	case TicketStatusResolved:
		return 4
	case TicketStatusClosed:
		return 5
	default:
		return 1
	}
}

func convertPriority(priority *TicketPriority) int {
	if priority == nil {
		return 2 // Default to Normal
	}
	switch *priority {
	case TicketPriorityLow:
		return 1
	case TicketPriorityNormal:
		return 2
	case TicketPriorityHigh:
		return 3
	case TicketPriorityUrgent:
		return 4
	case TicketPriorityCritical:
		return 5
	default:
		return 2
	}
}

func convertSearchType(searchType *SearchType) string {
	if searchType == nil {
		return "all"
	}
	switch *searchType {
	case SearchTypeTicket:
		return "ticket"
	case SearchTypeUser:
		return "user"
	case SearchTypeOrganization:
		return "organization"
	case SearchTypeAll:
		return "all"
	default:
		return "all"
	}
}

func convertSearchResults(results *service.SearchResults) *SearchResults {
	return &SearchResults{
		Tickets:       convertTicketsToGraphQL(results.Tickets),
		Users:         convertUsersToGraphQL(results.Users),
		Organizations: convertOrganizationsToGraphQL(results.Organizations),
		TotalCount:    results.TotalCount,
	}
}

func convertDashboardStats(stats *service.DashboardStats) *DashboardStats {
	return &DashboardStats{
		OpenTickets:         stats.OpenTickets,
		PendingTickets:      stats.PendingTickets,
		OverdueTickets:      stats.OverdueTickets,
		TodayCreated:        stats.TodayCreated,
		TodayClosed:         stats.TodayClosed,
		AverageResponseTime: stats.AverageResponseTime,
		SLACompliance:       stats.SLACompliance,
		AgentActivity:       convertAgentActivityToGraphQL(stats.AgentActivity),
		RecentTickets:       convertTicketsToGraphQL(stats.RecentTickets),
	}
}

func convertTicketsToGraphQL(tickets []*models.Ticket) []*Ticket {
	result := make([]*Ticket, len(tickets))
	for i, ticket := range tickets {
		result[i] = modelToGraphQLTicket(ticket)
	}
	return result
}

func convertUsersToGraphQL(users []*models.User) []*User {
	result := make([]*User, len(users))
	for i, user := range users {
		result[i] = modelToGraphQLUser(user)
	}
	return result
}

func convertOrganizationsToGraphQL(orgs []*models.Organization) []*Organization {
	result := make([]*Organization, len(orgs))
	for i, org := range orgs {
		result[i] = modelToGraphQLOrganization(org)
	}
	return result
}

func modelToGraphQLOrganization(org *models.Organization) *Organization {
	return &Organization{
		ID:        formatID(org.ID),
		Name:      org.Name,
		Domain:    org.Domain,
		IsActive:  org.IsActive,
		CreatedAt: org.CreatedAt,
		UpdatedAt: org.UpdatedAt,
	}
}

func convertAgentActivityToGraphQL(activities []*service.AgentActivity) []*AgentActivity {
	result := make([]*AgentActivity, len(activities))
	for i, activity := range activities {
		result[i] = &AgentActivity{
			UserID:         formatID(activity.UserID),
			Name:           activity.Name,
			TicketsHandled: activity.TicketsHandled,
			AverageTime:    activity.AverageTime,
			Status:         activity.Status,
		}
	}
	return result
}

func convertTriggerInputs(inputs []TriggerInput) []models.Trigger {
	triggers := make([]models.Trigger, len(inputs))
	for i, input := range inputs {
		triggers[i] = models.Trigger{
			Type:    convertTriggerType(input.Type),
			Config:  input.Config,
			Enabled: input.Enabled != nil && *input.Enabled,
		}
	}
	return triggers
}

func convertConditionInputs(inputs []ConditionInput) []models.Condition {
	if inputs == nil {
		return nil
	}
	conditions := make([]models.Condition, len(inputs))
	for i, input := range inputs {
		conditions[i] = models.Condition{
			Field:    input.Field,
			Operator: convertConditionOperator(input.Operator),
			Value:    input.Value,
			Logic:    convertLogicOperator(input.Logic),
		}
	}
	return conditions
}

func convertActionInputs(inputs []ActionInput) []models.Action {
	actions := make([]models.Action, len(inputs))
	for i, input := range inputs {
		order := i
		if input.Order != nil {
			order = *input.Order
		}
		continueOnError := false
		if input.ContinueOnError != nil {
			continueOnError = *input.ContinueOnError
		}
		var delaySeconds int
		if input.DelaySeconds != nil {
			delaySeconds = *input.DelaySeconds
		}
		actions[i] = models.Action{
			Type:            convertActionType(input.Type),
			Config:          input.Config,
			Order:           order,
			ContinueOnError: continueOnError,
			DelaySeconds:    delaySeconds,
		}
	}
	return actions
}

func convertTriggerType(triggerType TriggerType) models.TriggerType {
	switch triggerType {
	case TriggerTypeTicketCreated:
		return models.TriggerTypeTicketCreated
	case TriggerTypeTicketUpdated:
		return models.TriggerTypeTicketUpdated
	case TriggerTypeTicketAssigned:
		return models.TriggerTypeTicketAssigned
	case TriggerTypeStatusChanged:
		return models.TriggerTypeStatusChanged
	case TriggerTypePriorityChanged:
		return models.TriggerTypePriorityChanged
	case TriggerTypeCustomerReply:
		return models.TriggerTypeCustomerReply
	case TriggerTypeAgentReply:
		return models.TriggerTypeAgentReply
	case TriggerTypeTimeBased:
		return models.TriggerTypeTimeBased
	case TriggerTypeScheduled:
		return models.TriggerTypeScheduled
	default:
		return models.TriggerTypeTicketCreated
	}
}

func convertActionType(actionType ActionType) models.ActionType {
	switch actionType {
	case ActionTypeAssignTicket:
		return models.ActionTypeAssignTicket
	case ActionTypeChangeStatus:
		return models.ActionTypeChangeStatus
	case ActionTypeChangePriority:
		return models.ActionTypeChangePriority
	case ActionTypeSendEmail:
		return models.ActionTypeSendEmail
	case ActionTypeSendNotification:
		return models.ActionTypeSendNotification
	case ActionTypeAddTag:
		return models.ActionTypeAddTag
	case ActionTypeRemoveTag:
		return models.ActionTypeRemoveTag
	case ActionTypeAddNote:
		return models.ActionTypeAddNote
	case ActionTypeEscalate:
		return models.ActionTypeEscalate
	case ActionTypeWebhookCall:
		return models.ActionTypeWebhookCall
	default:
		return models.ActionTypeSendNotification
	}
}

func convertConditionOperator(op ConditionOperator) string {
	switch op {
	case ConditionOperatorEquals:
		return "equals"
	case ConditionOperatorNotEquals:
		return "not_equals"
	case ConditionOperatorContains:
		return "contains"
	case ConditionOperatorNotContains:
		return "not_contains"
	case ConditionOperatorGreaterThan:
		return "greater_than"
	case ConditionOperatorLessThan:
		return "less_than"
	case ConditionOperatorIn:
		return "in"
	case ConditionOperatorNotIn:
		return "not_in"
	default:
		return "equals"
	}
}

func convertLogicOperator(op *LogicOperator) string {
	if op == nil {
		return "AND"
	}
	switch *op {
	case LogicOperatorAnd:
		return "AND"
	case LogicOperatorOr:
		return "OR"
	default:
		return "AND"
	}
}

func parseIDPointer(id *string) int {
	if id == nil {
		return 0
	}
	return parseID(*id)
}