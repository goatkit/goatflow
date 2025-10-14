package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListStatesAPI handles GET /api/v1/states
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
		WHERE valid_id = $1
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
	// If DB returned zero rows, fail clearly to avoid masking misconfigurations
	if len(items) == 0 {
		c.Header("X-Guru-Error", "States lookup returned 0 rows (check seeds/migrations)")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "states lookup returned 0 rows"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}
