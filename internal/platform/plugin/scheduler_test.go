package plugin

import (
	"context"
	"testing"
)

type mockScheduler struct {
	PlatformScheduler
	jobs     []*PlatformScheduledJob
	handlers map[string]func(ctx context.Context, job *PlatformScheduledJob) error
}

func newMockScheduler() *mockScheduler {
	return &mockScheduler{
		handlers: make(map[string]func(ctx context.Context, job *PlatformScheduledJob) error),
	}
}

func (m *mockScheduler) AddJob(job *PlatformScheduledJob) error {
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *mockScheduler) RegisterHandler(name string, handler func(ctx context.Context, job *PlatformScheduledJob) error) {
	m.handlers[name] = handler
}

func TestRegisterPluginJobs(t *testing.T) {
	t.Run("Nil manager returns 0", func(t *testing.T) {
		count := RegisterPluginJobs(nil, newMockScheduler())
		if count != 0 {
			t.Errorf("expected 0, got %d", count)
		}
	})

	t.Run("Nil scheduler returns 0", func(t *testing.T) {
		mgr := NewManager(nil)
		count := RegisterPluginJobs(mgr, nil)
		if count != 0 {
			t.Errorf("expected 0, got %d", count)
		}
	})
}

func TestJobSpec(t *testing.T) {
	job := JobSpec{
		ID:          "test-job",
		Schedule:    "0 * * * *",
		Handler:     "my_handler",
		Description: "Test job",
		Enabled:     true,
		Timeout:     "5m",
	}

	if job.ID != "test-job" {
		t.Errorf("expected test-job, got %s", job.ID)
	}
	if job.Schedule != "0 * * * *" {
		t.Errorf("expected 0 * * * *, got %s", job.Schedule)
	}
	if !job.Enabled {
		t.Error("expected enabled")
	}
}
