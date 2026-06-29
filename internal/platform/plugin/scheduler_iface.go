package plugin

import "context"

// PlatformScheduledJob is the platform-level representation of a scheduled job.
// It contains only the fields needed by the plugin runtime, not the full product model.
type PlatformScheduledJob struct {
	Slug           string
	Handler        string
	Name           string
	Schedule       string
	TimeoutSeconds int
}

// PlatformScheduler is the interface that the plugin runtime uses to register
// jobs with the scheduler. The product layer provides an adapter that translates
// these calls into the product's scheduler.Service.
type PlatformScheduler interface {
	AddJob(job *PlatformScheduledJob) error
	RegisterHandler(name string, handler func(ctx context.Context, job *PlatformScheduledJob) error)
}
