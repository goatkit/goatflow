package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/ldap"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/xeonx/timeago"
	"golang.org/x/crypto/bcrypt"
)

// hashPasswordSHA256 hashes a password using SHA256 (compatible with OTRS)
func hashPasswordSHA256(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// generateSalt generates a random salt for password hashing
func generateSalt() string {
	// Generate 16 random bytes
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		// Fallback to timestamp-based salt if crypto/rand fails
		data := fmt.Sprintf("%d", time.Now().UnixNano())
		hash := sha256.Sum256([]byte(data))
		return hex.EncodeToString(hash[:16])
	}
	return hex.EncodeToString(salt)
}

// verifyPassword checks if a password matches a hashed password (with or without salt)
func verifyPassword(password, hashedPassword string) bool {
	// Check if it's a bcrypt hash (starts with $2a$, $2b$, or $2y$)
	if strings.HasPrefix(hashedPassword, "$2a$") || strings.HasPrefix(hashedPassword, "$2b$") || strings.HasPrefix(hashedPassword, "$2y$") {
		// Use bcrypt to compare
		err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
		return err == nil
	}

	// Check if it's a salted SHA256 hash (format: sha256$salt$hash)
	parts := strings.Split(hashedPassword, "$")
	if len(parts) == 3 && parts[0] == "sha256" {
		// Extract salt and hash
		salt := parts[1]
		expectedHash := parts[2]

		// Hash the password with the salt
		combined := password + salt
		hash := sha256.Sum256([]byte(combined))
		actualHash := hex.EncodeToString(hash[:])

		return actualHash == expectedHash
	}

	// Otherwise, treat as unsalted SHA256 hash (legacy)
	return hashPasswordSHA256(password) == hashedPassword
}

// Global variable to store pongo2 renderer
var pongo2Renderer *Pongo2Renderer

// Pongo2Renderer is a custom gin HTML renderer using pongo2
type Pongo2Renderer struct {
	templateSet *pongo2.TemplateSet
}

// HTML implements gin's HTMLRender interface
func (r *Pongo2Renderer) HTML(c *gin.Context, code int, name string, data interface{}) {
	// Convert gin.H to pongo2.Context
	var ctx pongo2.Context
	switch v := data.(type) {
	case pongo2.Context:
		ctx = v
	case gin.H:
		ctx = pongo2.Context(v)
	default:
		ctx = pongo2.Context{"data": data}
	}

	// Get the template
	tmpl, err := r.templateSet.FromFile(name)
	if err != nil {
		// Log the error and send a 500 response
		log.Printf("Template error for %s: %v", name, err)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	// Render the template
	output, err := tmpl.Execute(ctx)
	if err != nil {
		// Log the error and send a 500 response
		log.Printf("Template execution error for %s: %v", name, err)
		c.String(http.StatusInternalServerError, "Template execution error: %v", err)
		return
	}

	c.Data(code, "text/html; charset=utf-8", []byte(output))
}

// NewPongo2Renderer creates a new Pongo2Renderer with the given template directory
func NewPongo2Renderer(templateDir string) *Pongo2Renderer {
	loader := pongo2.MustNewLocalFileSystemLoader(templateDir)
	templateSet := pongo2.NewSet("gotrs", loader)
	templateSet.Debug = gin.IsDebugging()

	// Register custom filters
	templateSet.Globals["default"] = func(value interface{}, defaultValue interface{}) interface{} {
		if value == nil || value == "" {
			return defaultValue
		}
		return value
	}

	// Add a translation function stub
	templateSet.Globals["t"] = func(key string) string {
		// For now, just return the key
		// Later this can be replaced with actual translation
		translations := map[string]string{
			"app.name":                      "GOTRS",
			"app.description":               "GOTRS - Open Ticket Request System",
			"app.tagline":                   "Open Ticket Request System",
			"navigation.dashboard":          "Dashboard",
			"navigation.tickets":            "Tickets",
			"navigation.profile":            "Profile",
			"navigation.settings":           "Settings",
			"nav.dashboard":                 "Dashboard",
			"nav.tickets":                   "Tickets",
			"nav.queues":                    "Queues",
			"nav.profile":                   "Profile",
			"nav.settings":                  "Settings",
			"auth.login":                    "Login",
			"auth.logout":                   "Logout",
			"auth.signup":                   "Sign Up",
			"auth.signin":                   "Sign In",
			"auth.not_member":               "Not a member?",
			"auth.forgot_password":          "Forgot your password?",
			"auth.password":                 "Password",
			"auth.username":                 "Username",
			"auth.username_placeholder":     "Enter your username",
			"auth.username_tooltip":         "Your login username",
			"auth.email_placeholder":        "Enter your email address",
			"auth.email_tooltip":            "Your registered email address",
			"auth.password_placeholder":     "Enter your password",
			"auth.password_tooltip":         "Your account password",
			"auth.or":                       "Or",
			"user.email":                    "Email Address",
			"admin.dashboard":               "Admin Dashboard",
			"admin.dashboard_description":   "System administration and configuration",
			"admin.total_users":             "Total Users",
			"admin.total_groups":            "Total Groups",
			"admin.active_tickets":          "Active Tickets",
			"admin.total_queues":            "Total Queues",
			"admin.system_health":           "System Health",
			"admin.healthy":                 "Healthy",
			"admin.user_management":         "User Management",
			"admin.user_management_desc":    "Manage agents and customer users",
			"admin.group_management":        "Group Management",
			"admin.group_management_desc":   "Manage user groups and permissions",
			"admin.system_settings":         "System Settings",
			"admin.system_settings_desc":    "Configure system preferences",
			"admin.lookups":                 "Lookups",
			"admin.lookups_desc":            "Manage system lookups and configurations",
			"admin.email_templates":         "Email Templates",
			"admin.email_templates_desc":    "Customize email notifications",
			"admin.reports":                 "Reports",
			"admin.reports_desc":            "View system reports and analytics",
			"admin.backup_restore":          "Backup & Restore",
			"admin.backup_restore_desc":     "Manage system backups",
			"admin.users":                   "Users",
			"admin.groups":                  "Groups",
			"admin.permissions":             "Permissions",
			"admin.queues":                  "Queues",
			"admin.priorities":              "Priorities",
			"admin.states":                  "States",
			"admin.types":                   "Types",
			"admin.add_priority":            "Add Priority",
			"admin.add_state":               "Add State",
			"admin.add_type":                "Add Type",
			"admin.customer_users":          "Customer Users",
			"admin.customer_users_desc":     "Manage customer accounts and access",
			"admin.customer_companies":      "Customer Companies",
			"admin.customer_companies_desc": "Manage customer company information",
			"admin.add_company":             "Add Company",
			"admin.import":                  "Import",
			"admin.add_user":                "Add User",
			"admin.add_group":               "Add Group",
			"admin.add_customer_user":       "Add Customer",
			"admin.edit_user":               "Edit User",
			"admin.edit_group":              "Edit Group",
			"admin.title":                   "Title",
			"admin.login":                   "Login",
			"admin.first_name":              "First Name",
			"admin.last_name":               "Last Name",
			"admin.password":                "Password",
			"admin.leave_blank_to_keep":     "leave blank to keep current",
			"admin.status":                  "Status",
			"admin.active":                  "Active",
			"admin.inactive":                "Inactive",
			"admin.save":                    "Save",
			"admin.cancel":                  "Cancel",
			"admin.never":                   "Never",
			"admin.add_user_tooltip":        "Add a new user to the system",
			"admin.clear_search":            "Clear search",
			"admin.edit_user_tooltip":       "Edit user details",
			"admin.deactivate_user":         "Deactivate user",
			"admin.activate_user":           "Activate user",
			"admin.reset_password":          "Reset user password",
			"admin.delete_user":             "Delete user",
			"admin.title_mr":                "Mr.",
			"admin.title_ms":                "Ms.",
			"admin.title_mrs":               "Mrs.",
			"admin.title_dr":                "Dr.",
			"admin.users_description":       "Manage system users and permissions",
			"admin.groups_description":      "Manage user groups and access control",
			"admin.lookups.description":     "Manage system lookups and configurations",
			"dashboard.welcome_back":        "Welcome back",
			"tickets.new_ticket":            "New Ticket",
			"tickets.title":                 "Tickets",
			"tickets.overdue":               "Overdue",
			"dashboard.recent_tickets":      "Recent Tickets",
			"status.open":                   "Open",
			"status.new":                    "New",
			"status.pending":                "Pending",
			"status.closed":                 "Closed",
			"time.today":                    "Today",
			"priority.high":                 "High",
			"priority.medium":               "Medium",
			"priority.low":                  "Low",
			"priority.critical":             "Critical",
			"queues.queue_status":           "Queue Status",
			"common.view_all":               "View All",
			"common.actions":                "Actions",
			"common.edit":                   "Edit",
			"common.delete":                 "Delete",
			"common.save":                   "Save",
			"common.cancel":                 "Cancel",
			"common.search":                 "Search",
			"common.filter":                 "Filter",
			"common.clear":                  "Clear",
			"common.add":                    "Add",
			"common.new":                    "New",
			"common.status":                 "Status",
			"common.active":                 "Active",
			"common.inactive":               "Inactive",
			"common.name":                   "Name",
			"common.email":                  "Email",
			"common.description":            "Description",
			"common.created":                "Created",
			"common.updated":                "Updated",
			"common.id":                     "ID",
			"common.type":                   "Type",
			"common.color":                  "Color",
			"common.comment":                "Comment",
			"common.street":                 "Street",
			"common.zip":                    "ZIP Code",
			"common.city":                   "City",
			"common.country":                "Country",
			"common.url":                    "Website",
			"common.comments":               "Comments",
		}
		if val, ok := translations[key]; ok {
			return val
		}
		return key
	}

	return &Pongo2Renderer{
		templateSet: templateSet,
	}
}

// getUserFromContext safely extracts user from Gin context
func getUserMapForTemplate(c *gin.Context) gin.H {
	// First try to get the user object
	if userCtx, ok := c.Get("user"); ok {
		// Convert the user object to gin.H for template usage
		if user, ok := userCtx.(*models.User); ok {
			isAdmin := user.ID == 1 || strings.Contains(strings.ToLower(user.Login), "admin")
			return gin.H{
				"ID":        user.ID,
				"Login":     user.Login,
				"FirstName": user.FirstName,
				"LastName":  user.LastName,
				"Email":     user.Email,
				"IsActive":  user.ValidID == 1,
				"IsAdmin":   isAdmin,
				"Role":      map[bool]string{true: "Admin", false: "Agent"}[isAdmin],
			}
		}
		// If it's already gin.H, return as is
		if userH, ok := userCtx.(gin.H); ok {
			return userH
		}
	}

	// Try to construct from middleware-set values
	if userID, ok := c.Get("user_id"); ok {
		userEmail, _ := c.Get("user_email")
		userRole, _ := c.Get("user_role")

		// Try to load user details from database
		firstName := ""
		lastName := ""
		login := fmt.Sprintf("%v", userEmail)
		isInAdminGroup := false

		// Get database connection and load user details
		if db, err := database.GetDB(); err == nil {
			var dbFirstName, dbLastName, dbLogin sql.NullString
			userIDVal := uint(0)

			// Convert userID to uint
			switch v := userID.(type) {
			case uint:
				userIDVal = v
			case int:
				userIDVal = uint(v)
			case float64:
				userIDVal = uint(v)
			}

			if userIDVal > 0 {
				err := db.QueryRow(database.ConvertPlaceholders(`
					SELECT login, first_name, last_name
					FROM users
					WHERE id = $1`),
					userIDVal).Scan(&dbLogin, &dbFirstName, &dbLastName)

				if err == nil {
					if dbFirstName.Valid {
						firstName = dbFirstName.String
					}
					if dbLastName.Valid {
						lastName = dbLastName.String
					}
					if dbLogin.Valid {
						login = dbLogin.String
					}
				}

				// Check if user is in admin group for Dev menu access
				var count int
				err = db.QueryRow(database.ConvertPlaceholders(`
					SELECT COUNT(*)
					FROM group_user ug
					JOIN groups g ON ug.group_id = g.id
					WHERE ug.user_id = $1 AND g.name = 'admin'`),
					userIDVal).Scan(&count)
				if err == nil && count > 0 {
					isInAdminGroup = true
				}
			}
		}

		// If we still don't have names, try to parse from userName
		if firstName == "" && lastName == "" {
			userName, _ := c.Get("user_name")
			nameParts := strings.Fields(fmt.Sprintf("%v", userName))
			if len(nameParts) > 0 {
				firstName = nameParts[0]
			}
			if len(nameParts) > 1 {
				lastName = strings.Join(nameParts[1:], " ")
			}
		}

		isAdmin := userRole == "Admin"

		return gin.H{
			"ID":             userID,
			"Login":          login,
			"FirstName":      firstName,
			"LastName":       lastName,
			"Email":          fmt.Sprintf("%v", userEmail),
			"IsActive":       true,
			"IsAdmin":        isAdmin,
			"IsInAdminGroup": isInAdminGroup,
			"Role":           fmt.Sprintf("%v", userRole),
		}
	}

	// Return a default/guest user structure
	return gin.H{
		"ID":        0,
		"Login":     "guest",
		"FirstName": "",
		"LastName":  "",
		"Email":     "",
		"IsActive":  false,
		"IsAdmin":   false,
		"Role":      "Guest",
	}
}

// sendErrorResponse sends a JSON error response for HTMX/API requests
func sendErrorResponse(c *gin.Context, statusCode int, message string) {
	// Check if this is an API request that expects JSON
	if strings.Contains(c.GetHeader("Accept"), "application/json") ||
		strings.HasPrefix(c.Request.URL.Path, "/api/") ||
		c.GetHeader("HX-Request") == "true" {
		c.JSON(statusCode, gin.H{
			"success": false,
			"error":   message,
		})
		return
	}

	// For regular page requests, render an error page
	if pongo2Renderer != nil {
		pongo2Renderer.HTML(c, statusCode, "pages/error.pongo2", pongo2.Context{
			"StatusCode": statusCode,
			"Message":    message,
			"User":       getUserMapForTemplate(c),
		})
	} else {
		// Fallback to plain text if template renderer is not available
		c.String(statusCode, "Error: %s", message)
	}
}

// checkAdmin middleware ensures the user is an admin
func checkAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := getUserMapForTemplate(c)

		// Check if user is admin based on ID or login
		if userID, ok := user["ID"].(uint); ok {
			if userID == 1 || userID == 2 { // User ID 1 and 2 are admins
				c.Next()
				return
			}

			// Check if user is in admin group
			db, err := database.GetDB()
			if err == nil {
				var count int
				err = db.QueryRow(database.ConvertPlaceholders(`
					SELECT COUNT(*)
					FROM group_user ug
					JOIN groups g ON ug.group_id = g.id
					WHERE ug.user_id = $1 AND g.name = 'admin'`),
					userID).Scan(&count)
				if err == nil && count > 0 {
					c.Next()
					return
				}
			}
		}

		if login, ok := user["Login"].(string); ok {
			if strings.Contains(strings.ToLower(login), "admin") || login == "root@localhost" {
				c.Next()
				return
			}
		}

		// Not an admin
		sendErrorResponse(c, http.StatusForbidden, "Access denied. Admin privileges required.")
		c.Abort()
	}
}

