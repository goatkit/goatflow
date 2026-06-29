// Package runner provides background task runner and lifecycle management.
package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
)

// Runner manages and executes scheduled background tasks.
type Runner struct {
	cron     *cron.Cron
	registry *TaskRegistry
	logger   *log.Logger
	wg       sync.WaitGroup
}

// NewRunner creates a new task runner.
func NewRunner(registry *TaskRegistry) *Runner {
	return &Runner{
		cron:     cron.New(cron.WithSeconds()),
		registry: registry,
		logger:   log.New(os.Stdout, "[RUNNER] ", log.LstdFlags),
	}
}

// Start begins executing scheduled tasks.
func (r *Runner) Start(ctx context.Context) error {
	r.logger.Println("Starting task runner...")

	// Register all tasks with cron
	for name, task := range r.registry.All() {
		r.logger.Printf("Registering task: %s with schedule: %s", name, task.Schedule())

		_, err := r.cron.AddFunc(task.Schedule(), func() {
			r.executeTask(ctx, task)
		})
		if err != nil {
			return fmt.Errorf("failed to schedule task %s: %w", name, err)
		}
	}

	// Start the cron scheduler
	r.cron.Start()
	r.logger.Println("Task runner started successfully")

	// Wait for shutdown signal
	return r.waitForShutdown(ctx)
}

// executeTask runs a single task with timeout and error handling.
func (r *Runner) executeTask(ctx context.Context, task Task) {
	r.wg.Add(1)
	defer r.wg.Done()

	taskCtx, cancel := context.WithTimeout(ctx, task.Timeout())
	defer cancel()

	r.logger.Printf("Executing task: %s", task.Name())

	start := time.Now()
	err := task.Run(taskCtx)
	duration := time.Since(start)

	if err != nil {
		r.logger.Printf("Task %s failed after %v: %v", task.Name(), duration, err)
	} else {
		r.logger.Printf("Task %s completed successfully in %v", task.Name(), duration)
	}
}

// Stop gracefully shuts down the runner.
func (r *Runner) Stop() {
	r.logger.Println("Stopping task runner...")

	// Stop accepting new tasks
	ctx := r.cron.Stop()

	// Wait for running tasks to complete
	r.wg.Wait()

	r.logger.Println("Task runner stopped")
	<-ctx.Done()
}

// waitForShutdown waits for termination signals.
func (r *Runner) waitForShutdown(ctx context.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		r.logger.Printf("Received signal: %v", sig)
		r.Stop()
		return nil
	case <-ctx.Done():
		r.logger.Println("Context cancelled")
		r.Stop()
		return ctx.Err()
	}
}
