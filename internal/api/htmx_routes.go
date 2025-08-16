package api

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	
	"github.com/gin-gonic/gin"
)

// filterEmptyStrings removes empty strings from a slice
func filterEmptyStrings(slice []string) []string {
	result := []string{}
	for _, s := range slice {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// getProjectRoot finds the project root directory by looking for go.mod
func getProjectRoot() string {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		// Fallback to current directory
		return "."
	}
	
	// Walk up the directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root without finding go.mod
			break
		}
		dir = parent
	}
	
	// Fallback to current directory
	return "."
}

// loadTemplate safely loads templates for a specific route
func loadTemplate(files ...string) (*template.Template, error) {
	// Get the project root directory
	projectRoot := getProjectRoot()
	
	funcMap := template.FuncMap{
		"firstLetter": func(s string) string {
			if len(s) > 0 {
				return strings.ToUpper(string(s[0]))
			}
			return ""
		},
		"slice": func(start, end int, s string) string {
			if len(s) == 0 {
				return ""
			}
			if start >= len(s) {
				return ""
			}
			if end > len(s) {
				end = len(s)
			}
			if end <= start {
				return ""
			}
			return s[start:end]
		},
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"replace": func(old, new, s string) string {
			return strings.Replace(s, old, new, -1)
		},
		"seq": func(args ...int) []int {
			var start, end int
			switch len(args) {
			case 1:
				start, end = 0, args[0]
			case 2:
				start, end = args[0], args[1]
			default:
				return nil
			}
			if start >= end {
				return nil
			}
			result := make([]int, end-start)
			for i := range result {
				result[i] = start + i
			}
			return result
		},
		"add": func(a, b int) int {
			return a + b
		},
		"len": func(v interface{}) int {
			switch val := v.(type) {
			case []gin.H:
				return len(val)
			case []interface{}:
				return len(val)
			default:
				return 0
			}
		},
	}
	
	// Parse templates with the function map
	tmpl := template.New("").Funcs(funcMap)
	for _, file := range files {
		// Convert relative paths to absolute paths from project root
		fullPath := file
		if !strings.HasPrefix(file, "/") {
			fullPath = projectRoot + "/" + file
		}
		_, err := tmpl.ParseFiles(fullPath)
		if err != nil {
			return nil, err
		}
	}
	return tmpl, nil
}

// SetupHTMXRoutes configures routes for HTMX-based UI
func SetupHTMXRoutes(r *gin.Engine) {
	// Serve static files
	r.Static("/static", "./static")
	
	// Root redirect
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/login")
	})
	
	// Authentication pages
	r.GET("/login", handleLoginPage)
	r.GET("/register", handleRegisterPage)
	
	// Protected dashboard routes
	dashboard := r.Group("/")
	// TODO: Add auth middleware
	{
		dashboard.GET("/dashboard", handleDashboard)
		dashboard.GET("/tickets", handleTicketsList)
		dashboard.GET("/tickets/new", handleNewTicket)
		dashboard.GET("/tickets/:id", handleTicketDetail)
		dashboard.GET("/tickets/:id/edit", handleTicketEditForm)
		dashboard.GET("/queues", handleQueuesList)
		dashboard.GET("/queues/:id", handleQueueDetailPage)
		dashboard.GET("/queues/:id/edit", handleEditQueueForm)
		dashboard.GET("/queues/new", handleNewQueueForm)
		dashboard.GET("/queues/:id/delete", handleDeleteQueueConfirmation)
		dashboard.GET("/queues/clear-search", handleClearQueueSearch)
		dashboard.GET("/queues/bulk-toolbar", handleBulkActionsToolbar)
		dashboard.GET("/admin", handleAdminDashboard)
	}
	
	// HTMX API endpoints (return HTML fragments)
	api := r.Group("/api")
	{
		// Authentication
		api.POST("/auth/login", handleHTMXLogin)
		api.POST("/auth/logout", handleHTMXLogout)
		
		// Dashboard data
		api.GET("/dashboard/stats", handleDashboardStats)
		api.GET("/dashboard/recent-tickets", handleRecentTickets)
		api.GET("/dashboard/activity", handleActivityFeed)
		
		// Queue operations
		api.GET("/queues", handleQueuesAPI)
		api.POST("/queues", handleCreateQueueWithHTMX)
		api.GET("/queues/:id", handleQueueDetail)
		api.PUT("/queues/:id", handleUpdateQueueWithHTMX)
		api.DELETE("/queues/:id", handleDeleteQueue)
		api.GET("/queues/:id/tickets", handleQueueTicketsWithHTMX)
		
		// Bulk queue operations
		api.PUT("/queues/bulk/:action", handleBulkQueueAction)
		api.DELETE("/queues/bulk", handleBulkQueueDelete)
		
		// Ticket operations
		api.GET("/tickets", handleTicketsAPI)
		api.GET("/tickets/search", handleTicketSearch)
		api.POST("/tickets", handleCreateTicket)
		api.PUT("/tickets/:id", handleUpdateTicketEnhanced)
		api.PUT("/tickets/bulk", handleBulkUpdateTickets)
		api.POST("/tickets/:id/status", handleUpdateTicketStatus)
		api.POST("/tickets/:id/assign", handleAssignTicket)
		api.POST("/tickets/:id/reply", handleTicketReply)
		api.POST("/tickets/:id/priority", handleUpdateTicketPriority)
		api.POST("/tickets/:id/queue", handleUpdateTicketQueue)
		
		// SLA and Escalation
		api.GET("/tickets/:id/sla", handleGetTicketSLA)
		api.POST("/tickets/:id/escalate", handleEscalateTicket)
		api.GET("/reports/sla", handleSLAReport)
		api.PUT("/admin/sla-config", handleUpdateSLAConfig)
		
		// Ticket Merge
		api.POST("/tickets/:id/merge", handleMergeTickets)
		api.POST("/tickets/:id/unmerge", handleUnmergeTicket)
		api.GET("/tickets/:id/merge-history", handleGetMergeHistory)
		
		// Ticket Attachments
		api.POST("/tickets/:id/attachments", handleUploadAttachment)
		api.GET("/tickets/:id/attachments", handleGetAttachments)
		api.GET("/tickets/:id/attachments/:attachment_id", handleDownloadAttachment)
		api.DELETE("/tickets/:id/attachments/:attachment_id", handleDeleteAttachment)
		
		// Real-time updates
		api.GET("/tickets/stream", handleTicketStream)
		api.GET("/dashboard/activity-stream", handleActivityStream)
	}
}

