package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAPIQueueGet handles GET /api/queues/:id
func HandleAPIQueueGet(c *gin.Context) {
	queueID := c.Param("id")
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid queue ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	var queue struct {
		ID              int
		Name            string
		GroupID         int
		SystemAddressID sql.NullInt32
		Comments        sql.NullString
		UnlockTimeout   sql.NullInt32
		FollowUpLock    sql.NullInt32
		ValidID         int
	}

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT id, name, group_id, system_address_id, comments, 
		       unlock_timeout, follow_up_lock, valid_id
		FROM queue WHERE id = $1
	`), id).Scan(&queue.ID, &queue.Name, &queue.GroupID, &queue.SystemAddressID,
		&queue.Comments, &queue.UnlockTimeout, &queue.FollowUpLock, &queue.ValidID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Queue not found"})
		return
	}

	groups := make([]gin.H, 0)
	groupRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT g.id, g.name
		FROM groups g
		INNER JOIN queue_group qg ON g.id = qg.group_id
		WHERE qg.queue_id = $1
		ORDER BY g.name
	`), queue.ID)
	if err == nil {
		for groupRows.Next() {
			var gid int
			var gname string
			if scanErr := groupRows.Scan(&gid, &gname); scanErr == nil {
				groups = append(groups, gin.H{"id": gid, "name": gname})
			}
		}
		groupRows.Close()
	}

	response := gin.H{
		"id":       queue.ID,
		"name":     queue.Name,
		"group_id": queue.GroupID,
		"valid_id": queue.ValidID,
		"groups":   groups,
	}
	if queue.SystemAddressID.Valid {
		response["system_address_id"] = queue.SystemAddressID.Int32
	}
	if queue.Comments.Valid {
		response["comments"] = queue.Comments.String
	}
	if queue.UnlockTimeout.Valid {
		response["unlock_timeout"] = queue.UnlockTimeout.Int32
	}
	if queue.FollowUpLock.Valid {
		response["follow_up_lock"] = queue.FollowUpLock.Int32
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
}

// HandleAPIQueueDetails handles GET /api/queues/:id/details
func HandleAPIQueueDetails(c *gin.Context) {
	queueID := c.Param("id")
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get queue details with group + system address metadata
	var queue struct {
		ID                   int            `json:"id"`
		Name                 string         `json:"name"`
		GroupID              int            `json:"group_id"`
		GroupName            sql.NullString `json:"group_name"`
		SystemAddressID      *int           `json:"system_address_id"`
		SystemAddressEmail   sql.NullString `json:"system_address_email"`
		SystemAddressDisplay sql.NullString `json:"system_address_display"`
		Comments             sql.NullString `json:"comments"`
		UnlockTimeout        sql.NullInt32  `json:"unlock_timeout"`
		FollowUpLock         sql.NullInt32  `json:"follow_up_lock"`
		ValidID              int            `json:"valid_id"`
		TicketCount          int            `json:"ticket_count"`
	}

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT q.id, q.name, q.group_id, g.name as group_name,
		       q.system_address_id, sa.value0 AS system_address_email,
		       sa.value1 AS system_address_display,
		       q.comments, q.unlock_timeout, q.follow_up_lock, q.valid_id,
		       (SELECT COUNT(*) FROM ticket WHERE queue_id = q.id) as ticket_count
		FROM queue q
		LEFT JOIN groups g ON q.group_id = g.id
		LEFT JOIN system_address sa ON q.system_address_id = sa.id
		WHERE q.id = $1
	`), id).Scan(&queue.ID, &queue.Name, &queue.GroupID, &queue.GroupName,
		&queue.SystemAddressID, &queue.SystemAddressEmail, &queue.SystemAddressDisplay,
		&queue.Comments, &queue.UnlockTimeout, &queue.FollowUpLock, &queue.ValidID, &queue.TicketCount)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	response := gin.H{
		"id":           queue.ID,
		"name":         queue.Name,
		"group_id":     queue.GroupID,
		"valid_id":     queue.ValidID,
		"ticket_count": queue.TicketCount,
	}
	if queue.GroupName.Valid {
		response["group_name"] = queue.GroupName.String
	}
	if queue.SystemAddressID != nil {
		response["system_address_id"] = *queue.SystemAddressID
	}
	if queue.SystemAddressEmail.Valid {
		response["system_address_email"] = queue.SystemAddressEmail.String
	}
	if queue.SystemAddressDisplay.Valid {
		response["system_address_display"] = queue.SystemAddressDisplay.String
	}
	if queue.Comments.Valid {
		response["comments"] = queue.Comments.String
	}
	if queue.UnlockTimeout.Valid {
		response["unlock_timeout"] = queue.UnlockTimeout.Int32
	}
	if queue.FollowUpLock.Valid {
		response["follow_up_lock"] = queue.FollowUpLock.Int32
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
}

// HandleAPIQueueStatus handles PUT /api/queues/:id/status
func HandleAPIQueueStatus(c *gin.Context) {
	queueID := c.Param("id")
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	var req struct {
		ValidID int `json:"valid_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Update queue status
	_, err = db.Exec(database.ConvertPlaceholders(`UPDATE queue SET valid_id = $1 WHERE id = $2`), req.ValidID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update queue status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Queue status updated"})
}
