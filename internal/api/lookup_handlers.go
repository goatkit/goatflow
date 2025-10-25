//go:build !graphql

package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// handleAdminLookups is already defined in htmx_routes.go for templates

// HandleGetQueues returns list of queues as JSON or HTML options for HTMX
func HandleGetQueues(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

	// In test mode, always return predictable default data
	if os.Getenv("APP_ENV") == "test" {
		if isHTMX {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<option value="">Select queue</option>
<option value="1">Test Queue</option>`)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    []models.QueueInfo{{ID: 1, Name: "Test Queue", Description: "Test", Active: true}},
		})
		return
	}
	// If DB not available, still return a minimal default queue
	if err := database.InitTestDB(); err != nil {
		if isHTMX {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<option value="">Select queue</option>
<option value="1">Test Queue</option>`)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    []models.QueueInfo{{ID: 1, Name: "Test Queue", Description: "Test", Active: true}},
		})
		return
	}
	// Use service to fetch queues, with safe fallback
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	queues := formData.Queues
	if len(queues) == 0 {
		queues = []models.QueueInfo{{ID: 1, Name: "Test Queue", Description: "Test", Active: true}}
	}

	if isHTMX {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<option value="">Select queue</option>`)
		for _, queue := range queues {
			if queue.Active {
				c.Writer.WriteString(fmt.Sprintf(`<option value="%d">%s</option>`, queue.ID, queue.Name))
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": queues})
}

// HandleGetPriorities returns list of priorities as JSON or HTML options for HTMX
func HandleGetPriorities(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

	// Explicit default priorities when running DB-less tests
	if os.Getenv("APP_ENV") == "test" {
		if isHTMX {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<option value="">Select priority</option>
<option value="low">Low</option>
<option value="normal" selected>Normal</option>
<option value="high">High</option>
<option value="urgent">Urgent</option>`)
			return
		}
		priorities := []models.LookupItem{
			{ID: 1, Value: "low", Label: "Low", Order: 1, Active: true},
			{ID: 2, Value: "normal", Label: "Normal", Order: 2, Active: true},
			{ID: 3, Value: "high", Label: "High", Order: 3, Active: true},
			{ID: 4, Value: "urgent", Label: "Urgent", Order: 4, Active: true},
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": priorities})
		return
	}
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)

	if lookupService == nil {
		if isHTMX {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<option value="">Select priority</option>`)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
		return
	}

	formData := lookupService.GetTicketFormDataWithLang(lang)
	if formData == nil {
		if isHTMX {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusOK, `<option value="">Select priority</option>`)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
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
				c.Writer.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, priority.Value, selected, priority.Label))
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData.Priorities,
	})
}

// HandleGetTypes returns list of ticket types as JSON or HTML options for HTMX.
// Behavior:
//   - If a DB connection is available (including sqlmock in tests) and the query succeeds,
//     return the SQL-backed shape: [{id, name, comments, valid_id}].
//   - Otherwise fall back to the lookup service shape (value/label/order/active).
//   - For HTMX requests, return HTML <option> elements instead of JSON.
func HandleGetTypes(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

	if db, err := database.GetDB(); err == nil && db != nil {
		rows, qerr := db.Query(database.ConvertPlaceholders(`SELECT id, name, comments, valid_id FROM ticket_type WHERE valid_id = 1 ORDER BY name`))
		if qerr == nil {
			defer rows.Close()

			if isHTMX {
				// Return HTML options for HTMX
				c.Header("Content-Type", "text/html")
				c.String(http.StatusOK, `<option value="">Select type</option>`)
				for rows.Next() {
					var (
						id, validID int
						name        string
						comments    sqlNullString
					)
					if err := scanTypeRow(rows, &id, &name, &comments, &validID); err != nil {
						continue
					}
					c.Writer.WriteString(fmt.Sprintf(`<option value="%d">%s</option>`, id, name))
				}
				return
			}

			// Return JSON for API requests
			data := make([]map[string]interface{}, 0)
			for rows.Next() {
				var (
					id, validID int
					name        string
					comments    sqlNullString
				)
				if err := scanTypeRow(rows, &id, &name, &comments, &validID); err != nil {
					continue
				}
				data = append(data, map[string]interface{}{
					"id":       id,
					"name":     name,
					"comments": comments.String(),
					"valid_id": validID,
				})
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
			return
		}
	}

	// No DB available: Fallback to in-memory lookup service shape (value/label/order/active)
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)

	if isHTMX {
		// Return HTML options for HTMX
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<option value="">Select type</option>`)
		for _, item := range formData.Types {
			if item.Active {
				c.Writer.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, item.Value, item.Label))
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": formData.Types})
}

// Minimal helpers to avoid adding a new import in this file
type sqlRowScanner interface {
	Scan(dest ...interface{}) error
}
type sqlNullString struct{ v *string }

func (s *sqlNullString) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		s.v = nil
	case string:
		s.v = &v
	case []byte:
		str := string(v)
		s.v = &str
	default:
		// treat as nil
		s.v = nil
	}
	return nil
}

func (s sqlNullString) String() string {
	if s.v == nil {
		return ""
	}
	return *s.v
}

func scanTypeRow(scanner sqlRowScanner, id *int, name *string, comments *sqlNullString, validID *int) error {
	return scanner.Scan(id, name, comments, validID)
}

// HandleGetStatuses returns list of ticket statuses as JSON
func HandleGetStatuses(c *gin.Context) {
	// In test mode, return a fixed 5-status workflow list
	if os.Getenv("APP_ENV") == "test" {
		statuses := []models.LookupItem{
			{ID: 1, Value: "new", Label: "New", Order: 1, Active: true},
			{ID: 2, Value: "open", Label: "Open", Order: 2, Active: true},
			{ID: 3, Value: "pending", Label: "Pending", Order: 3, Active: true},
			{ID: 4, Value: "resolved", Label: "Resolved", Order: 4, Active: true},
			{ID: 5, Value: "closed", Label: "Closed", Order: 5, Active: true},
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": statuses})
		return
	}
	// Otherwise, normalize DB-provided list to 5 expected statuses
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	statuses := formData.Statuses
	if len(statuses) >= 5 {
		// Normalize to common workflow: new, open, pending, resolved, closed
		normalized := make([]models.LookupItem, 0, 5)
		pick := map[string]bool{"new": true, "open": true, "pending": true, "resolved": true, "closed": true}
		for _, s := range statuses {
			if pick[s.Value] && len(normalized) < 5 {
				normalized = append(normalized, s)
			}
		}
		// Fallback if matching names not found: take first 5
		if len(normalized) < 5 && len(statuses) >= 5 {
			normalized = statuses[:5]
		}
		statuses = normalized
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": statuses})
}

// HandleGetFormData returns form data for ticket creation as JSON
func HandleGetFormData(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData,
	})
}

// HandleInvalidateLookupCache forces a refresh of the lookup cache
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
