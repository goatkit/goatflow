package models

import (
	"strconv"
	"strings"
	"time"
)

// GenericAgentJob represents a generic agent job configuration.
// Jobs are stored as key-value pairs in the database (job_name, job_key, job_value).
type GenericAgentJob struct {
	Name      string            `json:"name"`
	Valid     bool              `json:"valid"`
	Config    map[string]string `json:"config"`
	LastRunAt *time.Time        `json:"last_run_at,omitempty"`
}

// ScheduleDays returns the days of week when this job should run (0=Sun, 6=Sat).
func (j *GenericAgentJob) ScheduleDays() []int {
	return j.getIntSlice("ScheduleDays")
}

// ScheduleHours returns the hours when this job should run (0-23).
func (j *GenericAgentJob) ScheduleHours() []int {
	return j.getIntSlice("ScheduleHours")
}

// ScheduleMinutes returns the minutes when this job should run (0-59).
func (j *GenericAgentJob) ScheduleMinutes() []int {
	return j.getIntSlice("ScheduleMinutes")
}

// ScheduleLastRun returns the last run time from config.
func (j *GenericAgentJob) ScheduleLastRun() *time.Time {
	if j.LastRunAt != nil {
		return j.LastRunAt
	}
	val := j.Config["ScheduleLastRun"]
	if val == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return nil
	}
	return &t
}

// MatchCriteria returns the ticket search criteria from the job config.
func (j *GenericAgentJob) MatchCriteria() *GenericAgentMatchCriteria {
	return &GenericAgentMatchCriteria{config: j.Config}
}

// Actions returns the actions to apply from the job config.
func (j *GenericAgentJob) Actions() *GenericAgentActions {
	return &GenericAgentActions{config: j.Config}
}

// getIntSlice parses a comma-separated or OTRS-style array from config.
func (j *GenericAgentJob) getIntSlice(key string) []int {
	var result []int

	// OTRS stores arrays as key[0], key[1], etc.
	for i := 0; i < 60; i++ {
		k := key + "[" + strconv.Itoa(i) + "]"
		if val, ok := j.Config[k]; ok {
			if n, err := strconv.Atoi(val); err == nil {
				result = append(result, n)
			}
		}
	}
	if len(result) > 0 {
		return result
	}

	// Also support comma-separated format
	if val, ok := j.Config[key]; ok && val != "" {
		parts := strings.Split(val, ",")
		for _, p := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				result = append(result, n)
			}
		}
	}

	return result
}

// GenericAgentMatchCriteria represents ticket search criteria.
type GenericAgentMatchCriteria struct {
	config map[string]string
}

// StateIDs returns state IDs to match.
func (c *GenericAgentMatchCriteria) StateIDs() []int {
	return c.getIntSlice("StateIDs")
}

// QueueIDs returns queue IDs to match.
func (c *GenericAgentMatchCriteria) QueueIDs() []int {
	return c.getIntSlice("QueueIDs")
}

// PriorityIDs returns priority IDs to match.
func (c *GenericAgentMatchCriteria) PriorityIDs() []int {
	return c.getIntSlice("PriorityIDs")
}

// TypeIDs returns type IDs to match.
func (c *GenericAgentMatchCriteria) TypeIDs() []int {
	return c.getIntSlice("TypeIDs")
}

// LockIDs returns lock IDs to match.
func (c *GenericAgentMatchCriteria) LockIDs() []int {
	return c.getIntSlice("LockIDs")
}

// OwnerIDs returns owner user IDs to match.
func (c *GenericAgentMatchCriteria) OwnerIDs() []int {
	return c.getIntSlice("OwnerIDs")
}

// ServiceIDs returns service IDs to match.
func (c *GenericAgentMatchCriteria) ServiceIDs() []int {
	return c.getIntSlice("ServiceIDs")
}

// SLAIDs returns SLA IDs to match.
func (c *GenericAgentMatchCriteria) SLAIDs() []int {
	return c.getIntSlice("SLAIDs")
}

// CustomerID returns customer ID pattern to match.
func (c *GenericAgentMatchCriteria) CustomerID() string {
	return c.config["CustomerID"]
}

// CustomerUserLogin returns customer user login to match.
func (c *GenericAgentMatchCriteria) CustomerUserLogin() string {
	return c.config["CustomerUserLogin"]
}

