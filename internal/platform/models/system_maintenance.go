package models

import (
	"time"
)

// SystemMaintenance represents a scheduled system maintenance window.
// This maps directly to the OTRS system_maintenance table.
type SystemMaintenance struct {
	ID               int       `json:"id"`
	StartDate        int64     `json:"start_date"`         // Unix epoch timestamp
	StopDate         int64     `json:"stop_date"`          // Unix epoch timestamp
	Comments         string    `json:"comments"`           // Admin reference/description
	LoginMessage     *string   `json:"login_message"`      // Message shown on login page
	ShowLoginMessage int       `json:"show_login_message"` // 0 or 1
	NotifyMessage    *string   `json:"notify_message"`     // Notification banner message
	ValidID          int       `json:"valid_id"`           // 1=valid, 2=invalid
	CreateTime       time.Time `json:"create_time"`
	CreateBy         int       `json:"create_by"`
	ChangeTime       time.Time `json:"change_time"`
	ChangeBy         int       `json:"change_by"`
}

// StartDateFormatted returns the start date as a formatted string for display.
func (m *SystemMaintenance) StartDateFormatted() string {
	return time.Unix(m.StartDate, 0).Format("2006-01-02 15:04")
}

// StopDateFormatted returns the stop date as a formatted string for display.
func (m *SystemMaintenance) StopDateFormatted() string {
	return time.Unix(m.StopDate, 0).Format("2006-01-02 15:04")
}

// StartDateInput returns the start date formatted for HTML datetime-local input.
func (m *SystemMaintenance) StartDateInput() string {
	return time.Unix(m.StartDate, 0).Format("2006-01-02T15:04")
}

// StopDateInput returns the stop date formatted for HTML datetime-local input.
func (m *SystemMaintenance) StopDateInput() string {
	return time.Unix(m.StopDate, 0).Format("2006-01-02T15:04")
}

// IsCurrentlyActive returns true if the maintenance is currently active.
func (m *SystemMaintenance) IsCurrentlyActive() bool {
	now := time.Now().Unix()
	return m.ValidID == 1 && m.StartDate <= now && m.StopDate >= now
}

// IsUpcoming returns true if the maintenance starts within the given minutes.
func (m *SystemMaintenance) IsUpcoming(withinMinutes int) bool {
	now := time.Now().Unix()
	futureThreshold := now + int64(withinMinutes*60)
	return m.ValidID == 1 && m.StartDate > now && m.StartDate <= futureThreshold
}

// IsPast returns true if the maintenance window has ended.
func (m *SystemMaintenance) IsPast() bool {
	return m.StopDate < time.Now().Unix()
}

// Duration returns the maintenance duration in minutes.
func (m *SystemMaintenance) Duration() int {
	return int((m.StopDate - m.StartDate) / 60)
}

// GetLoginMessage returns the login message or empty string if nil.
func (m *SystemMaintenance) GetLoginMessage() string {
	if m.LoginMessage == nil {
		return ""
	}
	return *m.LoginMessage
}

// GetNotifyMessage returns the notify message or empty string if nil.
func (m *SystemMaintenance) GetNotifyMessage() string {
	if m.NotifyMessage == nil {
		return ""
	}
	return *m.NotifyMessage
}

// ShowsLoginMessage returns true if the login message should be displayed.
func (m *SystemMaintenance) ShowsLoginMessage() bool {
	return m.ShowLoginMessage == 1
}

// IsValid returns true if the maintenance record is valid.
func (m *SystemMaintenance) IsValid() bool {
	return m.ValidID == 1
}