// routeExists checks if a route already exists in the router
func routeExists(r *gin.Engine, method string, path string) bool {
	routes := r.Routes()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

// safeRegisterRoute registers a route only if it doesn't already exist
func safeRegisterRoute(r *gin.Engine, group *gin.RouterGroup, method string, path string, handlers ...gin.HandlerFunc) bool {
	// Calculate full path
	fullPath := group.BasePath() + path

	// Check if route already exists
	if routeExists(r, method, fullPath) {
		log.Printf("WARNING: Route already exists: %s %s - skipping registration", method, fullPath)
		return false
	}

	// Register the route with panic recovery
	defer func() {
		if err := recover(); err != nil {
			log.Printf("ERROR: Failed to register route %s %s: %v", method, fullPath, err)
		}
	}()

	switch method {
	case "GET":
		group.GET(path, handlers...)
	case "POST":
		group.POST(path, handlers...)
	case "PUT":
		group.PUT(path, handlers...)
	case "DELETE":
		group.DELETE(path, handlers...)
	case "PATCH":
		group.PATCH(path, handlers...)
	default:
		log.Printf("WARNING: Unknown HTTP method: %s", method)
		return false
	}

	log.Printf("Successfully registered route: %s %s", method, fullPath)
	return true
}

// SetupHTMXRoutes sets up all HTMX routes on the given router
func SetupHTMXRoutes(r *gin.Engine) {
	// For testing or when called without auth services
	setupHTMXRoutesWithAuth(r, nil, nil, nil)
}

// NewHTMXRouter creates all routes for the HTMX UI
func NewHTMXRouter(jwtManager *auth.JWTManager, ldapProvider *ldap.Provider) *gin.Engine {
	r := gin.Default()
	setupHTMXRoutesWithAuth(r, jwtManager, ldapProvider, nil)
	return r
}

// setupHTMXRoutesWithAuth sets up all routes with optional authentication
func setupHTMXRoutesWithAuth(r *gin.Engine, jwtManager *auth.JWTManager, ldapProvider *ldap.Provider, i18nSvc interface{}) {

	// Initialize pongo2 renderer
	templateDir := "./templates"
	pongo2Renderer = NewPongo2Renderer(templateDir)

	// Serve static files
	r.Static("/static", "./static")
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.StaticFile("/favicon.svg", "./static/favicon.svg")

	// Health check endpoint - comprehensive check
	r.GET("/health", func(c *gin.Context) {
		health := gin.H{
			"status": "healthy",
			"checks": gin.H{},
		}

		// Check template rendering
		if pongo2Renderer != nil && pongo2Renderer.templateSet != nil {
			// Try to load a basic template
			if _, err := pongo2Renderer.templateSet.FromFile("layouts/base.pongo2"); err != nil {
				health["status"] = "unhealthy"
				health["checks"].(gin.H)["templates"] = gin.H{
					"status": "unhealthy",
					"error":  err.Error(),
				}
			} else {
				health["checks"].(gin.H)["templates"] = gin.H{
					"status": "healthy",
				}
			}
		} else {
			health["status"] = "unhealthy"
			health["checks"].(gin.H)["templates"] = gin.H{
				"status": "unhealthy",
				"error":  "Template renderer not initialized",
			}
		}

		// Return appropriate status code
		statusCode := http.StatusOK
		if health["status"] == "unhealthy" {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, health)
	})

	// Root redirect based on authentication
	r.GET("/", func(c *gin.Context) {
		// Try to validate existing session
		if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
			// Try to validate the token
			if jwtManager != nil {
				if _, err := jwtManager.ValidateToken(cookie); err == nil {
					// Valid session, redirect to dashboard
					c.Redirect(http.StatusFound, "/dashboard")
					return
				}
			} else {
				// No JWT manager, check for demo session
				if cookie == "demo_session_"+fmt.Sprint(time.Now().Unix()/86400*86400) ||
					strings.HasPrefix(cookie, "demo_session_") {
					c.Redirect(http.StatusFound, "/dashboard")
					return
				}
			}
		}
		// No valid session, redirect to login
		c.Redirect(http.StatusFound, "/login")
	})

	// Public routes (no auth required)
	// Commented out - now handled by YAML routes
	r.GET("/login", handleLoginPage)
	r.POST("/login", handleLogin(jwtManager))
	r.GET("/logout", handleLogout)
	r.POST("/logout", handleLogout)

	// Protected routes - require authentication
	protected := r.Group("")
	if jwtManager != nil {
		authMiddleware := middleware.NewAuthMiddleware(jwtManager)
		protected.Use(authMiddleware.RequireAuth())
	} else {
		// Development mode - use session middleware
		protected.Use(middleware.SessionMiddleware(jwtManager))
	}

	// Dashboard and main pages
	// Restored - YAML routes not working properly yet
	protected.GET("/dashboard", handleDashboard)
	protected.GET("/tickets", handleTickets)
	protected.GET("/ticket/new", handleNewTicket)
	protected.GET("/tickets/new", handleNewTicket)           // Plural URL pattern
	protected.GET("/claude-chat-demo", handleClaudeChatDemo) // Claude chat demo page

	// Agent ticket routes - for full functionality including S/MIME articles
	protected.GET("/agent/tickets", HandleAgentTickets)
	// Note: Individual ticket view now uses /ticket/:id route for consistency

	// WebSocket for real-time chat
	protected.GET("/ws/chat", HandleWebSocketChat)
	protected.GET("/ticket/:id", handleTicketDetail)
	protected.GET("/queues", handleQueues)
	protected.GET("/queues/:id", handleQueueDetail)
	// Disabled - using YAML redirect route instead
	// protected.GET("/profile", handleProfile)
	protected.GET("/settings", handleSettings)

	// API routes for user preferences
	protected.GET("/api/preferences/session-timeout", HandleGetSessionTimeout)
	protected.POST("/api/preferences/session-timeout", HandleSetSessionTimeout)

	// Developer routes - for Claude's development tools
	devRoutes := protected.Group("/dev")
	devRoutes.Use(checkAdmin()) // For now, require admin access
	{
		RegisterDevRoutes(devRoutes)
	}

	// Admin routes group - require admin privileges
	adminRoutes := protected.Group("/admin")
	adminRoutes.Use(checkAdmin())
	{
		// Admin dashboard and main sections
		adminRoutes.GET("", handleAdminDashboard)
		adminRoutes.GET("/dashboard", handleAdminDashboard)
		// Users now uses the dynamic module system
		adminRoutes.GET("/users", func(c *gin.Context) {
			c.Params = append(c.Params, gin.Param{Key: "module", Value: "users"})
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.GET("/queues", handleAdminQueues)
		adminRoutes.GET("/priorities", handleAdminPriorities)
		adminRoutes.GET("/lookups", handleAdminLookups)
		adminRoutes.GET("/roadmap", handleAdminRoadmap)
		adminRoutes.GET("/schema-discovery", handleSchemaDiscovery)
		adminRoutes.GET("/schema-monitoring", handleSchemaMonitoring)

		// User management routes - now handled by dynamic module
		adminRoutes.GET("/users/new", func(c *gin.Context) {
			c.Params = []gin.Param{{Key: "module", Value: "users"}, {Key: "id", Value: "new"}}
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.POST("/users", func(c *gin.Context) {
			c.Params = append(c.Params, gin.Param{Key: "module", Value: "users"})
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.GET("/users/:id", func(c *gin.Context) {
			id := c.Param("id")
			c.Params = []gin.Param{{Key: "module", Value: "users"}, {Key: "id", Value: id}}
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.GET("/users/:id/edit", func(c *gin.Context) {
			id := c.Param("id")
			c.Params = []gin.Param{{Key: "module", Value: "users"}, {Key: "id", Value: id}, {Key: "action", Value: "edit"}}
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.PUT("/users/:id", func(c *gin.Context) {
			id := c.Param("id")
			c.Params = []gin.Param{{Key: "module", Value: "users"}, {Key: "id", Value: id}}
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.DELETE("/users/:id", func(c *gin.Context) {
			id := c.Param("id")
			c.Params = []gin.Param{{Key: "module", Value: "users"}, {Key: "id", Value: id}}
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.PUT("/users/:id/status", func(c *gin.Context) {
			id := c.Param("id")
			c.Params = []gin.Param{{Key: "module", Value: "users"}, {Key: "id", Value: id}, {Key: "action", Value: "status"}}
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})
		adminRoutes.POST("/users/:id/reset-password", func(c *gin.Context) {
			id := c.Param("id")
			c.Params = []gin.Param{{Key: "module", Value: "users"}, {Key: "id", Value: id}, {Key: "action", Value: "reset-password"}}
			if dynamicHandler != nil {
				dynamicHandler.ServeModule(c)
			} else {
				c.JSON(500, gin.H{"error": "Dynamic module system not initialized"})
			}
		})

		// Queue management routes (disabled - handlers not implemented)
		// adminRoutes.GET("/queues/:id", handleGetQueue)
		// adminRoutes.POST("/queues", handleCreateQueue)
		// adminRoutes.PUT("/queues/:id", handleUpdateQueue)
		// adminRoutes.DELETE("/queues/:id", handleDeleteQueue)

		// Priority management routes (disabled - handlers not implemented)
		// adminRoutes.GET("/priorities/:id", handleGetPriority)
		// adminRoutes.POST("/priorities", handleCreatePriority)
		// adminRoutes.PUT("/priorities/:id", handleUpdatePriority)
		// adminRoutes.DELETE("/priorities/:id", handleDeletePriority)

		// State management routes (disabled - handlers not implemented)
		// adminRoutes.GET("/states", handleAdminStates)
		// adminRoutes.POST("/states/create", handleAdminStateCreate)
		// adminRoutes.POST("/states/:id/update", handleAdminStateUpdate)
		// adminRoutes.POST("/states/:id/delete", handleAdminStateDelete)
		// adminRoutes.GET("/states/types", handleGetStateTypes)

		// Type management routes (disabled - handlers not implemented)
		// adminRoutes.GET("/types", handleAdminTypes)
		// adminRoutes.POST("/types/create", handleAdminTypeCreate)
		// adminRoutes.POST("/types/:id/update", handleAdminTypeUpdate)
		// adminRoutes.POST("/types/:id/delete", handleAdminTypeDelete)

		// Permission management routes (OTRS Role equivalent)
		adminRoutes.GET("/permissions", handleAdminPermissions)
		adminRoutes.GET("/permissions/user/:userId", handleGetUserPermissionMatrix)
		adminRoutes.PUT("/permissions/user/:userId", handleUpdateUserPermissions)
		adminRoutes.POST("/permissions/user/:userId", handleUpdateUserPermissions) // HTML form support
		adminRoutes.GET("/permissions/group/:groupId", handleGetGroupPermissionMatrix)
		adminRoutes.POST("/permissions/clone", handleCloneUserPermissions)

		// Group Management (OTRS AdminGroup)
		adminRoutes.GET("/groups", handleAdminGroups)
		adminRoutes.GET("/groups/:id", handleGetGroup)
		adminRoutes.POST("/groups", handleCreateGroup)
		adminRoutes.PUT("/groups/:id", handleUpdateGroup)
		adminRoutes.DELETE("/groups/:id", handleDeleteGroup)
		adminRoutes.GET("/groups/:id/users", handleGetGroupUsers)
		adminRoutes.POST("/groups/:id/users", handleAddUserToGroup)
		adminRoutes.DELETE("/groups/:id/users/:userId", handleRemoveUserFromGroup)

		// Role Management (Higher level than groups)
		adminRoutes.GET("/roles", handleAdminRoles)
		adminRoutes.GET("/roles/:id", handleAdminRoleGet)
		adminRoutes.POST("/roles/create", handleAdminRoleCreate)
		adminRoutes.PUT("/roles/:id", handleAdminRoleUpdate)
		adminRoutes.DELETE("/roles/:id", handleAdminRoleDelete)
		adminRoutes.GET("/roles/:id/users", handleAdminRoleUsers)
		adminRoutes.POST("/roles/:id/users", handleAdminRoleUserAdd)
		adminRoutes.DELETE("/roles/:id/users/:userId", handleAdminRoleUserRemove)
		adminRoutes.GET("/roles/:id/permissions", handleAdminRolePermissions)
		adminRoutes.PUT("/roles/:id/permissions", handleAdminRolePermissions)

		// Customer management routes
		adminRoutes.GET("/customer-users", underConstruction("Customer Users"))
		adminRoutes.GET("/customer-companies", underConstruction("Customer Companies"))
		adminRoutes.GET("/customer-user-group", underConstruction("Customer User Groups"))
		adminRoutes.GET("/customers", underConstruction("Customer Management"))

		// Ticket configuration routes
		adminRoutes.GET("/states", handleAdminStates)
		adminRoutes.POST("/states/create", handleAdminStateCreate)
		adminRoutes.PUT("/states/:id/update", handleAdminStateUpdate)
		adminRoutes.DELETE("/states/:id/delete", handleAdminStateDelete)
		adminRoutes.GET("/states/types", handleGetStateTypes)

		adminRoutes.GET("/types", handleAdminTypes)
		adminRoutes.POST("/types/create", handleAdminTypeCreate)
		adminRoutes.POST("/types/:id/update", handleAdminTypeUpdate)
		adminRoutes.POST("/types/:id/delete", handleAdminTypeDelete)
		adminRoutes.GET("/services", handleAdminServices)
		adminRoutes.POST("/services/create", handleAdminServiceCreate)
		adminRoutes.PUT("/services/:id/update", handleAdminServiceUpdate)
		adminRoutes.DELETE("/services/:id/delete", handleAdminServiceDelete)
		adminRoutes.GET("/sla", handleAdminSLA)
		adminRoutes.POST("/sla/create", handleAdminSLACreate)
		adminRoutes.PUT("/sla/:id/update", handleAdminSLAUpdate)
		adminRoutes.DELETE("/sla/:id/delete", handleAdminSLADelete)

		// Attachment management
		adminRoutes.GET("/attachments", handleAdminAttachment)
		adminRoutes.POST("/attachments/create", handleAdminAttachmentCreate)
		adminRoutes.PUT("/attachments/:id/update", handleAdminAttachmentUpdate)
		adminRoutes.DELETE("/attachments/:id/delete", handleAdminAttachmentDelete)
		adminRoutes.GET("/attachments/:id/download", handleAdminAttachmentDownload)
		adminRoutes.PUT("/attachments/:id/toggle", handleAdminAttachmentToggle)

		// Communication templates
		adminRoutes.GET("/signatures", underConstruction("Email Signatures"))
		adminRoutes.GET("/salutations", underConstruction("Email Salutations"))
		adminRoutes.GET("/notifications", underConstruction("Notification Templates"))

		// System configuration
		adminRoutes.GET("/settings", underConstruction("System Settings"))
		adminRoutes.GET("/templates", underConstruction("Template Management"))
		adminRoutes.GET("/reports", underConstruction("Reports"))
		adminRoutes.GET("/backup", underConstruction("Backup & Restore"))

		// Dynamic Module System for side-by-side testing
		if db, err := database.GetDB(); err == nil {
			if err := SetupDynamicModules(adminRoutes, db); err != nil {
				log.Printf("WARNING: Failed to setup dynamic modules: %v", err)
			} else {
				log.Println("✅ Dynamic Module System integrated successfully")
			}
		} else {
			log.Printf("WARNING: Cannot setup dynamic modules without database: %v", err)
		}
	}

	// HTMX API endpoints (return HTML fragments)
	api := r.Group("/api")

	// Authentication endpoints (no auth required)
	{
		api.GET("/auth/login", handleHTMXLogin) // Also support GET for the form
		api.POST("/auth/login", handleLogin(jwtManager))
		api.POST("/auth/logout", handleHTMXLogout)
		api.GET("/auth/refresh", underConstructionAPI("/auth/refresh")) // GET for testing
		api.POST("/auth/refresh", underConstructionAPI("/auth/refresh"))
		api.GET("/auth/register", underConstructionAPI("/auth/register")) // GET for form
		api.POST("/auth/register", underConstructionAPI("/auth/register"))
	}

	// Get database connection for handlers that need it
	// db, _ := database.GetDB()

	// Protected API endpoints - require authentication
	protectedAPI := api.Group("")
	if jwtManager != nil {
		authMiddleware := middleware.NewAuthMiddleware(jwtManager)
		protectedAPI.Use(authMiddleware.RequireAuth())
	} else {
		// Development mode - use session middleware
		protectedAPI.Use(middleware.SessionMiddleware(jwtManager))
	}

	// Dashboard endpoints
	{
		protectedAPI.GET("/dashboard/stats", handleDashboardStats)
		protectedAPI.GET("/dashboard/recent-tickets", handleRecentTickets)
		protectedAPI.GET("/dashboard/notifications", handleNotifications)
		protectedAPI.GET("/dashboard/quick-actions", handleQuickActions)
		protectedAPI.GET("/dashboard/activity", handleActivity)
		protectedAPI.GET("/dashboard/performance", handlePerformance)
	}

	// Queue management endpoints
	{
		protectedAPI.GET("/queues/:id", HandleAPIQueueGet)
		protectedAPI.GET("/queues/:id/details", HandleAPIQueueDetails)
		protectedAPI.PUT("/queues/:id/status", HandleAPIQueueStatus)
	}

	// Agent Interface Routes
	agentRoutes := protected.Group("/agent")
	{
		// Get database connection for agent routes
		if db, err := database.GetDB(); err == nil {
			RegisterAgentRoutes(agentRoutes, db)
		}
	}

	// Customer Portal Routes
	customerRoutes := protected.Group("/customer")
	{
		// Get database connection for customer routes
		if db, err := database.GetDB(); err == nil {
			RegisterCustomerRoutes(customerRoutes, db)
		}
	}

	// Ticket endpoints
	{
		protectedAPI.GET("/tickets", handleAPITickets)
		protectedAPI.POST("/tickets", handleCreateTicket)
		protectedAPI.GET("/tickets/:id", handleGetTicket)
		protectedAPI.PUT("/tickets/:id", handleUpdateTicket)
		protectedAPI.DELETE("/tickets/:id", handleDeleteTicket)
		protectedAPI.POST("/tickets/:id/notes", handleAddTicketNote)
		protectedAPI.GET("/tickets/:id/history", handleGetTicketHistory)
		protectedAPI.GET("/tickets/:id/available-agents", handleGetAvailableAgents)
		protectedAPI.POST("/tickets/:id/assign", handleAssignTicket)
		protectedAPI.POST("/tickets/:id/close", handleCloseTicket)
		protectedAPI.POST("/tickets/:id/reopen", handleReopenTicket)
		protectedAPI.GET("/tickets/search", handleSearchTickets)
		protectedAPI.GET("/tickets/filter", handleFilterTickets)
		protectedAPI.POST("/tickets/:id/attachments", handleUploadAttachment)
		protectedAPI.GET("/tickets/:id/attachments/:attachment_id", handleDownloadAttachment)
		protectedAPI.GET("/tickets/:id/attachments/:attachment_id/thumbnail", handleGetThumbnail)
		protectedAPI.DELETE("/tickets/:id/attachments/:attachment_id", handleDeleteAttachment)
		protectedAPI.GET("/files/*path", handleServeFile)

		// Group management API endpoints
		protectedAPI.GET("/groups", handleGetGroups)
		protectedAPI.GET("/groups/:id/members", handleGetGroupMembers)
		protectedAPI.GET("/groups/:id", handleGetGroupAPI)

		// Ticket Advanced Search endpoints
		protectedAPI.GET("/tickets/advanced-search", handleAdvancedTicketSearch)
		protectedAPI.GET("/tickets/search/suggestions", handleSearchSuggestions)
		protectedAPI.GET("/tickets/search/export", handleExportSearchResults)
		protectedAPI.POST("/tickets/search/history", handleSaveSearchHistory)
		protectedAPI.GET("/tickets/search/history", handleGetSearchHistory)
		protectedAPI.DELETE("/tickets/search/history/:id", handleDeleteSearchHistory)
		protectedAPI.POST("/tickets/search/saved", handleCreateSavedSearch)
		protectedAPI.GET("/tickets/search/saved", handleGetSavedSearches)
		protectedAPI.GET("/tickets/search/saved/:id/execute", handleExecuteSavedSearch)
		protectedAPI.PUT("/tickets/search/saved/:id", handleUpdateSavedSearch)
		protectedAPI.DELETE("/tickets/search/saved/:id", handleDeleteSavedSearch)

		// Claude Code feedback endpoint
		protectedAPI.POST("/claude-feedback", handleClaudeFeedback)

		// Canned responses endpoints
		cannedResponseHandlers := NewCannedResponseHandlers()
		protectedAPI.GET("/canned-responses", cannedResponseHandlers.GetResponses)
		protectedAPI.GET("/canned-responses/quick", cannedResponseHandlers.GetQuickResponses)
		protectedAPI.GET("/canned-responses/popular", cannedResponseHandlers.GetPopularResponses)
		protectedAPI.GET("/canned-responses/categories", cannedResponseHandlers.GetCategories)
		protectedAPI.GET("/canned-responses/category/:category", cannedResponseHandlers.GetResponsesByCategory)
		protectedAPI.GET("/canned-responses/search", cannedResponseHandlers.SearchResponses)
		protectedAPI.GET("/canned-responses/user", cannedResponseHandlers.GetResponsesForUser)
		protectedAPI.GET("/canned-responses/:id", cannedResponseHandlers.GetResponseByID)

		// Ticket merge endpoints
		protectedAPI.POST("/tickets/:id/merge", handleMergeTickets)
		protectedAPI.POST("/tickets/:id/unmerge", handleUnmergeTicket)
		protectedAPI.GET("/tickets/:id/merge-history", handleGetMergeHistory)

		// Admin only canned response operations
		adminAPI := protectedAPI.Group("")
		adminAPI.Use(checkAdmin())
		{
			adminAPI.POST("/canned-responses", cannedResponseHandlers.CreateResponse)
			adminAPI.PUT("/canned-responses/:id", cannedResponseHandlers.UpdateResponse)
			adminAPI.DELETE("/canned-responses/:id", cannedResponseHandlers.DeleteResponse)
			adminAPI.POST("/canned-responses/apply", cannedResponseHandlers.ApplyResponse)
			adminAPI.GET("/canned-responses/export", cannedResponseHandlers.ExportResponses)
			adminAPI.POST("/canned-responses/import", cannedResponseHandlers.ImportResponses)
		}
	}

	// Lookup data endpoints (disabled - handlers in lookup_handlers.go.disabled)
	{
		// protectedAPI.GET("/lookups/queues", handleGetQueues)
		// protectedAPI.GET("/lookups/priorities", handleGetPriorities)
		// protectedAPI.GET("/lookups/types", handleGetTypes)
		// protectedAPI.GET("/lookups/statuses", handleGetStatuses)
		// protectedAPI.GET("/lookups/form-data", handleGetFormData)
		// protectedAPI.GET("/lookups/audit", handleGetAuditLogs)
		// protectedAPI.GET("/lookups/export", handleExportConfiguration)
		// protectedAPI.POST("/lookups/import", handleImportConfiguration)

		// State CRUD endpoints (disabled - handlers not implemented)
		// protectedAPI.GET("/states", handleGetStates)
		// protectedAPI.POST("/states", handleCreateState)
		// protectedAPI.PUT("/states/:id", handleUpdateState)
		// protectedAPI.DELETE("/states/:id", handleDeleteState)

		// Type CRUD endpoints (some handlers exist in lookup_crud_handlers.go)
		// protectedAPI.GET("/types", handleGetTypes)
		protectedAPI.POST("/types", handleCreateType)
		protectedAPI.PUT("/types/:id", handleUpdateType)
		protectedAPI.DELETE("/types/:id", handleDeleteType)

		// Customer search endpoint for autocomplete
		protectedAPI.GET("/customers/search", handleCustomerSearch)

		// Queue CRUD endpoints (disabled - handlers not implemented)
		// protectedAPI.GET("/queues", handleGetQueuesAPI)
		// protectedAPI.POST("/queues", handleCreateQueue)
		// protectedAPI.GET("/queues/:id", handleGetQueue)
		// protectedAPI.PUT("/queues/:id", handleUpdateQueue)
		// protectedAPI.DELETE("/queues/:id", handleDeleteQueue)
		// protectedAPI.GET("/queues/:id/details", handleGetQueueDetails)

		// Priority CRUD endpoints are handled by admin routes
		// protectedAPI.GET("/priorities/:id", handleGetPriority)
		// protectedAPI.POST("/priorities", handleCreatePriority)
		// protectedAPI.PUT("/priorities/:id", handleUpdatePriority)
		// protectedAPI.DELETE("/priorities/:id", handleDeletePriority)

		// Customer User CRUD endpoints (disabled - handlers not implemented)
		// db, _ := database.GetDB()
		// if db != nil {
		//	protectedAPI.GET("/customer-users", handleGetCustomerUsers(db))
		//	protectedAPI.GET("/customer-users/:id", handleGetCustomerUser(db))
		//	protectedAPI.GET("/customer-users/:id/details", handleGetCustomerUserDetails(db))
		//	protectedAPI.POST("/customer-users", handleCreateCustomerUser(db))
		//	protectedAPI.PUT("/customer-users/:id", handleUpdateCustomerUser(db))
		//	protectedAPI.DELETE("/customer-users/:id", handleDeleteCustomerUser(db))
		//	protectedAPI.POST("/customer-users/import", handleImportCustomerUsers(db))
		//	// protectedAPI.GET("/customer-companies", handleGetAvailableCompanies(db)) // Removed - duplicate with line 733
		//
		//	// Customer User Group assignments
		//	protectedAPI.GET("/customer-user-groups/:login", handleGetCustomerUserGroups(db))
		//	protectedAPI.POST("/customer-user-groups/:login", handleSaveCustomerUserGroups(db))
		//	protectedAPI.GET("/group-customer-users/:id", handleGetGroupCustomerUsers(db))
		//	protectedAPI.POST("/group-customer-users/:id", handleSaveGroupCustomerUsers(db))
		// }

		// Customer Company CRUD endpoints (disabled - handlers not implemented)
		// protectedAPI.GET("/customer-companies", handleGetCustomerCompaniesAPI)
		// protectedAPI.POST("/customer-companies", handleCreateCustomerCompanyAPI)
		// protectedAPI.GET("/customer-companies/:id", handleGetCustomerCompanyAPI)
		// protectedAPI.PUT("/customer-companies/:id", handleUpdateCustomerCompanyAPI)
		// protectedAPI.DELETE("/customer-companies/:id", handleDeleteCustomerCompanyAPI)
	}

	// Template endpoints (disabled - duplicate handlers in ticket_template_handlers.go)
	{
		// protectedAPI.GET("/templates", handleGetTemplates)
		// protectedAPI.GET("/templates/:id", handleGetTemplate)
		// protectedAPI.POST("/templates", handleCreateTemplate)
		// protectedAPI.PUT("/templates/:id", handleUpdateTemplate)
		// protectedAPI.DELETE("/templates/:id", handleDeleteTemplate)
		// protectedAPI.GET("/templates/search", handleSearchTemplates)
		// protectedAPI.GET("/templates/categories", handleGetTemplateCategories)
		// protectedAPI.GET("/templates/popular", handleGetPopularTemplates)
		// protectedAPI.POST("/templates/apply", handleApplyTemplate)
		// protectedAPI.GET("/templates/:id/load", handleLoadTemplateIntoForm)
		// protectedAPI.GET("/templates/modal", handleTemplateSelectionModal)
	}

	// SSE endpoints (Server-Sent Events for real-time updates)
	{
		protectedAPI.GET("/tickets/stream", handleTicketStream)
		protectedAPI.GET("/dashboard/activity-stream", handleActivityStream)
	}

	// Setup API v1 routes with existing services
	// TODO: SetupAPIv1Routes not implemented yet
	// SetupAPIv1Routes(r, jwtManager, ldapProvider, i18nSvc)

	// Catch-all for undefined routes
	r.NoRoute(func(c *gin.Context) {
		sendErrorResponse(c, http.StatusNotFound, "Page not found")
	})
}

// Helper function to show under construction message
func underConstruction(feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		pongo2Renderer.HTML(c, http.StatusOK, "pages/under_construction.pongo2", pongo2.Context{
			"Feature":    feature,
			"User":       getUserMapForTemplate(c),
			"ActivePage": "admin",
		})
	}
}

// Helper function for API endpoints under construction
func underConstructionAPI(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Endpoint %s is under construction", endpoint),
		})
	}
}

// Handler functions

// handleLoginPage shows the login page
func handleLoginPage(c *gin.Context) {
	// Check if already logged in
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// Check for error in query parameter
	errorMsg := c.Query("error")

	pongo2Renderer.HTML(c, http.StatusOK, "pages/login.pongo2", pongo2.Context{
		"Title": "Login - GOTRS",
		"error": errorMsg,
	})
}

// handleLogin processes login requests
func handleLogin(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get credentials from form
		username := c.PostForm("username")
		password := c.PostForm("password")

		// Authenticate against database
		validLogin := false
		userID := uint(1)

		// Get database connection
		db, err := database.GetDB()
		if err != nil {
			// No fallback - database connection is required
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "Database connection unavailable",
			})
			return
		}

		// Check credentials against database
		var dbUserID int
		var dbPassword string
		var validID int

		// Query user and verify password
		query := database.ConvertPlaceholders(`
			SELECT id, pw, valid_id
			FROM users
			WHERE login = $1
			AND valid_id = 1`)
		log.Printf("DEBUG: Querying for user '%s' with query: %s", username, query)
		err = db.QueryRow(query, username).Scan(&dbUserID, &dbPassword, &validID)
		if err != nil {
			log.Printf("DEBUG: Query error: %v", err)
		} else {
			log.Printf("DEBUG: Found user ID %d with password hash starting with: %.20s", dbUserID, dbPassword)
		}

		if err == nil && validID == 1 {
			// Verify the password (handles both salted and unsalted)
			log.Printf("DEBUG: Verifying password for user %d", dbUserID)
			if verifyPassword(password, dbPassword) {
				log.Printf("DEBUG: Password verification successful for user %d", dbUserID)
				validLogin = true
				userID = uint(dbUserID)
			} else {
				log.Printf("DEBUG: Password verification failed for user %d", dbUserID)
			}
		} else {
			// If database check fails, try legacy plain text (for migration period)
			// This should be removed once all passwords are migrated
			query2 := database.ConvertPlaceholders(`
				SELECT id, pw, valid_id
				FROM users
				WHERE login = $1
				AND pw = $2
				AND valid_id = 1`)
			err = db.QueryRow(query2, username, password).Scan(&dbUserID, &dbPassword, &validID)

			if err == nil && validID == 1 {
				validLogin = true
				userID = uint(dbUserID)

				// Update the password to use salted hashing
				// Generate salt and hash the password
				salt := generateSalt()
				combined := password + salt
				hash := sha256.Sum256([]byte(combined))
				hashedPassword := fmt.Sprintf("sha256$%s$%s", salt, hex.EncodeToString(hash[:]))

				updateQuery := database.ConvertPlaceholders(`
					UPDATE users
					SET pw = $1,
					    change_time = CURRENT_TIMESTAMP
					WHERE id = $2`)
				_, _ = db.Exec(updateQuery, hashedPassword, dbUserID)
			}
		}

		if !validLogin {
			// For API/HTMX requests, return JSON error
			if c.GetHeader("HX-Request") == "true" || strings.Contains(c.GetHeader("Accept"), "application/json") {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "Invalid credentials",
				})
				return
			}
			// For regular form submission, redirect back to login with error
			pongo2Renderer.HTML(c, http.StatusUnauthorized, "pages/login.pongo2", pongo2.Context{
				"Title": "Login - GOTRS",
				"Error": "Invalid username or password",
			})
			return
		}

		// Create session token
		var token string
		if jwtManager != nil {
			// Use JWT in production
			// For now, use default role "user" and tenantID 1
			tokenStr, err := jwtManager.GenerateToken(userID, username, "user", 1)
			if err != nil {
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to generate token")
				return
			}
			token = tokenStr
		} else {
			// Use simple session token in demo mode - include user ID in token
			token = fmt.Sprintf("demo_session_%d_%d", userID, time.Now().Unix())
		}

		// Get user's preferred session timeout
		sessionTimeout := constants.DefaultSessionTimeout // Default 24 hours
		if db != nil {
			prefService := service.NewUserPreferencesService(db)
			if userTimeout := prefService.GetSessionTimeout(int(userID)); userTimeout > 0 {
				sessionTimeout = userTimeout
			}
		}

		// Set cookie
		c.SetCookie("access_token", token, sessionTimeout, "/", "", false, true)

		// For HTMX requests, send redirect header
		if c.GetHeader("HX-Request") == "true" {
			c.Header("HX-Redirect", "/dashboard")
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"redirect": "/dashboard",
			})
			return
		}

		// For regular form submission, redirect
		c.Redirect(http.StatusFound, "/dashboard")
	}
}

// handleHTMXLogin handles HTMX login requests
func handleHTMXLogin(c *gin.Context) {
	// For demo, accept any credentials
	c.Header("HX-Redirect", "/dashboard")
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleHTMXLogout handles HTMX logout requests
func handleHTMXLogout(c *gin.Context) {
	// Clear the session cookie
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.Header("HX-Redirect", "/login")
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleLogout handles logout requests
func handleLogout(c *gin.Context) {
	// Clear the session cookie
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

// handleDashboard shows the main dashboard
func handleDashboard(c *gin.Context) {
	// Get database connection through repository pattern
	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Use repository for database operations
	ticketRepo := repository.NewTicketRepository(db)

	// Get ticket statistics using repository methods
	var openTickets, pendingTickets, closedToday int

	openTickets, err = ticketRepo.CountByStateID(2) // state_id = 2 for open
	if err != nil {
		openTickets = 0
	}

	pendingTickets, err = ticketRepo.CountByStateID(5) // state_id = 5 for pending
	if err != nil {
		pendingTickets = 0
	}

	closedToday, err = ticketRepo.CountClosedToday()
	if err != nil {
		closedToday = 0
	}

	stats := gin.H{
		"openTickets":     openTickets,
		"pendingTickets":  pendingTickets,
		"closedToday":     closedToday,
		"avgResponseTime": "N/A", // Would require more complex calculation
	}

	// Get recent tickets from database
	// ticketRepo already created above
	listReq := &models.TicketListRequest{
		Page:      1,
		PerPage:   5,
		SortBy:    "create_time",
		SortOrder: "desc",
	}
	ticketResponse, err := ticketRepo.List(listReq)
	tickets := []models.Ticket{}
	if err == nil && ticketResponse != nil {
		tickets = ticketResponse.Tickets
	}

	recentTickets := []gin.H{}
	if err == nil && tickets != nil {
		for _, ticket := range tickets {
			// Determine status label
			statusLabel := "unknown"
			switch ticket.TicketStateID {
			case 1:
				statusLabel = "new"
			case 2:
				statusLabel = "open"
			case 3:
				statusLabel = "closed"
			case 5:
				statusLabel = "pending"
			}

			// Calculate time ago
			timeAgo := timeago.English.Format(ticket.ChangeTime)

			recentTickets = append(recentTickets, gin.H{
				"id":       ticket.TicketNumber,
				"subject":  ticket.Title,
				"status":   statusLabel,
				"priority": getPriorityLabel(ticket.TicketPriorityID),
				"customer": ticket.CustomerUserID,
				"updated":  timeAgo,
			})
		}
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/dashboard.pongo2", pongo2.Context{
		"Title":         "Dashboard - GOTRS",
		"Stats":         stats,
		"RecentTickets": recentTickets,
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "dashboard",
	})
}

// handleTickets shows the tickets list page
func handleTickets(c *gin.Context) {
	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get filter and search parameters
	status := c.Query("status")
	priority := c.Query("priority")
	queue := c.Query("queue")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "created_desc")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit := 25

	// Build ticket list request
	req := &models.TicketListRequest{
		Search:  search,
		SortBy:  sortBy,
		Page:    page,
		PerPage: limit,
	}

	// Apply status filter
	if status != "" && status != "all" {
		stateID, _ := strconv.Atoi(status)
		if stateID > 0 {
			stateIDPtr := uint(stateID)
			req.StateID = &stateIDPtr
		}
	}

	// Apply priority filter
	if priority != "" && priority != "all" {
		priorityID, _ := strconv.Atoi(priority)
		if priorityID > 0 {
			priorityIDPtr := uint(priorityID)
			req.PriorityID = &priorityIDPtr
		}
	}

	// Apply queue filter
	if queue != "" && queue != "all" {
		queueID, _ := strconv.Atoi(queue)
		if queueID > 0 {
			queueIDPtr := uint(queueID)
			req.QueueID = &queueIDPtr
		}
	}

	// Get tickets from repository
	ticketRepo := repository.NewTicketRepository(db)
	result, err := ticketRepo.List(req)
	if err != nil {
		log.Printf("Error fetching tickets: %v", err)
		// Return empty list on error
		result = &models.TicketListResponse{
			Tickets: []models.Ticket{},
			Total:   0,
		}
	}

	log.Printf("DEBUG: Found %d tickets from database", len(result.Tickets))
	if len(result.Tickets) > 0 {
		log.Printf("DEBUG: First ticket: ID=%d, TN=%s, Title=%s", result.Tickets[0].ID, result.Tickets[0].TicketNumber, result.Tickets[0].Title)
	}

	// Convert tickets to template format
	tickets := make([]gin.H, 0, len(result.Tickets))
	for _, t := range result.Tickets {
		// Get state name
		stateName := "unknown"
		switch t.TicketStateID {
		case 1:
			stateName = "new"
		case 2:
			stateName = "open"
		case 3:
			stateName = "pending"
		case 4:
			stateName = "closed"
		}

		// Get priority name
		priorityName := "normal"
		switch t.TicketPriorityID {
		case 1:
			priorityName = "low"
		case 2:
			priorityName = "normal"
		case 3:
			priorityName = "high"
		case 4:
			priorityName = "critical"
		}

		tickets = append(tickets, gin.H{
			"id":       t.TicketNumber,
			"subject":  t.Title,
			"status":   stateName,
			"priority": priorityName,
			"queue":    fmt.Sprintf("Queue %d", t.QueueID), // Will fix with proper queue name lookup
			"customer": fmt.Sprintf("Customer %d", t.CustomerID),
			"agent":    fmt.Sprintf("User %d", t.UserID),
			"created":  t.CreateTime.Format("2006-01-02 15:04"),
			"updated":  t.ChangeTime.Format("2006-01-02 15:04"),
		})
	}

	log.Printf("DEBUG: Created %d tickets for template", len(tickets))

	// Get available filters
	states := []gin.H{
		{"id": 1, "name": "new"},
		{"id": 2, "name": "open"},
		{"id": 3, "name": "pending"},
		{"id": 4, "name": "closed"},
	}

	priorities := []gin.H{
		{"id": 1, "name": "low"},
		{"id": 2, "name": "normal"},
		{"id": 3, "name": "high"},
		{"id": 4, "name": "critical"},
	}

	// Get queues for filter
	queueRepo := repository.NewQueueRepository(db)
	queues, _ := queueRepo.List()
	queueList := make([]gin.H, 0, len(queues))
	for _, q := range queues {
		queueList = append(queueList, gin.H{
			"id":   q.ID,
			"name": q.Name,
		})
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/tickets.pongo2", pongo2.Context{
		"Title":          "Tickets - GOTRS",
		"Tickets":        tickets,
		"User":           getUserMapForTemplate(c),
		"ActivePage":     "tickets",
		"Statuses":       states,
		"Priorities":     priorities,
		"Queues":         queueList,
		"FilterStatus":   status,
		"FilterPriority": priority,
		"FilterQueue":    queue,
		"SearchQuery":    search,
		"SortBy":         sortBy,
		"CurrentPage":    page,
		"TotalPages":     (result.Total + limit - 1) / limit,
		"TotalTickets":   result.Total,
	})
}

// handleQueues shows the queues list page
func handleQueues(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get queues from database
	queueRepo := repository.NewQueueRepository(db)
	queues, err := queueRepo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch queues")
		return
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/queues.pongo2", pongo2.Context{
		"Title":      "Queues - GOTRS",
		"Queues":     queues,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "queues",
	})
}

// handleQueueDetail shows individual queue details
func handleQueueDetail(c *gin.Context) {
	queueID := c.Param("id")

	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get queue details from database
	queueRepo := repository.NewQueueRepository(db)
	id, err := strconv.ParseUint(queueID, 10, 32)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	queue, err := queueRepo.GetByID(uint(id))
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "Queue not found")
		return
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/queue_detail.pongo2", pongo2.Context{
		"Title":      "Queue: " + queue.Name + " - GOTRS",
		"Queue":      queue,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "queues",
	})
}

