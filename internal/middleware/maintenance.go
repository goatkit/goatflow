package middleware

import (
	"database/sql"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/repository"
)

// MaintenanceNotification middleware checks for active/upcoming maintenance
// and adds notification data to the context for templates.
func MaintenanceNotification(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		repo := repository.NewSystemMaintenanceRepository(db)

		// Check active maintenance
		if active, err := repo.IsActive(); err == nil && active != nil {
			// Use default message from config if not set in record
			if active.NotifyMessage == nil || *active.NotifyMessage == "" {
				cfg := config.Get()
				if cfg.Maintenance.DefaultNotifyMessage != "" {
					defaultMsg := cfg.Maintenance.DefaultNotifyMessage
					active.NotifyMessage = &defaultMsg
				}
			}
			c.Set("MaintenanceActive", active)
		}

		// Check upcoming - get minutes from config
		cfg := config.Get()
		upcomingMinutes := cfg.Maintenance.TimeNotifyUpcomingMinutes
		if upcomingMinutes == 0 {
			upcomingMinutes = 30 // Fallback if config not set
		}
		if coming, err := repo.IsComing(upcomingMinutes); err == nil && coming != nil {
			c.Set("MaintenanceComing", coming)
		}

		c.Next()
	}
}
