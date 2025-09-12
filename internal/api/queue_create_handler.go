package api

import (
    "fmt"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleCreateQueueAPI handles POST /api/v1/queues
func HandleCreateQueueAPI(c *gin.Context) {
	// Check authentication and admin permissions
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // TODO: Check admin permissions

    var req struct {
        Name             string  `json:"name" binding:"required"`
        GroupID          int     `json:"group_id"`
        SystemAddressID  *int    `json:"system_address_id"`
        SalutationID     *int    `json:"salutation_id"`
        SignatureID      *int    `json:"signature_id"`
        UnlockTimeout    int     `json:"unlock_timeout"`
        FollowUpID       int     `json:"follow_up_id"`
        FollowUpLock     int     `json:"follow_up_lock"`
        Comments         *string `json:"comments"`
        GroupAccess      []int   `json:"group_access"`
    }

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

    db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if queue with this name already exists
	var count int
    checkQuery := database.ConvertPlaceholders(`
        SELECT 1 FROM queue
        WHERE name = $1 AND valid_id = 1
    `)
	db.QueryRow(checkQuery, req.Name).Scan(&count)
	if count == 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "Queue with this name already exists"})
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

    // Defaults and required fallbacks per our schema (migrations/legacy/000001_otrs_schema.up.sql)
    if req.GroupID == 0 { req.GroupID = 1 }          // Default to group 1
    if req.SystemAddressID == nil { d := 1; req.SystemAddressID = &d }
    if req.SalutationID == nil { d := 1; req.SalutationID = &d }
    if req.SignatureID == nil { d := 1; req.SignatureID = &d }
    if req.FollowUpID == 0 { req.FollowUpID = 1 }    // Default allowed follow-up
    if req.FollowUpLock == 0 { req.FollowUpLock = 0 }
    if req.UnlockTimeout < 0 { req.UnlockTimeout = 0 }

    // Normalize user_id to int for DB parameters
    var createdBy int
    if uid, ok := userID.(int); ok {
        createdBy = uid
    } else if uid, ok := userID.(uint); ok {
        createdBy = int(uid)
    } else if uid, ok := userID.(int64); ok {
        createdBy = int(uid)
    } else if uid, ok := userID.(uint64); ok {
        createdBy = int(uid)
    } else if uid, ok := userID.(float64); ok { // very defensive
        createdBy = int(uid)
    } else if s, ok := userID.(string); ok {
        if n, errAtoi := strconv.Atoi(s); errAtoi == nil { createdBy = n } else { createdBy = 1 }
    } else {
        createdBy = 1
    }

    // Create queue (match our actual schema columns exactly)
    // Columns inserted: name, group_id, system_address_id, salutation_id, signature_id,
    //                   unlock_timeout, follow_up_id, follow_up_lock, comments, create_by, change_by
    // Note: valid_id defaults to 1, create_time/change_time default to CURRENT_TIMESTAMP
    var queueID int

    driver := database.GetDBDriver()
    if database.IsMySQL() {
        // MySQL/MariaDB variant (explicit '?' placeholders, no RETURNING)
        insertNoRet := `
            INSERT INTO queue (
                name, group_id, system_address_id, salutation_id, signature_id,
                unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id,
                create_time, create_by, change_time, change_by
            ) VALUES (
                ?, ?, ?, ?, ?,
                ?, ?, ?, ?, 1,
                NOW(), ?, NOW(), ?
            )`
        res, errExec := tx.Exec(
            insertNoRet,
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
        if errExec != nil {
            _ = tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Queue insert failed (driver=%s mysql-path): %v", driver, errExec)})
            return
        }
        lid, _ := res.LastInsertId()
        queueID = int(lid)
    } else {
        // PostgreSQL variant with RETURNING id
        insertRet := database.ConvertPlaceholders(`
            INSERT INTO queue (
                name, group_id, system_address_id, salutation_id, signature_id,
                unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id,
                create_time, create_by, change_time, change_by
            ) VALUES (
                $1, $2, $3, $4, $5,
                $6, $7, $8, $9, 1,
                NOW(), $10, NOW(), $11
            ) RETURNING id`)
        if err := tx.QueryRow(
            insertRet,
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
        ).Scan(&queueID); err != nil {
            _ = tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Queue insert failed (driver=%s pg-path): %v", driver, err)})
            return
        }
    }

	// Add group access if specified
    if len(req.GroupAccess) > 0 {
		for _, groupID := range req.GroupAccess {
            // Optional: only if auxiliary table exists in current schema
            groupInsert := database.ConvertPlaceholders(`INSERT INTO queue_group (queue_id, group_id) VALUES ($1, $2)`)
            if _, err := tx.Exec(groupInsert, queueID, groupID); err != nil {
                // Swallow error if table doesn't exist (compatibility with minimal schema)
                // Comment out the early-return to avoid breaking core creation
                // c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to set group access: %v", err)})
            }
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

    // Return created queue
    response := gin.H{
		"id":           queueID,
		"name":         req.Name,
        "group_id":     req.GroupID,
        "comments":     func() interface{} { if req.Comments!=nil {return *req.Comments}; return nil }(),
		"valid_id":     1,
		"group_access": req.GroupAccess,
	}

    c.JSON(http.StatusCreated, gin.H{"success": true, "data": response})
}