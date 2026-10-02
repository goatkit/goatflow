package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
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
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}
	userRole, _ := c.Get("user_role")
	if userRole != "Admin" {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin role required"})
		return
	}

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
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.GroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "group_id is required: every queue belongs to one group"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	groupName, err := lookupQueueGroup(db, req.GroupID)
	if err != nil {
		respondQueueGroupError(c, req.GroupID, err)
		return
	}

	// Check if queue with this name already exists (queue.name is UNIQUE,
	// including invalid queues).
	var count int
	checkQuery := database.ConvertPlaceholders(`
        SELECT 1 FROM queue
        WHERE name = ?
    `)
	row := db.QueryRow(checkQuery, req.Name)
	_ = row.Scan(&count) //nolint:errcheck // Count defaults to 0
	if count == 1 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "Queue with this name already exists"})
		return
	}

	// Defaults mirror baseline bootstrap data (schema/baseline/required_lookups.sql)
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
	if req.UnlockTimeout < 0 {
		req.UnlockTimeout = 0
	}

	// Get user ID for DB parameters
	createdBy := GetUserIDFromCtx(c, 1)

	// Create queue
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

	queueID64, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(insertQuery),
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
		createdBy,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Queue insert failed: %v", err),
		})
		return
	}
	queueID := int(queueID64)

	// Return created queue
	response := gin.H{
		"id":         queueID,
		"name":       req.Name,
		"group_id":   req.GroupID,
		"group_name": groupName,
		"groups":     queueGroupList(req.GroupID, groupName),
		"comments": func() interface{} {
			if req.Comments != nil {
				return *req.Comments
			}
			return nil
		}(),
		"valid_id": 1,
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": response})
}
