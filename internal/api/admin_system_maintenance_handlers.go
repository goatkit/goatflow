package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// handleAdminSystemMaintenance renders the system maintenance list page.
func handleAdminSystemMaintenance(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	repo := repository.NewSystemMaintenanceRepository(db)
	records, err := repo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch maintenance records")
		return
	}

	// Convert to template-friendly format
	maintenanceList := make([]gin.H, 0, len(records))
	for _, m := range records {
		maintenanceList = append(maintenanceList, gin.H{
			"ID":               m.ID,
			"StartDate":        m.StartDate,
			"StopDate":         m.StopDate,
			"StartDateFormatted": m.StartDateFormatted(),
			"StopDateFormatted":  m.StopDateFormatted(),
			"Comments":         m.Comments,
			"LoginMessage":     m.GetLoginMessage(),
			"ShowLoginMessage": m.ShowLoginMessage,
			"NotifyMessage":    m.GetNotifyMessage(),
			"ValidID":          m.ValidID,
			"IsValid":          m.IsValid(),
			"IsActive":         m.IsCurrentlyActive(),
			"IsPast":           m.IsPast(),
			"Duration":         m.Duration(),
			"CreateTime":       m.CreateTime,
			"ChangeTime":       m.ChangeTime,
		})
	}

	// Check if JSON response is requested
	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    maintenanceList,
		})
		return
	}

	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/system_maintenance.pongo2", pongo2.Context{
		"MaintenanceRecords": maintenanceList,
		"User":               getUserMapForTemplate(c),
		"ActivePage":         "admin",
	})
}

// handleAdminSystemMaintenanceNew renders the create new maintenance form.
func handleAdminSystemMaintenanceNew(c *gin.Context) {
	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/system_maintenance_form.pongo2", pongo2.Context{
		"IsNew":      true,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// handleAdminSystemMaintenanceEdit renders the edit maintenance form with session data.
func handleAdminSystemMaintenanceEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid maintenance ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	repo := repository.NewSystemMaintenanceRepository(db)
	m, err := repo.GetByID(id)
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "Maintenance record not found")
		return
	}

	// Get session counts for session management section
	sessionSvc, _ := getSessionService()
	var agentCount, customerCount int
	if sessionSvc != nil {
		sessions, _ := sessionSvc.ListSessions()
		for _, s := range sessions {
			if s.UserType == "Agent" {
				agentCount++
			} else if s.UserType == "Customer" {
				customerCount++
			}
		}
	}

	// Get session list for kill functionality
	var sessionList []gin.H
	if sessionSvc != nil {
		sessions, _ := sessionSvc.ListSessions()
		userRepo := repository.NewUserRepository(db)
		for _, s := range sessions {
			var userFullName string
			if user, err := userRepo.GetByID(uint(s.UserID)); err == nil && user != nil {
				userFullName = user.FirstName + " " + user.LastName
			}
			sessionList = append(sessionList, gin.H{
				"SessionID":    s.SessionID,
				"UserID":       s.UserID,
				"UserLogin":    s.UserLogin,
				"UserType":     s.UserType,
				"UserFullName": userFullName,
				"LastRequest":  s.LastRequest,
			})
		}
	}

	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	// Get current session ID to prevent killing own session
	currentSessionID, _ := c.Cookie("session_id")

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/system_maintenance_form.pongo2", pongo2.Context{
		"IsNew":            false,
		"Maintenance":      m,
		"AgentCount":       agentCount,
		"CustomerCount":    customerCount,
		"Sessions":         sessionList,
		"CurrentSessionID": currentSessionID,
		"User":             getUserMapForTemplate(c),
		"ActivePage":       "admin",
	})
}

