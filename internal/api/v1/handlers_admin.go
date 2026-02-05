package v1

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

var startTime = time.Now()

// Admin handlers - system info with real database stats.
func (router *APIRouter) handleGetSystemInfo(c *gin.Context) {
	db, err := database.GetDB()
	info := gin.H{
		"version":   "1.0.0",
		"go":        runtime.Version(),
		"platform":  runtime.GOOS + "/" + runtime.GOARCH,
		"uptime_ms": time.Since(startTime).Milliseconds(),
	}

	if err == nil {
		// Get ticket count
		var ticketCount int
		db.QueryRow(database.ConvertQuery(`SELECT COUNT(*) FROM ticket WHERE archive_flag = 0`)).Scan(&ticketCount)
		info["tickets"] = ticketCount

		// Get user count
		var userCount int
		db.QueryRow(database.ConvertQuery(`SELECT COUNT(*) FROM users WHERE valid_id = 1`)).Scan(&userCount)
		info["users"] = userCount

		// Get queue count
		var queueCount int
		db.QueryRow(database.ConvertQuery(`SELECT COUNT(*) FROM queue WHERE valid_id = 1`)).Scan(&queueCount)
		info["queues"] = queueCount

		// Get database type
		if database.IsPostgreSQL() {
			info["database"] = "PostgreSQL"
		} else {
			info["database"] = "MariaDB"
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    info,
	})
}

func (router *APIRouter) handleGetSystemSettings(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{},
		})
		return
	}

	// Get settings from sysconfig_modified table (user overrides)
	query := database.ConvertQuery(`
		SELECT name, value 
		FROM sysconfig_modified 
		WHERE is_valid = 1
		ORDER BY name
	`)

	settings := gin.H{}
	rows, err := db.Query(query)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, value string
			if rows.Scan(&name, &value) == nil {
				settings[name] = value
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    settings,
	})
}

func (router *APIRouter) handleUpdateSystemSettings(c *gin.Context) {
	var req map[string]interface{}

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
	updated := 0

	for name, value := range req {
		valueStr := ""
		switch v := value.(type) {
		case string:
			valueStr = v
		case bool:
			if v {
				valueStr = "1"
			} else {
				valueStr = "0"
			}
		case float64:
			valueStr = strconv.FormatFloat(v, 'f', -1, 64)
		default:
			continue
		}

		// Upsert into sysconfig_modified
		query := database.ConvertQuery(`
			INSERT INTO sysconfig_modified (name, value, is_valid, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = ?, change_time = ?, change_by = ?
		`)
		_, err := db.Exec(query, name, valueStr, now, userID, now, userID, valueStr, now, userID)
		if err == nil {
			updated++
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Settings updated successfully",
		Data:    gin.H{"updated": updated},
	})
}

func (router *APIRouter) handleListBackups(c *gin.Context) {
	// TODO: Implement actual backup listing
	backups := []gin.H{
		{
			"id":         1,
			"name":       "backup_2024_08_01.tar.gz",
			"size":       1073741824, // 1GB
			"created_at": time.Now().AddDate(0, 0, -7),
			"type":       "full",
		},
		{
			"id":         2,
			"name":       "backup_2024_08_29.tar.gz",
			"size":       536870912, // 512MB
			"created_at": time.Now().AddDate(0, 0, -1),
			"type":       "incremental",
		},
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    backups,
	})
}

func (router *APIRouter) handleCreateBackup(c *gin.Context) {
	var req struct {
		Type        string `json:"type"` // full or incremental
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// TODO: Implement actual backup creation
	backup := gin.H{
		"id":          3,
		"name":        "backup_2024_08_30.tar.gz",
		"size":        0,
		"status":      "in_progress",
		"type":        req.Type,
		"description": req.Description,
		"created_at":  time.Now(),
	}

	c.JSON(http.StatusAccepted, APIResponse{
		Success: true,
		Message: "Backup initiated",
		Data:    backup,
	})
}

func (router *APIRouter) handleRestoreBackup(c *gin.Context) {
	backupID := c.Param("id")

	// TODO: Implement actual backup restoration
	c.JSON(http.StatusAccepted, APIResponse{
		Success: true,
		Message: "Backup restoration initiated for backup " + backupID,
	})
}

func (router *APIRouter) handleGetAuditLogs(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: []interface{}{}})
		return
	}

	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	// Query ticket_history for audit trail
	query := database.ConvertQuery(`
		SELECT th.id, th.ticket_id, th.name, th.create_time, 
			u.login as user_login, th.history_type_id
		FROM ticket_history th
		LEFT JOIN users u ON u.id = th.create_by
		ORDER BY th.create_time DESC
		LIMIT ?
	`)

	rows, err := db.Query(query, limit)
	logs := []gin.H{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, ticketID, historyTypeID int
			var name string
			var createTime time.Time
			var userLogin *string
			if rows.Scan(&id, &ticketID, &name, &createTime, &userLogin, &historyTypeID) == nil {
				log := gin.H{
					"id":         id,
					"ticket_id":  ticketID,
					"action":     name,
					"type_id":    historyTypeID,
					"timestamp":  createTime,
				}
				if userLogin != nil {
					log["user"] = *userLogin
				}
				logs = append(logs, log)
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    logs,
	})
}

