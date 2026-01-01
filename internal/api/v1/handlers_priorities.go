package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Priority handlers - basic stubs for now.
func (router *APIRouter) handleListPriorities(c *gin.Context) {
	// TODO: Implement actual priority listing
	priorities := []gin.H{
		{"id": 1, "name": "1 very low"},
		{"id": 2, "name": "2 low"},
		{"id": 3, "name": "3 normal"},
		{"id": 4, "name": "4 high"},
		{"id": 5, "name": "5 very high"},
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    priorities,
	})
}

func (router *APIRouter) handleCreatePriority(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Color   string `json:"color"`
		Comment string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// TODO: Implement actual priority creation
	priority := gin.H{
		"id":         6,
		"name":       req.Name,
		"color":      req.Color,
		"comment":    req.Comment,
		"created_at": time.Now(),
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    priority,
	})
}

func (router *APIRouter) handleGetPriority(c *gin.Context) {
	priorityID := c.Param("id")

	// TODO: Implement actual priority fetching
	priority := gin.H{
		"id":         priorityID,
		"name":       "3 normal",
		"color":      "#000000",
		"created_at": time.Now().AddDate(-1, 0, 0),
		"updated_at": time.Now(),
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    priority,
	})
}

func (router *APIRouter) handleUpdatePriority(c *gin.Context) {
	priorityID := c.Param("id")

	var req struct {
		Name    string `json:"name"`
		Color   string `json:"color"`
		Comment string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// TODO: Implement actual priority update
	priority := gin.H{
		"id":         priorityID,
		"name":       req.Name,
		"color":      req.Color,
		"comment":    req.Comment,
		"updated_at": time.Now(),
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    priority,
	})
}

func (router *APIRouter) handleDeletePriority(c *gin.Context) {
	// priorityID := c.Param("id")

	// TODO: Implement actual priority deletion (or deactivation)
	c.JSON(http.StatusNoContent, nil)
}
