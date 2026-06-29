package plugin

import (
	"context"
	"log"
	"time"
)

// RegisterPluginJobs registers all plugin-defined jobs with the scheduler.
// Call this after plugins are loaded and the scheduler is created.
func RegisterPluginJobs(mgr *Manager, sched PlatformScheduler) int {
	if mgr == nil || sched == nil {
		return 0
	}

	registered := 0
	pluginJobs := mgr.Jobs()

	for _, pj := range pluginJobs {
		pluginName := pj.PluginName
		jobSpec := pj.JobSpec

		if !jobSpec.Enabled {
			continue
		}

		handlerName := "plugin." + pluginName + "." + jobSpec.ID

		pName := pluginName
		jHandler := jobSpec.Handler
		jID := jobSpec.ID

		sched.RegisterHandler(handlerName, func(ctx context.Context, job *PlatformScheduledJob) error {
			_, err := mgr.Call(ctx, pName, jHandler, nil)
			if err != nil {
				log.Printf("🔌 Plugin job %s.%s failed: %v", pName, jID, err)
				return err
			}
			log.Printf("🔌 Plugin job %s.%s completed", pName, jID)
			return nil
		})

		var timeoutSecs int
		if jobSpec.Timeout != "" {
			if d, err := time.ParseDuration(jobSpec.Timeout); err == nil {
				timeoutSecs = int(d.Seconds())
			}
		}
		if timeoutSecs == 0 {
			timeoutSecs = 300
		}

		scheduledJob := &PlatformScheduledJob{
			Slug:           handlerName,
			Handler:        handlerName,
			Name:           pluginName + ": " + jobSpec.Description,
			Schedule:       jobSpec.Schedule,
			TimeoutSeconds: timeoutSecs,
		}

		if err := sched.AddJob(scheduledJob); err != nil {
			log.Printf("⚠️  Failed to register plugin job %s: %v", handlerName, err)
			continue
		}

		log.Printf("🔌 Registered plugin job: %s (%s)", handlerName, jobSpec.Schedule)
		registered++
	}

	return registered
}