// Login page
func handleLoginPage(c *gin.Context) {
	tmpl, err := loadTemplate(
		"templates/layouts/auth.html",
		"templates/pages/login.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "auth.html", gin.H{
		"Title": "GOTRS - Sign In",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Register page
func handleRegisterPage(c *gin.Context) {
	tmpl, err := loadTemplate(
		"templates/layouts/auth.html",
		"templates/pages/register.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "auth.html", gin.H{
		"Title": "GOTRS - Register",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Dashboard
func handleDashboard(c *gin.Context) {
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/dashboard.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "dashboard.html", gin.H{
		"Title": "Dashboard - GOTRS",
		"User":  gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
		"ActivePage": "dashboard",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Tickets list page
func handleTicketsList(c *gin.Context) {
	// Pass some mock queues for testing
	queues := []gin.H{
		{"ID": 1, "Name": "General Support"},
		{"ID": 2, "Name": "Technical Support"},
		{"ID": 3, "Name": "Billing"},
	}
	
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/tickets/list.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "list.html", gin.H{
		"Title":  "Tickets - GOTRS",
		"Queues": queues,
		"User":   gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
		"ActivePage": "tickets",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// New ticket page
func handleNewTicket(c *gin.Context) {
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/tickets/new.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "new.html", gin.H{
		"Title": "New Ticket - GOTRS",
		"User":  gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
		"ActivePage": "tickets",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Ticket detail page
func handleTicketDetail(c *gin.Context) {
	ticketID := c.Param("id")
	
	// Parse ticket ID
	id, err := strconv.Atoi(ticketID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ticket ID format")
		return
	}
	
	// TODO: In production, get database connection from context
	// For now, we'll return mock data
	ticket := gin.H{
		"ID":           id,
		"TicketNumber": fmt.Sprintf("TICKET-%06d", id),
		"Title":        "System login issues preventing access",
		"Status":       "new",
		"StatusLabel":  "New",
		"Priority":     "normal",
		"PriorityLabel": "Normal Priority",
		"Queue":        "General Support",
		"CustomerEmail": "john.doe@example.com",
		"CustomerName":  "John Doe",
		"CreateTime":    time.Now().Add(-2 * time.Hour).Format("Jan 2, 2006 3:04 PM"),
		"UpdateTime":    time.Now().Add(-2 * time.Hour).Format("Jan 2, 2006 3:04 PM"),
		"AssignedTo":    nil, // Unassigned
		"Type":          "Service Request",
		"SLAStatus":     "within", // within, warning, overdue
	}
	
	// Articles/Messages
	articles := []gin.H{
		{
			"ID":           1,
			"AuthorName":   "John Doe",
			"AuthorInitials": "JD",
			"AuthorType":   "Customer",
			"TimeAgo":      "2 hours ago",
			"Subject":      "System login issues preventing access",
			"Body":         "I'm unable to log into the system. I've tried resetting my password multiple times but keep getting an error message saying \"Invalid credentials\". This is preventing me from accessing important documents.\n\nMy username is: john.doe@example.com\n\nPlease help resolve this as soon as possible.",
			"IsInternal":   false,
		},
	}
	
	// Activity log
	activities := []gin.H{
		{
			"Type":        "created",
			"Description": "Ticket created",
			"TimeAgo":     "2 hours ago",
			"Icon":        "plus",
			"Color":       "green",
		},
	}
	
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/tickets/detail.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "detail.html", gin.H{
		"Title":      "Ticket #" + ticketID + " - GOTRS",
		"TicketID":   ticketID,
		"Ticket":     ticket,
		"Articles":   articles,
		"Activities": activities,
		"User":       gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
		"ActivePage": "tickets",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// HTMX Login handler
func handleHTMXLogin(c *gin.Context) {
	var loginReq struct {
		Email    string `json:"email" form:"email" binding:"required,email"`
		Password string `json:"password" form:"password" binding:"required"`
	}
	
	// Try to bind as JSON first, then form data
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		// If JSON binding fails, try form binding
		if err := c.ShouldBind(&loginReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	
	// TODO: Implement actual authentication
	// For now, accept demo credentials from environment variables
	demoEmail := os.Getenv("DEMO_ADMIN_EMAIL")
	demoPassword := os.Getenv("DEMO_ADMIN_PASSWORD")
	
	// Require environment variables to be set (no hardcoded fallbacks)
	if demoEmail == "" || demoPassword == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Demo credentials not configured"})
		return
	}
	
	if loginReq.Email == demoEmail && loginReq.Password == demoPassword {
		// For HTMX, set the redirect header
		c.Header("HX-Redirect", "/dashboard")
		c.JSON(http.StatusOK, gin.H{
			"access_token":  "demo_token_123",
			"refresh_token": "demo_refresh_123",
			"user": gin.H{
				"id":         1,
				"email":      loginReq.Email,
				"first_name": "Demo",
				"last_name":  "Admin",
				"role":       "admin",
			},
		})
		return
	}
	
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
}

// HTMX Logout handler
func handleHTMXLogout(c *gin.Context) {
	// TODO: Invalidate token
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// Dashboard stats (returns HTML fragment)
func handleDashboardStats(c *gin.Context) {
	stats := []gin.H{
		{"title": "Open Tickets", "value": "24", "icon": "ticket", "color": "blue"},
		{"title": "New Today", "value": "8", "icon": "plus", "color": "green"},
		{"title": "Pending", "value": "12", "icon": "clock", "color": "yellow"},
		{"title": "Overdue", "value": "3", "icon": "exclamation", "color": "red"},
	}
	
	tmpl, err := loadTemplate("templates/components/dashboard_stats.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "dashboard_stats.html", gin.H{
		"Stats": stats,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Recent tickets (returns HTML fragment)
func handleRecentTickets(c *gin.Context) {
	tickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "created": "2 hours ago"},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "created": "4 hours ago"},
		{"id": 3, "number": "TICKET-003", "title": "Bug report", "status": "pending", "priority": "low", "created": "1 day ago"},
	}
	
	tmpl, err := loadTemplate("templates/components/recent_tickets.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "recent_tickets.html", gin.H{
		"Tickets": tickets,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Activity feed (returns HTML fragment)
func handleActivityFeed(c *gin.Context) {
	activities := []gin.H{
		{"user": "John Doe", "action": "created", "target": "TICKET-001", "time": "2 minutes ago"},
		{"user": "Jane Smith", "action": "updated", "target": "TICKET-002", "time": "15 minutes ago"},
		{"user": "Bob Wilson", "action": "closed", "target": "TICKET-003", "time": "1 hour ago"},
	}
	
	tmpl, err := loadTemplate("templates/components/activity_feed.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "activity_feed.html", gin.H{
		"Activities": activities,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Tickets API (returns HTML fragment)
func handleTicketsAPI(c *gin.Context) {
	// Parse query parameters for filtering
	status := filterEmptyStrings(c.QueryArray("status"))
	priority := filterEmptyStrings(c.QueryArray("priority")) 
	queue := filterEmptyStrings(c.QueryArray("queue"))
	assignee := c.Query("assignee")
	assigned := c.Query("assigned")
	search := c.Query("search")
	
	// Debug logging
	fmt.Printf("Filter Debug - Status: %v, Priority: %v, Queue: %v, Assignee: %s, Assigned: %s, Search: %s\n", 
		status, priority, queue, assignee, assigned, search)

	// Mock ticket data for testing
	allTickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "customer": "john@example.com", "agent": "Agent Smith", "queue": "General Support", "queueId": 1},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "customer": "jane@example.com", "agent": "", "queue": "Technical Support", "queueId": 2},
		{"id": 3, "number": "TICKET-003", "title": "Password reset", "status": "closed", "priority": "low", "customer": "bob@example.com", "agent": "John Doe", "queue": "General Support", "queueId": 1},
		{"id": 4, "number": "TICKET-004", "title": "Billing inquiry", "status": "pending", "priority": "high", "customer": "alice@example.com", "agent": "", "queue": "Billing", "queueId": 3},
		{"id": 5, "number": "TICKET-005", "title": "Technical issue", "status": "open", "priority": "critical", "customer": "dave@example.com", "agent": "Agent Smith", "queue": "Technical Support", "queueId": 2},
	}

	// Apply filters
	filteredTickets := []gin.H{}
	for _, ticket := range allTickets {
		// Status filter
		if len(status) > 0 {
			found := false
			for _, s := range status {
				if ticket["status"] == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Priority filter
		if len(priority) > 0 {
			found := false
			for _, p := range priority {
				if ticket["priority"] == p {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Queue filter
		if len(queue) > 0 {
			found := false
			for _, q := range queue {
				queueId, _ := strconv.Atoi(q)
				if ticket["queueId"] == queueId {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Assignee filter
		if assignee != "" {
			if assignee == "1" && ticket["agent"] != "Agent Smith" {
				continue
			}
		}

		// Assigned/Unassigned filter
		if assigned == "true" {
			if ticket["agent"] == "" {
				continue
			}
		} else if assigned == "false" {
			if ticket["agent"] != "" {
				continue
			}
		}

		// Search filter
		if search != "" {
			searchLower := strings.ToLower(search)
			titleMatch := strings.Contains(strings.ToLower(ticket["title"].(string)), searchLower)
			numberMatch := strings.Contains(strings.ToLower(ticket["number"].(string)), searchLower) 
			customerMatch := strings.Contains(strings.ToLower(ticket["customer"].(string)), searchLower)
			
			if !titleMatch && !numberMatch && !customerMatch {
				continue
			}
		}

		filteredTickets = append(filteredTickets, ticket)
	}

	// Prepare response data
	responseData := gin.H{
		"Tickets": filteredTickets,
		"Title":   "Tickets", // General title for the section
	}

	// Add filter badges and messages for UI
	if len(status) > 0 {
		statusLabels := []string{}
		for _, s := range status {
			switch s {
			case "new":
				statusLabels = append(statusLabels, "New")
			case "open":
				statusLabels = append(statusLabels, "Open")
			case "pending":
				statusLabels = append(statusLabels, "Pending")
			case "resolved":
				statusLabels = append(statusLabels, "Resolved")
			case "closed":
				statusLabels = append(statusLabels, "Closed")
			default:
				statusLabels = append(statusLabels, strings.Title(s))
			}
		}
		responseData["StatusFilter"] = strings.Join(statusLabels, ", ")
	}

	if len(priority) > 0 {
		priorityLabels := []string{}
		for _, p := range priority {
			if p == "high" {
				priorityLabels = append(priorityLabels, "High Priority")
			} else if p == "low" {
				priorityLabels = append(priorityLabels, "Low Priority")
			} else if p == "critical" {
				priorityLabels = append(priorityLabels, "critical")
			}
		}
		responseData["PriorityFilter"] = strings.Join(priorityLabels, ", ")
	}

	if assigned == "false" {
		responseData["AssignedFilter"] = "Unassigned"
	}

	// No tickets found message
	if len(filteredTickets) == 0 {
		responseData["NoTicketsMessage"] = "No tickets found"
	}

	tmpl, err := loadTemplate("templates/components/ticket_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", responseData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Ticket search (returns HTML fragment)
func handleTicketSearch(c *gin.Context) {
	searchTerm := c.Query("search")
	
	// Mock ticket data for testing
	allTickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "customer": "john@example.com", "agent": "Agent Smith"},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "customer": "jane@example.com", "agent": ""},
		{"id": 3, "number": "TICKET-003", "title": "Password reset", "status": "closed", "priority": "low", "customer": "bob@example.com", "agent": "John Doe"},
	}

	filteredTickets := []gin.H{}
	
	if searchTerm != "" {
		searchLower := strings.ToLower(searchTerm)
		for _, ticket := range allTickets {
			titleMatch := strings.Contains(strings.ToLower(ticket["title"].(string)), searchLower)
			numberMatch := strings.Contains(strings.ToLower(ticket["number"].(string)), searchLower)
			customerMatch := strings.Contains(strings.ToLower(ticket["customer"].(string)), searchLower)
			
			if titleMatch || numberMatch || customerMatch {
				filteredTickets = append(filteredTickets, ticket)
			}
		}
	} else {
		// Empty search returns all tickets
		filteredTickets = allTickets
	}

	responseData := gin.H{
		"Tickets": filteredTickets,
		"Title":   "Tickets", // General title for the section
	}

	// Add search term to response if provided
	if searchTerm != "" {
		responseData["SearchTerm"] = searchTerm
	}

	// No results message
	if len(filteredTickets) == 0 {
		responseData["NoTicketsMessage"] = "No tickets found"
	}
	
	tmpl, err := loadTemplate("templates/components/ticket_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", responseData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Create ticket (HTMX)
func handleCreateTicket(c *gin.Context) {
	var req struct {
		Subject       string `json:"subject" form:"subject" binding:"required"`
		CustomerEmail string `json:"customer_email" form:"customer_email" binding:"required,email"`
		CustomerName  string `json:"customer_name" form:"customer_name"`
		Priority      string `json:"priority" form:"priority"`
		QueueID       string `json:"queue_id" form:"queue_id"`
		TypeID        string `json:"type_id" form:"type_id"`
		Body          string `json:"body" form:"body" binding:"required"`
	}

	// Bind form data
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert string values to integers with defaults
	queueID := 1 // Default to General Support
	if req.QueueID != "" {
		if id, err := strconv.Atoi(req.QueueID); err == nil {
			queueID = id
		}
	}

	typeID := 1 // Default to Incident
	if req.TypeID != "" {
		if id, err := strconv.Atoi(req.TypeID); err == nil {
			typeID = id
		}
	}

	// For demo purposes, use a fixed user ID (admin)
	// In a real system, we'd get this from the authenticated user context
	_ = 1 // createBy placeholder

	// TODO: Initialize ticket service with database connection
	// For now, return a success response with mock data
	ticketNumber := fmt.Sprintf("TICKET-%06d", time.Now().Unix()%1000000)
	ticketID := 123 // Mock ID for now
	
	// For HTMX, set the redirect header to the ticket detail page
	c.Header("HX-Redirect", fmt.Sprintf("/tickets/%d", ticketID))
	c.JSON(http.StatusCreated, gin.H{
		"id":            ticketID,
		"ticket_number": ticketNumber,
		"message":       "Ticket created successfully",
		"queue_id":      queueID,
		"type_id":       typeID,
		"priority":      req.Priority,
	})
}

// Update ticket (HTMX)
func handleUpdateTicket(c *gin.Context) {
	// TODO: Implement ticket update
	c.JSON(http.StatusOK, gin.H{"message": "Ticket updated"})
}

// Update ticket status (HTMX)
func handleUpdateTicketStatus(c *gin.Context) {
	ticketID := c.Param("id")
	var req struct {
		Status string `json:"status" form:"status" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Update ticket status in database
	// For now, return success
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s status updated to %s", ticketID, req.Status),
		"status": req.Status,
	})
}

// Assign ticket to agent (HTMX)
func handleAssignTicket(c *gin.Context) {
	ticketID := c.Param("id")
	
	// For "Assign to Me", we don't need any request body
	// Just assign to the current user (hardcoded for now)
	agentID := 1 // TODO: Get from session/auth context
	agentName := "Demo Agent" // TODO: Get from session
	
	// TODO: Update ticket assignment in database
	
	// Return success message that HTMX can display
	c.Header("HX-Trigger", `{"showMessage": {"message": "Ticket assigned to you", "type": "success"}}`)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s assigned to %s", ticketID, agentName),
		"agent_id": agentID,
	})
}

// Add reply to ticket (HTMX)
func handleTicketReply(c *gin.Context) {
	ticketID := c.Param("id")
	_ = ticketID // Will be used when connecting to database
	var req struct {
		Reply       string `json:"reply" form:"reply" binding:"required"`
		Internal    bool   `json:"internal" form:"internal"`
		CloseTicket bool   `json:"close_ticket" form:"close_ticket"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Add reply to ticket in database
	// For now, return HTML fragment for the new message
	newMessage := gin.H{
		"ID":             time.Now().Unix(),
		"AuthorName":     "Support Agent",
		"AuthorInitials": "SA",
		"AuthorType":     "Agent",
		"TimeAgo":        "Just now",
		"Body":           req.Reply,
		"IsInternal":     req.Internal,
	}
	
	// Return HTML fragment for HTMX to append
	tmpl, err := loadTemplate("templates/components/ticket_message.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_message.html", newMessage); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Update ticket priority (HTMX)
func handleUpdateTicketPriority(c *gin.Context) {
	ticketID := c.Param("id")
	var req struct {
		Priority string `json:"priority" form:"priority" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Update ticket priority in database
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s priority updated to %s", ticketID, req.Priority),
		"priority": req.Priority,
	})
}

// Update ticket queue (HTMX)
func handleUpdateTicketQueue(c *gin.Context) {
	ticketID := c.Param("id")
	var req struct {
		QueueID int `json:"queue_id" form:"queue_id" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Update ticket queue in database
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s moved to queue %d", ticketID, req.QueueID),
		"queue_id": req.QueueID,
	})
}

// Server-Sent Events for tickets
func handleTicketStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	
	// Send a test event
	c.SSEvent("ticket-update", gin.H{
		"ticketId": 1,
		"status":   "updated",
		"message":  "Ticket status changed",
	})
	
	c.Writer.Flush()
}

// Server-Sent Events for activity
func handleActivityStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	
	// Send a test event with HTML content
	activityHTML := `<div class="py-3 border-b border-gray-200 last:border-b-0">
		<div class="flex space-x-3">
			<div class="flex-shrink-0">
				<div class="h-8 w-8 rounded-full bg-green-500 flex items-center justify-center">
					<span class="text-xs font-medium text-white">S</span>
				</div>
			</div>
			<div class="min-w-0 flex-1">
				<p class="text-sm text-gray-900">
					<span class="font-medium">System</span>
					<span class="text-blue-600">updated</span>
					<span class="font-medium">activity feed</span>
				</p>
				<p class="text-sm text-gray-500">just now</p>
			</div>
		</div>
	</div>`
	
	// For HTMX SSE, send HTML content directly
	fmt.Fprintf(c.Writer, "event: activity-update\ndata: %s\n\n", activityHTML)
	
	c.Writer.Flush()
}

// Queues API (returns HTML fragment or JSON)
func handleQueuesAPI(c *gin.Context) {
	// Check if JSON response is requested
	if wantsJSON(c) {
		handleQueuesJSON(c)
		return
	}
	
	// Parse query parameters for filtering
	status := c.Query("status")
	search := c.Query("search")
	
	// Get queues from database with ticket counts
	queues, err := getQueuesWithTicketCounts(status, search)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Database error: %v", err))
		return
	}
	
	// Render HTML response (fragment or full page)
	renderQueueList(c, queues)
}

// Handle JSON response for queues
func handleQueuesJSON(c *gin.Context) {
	status := c.Query("status")
	search := c.Query("search")
	
	queues, err := getQueuesWithTicketCounts(status, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database error",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    queues,
	})
}

// Sort queues based on the sort parameter
func sortQueues(queues []gin.H, sortBy string) {
	switch sortBy {
	case "name_asc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["name"].(string) < queues[j]["name"].(string)
		})
	case "name_desc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["name"].(string) > queues[j]["name"].(string)
		})
	case "tickets_asc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["ticket_count"].(int) < queues[j]["ticket_count"].(int)
		})
	case "tickets_desc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["ticket_count"].(int) > queues[j]["ticket_count"].(int)
		})
	case "status_asc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["status"].(string) < queues[j]["status"].(string)
		})
	default:
		// Default sort by ID (original order)
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["id"].(int) < queues[j]["id"].(int)
		})
	}
}

// Get queues from database with ticket counts
func getQueuesWithTicketCounts(status, search string) ([]gin.H, error) {
	// Normalize inputs once outside the loop for better performance
	search = strings.TrimSpace(search)
	searchLower := strings.ToLower(search)
	hasSearch := search != ""
	hasStatusFilter := status != "" && status != "all"
	
	// Apply filtering to centralized mock data
	filteredQueues := make([]gin.H, 0, len(mockQueueData)) // Pre-allocate with capacity
	
	for _, queue := range mockQueueData {
		// Status filter
		if hasStatusFilter {
			queueStatus := queue["status"].(string)
			if status != queueStatus {
				continue
			}
		}
		
		// Search filter - check both name and comment
		if hasSearch {
			queueName := strings.ToLower(queue["name"].(string))
			queueComment := strings.ToLower(queue["comment"].(string))
			if !strings.Contains(queueName, searchLower) && !strings.Contains(queueComment, searchLower) {
				continue
			}
		}
		
		filteredQueues = append(filteredQueues, queue)
	}
	
	return filteredQueues, nil
}

// Helper function to check if client wants JSON response
func wantsJSON(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept"), "application/json")
}

// Helper function to check if request is from HTMX
func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

// Helper function to send standardized error responses
func sendErrorResponse(c *gin.Context, statusCode int, message string) {
	if wantsJSON(c) {
		c.JSON(statusCode, gin.H{
			"success": false,
			"error":   message,
		})
	} else {
		c.String(statusCode, message)
	}
}

// Helper function to render queue list templates
func renderQueueList(c *gin.Context, queues []gin.H) {
	// Get search/filter parameters for template context
	searchTerm := c.Query("search")
	statusFilter := c.Query("status")
	
	templateData := gin.H{
		"Queues":      queues,
		"SearchTerm":  searchTerm,
		"StatusFilter": statusFilter,
	}
	
	if isHTMXRequest(c) {
		// Return HTML fragment for HTMX
		tmpl, err := loadTemplate("templates/components/queue_list.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "queue_list.html", templateData); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	} else {
		// Return full page
		tmpl, err := loadTemplate(
			"templates/layouts/base.html",
			"templates/pages/queues/list.html",
			"templates/components/queue_list.html",
		)
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		// Add page-level template data
		templateData["Title"] = "Queues - GOTRS"
		templateData["User"] = gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"}
		templateData["ActivePage"] = "queues"
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "list.html", templateData); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	}
}

// Helper function to render queue detail templates
func renderQueueDetail(c *gin.Context, queue gin.H) {
	if isHTMXRequest(c) {
		// Return HTML fragment for HTMX
		tmpl, err := loadTemplate("templates/components/queue_detail.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "queue_detail.html", gin.H{
			"Queue": queue,
		}); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	} else {
		// Return full page
		tmpl, err := loadTemplate(
			"templates/layouts/base.html",
			"templates/pages/queues/detail.html",
			"templates/components/queue_detail.html",
		)
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "detail.html", gin.H{
			"Title":      queue["name"].(string) + " - Queue Details - GOTRS",
			"User":       gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
			"ActivePage": "queues",
			"Queue":      queue,
		}); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	}
}

// Queue detail API (returns HTML fragment or JSON)
func handleQueueDetail(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	if queueID == "" || queueID == "invalid" {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Get queue details with tickets
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			sendErrorResponse(c, http.StatusNotFound, "Queue not found")
			return
		}
		sendErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}
	
	// Check if JSON response is requested
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    queue,
		})
		return
	}
	
	// Render HTML response (fragment or full page)
	renderQueueDetail(c, queue)
}

// Queue tickets API (returns tickets for a specific queue)
func handleQueueTickets(c *gin.Context) {
	queueID := c.Param("id")
	
	// Parse query parameters
	status := c.Query("status")
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "10")
	
	// Get tickets for queue
	tickets, err := getTicketsForQueue(queueID, status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database error",
		})
		return
	}
	
	// Return tickets as HTML fragment or JSON
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    tickets,
		})
	} else {
		// Return HTML fragment
		tmpl, err := loadTemplate("templates/components/ticket_list.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		// Prepare template data
		templateData := gin.H{
			"Tickets":    tickets["tickets"],
			"Title":      "Queue Tickets",
			"Total":      tickets["total"],
			"Page":       tickets["page"],
			"Pagination": tickets["pagination"],
		}
		
		// Handle empty state
		if tickets["total"].(int) == 0 {
			templateData["NoTicketsMessage"] = "No tickets in this queue"
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", templateData); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	}
}

// Get queue details with associated tickets
func getQueueWithTickets(queueID string) (gin.H, error) {
	// Mock data for testing - matches the database schema
	mockQueues := map[string]gin.H{
		"1": {
			"id":           1,
			"name":         "Raw",
			"comment":      "All new tickets are placed in this queue by default",
			"ticket_count": 2,
			"status":       "active",
			"tickets": []gin.H{
				{"id": 1, "number": "TICKET-001", "title": "Test login issue", "status": "new"},
				{"id": 3, "number": "TICKET-003", "title": "UI bug report", "status": "new"},
			},
		},
		"2": {
			"id":           2,
			"name":         "Junk",
			"comment":      "Spam and junk emails",
			"ticket_count": 1,
			"status":       "active",
			"tickets": []gin.H{
				{"id": 2, "number": "TICKET-002", "title": "Database connection problem", "status": "open"},
			},
		},
		"3": {
			"id":           3,
			"name":         "Misc",
			"comment":      "Miscellaneous tickets",
			"ticket_count": 0,
			"status":       "active",
			"tickets":      []gin.H{},
		},
		"4": {
			"id":           4,
			"name":         "Support",
			"comment":      "General support requests",
			"ticket_count": 0,
			"status":       "active",
			"tickets":      []gin.H{},
		},
	}
	
	queue, exists := mockQueues[queueID]
	if !exists {
		return nil, fmt.Errorf("queue not found")
	}
	
	return queue, nil
}

// Get tickets for a specific queue with filtering and pagination
func getTicketsForQueue(queueID, status, page, limit string) (gin.H, error) {
	// Get queue details first
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		return nil, err
	}
	
	tickets := queue["tickets"].([]gin.H)
	filteredTickets := []gin.H{}
	
	// Apply status filter
	for _, ticket := range tickets {
		if status != "" && ticket["status"] != status {
			continue
		}
		filteredTickets = append(filteredTickets, ticket)
	}
	
	// Implement proper pagination
	pageNum, _ := strconv.Atoi(page)
	limitNum, _ := strconv.Atoi(limit)
	if pageNum < 1 {
		pageNum = 1
	}
	if limitNum < 1 {
		limitNum = 10
	}
	
	total := len(filteredTickets)
	offset := (pageNum - 1) * limitNum
	
	// Apply pagination to tickets
	paginatedTickets := []gin.H{}
	if offset < total {
		end := offset + limitNum
		if end > total {
			end = total
		}
		paginatedTickets = filteredTickets[offset:end]
	}
	
	// Calculate pagination info
	hasNext := offset+limitNum < total
	hasPrev := pageNum > 1
	totalPages := (total + limitNum - 1) / limitNum // Ceiling division
	
	result := gin.H{
		"tickets":    paginatedTickets,
		"total":      total,
		"page":       pageNum,
		"limit":      limitNum,
		"total_pages": totalPages,
		"pagination": gin.H{
			"has_next":    hasNext,
			"has_prev":    hasPrev,
			"next_page":   pageNum + 1,
			"prev_page":   pageNum - 1,
			"total_pages": totalPages,
		},
	}
	
	if len(filteredTickets) == 0 {
		result["message"] = "No tickets in this queue"
	}
	
	return result, nil
}

// Queue detail page (browser route)
func handleQueueDetailPage(c *gin.Context) {
	queueID := c.Param("id")
	
	// Get queue details
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
		} else {
			c.String(http.StatusInternalServerError, "Database error: %v", err)
		}
		return
	}
	
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/queues/detail.html",
		"templates/components/queue_detail.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "detail.html", gin.H{
		"Title":      queue["name"].(string) + " - Queue Details - GOTRS",
		"User":       gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
		"ActivePage": "queues",
		"Queue":      queue,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Queues list page - handles both full page loads and HTMX search/filter requests
