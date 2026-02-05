package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// handleListQueues returns all queues the user has access to.
func (router *APIRouter) handleListQueues(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"queues": []interface{}{}, "total": 0},
		})
		return
	}

	// Get valid_id filter (default to only valid queues)
	showInvalid := c.Query("include_invalid") == "true"

	query := database.ConvertQuery(`
		SELECT q.id, q.name, q.group_id, g.name as group_name,
			q.calendar_name, q.first_response_time, q.update_time, q.solution_time,
			COALESCE(q.comment, '') as comment, q.valid_id,
			q.create_time, q.change_time
		FROM queue q
		LEFT JOIN ` + "`groups`" + ` g ON g.id = q.group_id
		WHERE 1=1
	`)
	args := []interface{}{}

	if !showInvalid {
		query += database.ConvertQuery(` AND q.valid_id = 1`)
	}

	query += ` ORDER BY q.name`

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"queues": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	type Queue struct {
		ID                int        `json:"id"`
		Name              string     `json:"name"`
		GroupID           int        `json:"group_id"`
		GroupName         *string    `json:"group_name"`
		CalendarName      *string    `json:"calendar_name"`
		FirstResponseTime *int       `json:"first_response_time"`
		UpdateTime        *int       `json:"update_time"`
		SolutionTime      *int       `json:"solution_time"`
		Comment           string     `json:"comment"`
		ValidID           int        `json:"valid_id"`
		CreatedAt         time.Time  `json:"created_at"`
		UpdatedAt         time.Time  `json:"updated_at"`
	}

	queues := []Queue{}
	for rows.Next() {
		var q Queue
		if err := rows.Scan(&q.ID, &q.Name, &q.GroupID, &q.GroupName,
			&q.CalendarName, &q.FirstResponseTime, &q.UpdateTime, &q.SolutionTime,
			&q.Comment, &q.ValidID, &q.CreatedAt, &q.UpdatedAt); err != nil {
			continue
		}
		queues = append(queues, q)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"queues": queues, "total": len(queues)},
	})
}

// handleCreateQueue creates a new queue.
func (router *APIRouter) handleCreateQueue(c *gin.Context) {
	var req struct {
		Name              string `json:"name" binding:"required"`
		GroupID           int    `json:"group_id" binding:"required"`
		SystemAddressID   *int   `json:"system_address_id"`
		CalendarName      string `json:"calendar_name"`
		FirstResponseTime *int   `json:"first_response_time"`
		UpdateTime        *int   `json:"update_time"`
		SolutionTime      *int   `json:"solution_time"`
		Comment           string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		INSERT INTO queue
			(name, group_id, system_address_id, calendar_name,
			 first_response_time, update_time, solution_time, comment,
			 valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
	`)

	result, err := db.Exec(query, req.Name, req.GroupID, req.SystemAddressID, req.CalendarName,
		req.FirstResponseTime, req.UpdateTime, req.SolutionTime, req.Comment,
		now, userID, now, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create queue")
		return
	}

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         id,
			"name":       req.Name,
			"group_id":   req.GroupID,
			"created_at": now,
		},
	})
}

// handleGetQueue returns a specific queue.
func (router *APIRouter) handleGetQueue(c *gin.Context) {
	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT q.id, q.name, q.group_id, g.name as group_name,
			q.system_address_id, q.calendar_name,
			q.first_response_time, q.update_time, q.solution_time,
			COALESCE(q.comment, '') as comment, q.valid_id,
			q.create_time, q.change_time
		FROM queue q
		LEFT JOIN ` + "`groups`" + ` g ON g.id = q.group_id
		WHERE q.id = ?
	`)

	var queue struct {
		ID                int       `json:"id"`
		Name              string    `json:"name"`
		GroupID           int       `json:"group_id"`
		GroupName         *string   `json:"group_name"`
		SystemAddressID   *int      `json:"system_address_id"`
		CalendarName      *string   `json:"calendar_name"`
		FirstResponseTime *int      `json:"first_response_time"`
		UpdateTime        *int      `json:"update_time"`
		SolutionTime      *int      `json:"solution_time"`
		Comment           string    `json:"comment"`
		ValidID           int       `json:"valid_id"`
		CreatedAt         time.Time `json:"created_at"`
		UpdatedAt         time.Time `json:"updated_at"`
	}

	err = db.QueryRow(query, queueID).Scan(
		&queue.ID, &queue.Name, &queue.GroupID, &queue.GroupName,
		&queue.SystemAddressID, &queue.CalendarName,
		&queue.FirstResponseTime, &queue.UpdateTime, &queue.SolutionTime,
		&queue.Comment, &queue.ValidID, &queue.CreatedAt, &queue.UpdatedAt,
	)
	if err != nil {
		sendError(c, http.StatusNotFound, "Queue not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    queue,
	})
}

// handleUpdateQueue updates a queue.
func (router *APIRouter) handleUpdateQueue(c *gin.Context) {
	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	var req struct {
		Name              string `json:"name"`
		GroupID           *int   `json:"group_id"`
		CalendarName      string `json:"calendar_name"`
		FirstResponseTime *int   `json:"first_response_time"`
		UpdateTime        *int   `json:"update_time"`
		SolutionTime      *int   `json:"solution_time"`
		Comment           string `json:"comment"`
		ValidID           *int   `json:"valid_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE queue SET
			name = COALESCE(NULLIF(?, ''), name),
			group_id = COALESCE(?, group_id),
			calendar_name = COALESCE(NULLIF(?, ''), calendar_name),
			first_response_time = COALESCE(?, first_response_time),
			update_time = COALESCE(?, update_time),
			solution_time = COALESCE(?, solution_time),
			comment = COALESCE(NULLIF(?, ''), comment),
			valid_id = COALESCE(?, valid_id),
			change_time = ?,
			change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, req.Name, req.GroupID, req.CalendarName,
		req.FirstResponseTime, req.UpdateTime, req.SolutionTime,
		req.Comment, req.ValidID, now, userID, queueID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update queue")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Queue not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         queueID,
			"updated_at": now,
		},
	})
}

// handleDeleteQueue soft-deletes a queue.
func (router *APIRouter) handleDeleteQueue(c *gin.Context) {
	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	// Soft delete by setting valid_id = 2
	query := database.ConvertQuery(`
		UPDATE queue SET valid_id = 2, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, time.Now(), userID, queueID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete queue")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Queue not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"message": "Queue deleted"},
	})
}

