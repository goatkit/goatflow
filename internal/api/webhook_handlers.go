package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/webhook"
	"github.com/goatkit/goatflow/internal/webhooks"
)

// Outbound webhook administration (routes/api-webhooks.yaml, admin only).

func init() {
	routing.RegisterHandler("handleWebhookList", handleWebhookList)
	routing.RegisterHandler("handleWebhookCreate", handleWebhookCreate)
	routing.RegisterHandler("handleWebhookEvents", handleWebhookEvents)
	routing.RegisterHandler("handleWebhookGet", handleWebhookGet)
	routing.RegisterHandler("handleWebhookUpdate", handleWebhookUpdate)
	routing.RegisterHandler("handleWebhookDelete", handleWebhookDelete)
	routing.RegisterHandler("handleWebhookTest", handleWebhookTest)
	routing.RegisterHandler("handleWebhookDeliveries", handleWebhookDeliveries)
	routing.RegisterHandler("handleWebhookDeliveryGet", handleWebhookDeliveryGet)
	routing.RegisterHandler("handleWebhookRedeliver", handleWebhookRedeliver)
}

const (
	defaultDeliveryListLimit = 50
	maxDeliveryListLimit     = 200
)

// webhookRequest is the body of create (all of name, url, events required)
// and update (only the fields present change). An empty secret removes it.
type webhookRequest struct {
	Name           *string            `json:"name"`
	URL            *string            `json:"url"`
	Secret         *string            `json:"secret"`
	Events         *[]string          `json:"events"`
	Headers        *map[string]string `json:"headers"`
	RetryCount     *int               `json:"retry_count"`
	TimeoutSeconds *int               `json:"timeout_seconds"`
	IsActive       *bool              `json:"is_active"`
}

// apply copies the fields present in the request onto w.
func (r *webhookRequest) apply(w *webhook.Webhook) {
	if r.Name != nil {
		w.Name = *r.Name
	}
	if r.URL != nil {
		w.URL = *r.URL
	}
	if r.Events != nil {
		w.Events = *r.Events
	}
	if r.Headers != nil {
		w.Headers = *r.Headers
	}
	if r.RetryCount != nil {
		w.RetryCount = *r.RetryCount
	}
	if r.TimeoutSeconds != nil {
		w.TimeoutSeconds = *r.TimeoutSeconds
	}
	if r.IsActive != nil {
		w.IsActive = *r.IsActive
	}
}

func webhookFail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

func webhookOK(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{"success": true, "data": data})
}

// webhookError maps repository/dispatcher errors to responses.
func webhookError(c *gin.Context, action string, err error) {
	var verr *webhook.ValidationError
	switch {
	case errors.As(err, &verr):
		webhookFail(c, http.StatusBadRequest, verr.Message)
	case errors.Is(err, webhook.ErrNotFound):
		webhookFail(c, http.StatusNotFound, "not found")
	case errors.Is(err, webhook.ErrDuplicateName):
		webhookFail(c, http.StatusConflict, err.Error())
	default:
		log.Printf("webhooks: %s: %v", action, err)
		webhookFail(c, http.StatusInternalServerError, "failed to "+action)
	}
}

// webhookDeps returns the repository and dispatcher, or writes 503.
func webhookDeps(c *gin.Context) (*webhook.Repository, *webhook.Dispatcher, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		webhookFail(c, http.StatusServiceUnavailable, "database unavailable")
		return nil, nil, false
	}
	repo := webhook.NewRepository(db)
	return repo, webhook.NewDispatcher(repo), true
}

func webhookActor(c *gin.Context) (int, bool) {
	id := GetUserIDFromCtx(c, 0)
	if id <= 0 {
		webhookFail(c, http.StatusUnauthorized, "authentication required")
		return 0, false
	}
	return id, true
}

func pathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		webhookFail(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

// handleWebhookList handles GET /api/v1/webhooks[?active=true|false].
//
//	@Summary		List webhooks
//	@Description	List outbound webhooks (admin only)
//	@Tags			Webhooks
//	@Produce		json
//	@Param			active	query		bool	false	"Only active (true) or inactive (false) webhooks"
//	@Success		200		{object}	map[string]interface{}	"success, data: []Webhook"
//	@Failure		400		{object}	map[string]interface{}	"Invalid filter"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Failure		403		{object}	map[string]interface{}	"Admin access required"
//	@Security		BearerAuth
//	@Router			/webhooks [get]
func handleWebhookList(c *gin.Context) {
	repo, _, ok := webhookDeps(c)
	if !ok {
		return
	}
	var active *bool
	if v := c.Query("active"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			webhookFail(c, http.StatusBadRequest, "active must be true or false")
			return
		}
		active = &b
	}
	list, err := repo.List(c.Request.Context(), active)
	if err != nil {
		webhookError(c, "list webhooks", err)
		return
	}
	webhookOK(c, http.StatusOK, list)
}