// handleNewTicket shows the new ticket form
func handleNewTicket(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get queues from database
	queues := []gin.H{}
	qRows, err := db.Query("SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name")
	if err == nil {
		defer qRows.Close()
		for qRows.Next() {
			var id int
			var name string
			if err := qRows.Scan(&id, &name); err == nil {
				queues = append(queues, gin.H{"id": strconv.Itoa(id), "name": name})
			}
		}
	}

	// Get priorities from database
	priorities := []gin.H{}
	pRows, err := db.Query("SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id")
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var id int
			var name string
			if err := pRows.Scan(&id, &name); err == nil {
				// Map priority colors
				color := "gray"
				switch id {
				case 1, 2:
					color = "green"
				case 3:
					color = "yellow"
				case 4:
					color = "orange"
				case 5:
					color = "red"
				}
				priorities = append(priorities, gin.H{"id": strconv.Itoa(id), "name": name, "color": color})
			}
		}
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/ticket_new.pongo2", pongo2.Context{
		"Title":      "New Ticket - GOTRS",
		"User":       getUserMapForTemplate(c),
		"ActivePage": "tickets",
		"Queues":     queues,
		"Priorities": priorities,
	})
}

// handleTicketDetail shows ticket details
func handleTicketDetail(c *gin.Context) {
	ticketID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get ticket from repository
	ticketRepo := repository.NewTicketRepository(db)
	ticket, err := ticketRepo.GetByTicketNumber(ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendErrorResponse(c, http.StatusNotFound, "Ticket not found")
		} else {
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve ticket")
		}
		return
	}

	// Get articles (notes/messages) for the ticket - include all articles for S/MIME support
	articleRepo := repository.NewArticleRepository(db)
	articles, err := articleRepo.GetByTicketID(uint(ticket.ID), true) // true = include all articles including S/MIME
	if err != nil {
		log.Printf("Error fetching articles: %v", err)
		articles = []models.Article{}
	}

	// Convert to template format - match the template's expected field names
	notes := make([]gin.H, 0, len(articles))
	for _, article := range articles {
		// Determine sender type based on CreateBy (simplified logic)
		senderType := "system"
		if article.CreateBy > 0 {
			senderType = "agent" // Assume any user > 0 is an agent
		}

		notes = append(notes, gin.H{
			"from_name":   fmt.Sprintf("User %d", article.CreateBy),
			"sender_type": senderType,
			"create_time": article.CreateTime.Format("2006-01-02 15:04"),
			"subject":     article.Subject,
			"body":        article.Body,
			"attachments": []gin.H{}, // Empty attachments for now
		})
	}

	// Get state name (matching OTRS states from database)
	stateName := "unknown"
	switch ticket.TicketStateID {
	case 1:
		stateName = "new"
	case 2:
		stateName = "open"
	case 3:
		stateName = "closed successful"
	case 4:
		stateName = "closed unsuccessful"
	case 5:
		stateName = "pending reminder"
	case 6:
		stateName = "pending auto close+"
	case 7:
		stateName = "pending auto close-"
	case 8:
		stateName = "removed"
	case 9:
		stateName = "merged"
	default:
		// For any other state, try to get from database
		var stateRow struct {
			Name string
		}
		err := db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = $1"), ticket.TicketStateID).Scan(&stateRow.Name)
		if err == nil {
			stateName = stateRow.Name
		}
	}

	// Get priority name
	priorityName := "normal"
	switch ticket.TicketPriorityID {
	case 1:
		priorityName = "low"
	case 2:
		priorityName = "normal"
	case 3:
		priorityName = "high"
	case 4:
		priorityName = "critical"
	}

	// Check if ticket is closed
	isClosed := false
	if ticket.TicketStateID == 3 || ticket.TicketStateID == 4 || strings.Contains(strings.ToLower(stateName), "closed") {
		isClosed = true
	}

	ticketData := gin.H{
		"id":        ticket.TicketNumber,
		"subject":   ticket.Title,
		"status":    stateName,
		"is_closed": isClosed,
		"priority":  priorityName,
		"queue":     fmt.Sprintf("Queue %d", ticket.QueueID),
		"customer": gin.H{
			"name":  fmt.Sprintf("Customer %d", ticket.CustomerID),
			"email": "customer@example.com",
			"phone": "",
		},
		"agent": gin.H{
			"name":  fmt.Sprintf("User %d", ticket.UserID),
			"email": "agent@gotrs.com",
		},
		"created":     ticket.CreateTime.Format("2006-01-02 15:04"),
		"updated":     ticket.ChangeTime.Format("2006-01-02 15:04"),
		"description": "Ticket description", // Need to get from first article
		"notes":       notes,
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/agent/ticket_view.pongo2", pongo2.Context{
		"Title":      fmt.Sprintf("Ticket %s - GOTRS", ticketID),
		"ticket":     ticketData,
		"articles":   notes, // Pass articles directly for the template loop
		"User":       getUserMapForTemplate(c),
		"ActivePage": "tickets",
	})
}

