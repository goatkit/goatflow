package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleCreateQueueAPI handles POST /api/v1/queues.
//
//	@Summary		Create queue
//	@Description	Create a new queue
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			queue	body		object	true	"Queue data (name, group_id, etc.)"
//	@Success		201		{object}	map[string]interface{}	"Created queue"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/queues [post]
func HandleCreateQueueAPI(c *gin.Context) {
	// Check authentication and admin permissions
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}
	_ = userID // TODO: Check admin permissions

	var req struct {
		Name            string  `json:"name" binding:"required"`
		GroupID         int     `json:"group_id"`
		SystemAddressID *int    `json:"system_address_id"`
		SalutationID    *int    `json:"salutation_id"`
		SignatureID     *int    `json:"signature_id"`
		UnlockTimeout   int     `json:"unlock_timeout"`
		FollowUpID      int     `json:"follow_up_id"`
		FollowUpLock    int     `json:"follow_up_lock"`
		Comments        *string `json:"comments"`
		GroupAccess     []int   `json:"group_access"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Check if queue with this name already exists
	var count int
	checkQuery := database.ConvertPlaceholders(`
        SELECT 1 FROM queue
        WHERE name = ? AND valid_id = 1
    `)
	row := db.QueryRow(checkQuery, req.Name)
	_ = row.Scan(&count) //nolint:errcheck // Count defaults to 0
	if count == 1 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "Queue with this name already exists"})
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to start transaction"})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Defaults mirror baseline bootstrap data (schema/baseline/required_lookups.sql)
	if req.GroupID == 0 {
		req.GroupID = 1
	} // Default to group 1
	if req.SystemAddressID == nil {
		d := 1
		req.SystemAddressID = &d
	}
	if req.SalutationID == nil {
		d := 1
		req.SalutationID = &d
	}
	if req.SignatureID == nil {
		d := 1
		req.SignatureID = &d
	}
	if req.FollowUpID == 0 {
		req.FollowUpID = 1
	} // Default allowed follow-up
	if req.FollowUpLock == 0 {
		req.FollowUpLock = 0
	}
	if req.UnlockTimeout < 0 {
		req.UnlockTimeout = 0
	}

	// Get user ID for DB parameters
	createdBy := GetUserIDFromCtx(c, 1)

	// Create queue - adapter handles MySQL vs PostgreSQL differences
	// Note: ? is used for both create_by and change_by (repeated placeholder)
	insertQuery := `
		INSERT INTO queue (
			name, group_id, system_address_id, salutation_id, signature_id,
			unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, 1,
			NOW(), ?, NOW(), ?
		) RETURNING id`

	queueID64, err := database.GetAdapter().InsertWithReturningTx(tx, insertQuery,
		req.Name,
		req.GroupID,
		req.SystemAddressID,
		req.SalutationID,
		req.SignatureID,
		req.UnlockTimeout,
		req.FollowUpID,
		req.FollowUpLock,
		req.Comments,
		createdBy,
	)
	if err != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Queue insert failed: %v", err),
		})
		return
	}
	queueID := int(queueID64)

	// Add group access if specified
	if len(req.GroupAccess) > 0 {
		for _, groupID := range req.GroupAccess {
			// Optional: only if auxiliary table exists in current schema
			groupInsert := database.ConvertPlaceholders(`INSERT INTO queue_group (queue_id, group_id) VALUES (?, ?)`)
			if _, err := tx.Exec(groupInsert, queueID, groupID); err != nil {
				// Swallow error if table doesn't exist (compatibility with minimal schema)
				// Comment out the early-return to avoid breaking core creation
				// c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to set group access: %v", err)})
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to commit transaction"})
		return
	}

	// Return created queue
	response := gin.H{
		"id":       queueID,
		"name":     req.Name,
		"group_id": req.GroupID,
		"comments": func() interface{} {
			if req.Comments != nil {
				return *req.Comments
			}
			return nil
		}(),
		"valid_id":     1,
		"group_access": req.GroupAccess,
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": response})
}