// handleCreateSystemMaintenance creates a new maintenance record.
func handleCreateSystemMaintenance(c *gin.Context) {
	log.Printf("handleCreateSystemMaintenance: received request")

	var input struct {
		StartDate        string  `json:"start_date" binding:"required"`
		StopDate         string  `json:"stop_date" binding:"required"`
		Comments         string  `json:"comments" binding:"required"`
		LoginMessage     *string `json:"login_message"`
		ShowLoginMessage int     `json:"show_login_message"`
		NotifyMessage    *string `json:"notify_message"`
		ValidID          int     `json:"valid_id"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("handleCreateSystemMaintenance: binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Start date, stop date, and comments are required",
		})
		return
	}
	log.Printf("handleCreateSystemMaintenance: input parsed - start=%s, stop=%s, comments=%s", input.StartDate, input.StopDate, input.Comments)

	// Parse dates (expecting ISO format: 2006-01-02T15:04)
	startDate, err := time.Parse("2006-01-02T15:04", input.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid start date format",
		})
		return
	}

	stopDate, err := time.Parse("2006-01-02T15:04", input.StopDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid stop date format",
		})
		return
	}

	// Validate start before stop
	if !startDate.Before(stopDate) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Start date must be before stop date",
		})
		return
	}

	// Validate comments length
	if len(input.Comments) > 250 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Comments must be 250 characters or less",
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

	// Get current user ID
	userID := 1 // Default to 1 if not available
	if u, exists := c.Get("user_id"); exists {
		if uid, ok := u.(int); ok {
			userID = uid
		}
	}

	// Default valid_id to 1 if not provided
	if input.ValidID == 0 {
		input.ValidID = 1
	}

	m := &models.SystemMaintenance{
		StartDate:        startDate.Unix(),
		StopDate:         stopDate.Unix(),
		Comments:         input.Comments,
		LoginMessage:     input.LoginMessage,
		ShowLoginMessage: input.ShowLoginMessage,
		NotifyMessage:    input.NotifyMessage,
		ValidID:          input.ValidID,
		CreateBy:         userID,
		ChangeBy:         userID,
	}

	repo := repository.NewSystemMaintenanceRepository(db)
	if err := repo.Create(m); err != nil {
		log.Printf("handleCreateSystemMaintenance: repo.Create error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create maintenance record",
		})
		return
	}

	log.Printf("handleCreateSystemMaintenance: created record with ID=%d", m.ID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Maintenance scheduled successfully",
		"data": gin.H{
			"id": m.ID,
		},
	})
}

// handleUpdateSystemMaintenance updates an existing maintenance record.
func handleUpdateSystemMaintenance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid maintenance ID",
		})
		return
	}

	var input struct {
		StartDate        *string `json:"start_date"`
		StopDate         *string `json:"stop_date"`
		Comments         *string `json:"comments"`
		LoginMessage     *string `json:"login_message"`
		ShowLoginMessage *int    `json:"show_login_message"`
		NotifyMessage    *string `json:"notify_message"`
		ValidID          *int    `json:"valid_id"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input",
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

	repo := repository.NewSystemMaintenanceRepository(db)
	m, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Maintenance record not found",
		})
		return
	}

	// Update fields if provided
	if input.StartDate != nil {
		startDate, err := time.Parse("2006-01-02T15:04", *input.StartDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid start date format",
			})
			return
		}
		m.StartDate = startDate.Unix()
	}

	if input.StopDate != nil {
		stopDate, err := time.Parse("2006-01-02T15:04", *input.StopDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid stop date format",
			})
			return
		}
		m.StopDate = stopDate.Unix()
	}

	// Validate start before stop
	if m.StartDate >= m.StopDate {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Start date must be before stop date",
		})
		return
	}

	if input.Comments != nil {
		if len(*input.Comments) > 250 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Comments must be 250 characters or less",
			})
			return
		}
		m.Comments = *input.Comments
	}

	if input.LoginMessage != nil {
		m.LoginMessage = input.LoginMessage
	}

	if input.ShowLoginMessage != nil {
		m.ShowLoginMessage = *input.ShowLoginMessage
	}

	if input.NotifyMessage != nil {
		m.NotifyMessage = input.NotifyMessage
	}

	if input.ValidID != nil {
		m.ValidID = *input.ValidID
	}

	// Get current user ID
	userID := 1
	if u, exists := c.Get("user_id"); exists {
		if uid, ok := u.(int); ok {
			userID = uid
		}
	}
	m.ChangeBy = userID

	if err := repo.Update(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update maintenance record",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Maintenance updated successfully",
	})
}

// handleDeleteSystemMaintenance deletes a maintenance record.
func handleDeleteSystemMaintenance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid maintenance ID",
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

	repo := repository.NewSystemMaintenanceRepository(db)
	if err := repo.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Maintenance record not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Maintenance deleted successfully",
	})
}

// handleGetSystemMaintenance returns a single maintenance record by ID (API endpoint).
func handleGetSystemMaintenance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid maintenance ID",
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

	repo := repository.NewSystemMaintenanceRepository(db)
	m, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Maintenance record not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":                 m.ID,
			"start_date":         m.StartDate,
			"stop_date":          m.StopDate,
			"start_date_formatted": m.StartDateFormatted(),
			"stop_date_formatted":  m.StopDateFormatted(),
			"comments":           m.Comments,
			"login_message":      m.GetLoginMessage(),
			"show_login_message": m.ShowLoginMessage,
			"notify_message":     m.GetNotifyMessage(),
			"valid_id":           m.ValidID,
			"is_active":          m.IsCurrentlyActive(),
			"create_time":        m.CreateTime,
			"change_time":        m.ChangeTime,
		},
	})
}

// Ensure sql import is used
var _ = sql.ErrNoRows

// Ensure service import is used
var _ = service.NewSessionService