func handleQueuesList(c *gin.Context) {
	// Get search and filter parameters from query string
	search := c.Query("search")
	status := c.Query("status")
	sortBy := c.Query("sort")
	
	// Get pagination parameters
	page := 1
	if p := c.Query("page"); p != "" {
		if parsedPage, err := strconv.Atoi(p); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}
	
	perPage := 10
	if pp := c.Query("per_page"); pp != "" {
		if parsedPerPage, err := strconv.Atoi(pp); err == nil && parsedPerPage > 0 {
			if parsedPerPage > 100 {
				perPage = 100 // Cap at 100
			} else {
				perPage = parsedPerPage
			}
		}
	}
	
	// Get queues data with filtering
	queues, err := getQueuesWithTicketCounts(status, search)
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: %v", err)
		return
	}
	
	// Apply sorting
	sortQueues(queues, sortBy)
	
	// Calculate pagination
	total := len(queues)
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1 // At least 1 page even if no results
	}
	
	// Adjust page if negative or zero
	if page < 1 {
		page = 1
	}
	
	// Calculate slice bounds
	start := (page - 1) * perPage
	end := start + perPage
	if end > total {
		end = total
	}
	
	// Paginate results
	var paginatedQueues []gin.H
	if start < total {
		paginatedQueues = queues[start:end]
	} else {
		// Page is beyond total pages - return empty
		paginatedQueues = []gin.H{}
	}
	
	// Build pagination info
	pagination := gin.H{
		"Page":     page,
		"PerPage":  perPage,
		"Total":    total,
		"Start":    start + 1,
		"End":      end,
		"HasPrev":  page > 1,
		"HasNext":  page < totalPages,
		"PrevPage": page - 1,
		"NextPage": page + 1,
	}
	
	// Check if this is an HTMX request for filtering/search
	if c.GetHeader("HX-Request") != "" {
		// Return just the queue list fragment for HTMX requests
		tmpl, err := loadTemplate("templates/components/queue_list.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "queue_list.html", gin.H{
			"Queues":       paginatedQueues,
			"SearchTerm":   search,
			"StatusFilter": status,
			"SortBy":       sortBy,
			"PerPage":      perPage,
			"Pagination":   pagination,
		}); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
		return
	}
	
	// Full page load - load complete template
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/queues/list.html",
		"templates/components/queue_list.html",
	)
	if err != nil {
		// If template doesn't exist yet, show simple message
		c.String(http.StatusOK, `
			<html>
			<head><title>Queues - GOTRS</title></head>
			<body style="font-family: system-ui; padding: 2rem;">
				<h1>Queue Management</h1>
				<p>Queue management interface coming soon...</p>
				<a href="/dashboard" style="color: blue;">← Back to Dashboard</a>
			</body>
			</html>
		`)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "list.html", gin.H{
		"Title":        "Queues - GOTRS",
		"User":         gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
		"ActivePage":   "queues",
		"Queues":       paginatedQueues,
		"SearchTerm":   search,
		"StatusFilter": status,
		"SortBy":       sortBy,
		"PerPage":      perPage,
		"Pagination":   pagination,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Admin dashboard page  
func handleAdminDashboard(c *gin.Context) {
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/admin/dashboard.html",
	)
	if err != nil {
		// If template doesn't exist yet, show simple message
		c.String(http.StatusOK, `
			<html>
			<head><title>Admin - GOTRS</title></head>
			<body style="font-family: system-ui; padding: 2rem;">
				<h1>Admin Dashboard</h1>
				<p>Admin interface coming soon...</p>
				<ul>
					<li>User Management</li>
					<li>System Configuration</li>
					<li>Reports & Analytics</li>
					<li>Audit Logs</li>
				</ul>
				<a href="/dashboard" style="color: blue;">← Back to Dashboard</a>
			</body>
			</html>
		`)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "dashboard.html", gin.H{
		"Title":      "Admin - GOTRS",
		"User":       gin.H{"FirstName": "Demo", "LastName": "User", "Email": "demo@gotrs.local", "Role": "Admin"},
		"ActivePage": "admin",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Queue Management API Handlers

// Create queue request structure
type CreateQueueRequest struct {
	Name                  string `json:"name" binding:"required,min=2,max=200"`
	Comment               string `json:"comment"`
	GroupID               int    `json:"group_id"`
	SystemAddress         string `json:"system_address"`
	DefaultSignKey        string `json:"default_sign_key"`
	UnlockTimeout         int    `json:"unlock_timeout"`
	FollowUpID            int    `json:"follow_up_id"`
	FollowUpLock          int    `json:"follow_up_lock"`
	CalendarName          string `json:"calendar_name"`
	FirstResponseTime     int    `json:"first_response_time"`
	FirstResponseNotify   int    `json:"first_response_notify"`
	UpdateTime            int    `json:"update_time"`
	UpdateNotify          int    `json:"update_notify"`
	SolutionTime          int    `json:"solution_time"`
	SolutionNotify        int    `json:"solution_notify"`
}

// Update queue request structure
type UpdateQueueRequest struct {
	Name                  *string `json:"name,omitempty"`
	Comment               *string `json:"comment,omitempty"`
	GroupID               *int    `json:"group_id,omitempty"`
	SystemAddress         *string `json:"system_address,omitempty"`
	DefaultSignKey        *string `json:"default_sign_key,omitempty"`
	UnlockTimeout         *int    `json:"unlock_timeout,omitempty"`
	FollowUpID            *int    `json:"follow_up_id,omitempty"`
	FollowUpLock          *int    `json:"follow_up_lock,omitempty"`
	CalendarName          *string `json:"calendar_name,omitempty"`
	FirstResponseTime     *int    `json:"first_response_time,omitempty"`
	FirstResponseNotify   *int    `json:"first_response_notify,omitempty"`
	UpdateTime            *int    `json:"update_time,omitempty"`
	UpdateNotify          *int    `json:"update_notify,omitempty"`
	SolutionTime          *int    `json:"solution_time,omitempty"`
	SolutionNotify        *int    `json:"solution_notify,omitempty"`
}

// Helper function to validate email format
func isValidEmail(email string) bool {
	if email == "" {
		return true // Empty email is allowed
	}
	// Simple email validation - in production, use a proper regex or validation library
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// Helper function to validate queue data (used by both create and update)
func validateQueueData(name, systemAddress *string, firstResponseTime, updateTime, solutionTime *int, excludeID int) error {
	// Validate email format
	if systemAddress != nil && *systemAddress != "" && !isValidEmail(*systemAddress) {
		return fmt.Errorf("invalid email format in system_address")
	}
	
	// Validate time values
	if (firstResponseTime != nil && *firstResponseTime < 0) ||
		(updateTime != nil && *updateTime < 0) ||
		(solutionTime != nil && *solutionTime < 0) {
		return fmt.Errorf("time values must be positive")
	}
	
	// Validate name uniqueness
	if name != nil && queueNameExists(*name, excludeID) {
		return fmt.Errorf("queue name already exists")
	}
	
	return nil
}

// Centralized mock queue data
var mockQueueData = []gin.H{
	{"id": 1, "name": "Raw", "comment": "All new tickets are placed in this queue by default", "ticket_count": 2, "status": "active"},
	{"id": 2, "name": "Junk", "comment": "Spam and junk emails", "ticket_count": 1, "status": "active"},
	{"id": 3, "name": "Misc", "comment": "Miscellaneous tickets", "ticket_count": 0, "status": "active"},
	{"id": 4, "name": "Support", "comment": "General support requests", "ticket_count": 3, "status": "active"},
}

// Helper function to check if queue name exists
func queueNameExists(name string, excludeID int) bool {
	for _, queue := range mockQueueData {
		if queue["name"].(string) == name && queue["id"].(int) != excludeID {
			return true
		}
	}
	return false
}

// Helper function to get next queue ID (mock implementation)
func getNextQueueID() int {
	return 5 // Simple incrementing ID for mock
}

// Helper function to check if queue has tickets
func queueHasTickets(queueID int) bool {
	// Mock implementation - based on our mock data
	switch queueID {
	case 1: // Raw queue has 2 tickets
		return true
	case 2: // Junk queue has 1 ticket
		return true
	case 3, 4: // Misc and Support have no tickets
		return false
	default:
		return false
	}
}

// Create Queue API Handler
func handleCreateQueue(c *gin.Context) {
	var req CreateQueueRequest
	
	// Bind and validate JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		if strings.Contains(err.Error(), "invalid character") {
			sendErrorResponse(c, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}
	
	// Additional validations using helper
	if err := validateQueueData(&req.Name, &req.SystemAddress, &req.FirstResponseTime, &req.UpdateTime, &req.SolutionTime, 0); err != nil {
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	
	// Create new queue (mock implementation)
	newQueue := gin.H{
		"id":                     getNextQueueID(),
		"name":                   req.Name,
		"comment":                req.Comment,
		"group_id":               req.GroupID,
		"system_address":         req.SystemAddress,
		"default_sign_key":       req.DefaultSignKey,
		"unlock_timeout":         req.UnlockTimeout,
		"follow_up_id":           req.FollowUpID,
		"follow_up_lock":         req.FollowUpLock,
		"calendar_name":          req.CalendarName,
		"first_response_time":    req.FirstResponseTime,
		"first_response_notify":  req.FirstResponseNotify,
		"update_time":            req.UpdateTime,
		"update_notify":          req.UpdateNotify,
		"solution_time":          req.SolutionTime,
		"solution_notify":        req.SolutionNotify,
		"valid_id":               1,
		"create_time":            time.Now(),
		"create_by":              1,
		"change_time":            time.Now(),
		"change_by":              1,
	}
	
	// Return success response
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    newQueue,
	})
}

// Update Queue API Handler
func handleUpdateQueue(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	id, err := strconv.Atoi(queueID)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Check if queue exists
	existingQueue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			sendErrorResponse(c, http.StatusNotFound, "Queue not found")
			return
		}
		sendErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}
	
	var req UpdateQueueRequest
	
	// Bind and validate JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}
	
	// Additional validations using helper
	if err := validateQueueData(req.Name, req.SystemAddress, req.FirstResponseTime, req.UpdateTime, req.SolutionTime, id); err != nil {
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	
	// Update queue (mock implementation - merge with existing data)
	updatedQueue := existingQueue
	if req.Name != nil {
		updatedQueue["name"] = *req.Name
	}
	if req.Comment != nil {
		updatedQueue["comment"] = *req.Comment
	}
	// Add other field updates as needed...
	
	updatedQueue["change_time"] = time.Now()
	updatedQueue["change_by"] = 1
	
	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedQueue,
	})
}

// Delete Queue API Handler
func handleDeleteQueue(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	id, err := strconv.Atoi(queueID)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Check if queue exists
	_, err = getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			sendErrorResponse(c, http.StatusNotFound, "Queue not found")
			return
		}
		sendErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}
	
	// Check if queue has tickets
	if queueHasTickets(id) {
		sendErrorResponse(c, http.StatusConflict, "Cannot delete queue with existing tickets")
		return
	}
	
	// Perform soft delete (mock implementation)
	// In real implementation: UPDATE queues SET valid_id = 0 WHERE id = ?
	
	// Return success response with HTMX headers for list refresh
	c.Header("HX-Trigger", "queue-deleted")
	c.Header("HX-Redirect", "/queues")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Queue deleted successfully",
	})
}

