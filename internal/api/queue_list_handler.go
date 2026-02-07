package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/services"
)

// HandleListQueuesAPI handles GET /api/v1/queues.
//
//	@Summary		List queues
//	@Description	Retrieve all queues the user has access to (RBAC filtered)
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			per_page	query		int		false	"Items per page"	default(20)
//	@Success		200			{object}	map[string]interface{}	"List of queues"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/queues [get]
func HandleListQueuesAPI(c *gin.Context) {
	// Check authentication
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	// Get user ID for RBAC filtering
	var userID int
	switch v := userIDVal.(type) {
	case int:
		userID = v
	case int64:
		userID = int(v)
	case uint:
		userID = int(v)
	case uint64:
		userID = int(v)
	case float64:
		userID = int(v)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Invalid user context",
		})
		return
	}

	// Parse query parameters
	validFilter := c.Query("valid") // "1" for valid only, "2" for invalid only, "" for all
	includeStats := c.Query("include_stats") == "true"

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection not available",
		})
		return
	}

	// RBAC: Get user's queue permissions to filter results
	permSvc := services.NewPermissionService(db)
	userQueuePerms, err := permSvc.GetUserQueuePermissions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check permissions",
		})
		return
	}

	// If user has no queue permissions, return empty list
	if len(userQueuePerms) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    []map[string]interface{}{},
		})
		return
	}

	// Build list of accessible queue IDs for SQL IN clause
	accessibleQueueIDs := make([]interface{}, 0, len(userQueuePerms))
	for queueID := range userQueuePerms {
		accessibleQueueIDs = append(accessibleQueueIDs, queueID)
	}

	// Build the query incrementally so placeholders convert correctly
	query := `
		SELECT 
			q.id,
			q.name,
			q.group_id,
			q.system_address_id,
			q.salutation_id,
			q.signature_id,
			q.unlock_timeout,
			q.follow_up_id,
			q.follow_up_lock,
			q.comments,
			q.valid_id,
			q.create_time,
			q.change_time,
			g.name as group_name
		FROM queue q
		LEFT JOIN groups g ON q.group_id = g.id`

	args := []interface{}{}
	conditions := []string{}

	// RBAC: Filter to only queues the user has access to
	placeholders := make([]string, len(accessibleQueueIDs))
	for i := range accessibleQueueIDs {
		placeholders[i] = "?"
	}
	conditions = append(conditions, "q.id IN ("+strings.Join(placeholders, ",")+")")
	args = append(args, accessibleQueueIDs...)

	// Add valid filter if specified
	if validFilter != "" {
		if valid, err := strconv.Atoi(validFilter); err == nil && (valid == 1 || valid == 2) {
			conditions = append(conditions, "q.valid_id = ?")
			args = append(args, valid)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY q.name"
	query = database.ConvertPlaceholders(query)

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve queues",
		})
		return
	}
	defer rows.Close()

	queues := []map[string]interface{}{}
	for rows.Next() {
		var queue struct {
			ID              int            `json:"id"`
			Name            string         `json:"name"`
			GroupID         sql.NullInt32  `json:"-"`
			SystemAddressID sql.NullInt32  `json:"-"`
			SalutationID    sql.NullInt32  `json:"-"`
			SignatureID     sql.NullInt32  `json:"-"`
			UnlockTimeout   sql.NullInt32  `json:"-"`
			FollowUpID      sql.NullInt32  `json:"-"`
			FollowUpLock    sql.NullInt32  `json:"-"`
			Comments        sql.NullString `json:"-"`
			ValidID         int            `json:"valid_id"`
			CreateTime      sql.NullTime   `json:"-"`
			ChangeTime      sql.NullTime   `json:"-"`
			GroupName       sql.NullString `json:"-"`
		}

		err := rows.Scan(
			&queue.ID,
			&queue.Name,
			&queue.GroupID,
			&queue.SystemAddressID,
			&queue.SalutationID,
			&queue.SignatureID,
			&queue.UnlockTimeout,
			&queue.FollowUpID,
			&queue.FollowUpLock,
			&queue.Comments,
			&queue.ValidID,
			&queue.CreateTime,
			&queue.ChangeTime,
			&queue.GroupName,
		)
		if err != nil {
			continue
		}

		queueMap := map[string]interface{}{
			"id":       queue.ID,
			"name":     queue.Name,
			"valid_id": queue.ValidID,
			"valid":    queue.ValidID == 1,
		}

		if queue.GroupID.Valid {
			queueMap["group_id"] = queue.GroupID.Int32
			if queue.GroupName.Valid {
				queueMap["group_name"] = queue.GroupName.String
			}
		}
		if queue.SystemAddressID.Valid {
			queueMap["system_address_id"] = queue.SystemAddressID.Int32
		}
		if queue.UnlockTimeout.Valid {
			queueMap["unlock_timeout"] = queue.UnlockTimeout.Int32
		}
		if queue.FollowUpID.Valid {
			queueMap["follow_up_id"] = queue.FollowUpID.Int32
		}
		if queue.FollowUpLock.Valid {
			queueMap["follow_up_lock"] = queue.FollowUpLock.Int32
		}
		if queue.Comments.Valid {
			queueMap["comment"] = queue.Comments.String
		}
		if queue.CreateTime.Valid {
			queueMap["create_time"] = queue.CreateTime.Time.Format("2006-01-02T15:04:05Z")
		}
		if queue.ChangeTime.Valid {
			queueMap["change_time"] = queue.ChangeTime.Time.Format("2006-01-02T15:04:05Z")
		}

		// Include statistics if requested
		if includeStats {
			// Get ticket counts for this queue
			statsQuery := database.ConvertPlaceholders(`
                SELECT 
                    COUNT(*) as total,
                    COUNT(CASE WHEN ticket_state_id IN (1, 4) THEN 1 END) as open_count,
                    COUNT(CASE WHEN ticket_state_id IN (2, 3) THEN 1 END) as closed_count,
                    COUNT(CASE WHEN ticket_state_id IN (5) THEN 1 END) as pending_count
                FROM ticket
                WHERE queue_id = ?
            `)

			var total, openCount, closedCount, pendingCount int
			err = db.QueryRow(statsQuery, queue.ID).Scan(&total, &openCount, &closedCount, &pendingCount)
			if err == nil {
				queueMap["ticket_count"] = total
				queueMap["open_tickets"] = openCount
				queueMap["closed_tickets"] = closedCount
				queueMap["pending_tickets"] = pendingCount
			}
		}

		// Get groups that have access to this queue
		groupQuery := database.ConvertPlaceholders(`
			SELECT DISTINCT g.id, g.name
			FROM groups g
			INNER JOIN queue_group qg ON g.id = qg.group_id
			WHERE qg.queue_id = ?
			ORDER BY g.name
		`)

		groupRows, err := db.Query(groupQuery, queue.ID)
		if err == nil {
			defer groupRows.Close()
			groups := []map[string]interface{}{}
			for groupRows.Next() {
				var groupID int
				var groupName string
				if err := groupRows.Scan(&groupID, &groupName); err == nil {
					groups = append(groups, map[string]interface{}{
						"id":   groupID,
						"name": groupName,
					})
				}
			}
			_ = groupRows.Err() //nolint:errcheck // Check for iteration errors
			queueMap["groups"] = groups
		} else {
			queueMap["groups"] = []interface{}{}
		}

		queues = append(queues, queueMap)
	}
	_ = rows.Err() //nolint:errcheck // Check for iteration errors

	// Apply ticket attribute relations filtering if requested
	filterAttr := c.Query("filter_attribute")
	filterValue := c.Query("filter_value")
	if filterAttr != "" && filterValue != "" {
		// Convert to []gin.H for the filter function
		items := make([]gin.H, len(queues))
		for i, q := range queues {
			items[i] = gin.H(q)
		}
		items = filterByTicketAttributeRelations(c, db, items, "Queue", filterAttr, filterValue)
		// Convert back
		queues = make([]map[string]interface{}, len(items))
		for i, item := range items {
			queues[i] = map[string]interface{}(item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    queues,
	})
}
