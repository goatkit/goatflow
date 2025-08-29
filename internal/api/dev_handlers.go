package api

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// handleDevDashboard shows the main developer dashboard
func handleDevDashboard(c *gin.Context) {
	// Ensure dashboard manager is initialized
	if config.DefaultDashboardManager == nil {
		pongo2Renderer.HTML(c, http.StatusInternalServerError, "error.pongo2", pongo2.Context{
			"error": "Dashboard manager not initialized",
		})
		return
	}
	
	// Load dashboard configuration from YAML
	dashboardConfig, err := config.DefaultDashboardManager.LoadDashboard("dev-dashboard")
	if err != nil {
		pongo2Renderer.HTML(c, http.StatusInternalServerError, "error.pongo2", pongo2.Context{
			"error": fmt.Sprintf("Failed to load dashboard config: %v", err),
		})
		return
	}

	db, _ := database.GetDB()

	// Calculate stats from YAML configuration
	stats := make(map[string]interface{})
	for _, stat := range dashboardConfig.Spec.Dashboard.Stats {
		var value interface{} = "N/A"
		
		// Execute SQL query if defined
		if stat.Query != "" && db != nil {
			var result int
			if err := db.QueryRow(stat.Query).Scan(&result); err == nil {
				if stat.Suffix != "" {
					value = fmt.Sprintf("%d%s", result, stat.Suffix)
				} else {
					value = result
				}
			}
		}
		
		// Execute command if defined
		if stat.Command != "" {
			cmd := exec.Command("sh", "-c", stat.Command)
			if output, err := cmd.Output(); err == nil {
				switch stat.Parser {
				case "build_status":
					value = "OK"
				case "test_status":
					if strings.Contains(string(output), "PASS") {
						value = "PASS"
					} else {
						value = "FAIL"
					}
				default:
					value = strings.TrimSpace(string(output))
				}
			} else {
				switch stat.Parser {
				case "build_status":
					value = "Failed"
				case "test_status":
					value = "FAIL"
				}
			}
		}
		
		stats[strings.ToLower(strings.ReplaceAll(stat.Name, " ", "_"))] = value
	}

	// Prepare tiles with color schemes and icons
	tilesData := make([]map[string]interface{}, len(dashboardConfig.Spec.Dashboard.Tiles))
	for i, tile := range dashboardConfig.Spec.Dashboard.Tiles {
		colorScheme := config.DefaultDashboardManager.GetColorScheme(dashboardConfig, tile.Color)
		iconPath := config.DefaultDashboardManager.GetIconPath(dashboardConfig, tile.Icon)
		
		tilesData[i] = map[string]interface{}{
			"name":        tile.Name,
			"description": tile.Description,
			"url":         tile.URL,
			"icon":        iconPath,
			"color":       colorScheme,
			"category":    tile.Category,
			"featured":    tile.Featured,
		}
	}

	// Prepare quick actions
	actionsData := make([]map[string]interface{}, len(dashboardConfig.Spec.Dashboard.QuickActions))
	for i, action := range dashboardConfig.Spec.Dashboard.QuickActions {
		colorScheme := config.DefaultDashboardManager.GetColorScheme(dashboardConfig, action.Color)
		iconPath := config.DefaultDashboardManager.GetIconPath(dashboardConfig, action.Icon)
		
		actionsData[i] = map[string]interface{}{
			"name":     action.Name,
			"action":   action.Action,
			"url":      action.URL,
			"endpoint": action.Endpoint,
			"icon":     iconPath,
			"color":    colorScheme,
			"confirm":  action.Confirm,
		}
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/dev/dashboard_dynamic.pongo2", pongo2.Context{
		"User":       getUserMapForTemplate(c),
		"ActivePage": "dev",
		"config":     dashboardConfig,
		"stats":      stats,
		"tiles":      tilesData,
		"actions":    actionsData,
	})
}

