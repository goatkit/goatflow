package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func init() {
	// Register handlers during module initialization
	RegisterHandler("HandleListServicesAPI", HandleListServicesAPI)
}

// HandleListServicesAPI handles GET /api/v1/services.
// Supports optional filtering via ticket attribute relations:
//   - filter_attribute: The attribute to filter by (e.g., "Queue", "Type")
//   - filter_value: The value of that attribute (e.g., "Sales", "incident")
//   - valid: Filter by valid_id (1 = valid, 2 = invalid, "all" = no filter)
func HandleListServicesAPI(c *gin.Context) {
	// Require authentication
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection not available"})
		return
	}

	validParam := strings.ToLower(strings.TrimSpace(c.Query("valid")))

	query := `
		SELECT id, name, comments, valid_id
		FROM service
	`
	var args []interface{}
	switch validParam {
	case "", "true", "1":
		query += " WHERE valid_id = ?"
		args = append(args, 1)
	case "false", "0":
		query += " WHERE valid_id <> ?"
		args = append(args, 1)
	case "all":
		// no additional filter
	default:
		// treat unexpected value as valid=true for safety
		query += " WHERE valid_id = ?"
		args = append(args, 1)
	}
	query += " ORDER BY name"

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch services"})
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var id, validID int
		var name, comments string
		if err := rows.Scan(&id, &name, &comments, &validID); err != nil {
			continue
		}
		items = append(items, gin.H{
			"id":       id,
			"name":     name,
			"comments": comments,
			"valid_id": validID,
		})
	}
	_ = rows.Err() //nolint:errcheck // Check for iteration errors

	// Apply ticket attribute relations filtering if requested
	filterAttr := c.Query("filter_attribute")
	filterValue := c.Query("filter_value")
	if filterAttr != "" && filterValue != "" {
		items = filterByTicketAttributeRelations(c, db, items, "Service", filterAttr, filterValue)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}
