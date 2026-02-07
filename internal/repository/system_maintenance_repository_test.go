package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/goatkit/goatflow/internal/models"
)

// TestSystemMaintenanceModel tests the model struct and its methods
func TestSystemMaintenanceModel(t *testing.T) {
	t.Run("StartDateFormatted", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC).Unix(),
		}
		formatted := m.StartDateFormatted()
		assert.Equal(t, "2025-01-15 10:30", formatted)
	})

	t.Run("StopDateFormatted", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StopDate: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC).Unix(),
		}
		formatted := m.StopDateFormatted()
		assert.Equal(t, "2025-01-15 12:00", formatted)
	})

	t.Run("IsCurrentlyActive_Active", func(t *testing.T) {
		now := time.Now().Unix()
		m := &models.SystemMaintenance{
			StartDate: now - 3600, // 1 hour ago
			StopDate:  now + 3600, // 1 hour from now
			ValidID:   1,
		}
		assert.True(t, m.IsCurrentlyActive())
	})

	t.Run("IsCurrentlyActive_NotActive_Future", func(t *testing.T) {
		now := time.Now().Unix()
		m := &models.SystemMaintenance{
			StartDate: now + 3600, // 1 hour from now
			StopDate:  now + 7200, // 2 hours from now
			ValidID:   1,
		}
		assert.False(t, m.IsCurrentlyActive())
	})

	t.Run("IsCurrentlyActive_NotActive_Past", func(t *testing.T) {
		now := time.Now().Unix()
		m := &models.SystemMaintenance{
			StartDate: now - 7200, // 2 hours ago
			StopDate:  now - 3600, // 1 hour ago
			ValidID:   1,
		}
		assert.False(t, m.IsCurrentlyActive())
	})

	t.Run("IsCurrentlyActive_Invalid", func(t *testing.T) {
		now := time.Now().Unix()
		m := &models.SystemMaintenance{
			StartDate: now - 3600,
			StopDate:  now + 3600,
			ValidID:   2, // Invalid
		}
		assert.False(t, m.IsCurrentlyActive())
	})

	t.Run("IsUpcoming_True", func(t *testing.T) {
		now := time.Now().Unix()
		m := &models.SystemMaintenance{
			StartDate: now + 1200, // 20 minutes from now
			StopDate:  now + 7200,
			ValidID:   1,
		}
		assert.True(t, m.IsUpcoming(30)) // Within 30 minutes
	})

	t.Run("IsUpcoming_False_TooFar", func(t *testing.T) {
		now := time.Now().Unix()
		m := &models.SystemMaintenance{
			StartDate: now + 7200, // 2 hours from now
			StopDate:  now + 10800,
			ValidID:   1,
		}
		assert.False(t, m.IsUpcoming(30)) // Not within 30 minutes
	})

	t.Run("IsUpcoming_False_AlreadyActive", func(t *testing.T) {
		now := time.Now().Unix()
		m := &models.SystemMaintenance{
			StartDate: now - 1200, // Already started
			StopDate:  now + 3600,
			ValidID:   1,
		}
		assert.False(t, m.IsUpcoming(30))
	})

	t.Run("IsPast", func(t *testing.T) {
		now := time.Now().Unix()

		pastM := &models.SystemMaintenance{StopDate: now - 3600}
		assert.True(t, pastM.IsPast())

		futureM := &models.SystemMaintenance{StopDate: now + 3600}
		assert.False(t, futureM.IsPast())
	})

	t.Run("Duration", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: 1000,
			StopDate:  4600, // 3600 seconds (60 minutes) later
		}
		assert.Equal(t, 60, m.Duration())
	})

	t.Run("GetLoginMessage_Nil", func(t *testing.T) {
		m := &models.SystemMaintenance{LoginMessage: nil}
		assert.Equal(t, "", m.GetLoginMessage())
	})

	t.Run("GetLoginMessage_Value", func(t *testing.T) {
		msg := "System is down for maintenance"
		m := &models.SystemMaintenance{LoginMessage: &msg}
		assert.Equal(t, "System is down for maintenance", m.GetLoginMessage())
	})

	t.Run("GetNotifyMessage_Nil", func(t *testing.T) {
		m := &models.SystemMaintenance{NotifyMessage: nil}
		assert.Equal(t, "", m.GetNotifyMessage())
	})

	t.Run("GetNotifyMessage_Value", func(t *testing.T) {
		msg := "Maintenance starting soon"
		m := &models.SystemMaintenance{NotifyMessage: &msg}
		assert.Equal(t, "Maintenance starting soon", m.GetNotifyMessage())
	})

	t.Run("ShowsLoginMessage", func(t *testing.T) {
		m1 := &models.SystemMaintenance{ShowLoginMessage: 1}
		assert.True(t, m1.ShowsLoginMessage())

		m0 := &models.SystemMaintenance{ShowLoginMessage: 0}
		assert.False(t, m0.ShowsLoginMessage())
	})

	t.Run("IsValid", func(t *testing.T) {
		mValid := &models.SystemMaintenance{ValidID: 1}
		assert.True(t, mValid.IsValid())

		mInvalid := &models.SystemMaintenance{ValidID: 2}
		assert.False(t, mInvalid.IsValid())
	})
}

// TestSystemMaintenanceModelFields tests field assignment
func TestSystemMaintenanceModelFields(t *testing.T) {
	loginMsg := "Login message"
	notifyMsg := "Notify message"
	now := time.Now()

	m := &models.SystemMaintenance{
		ID:               1,
		StartDate:        1704067200, // 2024-01-01 00:00:00 UTC
		StopDate:         1704070800, // 2024-01-01 01:00:00 UTC
		Comments:         "Monthly maintenance",
		LoginMessage:     &loginMsg,
		ShowLoginMessage: 1,
		NotifyMessage:    &notifyMsg,
		ValidID:          1,
		CreateTime:       now,
		CreateBy:         1,
		ChangeTime:       now,
		ChangeBy:         1,
	}

	assert.Equal(t, 1, m.ID)
	assert.Equal(t, int64(1704067200), m.StartDate)
	assert.Equal(t, int64(1704070800), m.StopDate)
	assert.Equal(t, "Monthly maintenance", m.Comments)
	assert.Equal(t, &loginMsg, m.LoginMessage)
	assert.Equal(t, 1, m.ShowLoginMessage)
	assert.Equal(t, &notifyMsg, m.NotifyMessage)
	assert.Equal(t, 1, m.ValidID)
	assert.Equal(t, 1, m.CreateBy)
	assert.Equal(t, 1, m.ChangeBy)
}
