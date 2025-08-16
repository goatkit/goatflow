package api

import (
	"html/template"
	"net/http"
	"os"
	"strings"
	"github.com/gin-gonic/gin"
)

// SetupHTMXRoutes configures routes for HTMX-based UI
func SetupHTMXRoutes(r *gin.Engine) {
	// Serve static files
	r.Static("/static", "./static")
	
	// Set up template functions
	r.SetFuncMap(template.FuncMap{
		"firstLetter": func(s string) string {
			if len(s) > 0 {
				return strings.ToUpper(string(s[0]))
			}
			return ""
		},
	})
	
	// Template routes
	r.LoadHTMLGlob("templates/**/*.html")
	
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
		
		// Ticket operations
		api.GET("/tickets", handleTicketsAPI)
		api.GET("/tickets/search", handleTicketSearch)
		api.POST("/tickets", handleCreateTicket)
		api.PUT("/tickets/:id", handleUpdateTicket)
		
		// Real-time updates
		api.GET("/tickets/stream", handleTicketStream)
		api.GET("/dashboard/activity-stream", handleActivityStream)
	}
}

// Login page
func handleLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title": "GOTRS - Sign In",
	})
}

// Register page
func handleRegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "pages/register.html", gin.H{
		"Title": "GOTRS - Register",
	})
}

// Dashboard
func handleDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Title": "Dashboard - GOTRS",
		"User":  gin.H{"FirstName": "Demo", "LastName": "User"},
	})
}

// Tickets list page
func handleTicketsList(c *gin.Context) {
	c.HTML(http.StatusOK, "pages/tickets/list.html", gin.H{
		"Title":  "Tickets - GOTRS",
		"Queues": []gin.H{}, // TODO: Load from database
	})
}

// New ticket page
func handleNewTicket(c *gin.Context) {
	c.HTML(http.StatusOK, "pages/tickets/new.html", gin.H{
		"Title": "New Ticket - GOTRS",
	})
}

// Ticket detail page
func handleTicketDetail(c *gin.Context) {
	ticketID := c.Param("id")
	c.HTML(http.StatusOK, "pages/tickets/detail.html", gin.H{
		"Title":    "Ticket #" + ticketID + " - GOTRS",
		"TicketID": ticketID,
	})
}

// HTMX Login handler
func handleHTMXLogin(c *gin.Context) {
	var loginReq struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
	
	c.HTML(http.StatusOK, "components/dashboard_stats.html", gin.H{
		"Stats": stats,
	})
}

// Recent tickets (returns HTML fragment)
func handleRecentTickets(c *gin.Context) {
	tickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "created": "2 hours ago"},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "created": "4 hours ago"},
		{"id": 3, "number": "TICKET-003", "title": "Bug report", "status": "pending", "priority": "low", "created": "1 day ago"},
	}
	
	c.HTML(http.StatusOK, "components/recent_tickets.html", gin.H{
		"Tickets": tickets,
	})
}

// Activity feed (returns HTML fragment)
func handleActivityFeed(c *gin.Context) {
	activities := []gin.H{
		{"user": "John Doe", "action": "created", "target": "TICKET-001", "time": "2 minutes ago"},
		{"user": "Jane Smith", "action": "updated", "target": "TICKET-002", "time": "15 minutes ago"},
		{"user": "Bob Wilson", "action": "closed", "target": "TICKET-003", "time": "1 hour ago"},
	}
	
	c.HTML(http.StatusOK, "components/activity_feed.html", gin.H{
		"Activities": activities,
	})
}

// Tickets API (returns HTML fragment)
func handleTicketsAPI(c *gin.Context) {
	// TODO: Parse query parameters and filter
	// status := c.Query("status")
	// priority := c.Query("priority")
	// queue := c.Query("queue")
	
	// TODO: Query database with filters
	tickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "customer": "john@example.com", "agent": "Agent Smith"},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "customer": "jane@example.com", "agent": ""},
	}
	
	c.HTML(http.StatusOK, "components/ticket_list.html", gin.H{
		"Tickets": tickets,
	})
}

// Ticket search (returns HTML fragment)
func handleTicketSearch(c *gin.Context) {
	query := c.Query("q")
	
	// TODO: Implement search with Zinc
	tickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues matching: " + query, "status": "open"},
	}
	
	c.HTML(http.StatusOK, "components/ticket_list.html", gin.H{
		"Tickets": tickets,
	})
}

// Create ticket (HTMX)
func handleCreateTicket(c *gin.Context) {
	// TODO: Implement ticket creation
	c.JSON(http.StatusCreated, gin.H{"message": "Ticket created", "id": 123})
}

// Update ticket (HTMX)
func handleUpdateTicket(c *gin.Context) {
	// TODO: Implement ticket update
	c.JSON(http.StatusOK, gin.H{"message": "Ticket updated"})
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
	
	// Send a test event
	c.SSEvent("activity", gin.H{
		"user":   "System",
		"action": "demo event",
		"time":   "now",
	})
	
	c.Writer.Flush()
}