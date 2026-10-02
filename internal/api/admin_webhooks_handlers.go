package api

import (
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/webhook"
	"github.com/goatkit/goatflow/internal/webhooks"
)

func init() {
	routing.RegisterHandler("handleAdminWebhooks", handleAdminWebhooks)
}

// webhookEventLabel pairs a subscribable event with the i18n key of its
// description, so the page can show translated descriptions for the events
// it loads from GET /api/v1/webhooks/events.
type webhookEventLabel struct {
	Event string
	Key   string
}

// webhookEventLabelKey is the i18n key describing event ("ticket.created" ->
// "admin.webhooks.events.ticket_created").
func webhookEventLabelKey(event string) string {
	return "admin.webhooks.events." + strings.ReplaceAll(event, ".", "_")
}

// handleAdminWebhooks renders the outbound webhooks admin page
// (GET /admin/webhooks). The page lists and edits webhooks, sends test
// deliveries and inspects/redelivers deliveries through /api/v1/webhooks.
func handleAdminWebhooks(c *gin.Context) {
	renderer := getPongo2Renderer()
	if renderer == nil || renderer.TemplateSet() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	labels := make([]webhookEventLabel, 0, len(webhooks.Events))
	for _, e := range webhooks.Events {
		labels = append(labels, webhookEventLabel{Event: e.Event, Key: webhookEventLabelKey(e.Event)})
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/webhooks.pongo2", pongo2.Context{
		"EventLabels":           labels,
		"DefaultRetryCount":     webhook.DefaultRetryCount,
		"MaxRetryCount":         webhook.MaxRetryCount,
		"DefaultTimeoutSeconds": webhook.DefaultTimeoutSeconds,
		"MaxTimeoutSeconds":     webhook.MaxTimeoutSeconds,
		"ActivePage":            "admin",
		"User":                  getUserMapForTemplate(c),
	})
}