// handleWebhookEvents handles GET /api/v1/webhooks/events.
//
//	@Summary		List webhook events
//	@Description	Events a webhook can subscribe to
//	@Tags			Webhooks
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"success, data: [{event, description}]"
//	@Security		BearerAuth
//	@Router			/webhooks/events [get]
func handleWebhookEvents(c *gin.Context) {
	webhookOK(c, http.StatusOK, webhooks.Events)
}

// handleWebhookCreate handles POST /api/v1/webhooks.
//
//	@Summary		Create webhook
//	@Description	Create an outbound webhook; deliveries are signed with X-Webhook-Signature (sha256=HMAC of the body) when a secret is set
//	@Tags			Webhooks
//	@Accept			json
//	@Produce		json
//	@Param			webhook	body		object	true	"name, url, events (required); secret, headers, retry_count, timeout_seconds, is_active"
//	@Success		201		{object}	map[string]interface{}	"success, data: Webhook"
//	@Failure		400		{object}	map[string]interface{}	"Validation error"
//	@Failure		409		{object}	map[string]interface{}	"Name already used"
//	@Security		BearerAuth
//	@Router			/webhooks [post]
func handleWebhookCreate(c *gin.Context) {
	userID, ok := webhookActor(c)
	if !ok {
		return
	}
	var req webhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		webhookFail(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	w := &webhook.Webhook{
		RetryCount:     webhook.DefaultRetryCount,
		TimeoutSeconds: webhook.DefaultTimeoutSeconds,
		IsActive:       true,
	}
	req.apply(w)
	if err := w.Normalize(webhooks.IsEvent); err != nil {
		webhookError(c, "create webhook", err)
		return
	}
	secret := ""
	if req.Secret != nil {
		secret = *req.Secret
	}
	if err := webhook.ValidateSecret(secret); err != nil {
		webhookError(c, "create webhook", err)
		return
	}
	repo, _, ok := webhookDeps(c)
	if !ok {
		return
	}
	created, err := repo.Create(c.Request.Context(), w, secret, userID)
	if err != nil {
		webhookError(c, "create webhook", err)
		return
	}
	webhookOK(c, http.StatusCreated, created)
}

// handleWebhookGet handles GET /api/v1/webhooks/:id.
//
//	@Summary		Get webhook
//	@Tags			Webhooks
//	@Produce		json
//	@Param			id	path		int	true	"Webhook ID"
//	@Success		200	{object}	map[string]interface{}	"success, data: Webhook"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/webhooks/{id} [get]
func handleWebhookGet(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	repo, _, ok := webhookDeps(c)
	if !ok {
		return
	}
	w, err := repo.Get(c.Request.Context(), id)
	if err != nil {
		webhookError(c, "load webhook", err)
		return
	}
	webhookOK(c, http.StatusOK, w)
}

// handleWebhookUpdate handles PUT /api/v1/webhooks/:id (partial update).
//
//	@Summary		Update webhook
//	@Description	Partial update: only fields present change; an empty secret removes it
//	@Tags			Webhooks
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int		true	"Webhook ID"
//	@Param			webhook	body		object	true	"Fields to change"
//	@Success		200		{object}	map[string]interface{}	"success, data: Webhook"
//	@Failure		400		{object}	map[string]interface{}	"Validation error"
//	@Failure		404		{object}	map[string]interface{}	"Not found"
//	@Failure		409		{object}	map[string]interface{}	"Name already used"
//	@Security		BearerAuth
//	@Router			/webhooks/{id} [put]
func handleWebhookUpdate(c *gin.Context) {
	userID, ok := webhookActor(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req webhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		webhookFail(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	repo, _, ok := webhookDeps(c)
	if !ok {
		return
	}
	w, err := repo.Get(c.Request.Context(), id)
	if err != nil {
		webhookError(c, "load webhook", err)
		return
	}
	req.apply(w)
	if err := w.Normalize(webhooks.IsEvent); err != nil {
		webhookError(c, "update webhook", err)
		return
	}
	var secret webhook.SecretChange
	if req.Secret != nil {
		if err := webhook.ValidateSecret(*req.Secret); err != nil {
			webhookError(c, "update webhook", err)
			return
		}
		secret = webhook.SecretChange{Set: true, Value: *req.Secret}
	}
	updated, err := repo.Update(c.Request.Context(), w, secret, userID)
	if err != nil {
		webhookError(c, "update webhook", err)
		return
	}
	webhookOK(c, http.StatusOK, updated)
}

// handleWebhookDelete handles DELETE /api/v1/webhooks/:id.
//
//	@Summary		Delete webhook
//	@Description	Deletes the webhook and its delivery log
//	@Tags			Webhooks
//	@Produce		json
//	@Param			id	path		int	true	"Webhook ID"
//	@Success		200	{object}	map[string]interface{}	"Deleted"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/webhooks/{id} [delete]
func handleWebhookDelete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	repo, _, ok := webhookDeps(c)
	if !ok {
		return
	}
	if err := repo.Delete(c.Request.Context(), id); err != nil {
		webhookError(c, "delete webhook", err)
		return
	}
	webhookOK(c, http.StatusOK, gin.H{"id": id})
}

// handleWebhookTest handles POST /api/v1/webhooks/:id/test: sends a
// webhook.test event now and returns the recorded delivery.
//
//	@Summary		Send test delivery
//	@Description	Sends a webhook.test event now (one attempt) and returns the recorded delivery
//	@Tags			Webhooks
//	@Produce		json
//	@Param			id	path		int	true	"Webhook ID"
//	@Success		200	{object}	map[string]interface{}	"success, data: WebhookDelivery"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/webhooks/{id}/test [post]
func handleWebhookTest(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	_, dispatcher, ok := webhookDeps(c)
	if !ok {
		return
	}
	d, err := dispatcher.Test(c.Request.Context(), id)
	if err != nil {
		webhookError(c, "send test delivery", err)
		return
	}
	webhookOK(c, http.StatusOK, d)
}

// handleWebhookDeliveries handles GET /api/v1/webhooks/:id/deliveries[?limit=N].
//
//	@Summary		List webhook deliveries
//	@Description	Newest deliveries first, without payload and response bodies
//	@Tags			Webhooks
//	@Produce		json
//	@Param			id		path		int	true	"Webhook ID"
//	@Param			limit	query		int	false	"1-200, default 50"
//	@Success		200		{object}	map[string]interface{}	"success, data: []WebhookDelivery"
//	@Failure		404		{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/webhooks/{id}/deliveries [get]
func handleWebhookDeliveries(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	limit := defaultDeliveryListLimit
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxDeliveryListLimit {
			webhookFail(c, http.StatusBadRequest, "limit must be between 1 and "+strconv.Itoa(maxDeliveryListLimit))
			return
		}
		limit = n
	}
	repo, _, ok := webhookDeps(c)
	if !ok {
		return
	}
	if _, err := repo.Get(c.Request.Context(), id); err != nil {
		webhookError(c, "load webhook", err)
		return
	}
	list, err := repo.ListDeliveries(c.Request.Context(), id, limit)
	if err != nil {
		webhookError(c, "list deliveries", err)
		return
	}
	webhookOK(c, http.StatusOK, list)
}

// handleWebhookDeliveryGet handles GET /api/v1/webhooks/deliveries/:id
// (includes payload and response body).
//
//	@Summary		Get webhook delivery
//	@Description	One delivery including payload and response body
//	@Tags			Webhooks
//	@Produce		json
//	@Param			id	path		int	true	"Delivery ID"
//	@Success		200	{object}	map[string]interface{}	"success, data: WebhookDelivery"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/webhooks/deliveries/{id} [get]
func handleWebhookDeliveryGet(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	repo, _, ok := webhookDeps(c)
	if !ok {
		return
	}
	d, err := repo.GetDelivery(c.Request.Context(), id)
	if err != nil {
		webhookError(c, "load delivery", err)
		return
	}
	webhookOK(c, http.StatusOK, d)
}

// handleWebhookRedeliver handles POST /api/v1/webhooks/deliveries/:id/redeliver:
// sends the delivery's payload again now and returns the new delivery.
//
//	@Summary		Redeliver webhook delivery
//	@Description	Sends the payload of an earlier delivery again now as a new delivery (one attempt)
//	@Tags			Webhooks
//	@Produce		json
//	@Param			id	path		int	true	"Delivery ID"
//	@Success		200	{object}	map[string]interface{}	"success, data: WebhookDelivery"
//	@Failure		404	{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/webhooks/deliveries/{id}/redeliver [post]
func handleWebhookRedeliver(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	_, dispatcher, ok := webhookDeps(c)
	if !ok {
		return
	}
	d, err := dispatcher.Redeliver(c.Request.Context(), id)
	if err != nil {
		webhookError(c, "redeliver", err)
		return
	}
	webhookOK(c, http.StatusOK, d)
}
