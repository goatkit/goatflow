package api

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

type Router struct {
	engine         *gin.Engine
	db             *sql.DB
	jwtManager     *auth.JWTManager
	authMiddleware *middleware.AuthMiddleware
	authHandler    *AuthHandler
	ticketHandler  *TicketHandler
}

func NewRouter(db *sql.DB, jwtSecret string) *Router {
	// Initialize JWT manager with 24 hour token duration
	jwtManager := auth.NewJWTManager(jwtSecret, 24*time.Hour)
	
	// Initialize repositories
	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	userRepo := repository.NewUserRepository(db)
	queueRepo := repository.NewQueueRepository(db)
	stateRepo := repository.NewTicketStateRepository(db)
	priorityRepo := repository.NewTicketPriorityRepository(db)
	
	// Initialize services
	authService := auth.NewAuthService(db, jwtManager)
	ticketService := service.NewTicketService(
		ticketRepo,
		articleRepo,
		userRepo,
		queueRepo,
		stateRepo,
		priorityRepo,
		db,
	)
	
	// Initialize handlers
	authHandler := NewAuthHandler(authService)
	ticketHandler := NewTicketHandler(ticketService, ticketRepo)
	
	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	
	return &Router{
		engine:         gin.Default(),
		db:             db,
		jwtManager:     jwtManager,
		authMiddleware: authMiddleware,
		authHandler:    authHandler,
		ticketHandler:  ticketHandler,
	}
}

func (r *Router) SetupRoutes() {
	// Health check endpoint
	r.engine.GET("/health", r.healthCheck)
	
	// API v1 routes
	v1 := r.engine.Group("/api/v1")
	{
		// Public auth routes
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/login", r.authHandler.Login)
			authGroup.POST("/refresh", r.authHandler.RefreshToken)
			authGroup.POST("/logout", r.authHandler.Logout)
		}
		
		// Protected auth routes
		authProtected := v1.Group("/auth")
		authProtected.Use(r.authMiddleware.RequireAuth())
		{
			authProtected.GET("/me", r.authHandler.GetCurrentUser)
			authProtected.POST("/change-password", r.authHandler.ChangePassword)
		}
		
		// User routes (admin only)
		userGroup := v1.Group("/users")
		userGroup.Use(r.authMiddleware.RequireAuth())
		userGroup.Use(r.authMiddleware.RequireRole(string(models.RoleAdmin)))
		{
			// TODO: Implement user management endpoints
			userGroup.GET("", r.placeholderHandler("List users"))
			userGroup.GET("/:id", r.placeholderHandler("Get user"))
			userGroup.POST("", r.placeholderHandler("Create user"))
			userGroup.PUT("/:id", r.placeholderHandler("Update user"))
			userGroup.DELETE("/:id", r.placeholderHandler("Delete user"))
		}
		
		// Ticket routes
		ticketGroup := v1.Group("/tickets")
		ticketGroup.Use(r.authMiddleware.RequireAuth())
		{
			// Basic CRUD operations
			ticketGroup.GET("", r.ticketHandler.ListTickets)
			ticketGroup.GET("/:id", r.ticketHandler.GetTicket)
			ticketGroup.POST("", r.authMiddleware.RequirePermission(auth.PermissionTicketCreate), r.ticketHandler.CreateTicket)
			ticketGroup.PUT("/:id", r.authMiddleware.RequirePermission(auth.PermissionTicketUpdate), r.ticketHandler.UpdateTicket)
			
			// Ticket articles (messages)
			ticketGroup.POST("/:id/articles", r.ticketHandler.AddArticle)
			ticketGroup.GET("/:id/articles", r.ticketHandler.GetArticles)
			
			// Ticket actions
			ticketGroup.POST("/:id/assign", r.authMiddleware.RequirePermission(auth.PermissionTicketAssign), r.ticketHandler.AssignTicket)
			ticketGroup.POST("/:id/escalate", r.authMiddleware.RequirePermission(auth.PermissionTicketUpdate), r.ticketHandler.EscalateTicket)
			ticketGroup.POST("/merge", r.authMiddleware.RequirePermission(auth.PermissionTicketUpdate), r.ticketHandler.MergeTickets)
			
			// Ticket history
			ticketGroup.GET("/:id/history", r.ticketHandler.GetTicketHistory)
		}
		
		// Queue routes (agent and admin)
		queueGroup := v1.Group("/queues")
		queueGroup.Use(r.authMiddleware.RequireAuth())
		queueGroup.Use(r.authMiddleware.RequireRole(string(models.RoleAgent), string(models.RoleAdmin)))
		{
			// TODO: Implement queue endpoints
			queueGroup.GET("", r.placeholderHandler("List queues"))
			queueGroup.GET("/:id", r.placeholderHandler("Get queue"))
			queueGroup.GET("/:id/tickets", r.placeholderHandler("Get queue tickets"))
		}
		
		// Report routes (agent and admin)
		reportGroup := v1.Group("/reports")
		reportGroup.Use(r.authMiddleware.RequireAuth())
		reportGroup.Use(r.authMiddleware.RequirePermission(auth.PermissionReportView))
		{
			// TODO: Implement report endpoints
			reportGroup.GET("/dashboard", r.placeholderHandler("Dashboard data"))
			reportGroup.GET("/tickets/summary", r.placeholderHandler("Ticket summary"))
			reportGroup.GET("/users/activity", r.placeholderHandler("User activity"))
		}
		
		// Admin routes
		adminGroup := v1.Group("/admin")
		adminGroup.Use(r.authMiddleware.RequireAuth())
		adminGroup.Use(r.authMiddleware.RequirePermission(auth.PermissionAdminAccess))
		{
			// TODO: Implement admin endpoints
			adminGroup.GET("/settings", r.placeholderHandler("Get settings"))
			adminGroup.PUT("/settings", r.placeholderHandler("Update settings"))
			adminGroup.GET("/audit-log", r.placeholderHandler("Get audit log"))
			adminGroup.GET("/system/info", r.placeholderHandler("System information"))
		}
	}
}

func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}

func (r *Router) healthCheck(c *gin.Context) {
	// Check database connection
	err := r.db.Ping()
	if err != nil {
		c.JSON(500, gin.H{
			"status": "unhealthy",
			"error":  "Database connection failed",
		})
		return
	}
	
	c.JSON(200, gin.H{
		"status":  "healthy",
		"service": "gotrs-api",
		"version": "0.1.0-alpha",
	})
}

func (r *Router) placeholderHandler(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": action + " - Coming soon",
			"status":  "not_implemented",
		})
	}
}