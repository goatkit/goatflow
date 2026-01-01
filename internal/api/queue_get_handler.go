package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleGetQueueAPI handles GET /api/v1/queues/:id.
func HandleGetQueueAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission checks later

	// Parse queue ID
	if _, err := strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid queue ID"})
		return
	}

	// Delegate to unified OTRS-based handler for consistency
	// Delegate to unified OTRS-based handler for consistency.
	// Preserve API v1 response shape {success, data}
	// We call the underlying handler and adapt its response if needed.
	HandleAPIQueueGet(c)
}

// HandleGetQueueAgentsAPI handles GET /api/v1/queues/:id/agents.
func HandleGetQueueAgentsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission checks later

	// Parse queue ID
	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid queue ID"})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection not available"})
		return
	}
	sqlDB := db

	// Get agents with permissions for this queue
	agents, err := getAgentsForQueue(sqlDB, queueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to get agents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"agents":  agents,
	})
}

// getAgentsForQueue gets agents with permissions for a specific queue.
func getAgentsForQueue(db *sql.DB, queueID int) ([]gin.H, error) {
	log.Printf("DEBUG: getAgentsForQueue called for queueID %d", queueID)
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT DISTINCT u.id, u.login, u.first_name, u.last_name
		FROM users u
		JOIN group_user gu ON u.id = gu.user_id
		JOIN queue q ON q.group_id = gu.group_id
		WHERE q.id = ?
		AND gu.permission_key = 'rw'
		AND u.valid_id = 1
		ORDER BY u.first_name, u.last_name
	`), queueID)
	if err != nil {
		log.Printf("DEBUG: getAgentsForQueue query error: %v", err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var agents []gin.H
	for rows.Next() {
		var id int
		var login, firstName, lastName string
		if err := rows.Scan(&id, &login, &firstName, &lastName); err != nil {
			log.Printf("DEBUG: getAgentsForQueue scan error: %v", err)
			return nil, err
		}

		displayName := firstName
		if lastName != "" {
			displayName += " " + lastName
		}
		if displayName == "" {
			displayName = login
		}

		agents = append(agents, gin.H{
			"id":    id,
			"name":  displayName,
			"login": login,
		})
	}
	log.Printf("DEBUG: getAgentsForQueue returning %d agents: %+v", len(agents), agents)
	return agents, nil
}
