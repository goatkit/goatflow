package runner

import (
	"context"
	"time"
)

// Task represents a background task that can be scheduled.
type Task interface {
	// Name returns the unique name of the task
	Name() string

	// Schedule returns the cron schedule expression for this task
	Schedule() string

	// Run executes the task
	Run(ctx context.Context) error

	// Timeout returns the maximum time this task should run
	Timeout() time.Duration
}

// TaskRegistry holds all registered tasks.
type TaskRegistry struct {
	tasks map[string]Task
}

// NewTaskRegistry creates a new task registry.
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks: make(map[string]Task),
	}
}

// Register adds a task to the registry.
func (r *TaskRegistry) Register(task Task) {
	r.tasks[task.Name()] = task
}

// Get returns a task by name.
func (r *TaskRegistry) Get(name string) (Task, bool) {
	task, exists := r.tasks[name]
	return task, exists
}

// All returns all registered tasks.
func (r *TaskRegistry) All() map[string]Task {
	return r.tasks
}
