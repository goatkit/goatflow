package main

import (
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/models"
)

func TestBuildSchedulerJobsFromConfigDefaultsWhenNil(t *testing.T) {
	jobs := buildSchedulerJobsFromConfig(nil)
	if len(jobs) == 0 {
		t.Fatalf("expected default jobs, got none")
	}
	job := findJobBySlug(jobs, "email-ingest")
	if job == nil {
		t.Fatalf("expected email-ingest job by default")
	}
	if job.Schedule != "*/2 * * * *" {
		t.Fatalf("unexpected default schedule: %s", job.Schedule)
	}
}

func TestBuildSchedulerJobsFromConfigDisablesInboundJob(t *testing.T) {
	cfg := &config.Config{}
	cfg.Email.Inbound.Enabled = false

	jobs := buildSchedulerJobsFromConfig(cfg)
	if job := findJobBySlug(jobs, "email-ingest"); job != nil {
		t.Fatalf("expected email-ingest job to be removed when inbound disabled")
	}
}

func TestBuildSchedulerJobsFromConfigAppliesOverrides(t *testing.T) {
	cfg := &config.Config{}
	cfg.Email.Inbound.Enabled = true
	cfg.Email.Inbound.PollInterval = 30 * time.Second
	cfg.Email.Inbound.WorkerCount = 4
	cfg.Email.Inbound.MaxAccounts = 9

	jobs := buildSchedulerJobsFromConfig(cfg)
	job := findJobBySlug(jobs, "email-ingest")
	if job == nil {
		t.Fatalf("expected email-ingest job present")
	}
	if job.Schedule != "@every 30s" {
		t.Fatalf("expected @every 30s schedule, got %s", job.Schedule)
	}
	if got := job.Config["worker_count"]; got != 4 {
		t.Fatalf("expected worker_count override, got %v", got)
	}
	if got := job.Config["max_accounts"]; got != 9 {
		t.Fatalf("expected max_accounts override, got %v", got)
	}
}

func findJobBySlug(jobs []*models.ScheduledJob, slug string) *models.ScheduledJob {
	for _, job := range jobs {
		if job != nil && job.Slug == slug {
			return job
		}
	}
	return nil
}
