package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// API v1 handler exports

// Tickets API handlers.
var HandleAPIv1TicketsList = HandleListTicketsAPI

var HandleAPIv1TicketGet = func(c *gin.Context) {
	id := c.Param("id")
	ticketRepo := GetTicketRepository()
	if ticketRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ticket repository not initialized",
		})
		return
	}

	// Convert string ID to uint
	ticketID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ticket ID",
		})
		return
	}

	ticket, err := ticketRepo.GetByID(uint(ticketID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Ticket not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ticket,
	})
}

var HandleAPIv1TicketCreate = HandleCreateTicketAPI

var HandleAPIv1TicketUpdate = HandleUpdateTicketAPI

var HandleAPIv1TicketDelete = HandleDeleteTicketAPI

// Users API handlers.
var HandleAPIv1UserMe = HandleUserMeAPI

var HandleAPIv1UsersList = func(c *gin.Context) {
	userRepo := GetUserRepository()
	if userRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "User repository not initialized",
		})
		return
	}

	users, err := userRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
	})
}

var HandleAPIv1UserGet = func(c *gin.Context) {
	id := c.Param("id")
	userRepo := GetUserRepository()
	if userRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "User repository not initialized",
		})
		return
	}

	// Convert string ID to uint
	var userID uint
	if _, err := fmt.Sscanf(id, "%d", &userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	user, err := userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

var HandleAPIv1UserCreate = func(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Endpoint /api/v1/users (POST) is under construction",
		"data":    nil,
	})
}

var HandleAPIv1UserUpdate = func(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Endpoint /api/v1/users/:id (PUT) is under construction",
		"data":    nil,
	})
}

var HandleAPIv1UserDelete = func(c *gin.Context) {
	c.JSON(http.StatusNoContent, gin.H{})
}

// Queues API handlers.
var HandleAPIv1QueuesList = func(c *gin.Context) {
	queueRepo := GetQueueRepository()
	if queueRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Queue repository not initialized",
		})
		return
	}

	queues, err := queueRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Convert to simpler format for API response
	queueList := make([]gin.H, 0, len(queues))
	for _, q := range queues {
		queueList = append(queueList, gin.H{
			"id":   q.ID,
			"name": q.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    queueList,
	})
}

var HandleAPIv1QueueGet = func(c *gin.Context) {
	id := c.Param("id")
	queueRepo := GetQueueRepository()
	if queueRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Queue repository not initialized",
		})
		return
	}

	// Convert string ID to uint
	queueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid queue ID",
		})
		return
	}

	queue, err := queueRepo.GetByID(uint(queueID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Queue not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    queue,
	})
}

var HandleAPIv1QueueCreate = func(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Endpoint /api/v1/queues (POST) is under construction",
		"data":    nil,
	})
}

var HandleAPIv1QueueUpdate = func(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Endpoint /api/v1/queues/:id (PUT) is under construction",
		"data":    nil,
	})
}

var HandleAPIv1QueueDelete = func(c *gin.Context) {
	c.JSON(http.StatusNoContent, gin.H{})
}

// Priorities API handlers.
var HandleAPIv1PrioritiesList = func(c *gin.Context) {
	priorityRepo := GetPriorityRepository()
	if priorityRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Priority repository not initialized",
		})
		return
	}

	priorities, err := priorityRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Convert to simpler format for API response
	priorityList := make([]gin.H, 0, len(priorities))
	for _, p := range priorities {
		priorityList = append(priorityList, gin.H{
			"id":   p.ID,
			"name": p.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    priorityList,
	})
}

var HandleAPIv1PriorityGet = func(c *gin.Context) {
	id := c.Param("id")
	priorityRepo := GetPriorityRepository()
	if priorityRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Priority repository not initialized",
		})
		return
	}

	// Convert string ID to uint
	priorityID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid priority ID",
		})
		return
	}

	priority, err := priorityRepo.GetByID(uint(priorityID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Priority not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    priority,
	})
}

// Articles API handlers.
var HandleAPIv1AddArticle = HandleCreateArticleAPI

var HandleAPIv1GetTicketArticles = func(c *gin.Context) {
	ticketID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Get ticket articles endpoint - under construction",
		"data":    gin.H{"ticket_id": ticketID},
	})
}

var HandleAPIv1GetTicketArticle = func(c *gin.Context) {
	ticketID := c.Param("id")
	articleID := c.Param("article_id")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Get ticket article endpoint - under construction",
		"data":    gin.H{"ticket_id": ticketID, "article_id": articleID},
	})
}

// Ticket action handlers.
var HandleAPIv1TicketClose = HandleCloseTicketAPI
var HandleAPIv1TicketReopen = HandleReopenTicketAPI
var HandleAPIv1TicketAssign = HandleAssignTicketAPI

// Auth handlers.
var HandleAPIv1AuthLogin = HandleLoginAPI
var HandleAPIv1AuthRefresh = HandleRefreshTokenAPI
var HandleAPIv1AuthLogout = HandleLogoutAPI
var HandleAPIv1AuthRegister = HandleRegisterAPI

// Search API handler (using existing HandleSearchAPI from search_handler.go)
