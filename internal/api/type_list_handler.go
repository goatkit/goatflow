package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// HandleListTypesAPI handles GET /api/v1/types.
// Supports optional filtering via ticket attribute relations:
//   - filter_attribute: The attribute to filter by (e.g., "Queue", "Service")
//   - filter_value: The value of that attribute (e.g., "Sales", "Gold Support")
//
// The list is restricted to valid types unless ?valid=false|0 or ?valid=all is given.
//
//	@Summary		List types
//	@Description	Retrieve ticket types (valid ones by default)
//	@Tags			Types
//	@Accept			json
//	@Produce		json
//	@Param			valid	query		string	false	"Filter by validity (true/1 = valid (default), false/0 = invalid, all)"
//	@Success		200		{object}	map[string]interface{}	"List of types"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Failure		500		{object}	map[string]interface{}	"Lookup failed"
//	@Security		BearerAuth
//	@Router			/types [get]
func HandleListTypesAPI(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "types lookup failed: database unavailable"})
		return
	}

	query := `SELECT id, name, valid_id FROM ticket_type`
	var args []interface{}
	switch strings.ToLower(strings.TrimSpace(c.Query("valid"))) {
	case "false", "0":
		query += " WHERE valid_id <> ?"
		args = append(args, 1)
	case "all":
	default:
		query += " WHERE valid_id = ?"
		args = append(args, 1)
	}
	query += " ORDER BY id"

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "types lookup failed"})
		return
	}
	defer rows.Close()

	items := []gin.H{}
	for rows.Next() {
		var id, validID int
		var name string
		if err := rows.Scan(&id, &name, &validID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "types lookup failed"})
			return
		}
		items = append(items, gin.H{"id": id, "name": name, "valid_id": validID})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "types lookup failed"})
		return
	}

	// Apply ticket attribute relations filtering if requested
	filterAttr := c.Query("filter_attribute")
	filterValue := c.Query("filter_value")
	if filterAttr != "" && filterValue != "" {
		if items, err = filterByTicketAttributeRelations(c, db, items, "Type", filterAttr, filterValue); err != nil {
			respondAttributeRelationFilterError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}
