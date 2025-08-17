package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// SLARepository defines the interface for SLA operations
type SLARepository interface {
	// SLA management
	CreateSLA(ctx context.Context, sla *models.SLA) error
	GetSLA(ctx context.Context, id uint) (*models.SLA, error)
	GetAllSLAs(ctx context.Context, activeOnly bool) ([]models.SLA, error)
	UpdateSLA(ctx context.Context, sla *models.SLA) error
	DeleteSLA(ctx context.Context, id uint) error
	FindApplicableSLA(ctx context.Context, queueID uint, priority int, ticketType string, tags []string) (*models.SLA, error)
	
	// Ticket SLA tracking
	CreateTicketSLA(ctx context.Context, ticketSLA *models.TicketSLA) error
	GetTicketSLA(ctx context.Context, ticketID uint) (*models.TicketSLA, error)
	UpdateTicketSLA(ctx context.Context, ticketSLA *models.TicketSLA) error
	GetSLAMetrics(ctx context.Context, slaID uint, from, to time.Time) (*models.SLAMetrics, error)
	
	// Business calendar
	CreateBusinessCalendar(ctx context.Context, calendar *models.BusinessCalendar) error
	GetBusinessCalendar(ctx context.Context, id uint) (*models.BusinessCalendar, error)
	UpdateBusinessCalendar(ctx context.Context, calendar *models.BusinessCalendar) error
	AddHoliday(ctx context.Context, holiday *models.Holiday) error
	GetHolidays(ctx context.Context, calendarID uint) ([]models.Holiday, error)
	
	// Escalation tracking
	RecordEscalation(ctx context.Context, escalation *models.SLAEscalationHistory) error
	GetEscalationHistory(ctx context.Context, ticketSLAID uint) ([]models.SLAEscalationHistory, error)
	
	// SLA pause/resume
	PauseSLA(ctx context.Context, pauseReason *models.SLAPauseReason) error
	ResumeSLA(ctx context.Context, ticketSLAID uint) error
	
	// Reporting
	GetSLAReport(ctx context.Context, from, to time.Time) (*models.SLAReport, error)
}

// MemorySLARepository is an in-memory implementation of SLARepository
type MemorySLARepository struct {
	slas           map[uint]*models.SLA
	ticketSLAs     map[uint]*models.TicketSLA // key is ticketID
	calendars      map[uint]*models.BusinessCalendar
	holidays       map[uint]*models.Holiday
	escalations    []models.SLAEscalationHistory
	pauseReasons   []models.SLAPauseReason
	mu             sync.RWMutex
	nextSLAID      uint
	nextTicketSLAID uint
	nextCalendarID uint
	nextHolidayID  uint
	nextEscalationID uint
	nextPauseID    uint
}

// NewMemorySLARepository creates a new in-memory SLA repository
func NewMemorySLARepository() *MemorySLARepository {
	return &MemorySLARepository{
		slas:           make(map[uint]*models.SLA),
		ticketSLAs:     make(map[uint]*models.TicketSLA),
		calendars:      make(map[uint]*models.BusinessCalendar),
		holidays:       make(map[uint]*models.Holiday),
		escalations:    []models.SLAEscalationHistory{},
		pauseReasons:   []models.SLAPauseReason{},
		nextSLAID:      1,
		nextTicketSLAID: 1,
		nextCalendarID: 1,
		nextHolidayID:  1,
		nextEscalationID: 1,
		nextPauseID:    1,
	}
}

// CreateSLA creates a new SLA
func (r *MemorySLARepository) CreateSLA(ctx context.Context, sla *models.SLA) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	sla.ID = r.nextSLAID
	r.nextSLAID++
	sla.CreatedAt = time.Now()
	sla.UpdatedAt = sla.CreatedAt
	
	// Create a copy to store
	stored := *sla
	r.slas[sla.ID] = &stored
	
	return nil
}

// GetSLA retrieves an SLA by ID
func (r *MemorySLARepository) GetSLA(ctx context.Context, id uint) (*models.SLA, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	sla, exists := r.slas[id]
	if !exists {
		return nil, fmt.Errorf("SLA %d not found", id)
	}
	
	// Return a copy
	result := *sla
	return &result, nil
}