// handleProfile shows user profile page
func handleProfile(c *gin.Context) {
	user := getUserMapForTemplate(c)

	pongo2Renderer.HTML(c, http.StatusOK, "pages/profile.pongo2", pongo2.Context{
		"Title":      "Profile - GOTRS",
		"User":       user,
		"ActivePage": "profile",
	})
}

// handleSettings shows settings page
func handleSettings(c *gin.Context) {
	user := getUserMapForTemplate(c)

	// TODO: Load actual user settings from database
	// For now, use default settings
	settings := gin.H{
		"emailNotifications": true,
		"autoRefresh":        false,
		"refreshInterval":    60,
		"theme":              "auto",
		"language":           "en",
		"timezone":           "UTC",
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/settings.pongo2", pongo2.Context{
		"Title":      "Settings - GOTRS",
		"User":       user,
		"Settings":   settings,
		"ActivePage": "settings",
	})
}

// API Handler functions

// handleDashboardStats returns dashboard statistics
func handleDashboardStats(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var openTickets, pendingTickets, closedToday int

	// Count open tickets
	db.QueryRow("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = 2").Scan(&openTickets)

	// Count pending tickets
	db.QueryRow("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = 5").Scan(&pendingTickets)

	// Count tickets closed today
	db.QueryRow(database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM ticket
		WHERE ticket_state_id = 3
		AND change_time >= CURRENT_DATE
	`)).Scan(&closedToday)

	stats := gin.H{
		"openTickets":     openTickets,
		"pendingTickets":  pendingTickets,
		"closedToday":     closedToday,
		"avgResponseTime": "N/A",
		"satisfaction":    "N/A",
	}
	c.JSON(http.StatusOK, stats)
}

// handleRecentTickets returns recent tickets for dashboard
func handleRecentTickets(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	ticketRepo := repository.NewTicketRepository(db)
	listReq := &models.TicketListRequest{
		Page:      1,
		PerPage:   5,
		SortBy:    "create_time",
		SortOrder: "desc",
	}
	ticketResponse, err := ticketRepo.List(listReq)
	tickets := []models.Ticket{}
	if err == nil && ticketResponse != nil {
		tickets = ticketResponse.Tickets
	}

	ticketList := []gin.H{}
	if err == nil && tickets != nil {
		for _, ticket := range tickets {
			// Determine status label
			statusLabel := "unknown"
			switch ticket.TicketStateID {
			case 1:
				statusLabel = "new"
			case 2:
				statusLabel = "open"
			case 3:
				statusLabel = "closed"
			case 5:
				statusLabel = "pending"
			}

			ticketList = append(ticketList, gin.H{
				"id":       ticket.TicketNumber,
				"subject":  ticket.Title,
				"status":   statusLabel,
				"priority": getPriorityLabel(ticket.TicketPriorityID),
				"customer": ticket.CustomerUserID,
				"updated":  timeago.English.Format(ticket.ChangeTime),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"tickets": ticketList})
}

// handleNotifications returns user notifications
func handleNotifications(c *gin.Context) {
	// TODO: Implement actual notifications from database
	// For now, return empty list
	notifications := []gin.H{}
	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

// handleQuickActions returns quick action items
func handleQuickActions(c *gin.Context) {
	actions := []gin.H{
		{"id": "new_ticket", "label": "New Ticket", "icon": "plus", "url": "/ticket/new"},
		{"id": "my_tickets", "label": "My Tickets", "icon": "list", "url": "/tickets?assigned=me"},
		{"id": "reports", "label": "Reports", "icon": "chart", "url": "/reports"},
	}
	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

// handleActivity returns recent activity
func handleActivity(c *gin.Context) {
	activities := []gin.H{
		{
			"id":     "1",
			"type":   "ticket_created",
			"user":   "John Doe",
			"action": "created ticket T-2024-001",
			"time":   "5 minutes ago",
		},
		{
			"id":     "2",
			"type":   "ticket_updated",
			"user":   "Alice Agent",
			"action": "updated ticket T-2024-002",
			"time":   "10 minutes ago",
		},
	}
	c.JSON(http.StatusOK, gin.H{"activities": activities})
}

// handlePerformance returns performance metrics
func handlePerformance(c *gin.Context) {
	metrics := gin.H{
		"responseTime": []gin.H{
			{"time": "00:00", "value": 2.1},
			{"time": "04:00", "value": 1.8},
			{"time": "08:00", "value": 3.2},
			{"time": "12:00", "value": 2.5},
			{"time": "16:00", "value": 2.8},
			{"time": "20:00", "value": 2.0},
		},
		"ticketVolume": []gin.H{
			{"day": "Mon", "created": 45, "closed": 42},
			{"day": "Tue", "created": 52, "closed": 48},
			{"day": "Wed", "created": 38, "closed": 40},
			{"day": "Thu", "created": 61, "closed": 55},
			{"day": "Fri", "created": 43, "closed": 45},
		},
	}
	c.JSON(http.StatusOK, metrics)
}

// Ticket API handlers

// handleAPITickets returns list of tickets
func handleAPITickets(c *gin.Context) {
	// Get query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")

	// Mock ticket data
	tickets := []gin.H{
		{
			"id":       "T-2024-001",
			"subject":  "Unable to access email",
			"status":   "open",
			"priority": "high",
		},
		{
			"id":       "T-2024-002",
			"subject":  "Software installation request",
			"status":   "pending",
			"priority": "medium",
		},
	}

	// Apply status filter if provided
	if status != "" {
		// Filter logic here
	}

	c.JSON(http.StatusOK, gin.H{
		"tickets": tickets,
		"page":    page,
		"limit":   limit,
		"total":   len(tickets),
	})
}

// handleCreateTicket creates a new ticket
func handleCreateTicket(c *gin.Context) {
	log.Println("DEBUG: handleCreateTicket called - NEW VERSION WITH DATABASE SAVE")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Parse the request
	var req service.CreateTicketRequest

	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Handle JSON request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	} else {
		// Handle form data
		req.Subject = c.PostForm("subject")
		req.Body = c.PostForm("description")
		if req.Body == "" {
			req.Body = c.PostForm("body")
		}
		req.Priority = c.PostForm("priority")

		// Parse queue ID
		if queueStr := c.PostForm("queue"); queueStr != "" {
			if queueID, err := strconv.Atoi(queueStr); err == nil {
				req.QueueID = queueID
			}
		}

		// Parse type ID
		if typeStr := c.PostForm("type"); typeStr != "" {
			if typeID, err := strconv.Atoi(typeStr); err == nil {
				req.TypeID = typeID
			}
		}

		// Handle customer data
		if customerEmail := c.PostForm("customer_email"); customerEmail != "" {
			req.CustomerEmail = customerEmail
			req.CustomerName = c.PostForm("customer_name")
		} else {
			// Try new customer fields
			req.CustomerEmail = c.PostForm("new_customer_email")
			req.CustomerName = c.PostForm("new_customer_name")
		}

		// Validate required fields
		if req.Subject == "" || req.Body == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Subject and description are required",
			})
			return
		}

		if req.CustomerEmail == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Customer email is required",
			})
			return
		}
	}

	// Get current user ID (use 1 for demo/test)
	createBy := 1
	if userCtx, ok := c.Get("user"); ok {
		if user, ok := userCtx.(*models.User); ok && user.ID > 0 {
			createBy = int(user.ID)
		}
	}

	// Create the ticket using the service
	ticketService := service.NewTicketService(db)
	result, err := ticketService.CreateTicket(&req, createBy)
	if err != nil {
		log.Printf("Error creating ticket: %v", err)
		// Provide more specific error messages
		errorMsg := "Failed to create ticket"
		if strings.Contains(err.Error(), "queue") {
			errorMsg = "Invalid queue selected. Please select a valid queue."
		} else if strings.Contains(err.Error(), "customer") {
			errorMsg = "Customer validation failed. Please check customer details."
		} else if strings.Contains(err.Error(), "database") {
			errorMsg = "Database error. Please try again later."
		} else if strings.Contains(err.Error(), "duplicate") {
			errorMsg = "A similar ticket already exists."
		} else {
			// Include the actual error for debugging
			errorMsg = fmt.Sprintf("Failed to create ticket: %v", err)
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   errorMsg,
			"details": err.Error(), // Include full error for debugging
		})
		return
	}

	// Return success with the actual ticket number
	c.JSON(http.StatusCreated, gin.H{
		"success":       true,
		"ticket_id":     result.TicketNumber,
		"ticket_number": result.TicketNumber,
		"id":            result.ID,
		"message":       "Ticket created successfully",
	})
}

// handleGetTicket returns a specific ticket
func handleGetTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Get ticket from repository
	ticketRepo := repository.NewTicketRepository(db)
	ticket, err := ticketRepo.GetByTicketNumber(ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Ticket not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to retrieve ticket",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"ticket":  ticket,
	})
}

// handleUpdateTicket updates a ticket
func handleUpdateTicket(c *gin.Context) {
	ticketID := c.Param("id")

	var updates gin.H
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ticket": gin.H{
			"id":      ticketID,
			"updated": time.Now().Format("2006-01-02 15:04"),
		},
	})
}

// handleDeleteTicket deletes a ticket (soft delete)
func handleDeleteTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// First get the ticket by number to get its ID
	ticketRepo := repository.NewTicketRepository(db)
	ticket, err := ticketRepo.GetByTicketNumber(ticketIDStr)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Ticket not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to retrieve ticket",
			})
		}
		return
	}

	// Soft delete the ticket
	err = ticketRepo.Delete(uint(ticket.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete ticket",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Ticket %s deleted", ticketIDStr),
	})
}

// handleAddTicketNote adds a note to a ticket
func handleAddTicketNote(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse the note data
	var noteData struct {
		Content  string `json:"content" binding:"required"`
		Internal bool   `json:"internal"`
	}

	if err := c.ShouldBindJSON(&noteData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Note content is required"})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get ticket to verify it exists
	ticketRepo := repository.NewTicketRepository(db)
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		// Try to get by ticket number instead
		ticket, err := ticketRepo.GetByTicketNumber(ticketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		ticketIDInt = ticket.ID
	}

	// Get current user
	userID := 1 // Default system user
	if userCtx, ok := c.Get("user"); ok {
		if user, ok := userCtx.(*models.User); ok && user.ID > 0 {
			userID = int(user.ID)
		}
	}

	// Create article (note) in database
	articleRepo := repository.NewArticleRepository(db)
	article := &models.Article{
		TicketID:               ticketIDInt,
		Subject:                "Note",
		Body:                   noteData.Content,
		SenderTypeID:           1, // Agent
		CommunicationChannelID: 7, // Note
		IsVisibleForCustomer:   0, // Internal note by default
		CreateBy:               userID,
		ChangeBy:               userID,
	}

	if !noteData.Internal {
		article.IsVisibleForCustomer = 1
	}

	err = articleRepo.Create(article)
	if err != nil {
		log.Printf("Error creating note: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save note"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":  true,
		"noteId":   article.ID,
		"ticketId": ticketIDInt,
		"created":  article.CreateTime.Format("2006-01-02 15:04"),
	})
}

// handleGetTicketHistory returns ticket history
func handleGetTicketHistory(c *gin.Context) {
	ticketID := c.Param("id")

	history := []gin.H{
		{
			"id":     "1",
			"action": "created",
			"user":   "System",
			"time":   "2024-01-10 09:00",
		},
		{
			"id":      "2",
			"action":  "assigned",
			"user":    "Admin",
			"time":    "2024-01-10 09:05",
			"details": "Assigned to Alice Agent",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"ticketId": ticketID,
		"history":  history,
	})
}

// handleGetAvailableAgents returns agents who have permissions for the ticket's queue
func handleGetAvailableAgents(c *gin.Context) {
	ticketID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Convert ticket ID to int
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Query to get agents who have permissions for the ticket's queue
	// This joins ticket -> queue -> groups -> group_user -> users
	query := `
		SELECT DISTINCT u.id, u.login, u.first_name, u.last_name
		FROM users u
		INNER JOIN group_user ug ON u.id = ug.user_id
		INNER JOIN queue q ON q.group_id = ug.group_id
		INNER JOIN ticket t ON t.queue_id = q.id
		WHERE t.id = $1
		  AND u.valid_id = 1
		  AND ug.permission_key IN ('rw', 'move_into', 'create', 'owner')
		  AND ug.permission_value = 1
		ORDER BY u.id
	`

	rows, err := db.Query(query, ticketIDInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agents"})
		return
	}
	defer rows.Close()

	agents := []gin.H{}
	for rows.Next() {
		var id int
		var login, firstName, lastName string
		if err := rows.Scan(&id, &login, &firstName, &lastName); err != nil {
			continue
		}

		agents = append(agents, gin.H{
			"id":    id,
			"name":  fmt.Sprintf("%s %s", firstName, lastName),
			"login": login,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"agents":  agents,
	})
}

// handleAssignTicket assigns a ticket to an agent
func handleAssignTicket(c *gin.Context) {
	ticketID := c.Param("id")

	var assignment struct {
		AgentID int `json:"agentId"`
	}
	if err := c.ShouldBindJSON(&assignment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Convert ticket ID to int
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Get current user for change_by
	userID := 1 // Default system user
	if userCtx, ok := c.Get("user"); ok {
		if user, ok := userCtx.(*models.User); ok && user.ID > 0 {
			userID = int(user.ID)
		}
	}

	// Update the ticket's responsible_user_id
	_, err = db.Exec(database.ConvertPlaceholders(`
		UPDATE ticket
		SET responsible_user_id = $1, change_time = NOW(), change_by = $2
		WHERE id = $3
	`), assignment.AgentID, userID, ticketIDInt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign ticket"})
		return
	}

	// Get the agent's name for the response
	var agentName string
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT first_name || ' ' || last_name
		FROM users
		WHERE id = $1
	`), assignment.AgentID).Scan(&agentName)

	if err != nil {
		agentName = fmt.Sprintf("Agent %d", assignment.AgentID)
	}

	c.JSON(http.StatusOK, gin.H{
		"ticketId": ticketID,
		"assigned": agentName,
		"time":     time.Now().Format("2006-01-02 15:04"),
	})
}

