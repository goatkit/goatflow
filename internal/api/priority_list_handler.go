package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// HandleListPrioritiesAPI handles GET /api/v1/priorities.
// Supports optional filtering via ticket attribute relations:
//   - filter_attribute: The attribute to filter by (e.g., "Queue", "State")
//   - filter_value: The value of that attribute (e.g., "Sales", "new")
//
// HandleListPrioritiesAPI handles GET /api/v1/priorities.
//
//	@Summary		List priorities
//	@Description	Retrieve all ticket priorities
//	@Tags			Priorities
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of priorities"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/priorities [get]
func HandleListPrioritiesAPI(c *gin.Context) {
	// Require authentication similar to other admin lookups
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.Header("X-Guru-Error", "Priorities lookup failed: database unavailable")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "priorities lookup failed: database unavailable"})
		return
	}

	validParam := strings.ToLower(strings.TrimSpace(c.Query("valid")))

	query := `
		SELECT id, name, color, valid_id
		FROM ticket_priority
	`
	var rowsArgs []interface{}
	switch validParam {
	case "", "true", "1":
		query += " WHERE valid_id = ?"
		rowsArgs = append(rowsArgs, 1)
	case "false", "0":
		query += " WHERE valid_id <> ?"
		rowsArgs = append(rowsArgs, 1)
	case "all":
		// no additional filter
	default:
		// treat unexpected value as valid=true for safety
		query += " WHERE valid_id = ?"
		rowsArgs = append(rowsArgs, 1)
	}
	query += " ORDER BY id"

	rows, err := db.Query(database.ConvertPlaceholders(query), rowsArgs...)
	if err != nil {
		c.Header("X-Guru-Error", "Priorities lookup failed: query error")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch priorities"})
		return
	}
	defer rows.Close()

	items := []gin.H{}
	for rows.Next() {
		var id, validID int
		var name, color string
		if err := rows.Scan(&id, &name, &color, &validID); err != nil {
			c.Header("X-Guru-Error", "Priorities lookup failed: scan error")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch priorities"})
			return
		}
		items = append(items, gin.H{"id": id, "name": name, "color": color, "valid_id": validID})
	}
	if err := rows.Err(); err != nil {
		c.Header("X-Guru-Error", "Priorities lookup failed: query error")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch priorities"})
		return
	}

	// Apply ticket attribute relations filtering if requested
	filterAttr := c.Query("filter_attribute")
	filterValue := c.Query("filter_value")
	if filterAttr != "" && filterValue != "" {
		if items, err = filterByTicketAttributeRelations(c, db, items, "Priority", filterAttr, filterValue); err != nil {
			respondAttributeRelationFilterError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}