// GetAllSLAs retrieves all SLAs
func (r *MemorySLARepository) GetAllSLAs(ctx context.Context, activeOnly bool) ([]models.SLA, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var slas []models.SLA
	for _, sla := range r.slas {
		if !activeOnly || sla.IsActive {
			slas = append(slas, *sla)
		}
	}
	
	return slas, nil
}

// UpdateSLA updates an existing SLA
func (r *MemorySLARepository) UpdateSLA(ctx context.Context, sla *models.SLA) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.slas[sla.ID]; !exists {
		return fmt.Errorf("SLA %d not found", sla.ID)
	}
	
	sla.UpdatedAt = time.Now()
	stored := *sla
	r.slas[sla.ID] = &stored
	
	return nil
}

// DeleteSLA deletes an SLA
func (r *MemorySLARepository) DeleteSLA(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.slas[id]; !exists {
		return fmt.Errorf("SLA %d not found", id)
	}
	
	delete(r.slas, id)
	return nil
}

// FindApplicableSLA finds the best matching SLA for given conditions
func (r *MemorySLARepository) FindApplicableSLA(ctx context.Context, queueID uint, priority int, ticketType string, tags []string) (*models.SLA, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var bestSLA *models.SLA
	highestPriority := 0
	
	for _, sla := range r.slas {
		if !sla.IsActive {
			continue
		}
		
		// Check if conditions match
		if !r.matchesConditions(sla, queueID, priority, ticketType, tags) {
			continue
		}
		
		// Select SLA with highest priority
		if sla.Priority > highestPriority {
			slaCopy := *sla
			bestSLA = &slaCopy
			highestPriority = sla.Priority
		}
	}
	
	if bestSLA == nil {
		return nil, fmt.Errorf("no applicable SLA found")
	}
	
	return bestSLA, nil
}

