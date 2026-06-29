package scheduler

import (
	"context"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/plugin"
)

// PluginAdapter adapts the product scheduler.Service to the plugin runtime's
// PlatformScheduler interface, breaking the transitive import dependency from
// internal/plugin to internal/models and internal/services/scheduler.
type PluginAdapter struct {
	svc *Service
}

// NewPluginAdapter wraps a scheduler.Service as a PlatformScheduler.
func NewPluginAdapter(svc *Service) *PluginAdapter {
	return &PluginAdapter{svc: svc}
}

// AddJob translates a platform-level job to the product's ScheduledJob and
// delegates to the underlying scheduler.Service.
func (a *PluginAdapter) AddJob(job *plugin.PlatformScheduledJob) error {
	if job == nil {
		return nil
	}
	mj := &models.ScheduledJob{
		Slug:           job.Slug,
		Handler:        job.Handler,
		Name:           job.Name,
		Schedule:       job.Schedule,
		TimeoutSeconds: job.TimeoutSeconds,
	}
	return a.svc.AddJob(mj)
}

// RegisterHandler wraps a platform-level handler so it can be registered with
// the product scheduler.
func (a *PluginAdapter) RegisterHandler(name string, handler func(ctx context.Context, job *plugin.PlatformScheduledJob) error) {
	if handler == nil {
		a.svc.RegisterHandler(name, nil)
		return
	}
	a.svc.RegisterHandler(name, func(ctx context.Context, job *models.ScheduledJob) error {
		pj := &plugin.PlatformScheduledJob{
			Slug:           job.Slug,
			Handler:        job.Handler,
			Name:           job.Name,
			Schedule:       job.Schedule,
			TimeoutSeconds: job.TimeoutSeconds,
		}
		return handler(ctx, pj)
	})
}
