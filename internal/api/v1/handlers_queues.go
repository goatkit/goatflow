package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Queue handlers - basic stubs for now.
func (router *APIRouter) handleListQueues(c *gin.Context) {
	// TODO: Implement actual queue listing
	queues := []gin.H{
		{
			"id":   1,
			"name": "Raw",
		},
		{
			"id":   2,
			"name": "Junk",
		},
		{
			"id":   3,
			"name": "Misc",
		},
		{
			"id":   4,
			"name": "Postmaster",
		},
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    queues,
	})
}

func (router *APIRouter) handleCreateQueue(c *gin.Context) {
	var req struct {
		Name            string `json:"name" binding:"required"`
		SystemAddressID int    `json:"system_address_id"`
		Comment         string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// TODO: Implement actual queue creation
	queue := gin.H{
		"id":         5,
		"name":       req.Name,
		"comment":    req.Comment,
		"created_at": time.Now(),
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    queue,
	})
}

func (router *APIRouter) handleGetQueue(c *gin.Context) {
	queueID := c.Param("id")

	// TODO: Implement actual queue fetching
	queue := gin.H{
		"id":         queueID,
		"name":       "Raw",
		"comment":    "Unassigned tickets",
		"created_at": time.Now().AddDate(-1, 0, 0),
		"updated_at": time.Now(),
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    queue,
	})
}

func (router *APIRouter) handleUpdateQueue(c *gin.Context) {
	queueID := c.Param("id")

	var req struct {
		Name    string `json:"name"`
		Comment string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// TODO: Implement actual queue update
	queue := gin.H{
		"id":         queueID,
		"name":       req.Name,
		"comment":    req.Comment,
		"updated_at": time.Now(),
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    queue,
	})
}

func (router *APIRouter) handleDeleteQueue(c *gin.Context) {
	// queueID := c.Param("id")

	// TODO: Implement actual queue deletion (or deactivation)
	c.JSON(http.StatusNoContent, nil)
}

func (router *APIRouter) handleGetQueueTickets(c *gin.Context) {
	queueID := c.Param("id")

	// TODO: Implement actual queue tickets fetching
	tickets := []gin.H{
		{
			"id":       1,
			"number":   "2024080100001",
			"title":    "Sample ticket in queue",
			"queue_id": queueID,
		},
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    tickets,
	})
}

func (router *APIRouter) handleGetQueueStats(c *gin.Context) {
	queueID := c.Param("id")

	// TODO: Implement actual queue statistics
	stats := gin.H{
		"queue_id":     queueID,
		"total":        42,
		"open":         10,
		"closed":       30,
		"pending":      2,
		"avg_response": "2h 30m",
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}
