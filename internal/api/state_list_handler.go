package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/services/ticketattributerelations"
)

// HandleListStatesAPI handles GET /api/v1/states.
// Supports optional filtering via ticket attribute relations:
//   - filter_attribute: The attribute to filter by (e.g., "Queue", "Priority")
//   - filter_value: The value of that attribute (e.g., "Sales", "new")
//
// HandleListStatesAPI handles GET /api/v1/states.
//
//	@Summary		List states
//	@Description	Retrieve all ticket states
//	@Tags			States
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of states"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/states [get]
func HandleListStatesAPI(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.Header("X-Guru-Error", "States lookup failed: database unavailable")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "states lookup failed: database unavailable"})
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name, valid_id
		FROM ticket_state
		WHERE valid_id = ?
		ORDER BY id
	`), 1)
	if err != nil {
		c.Header("X-Guru-Error", "States lookup failed: query error")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "states lookup failed: query error"})
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var id, validID int
		var name string
		if err := rows.Scan(&id, &name, &validID); err == nil {
			items = append(items, gin.H{"id": id, "name": name, "valid_id": validID})
		}
	}
	_ = rows.Err() //nolint:errcheck // Check for iteration errors
	// If DB returned zero rows, fail clearly to avoid masking misconfigurations
	if len(items) == 0 {
		c.Header("X-Guru-Error", "States lookup returned 0 rows (check seeds/migrations)")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "states lookup returned 0 rows"})
		return
	}

	// Apply ticket attribute relations filtering if requested
	filterAttr := c.Query("filter_attribute")
	filterValue := c.Query("filter_value")
	if filterAttr != "" && filterValue != "" {
		items = filterByTicketAttributeRelations(c, db, items, "State", filterAttr, filterValue)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// filterByTicketAttributeRelations filters items based on ticket attribute relations.
// targetAttribute is what we're filtering (e.g., "State", "Priority")
// filterAttribute is what we're filtering by (e.g., "Queue")
// filterValue is the current value of filterAttribute (e.g., "Sales")
func filterByTicketAttributeRelations(c *gin.Context, db interface{}, items []gin.H, targetAttribute, filterAttribute, filterValue string) []gin.H {
	dbConn, err := database.GetDB()
	if err != nil || dbConn == nil {
		return items // Return unfiltered if DB unavailable
	}

	svc := ticketattributerelations.NewService(dbConn)
	result, err := svc.EvaluateRelations(c.Request.Context(), filterAttribute, filterValue)
	if err != nil {
		return items // Return unfiltered on error
	}

	// Check if there are any restrictions for this target attribute
	allowedValues, hasRestrictions := result[targetAttribute]
	if !hasRestrictions || len(allowedValues) == 0 {
		return items // No restrictions, return all
	}

	// Build allowed set for fast lookup
	allowedSet := make(map[string]bool)
	for _, v := range allowedValues {
		allowedSet[v] = true
	}

	// Filter items by name
	var filtered []gin.H
	for _, item := range items {
		name, ok := item["name"].(string)
		if ok && allowedSet[name] {
			filtered = append(filtered, item)
		}
	}

	// If filtering would remove all items, return original list
	// (this prevents broken UX if relations are misconfigured)
	if len(filtered) == 0 {
		return items
	}

	return filtered
}