// handleCloseTicket closes a ticket
func handleCloseTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse request body
	var closeData struct {
		StateID        int    `json:"state_id"`
		Resolution     string `json:"resolution"`
		Notes          string `json:"notes" binding:"required"`
		TimeUnits      int    `json:"time_units"`
		NotifyCustomer bool   `json:"notify_customer"`
	}

	if err := c.ShouldBindJSON(&closeData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Default to closed successful if not specified
	if closeData.StateID == 0 {
		closeData.StateID = 3
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get ticket to verify it exists
	ticketRepo := repository.NewTicketRepository(db)
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		// Try to get by ticket number
		ticket, err := ticketRepo.GetByTicketNumber(ticketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		ticketIDInt = ticket.ID
	}

	// Get current user
	userID := 1 // Default system user
	if userCtx, ok := c.Get("user"); ok {
		if user, ok := userCtx.(*models.User); ok && user.ID > 0 {
			userID = int(user.ID)
		}
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Update ticket state
	_, err = tx.Exec(database.ConvertPlaceholders(`
		UPDATE ticket
		SET ticket_state_id = $1, change_time = NOW(), change_by = $2
		WHERE id = $3
	`), closeData.StateID, userID, ticketIDInt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close ticket"})
		return
	}

	// Add close note as an article (skip for now - articleRepo doesn't support transactions yet)
	// We'll just update the ticket state for now
	// TODO: Add transaction support to article repository

	// TODO: Store time units if time tracking is implemented

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"ticketId": ticketIDInt,
		"status":   "closed",
		"stateId":  closeData.StateID,
		"closedAt": time.Now().Format("2006-01-02 15:04"),
	})
}

// handleReopenTicket reopens a ticket
func handleReopenTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse the request body for additional reopen data
	var reopenData struct {
		StateID        int    `json:"state_id"`
		Reason         string `json:"reason" binding:"required"`
		Notes          string `json:"notes"`
		NotifyCustomer bool   `json:"notify_customer"`
	}

	if err := c.ShouldBindJSON(&reopenData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reopen request: " + err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get ticket to verify it exists
	ticketRepo := repository.NewTicketRepository(db)
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		// Try to get by ticket number
		ticket, err := ticketRepo.GetByTicketNumber(ticketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		ticketIDInt = ticket.ID
	}

	// Default to state 2 (open) if not specified or invalid
	targetStateID := reopenData.StateID
	if targetStateID != 1 && targetStateID != 2 {
		targetStateID = 2 // Default to open
	}

	// Update ticket state
	_, err = db.Exec(database.ConvertPlaceholders(`
		UPDATE ticket
		SET ticket_state_id = $1, change_time = NOW(), change_by = $2
		WHERE id = $3
	`), targetStateID, 1, ticketIDInt) // Using system user (1) for now

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reopen ticket"})
		return
	}

	// Add a reopen note/history entry
	reopenNote := fmt.Sprintf("Ticket reopened\nReason: %s", reopenData.Reason)
	if reopenData.Notes != "" {
		reopenNote += fmt.Sprintf("\nAdditional notes: %s", reopenData.Notes)
	}

	// Insert history/note entry
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_type_id, subject, body, created_time, created_by, change_time, change_by)
		VALUES ($1, 1, $2, $3, NOW(), $4, NOW(), $4)
	`), ticketIDInt, "Ticket Reopened", reopenNote, 1) // Using system user (1) for now

	if err != nil {
		// Log the error but don't fail the reopen operation
		fmt.Printf("Warning: Failed to add reopen note: %v\n", err)
	}

	// TODO: Implement customer notification if reopenData.NotifyCustomer is true

	statusText := "open"
	if targetStateID == 1 {
		statusText = "new"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"ticketId":   ticketIDInt,
		"status":     statusText,
		"reason":     reopenData.Reason,
		"reopenedAt": time.Now().Format("2006-01-02 15:04"),
	})
}

// handleSearchTickets searches tickets
func handleSearchTickets(c *gin.Context) {
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusOK, gin.H{
			"query":   query,
			"results": []gin.H{},
			"total":   0,
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Search in ticket title and number
	results := []gin.H{}
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, tn, title
		FROM ticket
		WHERE title ILIKE $1 OR tn ILIKE $1
		LIMIT 20
	`), "%"+query+"%")

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var tn, title string
			if err := rows.Scan(&id, &tn, &title); err == nil {
				results = append(results, gin.H{
					"id":        tn,
					"subject":   title,
					"relevance": 1.0,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"results": results,
		"total":   len(results),
	})
}

