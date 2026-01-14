package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/service/genericinterface"
)

var giService *genericinterface.Service

// initWebserviceService initializes the GenericInterface service.
func initWebserviceService() {
	db, err := database.GetDB()
	if err != nil {
		return
	}
	giService = genericinterface.NewService(db)
}

// getGIService returns the GenericInterface service, initializing if needed.
func getGIService() *genericinterface.Service {
	if giService == nil {
		initWebserviceService()
	}
	return giService
}

// handleAdminWebservices renders the webservice management page.
func handleAdminWebservices(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Get search and filter parameters
	searchQuery := c.Query("search")
	validFilter := c.DefaultQuery("valid", "all")

	// Get all webservices
	webservices, err := svc.ListWebservices(c.Request.Context())
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch webservices")
		return
	}

	// Apply filters
	var filtered []*models.WebserviceConfig
	for _, ws := range webservices {
		// Search filter
		if searchQuery != "" {
			searchLower := strings.ToLower(searchQuery)
			nameLower := strings.ToLower(ws.Name)
			descLower := ""
			if ws.Config != nil {
				descLower = strings.ToLower(ws.Config.Description)
			}
			if !strings.Contains(nameLower, searchLower) && !strings.Contains(descLower, searchLower) {
				continue
			}
		}

		// Valid filter
		if validFilter == "valid" && ws.ValidID != 1 {
			continue
		}
		if validFilter == "invalid" && ws.ValidID == 1 {
			continue
		}

		filtered = append(filtered, ws)
	}

	// Check if JSON response is requested
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    filtered,
		})
		return
	}

	// Render the template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/webservices.pongo2", pongo2.Context{
		"Title":        "Web Services",
		"Webservices":  filtered,
		"SearchQuery":  searchQuery,
		"ValidFilter":  validFilter,
		"User":         getUserMapForTemplate(c),
		"ActivePage":   "admin",
	})
}

// handleAdminWebserviceNew renders the new webservice form.
func handleAdminWebserviceNew(c *gin.Context) {
	// Render the form template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/webservice_form.pongo2", pongo2.Context{
		"Title":       "New Web Service",
		"IsNew":       true,
		"Webservice":  nil,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminWebserviceEdit renders the webservice edit form.
func handleAdminWebserviceEdit(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Service unavailable")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid webservice ID")
		return
	}

	ws, err := svc.GetWebserviceByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			sendErrorResponse(c, http.StatusNotFound, "Webservice not found")
		} else {
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch webservice")
		}
		return
	}

	// Render the form template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/webservice_form.pongo2", pongo2.Context{
		"Title":       "Edit Web Service",
		"IsNew":       false,
		"Webservice":  ws,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminWebserviceGet returns a webservice's details as JSON.
func handleAdminWebserviceGet(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid webservice ID",
		})
		return
	}

	ws, err := svc.GetWebserviceByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Webservice not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch webservice",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ws,
	})
}

// handleCreateWebservice creates a new webservice.
func handleCreateWebservice(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	var input struct {
		Name         string                       `json:"name" binding:"required"`
		Description  string                       `json:"description"`
		RemoteSystem string                       `json:"remote_system"`
		ValidID      int                          `json:"valid_id"`
		Config       *models.WebserviceConfigData `json:"config"`
	}

	// Default valid_id to 1 if not provided
	input.ValidID = 1

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
		})
		return
	}

	// Validate name is not empty
	if strings.TrimSpace(input.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
		})
		return
	}

	// Check for duplicate name
	exists, err := svc.WebserviceExists(c.Request.Context(), input.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check for duplicates",
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "A webservice with this name already exists",
		})
		return
	}

	// Build config if not provided
	config := input.Config
	if config == nil {
		config = &models.WebserviceConfigData{
			Description:  input.Description,
			RemoteSystem: input.RemoteSystem,
			Debugger: models.DebuggerConfig{
				DebugThreshold: "error",
				TestMode:       "0",
			},
			Requester: models.RequesterConfig{
				Transport: models.TransportConfig{
					Type: "HTTP::REST",
				},
			},
		}
	}

	ws := &models.WebserviceConfig{
		Name:    input.Name,
		ValidID: input.ValidID,
		Config:  config,
	}

	// Get user ID from session
	userID := 1 // Default to admin
	if user, ok := c.Get("user"); ok {
		if u, ok := user.(map[string]interface{}); ok {
			if id, ok := u["id"].(int); ok {
				userID = id
			}
		}
	}

	id, err := svc.CreateWebservice(c.Request.Context(), ws, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create webservice: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":   id,
			"name": input.Name,
		},
		"message": "Webservice created successfully",
	})
}

