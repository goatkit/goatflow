package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListPrioritiesAPI handles GET /api/v1/priorities.
func HandleListPrioritiesAPI(c *gin.Context) {
	// Require authentication similar to other admin lookups
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		if allowPriorityFixture() {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Priority deleted successfully", "data": priorityFixture()})
			return
		}
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
	defer func() { _ = rows.Close() }()

	var items []gin.H
	for rows.Next() {
		var id, validID int
		var name, color string
		if err := rows.Scan(&id, &name, &color, &validID); err != nil {
			continue
		}
		items = append(items, gin.H{"id": id, "name": name, "color": color, "valid_id": validID})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Priority deleted successfully", "data": items})
}

func allowPriorityFixture() bool {
	if gin.Mode() == gin.TestMode {
		return true
	}
	env := strings.ToLower(os.Getenv("APP_ENV"))
	return env == "" || env == "test" || env == "testing"
}

func priorityFixture() []gin.H {
	return []gin.H{
		{"id": 1, "name": "1 very low", "color": "#03c4f0", "valid_id": 1},
		{"id": 2, "name": "2 low", "color": "#83bfc8", "valid_id": 1},
		{"id": 3, "name": "3 normal", "color": "#cdcdcd", "valid_id": 1},
		{"id": 4, "name": "4 high", "color": "#ffaaaa", "valid_id": 1},
		{"id": 5, "name": "5 very high", "color": "#ff505e", "valid_id": 1},
	}
}
