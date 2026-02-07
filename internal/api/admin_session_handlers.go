package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/service"
)

func init() {
	routing.RegisterHandler("handleAdminSessions", handleAdminSessions)
	routing.RegisterHandler("handleKillSession", handleKillSession)
	routing.RegisterHandler("handleKillUserSessions", handleKillUserSessions)
	routing.RegisterHandler("handleKillAllSessions", handleKillAllSessions)
}

// getSessionService creates a session service with database connection.
func getSessionService() (*service.SessionService, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	repo := repository.NewSessionRepository(db)
	return service.NewSessionService(repo), nil
}

// handleAdminSessions renders the session management page.
func handleAdminSessions(c *gin.Context) {
	sessionSvc, err := getSessionService()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	sessions, err := sessionSvc.ListSessions()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}

	// Get user repository to lookup user details
	db, _ := database.GetDB()
	userRepo := repository.NewUserRepository(db)

	// Convert sessions to template-friendly format
	sessionList := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		// Lookup user details for title and full name
		var userTitle, userFullName string
		if user, err := userRepo.GetByID(uint(s.UserID)); err == nil && user != nil {
			userTitle = user.Title
			userFullName = strings.TrimSpace(user.FirstName + " " + user.LastName)
		}

		sessionList = append(sessionList, gin.H{
			"SessionID":    s.SessionID,
			"UserID":       s.UserID,
			"UserLogin":    s.UserLogin,
			"UserType":     s.UserType,
			"UserTitle":    userTitle,
			"UserFullName": userFullName,
			"CreateTime":   s.CreateTime,
			"LastRequest":  s.LastRequest,
			"RemoteAddr":   s.RemoteAddr,
			"UserAgent":    s.UserAgent,
		})
	}

	// Check if JSON is requested
	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"sessions": sessionList,
			"count":    len(sessionList),
		})
		return
	}

	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	// Get current session ID to mark in the UI
	currentSessionID, _ := c.Cookie("session_id")

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/sessions.pongo2", pongo2.Context{
		"Sessions":         sessionList,
		"CurrentSessionID": currentSessionID,
		"User":             getUserMapForTemplate(c),
		"ActivePage":       "admin",
	})
}

// handleKillSession terminates a specific session.
func handleKillSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Session ID is required",
		})
		return
	}

	sessionSvc, err := getSessionService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	if err := sessionSvc.KillSession(sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Session not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session terminated successfully",
	})
}

// handleKillUserSessions terminates all sessions for a specific user.
func handleKillUserSessions(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	sessionSvc, err := getSessionService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	count, err := sessionSvc.KillUserSessions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to terminate sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User sessions terminated successfully",
		"count":   count,
	})
}

// handleKillAllSessions terminates all sessions (emergency action).
func handleKillAllSessions(c *gin.Context) {
	// Get confirmation from request
	var req struct {
		Confirm bool `json:"confirm" form:"confirm"`
	}
	if err := c.ShouldBind(&req); err != nil || !req.Confirm {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Confirmation required to terminate all sessions",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Delete all sessions directly from the database
	result, err := db.Exec(database.ConvertPlaceholders("DELETE FROM sessions"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to terminate sessions",
		})
		return
	}

	count, _ := result.RowsAffected()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All sessions terminated successfully",
		"count":   count,
	})
}

// Helper to check if session service is available (for tests).
func sessionServiceAvailable() bool {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return false
	}
	// Check if sessions table exists
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'sessions')"
	if database.IsMySQL() {
		query = "SELECT COUNT(*) > 0 FROM information_schema.tables WHERE table_name = 'sessions' AND table_schema = DATABASE()"
	}
	if err := db.QueryRow(query).Scan(&exists); err != nil {
		return false
	}
	return exists
}

// Ensure the import is used
var _ = sql.ErrNoRows
