package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// IncidentHandlers handles incident-related HTTP requests.
type IncidentHandlers struct {
	// TODO: Add incident service when interface is defined
}

// NewIncidentHandlers creates a new incident handlers instance.
func NewIncidentHandlers() *IncidentHandlers {
	return &IncidentHandlers{}
}

// RegisterRoutes registers incident routes.
func (h *IncidentHandlers) RegisterRoutes(router *gin.RouterGroup) {
	incidents := router.Group("/incidents")
	{
		// All routes return not implemented for now
		incidents.POST("", h.notImplemented)
		incidents.GET("", h.notImplemented)
		incidents.GET("/:id", h.notImplemented)
		incidents.PUT("/:id", h.notImplemented)
		incidents.DELETE("/:id", h.notImplemented)
	}
}

func (h *IncidentHandlers) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Incident management not yet implemented",
	})
}
