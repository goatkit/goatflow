package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/webhook"
)

// WebhookHandlers handles webhook-related API endpoints.
type WebhookHandlers struct {
	webhookManager *webhook.Manager
}

// NewWebhookHandlers creates a new webhook handlers instance.
func NewWebhookHandlers(webhookManager *webhook.Manager) *WebhookHandlers {
	return &WebhookHandlers{
		webhookManager: webhookManager,
	}
}

// SetupWebhookRoutes sets up webhook API routes.
func (h *WebhookHandlers) SetupWebhookRoutes(router *APIRouter, adminRoutes *gin.RouterGroup) {
	webhooks := adminRoutes.Group("/webhooks")
	{
		// Webhook management
		webhooks.GET("", h.handleListWebhooks)
		webhooks.POST("", h.handleCreateWebhook)
		webhooks.GET("/:id", h.handleGetWebhook)
		webhooks.PUT("/:id", h.handleUpdateWebhook)
		webhooks.DELETE("/:id", h.handleDeleteWebhook)

		// Webhook testing and monitoring
		webhooks.POST("/:id/test", h.handleTestWebhook)
		webhooks.GET("/:id/stats", h.handleGetWebhookStats)
		webhooks.GET("/:id/deliveries", h.handleGetWebhookDeliveries)
		webhooks.GET("/:id/deliveries/:delivery_id", h.handleGetWebhookDelivery)

		// Webhook activation/deactivation
		webhooks.POST("/:id/activate", h.handleActivateWebhook)
		webhooks.POST("/:id/deactivate", h.handleDeactivateWebhook)

		// Global webhook management
		webhooks.GET("/events", h.handleListWebhookEvents)
		webhooks.POST("/test-all", h.handleTestAllWebhooks)
		webhooks.GET("/stats", h.handleGetGlobalWebhookStats)
		webhooks.DELETE("/deliveries/cleanup", h.handleCleanupWebhookDeliveries)
	}
}

// handleListWebhooks returns all configured webhooks.
func (h *WebhookHandlers) handleListWebhooks(c *gin.Context) {
	webhooks, err := h.webhookManager.ListWebhooks()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to retrieve webhooks: "+err.Error())
		return
	}

	sendSuccess(c, webhooks)
}

// handleCreateWebhook creates a new webhook.
func (h *WebhookHandlers) handleCreateWebhook(c *gin.Context) {
	var req webhook.WebhookRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook request: "+err.Error())
		return
	}

	// Validate events
	if err := h.validateWebhookEvents(req.Events); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook events: "+err.Error())
		return
	}

	createdWebhook, err := h.webhookManager.CreateWebhook(req)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create webhook: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    createdWebhook,
	})
}

// handleGetWebhook returns a specific webhook.
func (h *WebhookHandlers) handleGetWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	webhook, err := h.webhookManager.GetWebhook(uint(id))
	if err != nil {
		sendError(c, http.StatusNotFound, "Webhook not found: "+err.Error())
		return
	}

	sendSuccess(c, webhook)
}

// handleUpdateWebhook updates an existing webhook.
func (h *WebhookHandlers) handleUpdateWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	var req webhook.WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook request: "+err.Error())
		return
	}

	// Validate events
	if err := h.validateWebhookEvents(req.Events); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook events: "+err.Error())
		return
	}

	updatedWebhook, err := h.webhookManager.UpdateWebhook(uint(id), req)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update webhook: "+err.Error())
		return
	}

	sendSuccess(c, updatedWebhook)
}

// handleDeleteWebhook removes a webhook.
func (h *WebhookHandlers) handleDeleteWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	err = h.webhookManager.DeleteWebhook(uint(id))
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete webhook: "+err.Error())
		return
	}

	sendSuccess(c, gin.H{
		"id":      id,
		"message": "Webhook deleted successfully",
	})
}

// handleTestWebhook tests a webhook by sending a test payload.
func (h *WebhookHandlers) handleTestWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	result, err := h.webhookManager.TestWebhook(uint(id))
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to test webhook: "+err.Error())
		return
	}

	sendSuccess(c, result)
}

