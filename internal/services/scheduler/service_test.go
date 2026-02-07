package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/goatkit/goatflow/internal/models"
)

func TestScheduleJobsRegistersEntries(t *testing.T) {
	job := &models.ScheduledJob{Slug: "test", Handler: "noop", Schedule: "* * * * *"}
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	svc := NewService(nil,
		WithJobs([]*models.ScheduledJob{job}),
		WithCron(cronEngine),
	)
	t.Cleanup(func() { cronEngine.Stop() })

	svc.RegisterHandler("noop", func(ctx context.Context, j *models.ScheduledJob) error { return nil })
	svc.scheduleAllJobs()

	if _, ok := svc.entries["test"]; !ok {
		t.Fatalf("expected entry for job slug test")
	}
}

func TestExecuteJobSuccessUpdatesState(t *testing.T) {
	job := &models.ScheduledJob{Slug: "run", Handler: "test", Schedule: "* * * * *"}
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	svc := NewService(nil,
		WithJobs([]*models.ScheduledJob{job}),
		WithCron(cronEngine),
	)
	t.Cleanup(func() { cronEngine.Stop() })

	var ran int32
	svc.RegisterHandler("test", func(ctx context.Context, j *models.ScheduledJob) error {
		atomic.AddInt32(&ran, 1)
		return nil
	})

	svc.scheduleAllJobs()
	entry, ok := svc.entries["run"]
	if !ok {
		t.Fatalf("missing entry for job")
	}

	svc.executeJob("run", entry)

	if atomic.LoadInt32(&ran) != 1 {
		t.Fatalf("expected handler to run once")
	}
	state := svc.jobSnapshot("run")
	if state == nil {
		t.Fatalf("expected job state")
	}
	if state.LastStatus != statusSuccess {
		t.Fatalf("expected status success, got %s", state.LastStatus)
	}
	if state.LastRunAt == nil {
		t.Fatalf("expected last run timestamp")
	}
	if state.ErrorMessage != nil {
		t.Fatalf("unexpected error message: %s", *state.ErrorMessage)
	}
}

func TestExecuteJobMissingHandlerMarksFailure(t *testing.T) {
	job := &models.ScheduledJob{Slug: "missing", Handler: "unknown", Schedule: "* * * * *"}
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	svc := NewService(nil,
		WithJobs([]*models.ScheduledJob{job}),
		WithCron(cronEngine),
	)
	t.Cleanup(func() { cronEngine.Stop() })

	svc.scheduleAllJobs()
	entry, ok := svc.entries["missing"]
	if !ok {
		t.Fatalf("missing entry for job")
	}

	svc.executeJob("missing", entry)
	state := svc.jobSnapshot("missing")
	if state == nil {
		t.Fatalf("expected job state")
	}
	if state.LastStatus != statusFailed {
		t.Fatalf("expected failed status, got %s", state.LastStatus)
	}
	if state.ErrorMessage == nil {
		t.Fatalf("expected error message for missing handler")
	}
}

func TestWithLocationOverridesDefault(t *testing.T) {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatalf("expected to load test location: %v", err)
	}
	svc := NewService(nil, WithLocation(loc))
	now := svc.now()
	if now.Location() != loc {
		t.Fatalf("expected location %s, got %s", loc, now.Location())
	}
}