// matchesConditions checks if ticket matches SLA conditions
func (r *MemorySLARepository) matchesConditions(sla *models.SLA, queueID uint, priority int, ticketType string, tags []string) bool {
	conditions := sla.Conditions
	
	// Check queue
	if len(conditions.Queues) > 0 {
		found := false
		for _, q := range conditions.Queues {
			if q == queueID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check priority
	if len(conditions.Priorities) > 0 {
		found := false
		for _, p := range conditions.Priorities {
			if p == priority {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check type
	if len(conditions.Types) > 0 && ticketType != "" {
		found := false
		for _, t := range conditions.Types {
			if t == ticketType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check tags
	if len(conditions.Tags) > 0 && len(tags) > 0 {
		for _, requiredTag := range conditions.Tags {
			found := false
			for _, tag := range tags {
				if tag == requiredTag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	
	return true
}

// CreateTicketSLA creates a new ticket SLA tracking record
func (r *MemorySLARepository) CreateTicketSLA(ctx context.Context, ticketSLA *models.TicketSLA) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	ticketSLA.ID = r.nextTicketSLAID
	r.nextTicketSLAID++
	ticketSLA.CreatedAt = time.Now()
	ticketSLA.UpdatedAt = ticketSLA.CreatedAt
	
	// Store by ticket ID for easy lookup
	stored := *ticketSLA
	r.ticketSLAs[ticketSLA.TicketID] = &stored
	
	return nil
}

// GetTicketSLA retrieves ticket SLA by ticket ID
func (r *MemorySLARepository) GetTicketSLA(ctx context.Context, ticketID uint) (*models.TicketSLA, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	ticketSLA, exists := r.ticketSLAs[ticketID]
	if !exists {
		return nil, fmt.Errorf("ticket SLA for ticket %d not found", ticketID)
	}
	
	// Return a copy
	result := *ticketSLA
	return &result, nil
}

// UpdateTicketSLA updates a ticket SLA record
func (r *MemorySLARepository) UpdateTicketSLA(ctx context.Context, ticketSLA *models.TicketSLA) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.ticketSLAs[ticketSLA.TicketID]; !exists {
		return fmt.Errorf("ticket SLA for ticket %d not found", ticketSLA.TicketID)
	}
	
	ticketSLA.UpdatedAt = time.Now()
	stored := *ticketSLA
	r.ticketSLAs[ticketSLA.TicketID] = &stored
	
	return nil
}

// GetSLAMetrics calculates SLA metrics for a specific SLA
func (r *MemorySLARepository) GetSLAMetrics(ctx context.Context, slaID uint, from, to time.Time) (*models.SLAMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	metrics := &models.SLAMetrics{
		SLAID:  slaID,
		Period: fmt.Sprintf("%s to %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
	}
	
	// Get SLA name
	if sla, exists := r.slas[slaID]; exists {
		metrics.SLAName = sla.Name
	}
	
	// Calculate metrics from ticket SLAs
	for _, ticketSLA := range r.ticketSLAs {
		if ticketSLA.SLAID != slaID {
			continue
		}
		
		// Only count tickets within the time range
		if ticketSLA.CreatedAt.Before(from) || ticketSLA.CreatedAt.After(to) {
			continue
		}
		
		metrics.TotalTickets++
		
		switch ticketSLA.Status {
		case "met":
			metrics.MetCount++
		case "breached":
			metrics.BreachedCount++
		case "pending", "in_progress":
			metrics.PendingCount++
		}
		
		// Count escalations
		if ticketSLA.EscalationLevel > 0 {
			metrics.TotalEscalations++
		}
	}
	
	// Calculate compliance percentage
	if metrics.TotalTickets > 0 {
		metrics.CompliancePercent = float64(metrics.MetCount) / float64(metrics.TotalTickets) * 100
	}
	
	return metrics, nil
}

// CreateBusinessCalendar creates a new business calendar
func (r *MemorySLARepository) CreateBusinessCalendar(ctx context.Context, calendar *models.BusinessCalendar) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	calendar.ID = r.nextCalendarID
	r.nextCalendarID++
	calendar.CreatedAt = time.Now()
	calendar.UpdatedAt = calendar.CreatedAt
	
	// Create a copy to store
	stored := *calendar
	r.calendars[calendar.ID] = &stored
	
	return nil
}

// GetBusinessCalendar retrieves a business calendar
func (r *MemorySLARepository) GetBusinessCalendar(ctx context.Context, id uint) (*models.BusinessCalendar, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	calendar, exists := r.calendars[id]
	if !exists {
		return nil, fmt.Errorf("calendar %d not found", id)
	}
	
	// Return a copy
	result := *calendar
	return &result, nil
}

// UpdateBusinessCalendar updates a business calendar
func (r *MemorySLARepository) UpdateBusinessCalendar(ctx context.Context, calendar *models.BusinessCalendar) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.calendars[calendar.ID]; !exists {
		return fmt.Errorf("calendar %d not found", calendar.ID)
	}
	
	calendar.UpdatedAt = time.Now()
	stored := *calendar
	r.calendars[calendar.ID] = &stored
	
	return nil
}

// AddHoliday adds a holiday to a calendar
func (r *MemorySLARepository) AddHoliday(ctx context.Context, holiday *models.Holiday) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	holiday.ID = r.nextHolidayID
	r.nextHolidayID++
	
	// Create a copy to store
	stored := *holiday
	r.holidays[holiday.ID] = &stored
	
	return nil
}

// GetHolidays gets holidays for a calendar
func (r *MemorySLARepository) GetHolidays(ctx context.Context, calendarID uint) ([]models.Holiday, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var holidays []models.Holiday
	for _, holiday := range r.holidays {
		if holiday.CalendarID == calendarID {
			holidays = append(holidays, *holiday)
		}
	}
	
	return holidays, nil
}

// RecordEscalation records an escalation event
func (r *MemorySLARepository) RecordEscalation(ctx context.Context, escalation *models.SLAEscalationHistory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	escalation.ID = r.nextEscalationID
	r.nextEscalationID++
	
	r.escalations = append(r.escalations, *escalation)
	
	// Update ticket SLA escalation level
	for _, ticketSLA := range r.ticketSLAs {
		if ticketSLA.ID == escalation.TicketSLAID {
			ticketSLA.EscalationLevel++
			now := time.Now()
			ticketSLA.LastEscalationAt = &now
			break
		}
	}
	
	return nil
}

// GetEscalationHistory gets escalation history for a ticket SLA
func (r *MemorySLARepository) GetEscalationHistory(ctx context.Context, ticketSLAID uint) ([]models.SLAEscalationHistory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var history []models.SLAEscalationHistory
	for _, escalation := range r.escalations {
		if escalation.TicketSLAID == ticketSLAID {
			history = append(history, escalation)
		}
	}
	
	return history, nil
}

// PauseSLA pauses SLA tracking
func (r *MemorySLARepository) PauseSLA(ctx context.Context, pauseReason *models.SLAPauseReason) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	pauseReason.ID = r.nextPauseID
	r.nextPauseID++
	
	r.pauseReasons = append(r.pauseReasons, *pauseReason)
	
	// Update ticket SLA
	for _, ticketSLA := range r.ticketSLAs {
		if ticketSLA.ID == pauseReason.TicketSLAID {
			now := time.Now()
			ticketSLA.PausedAt = &now
			break
		}
	}
	
	return nil
}

// ResumeSLA resumes SLA tracking
func (r *MemorySLARepository) ResumeSLA(ctx context.Context, ticketSLAID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Find ticket SLA
	var ticketSLA *models.TicketSLA
	for _, ts := range r.ticketSLAs {
		if ts.ID == ticketSLAID {
			ticketSLA = ts
			break
		}
	}
	
	if ticketSLA == nil {
		return fmt.Errorf("ticket SLA %d not found", ticketSLAID)
	}
	
	if ticketSLA.PausedAt == nil {
		return fmt.Errorf("ticket SLA %d is not paused", ticketSLAID)
	}
	
	// Calculate pause duration
	pauseDuration := time.Since(*ticketSLA.PausedAt)
	ticketSLA.TotalPausedMinutes += int(pauseDuration.Minutes())
	ticketSLA.PausedAt = nil
	
	// Update pause reason
	now := time.Now()
	for i := range r.pauseReasons {
		if r.pauseReasons[i].TicketSLAID == ticketSLAID && r.pauseReasons[i].ResumedAt == nil {
			r.pauseReasons[i].ResumedAt = &now
			r.pauseReasons[i].Duration = int(pauseDuration.Minutes())
			break
		}
	}
	
	return nil
}

// GetSLAReport generates an SLA report
func (r *MemorySLARepository) GetSLAReport(ctx context.Context, from, to time.Time) (*models.SLAReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	report := &models.SLAReport{
		StartDate: from,
		EndDate:   to,
		Metrics:   []models.SLAMetrics{},
	}
	
	// Collect metrics for each SLA
	slaMetrics := make(map[uint]*models.SLAMetrics)
	
	for _, ticketSLA := range r.ticketSLAs {
		if ticketSLA.CreatedAt.Before(from) || ticketSLA.CreatedAt.After(to) {
			continue
		}
		
		report.TotalTickets++
		
		if ticketSLA.Status == "breached" {
			report.TotalBreaches++
		}
		
		// Aggregate by SLA
		if _, exists := slaMetrics[ticketSLA.SLAID]; !exists {
			slaMetrics[ticketSLA.SLAID] = &models.SLAMetrics{
				SLAID: ticketSLA.SLAID,
			}
		}
		
		metrics := slaMetrics[ticketSLA.SLAID]
		metrics.TotalTickets++
		
		switch ticketSLA.Status {
		case "met":
			metrics.MetCount++
		case "breached":
			metrics.BreachedCount++
		case "pending", "in_progress":
			metrics.PendingCount++
		}
	}
	
	// Calculate overall compliance
	if report.TotalTickets > 0 {
		report.OverallCompliance = float64(report.TotalTickets-report.TotalBreaches) / float64(report.TotalTickets) * 100
	}
	
	// Convert map to slice
	for _, metrics := range slaMetrics {
		if metrics.TotalTickets > 0 {
			metrics.CompliancePercent = float64(metrics.MetCount) / float64(metrics.TotalTickets) * 100
		}
		report.Metrics = append(report.Metrics, *metrics)
	}
	
	return report, nil
}