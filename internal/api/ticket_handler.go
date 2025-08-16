package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// TicketHandler handles ticket-related HTTP requests
type TicketHandler struct {
	ticketService *service.TicketService
	ticketRepo    *repository.TicketRepository
}

// NewTicketHandler creates a new ticket handler
func NewTicketHandler(ticketService *service.TicketService, ticketRepo *repository.TicketRepository) *TicketHandler {
	return &TicketHandler{
		ticketService: ticketService,
		ticketRepo:    ticketRepo,
	}
}

// CreateTicket creates a new ticket
// @Summary Create a new ticket
// @Description Create a new ticket with optional initial article
// @Tags tickets
// @Accept json
// @Produce json
// @Param ticket body models.CreateTicketRequest true "Ticket data"
// @Success 201 {object} models.Ticket
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/v1/tickets [post]
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	var req models.CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)
	req.CreateBy = claims.UserID
	req.TenantID = claims.TenantID

	ticket, err := h.ticketService.CreateTicket(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create ticket",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

// GetTicket retrieves a ticket by ID
// @Summary Get a ticket by ID
// @Description Get detailed information about a specific ticket
// @Tags tickets
// @Accept json
// @Produce json
// @Param id path int true "Ticket ID"
// @Success 200 {object} models.Ticket
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/{id} [get]
func (h *TicketHandler) GetTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ticket ID",
		})
		return
	}

	// Get user from context for permission check
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)

	ticket, err := h.ticketRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Ticket not found",
		})
		return
	}

	// Check tenant access
	if ticket.TenantID != claims.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	// Check customer access (customers can only see their own tickets)
	if claims.Role == "Customer" && ticket.CustomerUserID != claims.UserID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	c.JSON(http.StatusOK, ticket)
}

// ListTickets lists tickets with filtering and pagination
// @Summary List tickets
// @Description Get a list of tickets with optional filtering and pagination
// @Tags tickets
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param search query string false "Search term"
// @Param queue_id query int false "Filter by queue ID"
// @Param state_id query int false "Filter by state ID"
// @Param priority_id query int false "Filter by priority ID"
// @Success 200 {object} models.TicketListResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tickets [get]
func (h *TicketHandler) ListTickets(c *gin.Context) {
	var req models.TicketListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid query parameters",
			Details: err.Error(),
		})
		return
	}

	// Set defaults
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)
	req.TenantID = claims.TenantID

	// If customer, only show their tickets
	if claims.Role == "Customer" {
		req.CustomerID = strconv.FormatUint(uint64(claims.UserID), 10)
	}

	response, err := h.ticketRepo.List(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list tickets",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateTicket updates a ticket
// @Summary Update a ticket
// @Description Update ticket properties
// @Tags tickets
// @Accept json
// @Produce json
// @Param id path int true "Ticket ID"
// @Param ticket body models.UpdateTicketRequest true "Update data"
// @Success 200 {object} models.Ticket
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/{id} [put]
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ticket ID",
		})
		return
	}

	var req models.UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)
	req.UpdateBy = claims.UserID

	// Check if ticket exists and user has access
	existingTicket, err := h.ticketRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Ticket not found",
		})
		return
	}

	// Check tenant access
	if existingTicket.TenantID != claims.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	// Customers cannot update tickets
	if claims.Role == "Customer" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Customers cannot update tickets",
		})
		return
	}

	ticket, err := h.ticketService.UpdateTicket(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to update ticket",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ticket)
}

// AddArticle adds an article to a ticket
// @Summary Add article to ticket
// @Description Add a new article (comment/reply) to a ticket
// @Tags tickets
// @Accept json
// @Produce json
// @Param id path int true "Ticket ID"
// @Param article body models.CreateArticleRequest true "Article data"
// @Success 201 {object} models.Article
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/{id}/articles [post]
func (h *TicketHandler) AddArticle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ticket ID",
		})
		return
	}

	var req models.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)
	req.CreateBy = claims.UserID

	// Check if ticket exists and user has access
	ticket, err := h.ticketRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Ticket not found",
		})
		return
	}

	// Check tenant access
	if ticket.TenantID != claims.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	// Check customer access
	if claims.Role == "Customer" && ticket.CustomerUserID != claims.UserID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	article, err := h.ticketService.AddArticle(uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to add article",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, article)
}