// handleFilterTickets filters tickets
func handleFilterTickets(c *gin.Context) {
	// Get filter parameters
	filters := gin.H{
		"status":   c.Query("status"),
		"priority": c.Query("priority"),
		"queue":    c.Query("queue"),
		"agent":    c.Query("agent"),
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Build dynamic query based on filters
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argCount := 0

	if status, ok := filters["status"].(string); ok && status != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND ticket_state_id = $%d", argCount)
		// Map status name to ID
		statusID := 0
		switch status {
		case "new":
			statusID = 1
		case "open":
			statusID = 2
		case "closed":
			statusID = 3
		case "pending":
			statusID = 5
		}
		args = append(args, statusID)
	}

	if priority, ok := filters["priority"].(string); ok && priority != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND ticket_priority_id = $%d", argCount)
		args = append(args, priority)
	}

	if queue, ok := filters["queue"].(string); ok && queue != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND queue_id = $%d", argCount)
		args = append(args, queue)
	}

	if agent, ok := filters["agent"].(string); ok && agent != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, agent)
	}

	query := fmt.Sprintf(`
		SELECT id, tn, title, ticket_state_id, ticket_priority_id
		FROM ticket
		%s
		LIMIT 50
	`, whereClause)

	tickets := []gin.H{}
	rows, err := db.Query(query, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, stateID, priorityID int
			var tn, title string
			if err := rows.Scan(&id, &tn, &title, &stateID, &priorityID); err == nil {
				tickets = append(tickets, gin.H{
					"id":       tn,
					"subject":  title,
					"status":   stateID,
					"priority": priorityID,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"filters": filters,
		"tickets": tickets,
		"total":   len(tickets),
	})
}

// Attachment handlers are defined in ticket_attachment_handler.go

/* Commented out - defined in ticket_attachment_handler.go
func handleUploadAttachment(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	// Create attachment record
	attachment := gin.H{
		"id":       fmt.Sprintf("A-%d", time.Now().Unix()),
		"ticketId": ticketID,
		"filename": header.Filename,
		"size":     header.Size,
		"mimeType": header.Header.Get("Content-Type"),
		"uploaded": time.Now().Format("2006-01-02 15:04"),
	}

	c.JSON(http.StatusCreated, gin.H{"attachment": attachment})
}

func handleDownloadAttachment(c *gin.Context) {
	ticketID := c.Param("id")
	attachmentID := c.Param("attachment_id")

	// Mock file data
	data := []byte("This is a mock attachment file content")

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"attachment_%s_%s.txt\"", ticketID, attachmentID))
	c.Data(http.StatusOK, "text/plain", data)
}

func handleGetThumbnail(c *gin.Context) {
	// Return a simple placeholder image
	c.Header("Content-Type", "image/svg+xml")
	c.String(http.StatusOK, `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
		<rect width="100" height="100" fill="#ddd"/>
		<text x="50" y="50" text-anchor="middle" dy=".3em" fill="#999">Thumbnail</text>
	</svg>`)
}

func handleDeleteAttachment(c *gin.Context) {
	ticketID := c.Param("id")
	attachmentID := c.Param("attachment_id")

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Attachment %s deleted from ticket %s", attachmentID, ticketID),
	})
}
*/ // End of attachment handler duplicates

// handleServeFile is defined in file_handler.go

// Lookup data handlers

// Lookup data handlers are now defined in separate files:
// - handleGetQueues in lookup_handlers.go or queue_handlers.go
// - handleGetPriorities in priority_handlers.go
// - handleGetTypes in type_handlers.go
// - handleGetStatuses in lookup_handlers.go
// - handleGetFormData in lookup_handlers.go

// Template handlers are defined in ticket_template_handlers.go

/* Commented out - defined in ticket_template_handlers.go
func handleGetTemplates(c *gin.Context) {
	templates := []gin.H{
		{
			"id":          "1",
			"name":        "Password Reset",
			"category":    "Support",
			"description": "Standard password reset template",
		},
		{
			"id":          "2",
			"name":        "New User Setup",
			"category":    "IT",
			"description": "Template for new user onboarding",
		},
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// handleGetTemplate returns a specific template
func handleGetTemplate(c *gin.Context) {
	templateID := c.Param("id")

	template := gin.H{
		"id":          templateID,
		"name":        "Password Reset",
		"category":    "Support",
		"subject":     "Password Reset Request",
		"description": "User needs password reset",
		"priority":    "medium",
		"queue":       "Support",
	}

	c.JSON(http.StatusOK, gin.H{"template": template})
}

// handleCreateTemplate creates a new template
func handleCreateTemplate(c *gin.Context) {
	var template gin.H
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template["id"] = fmt.Sprintf("T-%d", time.Now().Unix())
	template["created"] = time.Now().Format("2006-01-02 15:04")

	c.JSON(http.StatusCreated, gin.H{"template": template})
}

// handleUpdateTemplate updates a template
func handleUpdateTemplate(c *gin.Context) {
	templateID := c.Param("id")

	var updates gin.H
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"template": gin.H{
			"id":      templateID,
			"updated": time.Now().Format("2006-01-02 15:04"),
		},
	})
}

// handleDeleteTemplate deletes a template
func handleDeleteTemplate(c *gin.Context) {
	templateID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Template %s deleted", templateID)})
}

// handleSearchTemplates searches templates
func handleSearchTemplates(c *gin.Context) {
	query := c.Query("q")
	category := c.Query("category")

	templates := []gin.H{
		{
			"id":       "1",
			"name":     "Password Reset",
			"category": category,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"query":     query,
		"templates": templates,
	})
}

// handleGetTemplateCategories returns template categories
func handleGetTemplateCategories(c *gin.Context) {
	categories := []string{"Support", "IT", "Network", "Billing", "General"}
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// handleGetPopularTemplates returns popular templates
func handleGetPopularTemplates(c *gin.Context) {
	templates := []gin.H{
		{
			"id":       "1",
			"name":     "Password Reset",
			"useCount": 150,
		},
		{
			"id":       "2",
			"name":     "New User Setup",
			"useCount": 89,
		},
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// handleApplyTemplate applies a template to a ticket
func handleApplyTemplate(c *gin.Context) {
	var request gin.H
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ticketId":   request["ticketId"],
		"templateId": request["templateId"],
		"applied":    true,
	})
}

// handleLoadTemplateIntoForm loads template data for form population
func handleLoadTemplateIntoForm(c *gin.Context) {
	templateID := c.Param("id")

	formData := gin.H{
		"subject":     "Password Reset Request",
		"description": "User needs password reset for their account",
		"priority":    "medium",
		"queue":       "Support",
		"type":        "Request",
	}

	c.JSON(http.StatusOK, gin.H{
		"templateId": templateID,
		"formData":   formData,
	})
}

// handleTemplateSelectionModal returns HTML for template selection modal
func handleTemplateSelectionModal(c *gin.Context) {
	// Return HTML fragment for HTMX
	html := `
	<div class="modal-content">
		<h3>Select Template</h3>
		<ul>
			<li><a href="#" onclick="selectTemplate('1')">Password Reset</a></li>
			<li><a href="#" onclick="selectTemplate('2')">New User Setup</a></li>
		</ul>
	</div>
	`
	c.Data(http.StatusOK, "text/html", []byte(html))
}
*/ // End of template handler duplicates

// SSE handlers

// handleTicketStream provides real-time ticket updates via SSE
func handleTicketStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Send a ping event every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Send initial connection event
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"message\": \"Connected to ticket stream\"}\n\n")
	c.Writer.Flush()

	// Simulate ticket updates
	for {
		select {
		case <-ticker.C:
			// Send ping to keep connection alive
			fmt.Fprintf(c.Writer, "event: ping\ndata: {\"time\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			// Client disconnected
			return
		}
	}
}

// handleActivityStream provides real-time activity updates
func handleActivityStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Send activity updates periodically
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			activity := gin.H{
				"type":   "ticket_update",
				"user":   "System",
				"action": "Ticket updated",
				"time":   time.Now().Format("15:04:05"),
			}
			data, _ := json.Marshal(activity)
			fmt.Fprintf(c.Writer, "event: activity\ndata: %s\n\n", data)
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// Admin handlers

// handleAdminDashboard shows the admin dashboard
func handleAdminDashboard(c *gin.Context) {
	// Get some stats from the database
	db, _ := database.GetDB()

	userCount := 0
	groupCount := 0
	activeTickets := 0
	queueCount := 0

	if db != nil {
		db.QueryRow("SELECT COUNT(*) FROM users WHERE valid_id = 1").Scan(&userCount)
		db.QueryRow("SELECT COUNT(*) FROM groups WHERE valid_id = 1").Scan(&groupCount)
		db.QueryRow("SELECT COUNT(*) FROM queue WHERE valid_id = 1").Scan(&queueCount)
		// Note: ticket table might not exist yet
		db.QueryRow("SELECT COUNT(*) FROM ticket WHERE ticket_state_id IN (1,2,3,4)").Scan(&activeTickets)
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/dashboard.pongo2", pongo2.Context{
		"UserCount":     userCount,
		"GroupCount":    groupCount,
		"ActiveTickets": activeTickets,
		"QueueCount":    queueCount,
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "admin",
	})
}

// handleSchemaDiscovery shows the schema discovery page
func handleSchemaDiscovery(c *gin.Context) {
	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/schema_discovery.pongo2", pongo2.Context{
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
		"Title":      "Schema Discovery",
	})
}

// handleSchemaMonitoring shows the schema monitoring dashboard
func handleSchemaMonitoring(c *gin.Context) {
	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/schema_monitoring.pongo2", pongo2.Context{
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
		"Title":      "Schema Discovery Monitor",
	})
}

// handleAdminUsers shows the admin users page
func handleAdminUsers(c *gin.Context) {
	fmt.Println("DEBUG: handleAdminUsers - Starting")
	db, err := database.GetDB()
	if err != nil {
		fmt.Printf("DEBUG: handleAdminUsers - Database connection failed: %v\n", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	fmt.Println("DEBUG: handleAdminUsers - Database connected, fetching users")
	// Get users from database with their groups
	userRepo := repository.NewUserRepository(db)
	users, err := userRepo.ListWithGroups()
	if err != nil {
		fmt.Printf("DEBUG: handleAdminUsers - Failed to fetch users: %v\n", err)
		sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch users: %v", err))
		return
	}
	fmt.Printf("DEBUG: handleAdminUsers - Fetched %d users\n", len(users))

	// Get groups for filter
	groupRepo := repository.NewGroupRepository(db)
	groups, err := groupRepo.List()
	if err != nil {
		// If we can't get groups, just continue with empty list
		groups = []*models.Group{}
	}

	// Apply filters if present
	search := c.Query("search")
	statusFilter := c.Query("status")
	groupFilter := c.Query("group")

	// Filter users based on search and filters
	var filteredUsers []*models.User
	for _, user := range users {
		// Skip if search doesn't match
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(user.Login), searchLower) &&
				!strings.Contains(strings.ToLower(user.FirstName), searchLower) &&
				!strings.Contains(strings.ToLower(user.LastName), searchLower) &&
				!strings.Contains(strings.ToLower(user.Email), searchLower) {
				continue
			}
		}

		// Skip if status doesn't match
		if statusFilter != "" && statusFilter != "all" {
			if statusFilter == "active" && user.ValidID != 1 {
				continue
			}
			if statusFilter == "inactive" && user.ValidID != 2 {
				continue
			}
		}

		// Skip if group doesn't match
		if groupFilter != "" && groupFilter != "all" {
			// TODO: Check if user is in the specified group
			// For now, just include all users when group filter is set
		}

		filteredUsers = append(filteredUsers, user)
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/users.pongo2", pongo2.Context{
		"Users":        filteredUsers,
		"Groups":       groups,
		"Search":       search,
		"StatusFilter": statusFilter,
		"GroupFilter":  groupFilter,
		"User":         getUserMapForTemplate(c),
		"ActivePage":   "admin",
		"t": func(key string) string {
			// Simple translation fallback - just return the key for now
			translations := map[string]string{
				"admin.users":             "Users",
				"app.name":                "GOTRS",
				"admin.users_description": "Manage system users and their permissions",
				"admin.add_user_tooltip":  "Add new user",
			}
			if val, ok := translations[key]; ok {
				return val
			}
			return key
		},
	})
}

// handleNewUser shows the new user form
func handleNewUser(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get groups for the form
	groupRepo := repository.NewGroupRepository(db)
	groups, _ := groupRepo.List()

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/user_form.pongo2", pongo2.Context{
		"Title":      "New User",
		"Groups":     groups,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// ============================================================================
// ARCHIVED USER HANDLERS - Replaced by Dynamic Module System
// These handlers are no longer used. The /admin/users routes now forward to
// the dynamic module system. Kept here temporarily for reference.
// TODO: Remove these functions once migration is fully verified
// ============================================================================

// handleCreateUser creates a new user - ARCHIVED: Use dynamic module instead
func handleCreateUser(c *gin.Context) {
	var userForm struct {
		Login     string   `form:"login" binding:"required"`
		Email     string   `form:"email"`
		FirstName string   `form:"first_name" binding:"required"`
		LastName  string   `form:"last_name" binding:"required"`
		Password  string   `form:"password" binding:"required"`
		Groups    []string `form:"groups[]"`
		IsActive  bool     `form:"is_active"`
	}

	if err := c.ShouldBind(&userForm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	userRepo := repository.NewUserRepository(db)

	// Create the user
	user := &models.User{
		Login:     userForm.Login,
		Email:     userForm.Email,
		FirstName: userForm.FirstName,
		LastName:  userForm.LastName,
		ValidID:   1, // Active by default
	}

	if !userForm.IsActive {
		user.ValidID = 2 // Inactive
	}

	// Hash the password
	// TODO: Implement proper password hashing
	user.Password = userForm.Password // For now, store as plain text (NOT FOR PRODUCTION!)

	if err := userRepo.Create(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Add user to groups
	if len(userForm.Groups) > 0 {
		groupRepo := repository.NewGroupRepository(db)
		for _, groupIDStr := range userForm.Groups {
			if groupID, err := strconv.ParseUint(groupIDStr, 10, 32); err == nil {
				groupRepo.AddUserToGroup(user.ID, uint(groupID))
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":  true,
		"user":     user,
		"redirect": "/admin/users",
	})
}

// handleGetUser returns user details
func handleGetUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get user's groups
	groupRepo := repository.NewGroupRepository(db)
	groupNames, _ := groupRepo.GetUserGroups(user.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         user.ID,
			"login":      user.Login,
			"title":      user.Title,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"email":      user.Email,
			"valid_id":   user.ValidID,
			"groups":     groupNames,
		},
	})
}

// handleEditUser shows the edit user form
func handleEditUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "User not found")
		return
	}

	// Get all groups and user's current groups
	groupRepo := repository.NewGroupRepository(db)
	groups, _ := groupRepo.List()
	userGroups, _ := groupRepo.GetUserGroups(user.ID)

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/user_form.pongo2", pongo2.Context{
		"Title":      "Edit User",
		"EditUser":   user,
		"Groups":     groups,
		"UserGroups": userGroups,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// handleUpdateUser updates a user
func handleUpdateUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var userForm struct {
		Login     string   `form:"login" json:"login"`
		Email     string   `form:"email" json:"email"`
		FirstName string   `form:"first_name" json:"first_name"`
		LastName  string   `form:"last_name" json:"last_name"`
		Password  string   `form:"password" json:"password"`
		Groups    []string `form:"groups[]" json:"groups"`
		IsActive  bool     `form:"is_active" json:"is_active"`
	}

	if err := c.ShouldBind(&userForm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update user fields
	if userForm.Login != "" {
		user.Login = userForm.Login
	}
	if userForm.Email != "" {
		user.Email = userForm.Email
	}
	if userForm.FirstName != "" {
		user.FirstName = userForm.FirstName
	}
	if userForm.LastName != "" {
		user.LastName = userForm.LastName
	}
	if userForm.Password != "" {
		// TODO: Implement proper password hashing
		user.Password = userForm.Password
	}

	user.ValidID = 1
	if !userForm.IsActive {
		user.ValidID = 2
	}

	if err := userRepo.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Update group memberships
	// TODO: Implement group membership updates

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// handleDeleteUser deletes a user
func handleDeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	userRepo := repository.NewUserRepository(db)

	// In OTRS style, we don't actually delete users, we mark them as invalid
	user, err := userRepo.GetByID(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.ValidID = 2 // Mark as invalid/inactive
	if err := userRepo.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// handleUpdateUserStatus updates a user's active/inactive status
func handleUpdateUserStatus(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var request struct {
		ValidID int `json:"valid_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.ValidID = request.ValidID
	user.ChangeTime = time.Now()
	user.ChangeBy = int(getUserFromContext(c).ID)

	if err := userRepo.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User status updated successfully",
	})
}

// handleResetUserPassword resets a user's password
func handleResetUserPassword(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var request struct {
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if request.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Hash the new password - use bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user.Password = string(hashedPassword)
	user.ChangeTime = time.Now()
	user.ChangeBy = int(getUserFromContext(c).ID)

	if err := userRepo.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password reset successfully",
	})
}

// handleAdminGroups shows the admin groups page
func handleAdminGroups(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	groups, err := groupRepo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch groups")
		return
	}

	// Convert groups to display format
	var groupList []gin.H
	for _, group := range groups {
		// Get member count for each group
		groupIDUint, _ := group.ID.(uint)
		members, _ := groupRepo.GetGroupMembers(groupIDUint)
		memberCount := len(members)

		groupList = append(groupList, gin.H{
			"ID":          group.ID,
			"Name":        group.Name,
			"Description": group.Comments,
			"MemberCount": memberCount,
			"ValidID":     group.ValidID,
			"IsActive":    group.ValidID == 1,
			"IsSystem":    group.Name == "admin" || group.Name == "users" || group.Name == "stats",
			"CreateTime":  group.CreateTime,
		})
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/groups.pongo2", pongo2.Context{
		"Groups":     groupList,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// handleCreateGroup creates a new group
func handleCreateGroup(c *gin.Context) {
	var groupForm struct {
		Name        string `form:"name" json:"name" binding:"required"`
		Description string `form:"description" json:"description"`
	}

	if err := c.ShouldBind(&groupForm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get current user for audit fields
	userID := 1 // Default to system user
	if userCtx, ok := c.Get("user"); ok {
		if userData, ok := userCtx.(*models.User); ok && userData != nil {
			userID = int(userData.ID)
		}
	}

	groupRepo := repository.NewGroupRepository(db)
	group := &models.Group{
		Name:     groupForm.Name,
		Comments: groupForm.Description,
		ValidID:  1, // Active by default
		CreateBy: userID,
		ChangeBy: userID,
	}

	if err := groupRepo.Create(group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"group":   group,
	})
}

// handleGetGroup returns group details
func handleGetGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	group, err := groupRepo.GetByID(uint(groupID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Get group members
	groupIDUint, _ := group.ID.(uint)
	members, _ := groupRepo.GetGroupMembers(groupIDUint)

	// Format response to match frontend expectations
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"role": gin.H{
			"ID":          group.ID,
			"Name":        group.Name,
			"Description": group.Comments,
			"IsActive":    group.ValidID == 1,
			"Permissions": []string{}, // Groups don't have permissions in OTRS
		},
		"members": members,
	})
}

// handleUpdateGroup updates a group
func handleUpdateGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var groupForm struct {
		Name        string `form:"name" json:"name"`
		Description string `form:"description" json:"description"`
		ValidID     int    `form:"valid_id" json:"valid_id"`
	}

	if err := c.ShouldBind(&groupForm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	group, err := groupRepo.GetByID(uint(groupID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Get current user for audit fields
	userID := 1 // Default to system user
	if userCtx, ok := c.Get("user"); ok {
		if userData, ok := userCtx.(*models.User); ok && userData != nil {
			userID = int(userData.ID)
		}
	}

	// Update group fields
	if groupForm.Name != "" {
		group.Name = groupForm.Name
	}
	if groupForm.Description != "" {
		group.Comments = groupForm.Description
	}
	if groupForm.ValidID > 0 {
		group.ValidID = groupForm.ValidID
	}
	group.ChangeBy = userID

	if err := groupRepo.Update(group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"group":   group,
	})
}

// handleDeleteGroup deletes a group
func handleDeleteGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	group, err := groupRepo.GetByID(uint(groupID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Don't delete system groups
	if group.Name == "admin" || group.Name == "users" || group.Name == "stats" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete system groups"})
		return
	}

	// Get current user for audit fields
	userID := 1 // Default to system user
	if userCtx, ok := c.Get("user"); ok {
		if userData, ok := userCtx.(*models.User); ok && userData != nil {
			userID = int(userData.ID)
		}
	}

	// In OTRS style, we mark groups as invalid rather than deleting them
	group.ValidID = 2 // Mark as invalid
	group.ChangeBy = userID

	if err := groupRepo.Update(group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group deleted successfully",
	})
}

// handleAdminQueues shows the admin queues page
func handleAdminQueues(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get queues from database
	queueRepo := repository.NewQueueRepository(db)
	queues, err := queueRepo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch queues")
		return
	}

	// Get groups for dropdown
	var groups []gin.H
	groupRows, err := db.Query("SELECT id, name FROM groups WHERE valid_id = 1 ORDER BY name")
	if err == nil {
		defer groupRows.Close()
		for groupRows.Next() {
			var id int
			var name string
			if err := groupRows.Scan(&id, &name); err == nil {
				groups = append(groups, gin.H{"ID": id, "Name": name})
			}
		}
	}

	// For now, we'll use empty arrays for these as they may not exist in OTRS schema
	// These would typically come from system_address, salutation, and signature tables
	systemAddresses := []gin.H{}
	salutations := []gin.H{}
	signatures := []gin.H{}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/queues.pongo2", pongo2.Context{
		"Queues":          queues,
		"Groups":          groups,
		"SystemAddresses": systemAddresses,
		"Salutations":     salutations,
		"Signatures":      signatures,
		"User":            getUserMapForTemplate(c),
		"ActivePage":      "admin",
	})
}

// handleAdminPriorities shows the admin priorities page
func handleAdminPriorities(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get priorities from database
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name, color, valid_id
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY id
	`))
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch priorities")
		return
	}
	defer rows.Close()

	var priorities []gin.H
	for rows.Next() {
		var id, validID int
		var name string
		var color sql.NullString

		err := rows.Scan(&id, &name, &color, &validID)
		if err != nil {
			continue
		}

		priority := gin.H{
			"id":       id,
			"name":     name,
			"valid_id": validID,
		}

		if color.Valid {
			priority["color"] = color.String
		}

		priorities = append(priorities, priority)
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/priorities.pongo2", pongo2.Context{
		"Priorities": priorities,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// handleAdminLookups shows the admin lookups page
func handleAdminLookups(c *gin.Context) {
	// Get the current tab from query parameter
	currentTab := c.Query("tab")
	if currentTab == "" {
		currentTab = "priorities" // Default to priorities tab
	}

	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get various lookup data
	// Ticket States
	var ticketStates []gin.H
	rows, err := db.Query("SELECT id, name, type_id, comments FROM ticket_state WHERE valid_id = 1 ORDER BY name")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, typeID int
			var name string
			var comments sql.NullString
			rows.Scan(&id, &name, &typeID, &comments)

			state := gin.H{
				"ID":     id,
				"Name":   name,
				"TypeID": typeID,
			}
			if comments.Valid {
				state["Comments"] = comments.String
			}

			// Add type name for display
			var typeName string
			switch typeID {
			case 1:
				typeName = "New"
			case 2:
				typeName = "Open"
			case 3:
				typeName = "Pending"
			case 4:
				typeName = "Closed"
			default:
				typeName = "Unknown"
			}
			state["TypeName"] = typeName

			ticketStates = append(ticketStates, state)
		}
	}

	// Ticket Priorities
	var priorities []gin.H
	rows, err = db.Query("SELECT id, name, color FROM ticket_priority WHERE valid_id = 1 ORDER BY id")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			var color sql.NullString
			rows.Scan(&id, &name, &color)

			priority := gin.H{
				"ID":   id,
				"Name": name,
			}
			if color.Valid {
				priority["Color"] = color.String
			}

			priorities = append(priorities, priority)
		}
	}

	// Ticket Types
	var types []gin.H
	rows, err = db.Query("SELECT id, name, comments FROM ticket_type WHERE valid_id = 1 ORDER BY name")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			var comments sql.NullString
			rows.Scan(&id, &name, &comments)

			ticketType := gin.H{
				"ID":   id,
				"Name": name,
			}
			if comments.Valid {
				ticketType["Comments"] = comments.String
			}

			types = append(types, ticketType)
		}
	}

	// Services
	var services []gin.H
	rows, err = db.Query("SELECT id, name FROM service WHERE valid_id = 1 ORDER BY name")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var service gin.H
			var id int
			var name string
			rows.Scan(&id, &name)
			service = gin.H{"id": id, "name": name}
			services = append(services, service)
		}
	}

	// SLAs
	var slas []gin.H
	rows, err = db.Query("SELECT id, name FROM sla WHERE valid_id = 1 ORDER BY name")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sla gin.H
			var id int
			var name string
			rows.Scan(&id, &name)
			sla = gin.H{"id": id, "name": name}
			slas = append(slas, sla)
		}
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/lookups.pongo2", pongo2.Context{
		"TicketStates": ticketStates,
		"Priorities":   priorities,
		"TicketTypes":  types,
		"Services":     services,
		"SLAs":         slas,
		"User":         getUserMapForTemplate(c),
		"ActivePage":   "admin",
		"CurrentTab":   currentTab,
	})
}

// handleGetAuditLogs is defined in lookup_crud_handlers.go

// handleExportConfiguration is defined in lookup_crud_handlers.go

// handleImportConfiguration is defined in lookup_crud_handlers.go

// Advanced search handlers are defined in ticket_advanced_search_handler.go

/* Commented out - defined in ticket_advanced_search_handler.go
func handleAdvancedTicketSearch(c *gin.Context) {
	// Get search parameters
	searchParams := gin.H{
		"ticket_number": c.Query("ticket_number"),
		"subject":       c.Query("subject"),
		"body":          c.Query("body"),
		"customer":      c.Query("customer"),
		"agent":         c.Query("agent"),
		"queue":         c.Query("queue"),
		"priority":      c.Query("priority"),
		"state":         c.Query("state"),
		"created_from":  c.Query("created_from"),
		"created_to":    c.Query("created_to"),
		"updated_from":  c.Query("updated_from"),
		"updated_to":    c.Query("updated_to"),
	}

	// Mock search results
	results := []gin.H{
		{
			"id":       "T-2024-001",
			"subject":  "Advanced search result",
			"customer": "John Doe",
			"status":   "open",
			"priority": "high",
			"created":  "2024-01-10",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"params":  searchParams,
		"results": results,
		"total":   len(results),
	})
}

func handleSearchSuggestions(c *gin.Context) {
	query := c.Query("q")
	field := c.Query("field")

	suggestions := []string{}

	// Provide suggestions based on field
	switch field {
	case "customer":
		suggestions = []string{"John Doe", "Jane Smith", "Bob Johnson"}
	case "agent":
		suggestions = []string{"Alice Agent", "Bob Tech", "Charlie Support"}
	case "queue":
		suggestions = []string{"Support", "IT", "Network", "Billing"}
	default:
		// General suggestions
		if len(query) > 0 {
			suggestions = []string{
				query + " suggestion 1",
				query + " suggestion 2",
				query + " suggestion 3",
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
	})
}

func handleExportSearchResults(c *gin.Context) {
	format := c.Query("format") // csv, xlsx, pdf

	// Mock export data
	data := [][]string{
		{"Ticket ID", "Subject", "Customer", "Status", "Priority"},
		{"T-2024-001", "Email issue", "John Doe", "Open", "High"},
		{"T-2024-002", "Password reset", "Jane Smith", "Closed", "Medium"},
	}

	switch format {
	case "csv":
		c.Header("Content-Disposition", "attachment; filename=\"search-results.csv\"")
		c.Header("Content-Type", "text/csv")

		// Write CSV data
		for _, row := range data {
			c.Writer.WriteString(strings.Join(row, ",") + "\n")
		}
	default:
		c.JSON(http.StatusOK, gin.H{
			"format": format,
			"data":   data,
		})
	}
}

func handleSaveSearchHistory(c *gin.Context) {
	var searchData gin.H
	if err := c.ShouldBindJSON(&searchData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save to history (in memory for demo)
	historyEntry := gin.H{
		"id":        fmt.Sprintf("SH-%d", time.Now().Unix()),
		"query":     searchData["query"],
		"filters":   searchData["filters"],
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"entry":   historyEntry,
	})
}

func handleGetSearchHistory(c *gin.Context) {
	// Mock search history
	history := []gin.H{
		{
			"id":        "SH-1",
			"query":     "email problem",
			"filters":   gin.H{"status": "open", "priority": "high"},
			"timestamp": "2024-01-10 14:30",
		},
		{
			"id":        "SH-2",
			"query":     "password reset",
			"filters":   gin.H{"queue": "Support"},
			"timestamp": "2024-01-10 13:15",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
	})
}

func handleDeleteSearchHistory(c *gin.Context) {
	historyID := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Search history %s deleted", historyID),
	})
}

func handleCreateSavedSearch(c *gin.Context) {
	var searchData gin.H
	if err := c.ShouldBindJSON(&searchData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	savedSearch := gin.H{
		"id":          fmt.Sprintf("SS-%d", time.Now().Unix()),
		"name":        searchData["name"],
		"description": searchData["description"],
		"query":       searchData["query"],
		"filters":     searchData["filters"],
		"created":     time.Now().Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"search":  savedSearch,
	})
}

func handleGetSavedSearches(c *gin.Context) {
	// Mock saved searches
	searches := []gin.H{
		{
			"id":          "SS-1",
			"name":        "Open High Priority",
			"description": "All open tickets with high priority",
			"query":       "",
			"filters":     gin.H{"status": "open", "priority": "high"},
			"created":     "2024-01-01",
		},
		{
			"id":          "SS-2",
			"name":        "My Tickets",
			"description": "Tickets assigned to me",
			"query":       "",
			"filters":     gin.H{"assigned": "me"},
			"created":     "2024-01-05",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"searches": searches,
	})
}
*/ // End of commented out duplicates

/* Commented out - defined in ticket_advanced_search_handler.go
func handleExecuteSavedSearch(c *gin.Context) {
	searchID := c.Param("id")

	// Mock execution results
	results := []gin.H{
		{
			"id":       "T-2024-001",
			"subject":  "Saved search result",
			"status":   "open",
			"priority": "high",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"searchId": searchID,
		"results":  results,
		"total":    len(results),
	})
}

func handleUpdateSavedSearch(c *gin.Context) {
	searchID := c.Param("id")

	var updates gin.H
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"searchId": searchID,
		"updated": time.Now().Format("2006-01-02 15:04:05"),
	})
}

func handleDeleteSavedSearch(c *gin.Context) {
	searchID := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Saved search %s deleted", searchID),
	})
}
*/ // End of saved search duplicates

// Ticket merge handlers are defined in ticket_merge_handler.go

/* Commented out - defined in ticket_merge_handler.go
func handleMergeTickets(c *gin.Context) {
	mainTicketID := c.Param("id")

	var mergeRequest struct {
		TicketIDs []string `json:"ticket_ids"`
	}

	if err := c.ShouldBindJSON(&mergeRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"mainTicket":   mainTicketID,
		"mergedTickets": mergeRequest.TicketIDs,
		"message":      fmt.Sprintf("Successfully merged %d tickets", len(mergeRequest.TicketIDs)),
	})
}

func handleUnmergeTicket(c *gin.Context) {
	ticketID := c.Param("id")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"ticketId": ticketID,
		"message": "Ticket unmerged successfully",
	})
}

func handleGetMergeHistory(c *gin.Context) {
	ticketID := c.Param("id")

	history := []gin.H{
		{
			"id":        "MH-1",
			"action":    "merged",
			"tickets":   []string{"T-2024-002", "T-2024-003"},
			"user":      "admin",
			"timestamp": "2024-01-10 10:00",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"ticketId": ticketID,
		"history":  history,
	})
}
*/ // End of merge handler duplicates

// Permission Management handlers

// handleAdminPermissions displays the permission management page
func handleAdminPermissions(c *gin.Context) {
	// Prevent caching of this page
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get all users
	userRepo := repository.NewUserRepository(db)
	users, err := userRepo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch users")
		return
	}

	// Get selected user ID from query param
	selectedUserIDStr := c.Query("user")
	var selectedUserID uint
	if selectedUserIDStr != "" {
		if id, err := strconv.ParseUint(selectedUserIDStr, 10, 32); err == nil {
			selectedUserID = uint(id)
		}
	}

	// If a user is selected, get their permission matrix
	var permissionMatrix *service.PermissionMatrix
	if selectedUserID > 0 {
		permService := service.NewPermissionService(db)
		permissionMatrix, err = permService.GetUserPermissionMatrix(selectedUserID)
		if err != nil {
			// Log error but don't fail the page
			log.Printf("Failed to get permission matrix for user %d: %v", selectedUserID, err)
		} else if permissionMatrix != nil {
			log.Printf("Got permission matrix for user %d: %d groups", selectedUserID, len(permissionMatrix.Groups))
			// Debug: Log OBC permissions specifically
			for _, gp := range permissionMatrix.Groups {
				if gp.Group.Name == "OBC" {
					log.Printf("OBC permissions for user %d: %+v", selectedUserID, gp.Permissions)
				}
			}
		} else {
			log.Printf("Permission matrix is nil for user %d", selectedUserID)
		}
	}

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/permissions.pongo2", pongo2.Context{
		"Users":            users,
		"SelectedUserID":   selectedUserID,
		"PermissionMatrix": permissionMatrix,
		"User":             getUserMapForTemplate(c),
		"ActivePage":       "admin",
	})
}

// handleGetUserPermissionMatrix returns the permission matrix for a user
func handleGetUserPermissionMatrix(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	permService := service.NewPermissionService(db)
	matrix, err := permService.GetUserPermissionMatrix(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    matrix,
	})
}

// handleUpdateUserPermissions updates all permissions for a user
func handleUpdateUserPermissions(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	// Parse permission data from form
	permissions := make(map[uint]map[string]bool)

	// Parse form data - handle both multipart and urlencoded
	var formValues map[string][]string

	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		// Parse multipart form
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid multipart form data"})
			return
		}
		formValues = c.Request.MultipartForm.Value
	} else {
		// Parse URL-encoded form
		if err := c.Request.ParseForm(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid form data"})
			return
		}
		formValues = c.Request.PostForm
	}

	// Debug: Log all received form data
	log.Printf("DEBUG: Received form data for user %d (Content-Type: %s):", userID, contentType)
	for key, values := range formValues {
		if strings.HasPrefix(key, "perm_") {
			log.Printf("  %s = %v", key, values)
		}
	}

	// First, collect all groups that have checkboxes
	groupsWithCheckboxes := make(map[uint]bool)

	// Process each permission checkbox
	// Format: perm_<groupID>_<permissionKey>
	for key, values := range formValues {
		if strings.HasPrefix(key, "perm_") && len(values) > 0 {
			// Split into exactly 3 parts to handle permission keys with underscores (e.g., "move_into")
			parts := strings.SplitN(key, "_", 3)
			if len(parts) == 3 {
				groupID, _ := strconv.ParseUint(parts[1], 10, 32)
				permKey := parts[2]

				groupsWithCheckboxes[uint(groupID)] = true

				if permissions[uint(groupID)] == nil {
					permissions[uint(groupID)] = make(map[string]bool)
				}
				permissions[uint(groupID)][permKey] = (values[0] == "1" || values[0] == "on")
			}
		}
	}

	// Ensure all groups with checkboxes have all permission keys
	for groupID := range groupsWithCheckboxes {
		if permissions[groupID] == nil {
			permissions[groupID] = make(map[string]bool)
		}
		// Ensure all permission keys exist (default to false if not set)
		for _, key := range []string{"ro", "move_into", "create", "note", "owner", "priority", "rw"} {
			if _, exists := permissions[groupID][key]; !exists {
				permissions[groupID][key] = false
			}
		}
	}

	// Debug log
	log.Printf("DEBUG: Updating permissions for user %d, received %d groups with checkboxes", userID, len(groupsWithCheckboxes))
	for gid, perms := range permissions {
		hasAny := false
		for _, v := range perms {
			if v {
				hasAny = true
				break
			}
		}
		log.Printf("  Group %d: has permissions=%v", gid, hasAny)
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	permService := service.NewPermissionService(db)
	if err := permService.UpdateUserPermissions(uint(userID), permissions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update permissions"})
		return
	}

	// Always return JSON for this endpoint since it's called via AJAX
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Permissions updated successfully",
	})
}

