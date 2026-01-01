package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

func TestSLARepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateSLA", func(t *testing.T) {
		repo := NewMemorySLARepository()

		sla := &models.SLA{
			Name:              "Gold Support",
			Description:       "24x7 premium support",
			FirstResponseTime: 30,  // 30 minutes
			UpdateTime:        120, // 2 hours
			SolutionTime:      480, // 8 hours
			Priority:          5,
			IsActive:          true,
			Conditions: models.SLAConditions{
				Priorities: []int{4, 5}, // High and urgent
				Queues:     []uint{1, 2},
			},
		}

		err := repo.CreateSLA(ctx, sla)
		require.NoError(t, err)
		assert.NotZero(t, sla.ID)
		assert.NotZero(t, sla.CreatedAt)
	})

	t.Run("GetSLA", func(t *testing.T) {
		repo := NewMemorySLARepository()

		sla := &models.SLA{
			Name:              "Silver Support",
			FirstResponseTime: 60,
			UpdateTime:        240,
			SolutionTime:      960,
			Priority:          3,
			IsActive:          true,
		}
		repo.CreateSLA(ctx, sla)

		retrieved, err := repo.GetSLA(ctx, sla.ID)
		require.NoError(t, err)
		assert.Equal(t, "Silver Support", retrieved.Name)
		assert.Equal(t, 60, retrieved.FirstResponseTime)
	})

	t.Run("GetAllSLAs", func(t *testing.T) {
		repo := NewMemorySLARepository()

		slas := []models.SLA{
			{Name: "Gold", Priority: 5, IsActive: true},
			{Name: "Silver", Priority: 3, IsActive: true},
			{Name: "Bronze", Priority: 1, IsActive: true},
			{Name: "Inactive", Priority: 2, IsActive: false},
		}

		for i := range slas {
			repo.CreateSLA(ctx, &slas[i])
		}

		// Get all active SLAs
		activeSLAs, err := repo.GetAllSLAs(ctx, true)
		require.NoError(t, err)
		assert.Len(t, activeSLAs, 3)

		// Get all SLAs including inactive
		allSLAs, err := repo.GetAllSLAs(ctx, false)
		require.NoError(t, err)
		assert.Len(t, allSLAs, 4)
	})

	t.Run("UpdateSLA", func(t *testing.T) {
		repo := NewMemorySLARepository()

		sla := &models.SLA{
			Name:              "Standard",
			FirstResponseTime: 120,
			IsActive:          true,
		}
		repo.CreateSLA(ctx, sla)

		// Update SLA
		sla.FirstResponseTime = 60
		sla.Description = "Updated description"

		err := repo.UpdateSLA(ctx, sla)
		require.NoError(t, err)

		retrieved, err := repo.GetSLA(ctx, sla.ID)
		require.NoError(t, err)
		assert.Equal(t, 60, retrieved.FirstResponseTime)
		assert.Equal(t, "Updated description", retrieved.Description)
		assert.NotEqual(t, retrieved.CreatedAt, retrieved.UpdatedAt)
	})

	t.Run("FindApplicableSLA", func(t *testing.T) {
		repo := NewMemorySLARepository()

		// Create SLAs with different conditions
		goldSLA := &models.SLA{
			Name:     "Gold",
			Priority: 5,
			IsActive: true,
			Conditions: models.SLAConditions{
				Priorities: []int{4, 5},
				Queues:     []uint{1},
			},
		}
		repo.CreateSLA(ctx, goldSLA)

		silverSLA := &models.SLA{
			Name:     "Silver",
			Priority: 3,
			IsActive: true,
			Conditions: models.SLAConditions{
				Priorities: []int{2, 3},
				Queues:     []uint{1, 2},
			},
		}
		repo.CreateSLA(ctx, silverSLA)

		// Find SLA for high priority ticket in queue 1
		applicable, err := repo.FindApplicableSLA(ctx, 1, 4, "", nil)
		require.NoError(t, err)
		assert.Equal(t, "Gold", applicable.Name)

		// Find SLA for normal priority ticket in queue 2
		applicable, err = repo.FindApplicableSLA(ctx, 2, 2, "", nil)
		require.NoError(t, err)
		assert.Equal(t, "Silver", applicable.Name)

		// No applicable SLA for low priority
		applicable, err = repo.FindApplicableSLA(ctx, 1, 1, "", nil)
		assert.Error(t, err)
		assert.Nil(t, applicable)
	})

	t.Run("CreateTicketSLA", func(t *testing.T) {
		repo := NewMemorySLARepository()

		now := time.Now()
		ticketSLA := &models.TicketSLA{
			TicketID:         100,
			SLAID:            1,
			FirstResponseDue: &now,
			Status:           "pending",
		}

		err := repo.CreateTicketSLA(ctx, ticketSLA)
		require.NoError(t, err)
		assert.NotZero(t, ticketSLA.ID)
		assert.NotZero(t, ticketSLA.CreatedAt)
	})

	t.Run("UpdateTicketSLA", func(t *testing.T) {
		repo := NewMemorySLARepository()

		ticketSLA := &models.TicketSLA{
			TicketID: 200,
			SLAID:    1,
			Status:   "pending",
		}
		repo.CreateTicketSLA(ctx, ticketSLA)

		// Update with response time
		now := time.Now()
		ticketSLA.FirstResponseAt = &now
		ticketSLA.Status = "in_progress"

		err := repo.UpdateTicketSLA(ctx, ticketSLA)
		require.NoError(t, err)

		retrieved, err := repo.GetTicketSLA(ctx, 200)
		require.NoError(t, err)
		assert.NotNil(t, retrieved.FirstResponseAt)
		assert.Equal(t, "in_progress", retrieved.Status)
	})

	t.Run("GetSLAMetrics", func(t *testing.T) {
		repo := NewMemorySLARepository()

		// Create some ticket SLAs
		now := time.Now()
		ticketSLAs := []models.TicketSLA{
			{TicketID: 1, SLAID: 1, Status: "met", SolutionAt: &now},
			{TicketID: 2, SLAID: 1, Status: "met", SolutionAt: &now},
			{TicketID: 3, SLAID: 1, Status: "breached", BreachTime: &now},
			{TicketID: 4, SLAID: 1, Status: "pending"},
			{TicketID: 5, SLAID: 2, Status: "met", SolutionAt: &now},
		}

		for i := range ticketSLAs {
			repo.CreateTicketSLA(ctx, &ticketSLAs[i])
		}

		// Get metrics for SLA 1
		metrics, err := repo.GetSLAMetrics(ctx, 1, now.Add(-24*time.Hour), now.Add(time.Hour))
		require.NoError(t, err)
		assert.Equal(t, uint(1), metrics.SLAID)
		assert.Equal(t, 4, metrics.TotalTickets)
		assert.Equal(t, 2, metrics.MetCount)
		assert.Equal(t, 1, metrics.BreachedCount)
		assert.Equal(t, 1, metrics.PendingCount)
		assert.Equal(t, 50.0, metrics.CompliancePercent) // 2 met out of 4
	})

	t.Run("CreateBusinessCalendar", func(t *testing.T) {
		repo := NewMemorySLARepository()

		calendar := &models.BusinessCalendar{
			Name:        "Standard Business Hours",
			Description: "Mon-Fri 9-5",
			TimeZone:    "America/New_York",
			IsDefault:   true,
			WorkingHours: []models.WorkingHours{
				{DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00", IsWorkingDay: true},
				{DayOfWeek: 2, StartTime: "09:00", EndTime: "17:00", IsWorkingDay: true},
				{DayOfWeek: 3, StartTime: "09:00", EndTime: "17:00", IsWorkingDay: true},
				{DayOfWeek: 4, StartTime: "09:00", EndTime: "17:00", IsWorkingDay: true},
				{DayOfWeek: 5, StartTime: "09:00", EndTime: "17:00", IsWorkingDay: true},
				{DayOfWeek: 0, IsWorkingDay: false}, // Sunday
				{DayOfWeek: 6, IsWorkingDay: false}, // Saturday
			},
		}

		err := repo.CreateBusinessCalendar(ctx, calendar)
		require.NoError(t, err)
		assert.NotZero(t, calendar.ID)
		assert.Len(t, calendar.WorkingHours, 7)
	})

	t.Run("AddHoliday", func(t *testing.T) {
		repo := NewMemorySLARepository()

		// Create calendar first
		calendar := &models.BusinessCalendar{
			Name:     "Test Calendar",
			TimeZone: "UTC",
		}
		repo.CreateBusinessCalendar(ctx, calendar)

		// Add holiday
		holiday := &models.SLAHoliday{
			CalendarID:  calendar.ID,
			Name:        "New Year",
			Date:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			IsRecurring: true,
		}

		err := repo.AddHoliday(ctx, holiday)
		require.NoError(t, err)
		assert.NotZero(t, holiday.ID)

		// Get holidays
		holidays, err := repo.GetHolidays(ctx, calendar.ID)
		require.NoError(t, err)
		assert.Len(t, holidays, 1)
		assert.Equal(t, "New Year", holidays[0].Name)
	})

	t.Run("RecordEscalation", func(t *testing.T) {
		repo := NewMemorySLARepository()

		// Create ticket SLA first
		ticketSLA := &models.TicketSLA{
			TicketID: 300,
			SLAID:    1,
			Status:   "in_progress",
		}
		repo.CreateTicketSLA(ctx, ticketSLA)

		// Record escalation
		escalation := &models.SLAEscalationHistory{
			TicketSLAID:      ticketSLA.ID,
			EscalationRuleID: 1,
			EscalatedAt:      time.Now(),
			NotifiedUsers:    []uint{1, 2, 3},
			Actions:          `{"priority_changed": true, "assigned_to": 5}`,
			Success:          true,
		}

		err := repo.RecordEscalation(ctx, escalation)
		require.NoError(t, err)
		assert.NotZero(t, escalation.ID)

		// Get escalation history
		history, err := repo.GetEscalationHistory(ctx, ticketSLA.ID)
		require.NoError(t, err)
		assert.Len(t, history, 1)
		assert.Equal(t, uint(1), history[0].EscalationRuleID)
	})

	t.Run("PauseAndResumeSLA", func(t *testing.T) {
		repo := NewMemorySLARepository()

		// Create ticket SLA
		ticketSLA := &models.TicketSLA{
			TicketID: 400,
			SLAID:    1,
			Status:   "in_progress",
		}
		repo.CreateTicketSLA(ctx, ticketSLA)

		// Pause SLA
		pauseReason := &models.SLAPauseReason{
			TicketSLAID: ticketSLA.ID,
			Reason:      "Waiting for customer response",
			PausedBy:    1,
			PausedAt:    time.Now(),
		}

		err := repo.PauseSLA(ctx, pauseReason)
		require.NoError(t, err)

		// Check that SLA is paused
		retrieved, err := repo.GetTicketSLA(ctx, 400)
		require.NoError(t, err)
		assert.NotNil(t, retrieved.PausedAt)

		// Add a small delay to ensure some time passes
		time.Sleep(10 * time.Millisecond)

		// Resume SLA
		err = repo.ResumeSLA(ctx, ticketSLA.ID)
		require.NoError(t, err)

		// Check that SLA is resumed
		retrieved, err = repo.GetTicketSLA(ctx, 400)
		require.NoError(t, err)
		assert.Nil(t, retrieved.PausedAt)
		assert.GreaterOrEqual(t, retrieved.TotalPausedMinutes, 0)
	})

	t.Run("GetSLAReport", func(t *testing.T) {
		repo := NewMemorySLARepository()

		// Setup test data - use time range that includes current time
		now := time.Now()
		startTime := now.Add(-1 * time.Hour)
		endTime := now.Add(1 * time.Hour)

		for i := 1; i <= 5; i++ {
			ticketSLA := &models.TicketSLA{
				TicketID: uint(500 + i),
				SLAID:    1,
				Status:   "met",
			}
			if i == 5 {
				ticketSLA.Status = "breached"
			}
			repo.CreateTicketSLA(ctx, ticketSLA)
		}

		// Get report
		report, err := repo.GetSLAReport(ctx, startTime, endTime)
		require.NoError(t, err)
		assert.Equal(t, 5, report.TotalTickets)
		assert.Equal(t, 1, report.TotalBreaches)
		assert.Equal(t, 80.0, report.OverallCompliance) // 4 out of 5 met
	})
}
