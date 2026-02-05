package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

// handleHealth returns API health status.
func (router *APIRouter) handleHealth(c *gin.Context) {
	sendSuccess(c, gin.H{
		"status":    "healthy",
		"service":   "gotrs-api",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC(),
	})
}

// handleAPIInfo returns API information.
func (router *APIRouter) handleAPIInfo(c *gin.Context) {
	sendSuccess(c, gin.H{
		"name":        "GOTRS API",
		"version":     "1.0.0",
		"description": "Modern Open Source Ticketing System API",
		"endpoints": gin.H{
			"tickets":   "/api/v1/tickets",
			"users":     "/api/v1/users",
			"queues":    "/api/v1/queues",
			"search":    "/api/v1/search",
			"dashboard": "/api/v1/dashboard",
		},
		"authentication": gin.H{
			"type":   "JWT",
			"login":  "/api/v1/auth/login",
			"logout": "/api/v1/auth/logout",
		},
		"features": []string{
			"ticket_management",
			"user_management",
			"queue_management",
			"advanced_search",
			"file_attachments",
			"sla_management",
			"rbac",
			"audit_logging",
		},
	})
}

// handleSystemStatus returns system status.
func (router *APIRouter) handleSystemStatus(c *gin.Context) {
	// This would normally check various system components
	sendSuccess(c, gin.H{
		"status": "operational",
		"components": gin.H{
			"database":        "healthy",
			"search":          "healthy",
			"file_store":      "healthy",
			"email":           "healthy",
			"background_jobs": "healthy",
		},
		"uptime":      "99.9%",
		"last_update": time.Now().UTC(),
	})
}

// handleGetCurrentUser returns current user information.
func (router *APIRouter) handleGetCurrentUser(c *gin.Context) {
	userID, email, role, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	sendSuccess(c, gin.H{
		"id":          userID,
		"email":       email,
		"role":        role,
		"name":        email, // Using email as name for now
		"permissions": router.rbac.GetRolePermissions(role),
	})
}

// handleUpdateCurrentUser updates current user information.
func (router *APIRouter) handleUpdateCurrentUser(c *gin.Context) {
	var updateRequest struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid update request: "+err.Error())
		return
	}

	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE users SET
			first_name = COALESCE(NULLIF(?, ''), first_name),
			last_name = COALESCE(NULLIF(?, ''), last_name),
			change_time = ?,
			change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, updateRequest.FirstName, updateRequest.LastName, now, userID, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		sendError(c, http.StatusNotFound, "User not found")
		return
	}

	sendSuccess(c, gin.H{
		"id":         userID,
		"first_name": updateRequest.FirstName,
		"last_name":  updateRequest.LastName,
		"updated_at": now,
	})
}

// handleGetUserPreferences returns user preferences.
func (router *APIRouter) handleGetUserPreferences(c *gin.Context) {
	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		// Return defaults if DB unavailable
		sendSuccess(c, gin.H{
			"user_id":  userID,
			"language": "en",
			"timezone": "UTC",
			"theme":    "light",
		})
		return
	}

	// Get preferences from user_preferences table
	query := database.ConvertQuery(`
		SELECT preferences_key, preferences_value
		FROM user_preferences
		WHERE user_id = ?
	`)

	rows, err := db.Query(query, userID)
	prefs := gin.H{"user_id": userID}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key, value string
			if rows.Scan(&key, &value) == nil {
				prefs[key] = value
			}
		}
	}

	// Add defaults for missing keys
	if _, ok := prefs["language"]; !ok {
		prefs["language"] = "en"
	}
	if _, ok := prefs["timezone"]; !ok {
		prefs["timezone"] = "UTC"
	}
	if _, ok := prefs["theme"]; !ok {
		prefs["theme"] = "light"
	}

	sendSuccess(c, prefs)
}