// handleGetGroupPermissionMatrix gets all users' permissions for a group
func handleGetGroupPermissionMatrix(c *gin.Context) {
	groupIDStr := c.Param("groupId")
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	permService := service.NewPermissionService(db)
	matrix, err := permService.GetGroupPermissionMatrix(uint(groupID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    matrix,
	})
}

// handleCloneUserPermissions copies permissions from one user to another
func handleCloneUserPermissions(c *gin.Context) {
	sourceUserID, _ := strconv.ParseUint(c.PostForm("source_user_id"), 10, 32)
	targetUserID, _ := strconv.ParseUint(c.PostForm("target_user_id"), 10, 32)

	if sourceUserID == 0 || targetUserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user IDs"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	permService := service.NewPermissionService(db)
	if err := permService.CloneUserPermissions(uint(sourceUserID), uint(targetUserID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to clone permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Permissions cloned successfully",
	})
}

// Group user management handlers (now properly named for groups, not roles)

// handleGetGroupUsers returns users assigned to a group
func handleGetGroupUsers(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)

	// Get the group details
	group, err := groupRepo.GetByID(uint(groupID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Group not found"})
		return
	}

	// Get members of this group
	members, err := groupRepo.GetGroupMembers(uint(groupID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch group members"})
		return
	}

	// Get all users for the "available users" list
	userRepo := repository.NewUserRepository(db)
	allUsers, err := userRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch users"})
		return
	}

	// Filter out users who are already members
	memberIDs := make(map[uint]bool)
	for _, member := range members {
		memberIDs[member.ID] = true
	}

	availableUsers := make([]*models.User, 0)
	for _, user := range allUsers {
		if !memberIDs[user.ID] && user.ValidID == 1 {
			availableUsers = append(availableUsers, user)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"group": gin.H{
			"id":          group.ID,
			"name":        group.Name,
			"description": group.Comments,
		},
		"members":         members,
		"available_users": availableUsers,
	})
}

// handleAddUserToGroup assigns a user to a group
func handleAddUserToGroup(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}

	var req struct {
		UserID uint `form:"user_id" json:"user_id" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request data"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)

	// Add user to group
	err = groupRepo.AddUserToGroup(req.UserID, uint(groupID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to add user to group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User assigned to group successfully",
	})
}

// handleRemoveUserFromGroup removes a user from a group
func handleRemoveUserFromGroup(c *gin.Context) {
	groupIDStr := c.Param("id")
	userIDStr := c.Param("userId")

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)

	// Remove user from group
	err = groupRepo.RemoveUserFromGroup(uint(userID), uint(groupID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to remove user from group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User removed from group successfully",
	})
}

// handleCustomerSearch handles customer search for autocomplete
func handleCustomerSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Search for customers by login, email, first name, or last name
	// Using ILIKE for case-insensitive search and supporting wildcard *
	searchTerm := strings.ReplaceAll(query, "*", "%")
	if !strings.Contains(searchTerm, "%") {
		searchTerm = "%" + searchTerm + "%"
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, login, email, first_name, last_name, customer_id
		FROM customer_user
		WHERE valid_id = 1
		  AND (login ILIKE $1
		       OR email ILIKE $1
		       OR first_name ILIKE $1
		       OR last_name ILIKE $1
		       OR CONCAT(first_name, ' ', last_name) ILIKE $1)
		LIMIT 10`),
		searchTerm)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search customers"})
		return
	}
	defer rows.Close()

	var customers []gin.H
	for rows.Next() {
		var id int
		var login, email, firstName, lastName, customerID string
		err := rows.Scan(&id, &login, &email, &firstName, &lastName, &customerID)
		if err != nil {
			continue
		}

		customers = append(customers, gin.H{
			"id":          id,
			"login":       login,
			"email":       email,
			"first_name":  firstName,
			"last_name":   lastName,
			"full_name":   firstName + " " + lastName,
			"customer_id": customerID,
			"display":     fmt.Sprintf("%s %s (%s)", firstName, lastName, email),
		})
	}

	if customers == nil {
		customers = []gin.H{}
	}

	c.JSON(http.StatusOK, customers)
}

// getPriorityLabel returns a human-readable priority label
func getPriorityLabel(priorityID int) string {
	switch priorityID {
	case 1:
		return "very low"
	case 2:
		return "low"
	case 3:
		return "normal"
	case 4:
		return "high"
	case 5:
		return "very high"
	default:
		return "unknown"
	}
}

// handleGetGroups returns all groups as JSON for API requests
func handleGetGroups(c *gin.Context) {
	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Query for all groups
	query := `
		SELECT id, name, valid_id
		FROM groups
		WHERE valid_id = 1
		ORDER BY name`

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch groups",
		})
		return
	}
	defer rows.Close()

	groups := []map[string]interface{}{}
	for rows.Next() {
		var id, validID int
		var name string
		err := rows.Scan(&id, &name, &validID)
		if err != nil {
			continue
		}

		group := map[string]interface{}{
			"id":       id,
			"name":     name,
			"valid_id": validID,
		}
		groups = append(groups, group)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"groups":  groups,
	})
}

// handleGetGroupMembers returns group members as JSON for API requests
func handleGetGroupMembers(c *gin.Context) {
	groupID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Query for group members
	query := `
		SELECT DISTINCT u.id, u.login, u.first_name, u.last_name
		FROM users u
		INNER JOIN group_user gu ON u.id = gu.user_id
		WHERE gu.group_id = $1 AND u.valid_id = 1
		ORDER BY u.id`

	rows, err := db.Query(query, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch group members",
		})
		return
	}
	defer rows.Close()

	members := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var login, firstName, lastName sql.NullString
		err := rows.Scan(&id, &login, &firstName, &lastName)
		if err != nil {
			continue
		}

		member := map[string]interface{}{
			"id":         id,
			"login":      login.String,
			"first_name": firstName.String,
			"last_name":  lastName.String,
		}
		members = append(members, member)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    members,
	})
}

// handleGetGroupAPI returns group details as JSON for API requests
func handleGetGroupAPI(c *gin.Context) {
	groupID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Query for group details
	var id int
	var name, comments sql.NullString
	var validID sql.NullInt32

	query := `SELECT id, name, comments, valid_id FROM groups WHERE id = $1`
	err = db.QueryRow(query, groupID).Scan(&id, &name, &comments, &validID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Group not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch group",
			})
		}
		return
	}

	group := map[string]interface{}{
		"id":       id,
		"name":     name.String,
		"comments": comments.String,
		"valid_id": validID.Int32,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    group,
	})
}

// handleClaudeChatDemo shows the Claude chat demo page
func handleClaudeChatDemo(c *gin.Context) {
	pongo2Renderer.HTML(c, http.StatusOK, "pages/claude_chat_demo.pongo2", pongo2.Context{
		"User":       getUserMapForTemplate(c),
		"ActivePage": "demo",
		"Title":      "Claude Chat Demo",
	})
}

// handleClaudeFeedback handles feedback from the Claude Code chat component and creates tickets
func handleClaudeFeedback(c *gin.Context) {
	var feedback struct {
		Message string `json:"message"`
		Context struct {
			Page             string `json:"page"`
			URL              string `json:"url"`
			CurrentURL       string `json:"currentUrl"`  // Added field
			CurrentPath      string `json:"currentPath"` // Added field
			PageTitle        string `json:"pageTitle"`   // Added field
			Timestamp        string `json:"timestamp"`
			UserAgent        string `json:"userAgent"`
			ScreenResolution string `json:"screenResolution"`
			ViewportSize     string `json:"viewportSize"`
			User             string `json:"user"`
			MousePosition    struct {
				X int `json:"x"`
				Y int `json:"y"`
			} `json:"mousePosition"`
			SelectedElement *struct {
				Selector  string `json:"selector"`
				TagName   string `json:"tagName"`
				ID        string `json:"id"`
				ClassName string `json:"className"`
				Text      string `json:"text"`
				Position  struct {
					Top    float64 `json:"top"`
					Left   float64 `json:"left"`
					Width  float64 `json:"width"`
					Height float64 `json:"height"`
				} `json:"position"`
				Attributes []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"attributes"`
			} `json:"selectedElement"`
			Forms  []interface{} `json:"forms"`
			Errors []string      `json:"errors"`
			Tables []struct {
				ID      string `json:"id"`
				Rows    int    `json:"rows"`
				Columns int    `json:"columns"`
			} `json:"tables"`
		} `json:"context"`
		Timestamp string `json:"timestamp"`
	}

	if err := c.ShouldBindJSON(&feedback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid feedback format",
		})
		return
	}

	// Log the feedback with full context
	log.Printf("===== CLAUDE CODE FEEDBACK =====")
	log.Printf("Message: %s", feedback.Message)
	log.Printf("Page: %s", feedback.Context.Page)
	log.Printf("URL: %s", feedback.Context.URL)
	log.Printf("User: %s", feedback.Context.User)
	log.Printf("Timestamp: %s", feedback.Timestamp)

	if feedback.Context.SelectedElement != nil {
		log.Printf("Selected Element: %s", feedback.Context.SelectedElement.Selector)
		log.Printf("  Tag: %s, ID: %s, Class: %s",
			feedback.Context.SelectedElement.TagName,
			feedback.Context.SelectedElement.ID,
			feedback.Context.SelectedElement.ClassName)
		log.Printf("  Position: top=%f, left=%f, width=%f, height=%f",
			feedback.Context.SelectedElement.Position.Top,
			feedback.Context.SelectedElement.Position.Left,
			feedback.Context.SelectedElement.Position.Width,
			feedback.Context.SelectedElement.Position.Height)
	}

	if len(feedback.Context.Errors) > 0 {
		log.Printf("Page Errors: %v", feedback.Context.Errors)
	}

	log.Printf("Mouse Position: x=%d, y=%d",
		feedback.Context.MousePosition.X,
		feedback.Context.MousePosition.Y)
	log.Printf("================================")

	// Create a ticket in the Claude Code queue
	db, err := database.GetDB()
	if err != nil {
		log.Printf("Failed to get database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Generate ticket number (format: YYYYMMDDHHMMSS)
	ticketNumber := time.Now().Format("20060102150405")

	// Build ticket title
	title := fmt.Sprintf("Claude Code: %s", feedback.Message)
	if len(title) > 255 {
		title = title[:252] + "..."
	}

	// Build detailed description with context
	var description strings.Builder
	description.WriteString(fmt.Sprintf("Message: %s\n\n", feedback.Message))

	// Use CurrentURL/CurrentPath if available, fallback to URL/Page
	if feedback.Context.CurrentURL != "" {
		description.WriteString(fmt.Sprintf("Current URL: %s\n", feedback.Context.CurrentURL))
	} else if feedback.Context.URL != "" {
		description.WriteString(fmt.Sprintf("URL: %s\n", feedback.Context.URL))
	}

	if feedback.Context.CurrentPath != "" {
		description.WriteString(fmt.Sprintf("Current Path: %s\n", feedback.Context.CurrentPath))
	} else if feedback.Context.Page != "" {
		description.WriteString(fmt.Sprintf("Page: %s\n", feedback.Context.Page))
	}

	if feedback.Context.PageTitle != "" {
		description.WriteString(fmt.Sprintf("Page Title: %s\n", feedback.Context.PageTitle))
	}

	description.WriteString(fmt.Sprintf("Timestamp: %s\n", feedback.Timestamp))
	description.WriteString(fmt.Sprintf("User Agent: %s\n", feedback.Context.UserAgent))
	description.WriteString(fmt.Sprintf("Screen: %s, Viewport: %s\n",
		feedback.Context.ScreenResolution, feedback.Context.ViewportSize))

	if feedback.Context.SelectedElement != nil {
		description.WriteString("\n=== Selected Element ===\n")
		description.WriteString(fmt.Sprintf("Selector: %s\n", feedback.Context.SelectedElement.Selector))
		description.WriteString(fmt.Sprintf("Tag: %s, ID: %s, Class: %s\n",
			feedback.Context.SelectedElement.TagName,
			feedback.Context.SelectedElement.ID,
			feedback.Context.SelectedElement.ClassName))
		description.WriteString(fmt.Sprintf("Position: top=%f, left=%f, width=%f, height=%f\n",
			feedback.Context.SelectedElement.Position.Top,
			feedback.Context.SelectedElement.Position.Left,
			feedback.Context.SelectedElement.Position.Width,
			feedback.Context.SelectedElement.Position.Height))
		if feedback.Context.SelectedElement.Text != "" {
			description.WriteString(fmt.Sprintf("Text: %s\n", feedback.Context.SelectedElement.Text))
		}
	}

	if len(feedback.Context.Errors) > 0 {
		description.WriteString("\n=== Page Errors ===\n")
		for _, err := range feedback.Context.Errors {
			description.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	description.WriteString(fmt.Sprintf("\nMouse Position: x=%d, y=%d\n",
		feedback.Context.MousePosition.X,
		feedback.Context.MousePosition.Y))

	// Get current user ID or default to 1 (admin)
	userID := 1
	if userVal, exists := c.Get("user_id"); exists {
		if uid, ok := userVal.(uint); ok {
			userID = int(uid)
		}
	}

	// Create ticket in database
	var ticketID int64
	err = db.QueryRow(database.ConvertPlaceholders(`
		INSERT INTO ticket (
			tn, title, queue_id, ticket_lock_id, type_id,
			user_id, responsible_user_id, ticket_priority_id, ticket_state_id,
			customer_id, customer_user_id,
			create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, 14, 1, 1,
			$3, $3, 3, 1,
			'Claude Code', $4,
			CURRENT_TIMESTAMP, $3, CURRENT_TIMESTAMP, $3
		) RETURNING id`),
		ticketNumber, title, userID, feedback.Context.User).Scan(&ticketID)

	if err != nil {
		log.Printf("Failed to create ticket: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create ticket",
		})
		return
	}

	// Create article first (without content - that goes in article_data_mime)
	var articleID int64
	err = db.QueryRow(database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id, article_type_id, article_sender_type_id,
			communication_channel_id, is_visible_for_customer,
			create_time, create_by, change_time, change_by
		) VALUES (
			$1, 1, 3,
			1, 1,
			CURRENT_TIMESTAMP, $2, CURRENT_TIMESTAMP, $2
		) RETURNING id`),
		ticketID, userID).Scan(&articleID)

	if err != nil {
		log.Printf("Failed to create article: %v", err)
	} else {
		// Now create the article_data_mime entry with the actual content and context
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (
				article_id, a_from, a_to, a_subject, a_body,
				a_content_type, incoming_time,
				create_time, create_by, change_time, change_by
			) VALUES (
				$1, $2, 'Claude Code Queue', $3, $4,
				'text/plain; charset=utf-8', 0,
				CURRENT_TIMESTAMP, $5, CURRENT_TIMESTAMP, $5
			)`),
			articleID,
			feedback.Context.User,
			title,
			[]byte(description.String()), // a_body is bytea type
			userID)

		if err != nil {
			log.Printf("Failed to create article_data_mime: %v", err)
		}
	}

	log.Printf("Created ticket #%s (ID: %d) in Claude Code queue", ticketNumber, ticketID)

	// Return success with ticket number
	response := fmt.Sprintf("Ticket #%s created! I'll review this issue. ", ticketNumber)

	if feedback.Context.SelectedElement != nil {
		response += fmt.Sprintf("I can see you're pointing at '%s'. ",
			feedback.Context.SelectedElement.Selector)
	}

	response += "You can track progress in the Claude Code queue."

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"response":      response,
		"ticket_number": ticketNumber,
		"ticket_id":     ticketID,
	})
}
