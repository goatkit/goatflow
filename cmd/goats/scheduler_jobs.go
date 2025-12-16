package main

import (
	"fmt"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/services/scheduler"
)

func buildSchedulerJobsFromConfig(cfg *config.Config) []*models.ScheduledJob {
	jobs := scheduler.DefaultJobs()
	if cfg == nil {
		return jobs
	}

	inbound := cfg.Email.Inbound
	if !inbound.Enabled {
		return filterJobsBySlug(jobs, "email-ingest")
	}

	for _, job := range jobs {
		if job == nil || job.Slug != "email-ingest" {
			continue
		}
		if inbound.PollInterval > 0 {
			job.Schedule = fmt.Sprintf("@every %s", inbound.PollInterval)
		}
		if job.Config == nil {
			job.Config = make(map[string]any)
		}
		if inbound.WorkerCount > 0 {
			job.Config["worker_count"] = inbound.WorkerCount
		}
		if inbound.MaxAccounts > 0 {
			job.Config["max_accounts"] = inbound.MaxAccounts
		}
	}

	return jobs
}

func filterJobsBySlug(jobs []*models.ScheduledJob, slug string) []*models.ScheduledJob {
	if slug == "" || len(jobs) == 0 {
		return jobs
	}
	filtered := make([]*models.ScheduledJob, 0, len(jobs))
	for _, job := range jobs {
		if job == nil || job.Slug == slug {
			continue
		}
		filtered = append(filtered, job)
	}
	return filtered
}
