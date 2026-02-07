package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// handleGetTicketHistory returns the history/timeline for a ticket.
func (router *APIRouter) handleGetTicketHistory(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid ticket ID",
		})
		return
	}

	// Get limit from query params (default 50)
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	ticketRepo := repository.NewTicketRepository(db)

	// Verify ticket exists
	ticket, err := ticketRepo.GetByID(uint(ticketID))
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Ticket not found",
		})
		return
	}

	// Get history entries
	history, err := ticketRepo.GetTicketHistoryEntries(uint(ticketID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve ticket history",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"ticket_id": ticketID,
			"count":     len(history),
			"history":   history,
		},
	})
}

// handleGetTicketLinks returns linked tickets for a ticket.
func (router *APIRouter) handleGetTicketLinks(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid ticket ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	ticketRepo := repository.NewTicketRepository(db)

	// Verify ticket exists
	ticket, err := ticketRepo.GetByID(uint(ticketID))
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Ticket not found",
		})
		return
	}

	// Get linked tickets (up to 100)
	links, err := ticketRepo.GetTicketLinks(uint(ticketID), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve ticket links",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"ticket_id": ticketID,
			"count":     len(links),
			"links":     links,
		},
	})
}
