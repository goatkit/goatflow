package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// GenericAgentRepository handles database operations for generic agent jobs.
type GenericAgentRepository struct {
	db *sql.DB
}

// NewGenericAgentRepository creates a new generic agent repository.
func NewGenericAgentRepository(db *sql.DB) *GenericAgentRepository {
	return &GenericAgentRepository{db: db}
}

// GetValidJobs returns all valid (enabled) generic agent jobs.
func (r *GenericAgentRepository) GetValidJobs(ctx context.Context) ([]*models.GenericAgentJob, error) {
	return r.getJobsWithFilter(ctx, true)
}

// GetAllJobs returns all generic agent jobs regardless of validity.
func (r *GenericAgentRepository) GetAllJobs(ctx context.Context) ([]*models.GenericAgentJob, error) {
	return r.getJobsWithFilter(ctx, false)
}

// getJobsWithFilter retrieves jobs with optional validity filter.
func (r *GenericAgentRepository) getJobsWithFilter(ctx context.Context, validOnly bool) ([]*models.GenericAgentJob, error) {
	// Get all unique job names
	query := database.ConvertPlaceholders(`SELECT DISTINCT job_name FROM generic_agent_jobs ORDER BY job_name`)
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		jobNames = append(jobNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build job details for each job
	var jobs []*models.GenericAgentJob
	for _, jobName := range jobNames {
		job, err := r.GetJob(ctx, jobName)
		if err != nil {
			continue
		}
		if validOnly && !job.Valid {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetJob returns a single job by name.
func (r *GenericAgentRepository) GetJob(ctx context.Context, name string) (*models.GenericAgentJob, error) {
	query := database.ConvertPlaceholders(`
		SELECT job_key, COALESCE(job_value, '')
		FROM generic_agent_jobs
		WHERE job_name = ?
	`)

	rows, err := r.db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	job := &models.GenericAgentJob{
		Name:   name,
		Valid:  true, // Default to valid
		Config: make(map[string]string),
	}

	found := false
	for rows.Next() {
		found = true
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		job.Config[key] = value
		if key == "Valid" && value == "0" {
			job.Valid = false
		}
		if key == "ScheduleLastRun" && value != "" {
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				job.LastRunAt = &t
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if !found {
		return nil, sql.ErrNoRows
	}

	return job, nil
}

// UpdateLastRun updates the last run time for a job.
func (r *GenericAgentRepository) UpdateLastRun(ctx context.Context, name string, runTime time.Time) error {
	// Delete existing ScheduleLastRun key and insert new one
	deleteQuery := database.ConvertPlaceholders(`
		DELETE FROM generic_agent_jobs
		WHERE job_name = ? AND job_key = 'ScheduleLastRun'
	`)
	if _, err := r.db.ExecContext(ctx, deleteQuery, name); err != nil {
		return err
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO generic_agent_jobs (job_name, job_key, job_value)
		VALUES (?, 'ScheduleLastRun', ?)
	`)
	_, err := r.db.ExecContext(ctx, insertQuery, name, runTime.Format(time.RFC3339))
	return err
}

// SetConfigValue sets a single config key-value for a job.
func (r *GenericAgentRepository) SetConfigValue(ctx context.Context, name, key, value string) error {
	// Upsert pattern: delete then insert
	deleteQuery := database.ConvertPlaceholders(`
		DELETE FROM generic_agent_jobs
		WHERE job_name = ? AND job_key = ?
	`)
	if _, err := r.db.ExecContext(ctx, deleteQuery, name, key); err != nil {
		return err
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO generic_agent_jobs (job_name, job_key, job_value)
		VALUES (?, ?, ?)
	`)
	_, err := r.db.ExecContext(ctx, insertQuery, name, key, value)
	return err
}

// JobExists checks if a job with the given name exists.
func (r *GenericAgentRepository) JobExists(ctx context.Context, name string) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(SELECT 1 FROM generic_agent_jobs WHERE job_name = ? LIMIT 1)
	`)
	var exists bool
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	return exists, err
}