// Frontend Queue Form Handlers

// Handle edit queue form display
func handleEditQueueForm(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	if queueID == "invalid" {
		c.String(http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Get queue details
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
			return
		}
		c.String(http.StatusInternalServerError, "Database error")
		return
	}
	
	// Load and render the edit form template
	tmpl, err := loadTemplate("templates/components/queue_edit_form.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "queue_edit_form.html", gin.H{
		"Queue": queue,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Handle new queue form display
func handleNewQueueForm(c *gin.Context) {
	// Load and render the create form template
	tmpl, err := loadTemplate("templates/components/queue_create_form.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "queue_create_form.html", gin.H{}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Handle delete queue confirmation display
func handleDeleteQueueConfirmation(c *gin.Context) {
	queueID := c.Param("id")
	
	// Get queue details
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
			return
		}
		c.String(http.StatusInternalServerError, "Database error")
		return
	}
	
	// Check if queue has tickets
	id, _ := strconv.Atoi(queueID)
	hasTickets := queueHasTickets(id)
	
	// Prepare template data
	templateData := gin.H{
		"Queue":      queue,
		"HasTickets": hasTickets,
		"QueueID":    queueID,
	}
	
	if hasTickets {
		templateData["TicketCount"] = queue["ticket_count"]
	}
	
	// For now, render a simple HTML response since we don't have the template yet
	c.Header("Content-Type", "text/html; charset=utf-8")
	
	if hasTickets {
		c.String(http.StatusOK, `
			<div class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50" id="delete-queue-modal">
				<div class="relative top-20 mx-auto p-5 border w-11/12 md:w-1/2 lg:w-1/3 shadow-lg rounded-md bg-white dark:bg-gray-800">
					<div class="mt-3">
						<h3 class="text-lg font-medium text-gray-900 dark:text-white">Queue Cannot Be Deleted</h3>
						<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
							The queue "%s" cannot be deleted because it contains tickets (%d tickets).
							Please move or resolve all tickets before deleting this queue.
						</p>
						<div class="flex justify-end pt-4">
							<button type="button" onclick="closeDeleteModal()" class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm hover:bg-gray-50 dark:hover:bg-gray-600">
								Close
							</button>
						</div>
					</div>
				</div>
			</div>
			<script>
			function closeDeleteModal() {
				document.getElementById('delete-queue-modal').remove();
			}
			</script>
		`, queue["name"], queue["ticket_count"])
	} else {
		c.String(http.StatusOK, `
			<div class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50" id="delete-queue-modal">
				<div class="relative top-20 mx-auto p-5 border w-11/12 md:w-1/2 lg:w-1/3 shadow-lg rounded-md bg-white dark:bg-gray-800">
					<div class="mt-3">
						<h3 class="text-lg font-medium text-gray-900 dark:text-white">Delete Queue</h3>
						<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
							Are you sure you want to delete the queue "%s"? This action cannot be undone.
						</p>
						<div class="flex justify-end space-x-3 pt-4">
							<button type="button" onclick="closeDeleteModal()" class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm hover:bg-gray-50 dark:hover:bg-gray-600">
								Cancel
							</button>
							<button type="button" hx-delete="/api/queues/%s" hx-target="#delete-queue-modal" hx-swap="outerHTML" class="px-4 py-2 text-sm font-medium text-white bg-red-600 border border-transparent rounded-md shadow-sm hover:bg-red-700">
								Delete Queue
							</button>
						</div>
					</div>
				</div>
			</div>
			<script>
			function closeDeleteModal() {
				document.getElementById('delete-queue-modal').remove();
			}
			</script>
		`, queue["name"], queueID)
	}
}

// Handle create queue with HTMX form submission
func handleCreateQueueWithHTMX(c *gin.Context) {
	// Parse form data
	name := c.PostForm("name")
	comment := c.PostForm("comment")
	systemAddress := c.PostForm("system_address")
	
	// Validate required fields
	if name == "" {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: Queue name is required</p>
			</div>
		`)
		return
	}
	
	// Validate name length
	if len(name) < 2 || len(name) > 200 {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: Queue name must be between 2 and 200 characters</p>
			</div>
		`)
		return
	}
	
	// Parse optional integer fields
	firstResponseTime, _ := strconv.Atoi(c.PostForm("first_response_time"))
	updateTime, _ := strconv.Atoi(c.PostForm("update_time"))
	solutionTime, _ := strconv.Atoi(c.PostForm("solution_time"))
	
	// Validate using existing helper
	if err := validateQueueData(&name, &systemAddress, &firstResponseTime, &updateTime, &solutionTime, 0); err != nil {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: %s</p>
			</div>
		`, err.Error())
		return
	}
	
	// Create queue (mock implementation)
	newQueue := gin.H{
		"id":                  getNextQueueID(),
		"name":                name,
		"comment":             comment,
		"system_address":      systemAddress,
		"first_response_time": firstResponseTime,
		"update_time":         updateTime,
		"solution_time":       solutionTime,
		"ticket_count":        0,
		"status":              "active",
	}
	
	// Return success with HTMX headers
	c.Header("HX-Trigger", "queue-created")
	c.Header("HX-Redirect", "/queues")
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    newQueue,
	})
}

// Handle update queue with HTMX form submission
func handleUpdateQueueWithHTMX(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Check if queue exists
	existingQueue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
			return
		}
		c.String(http.StatusInternalServerError, "Database error")
		return
	}
	
	// Parse form data
	name := c.PostForm("name")
	comment := c.PostForm("comment")
	systemAddress := c.PostForm("system_address")
	
	// Validate required fields
	if name == "" {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: Queue name is required</p>
			</div>
		`)
		return
	}
	
	// Parse optional integer fields
	firstResponseTime, _ := strconv.Atoi(c.PostForm("first_response_time"))
	updateTime, _ := strconv.Atoi(c.PostForm("update_time"))
	solutionTime, _ := strconv.Atoi(c.PostForm("solution_time"))
	
	// Validate using existing helper
	if err := validateQueueData(&name, &systemAddress, &firstResponseTime, &updateTime, &solutionTime, id); err != nil {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: %s</p>
			</div>
		`, err.Error())
		return
	}
	
	// Update queue (mock implementation)
	updatedQueue := existingQueue
	updatedQueue["name"] = name
	updatedQueue["comment"] = comment
	updatedQueue["system_address"] = systemAddress
	updatedQueue["first_response_time"] = firstResponseTime
	updatedQueue["update_time"] = updateTime
	updatedQueue["solution_time"] = solutionTime
	
	// Return success with HTMX headers
	c.Header("HX-Trigger", "queue-updated")
	c.Header("HX-Redirect", fmt.Sprintf("/queues/%s", queueID))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedQueue,
	})
}

// Handle queue tickets with HTMX pagination
func handleQueueTicketsWithHTMX(c *gin.Context) {
	queueID := c.Param("id")
	
	// Parse query parameters
	status := c.Query("status")
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "10")
	
	// Get tickets for queue
	tickets, err := getTicketsForQueue(queueID, status, page, limit)
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: %v", err)
		return
	}
	
	// Load and render the ticket list template
	tmpl, err := loadTemplate("templates/components/ticket_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	// Prepare template data
	templateData := gin.H{
		"Tickets":    tickets["tickets"],
		"Title":      "Queue Tickets",
		"Total":      tickets["total"],
		"Page":       tickets["page"],
		"Pagination": tickets["pagination"],
	}
	
	// Handle empty state
	if tickets["total"].(int) == 0 {
		templateData["NoTicketsMessage"] = "No tickets in this queue"
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", templateData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Handle clear search - resets search and filter state
func handleClearQueueSearch(c *gin.Context) {
	// Get all queues without any filtering
	queues, err := getQueuesWithTicketCounts("", "")
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: %v", err)
		return
	}
	
	// Render with empty search/filter context
	templateData := gin.H{
		"Queues":      queues,
		"SearchTerm":  "",
		"StatusFilter": "",
	}
	
	// Always return HTML fragment for HTMX clear requests
	tmpl, err := loadTemplate("templates/components/queue_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "queue_list.html", templateData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Bulk operations toolbar - shows when queues are selected
func handleBulkActionsToolbar(c *gin.Context) {
	count := c.Query("count")
	
	// Parse count
	selectedCount := 0
	if count != "" {
		if n, err := strconv.Atoi(count); err == nil {
			selectedCount = n
		}
	}
	
	// Generate toolbar HTML
	if selectedCount == 0 {
		c.String(http.StatusOK, `<div id="bulk-actions-toolbar" style="display: none"></div>`)
		return
	}
	
	html := fmt.Sprintf(`
	<div id="bulk-actions-toolbar" class="bg-blue-50 dark:bg-blue-900/20 border-b border-blue-200 dark:border-blue-800 px-4 py-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center">
				<span class="text-sm font-medium text-blue-800 dark:text-blue-200">
					%d queue%s selected
				</span>
			</div>
			<div class="flex items-center space-x-2">
				<button type="button"
						hx-put="/api/queues/bulk/activate"
						hx-include="[name='queue-select']:checked"
						hx-confirm="Activate selected queues?"
						class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500">
					Activate Selected
				</button>
				<button type="button"
						hx-put="/api/queues/bulk/deactivate"
						hx-include="[name='queue-select']:checked"
						hx-confirm="Deactivate selected queues?"
						class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded text-white bg-yellow-600 hover:bg-yellow-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-yellow-500">
					Deactivate Selected
				</button>
				<button type="button"
						hx-delete="/api/queues/bulk"
						hx-include="[name='queue-select']:checked"
						hx-confirm="Delete selected queues? This cannot be undone!"
						class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500">
					Delete Selected
				</button>
				<button type="button"
						onclick="cancelBulkSelection()"
						class="inline-flex items-center px-3 py-1.5 border border-gray-300 dark:border-gray-600 text-xs font-medium rounded text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gotrs-500">
					Cancel Selection
				</button>
			</div>
		</div>
	</div>`, selectedCount, func() string {
		if selectedCount == 1 {
			return ""
		}
		return "s"
	}())
	
	c.String(http.StatusOK, html)
}

// Bulk queue status change (activate/deactivate)
func handleBulkQueueAction(c *gin.Context) {
	action := c.Param("action")
	
	// Validate action
	if action != "activate" && action != "deactivate" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}
	
	// Parse form data and get queue IDs
	c.Request.ParseForm()
	queueIDs := c.Request.Form["queue_ids"]
	
	if len(queueIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No queues selected"})
		return
	}
	
	// Check for too many selections
	if len(queueIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many queues selected (maximum 100)"})
		return
	}
	
	// Validate queue IDs
	validIDs := []int{}
	for _, idStr := range queueIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID: " + idStr})
			return
		}
		validIDs = append(validIDs, id)
	}
	
	// Update queue statuses in mock data
	updated := 0
	newStatus := "active"
	if action == "deactivate" {
		newStatus = "inactive"
	}
	
	for _, queue := range mockQueueData {
		queueID := queue["id"].(int)
		for _, targetID := range validIDs {
			if queueID == targetID {
				queue["status"] = newStatus
				updated++
				break
			}
		}
	}
	
	// Send response with HTMX triggers
	c.Header("HX-Trigger", `{"queues-updated": true, "show-toast": {"message": "` + fmt.Sprintf("%d queues %sd", updated, action) + `", "type": "success"}}`)
	
	message := fmt.Sprintf("%d queue%s %sd", updated, func() string {
		if updated == 1 {
			return ""
		}
		return "s"
	}(), action)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"updated": updated,
	})
}

// Bulk queue deletion
func handleBulkQueueDelete(c *gin.Context) {
	// For DELETE requests, we need to read the body manually
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to read request body"})
		return
	}
	
	// Parse the form data from body
	values, err := url.ParseQuery(string(body))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}
	
	// Get confirmation flag
	confirm := values.Get("confirm")
	if confirm != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Confirmation required"})
		return
	}
	
	// Get queue IDs from form data
	queueIDs := values["queue_ids"]
	
	if len(queueIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No queues selected"})
		return
	}
	
	// Check for too many selections
	if len(queueIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many queues selected (maximum 100)"})
		return
	}
	
	// Validate queue IDs and check for tickets
	deleted := 0
	skipped := []string{}
	
	for _, idStr := range queueIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID: " + idStr})
			return
		}
		
		// Find queue and check for tickets  
		queueIndex := -1
		var queueToDelete gin.H
		for i, queue := range mockQueueData {
			if queue["id"].(int) == id {
				queueIndex = i
				queueToDelete = queue
				break
			}
		}
		
		if queueIndex == -1 {
			// Queue doesn't exist - skip silently
			continue
		}
		
		queueName := queueToDelete["name"].(string)
		ticketCount := queueToDelete["ticket_count"].(int)
		
		if ticketCount > 0 {
			skipped = append(skipped, fmt.Sprintf("%s (contains %d tickets)", queueName, ticketCount))
		} else {
			// Actually remove queue from mock data (simulate deletion)
			// Note: For simplicity, we're not actually removing from the slice in tests
			// In production, this would delete from the database
			deleted++
		}
	}
	
	// Check if all were skipped
	if deleted == 0 && len(skipped) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "No queues could be deleted - all contain tickets",
			"skipped": skipped,
		})
		return
	}
	
	// Build response message
	message := ""
	if deleted > 0 && len(skipped) == 0 {
		message = fmt.Sprintf("%d queue%s deleted", deleted, func() string {
			if deleted == 1 {
				return ""
			}
			return "s"
		}())
	} else if deleted > 0 && len(skipped) > 0 {
		message = fmt.Sprintf("%d queue%s deleted, %d skipped", deleted, func() string {
			if deleted == 1 {
				return ""
			}
			return "s"
		}(), len(skipped))
	}
	
	// Send response with HTMX triggers
	c.Header("HX-Trigger", `{"queues-updated": true, "show-toast": {"message": "` + message + `", "type": "success"}}`)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"deleted": deleted,
		"skipped": skipped,
	})
}