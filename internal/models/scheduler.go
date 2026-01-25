package models

import "time"

// ScheduledJob represents an in-memory background job definition.
type ScheduledJob struct {
	Name           string
	Slug           string
	Handler        string
	Schedule       string
	TimeoutSeconds int
	Config         map[string]any
	RunOnStartup   bool // If true, execute immediately when scheduler starts
	LastRunAt      *time.Time
	NextRunAt      *time.Time
	LastStatus     string
	ErrorMessage   *string
	LastDurationMS int64
}

// Clone returns a deep copy of the job so schedule mutations stay isolated.
func (j *ScheduledJob) Clone() *ScheduledJob {
	if j == nil {
		return nil
	}
	copy := *j
	if j.Config != nil {
		cfg := make(map[string]any, len(j.Config))
		for k, v := range j.Config {
			cfg[k] = v
		}
		copy.Config = cfg
	}
	if j.LastRunAt != nil {
		lr := *j.LastRunAt
		copy.LastRunAt = &lr
	}
	if j.NextRunAt != nil {
		nr := *j.NextRunAt
		copy.NextRunAt = &nr
	}
	if j.ErrorMessage != nil {
		err := *j.ErrorMessage
		copy.ErrorMessage = &err
	}
	return &copy
}