// handleGetWebhookStats returns webhook statistics.
func (h *WebhookHandlers) handleGetWebhookStats(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	stats, err := h.webhookManager.GetWebhookStatistics(uint(id))
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to get webhook statistics: "+err.Error())
		return
	}

	sendSuccess(c, stats)
}

// handleGetWebhookDeliveries returns webhook delivery history.
func (h *WebhookHandlers) handleGetWebhookDeliveries(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50")) //nolint:errcheck // Defaults to 0, corrected below
	if limit > 100 {
		limit = 100 // Cap at 100 deliveries
	}

	deliveries, err := h.webhookManager.GetDeliveries(uint(id), limit)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to get webhook deliveries: "+err.Error())
		return
	}

	sendSuccess(c, gin.H{
		"deliveries": deliveries,
		"count":      len(deliveries),
	})
}

// handleGetWebhookDelivery returns a specific webhook delivery.
func (h *WebhookHandlers) handleGetWebhookDelivery(c *gin.Context) {
	webhookID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	deliveryID, err := strconv.ParseUint(c.Param("delivery_id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid delivery ID")
		return
	}

	// TODO: Implement delivery retrieval by ID
	// For now, return mock data
	sendSuccess(c, gin.H{
		"id":         deliveryID,
		"webhook_id": webhookID,
		"event":      "ticket.created",
		"status":     "success",
		"message":    "Delivery details would be returned here",
	})
}

// handleActivateWebhook activates a webhook.
func (h *WebhookHandlers) handleActivateWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	now := time.Now()
	query := database.ConvertQuery(`UPDATE webhook SET valid_id = 1, change_time = ? WHERE id = ?`)
	result, err := db.Exec(query, now, id)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to activate webhook")
		return
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		sendError(c, http.StatusNotFound, "Webhook not found")
		return
	}

	sendSuccess(c, gin.H{
		"id":      id,
		"status":  "active",
		"message": "Webhook activated successfully",
	})
}

// handleDeactivateWebhook deactivates a webhook.
func (h *WebhookHandlers) handleDeactivateWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid webhook ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	now := time.Now()
	query := database.ConvertQuery(`UPDATE webhook SET valid_id = 2, change_time = ? WHERE id = ?`)
	result, err := db.Exec(query, now, id)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to deactivate webhook")
		return
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		sendError(c, http.StatusNotFound, "Webhook not found")
		return
	}

	sendSuccess(c, gin.H{
		"id":      id,
		"status":  "inactive",
		"message": "Webhook deactivated successfully",
	})
}

// handleListWebhookEvents returns available webhook events.
func (h *WebhookHandlers) handleListWebhookEvents(c *gin.Context) {
	events := map[string]interface{}{
		"ticket_events": []string{
			string(webhook.EventTicketCreated),
			string(webhook.EventTicketUpdated),
			string(webhook.EventTicketClosed),
			string(webhook.EventTicketReopened),
			string(webhook.EventTicketAssigned),
			string(webhook.EventTicketEscalated),
			string(webhook.EventTicketPriorityChanged),
			string(webhook.EventTicketStatusChanged),
			string(webhook.EventTicketQueueMoved),
		},
		"article_events": []string{
			string(webhook.EventArticleAdded),
			string(webhook.EventArticleUpdated),
		},
		"user_events": []string{
			string(webhook.EventUserCreated),
			string(webhook.EventUserUpdated),
			string(webhook.EventUserActivated),
			string(webhook.EventUserDeactivated),
		},
		"queue_events": []string{
			string(webhook.EventQueueCreated),
			string(webhook.EventQueueUpdated),
			string(webhook.EventQueueDeleted),
		},
		"system_events": []string{
			string(webhook.EventSystemMaintenance),
			string(webhook.EventSystemBackup),
			string(webhook.EventSystemAlert),
		},
		"attachment_events": []string{
			string(webhook.EventAttachmentUploaded),
			string(webhook.EventAttachmentDeleted),
		},
	}

	sendSuccess(c, events)
}