// handleClaudeTickets shows the Claude ticket monitoring interface
func handleClaudeTickets(c *gin.Context) {
	db, _ := database.GetDB()

	// Get all tickets from Claude Code queue
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT 
			t.id,
			t.tn,
			t.title,
			t.create_time,
			ts.name as state,
			COALESCE(cu.login, t.customer_user_id) as customer,
			COALESCE(u.login, 'unassigned') as assigned_to
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		LEFT JOIN customer_user cu ON t.customer_user_id = cu.login
		LEFT JOIN users u ON t.responsible_user_id = u.id
		WHERE t.queue_id = 14
		ORDER BY t.create_time DESC
		LIMIT 50
	`))
	if err != nil {
		pongo2Renderer.HTML(c, http.StatusInternalServerError, "error.pongo2", pongo2.Context{
			"error": "Failed to fetch tickets",
		})
		return
	}
	defer rows.Close()

	type Article struct {
		From    string
		Subject string
		Body    string
		Time    string
	}

	type Ticket struct {
		ID         int
		Number     string
		Title      string
		CreateTime string
		State      string
		Customer   string
		AssignedTo string
		Articles   []Article
	}


	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		err := rows.Scan(&t.ID, &t.Number, &t.Title, &t.CreateTime, &t.State, &t.Customer, &t.AssignedTo)
		if err != nil {
			continue
		}

		// Get articles for this ticket
		articleRows, err := db.Query(database.ConvertPlaceholders(`
			SELECT 
				adm.a_from,
				adm.a_subject,
				ENCODE(adm.a_body, 'escape') as body,
				a.create_time
			FROM article a
			JOIN article_data_mime adm ON a.id = adm.article_id
			WHERE a.ticket_id = $1
			ORDER BY a.create_time DESC
			LIMIT 5
		`), t.ID)
		if err == nil {
			defer articleRows.Close()
			for articleRows.Next() {
				var art Article
				articleRows.Scan(&art.From, &art.Subject, &art.Body, &art.Time)
				t.Articles = append(t.Articles, art)
			}
		}

		tickets = append(tickets, t)
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/dev/claude_tickets.pongo2", pongo2.Context{
		"User":       getUserFromContext(c),
		"ActivePage": "dev",
		"tickets":    tickets,
	})
}

// handleDevAction handles quick actions from the dev dashboard
func handleDevAction(c *gin.Context) {
	action := c.Param("action")

	switch action {
	case "restart-backend":
		cmd := exec.Command("./scripts/container-wrapper.sh", "restart", "gotrs-backend")
		output, err := cmd.CombinedOutput()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Failed to restart: %v", err),
				"output":  string(output),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Backend service restarted successfully",
		})

	case "clear-cache":
		// Clear various caches
		// For now, just clear session storage
		db, _ := database.GetDB()
		_, err := db.Exec("DELETE FROM sessions WHERE create_time < NOW() - INTERVAL '1 hour'")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to clear cache",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Cache cleared successfully",
		})

	case "check-health":
		// Check all services
		services := []string{"gotrs-backend", "gotrs-postgres", "nginx", "valkey", "mailhog"}
		healthy := true
		var unhealthy []string

		for _, service := range services {
			cmd := exec.Command("./scripts/container-wrapper.sh", "exec", service, "echo", "OK")
			if err := cmd.Run(); err != nil {
				healthy = false
				unhealthy = append(unhealthy, service)
			}
		}

		if healthy {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "All services healthy",
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("Unhealthy services: %s", strings.Join(unhealthy, ", ")),
			})
		}

	case "add-ticket-response":
		// Add a response/article to a ticket
		var req struct {
			Ticket   string `json:"ticket"`
			Response string `json:"response"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request format",
			})
			return
		}

		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Database connection failed",
			})
			return
		}

		// Get ticket ID from ticket number
		var ticketID int
		err = db.QueryRow("SELECT id FROM ticket WHERE tn = $1", req.Ticket).Scan(&ticketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Ticket not found",
			})
			return
		}

		// Insert article
		var articleID int
		err = db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, 
				is_visible_for_customer, create_by, change_by, create_time, change_time)
			VALUES ($1, 1, 1, 1, 1, 1, NOW(), NOW())
			RETURNING id
		`), ticketID).Scan(&articleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Failed to create article: %v", err),
			})
			return
		}

		// Insert article content
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body, 
				a_content_type, incoming_time, create_by, change_by, create_time, change_time)
			VALUES ($1, 'claude@gotrs.local', 'customer@gotrs.local', 
				'Response added via Claude Ticket Monitor', $2, 'text/plain',
				EXTRACT(EPOCH FROM NOW())::integer, 1, 1, NOW(), NOW())
		`), articleID, req.Response)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Failed to create article content: %v", err),
			})
			return
		}

		// Check if ticket is closed and reopen it
		var currentStateID int
		var currentStateName string
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT ts.id, ts.name 
			FROM ticket t 
			JOIN ticket_state ts ON t.ticket_state_id = ts.id 
			WHERE t.id = $1
		`), ticketID).Scan(&currentStateID, &currentStateName)
		if err == nil {
			// If ticket is in a closed state, reopen it
			if strings.Contains(strings.ToLower(currentStateName), "closed") || 
			   strings.Contains(strings.ToLower(currentStateName), "removed") ||
			   strings.Contains(strings.ToLower(currentStateName), "merged") {
				// Get the ID for 'open' state
				var openStateID int
				err = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'open' LIMIT 1").Scan(&openStateID)
				if err == nil {
					// Update ticket to open state
					_, err = db.Exec(database.ConvertPlaceholders(`
						UPDATE ticket 
						SET ticket_state_id = $1, change_time = NOW(), change_by = 1 
						WHERE id = $2
					`), openStateID, ticketID)
					if err == nil {
						c.JSON(http.StatusOK, gin.H{
							"success": true,
							"message": "Response added and ticket reopened successfully",
						})
						return
					}
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Response added successfully",
		})

	case "update-ticket-status":
		// Update ticket status
		var req struct {
			Ticket string `json:"ticket"`
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request format",
			})
			return
		}

		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Database connection failed",
			})
			return
		}

		// Get state ID from state name
		var stateID int
		err = db.QueryRow("SELECT id FROM ticket_state WHERE name = $1", req.Status).Scan(&stateID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid status: %s", req.Status),
			})
			return
		}

		// Update ticket status
		result, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket SET ticket_state_id = $1, change_time = NOW(), change_by = 1
			WHERE tn = $2
		`), stateID, req.Ticket)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("Failed to update ticket: %v", err),
			})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Ticket not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Ticket status updated successfully",
		})

	case "get-ticket-states":
		// Get available ticket states for dropdown
		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Database connection failed",
			})
			return
		}

		rows, err := db.Query("SELECT id, name FROM ticket_state ORDER BY name")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to fetch states",
			})
			return
		}
		defer rows.Close()

		var states []map[string]interface{}
		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				continue
			}
			states = append(states, map[string]interface{}{
				"id":   id,
				"name": name,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"states":  states,
		})

	default:
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Unknown action",
		})
	}
}