// handleGetQueueTickets returns tickets in a specific queue.
func (router *APIRouter) handleGetQueueTickets(c *gin.Context) {
	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"tickets": []interface{}{}, "total": 0},
		})
		return
	}

	limit := 25
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Count total
	countQuery := database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket WHERE queue_id = ? AND archive_flag = 0
	`)
	var total int
	db.QueryRow(countQuery, queueID).Scan(&total)

	// Get tickets
	query := database.ConvertQuery(`
		SELECT t.id, t.tn, t.title, t.create_time, t.change_time,
			ts.name as state, tp.name as priority,
			CONCAT(u.first_name, ' ', u.last_name) as owner
		FROM ticket t
		LEFT JOIN ticket_state ts ON ts.id = t.ticket_state_id
		LEFT JOIN ticket_priority tp ON tp.id = t.ticket_priority_id
		LEFT JOIN users u ON u.id = t.user_id
		WHERE t.queue_id = ? AND t.archive_flag = 0
		ORDER BY t.change_time DESC
		LIMIT ? OFFSET ?
	`)

	rows, err := db.Query(query, queueID, limit, offset)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"tickets": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	tickets := []gin.H{}
	for rows.Next() {
		var id int
		var tn, title string
		var createTime, changeTime time.Time
		var state, priority, owner *string

		if err := rows.Scan(&id, &tn, &title, &createTime, &changeTime, &state, &priority, &owner); err != nil {
			continue
		}

		tickets = append(tickets, gin.H{
			"id":         id,
			"number":     tn,
			"title":      title,
			"state":      state,
			"priority":   priority,
			"owner":      owner,
			"created_at": createTime,
			"updated_at": changeTime,
		})
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"tickets": tickets,
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		},
	})
}

// handleGetQueueStats returns statistics for a queue.
func (router *APIRouter) handleGetQueueStats(c *gin.Context) {
	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"queue_id": queueID,
				"total":    0,
				"by_state": gin.H{},
			},
		})
		return
	}

	// Get counts by state
	stateQuery := database.ConvertQuery(`
		SELECT ts.name, COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		WHERE t.queue_id = ? AND t.archive_flag = 0
		GROUP BY ts.name
	`)

	rows, err := db.Query(stateQuery, queueID)
	byState := gin.H{}
	total := 0
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var stateName string
			var count int
			if rows.Scan(&stateName, &count) == nil {
				byState[stateName] = count
				total += count
			}
		}
	}

	// Get counts by priority
	priorityQuery := database.ConvertQuery(`
		SELECT tp.name, COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_priority tp ON tp.id = t.ticket_priority_id
		WHERE t.queue_id = ? AND t.archive_flag = 0
		GROUP BY tp.name
	`)

	rows2, err := db.Query(priorityQuery, queueID)
	byPriority := gin.H{}
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var priorityName string
			var count int
			if rows2.Scan(&priorityName, &count) == nil {
				byPriority[priorityName] = count
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"queue_id":    queueID,
			"total":       total,
			"by_state":    byState,
			"by_priority": byPriority,
		},
	})
}