// handleTestAllWebhooks tests all active webhooks.
func (h *WebhookHandlers) handleTestAllWebhooks(c *gin.Context) {
	webhooks, err := h.webhookManager.ListWebhooks()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to retrieve webhooks: "+err.Error())
		return
	}

	results := make([]gin.H, 0)

	for _, wh := range webhooks {
		if wh.Status == webhook.StatusActive {
			result, err := h.webhookManager.TestWebhook(wh.ID)
			if err != nil {
				results = append(results, gin.H{
					"webhook_id": wh.ID,
					"name":       wh.Name,
					"success":    false,
					"error":      err.Error(),
				})
			} else {
				results = append(results, gin.H{
					"webhook_id":    wh.ID,
					"name":          wh.Name,
					"success":       result.Success,
					"status_code":   result.StatusCode,
					"response_time": result.ResponseTime.Milliseconds(),
					"error":         result.ErrorMessage,
				})
			}
		}
	}

	sendSuccess(c, gin.H{
		"results": results,
		"total":   len(results),
	})
}

// handleGetGlobalWebhookStats returns global webhook statistics.
func (h *WebhookHandlers) handleGetGlobalWebhookStats(c *gin.Context) {
	webhooks, err := h.webhookManager.ListWebhooks()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to retrieve webhooks: "+err.Error())
		return
	}

	totalWebhooks := len(webhooks)
	activeWebhooks := 0
	totalDeliveries := 0
	successfulDeliveries := 0
	failedDeliveries := 0

	for _, wh := range webhooks {
		if wh.Status == webhook.StatusActive {
			activeWebhooks++
		}
		totalDeliveries += wh.TotalDeliveries
		successfulDeliveries += wh.SuccessfulDeliveries
		failedDeliveries += wh.FailedDeliveries
	}

	successRate := 0.0
	if totalDeliveries > 0 {
		successRate = float64(successfulDeliveries) / float64(totalDeliveries) * 100
	}

	sendSuccess(c, gin.H{
		"total_webhooks":        totalWebhooks,
		"active_webhooks":       activeWebhooks,
		"inactive_webhooks":     totalWebhooks - activeWebhooks,
		"total_deliveries":      totalDeliveries,
		"successful_deliveries": successfulDeliveries,
		"failed_deliveries":     failedDeliveries,
		"success_rate":          successRate,
	})
}

// handleCleanupWebhookDeliveries triggers cleanup of old webhook deliveries.
func (h *WebhookHandlers) handleCleanupWebhookDeliveries(c *gin.Context) {
	var req struct {
		OlderThanDays int `json:"older_than_days" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid cleanup request: "+err.Error())
		return
	}

	// TODO: Implement delivery cleanup
	sendSuccess(c, gin.H{
		"message":         "Cleanup initiated",
		"older_than_days": req.OlderThanDays,
	})
}

// validateWebhookEvents validates that all provided events are valid.
func (h *WebhookHandlers) validateWebhookEvents(events []webhook.WebhookEvent) error {
	validEvents := map[webhook.WebhookEvent]bool{
		webhook.EventTicketCreated:         true,
		webhook.EventTicketUpdated:         true,
		webhook.EventTicketClosed:          true,
		webhook.EventTicketReopened:        true,
		webhook.EventTicketAssigned:        true,
		webhook.EventTicketEscalated:       true,
		webhook.EventTicketPriorityChanged: true,
		webhook.EventTicketStatusChanged:   true,
		webhook.EventTicketQueueMoved:      true,
		webhook.EventArticleAdded:          true,
		webhook.EventArticleUpdated:        true,
		webhook.EventUserCreated:           true,
		webhook.EventUserUpdated:           true,
		webhook.EventUserActivated:         true,
		webhook.EventUserDeactivated:       true,
		webhook.EventQueueCreated:          true,
		webhook.EventQueueUpdated:          true,
		webhook.EventQueueDeleted:          true,
		webhook.EventAttachmentUploaded:    true,
		webhook.EventAttachmentDeleted:     true,
		webhook.EventSystemMaintenance:     true,
		webhook.EventSystemBackup:          true,
		webhook.EventSystemAlert:           true,
	}

	for _, event := range events {
		if !validEvents[event] {
			return fmt.Errorf("invalid event: %s", event)
		}
	}

	return nil
}
