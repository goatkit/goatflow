package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListTypesAPI handles GET /api/v1/types.
// Supports optional filtering via ticket attribute relations:
//   - filter_attribute: The attribute to filter by (e.g., "Queue", "Service")
//   - filter_value: The value of that attribute (e.g., "Sales", "Gold Support")
func HandleListTypesAPI(c *gin.Context) {
	// Optional auth for now (treat as public list if no token)
	db, err := database.GetDB()
	if err != nil || db == nil {
		// Fallback minimal list
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []gin.H{
			{"id": 1, "name": "incident", "label": "Incident"},
			{"id": 2, "name": "service_request", "label": "Service Request"},
		}})
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id
		FROM ticket_type
		ORDER BY id
	`))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var id, validID int
		var name, comments string
		if err := rows.Scan(&id, &name, &comments, &validID); err == nil {
			items = append(items, gin.H{"id": id, "name": name, "valid_id": validID})
		}
	}
	_ = rows.Err() //nolint:errcheck // Check for iteration errors

	// Apply ticket attribute relations filtering if requested
	filterAttr := c.Query("filter_attribute")
	filterValue := c.Query("filter_value")
	if filterAttr != "" && filterValue != "" {
		items = filterByTicketAttributeRelations(c, db, items, "Type", filterAttr, filterValue)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}
