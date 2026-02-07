package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/repository"
)

// HandleDebugTicketNumber returns current ticket number generator info.
func HandleDebugTicketNumber(c *gin.Context) {
	name, dateBased := repository.TicketNumberGeneratorInfo()
	if name == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "generator not initialized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"generator":  name,
			"date_based": dateBased,
		},
	})
}