// handleUpdateWebservice updates an existing webservice.
func handleUpdateWebservice(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid webservice ID",
		})
		return
	}

	var input struct {
		Name         string                       `json:"name" binding:"required"`
		Description  string                       `json:"description"`
		RemoteSystem string                       `json:"remote_system"`
		ValidID      int                          `json:"valid_id"`
		Config       *models.WebserviceConfigData `json:"config"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
		})
		return
	}

	// Validate name is not empty
	if strings.TrimSpace(input.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
		})
		return
	}

	// Check for duplicate name (excluding current ID)
	exists, err := svc.WebserviceExistsExcluding(c.Request.Context(), input.Name, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check for duplicates",
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "A webservice with this name already exists",
		})
		return
	}

	// Get existing webservice
	ws, err := svc.GetWebserviceByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Webservice not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch webservice",
			})
		}
		return
	}

	// Update fields
	ws.Name = input.Name
	ws.ValidID = input.ValidID
	if input.Config != nil {
		ws.Config = input.Config
	} else {
		// Update description/remote_system in existing config
		if ws.Config == nil {
			ws.Config = &models.WebserviceConfigData{}
		}
		ws.Config.Description = input.Description
		ws.Config.RemoteSystem = input.RemoteSystem
	}

	// Get user ID from session
	userID := 1
	if user, ok := c.Get("user"); ok {
		if u, ok := user.(map[string]interface{}); ok {
			if uid, ok := u["id"].(int); ok {
				userID = uid
			}
		}
	}

	err = svc.UpdateWebservice(c.Request.Context(), ws, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update webservice: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webservice updated successfully",
	})
}

// handleDeleteWebservice deletes a webservice.
func handleDeleteWebservice(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid webservice ID",
		})
		return
	}

	err = svc.DeleteWebservice(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Webservice not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to delete webservice: " + err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webservice deleted successfully",
	})
}

// handleTestWebservice tests a webservice connection.
func handleTestWebservice(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid webservice ID",
		})
		return
	}

	ws, err := svc.GetWebserviceByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Webservice not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch webservice",
			})
		}
		return
	}

	// Get the REST transport and test connection
	transport, err := svc.GetTransport(ws.Config.Requester.Transport.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Unsupported transport type: " + ws.Config.Requester.Transport.Type,
		})
		return
	}

	// Type assert to RESTTransport to access TestConnection
	if restTransport, ok := transport.(*genericinterface.RESTTransport); ok {
		err = restTransport.TestConnection(c.Request.Context(), ws.Config.Requester.Transport.Config)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "Connection test failed: " + err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Connection test successful",
	})
}

// handleAdminWebserviceHistory shows the configuration history for a webservice.
func handleAdminWebserviceHistory(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Service unavailable")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid webservice ID")
		return
	}

	ws, err := svc.GetWebserviceByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			sendErrorResponse(c, http.StatusNotFound, "Webservice not found")
		} else {
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch webservice")
		}
		return
	}

	history, err := svc.GetHistory(c.Request.Context(), id)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch history")
		return
	}

	// Render the template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/webservice_history.pongo2", pongo2.Context{
		"Title":       "Web Service History",
		"Webservice":  ws,
		"History":     history,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleRestoreWebserviceHistory restores a webservice from a history entry.
func handleRestoreWebserviceHistory(c *gin.Context) {
	svc := getGIService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	historyIDStr := c.Param("historyId")
	historyID, err := strconv.ParseInt(historyIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid history ID",
		})
		return
	}

	// Get user ID from session
	userID := 1
	if user, ok := c.Get("user"); ok {
		if u, ok := user.(map[string]interface{}); ok {
			if uid, ok := u["id"].(int); ok {
				userID = uid
			}
		}
	}

	err = svc.RestoreFromHistory(c.Request.Context(), historyID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to restore configuration: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration restored successfully",
	})
}
