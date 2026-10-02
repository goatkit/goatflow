package api

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/middleware"
)

// handleAdminLookups is already defined in htmx_routes.go for templates

// lookupFormData loads the translated lookup lists for the request language.
// On failure it writes a 500 response and returns false.
func lookupFormData(c *gin.Context) (*models.TicketFormData, bool) {
	formData, err := GetLookupService().GetTicketFormDataWithLang(middleware.GetLanguage(c))
	if err != nil {
		log.Printf("lookup handlers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load lookup data"})
		return nil, false
	}
	return formData, true
}

// HandleGetQueues returns list of queues as JSON or HTML options for HTMX.
func HandleGetQueues(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

	formData, ok := lookupFormData(c)
	if !ok {
		return
	}
	queues := formData.Queues

	if isHTMX {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<option value="">Select queue</option>`)
		for _, queue := range queues {
			if queue.Active {
				_, _ = c.Writer.WriteString(fmt.Sprintf(`<option value="%d">%s</option>`, queue.ID, template.HTMLEscapeString(queue.Name))) //nolint:errcheck // Best-effort HTML write
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": queues})
}

// HandleGetPriorities returns list of priorities as JSON or HTML options for HTMX.
func HandleGetPriorities(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

	formData, ok := lookupFormData(c)
	if !ok {
		return
	}

	if isHTMX {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<option value="">Select priority</option>`)
		for _, priority := range formData.Priorities {
			if priority.Active {
				selected := ""
				if priority.Value == "normal" {
					selected = " selected"
				}
				_, _ = c.Writer.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, template.HTMLEscapeString(priority.Value), selected, template.HTMLEscapeString(priority.Label))) //nolint:errcheck // Best-effort HTML write
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData.Priorities,
	})
}

// HandleGetTypes returns the valid ticket_type rows (id/value/label/order/active)
// as JSON, or as HTML <option> elements (value = type id) for HTMX.
func HandleGetTypes(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

	formData, ok := lookupFormData(c)
	if !ok {
		return
	}

	if isHTMX {
		// Return HTML options for HTMX
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<option value="">Select type</option>`)
		for _, item := range formData.Types {
			if item.Active {
				_, _ = c.Writer.WriteString(fmt.Sprintf(`<option value="%d">%s</option>`, item.ID, template.HTMLEscapeString(item.Label))) //nolint:errcheck // Best-effort HTML write
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": formData.Types})
}

// HandleGetStatuses returns list of ticket statuses as JSON.
func HandleGetStatuses(c *gin.Context) {
	formData, ok := lookupFormData(c)
	if !ok {
		return
	}
	statuses := formData.Statuses

	c.JSON(http.StatusOK, gin.H{"success": true, "data": statuses})
}

// HandleGetFormData returns form data for ticket creation as JSON.
func HandleGetFormData(c *gin.Context) {
	formData, ok := lookupFormData(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData,
	})
}

// HandleInvalidateLookupCache forces a refresh of the lookup cache.
func HandleInvalidateLookupCache(c *gin.Context) {
	userRole := c.GetString("user_role")
	if userRole != "Admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Admin access required",
		})
		return
	}

	lookupService := GetLookupService()
	lookupService.InvalidateCache()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Lookup cache invalidated successfully",
	})
}