func (router *APIRouter) handleGetSystemLogs(c *gin.Context) {
	// TODO: Implement actual system logs fetching
	logs := []gin.H{
		{
			"timestamp": time.Now().Add(-5 * time.Minute),
			"level":     "INFO",
			"message":   "System started",
		},
		{
			"timestamp": time.Now().Add(-1 * time.Minute),
			"level":     "WARNING",
			"message":   "High memory usage detected",
		},
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    logs,
	})
}

func (router *APIRouter) handleGetAuditLog(c *gin.Context) {
	// Alias for handleGetAuditLogs
	router.handleGetAuditLogs(c)
}

func (router *APIRouter) handleGetAuditStats(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{}})
		return
	}

	stats := gin.H{}

	// Total history events
	var total int
	db.QueryRow(database.ConvertQuery(`SELECT COUNT(*) FROM ticket_history`)).Scan(&total)
	stats["total_events"] = total

	// Events today
	var today int
	db.QueryRow(database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket_history 
		WHERE DATE(create_time) = CURRENT_DATE
	`)).Scan(&today)
	stats["events_today"] = today

	// Most active user
	var mostActiveUser string
	db.QueryRow(database.ConvertQuery(`
		SELECT u.login
		FROM ticket_history th
		JOIN users u ON u.id = th.create_by
		GROUP BY th.create_by, u.login
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`)).Scan(&mostActiveUser)
	stats["most_active_user"] = mostActiveUser

	// Most common history type
	var mostCommonAction string
	db.QueryRow(database.ConvertQuery(`
		SELECT ht.name
		FROM ticket_history th
		JOIN ticket_history_type ht ON ht.id = th.history_type_id
		GROUP BY th.history_type_id, ht.name
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`)).Scan(&mostCommonAction)
	stats["most_common_action"] = mostCommonAction

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

func (router *APIRouter) handleGetTicketReports(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{}})
		return
	}

	report := gin.H{}

	// Total tickets
	var total int
	db.QueryRow(database.ConvertQuery(`SELECT COUNT(*) FROM ticket WHERE archive_flag = 0`)).Scan(&total)
	report["total_tickets"] = total

	// By state
	stateQuery := database.ConvertQuery(`
		SELECT ts.name, COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		WHERE t.archive_flag = 0
		GROUP BY ts.name
	`)
	byState := gin.H{}
	if rows, err := db.Query(stateQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
				byState[name] = cnt
			}
		}
	}
	report["by_state"] = byState

	// By priority
	priorityQuery := database.ConvertQuery(`
		SELECT tp.name, COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_priority tp ON tp.id = t.ticket_priority_id
		WHERE t.archive_flag = 0
		GROUP BY tp.name
	`)
	byPriority := gin.H{}
	if rows, err := db.Query(priorityQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
				byPriority[name] = cnt
			}
		}
	}
	report["by_priority"] = byPriority

	// By queue
	queueQuery := database.ConvertQuery(`
		SELECT q.name, COUNT(*) as cnt
		FROM ticket t
		JOIN queue q ON q.id = t.queue_id
		WHERE t.archive_flag = 0
		GROUP BY q.name
	`)
	byQueue := gin.H{}
	if rows, err := db.Query(queueQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
				byQueue[name] = cnt
			}
		}
	}
	report["by_queue"] = byQueue

	// Created this month
	var thisMonth int
	monthQuery := database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket 
		WHERE archive_flag = 0 
		AND create_time >= DATE_FORMAT(CURRENT_DATE, '%Y-%m-01')
	`)
	db.QueryRow(monthQuery).Scan(&thisMonth)
	report["created_this_month"] = thisMonth

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    report,
	})
}

func (router *APIRouter) handleGetSystemConfig(c *gin.Context) {
	// Alias for handleGetSystemSettings
	router.handleGetSystemSettings(c)
}

func (router *APIRouter) handleUpdateSystemConfig(c *gin.Context) {
	// Alias for handleUpdateSystemSettings
	router.handleUpdateSystemSettings(c)
}

func (router *APIRouter) handleGetSystemStats(c *gin.Context) {
	// Alias for handleGetSystemInfo
	router.handleGetSystemInfo(c)
}

func (router *APIRouter) handleToggleMaintenanceMode(c *gin.Context) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Message string `json:"message"`
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
	enabledStr := "0"
	if req.Enabled {
		enabledStr = "1"
	}

	// Store maintenance mode in sysconfig_modified
	query := database.ConvertQuery(`
		INSERT INTO sysconfig_modified (name, value, is_valid, create_time, create_by, change_time, change_by)
		VALUES ('MaintenanceMode', ?, 1, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE value = ?, change_time = ?, change_by = ?
	`)
	db.Exec(query, enabledStr, now, userID, now, userID, enabledStr, now, userID)

	// Store maintenance message
	msgQuery := database.ConvertQuery(`
		INSERT INTO sysconfig_modified (name, value, is_valid, create_time, create_by, change_time, change_by)
		VALUES ('MaintenanceMessage', ?, 1, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE value = ?, change_time = ?, change_by = ?
	`)
	db.Exec(msgQuery, req.Message, now, userID, now, userID, req.Message, now, userID)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Maintenance mode updated",
		Data: gin.H{
			"enabled": req.Enabled,
			"message": req.Message,
		},
	})
}