// handleDevLogs shows the log viewer
func handleDevLogs(c *gin.Context) {
	service := c.DefaultQuery("service", "gotrs-backend")
	lines := c.DefaultQuery("lines", "100")

	// Get logs from the service
	cmd := exec.Command("./scripts/container-wrapper.sh", "logs", "--tail", lines, service)
	output, err := cmd.Output()
	if err != nil {
		pongo2Renderer.HTML(c, http.StatusInternalServerError, "error.pongo2", pongo2.Context{
			"error": fmt.Sprintf("Failed to get logs: %v", err),
		})
		return
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/dev/logs.pongo2", pongo2.Context{
		"User":       getUserFromContext(c),
		"ActivePage": "dev",
		"service":    service,
		"logs":       string(output),
		"lines":      lines,
	})
}

// handleDevDatabase shows database explorer
func handleDevDatabase(c *gin.Context) {
	db, _ := database.GetDB()

	// Get list of tables
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT tablename 
		FROM pg_tables 
		WHERE schemaname = 'public' 
		ORDER BY tablename
	`))
	if err != nil {
		pongo2Renderer.HTML(c, http.StatusInternalServerError, "error.pongo2", pongo2.Context{
			"error": "Failed to get tables",
		})
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		rows.Scan(&table)
		tables = append(tables, table)
	}

	// If a query was submitted, run it
	var queryResult interface{}
	query := c.PostForm("query")
	if query != "" {
		// Only allow SELECT queries for safety
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
			rows, err := db.Query(query)
			if err != nil {
				queryResult = fmt.Sprintf("Error: %v", err)
			} else {
				defer rows.Close()
				
				// Get column names
				columns, err := rows.Columns()
				if err != nil {
					queryResult = fmt.Sprintf("Error getting columns: %v", err)
				} else {
					// Prepare result structure
					var results []map[string]interface{}
					
					// Create a slice of interface{} to hold each column value
					values := make([]interface{}, len(columns))
					valuePtrs := make([]interface{}, len(columns))
					for i := range values {
						valuePtrs[i] = &values[i]
					}
					
					// Fetch all rows
					for rows.Next() {
						err := rows.Scan(valuePtrs...)
						if err != nil {
							queryResult = fmt.Sprintf("Error scanning row: %v", err)
							break
						}
						
						// Create a map for this row
						row := make(map[string]interface{})
						for i, col := range columns {
							val := values[i]
							// Handle different types
							switch v := val.(type) {
							case []byte:
								row[col] = string(v)
							case nil:
								row[col] = "NULL"
							default:
								row[col] = v
							}
						}
						results = append(results, row)
					}
					
					// Format results as a table-like string
					if len(results) == 0 {
						queryResult = "No rows returned"
					} else {
						// Build formatted output
						var output strings.Builder
						
						// Header
						for i, col := range columns {
							if i > 0 {
								output.WriteString(" | ")
							}
							output.WriteString(col)
						}
						output.WriteString("\n")
						output.WriteString(strings.Repeat("-", len(output.String())))
						output.WriteString("\n")
						
						// Rows
						for _, row := range results {
							for i, col := range columns {
								if i > 0 {
									output.WriteString(" | ")
								}
								output.WriteString(fmt.Sprintf("%v", row[col]))
							}
							output.WriteString("\n")
						}
						
						queryResult = output.String()
					}
				}
			}
		} else {
			queryResult = "Only SELECT queries are allowed"
		}
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/dev/database.pongo2", pongo2.Context{
		"User":        getUserFromContext(c),
		"ActivePage":  "dev",
		"tables":      tables,
		"query":       query,
		"queryResult": queryResult,
	})
}

// RegisterDevRoutes registers all developer routes
func RegisterDevRoutes(r *gin.RouterGroup) {
	// Note: Routes are now handled via YAML configuration files
	// See routes/dev/*.yaml for route definitions
	
	// Commented out - now handled by YAML routes
	// // Main dashboard
	// r.GET("", handleDevDashboard)
	// r.GET("/", handleDevDashboard)
	// 
	// // Tools
	// r.GET("/claude-tickets", handleClaudeTickets)
	// r.GET("/database", handleDevDatabase)
	// r.POST("/database", handleDevDatabase)
	// r.GET("/logs", handleDevLogs)
	// 
	// // Server-Sent Events for real-time updates
	// r.GET("/tickets/events", handleTicketEvents)
	// 
	// // Actions
	// r.POST("/action/:action", handleDevAction)
	// 
	// // TODO: Add more dev tools as needed
	// // dev.GET("/api-tester", handleAPITester)
	// // dev.GET("/templates", handleTemplatePlayground)
	// // dev.GET("/services", handleServiceMonitor)
	// // dev.GET("/migrations", handleMigrationManager)
	// dev.GET("/websocket", handleWebSocketTest)
	// dev.GET("/tests", handleTestRunner)
}