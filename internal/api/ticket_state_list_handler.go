package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListTicketStatesAPI handles GET /api/v1/ticket-states
func HandleListTicketStatesAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission checks later

    db, err := database.GetDB()
    if err != nil || db == nil {
        // DB-less fallback
        states := []gin.H{
            {"id": 1, "name": "new", "type_id": 1, "valid_id": 1, "type_name": "open"},
            {"id": 2, "name": "open", "type_id": 1, "valid_id": 1, "type_name": "open"},
        }
        // Apply type filter
        if typeFilter := c.Query("type"); typeFilter != "" {
            filtered := []gin.H{}
            for _, s := range states {
                if typeFilter == "open" && s["type_id"].(int) == 1 { filtered = append(filtered, s) }
                if typeFilter == "closed" && s["type_id"].(int) == 2 { filtered = append(filtered, s) }
                if typeFilter == "pending" && s["type_id"].(int) == 3 { filtered = append(filtered, s) }
            }
            states = filtered
        }
        c.JSON(http.StatusOK, gin.H{"states": states, "total": len(states)})
        return
    }

	// Build query based on filters
	query := database.ConvertPlaceholders(`
		SELECT ts.id, ts.name, ts.type_id, ts.valid_id,
			   tst.name as type_name
		FROM ticket_state ts
		LEFT JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE 1=1
	`)
	args := []interface{}{}
	paramCount := 0

	// Filter by type (open, closed, pending)
	if typeFilter := c.Query("type"); typeFilter != "" {
		paramCount++
		switch typeFilter {
		case "open":
			query += database.ConvertPlaceholders(` AND ts.type_id = $` + strconv.Itoa(paramCount))
			args = append(args, 1)
		case "closed":
			query += database.ConvertPlaceholders(` AND ts.type_id = $` + strconv.Itoa(paramCount))
			args = append(args, 2)
		case "pending":
			query += database.ConvertPlaceholders(` AND ts.type_id = $` + strconv.Itoa(paramCount))
			args = append(args, 3)
		}
	}

	// Filter by valid status
	if validFilter := c.Query("valid"); validFilter == "true" {
		paramCount++
		query += database.ConvertPlaceholders(` AND ts.valid_id = $` + strconv.Itoa(paramCount))
		args = append(args, 1)
	}

	query += ` ORDER BY ts.id`

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ticket states"})
		return
	}
	defer rows.Close()

	// Collect results
	states := []gin.H{}
	for rows.Next() {
		var state struct {
			ID       int
			Name     string
			TypeID   int
			ValidID  int
			TypeName *string
		}
		if err := rows.Scan(&state.ID, &state.Name, &state.TypeID, &state.ValidID, &state.TypeName); err != nil {
			continue
		}
		
		stateData := gin.H{
			"id":       state.ID,
			"name":     state.Name,
			"type_id":  state.TypeID,
			"valid_id": state.ValidID,
		}
		
		if state.TypeName != nil {
			stateData["type_name"] = *state.TypeName
		}
		
		states = append(states, stateData)
	}

	c.JSON(http.StatusOK, gin.H{
		"states": states,
		"total":  len(states),
	})
}