func (router *APIRouter) handleGetUserReports(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{}})
		return
	}

	report := gin.H{}

	// Total active users
	var totalUsers int
	db.QueryRow(database.ConvertQuery(`SELECT COUNT(*) FROM users WHERE valid_id = 1`)).Scan(&totalUsers)
	report["total_active_users"] = totalUsers

	// Users by role
	roleQuery := database.ConvertQuery(`
		SELECT r.name, COUNT(DISTINCT ugr.user_id) as cnt
		FROM role r
		LEFT JOIN group_role gr ON gr.role_id = r.id
		LEFT JOIN user_group ugr ON ugr.group_id = gr.group_id
		WHERE r.valid_id = 1
		GROUP BY r.name
	`)
	byRole := gin.H{}
	if rows, err := db.Query(roleQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
				byRole[name] = cnt
			}
		}
	}
	report["by_role"] = byRole

	// Top agents by ticket count
	agentQuery := database.ConvertQuery(`
		SELECT u.login, COUNT(t.id) as ticket_count
		FROM users u
		LEFT JOIN ticket t ON t.user_id = u.id AND t.archive_flag = 0
		WHERE u.valid_id = 1
		GROUP BY u.id, u.login
		ORDER BY ticket_count DESC
		LIMIT 10
	`)
	topAgents := []gin.H{}
	if rows, err := db.Query(agentQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var login string
			var cnt int
			if rows.Scan(&login, &cnt) == nil {
				topAgents = append(topAgents, gin.H{"login": login, "ticket_count": cnt})
			}
		}
	}
	report["top_agents"] = topAgents

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    report,
	})
}

func (router *APIRouter) handleGetSLAReports(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{}})
		return
	}

	report := gin.H{}

	// Get SLAs with ticket counts
	slaQuery := database.ConvertQuery(`
		SELECT s.name, COUNT(DISTINCT t.id) as ticket_count
		FROM sla s
		LEFT JOIN ticket t ON t.sla_id = s.id AND t.archive_flag = 0
		WHERE s.valid_id = 1
		GROUP BY s.id, s.name
	`)
	bySLA := []gin.H{}
	if rows, err := db.Query(slaQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
				bySLA = append(bySLA, gin.H{"sla": name, "ticket_count": cnt})
			}
		}
	}
	report["by_sla"] = bySLA

	// Tickets with escalation
	var escalated int
	db.QueryRow(database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket 
		WHERE archive_flag = 0 
		AND escalation_time > 0
	`)).Scan(&escalated)
	report["escalated_tickets"] = escalated

	// Active SLA count
	var activeSLAs int
	db.QueryRow(database.ConvertQuery(`SELECT COUNT(*) FROM sla WHERE valid_id = 1`)).Scan(&activeSLAs)
	report["active_slas"] = activeSLAs

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    report,
	})
}

func (router *APIRouter) handleGetPerformanceReports(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{}})
		return
	}

	report := gin.H{}

	// Tickets created per day (last 7 days)
	dailyQuery := database.ConvertQuery(`
		SELECT DATE(create_time) as day, COUNT(*) as cnt
		FROM ticket
		WHERE create_time >= DATE_SUB(CURRENT_DATE, INTERVAL 7 DAY)
		GROUP BY DATE(create_time)
		ORDER BY day
	`)
	daily := []gin.H{}
	if rows, err := db.Query(dailyQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var day time.Time
			var cnt int
			if rows.Scan(&day, &cnt) == nil {
				daily = append(daily, gin.H{"date": day.Format("2006-01-02"), "count": cnt})
			}
		}
	}
	report["tickets_per_day"] = daily

	// Average articles per ticket
	var avgArticles float64
	db.QueryRow(database.ConvertQuery(`
		SELECT AVG(article_count) FROM (
			SELECT COUNT(*) as article_count
			FROM article
			GROUP BY ticket_id
		) sub
	`)).Scan(&avgArticles)
	report["avg_articles_per_ticket"] = avgArticles

	// Active sessions count
	var sessions int
	db.QueryRow(database.ConvertQuery(`SELECT COUNT(DISTINCT session_id) FROM sessions`)).Scan(&sessions)
	report["active_sessions"] = sessions

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    report,
	})
}

func (router *APIRouter) handleExportReport(c *gin.Context) {
	reportID := c.Param("id")

	// TODO: Implement actual report export
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=report_"+reportID+".pdf")
	c.String(http.StatusOK, "Report content here")
}