// GetArticles gets articles for a ticket
// @Summary Get ticket articles
// @Description Get all articles (comments/replies) for a ticket
// @Tags tickets
// @Accept json
// @Produce json
// @Param id path int true "Ticket ID"
// @Success 200 {array} models.Article
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/{id}/articles [get]
func (h *TicketHandler) GetArticles(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ticket ID",
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)

	// Check if ticket exists and user has access
	ticket, err := h.ticketRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Ticket not found",
		})
		return
	}

	// Check tenant access
	if ticket.TenantID != claims.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	// Check customer access
	includeInternal := true
	if claims.Role == "Customer" {
		if ticket.CustomerUserID != claims.UserID {
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error: "Access denied",
			})
			return
		}
		includeInternal = false // Customers cannot see internal articles
	}

	// Get articles from repository
	articleRepo := repository.NewArticleRepository(h.ticketRepo.GetDB())
	articles, err := articleRepo.GetByTicketID(uint(id), includeInternal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get articles",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, articles)
}

// AssignTicket assigns a ticket to an agent
// @Summary Assign ticket to agent
// @Description Assign a ticket to a specific agent
// @Tags tickets
// @Accept json
// @Produce json
// @Param id path int true "Ticket ID"
// @Param assignment body models.AssignTicketRequest true "Assignment data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/{id}/assign [post]
func (h *TicketHandler) AssignTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ticket ID",
		})
		return
	}

	var req models.AssignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)

	// Only agents and admins can assign tickets
	if claims.Role != "Agent" && claims.Role != "Admin" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Only agents and admins can assign tickets",
		})
		return
	}

	err = h.ticketService.AssignTicket(uint(id), req.AgentID, claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to assign ticket",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Ticket assigned successfully",
	})
}

// EscalateTicket escalates a ticket priority
// @Summary Escalate ticket priority
// @Description Change ticket priority to a higher level with reason
// @Tags tickets
// @Accept json
// @Produce json
// @Param id path int true "Ticket ID"
// @Param escalation body models.EscalateTicketRequest true "Escalation data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/{id}/escalate [post]
func (h *TicketHandler) EscalateTicket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ticket ID",
		})
		return
	}

	var req models.EscalateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)

	// Only agents and admins can escalate tickets
	if claims.Role != "Agent" && claims.Role != "Admin" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Only agents and admins can escalate tickets",
		})
		return
	}

	err = h.ticketService.EscalateTicket(uint(id), req.PriorityID, claims.UserID, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to escalate ticket",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Ticket escalated successfully",
	})
}

// MergeTickets merges two tickets
// @Summary Merge tickets
// @Description Merge a source ticket into a target ticket
// @Tags tickets
// @Accept json
// @Produce json
// @Param merge body models.MergeTicketsRequest true "Merge data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/merge [post]
func (h *TicketHandler) MergeTickets(c *gin.Context) {
	var req models.MergeTicketsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)

	// Only agents and admins can merge tickets
	if claims.Role != "Agent" && claims.Role != "Admin" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Only agents and admins can merge tickets",
		})
		return
	}

	err := h.ticketService.MergeTickets(req.TargetTicketID, req.SourceTicketID, claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to merge tickets",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Tickets merged successfully",
	})
}

// GetTicketHistory gets the history of a ticket
// @Summary Get ticket history
// @Description Get the change history of a ticket
// @Tags tickets
// @Accept json
// @Produce json
// @Param id path int true "Ticket ID"
// @Success 200 {array} models.TicketHistory
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tickets/{id}/history [get]
func (h *TicketHandler) GetTicketHistory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ticket ID",
		})
		return
	}

	// Get user from context
	userClaims, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
		})
		return
	}

	claims := userClaims.(*auth.Claims)

	// Check if ticket exists and user has access
	ticket, err := h.ticketRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Ticket not found",
		})
		return
	}

	// Check tenant access
	if ticket.TenantID != claims.TenantID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	// Check customer access
	if claims.Role == "Customer" && ticket.CustomerUserID != claims.UserID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "Access denied",
		})
		return
	}

	history, err := h.ticketService.GetTicketHistory(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get ticket history",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, history)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Message string `json:"message"`
}