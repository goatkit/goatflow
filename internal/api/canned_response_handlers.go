package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// CannedResponseHandlers manages canned response API endpoints.
type CannedResponseHandlers struct {
	service *service.CannedResponseService
}

// CannedResponseHandlerExports provides singleton access for YAML route registration.
var CannedResponseHandlerExports = NewCannedResponseHandlers()

// NewCannedResponseHandlers creates a new canned response handlers instance.
func NewCannedResponseHandlers() *CannedResponseHandlers {
	repo := repository.NewMemoryCannedResponseRepository()
	srv := service.NewCannedResponseService(repo)

	// Initialize with some default responses
	initializeDefaultResponses(srv)

	return &CannedResponseHandlers{
		service: srv,
	}
}

// initializeDefaultResponses creates some default canned responses.
func initializeDefaultResponses(srv *service.CannedResponseService) {
	defaults := []models.CannedResponse{
		{
			Name:     "Welcome Message",
			Shortcut: "/welcome",
			Category: "Greetings",
			Subject:  "Welcome to our support",
			Content: "Hello {{customer_name}},\n\n" +
				"Welcome to our support system. We're here to help you with any questions or issues you may have.\n\n" +
				"Best regards,\n{{agent_name}}",
			ContentType: "text/plain",
			Tags:        []string{"greeting", "welcome", "new"},
			IsPublic:    true,
			IsActive:    true,
		},
		{
			Name:     "Request More Information",
			Shortcut: "/info",
			Category: "Information",
			Content: "Hello {{customer_name}},\n\n" +
				"To better assist you with your request, could you please provide the following information:\n\n" +
				"1. {{info_needed}}\n2. Any error messages you're seeing\n3. Steps to reproduce the issue\n\n" +
				"Thank you,\n{{agent_name}}",
			ContentType: "text/plain",
			Tags:        []string{"information", "request", "details"},
			IsPublic:    true,
			IsActive:    true,
		},
		{
			Name:     "Password Reset Instructions",
			Shortcut: "/password",
			Category: "Account",
			Subject:  "Password Reset Instructions",
			Content: "Hello {{customer_name}},\n\n" +
				"To reset your password, please follow these steps:\n\n" +
				"1. Click on the 'Forgot Password' link on the login page\n" +
				"2. Enter your email address\n" +
				"3. Check your email for the reset link\n" +
				"4. Follow the link to create a new password\n\n" +
				"If you continue to have issues, please let us know.\n\n" +
				"Best regards,\n{{agent_name}}",
			ContentType: "text/plain",
			Tags:        []string{"password", "account", "reset", "security"},
			IsPublic:    true,
			IsActive:    true,
		},
		{
			Name:     "Ticket Resolved",
			Shortcut: "/resolved",
			Category: "Resolution",
			Subject:  "Ticket Resolved - {{ticket_number}}",
			Content: "Hello {{customer_name}},\n\n" +
				"We're pleased to inform you that your ticket {{ticket_number}} has been resolved.\n\n" +
				"If you have any further questions or if the issue persists, please don't hesitate to contact us.\n\n" +
				"Thank you for your patience.\n\nBest regards,\n{{agent_name}}",
			ContentType: "text/plain",
			Tags:        []string{"resolved", "closed", "complete"},
			IsPublic:    true,
			IsActive:    true,
		},
		{
			Name:     "Escalation Notice",
			Shortcut: "/escalate",
			Category: "Escalation",
			Subject:  "Ticket Escalated - {{ticket_number}}",
			Content: "Hello {{customer_name}},\n\n" +
				"Your ticket {{ticket_number}} has been escalated to our senior support team for further investigation.\n\n" +
				"We will update you as soon as we have more information.\n\n" +
				"Thank you for your patience.\n\nBest regards,\n{{agent_name}}",
			ContentType: "text/plain",
			Tags:        []string{"escalation", "priority", "urgent"},
			IsPublic:    true,
			IsActive:    true,
		},
	}

	ctx := context.Background()
	for _, resp := range defaults {
		_ = srv.CreateResponse(ctx, &resp) //nolint:errcheck // Best-effort seed
	}
}

