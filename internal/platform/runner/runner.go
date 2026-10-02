// Package runner provides background task runner and lifecycle management.
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// DefaultStopTimeout bounds how long Start waits for running tasks after
// its context is cancelled when NewRunner gets no positive timeout.
const DefaultStopTimeout = 5 * time.Second

// Runner manages and executes scheduled background tasks.
type Runner struct {
	cron        *cron.Cron
	registry    *TaskRegistry
	logger      *slog.Logger
	stopTimeout time.Duration
}

// NewRunner creates a new task runner. stopTimeout bounds how long Start
// waits for running tasks to return once they have been cancelled.
// Log lines go through the process-wide slog logger, so they follow
// LOG_FORMAT, LOG_LEVEL and LOG_OUTPUT.
func NewRunner(registry *TaskRegistry, stopTimeout time.Duration) *Runner {
	if stopTimeout <= 0 {
		stopTimeout = DefaultStopTimeout
	}
	return &Runner{
		cron:        cron.New(cron.WithSeconds()),
		registry:    registry,
		logger:      slog.Default().With("component", "runner"),
		stopTimeout: stopTimeout,
	}
}

// Start schedules every registered task and blocks until ctx is cancelled
// (the caller cancels it on SIGTERM/SIGINT). Cancellation stops new runs,
// cancels the context of every running task and waits up to the stop
// timeout for them to return. It returns nil after a clean stop and an
// error when tasks were still running at the deadline.
func (r *Runner) Start(ctx context.Context) error {
	r.logger.Info("starting task runner")

	// Tasks run under taskCtx, which ends with ctx: a stop signal reaches
	// every running task immediately instead of after its own timeout.
	taskCtx, cancelTasks := context.WithCancel(ctx)
	defer cancelTasks()

	for name, task := range r.registry.All() {
		r.logger.Info("registering task", "task", name, "schedule", task.Schedule())

		_, err := r.cron.AddFunc(task.Schedule(), func() {
			r.executeTask(taskCtx, task)
		})
		if err != nil {
			return fmt.Errorf("failed to schedule task %s: %w", name, err)
		}
	}

	r.cron.Start()
	r.logger.Info("task runner started")

	<-ctx.Done()
	r.logger.Info("stopping task runner", "reason", context.Cause(ctx), "stop_timeout", r.stopTimeout.String())
	cancelTasks()
	return r.stop()
}

// executeTask runs a single task with timeout and error handling.
func (r *Runner) executeTask(ctx context.Context, task Task) {
	if ctx.Err() != nil {
		return
	}
	taskCtx, cancel := context.WithTimeout(ctx, task.Timeout())
	defer cancel()

	r.logger.Info("executing task", "task", task.Name())

	start := time.Now()
	err := task.Run(taskCtx)
	duration := time.Since(start)

	if err != nil {
		r.logger.Error("task failed", "task", task.Name(), "duration", duration.String(), "error", err)
	} else {
		r.logger.Info("task completed", "task", task.Name(), "duration", duration.String())
	}
}

// stop prevents new runs and waits up to the stop timeout for running tasks.
func (r *Runner) stop() error {
	done := r.cron.Stop() // done closes when every running job has returned
	select {
	case <-done.Done():
		r.logger.Info("task runner stopped")
		return nil
	case <-time.After(r.stopTimeout):
		r.logger.Error("tasks still running at stop timeout; exiting without them", "stop_timeout", r.stopTimeout.String())
		return fmt.Errorf("runner: tasks still running after %s", r.stopTimeout)
	}
}
