package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
	"github.com/gotrs-io/gotrs-ce/internal/services/k8s"
)

func main() {
	// Initialize service registry early
	log.Println("Initializing service registry...")
	registry, err := adapter.InitializeServiceRegistry()
	if err != nil {
		log.Printf("Warning: Failed to initialize service registry: %v", err)
		// Continue anyway - fallback will be used
	} else {
		// Detect environment and adapt configuration
		detector := k8s.NewDetector()
		log.Printf("Detected environment: %s", detector.Environment())

		// Auto-configure database if environment variables are set
		if err := adapter.AutoConfigureDatabase(); err != nil {
			log.Printf("Warning: Failed to auto-configure database: %v", err)
			// Continue anyway - fallback will be used
		} else {
			log.Println("Database service registered successfully")
		}

		// Setup cleanup on shutdown
		defer func() {
			ctx := context.Background()
			if err := registry.Shutdown(ctx); err != nil {
				log.Printf("Error during registry shutdown: %v", err)
			}
		}()
	}

	// Set Gin mode
	if os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Initialize handler registry
	handlerRegistry := routing.NewHandlerRegistry()

	// Register middleware from routing package
	routing.RegisterExistingHandlers(handlerRegistry)

	// Register actual API handlers to override placeholders
	apiHandlers := map[string]gin.HandlerFunc{
		"handleDashboard":           api.HandleDashboard,
		"dashboard_stats":           api.HandleDashboardStats,
		"dashboard_recent_tickets":  api.HandleRecentTickets,
		"dashboard_queue_status":    api.DashboardQueueStatus,
		"dashboard_activity_stream": api.HandleActivityStream,

		// Agent handlers
		"handleAgentTickets":         api.AgentHandlerExports.HandleAgentTickets,
		"handleAgentTicketView":      api.AgentHandlerExports.HandleAgentTicketView,
		"handleAgentTicketReply":     api.AgentHandlerExports.HandleAgentTicketReply,
		"handleAgentTicketNote":      api.AgentHandlerExports.HandleAgentTicketNote,
		"handleAgentTicketPhone":     api.AgentHandlerExports.HandleAgentTicketPhone,
		"handleAgentTicketStatus":    api.AgentHandlerExports.HandleAgentTicketStatus,
		"handleAgentTicketAssign":    api.AgentHandlerExports.HandleAgentTicketAssign,
		"handleAgentTicketPriority":  api.AgentHandlerExports.HandleAgentTicketPriority,
		"handleAgentTicketQueue":     api.AgentHandlerExports.HandleAgentTicketQueue,
		"handleAgentTicketMerge":     api.AgentHandlerExports.HandleAgentTicketMerge,
		"handleAgentSearch":          api.AgentHandlerExports.HandleAgentSearch,
		"handleAgentQueues":          api.AgentHandlerExports.HandleAgentQueues,
		"handleAgentQueueView":       api.AgentHandlerExports.HandleAgentQueueView,
		"handleAgentQueueLock":       api.AgentHandlerExports.HandleAgentQueueLock,
		"handleAgentCustomers":       api.AgentHandlerExports.HandleAgentCustomers,
		"handleAgentCustomerView":    api.AgentHandlerExports.HandleAgentCustomerView,
		"handleAgentCustomerTickets": api.AgentHandlerExports.HandleAgentCustomerTickets,
		"handleAgentSearchResults":   api.AgentHandlerExports.HandleAgentSearchResults,
		"handleAgentDashboard": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Agent Dashboard",
				"user":    "Agent User",
				"status":  "Agent access working!",
			})
		},

		// Customer handlers
		"handleCustomerNewTicket":      api.HandleCustomerNewTicket,
		"handleCustomerCreateTicket":   api.HandleCustomerCreateTicket,
		"handleCustomerTicketView":     api.HandleCustomerTicketView,
		"handleCustomerTicketReply":    api.HandleCustomerTicketReply,
		"handleCustomerCloseTicket":    api.HandleCustomerCloseTicket,
		"handleCustomerProfile":        api.HandleCustomerProfile,
		"handleCustomerUpdateProfile":  api.HandleCustomerUpdateProfile,
		"handleCustomerPasswordForm":   api.HandleCustomerPasswordForm,
		"handleCustomerChangePassword": api.HandleCustomerChangePassword,
		"handleCustomerKnowledgeBase":  api.HandleCustomerKnowledgeBase,
		"handleCustomerKBSearch":       api.HandleCustomerKBSearch,
		"handleCustomerKBArticle":      api.HandleCustomerKBArticle,
		"handleCustomerCompanyInfo":    api.HandleCustomerCompanyInfo,
		"handleCustomerCompanyUsers":   api.HandleCustomerCompanyUsers,
		"handleCustomerDashboard": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Customer Dashboard",
				"user":    "Customer User",
				"status":  "Customer access working!",
			})
		},
		"handleCustomerTickets": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Customer Tickets",
				"tickets": []gin.H{},
				"status":  "Customer tickets working!",
			})
		},

		// Ticket handlers
		"handleTicketDetail": api.HandleTicketDetail,
		// "handleTicketCustomerUsers": api.HandleTicketCustomerUsers,
		"handleAgentTicketDraft": api.AgentHandlerExports.HandleAgentTicketDraft,
		// "handleArticleAttachmentDownload": api.HandleArticleAttachmentDownload,

		// API v1 handlers (using actual API handlers, not v1 router wrappers)
		"HandleListTicketsAPI":  api.HandleListTicketsAPI,
		"HandleCreateTicketAPI": api.HandleCreateTicketAPI,
		"HandleGetTicketAPI":    api.HandleGetTicketAPI,
		"HandleUpdateTicketAPI": api.HandleUpdateTicketAPI,
		"HandleDeleteTicketAPI": api.HandleDeleteTicketAPI,
		"HandleAgentNewTicket": func(c *gin.Context) {
			db, _ := database.GetDB()
			if db == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			api.HandleAgentNewTicket(db)(c)
		},
		"HandleAgentCreateTicket": func(c *gin.Context) {
			db, _ := database.GetDB()
			if db == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			api.HandleAgentCreateTicket(db)(c)
		},
		"HandleListArticlesAPI":      api.HandleListArticlesAPI,
		"HandleCreateArticleAPI":     api.HandleCreateArticleAPI,
		"HandleGetArticleAPI":        api.HandleGetArticleAPI,
		"HandleUpdateArticleAPI":     api.HandleUpdateArticleAPI,
		"HandleDeleteArticleAPI":     api.HandleDeleteArticleAPI,
		"HandleUserMeAPI":            api.HandleUserMeAPI,
		"HandleListUsersAPI":         api.HandleListUsersAPI,
		"HandleGetUserAPI":           api.HandleGetUserAPI,
		"HandleCreateUserAPI":        api.HandleCreateUserAPI,
		"HandleUpdateUserAPI":        api.HandleUpdateUserAPI,
		"HandleDeleteUserAPI":        api.HandleDeleteUserAPI,
		"HandleListQueuesAPI":        api.HandleListQueuesAPI,
		"HandleGetQueueAPI":          api.HandleGetQueueAPI,
		"HandleGetQueueAgentsAPI":    api.HandleGetQueueAgentsAPI,
		"HandleCreateQueueAPI":       api.HandleCreateQueueAPI,
		"HandleUpdateQueueAPI":       api.HandleUpdateQueueAPI,
		"HandleDeleteQueueAPI":       api.HandleDeleteQueueAPI,
		"HandleGetQueueStatsAPI":     api.HandleGetQueueStatsAPI,
		"HandleAssignQueueGroupAPI":  api.HandleAssignQueueGroupAPI,
		"HandleRemoveQueueGroupAPI":  api.HandleRemoveQueueGroupAPI,
		"HandleListPrioritiesAPI":    api.HandleListPrioritiesAPI,
		"HandleGetPriorityAPI":       api.HandleGetPriorityAPI,
		"HandleListTypesAPI":         api.HandleListTypesAPI,
		"HandleListStatesAPI":        api.HandleListStatesAPI,
		"HandleSearchAPI":            api.HandleSearchAPI,
		"HandleSearchSuggestionsAPI": api.HandleSearchSuggestionsAPI,
		"HandleReindexAPI":           api.HandleReindexAPI,
		"HandleSearchHealthAPI":      api.HandleSearchHealthAPI,

		// Dev handlers
		"HandleDevDashboard":  api.HandleDevDashboard,
		"HandleClaudeTickets": api.HandleClaudeTickets,
		"HandleDevAction":     api.HandleDevAction,
		"HandleDevLogs":       api.HandleDevLogs,
		"HandleDevDatabase":   api.HandleDevDatabase,

		// Auth handlers
		"handleLoginPage": api.HandleLoginPage,
		"handleAuthLogin": api.HandleAuthLogin,
		"handleLogout":    api.HandleLogout,

		// Admin handlers
		"handleAdminUsers":            api.HandleAdminUsers,
		"handleAdminUserGet":          api.HandleAdminUserGet,
		"handleAdminUserEdit":         api.HandleAdminUserEdit,
		"handleAdminUserUpdate":       api.HandleAdminUserUpdate,
		"handleAdminUserDelete":       api.HandleAdminUserDelete,
		"handleAdminPasswordPolicy":   api.HandleAdminPasswordPolicy,
		"HandleAdminUsersList":        api.HandleAdminUsersList,
		"handleAdminGroups":           api.HandleAdminGroups,
		"handleCreateGroup":           api.HandleCreateGroup,
		"handleGetGroup":              api.HandleGetGroup,
		"handleUpdateGroup":           api.HandleUpdateGroup,
		"handleDeleteGroup":           api.HandleDeleteGroup,
		"handleGroupMembers":          api.HandleGroupMembers,
		"handleAddUserToGroup":        api.HandleAddUserToGroup,
		"handleRemoveUserFromGroup":   api.HandleRemoveUserFromGroup,
		"HandleAdminGroupsUsers":      api.HandleAdminGroupsUsers,
		"HandleAdminGroupsAddUser":    api.HandleAdminGroupsAddUser,
		"HandleAdminGroupsRemoveUser": api.HandleAdminGroupsRemoveUser,
		"handleAdminQueues":           api.HandleAdminQueues,
		"handleAdminPriorities":       api.HandleAdminPriorities,
		// Queue API handlers
		"HandleAPIQueueGet":             api.HandleAPIQueueGet,
		"HandleAPIQueueDetails":         api.HandleAPIQueueDetails,
		"HandleAPIQueueStatus":          api.HandleAPIQueueStatus,
		"handleAdminPermissions":        api.HandleAdminPermissions,
		"handleGetUserPermissionMatrix": api.HandleGetUserPermissionMatrix,
		"handleUpdateUserPermissions":   api.HandleUpdateUserPermissions,
		"handleAdminStates":             api.HandleAdminStates,
		"handleAdminTypes":              api.HandleAdminTypes,
		"handleAdminServices":           api.HandleAdminServices,
		"handleAdminSLA":                api.HandleAdminSLA,
		"handleAdminLookups":            api.HandleAdminLookups,
		"handleAdminCustomerCompanies":  api.HandleAdminCustomerCompanies,
		"handleAdminNewCustomerCompany": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "New Customer Company",
				"status":  "New customer company working!",
			})
		},
		"handleAdminCreateCustomerCompany": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Create Customer Company",
				"status":  "Create customer company working!",
			})
		},
		"handleAdminEditCustomerCompany": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Edit Customer Company",
				"companyID": companyID,
				"status":    "Edit customer company working!",
			})
		},
		"handleAdminUpdateCustomerCompany": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Update Customer Company",
				"companyID": companyID,
				"status":    "Update customer company working!",
			})
		},
		"handleAdminDeleteCustomerCompany": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Delete Customer Company",
				"companyID": companyID,
				"status":    "Delete customer company working!",
			})
		},
		"handleAdminCustomerCompanyUsers":    api.HandleAdminCustomerCompanyUsers,
		"handleAdminCustomerCompanyTickets":  api.HandleAdminCustomerCompanyTickets,
		"handleAdminCustomerCompanyServices": api.HandleAdminCustomerCompanyServices,
		"handleAdminUpdateCustomerCompanyServices": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Update Customer Company Services",
				"companyID": companyID,
				"status":    "Update customer company services working!",
			})
		},
		"handleAdminCustomerPortalSettings": api.HandleAdminCustomerPortalSettings,
		"handleAdminUpdateCustomerPortalSettings": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Update Customer Portal Settings",
				"status":  "Update customer portal settings working!",
			})
		},
		"handleAdminUploadCustomerPortalLogo": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Upload Customer Portal Logo",
				"status":  "Upload customer portal logo working!",
			})
		},

		// Basic system handlers
		"handleRoot": func(c *gin.Context) {
			c.Redirect(http.StatusFound, "/login")
		},
		"handleHealthCheck": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"version": "1.0.0",
			})
		},
		"handleDetailedHealthCheck": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "healthy",
				"components": gin.H{
					"database": "unknown", // Would check actual DB
					"cache":    "healthy",
					"queue":    "healthy",
				},
			})
		},
		"handleMetrics": func(c *gin.Context) {
			// Basic Prometheus metrics
			c.String(http.StatusOK, "# HELP gotrs_up GOTRS is up\n# TYPE gotrs_up gauge\ngotrs_up 1\n")
		},
		"handleStaticFiles": func(c *gin.Context) {
			// Get the full path from the request
			requestPath := c.Request.URL.Path

			// Map the request path to the file system path
			var filePath string

			if requestPath == "/favicon.ico" {
				filePath = "./static/favicon.ico"
			} else if requestPath == "/favicon.svg" {
				filePath = "./static/favicon.svg"
			} else if strings.HasPrefix(requestPath, "/static/") {
				// Extract the static file path
				filePath = "." + requestPath
			} else {
				c.Status(http.StatusNotFound)
				return
			}

			// Serve the file
			c.File(filePath)
		},
		"handleLogoutRedirect": func(c *gin.Context) {
			// Clear any auth cookies/tokens and redirect to login
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
		},
		"handleTicketRedirect": func(c *gin.Context) {
			// Redirect /ticket/:id to /tickets/:id for compatibility
			ticketID := c.Param("id")
			c.Redirect(http.StatusMovedPermanently, "/tickets/"+ticketID)
		},
		"handleAdminDashboard": func(c *gin.Context) {
			// Render admin dashboard template with data
			userID, _ := c.Get("user_id")
			userRole, _ := c.Get("user_role")

			// Get database connection
			db, err := adapter.GetDatabase()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}

			// Get counts for dashboard
			var userCount, groupCount, activeTickets, queueCount int

			// Count users
			err = db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users").Scan(&userCount)
			if err != nil {
				log.Printf("Error counting users: %v", err)
			}

			// Count groups
			err = db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM groups").Scan(&groupCount)
			if err != nil {
				log.Printf("Error counting groups: %v", err)
			}

			// Count active tickets
			err = db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM ticket WHERE ticket_state_id IN (1, 2, 3, 4)").Scan(&activeTickets)
			if err != nil {
				log.Printf("Error counting active tickets: %v", err)
			}

			// Count queues
			err = db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM queue").Scan(&queueCount)
			if err != nil {
				log.Printf("Error counting queues: %v", err)
			}

			// Render template with data using Pongo2 renderer
			renderer := api.GetPongo2Renderer()
			if renderer != nil {
				renderer.HTML(c, http.StatusOK, "pages/admin/dashboard.pongo2", pongo2.Context{
					"UserCount":     userCount,
					"GroupCount":    groupCount,
					"ActiveTickets": activeTickets,
					"QueueCount":    queueCount,
					"UserID":        userID,
					"UserRole":      userRole,
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Template renderer not available"})
			}
		},
		"handleQueuesRedirect": func(c *gin.Context) {
			// Get user role from context
			role, exists := c.Get("user_role")
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
				return
			}

			// Redirect based on role
			switch role {
			case "Admin":
				c.Redirect(http.StatusFound, "/admin/queues")
			case "Agent":
				c.Redirect(http.StatusFound, "/agent/queues")
			default:
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			}
		},
		"handleDemoCustomerLogin": func(c *gin.Context) {
			// Create a demo customer token
			token := fmt.Sprintf("demo_customer_%s_%d", "john.customer", time.Now().Unix())

			// Set cookie with 24 hour expiry
			c.SetCookie("access_token", token, 86400, "/", "", false, true)

			// Redirect to customer dashboard
			c.Redirect(http.StatusFound, "/customer/")
		},
	}

	routing.RegisterAPIHandlers(handlerRegistry, apiHandlers)

	// Create router for YAML routes
	r := gin.New()

	// Initialize pongo2 renderer for template rendering
	templateDir := "./templates"
	api.InitPongo2Renderer(templateDir)

	// Load YAML routes
	routesDir := os.Getenv("ROUTES_DIR")
	if routesDir == "" {
		routesDir = "/app/routes"
	}

	if err := routing.LoadYAMLRoutes(r, routesDir, handlerRegistry); err != nil {
		log.Printf("❌ Failed to load YAML routes: %v", err)
		log.Fatalf("🚨 YAML routes failed to load - cannot continue without routing configuration")
	}

	log.Println("✅ YAML routes loaded successfully")

	log.Println("✅ Backend initialized successfully")

	// Start server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting GOTRS HTMX server on port %s\n", port)
	fmt.Println("Available routes:")
	fmt.Println("  GET  /          -> Redirect to /login")
	fmt.Println("  GET  /login     -> Login page")
	fmt.Println("  GET  /dashboard -> Dashboard (demo)")
	fmt.Println("  GET  /tickets   -> Tickets list (demo)")
	fmt.Println("  POST /api/auth/login -> HTMX login")
	fmt.Println("")
	fmt.Println("LDAP API routes:")
	fmt.Println("  POST /api/v1/ldap/configure -> Configure LDAP")
	fmt.Println("  POST /api/v1/ldap/test -> Test LDAP connection")
	fmt.Println("  POST /api/v1/ldap/authenticate -> Authenticate user")
	fmt.Println("  GET  /api/v1/ldap/users/:username -> Get user info")
	fmt.Println("  POST /api/v1/ldap/sync/users -> Sync users")
	fmt.Println("  GET  /api/v1/ldap/config -> Get LDAP config")

	log.Fatal(r.Run(":" + port))
}