// Title returns title pattern to match.
func (c *GenericAgentMatchCriteria) Title() string {
	return c.config["Title"]
}

// TicketCreateTimeOlderMinutes returns the age filter in minutes.
func (c *GenericAgentMatchCriteria) TicketCreateTimeOlderMinutes() int {
	return c.getInt("TicketCreateTimeOlderMinutes")
}

// TicketCreateTimeNewerMinutes returns the recency filter in minutes.
func (c *GenericAgentMatchCriteria) TicketCreateTimeNewerMinutes() int {
	return c.getInt("TicketCreateTimeNewerMinutes")
}

// TicketChangeTimeOlderMinutes returns the last change age filter.
func (c *GenericAgentMatchCriteria) TicketChangeTimeOlderMinutes() int {
	return c.getInt("TicketChangeTimeOlderMinutes")
}

// TicketChangeTimeNewerMinutes returns the last change recency filter.
func (c *GenericAgentMatchCriteria) TicketChangeTimeNewerMinutes() int {
	return c.getInt("TicketChangeTimeNewerMinutes")
}

// TicketPendingTimeOlderMinutes returns pending time age filter.
func (c *GenericAgentMatchCriteria) TicketPendingTimeOlderMinutes() int {
	return c.getInt("TicketPendingTimeOlderMinutes")
}

// TicketPendingTimeNewerMinutes returns pending time recency filter.
func (c *GenericAgentMatchCriteria) TicketPendingTimeNewerMinutes() int {
	return c.getInt("TicketPendingTimeNewerMinutes")
}

// TicketEscalationTimeOlderMinutes returns escalation time age filter.
func (c *GenericAgentMatchCriteria) TicketEscalationTimeOlderMinutes() int {
	return c.getInt("TicketEscalationTimeOlderMinutes")
}

// TicketEscalationTimeNewerMinutes returns escalation time recency filter.
func (c *GenericAgentMatchCriteria) TicketEscalationTimeNewerMinutes() int {
	return c.getInt("TicketEscalationTimeNewerMinutes")
}

// HasCriteria returns true if any match criteria are defined.
func (c *GenericAgentMatchCriteria) HasCriteria() bool {
	return len(c.StateIDs()) > 0 ||
		len(c.QueueIDs()) > 0 ||
		len(c.PriorityIDs()) > 0 ||
		len(c.TypeIDs()) > 0 ||
		len(c.LockIDs()) > 0 ||
		len(c.OwnerIDs()) > 0 ||
		len(c.ServiceIDs()) > 0 ||
		len(c.SLAIDs()) > 0 ||
		c.CustomerID() != "" ||
		c.CustomerUserLogin() != "" ||
		c.Title() != "" ||
		c.TicketCreateTimeOlderMinutes() > 0 ||
		c.TicketCreateTimeNewerMinutes() > 0 ||
		c.TicketChangeTimeOlderMinutes() > 0 ||
		c.TicketChangeTimeNewerMinutes() > 0 ||
		c.TicketPendingTimeOlderMinutes() > 0 ||
		c.TicketPendingTimeNewerMinutes() > 0 ||
		c.TicketEscalationTimeOlderMinutes() > 0 ||
		c.TicketEscalationTimeNewerMinutes() > 0
}

func (c *GenericAgentMatchCriteria) getIntSlice(key string) []int {
	var result []int
	// OTRS array format: key[0], key[1], etc.
	for i := 0; i < 100; i++ {
		k := key + "[" + strconv.Itoa(i) + "]"
		if val, ok := c.config[k]; ok {
			if n, err := strconv.Atoi(val); err == nil {
				result = append(result, n)
			}
		}
	}
	if len(result) > 0 {
		return result
	}
	// Comma-separated format
	if val, ok := c.config[key]; ok && val != "" {
		parts := strings.Split(val, ",")
		for _, p := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				result = append(result, n)
			}
		}
	}
	return result
}

