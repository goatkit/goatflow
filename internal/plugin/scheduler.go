package plugin

import (
	"context"
	"log"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/services/scheduler"
)

// RegisterPluginJobs registers all plugin-defined jobs with the scheduler.
// Call this after plugins are loaded and the scheduler is created.
func RegisterPluginJobs(mgr *Manager, sched *scheduler.Service) int {
	if mgr == nil || sched == nil {
		return 0
	}

	registered := 0
	pluginJobs := mgr.Jobs()

	for _, pj := range pluginJobs {
		pluginName := pj.PluginName
		jobSpec := pj.JobSpec

		// Skip disabled jobs
		if !jobSpec.Enabled {
			continue
		}

		// Create a unique handler name for this plugin job
		handlerName := "plugin." + pluginName + "." + jobSpec.ID

		// Capture loop variables for closure
		pName := pluginName
		jHandler := jobSpec.Handler
		jID := jobSpec.ID

		// Register the handler that will call the plugin
		sched.RegisterHandler(handlerName, func(ctx context.Context, job *models.ScheduledJob) error {
			// Call the plugin's job handler
			_, err := mgr.Call(ctx, pName, jHandler, nil)
			if err != nil {
				log.Printf("🔌 Plugin job %s.%s failed: %v", pName, jID, err)
				return err
			}
			log.Printf("🔌 Plugin job %s.%s completed", pName, jID)
			return nil
		})

		// Parse timeout if specified
		var timeoutSecs int
		if jobSpec.Timeout != "" {
			if d, err := time.ParseDuration(jobSpec.Timeout); err == nil {
				timeoutSecs = int(d.Seconds())
			}
		}
		if timeoutSecs == 0 {
			timeoutSecs = 300 // Default 5 min timeout
		}

		// Create the scheduled job
		scheduledJob := &models.ScheduledJob{
			Slug:           handlerName,
			Handler:        handlerName,
			Name:           pluginName + ": " + jobSpec.Description,
			Schedule:       jobSpec.Schedule,
			TimeoutSeconds: timeoutSecs,
		}

		// Add to scheduler
		if err := sched.AddJob(scheduledJob); err != nil {
			log.Printf("⚠️  Failed to register plugin job %s: %v", handlerName, err)
			continue
		}

		log.Printf("🔌 Registered plugin job: %s (%s)", handlerName, jobSpec.Schedule)
		registered++
	}

	return registered
}
