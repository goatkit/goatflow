package v1

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// Dashboard handlers

func (router *APIRouter) handleGetDashboardStats(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	var totalTickets, openTickets, closedToday int

	// Total tickets
	row1 := db.QueryRow("SELECT COUNT(*) FROM ticket")
	_ = row1.Scan(&totalTickets) //nolint:errcheck // Defaults to 0

	// Open tickets (state types: new=1, open=2, pending reminder=4, pending auto=5)
	row2 := db.QueryRow(`
		SELECT COUNT(*) FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE ts.type_id IN (1, 2, 4, 5)
	`)
	_ = row2.Scan(&openTickets) //nolint:errcheck // Defaults to 0

	// Closed today (state type 3 = closed)
	row3 := db.QueryRow(database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE ts.type_id = 3 AND DATE(t.change_time) = CURRENT_DATE
	`))
	_ = row3.Scan(&closedToday) //nolint:errcheck // Defaults to 0

	stats := gin.H{
		"total_tickets":         totalTickets,
		"open_tickets":          openTickets,
		"closed_today":          closedToday,
		"avg_response_time":     "N/A",
		"customer_satisfaction": 0,
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// fetchChartData executes a query expecting (name, count) rows and returns labels/data for charts.
func fetchChartData(db *sql.DB, query string) ([]string, []int, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var labels []string
	var data []int
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			continue
		}
		labels = append(labels, name)
		data = append(data, count)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return labels, data, nil
}

func (router *APIRouter) handleGetTicketsByStatusChart(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Database unavailable"})
		return
	}

	query := `
		SELECT tst.name, COUNT(t.id) as count
		FROM ticket_state_type tst
		LEFT JOIN ticket_state ts ON ts.type_id = tst.id
		LEFT JOIN ticket t ON t.ticket_state_id = ts.id
		GROUP BY tst.id, tst.name
		ORDER BY tst.id
	`
	labels, data, err := fetchChartData(db, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Failed to query ticket states"})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{"labels": labels, "data": data}})
}

func (router *APIRouter) handleGetTicketsByPriorityChart(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Database unavailable"})
		return
	}

	query := `
		SELECT tp.name, COUNT(t.id) as count
		FROM ticket_priority tp
		LEFT JOIN ticket t ON t.ticket_priority_id = tp.id
		WHERE tp.valid_id = 1
		GROUP BY tp.id, tp.name
		ORDER BY tp.id
	`
	labels, data, err := fetchChartData(db, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Failed to query ticket priorities"})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{"labels": labels, "data": data}})
}

func (router *APIRouter) handleGetTicketsOverTimeChart(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	// Get ticket counts for last 7 days
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT 
			DATE(create_time) as date,
			COUNT(*) as created
		FROM ticket
		WHERE create_time >= DATE_SUB(CURRENT_DATE, INTERVAL 6 DAY)
		GROUP BY DATE(create_time)
		ORDER BY date
	`))
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to query tickets over time",
		})
		return
	}
	defer rows.Close()

	dateCreated := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			continue
		}
		dateCreated[date] = count
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Error iterating tickets over time",
		})
		return
	}

	// Get closed tickets for last 7 days (state type 3 = closed)
	closedRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT 
			DATE(t.change_time) as date,
			COUNT(*) as closed
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE ts.type_id = 3 AND t.change_time >= DATE_SUB(CURRENT_DATE, INTERVAL 6 DAY)
		GROUP BY DATE(t.change_time)
		ORDER BY date
	`))
	if err == nil {
		defer closedRows.Close()
	}

	dateClosed := make(map[string]int)
	if closedRows != nil {
		for closedRows.Next() {
			var date string
			var count int
			if err := closedRows.Scan(&date, &count); err != nil {
				continue
			}
			dateClosed[date] = count
		}
		if err := closedRows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Error iterating closed tickets",
			})
			return
		}
	}

	// Build labels and datasets for last 7 days
	var labels []string
	var createdData []int
	var closedData []int
	now := time.Now()
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		labels = append(labels, date.Format("Mon"))
		createdData = append(createdData, dateCreated[dateStr])
		closedData = append(closedData, dateClosed[dateStr])
	}

	chartData := gin.H{
		"labels": labels,
		"datasets": []gin.H{
			{
				"label": "Created",
				"data":  createdData,
			},
			{
				"label": "Closed",
				"data":  closedData,
			},
		},
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    chartData,
	})
}

func (router *APIRouter) handleGetRecentActivity(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	// Get recent ticket history entries
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT th.id, th.ticket_id, t.tn, th.history_type_id, th.name, th.create_time
		FROM ticket_history th
		JOIN ticket t ON th.ticket_id = t.id
		ORDER BY th.create_time DESC
		LIMIT 10
	`))
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to query recent activity",
		})
		return
	}
	defer rows.Close()

	var activities []gin.H
	for rows.Next() {
		var id, ticketID, historyTypeID int
		var ticketNumber, name string
		var createTime sql.NullTime
		if err := rows.Scan(&id, &ticketID, &ticketNumber, &historyTypeID, &name, &createTime); err != nil {
			continue
		}

		activityType := "ticket_updated"
		if historyTypeID == 1 {
			activityType = "ticket_created"
		}

		activity := gin.H{
			"id":            id,
			"type":          activityType,
			"message":       name,
			"ticket_id":     ticketID,
			"ticket_number": ticketNumber,
		}
		if createTime.Valid {
			activity["timestamp"] = createTime.Time
		}

		activities = append(activities, activity)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Error iterating recent activity",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    activities,
	})
}

func (router *APIRouter) handleGetMyTickets(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Success: false,
			Error:   "User not authenticated",
		})
		return
	}

	// Get tickets assigned to the current user
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT t.id, t.tn, t.title, ts.name as status, tp.name as priority
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
		WHERE t.responsible_user_id = $1 OR t.user_id = $1
		ORDER BY t.create_time DESC
		LIMIT 20
	`), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to query user tickets",
		})
		return
	}
	defer rows.Close()

	var tickets []gin.H
	for rows.Next() {
		var id int
		var number, title, status, priority string
		if err := rows.Scan(&id, &number, &title, &status, &priority); err != nil {
			continue
		}
		tickets = append(tickets, gin.H{
			"id":       id,
			"number":   number,
			"title":    title,
			"status":   status,
			"priority": priority,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Error iterating user tickets",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    tickets,
	})
}
