//go:build !graphql

package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

// handleAdminLookups is already defined in htmx_routes.go for templates

// HandleGetQueues returns list of queues as JSON or HTML options for HTMX.
func HandleGetQueues(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	queues := formData.Queues

	if isHTMX {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<option value="">Select queue</option>`)
		for _, queue := range queues {
			if queue.Active {
				_, _ = c.Writer.WriteString(fmt.Sprintf(`<option value="%d">%s</option>`, queue.ID, queue.Name)) //nolint:errcheck // Best-effort HTML write
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": queues})
}

// HandleGetPriorities returns list of priorities as JSON or HTML options for HTMX.
func HandleGetPriorities(c *gin.Context) {
	isHTMX := c.GetHeader("HX-Request") == "true"

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
				_, _ = c.Writer.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, priority.Value, selected, priority.Label)) //nolint:errcheck // Best-effort HTML write
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
		typeQuery := `SELECT id, name, comments, valid_id FROM ticket_type WHERE valid_id = 1 ORDER BY name`
		rows, qerr := db.Query(database.ConvertPlaceholders(typeQuery))
		if qerr == nil {
			defer rows.Close()

			if isHTMX {
				var builder strings.Builder
				builder.WriteString(`<option value="">Select type</option>`)
				wroteOption := false
				for rows.Next() {
					var (
						id, validID int
						name        string
						comments    sqlNullString
					)
					if err := scanTypeRow(rows, &id, &name, &comments, &validID); err != nil {
						continue
					}
					builder.WriteString(fmt.Sprintf(`<option value="%d">%s</option>`, id, name))
					wroteOption = true
				}
				if err := rows.Err(); err == nil && wroteOption {
					c.Header("Content-Type", "text/html")
					c.String(http.StatusOK, builder.String())
					return
				}
			}

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
			if err := rows.Err(); err == nil && len(data) > 0 {
				c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
				return
			}
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
				_, _ = c.Writer.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, item.Value, item.Label)) //nolint:errcheck // Best-effort HTML write
			}
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": formData.Types})
}

// Minimal helpers to avoid adding a new import in this file.
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

// HandleGetStatuses returns list of ticket statuses as JSON.
func HandleGetStatuses(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	statuses := formData.Statuses

	c.JSON(http.StatusOK, gin.H{"success": true, "data": statuses})
}

// HandleGetFormData returns form data for ticket creation as JSON.
func HandleGetFormData(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)

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