// GetResponses retrieves all active canned responses.
func (h *CannedResponseHandlers) GetResponses(c *gin.Context) {
	responses, err := h.service.GetActiveResponses(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, responses)
}

// GetResponseByID retrieves a specific canned response.
func (h *CannedResponseHandlers) GetResponseByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	response, err := h.service.GetResponse(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response not found"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetResponsesByCategory retrieves responses by category.
func (h *CannedResponseHandlers) GetResponsesByCategory(c *gin.Context) {
	category := c.Param("category")

	responses, err := h.service.GetResponsesByCategory(c.Request.Context(), category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, responses)
}

// GetQuickResponses retrieves responses with shortcuts.
func (h *CannedResponseHandlers) GetQuickResponses(c *gin.Context) {
	responses, err := h.service.GetQuickResponses(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, responses)
}

// GetResponsesForUser retrieves responses accessible to the current user.
func (h *CannedResponseHandlers) GetResponsesForUser(c *gin.Context) {
	// TODO: Get actual user ID from session/auth
	userID := uint(1)

	responses, err := h.service.GetResponsesForUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, responses)
}

// SearchResponses searches for canned responses.
func (h *CannedResponseHandlers) SearchResponses(c *gin.Context) {
	var filter models.CannedResponseFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		// Try query parameters as fallback
		filter.Query = c.Query("q")
		filter.Category = c.Query("category")
		if limitStr := c.Query("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				filter.Limit = limit
			}
		}
	}

	// Default limit if not specified
	if filter.Limit == 0 {
		filter.Limit = 50
	}

	responses, err := h.service.SearchResponses(c.Request.Context(), &filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, responses)
}

// GetPopularResponses retrieves the most used responses.
func (h *CannedResponseHandlers) GetPopularResponses(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	responses, err := h.service.GetPopularResponses(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, responses)
}

// GetCategories retrieves all canned response categories.
func (h *CannedResponseHandlers) GetCategories(c *gin.Context) {
	categories, err := h.service.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// CreateResponse creates a new canned response.
func (h *CannedResponseHandlers) CreateResponse(c *gin.Context) {
	var response models.CannedResponse
	if err := c.ShouldBindJSON(&response); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Set owner from current user
	response.OwnerID = 1
	response.CreatedBy = 1

	if err := h.service.CreateResponse(c.Request.Context(), &response); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// UpdateResponse updates an existing canned response.
func (h *CannedResponseHandlers) UpdateResponse(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	var response models.CannedResponse
	if err := c.ShouldBindJSON(&response); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response.ID = uint(id)
	response.UpdatedBy = 1 // TODO: Get from current user

	if err := h.service.UpdateResponse(c.Request.Context(), &response); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeleteResponse deletes a canned response.
func (h *CannedResponseHandlers) DeleteResponse(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	// TODO: Check permissions

	if err := h.service.DeleteResponse(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Response deleted successfully"})
}

// ApplyResponse applies a canned response to a ticket.
func (h *CannedResponseHandlers) ApplyResponse(c *gin.Context) {
	var application models.CannedResponseApplication
	if err := c.ShouldBindJSON(&application); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Get actual user ID from session/auth
	userID := uint(1)

	// Create auto-fill context
	// TODO: Get actual context from ticket/user data
	autoFillCtx := &models.AutoFillContext{
		AgentName:     "Support Agent",
		AgentEmail:    "support@example.com",
		TicketNumber:  "TICKET-" + strconv.Itoa(int(application.TicketID)),
		CustomerName:  "Customer",
		CustomerEmail: "customer@example.com",
		QueueName:     "General",
	}

	result, err := h.service.ApplyResponseWithContext(
		c.Request.Context(),
		&application,
		userID,
		autoFillCtx,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ExportResponses exports all canned responses as JSON.
func (h *CannedResponseHandlers) ExportResponses(c *gin.Context) {
	data, err := h.service.ExportResponses(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=canned_responses.json")
	c.Data(http.StatusOK, "application/json", data)
}

// ImportResponses imports canned responses from JSON.
func (h *CannedResponseHandlers) ImportResponses(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Open the file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	// Read file content
	data := make([]byte, file.Size)
	if _, err := src.Read(data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	// Import the data
	if err := h.service.ImportResponses(c.Request.Context(), data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Responses imported successfully"})
}
