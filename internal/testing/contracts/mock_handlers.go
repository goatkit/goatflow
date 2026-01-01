package contracts

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MockHandlers provides mock implementations for contract testing.
type MockHandlers struct{}

// Auth handlers.
func (m *MockHandlers) HandleLogin(c *gin.Context) {
	var loginRequest struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if loginRequest.Login == "" || loginRequest.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Login and password are required",
		})
		return
	}

	// Check credentials
	if loginRequest.Login == "testuser" && loginRequest.Password == "testpass123" {
		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test_access_token",
			"refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test_refresh_token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"user": gin.H{
				"id":         1,
				"login":      "testuser",
				"email":      "test@example.com",
				"first_name": "Test",
				"last_name":  "User",
				"role":       "agent",
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"error":   "Invalid credentials",
	})
}

func (m *MockHandlers) HandleRefresh(c *gin.Context) {
	var refreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.ShouldBindJSON(&refreshRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if refreshRequest.RefreshToken == "valid_refresh_token_here" {
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.new_access_token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"error":   "Invalid refresh token",
	})
}

func (m *MockHandlers) HandleLogout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logged out successfully",
	})
}

// Ticket handlers.
func (m *MockHandlers) HandleListTickets(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"tickets": []gin.H{
				{
					"id":            1,
					"ticket_number": "2024000001",
					"title":         "Test Ticket",
					"state_id":      1,
					"priority_id":   3,
					"queue_id":      1,
				},
			},
			"total":    1,
			"page":     1,
			"per_page": 20,
		},
	})
}

func (m *MockHandlers) HandleGetTicket(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	id := c.Param("id")
	if id == "99999" {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Ticket not found",
		})
		return
	}

	ticketID, _ := strconv.Atoi(id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":               ticketID,
			"ticket_number":    "2024000001",
			"title":            "Test Ticket",
			"state_id":         1,
			"priority_id":      3,
			"queue_id":         1,
			"customer_id":      "CUST001",
			"customer_user_id": "customer@example.com",
		},
	})
}

func (m *MockHandlers) HandleCreateTicket(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	var createRequest struct {
		Title      string `json:"title"`
		QueueID    int    `json:"queue_id"`
		PriorityID int    `json:"priority_id"`
	}

	if err := c.ShouldBindJSON(&createRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if createRequest.Title == "" || createRequest.QueueID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Title and queue_id are required",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":            123,
			"ticket_number": "2024000123",
		},
	})
}

func (m *MockHandlers) HandleUpdateTicket(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (m *MockHandlers) HandleDeleteTicket(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// Ticket action handlers.
func (m *MockHandlers) HandleCloseTicket(c *gin.Context) {
	id := c.Param("id")

	var closeRequest struct {
		Resolution string `json:"resolution"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&closeRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if id == "2" {
		// Simulate already closed ticket
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Ticket is already closed",
		})
		return
	}

	ticketID, _ := strconv.Atoi(id)
	stateID := 3 // closed unsuccessful by default
	if closeRequest.Resolution == "resolved" {
		stateID = 2 // closed successful
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"id":         ticketID,
		"state_id":   stateID,
		"state":      "closed",
		"resolution": closeRequest.Resolution,
		"closed_at":  "2024-01-01T12:00:00Z",
	})
}

func (m *MockHandlers) HandleReopenTicket(c *gin.Context) {
	id := c.Param("id")

	var reopenRequest struct {
		Reason string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&reopenRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if id == "3" {
		// Simulate already open ticket
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Ticket is already open",
		})
		return
	}

	ticketID, _ := strconv.Atoi(id)
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"id":          ticketID,
		"state_id":    4,
		"state":       "open",
		"reason":      reopenRequest.Reason,
		"reopened_at": "2024-01-01T12:00:00Z",
	})
}

func (m *MockHandlers) HandleAssignTicket(c *gin.Context) {
	id := c.Param("id")

	var assignRequest struct {
		AssignedTo int    `json:"assigned_to"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&assignRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if assignRequest.AssignedTo == 99999 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	ticketID, _ := strconv.Atoi(id)
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"id":          ticketID,
		"assigned_to": assignRequest.AssignedTo,
		"assignee":    "Agent Name",
		"assigned_at": "2024-01-01T12:00:00Z",
	})
}

// User handlers.
func (m *MockHandlers) HandleListUsers(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	// Parse pagination
	page := 1
	perPage := 20
	if p := c.Query("page"); p != "" {
		// Just acknowledge the param
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": []gin.H{
			{
				"id":         1,
				"login":      "admin",
				"email":      "admin@example.com",
				"first_name": "System",
				"last_name":  "Admin",
				"valid_id":   1,
				"groups": []gin.H{
					{"id": 1, "name": "admin"},
				},
			},
			{
				"id":         2,
				"login":      "agent1",
				"email":      "agent1@example.com",
				"first_name": "Test",
				"last_name":  "Agent",
				"valid_id":   1,
				"groups":     []gin.H{},
			},
		},
		"pagination": gin.H{
			"page":        page,
			"per_page":    perPage,
			"total":       2,
			"total_pages": 1,
			"has_next":    false,
			"has_prev":    false,
		},
	})
}

func (m *MockHandlers) HandleGetUser(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	id := c.Param("id")
	if id == "99999" {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	userID, _ := strconv.Atoi(id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         userID,
			"login":      "testuser",
			"email":      "test@example.com",
			"first_name": "Test",
			"last_name":  "User",
			"valid_id":   1,
			"groups": []gin.H{
				{"id": 1, "name": "users"},
			},
		},
	})
}

func (m *MockHandlers) HandleCreateUser(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	var createRequest struct {
		Login    string `json:"login"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&createRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Check for duplicate
	if createRequest.Login == "admin" {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Login already exists",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User created successfully",
		"data": gin.H{
			"id":    123,
			"login": createRequest.Login,
			"email": createRequest.Email,
		},
	})
}

func (m *MockHandlers) HandleUpdateUser(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully",
	})
}

func (m *MockHandlers) HandleDeleteUser(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	id := c.Param("id")
	if id == "1" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Cannot delete system user",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func (m *MockHandlers) HandleGetUserGroups(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": []gin.H{
			{
				"id":               1,
				"name":             "users",
				"permission_key":   "rw",
				"permission_value": 1,
			},
			{
				"id":               2,
				"name":             "admin",
				"permission_key":   "rw",
				"permission_value": 1,
			},
		},
	})
}

func (m *MockHandlers) HandleAddUserToGroup(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User added to group successfully",
	})
}

func (m *MockHandlers) HandleRemoveUserFromGroup(c *gin.Context) {
	// Check for auth header
	if c.GetHeader("Authorization") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	c.Status(http.StatusNoContent)
}