// handleUpdateUserPreferences updates user preferences.
func (router *APIRouter) handleUpdateUserPreferences(c *gin.Context) {
	var prefsRequest map[string]interface{}

	if err := c.ShouldBindJSON(&prefsRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid preferences request: "+err.Error())
		return
	}

	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	now := time.Now()

	// Update each preference using REPLACE (upsert)
	for key, value := range prefsRequest {
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
		default:
			continue // Skip complex types
		}

		query := database.ConvertQuery(`
			REPLACE INTO user_preferences (user_id, preferences_key, preferences_value)
			VALUES (?, ?, ?)
		`)
		db.Exec(query, userID, key, valueStr)
	}

	sendSuccess(c, gin.H{
		"user_id":    userID,
		"updated_at": now,
		"message":    "Preferences updated",
	})
}

// handleChangePassword changes user password.
func (router *APIRouter) handleChangePassword(c *gin.Context) {
	var passwordRequest struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&passwordRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid password change request: "+err.Error())
		return
	}

	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Get current password hash
	var currentHash string
	query := database.ConvertQuery(`SELECT pw FROM users WHERE id = ?`)
	if err := db.QueryRow(query, userID).Scan(&currentHash); err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to verify user")
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(passwordRequest.CurrentPassword)); err != nil {
		sendError(c, http.StatusUnauthorized, "Current password is incorrect")
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(passwordRequest.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to process new password")
		return
	}

	// Update password
	updateQuery := database.ConvertQuery(`UPDATE users SET pw = ?, change_time = ?, change_by = ? WHERE id = ?`)
	now := time.Now()
	if _, err := db.Exec(updateQuery, string(newHash), now, userID, userID); err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update password")
		return
	}

	sendSuccess(c, gin.H{
		"message":    "Password changed successfully",
		"changed_at": now,
	})
}

// handleGetUserSessions returns user's active sessions.
func (router *APIRouter) handleGetUserSessions(c *gin.Context) {
	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendSuccess(c, []interface{}{})
		return
	}

	// Get unique session IDs for this user from sessions table
	query := database.ConvertQuery(`
		SELECT DISTINCT s1.session_id,
			MAX(CASE WHEN s1.data_key = 'UserRemoteAddr' THEN s1.data_value END) as ip,
			MAX(CASE WHEN s1.data_key = 'UserRemoteUserAgent' THEN s1.data_value END) as agent,
			MAX(CASE WHEN s1.data_key = 'CreateTime' THEN s1.data_value END) as created,
			MAX(CASE WHEN s1.data_key = 'LastRequest' THEN s1.data_value END) as last_request
		FROM sessions s1
		WHERE s1.session_id IN (
			SELECT session_id FROM sessions 
			WHERE data_key = 'UserID' AND data_value = ?
		)
		GROUP BY s1.session_id
	`)

	rows, err := db.Query(query, userID)
	sessions := []gin.H{}
	currentSessionID := c.GetString("session_id")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sessionID string
			var ip, agent, created, lastRequest *string
			if rows.Scan(&sessionID, &ip, &agent, &created, &lastRequest) == nil {
				sessions = append(sessions, gin.H{
					"id":          sessionID,
					"user_id":     userID,
					"ip_address":  ip,
					"user_agent":  agent,
					"created_at":  created,
					"last_seen":   lastRequest,
					"current":     sessionID == currentSessionID,
				})
			}
		}
	}

	sendSuccess(c, sessions)
}

// handleRevokeSession revokes a user session.
func (router *APIRouter) handleRevokeSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		sendError(c, http.StatusBadRequest, "Session ID required")
		return
	}

	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Verify the session belongs to the current user
	verifyQuery := database.ConvertQuery(`
		SELECT 1 FROM sessions 
		WHERE session_id = ? AND data_key = 'UserID' AND data_value = ?
	`)
	var exists2 int
	if err := db.QueryRow(verifyQuery, sessionID, userID).Scan(&exists2); err != nil {
		sendError(c, http.StatusNotFound, "Session not found or not owned by user")
		return
	}

	// Delete all session data
	deleteQuery := database.ConvertQuery(`DELETE FROM sessions WHERE session_id = ?`)
	result, err := db.Exec(deleteQuery, sessionID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to revoke session")
		return
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		sendError(c, http.StatusNotFound, "Session not found")
		return
	}

	sendSuccess(c, gin.H{
		"session_id": sessionID,
		"message":    "Session revoked successfully",
		"revoked_at": time.Now(),
	})
}