func (c *GenericAgentMatchCriteria) getInt(key string) int {
	if val, ok := c.config[key]; ok {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return 0
}

// GenericAgentActions represents actions to apply to matching tickets.
type GenericAgentActions struct {
	config map[string]string
}

// NewStateID returns the new state ID to set, or nil if not specified.
func (a *GenericAgentActions) NewStateID() *int {
	return a.getIntPtr("NewStateID")
}

// NewQueueID returns the new queue ID to set.
func (a *GenericAgentActions) NewQueueID() *int {
	return a.getIntPtr("NewQueueID")
}

// NewPriorityID returns the new priority ID to set.
func (a *GenericAgentActions) NewPriorityID() *int {
	return a.getIntPtr("NewPriorityID")
}

// NewOwnerID returns the new owner user ID to set.
func (a *GenericAgentActions) NewOwnerID() *int {
	return a.getIntPtr("NewOwnerID")
}

// NewResponsibleID returns the new responsible user ID to set.
func (a *GenericAgentActions) NewResponsibleID() *int {
	return a.getIntPtr("NewResponsibleID")
}

// NewLockID returns the new lock ID to set.
func (a *GenericAgentActions) NewLockID() *int {
	return a.getIntPtr("NewLockID")
}

// NewTypeID returns the new type ID to set.
func (a *GenericAgentActions) NewTypeID() *int {
	return a.getIntPtr("NewTypeID")
}

// NewServiceID returns the new service ID to set.
func (a *GenericAgentActions) NewServiceID() *int {
	return a.getIntPtr("NewServiceID")
}

// NewSLAID returns the new SLA ID to set.
func (a *GenericAgentActions) NewSLAID() *int {
	return a.getIntPtr("NewSLAID")
}

// NewCustomerID returns the new customer ID to set.
func (a *GenericAgentActions) NewCustomerID() string {
	return a.config["NewCustomerID"]
}

// NewCustomerUserLogin returns the new customer user login to set.
func (a *GenericAgentActions) NewCustomerUserLogin() string {
	return a.config["NewCustomerUserLogin"]
}

// NewTitle returns the new title to set.
func (a *GenericAgentActions) NewTitle() string {
	return a.config["NewTitle"]
}

// NoteBody returns the note body to add.
func (a *GenericAgentActions) NoteBody() string {
	return a.config["NewNoteBody"]
}

// NoteSubject returns the note subject to add.
func (a *GenericAgentActions) NoteSubject() string {
	return a.config["NewNoteSubject"]
}

// NewPendingTime returns the pending time to set.
func (a *GenericAgentActions) NewPendingTime() *time.Time {
	val := a.config["NewPendingTime"]
	if val == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		// Try other formats
		t, err = time.Parse("2006-01-02 15:04:05", val)
		if err != nil {
			return nil
		}
	}
	return &t
}

// NewPendingTimeDiff returns pending time offset in minutes.
func (a *GenericAgentActions) NewPendingTimeDiff() int {
	return a.getInt("NewPendingTimeDiff")
}

// Delete returns true if tickets should be deleted.
func (a *GenericAgentActions) Delete() bool {
	return a.config["NewDelete"] == "1"
}

// HasActions returns true if any actions are defined.
func (a *GenericAgentActions) HasActions() bool {
	return a.NewStateID() != nil ||
		a.NewQueueID() != nil ||
		a.NewPriorityID() != nil ||
		a.NewOwnerID() != nil ||
		a.NewResponsibleID() != nil ||
		a.NewLockID() != nil ||
		a.NewTypeID() != nil ||
		a.NewServiceID() != nil ||
		a.NewSLAID() != nil ||
		a.NewCustomerID() != "" ||
		a.NewCustomerUserLogin() != "" ||
		a.NewTitle() != "" ||
		a.NoteBody() != "" ||
		a.NewPendingTime() != nil ||
		a.NewPendingTimeDiff() != 0 ||
		a.Delete()
}

func (a *GenericAgentActions) getIntPtr(key string) *int {
	if val, ok := a.config[key]; ok && val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return &n
		}
	}
	return nil
}

func (a *GenericAgentActions) getInt(key string) int {
	if val, ok := a.config[key]; ok {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return 0
}

// DynamicFieldValues returns dynamic field values to set.
// Keys are field names (without DynamicField_ prefix), values are the values to set.
func (a *GenericAgentActions) DynamicFieldValues() map[string]string {
	result := make(map[string]string)
	prefix := "DynamicField_"
	for k, v := range a.config {
		if strings.HasPrefix(k, prefix) {
			fieldName := strings.TrimPrefix(k, prefix)
			if fieldName != "" {
				result[fieldName] = v
			}
		}
	}
	return result
}
