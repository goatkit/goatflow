// Package models provides domain models and data structures.
package models

import (
	"time"
)

// DayOfWeek represents a day of the week.
type DayOfWeek int

const (
	Sunday DayOfWeek = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

// BusinessHoursConfig represents business hours configuration.
type BusinessHoursConfig struct {
	ID          int                 `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Timezone    string              `json:"timezone"` // e.g., "America/New_York"
	IsDefault   bool                `json:"is_default"`
	IsActive    bool                `json:"is_active"`
	WorkingDays []WorkingDay        `json:"working_days"`
	Holidays    []Holiday           `json:"holidays"`
	Exceptions  []BusinessException `json:"exceptions"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// WorkingDay defines working hours for a specific day.
type WorkingDay struct {
	Day       DayOfWeek   `json:"day"`
	IsWorking bool        `json:"is_working"`
	Shifts    []TimeShift `json:"shifts"` // Multiple shifts per day support
}

// TimeShift represents a working shift.
type TimeShift struct {
	StartTime  string `json:"start_time"`            // Format: "09:00"
	EndTime    string `json:"end_time"`              // Format: "17:00"
	BreakStart string `json:"break_start,omitempty"` // Optional break
	BreakEnd   string `json:"break_end,omitempty"`
}

// Holiday represents a non-working holiday.
type Holiday struct {
	ID           int       `json:"id"`
	ConfigID     int       `json:"config_id"`
	Name         string    `json:"name"`
	Date         time.Time `json:"date"`
	IsRecurring  bool      `json:"is_recurring"`            // Repeats every year
	IsFloating   bool      `json:"is_floating"`             // e.g., "Last Monday of May"
	FloatingRule string    `json:"floating_rule,omitempty"` // Rule for floating holidays
}

// BusinessException represents an exception to normal business hours.
type BusinessException struct {
	ID        int       `json:"id"`
	ConfigID  int       `json:"config_id"`
	Name      string    `json:"name"`
	Date      time.Time `json:"date"`
	IsWorking bool      `json:"is_working"` // Override to working/non-working
	StartTime string    `json:"start_time,omitempty"`
	EndTime   string    `json:"end_time,omitempty"`
	Reason    string    `json:"reason"`
}

// BusinessHoursCalculator provides business hours calculations.
type BusinessHoursCalculator struct {
	Config          *BusinessHoursConfig
	locationCache   *time.Location
	holidayCache    map[string]bool
	workingDayCache map[DayOfWeek]*WorkingDay
}

// NewBusinessHoursCalculator creates a new business hours calculator.
func NewBusinessHoursCalculator(config *BusinessHoursConfig) (*BusinessHoursCalculator, error) {
	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		return nil, err
	}

	bc := &BusinessHoursCalculator{
		Config:          config,
		locationCache:   loc,
		holidayCache:    make(map[string]bool),
		workingDayCache: make(map[DayOfWeek]*WorkingDay),
	}

	// Cache holidays
	for _, holiday := range config.Holidays {
		dateKey := holiday.Date.Format("2006-01-02")
		bc.holidayCache[dateKey] = true
	}

	// Cache working days
	for i := range config.WorkingDays {
		wd := &config.WorkingDays[i]
		bc.workingDayCache[wd.Day] = wd
	}

	return bc, nil
}

// IsBusinessDay checks if a given date is a business day.
func (bc *BusinessHoursCalculator) IsBusinessDay(date time.Time) bool {
	// Convert to calendar timezone
	localDate := date.In(bc.locationCache)

	// Check if it's a holiday
	dateKey := localDate.Format("2006-01-02")
	if bc.holidayCache[dateKey] {
		return false
	}

	// Check if there's an exception for this date
	for _, exception := range bc.Config.Exceptions {
		if exception.Date.Format("2006-01-02") == dateKey {
			return exception.IsWorking
		}
	}

	// Check regular working days
	dow := DayOfWeek(localDate.Weekday())
	if wd, exists := bc.workingDayCache[dow]; exists {
		return wd.IsWorking
	}

	return false
}

// IsWithinBusinessHours checks if a given time is within business hours.
func (bc *BusinessHoursCalculator) IsWithinBusinessHours(t time.Time) bool {
	if !bc.IsBusinessDay(t) {
		return false
	}

	localTime := t.In(bc.locationCache)
	dow := DayOfWeek(localTime.Weekday())

	// Check exceptions first
	dateKey := localTime.Format("2006-01-02")
	for _, exception := range bc.Config.Exceptions {
		if exception.Date.Format("2006-01-02") == dateKey && exception.IsWorking {
			if exception.StartTime != "" && exception.EndTime != "" {
				return bc.isTimeInRange(localTime, exception.StartTime, exception.EndTime)
			}
		}
	}

	// Check regular working hours
	if wd, exists := bc.workingDayCache[dow]; exists && wd.IsWorking {
		for _, shift := range wd.Shifts {
			if bc.isTimeInShift(localTime, shift) {
				return true
			}
		}
	}

	return false
}

// GetNextBusinessDay returns the next business day from the given date.
func (bc *BusinessHoursCalculator) GetNextBusinessDay(from time.Time) time.Time {
	next := from.In(bc.locationCache).AddDate(0, 0, 1)
	next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, bc.locationCache)

	for !bc.IsBusinessDay(next) {
		next = next.AddDate(0, 0, 1)
	}

	return next
}

// GetNextBusinessHour returns the next business hour from the given time.
func (bc *BusinessHoursCalculator) GetNextBusinessHour(from time.Time) time.Time {
	localTime := from.In(bc.locationCache)

	// If already in business hours, return the same time
	if bc.IsWithinBusinessHours(localTime) {
		return from
	}

	// Check if later today has business hours
	dow := DayOfWeek(localTime.Weekday())
	if wd, exists := bc.workingDayCache[dow]; exists && wd.IsWorking {
		for _, shift := range wd.Shifts {
			startTime := bc.parseTimeOnDate(localTime, shift.StartTime)
			if startTime.After(localTime) {
				return startTime
			}
		}
	}

	// Move to next business day
	nextDay := bc.GetNextBusinessDay(localTime)
	dow = DayOfWeek(nextDay.Weekday())
	if wd, exists := bc.workingDayCache[dow]; exists && len(wd.Shifts) > 0 {
		return bc.parseTimeOnDate(nextDay, wd.Shifts[0].StartTime)
	}

	return nextDay
}

// AddBusinessHours adds business hours to a given time.
func (bc *BusinessHoursCalculator) AddBusinessHours(from time.Time, hours float64) time.Time {
	if hours == 0 {
		return from
	}

	minutes := int(hours * 60)
	current := bc.GetNextBusinessHour(from)
	remainingMinutes := minutes

	for remainingMinutes > 0 {
		if !bc.IsWithinBusinessHours(current) {
			current = bc.GetNextBusinessHour(current)
		}

		// Calculate minutes until end of current business period
		endOfPeriod := bc.getEndOfBusinessPeriod(current)
		availableMinutes := int(endOfPeriod.Sub(current).Minutes())

		if availableMinutes >= remainingMinutes {
			return current.Add(time.Duration(remainingMinutes) * time.Minute)
		}

		remainingMinutes -= availableMinutes
		current = bc.GetNextBusinessHour(endOfPeriod.Add(time.Minute))
	}

	return current
}

// GetBusinessHoursBetween calculates business hours between two times.
func (bc *BusinessHoursCalculator) GetBusinessHoursBetween(start, end time.Time) float64 {
	if end.Before(start) {
		return 0
	}

	totalMinutes := 0
	current := start.In(bc.locationCache)
	endLocal := end.In(bc.locationCache)

	for current.Before(endLocal) {
		if bc.IsWithinBusinessHours(current) {
			// Find the end of this business period
			periodEnd := bc.getEndOfBusinessPeriod(current)
			if periodEnd.After(endLocal) {
				periodEnd = endLocal
			}

			minutes := int(periodEnd.Sub(current).Minutes())
			totalMinutes += minutes
			current = periodEnd
		} else {
			// Skip to next business hour
			current = bc.GetNextBusinessHour(current)
			if current.After(endLocal) {
				break
			}
		}
	}

	return float64(totalMinutes) / 60.0
}

// Helper methods

// isTimeInRange checks if a time falls within a time range.
func (bc *BusinessHoursCalculator) isTimeInRange(t time.Time, startStr, endStr string) bool {
	start := bc.parseTimeOnDate(t, startStr)
	end := bc.parseTimeOnDate(t, endStr)

	// Handle overnight shifts
	if end.Before(start) {
		end = end.AddDate(0, 0, 1)
	}

	return (t.Equal(start) || t.After(start)) && t.Before(end)
}

// isTimeInShift checks if a time falls within a shift.
func (bc *BusinessHoursCalculator) isTimeInShift(t time.Time, shift TimeShift) bool {
	inWorkTime := bc.isTimeInRange(t, shift.StartTime, shift.EndTime)
	if !inWorkTime {
		return false
	}

	// Check if in break time
	if shift.BreakStart != "" && shift.BreakEnd != "" {
		inBreak := bc.isTimeInRange(t, shift.BreakStart, shift.BreakEnd)
		return !inBreak
	}

	return true
}

// parseTimeOnDate parses a time string on a specific date.
func (bc *BusinessHoursCalculator) parseTimeOnDate(date time.Time, timeStr string) time.Time {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return date
	}

	return time.Date(
		date.Year(), date.Month(), date.Day(),
		t.Hour(), t.Minute(), 0, 0,
		bc.locationCache,
	)
}

// getEndOfBusinessPeriod gets the end of the current business period.
func (bc *BusinessHoursCalculator) getEndOfBusinessPeriod(t time.Time) time.Time {
	localTime := t.In(bc.locationCache)
	dow := DayOfWeek(localTime.Weekday())

	// Check exceptions first
	dateKey := localTime.Format("2006-01-02")
	for _, exception := range bc.Config.Exceptions {
		if exception.Date.Format("2006-01-02") == dateKey && exception.IsWorking {
			if exception.EndTime != "" {
				return bc.parseTimeOnDate(localTime, exception.EndTime)
			}
		}
	}

	// Check regular working hours
	if wd, exists := bc.workingDayCache[dow]; exists && wd.IsWorking {
		for _, shift := range wd.Shifts {
			if bc.isTimeInShift(localTime, shift) {
				// Check if we're before a break
				if shift.BreakStart != "" {
					breakStart := bc.parseTimeOnDate(localTime, shift.BreakStart)
					if localTime.Before(breakStart) {
						return breakStart
					}
				}
				return bc.parseTimeOnDate(localTime, shift.EndTime)
			}
		}
	}

	return localTime
}

// GetDefaultBusinessHours returns a default business hours configuration.
func GetDefaultBusinessHours() *BusinessHoursConfig {
	return &BusinessHoursConfig{
		Name:        "Default Business Hours",
		Description: "Standard Monday-Friday 9-5",
		Timezone:    "America/New_York",
		IsDefault:   true,
		IsActive:    true,
		WorkingDays: []WorkingDay{
			{Day: Monday, IsWorking: true, Shifts: []TimeShift{{StartTime: "09:00", EndTime: "17:00"}}},
			{Day: Tuesday, IsWorking: true, Shifts: []TimeShift{{StartTime: "09:00", EndTime: "17:00"}}},
			{Day: Wednesday, IsWorking: true, Shifts: []TimeShift{{StartTime: "09:00", EndTime: "17:00"}}},
			{Day: Thursday, IsWorking: true, Shifts: []TimeShift{{StartTime: "09:00", EndTime: "17:00"}}},
			{Day: Friday, IsWorking: true, Shifts: []TimeShift{{StartTime: "09:00", EndTime: "17:00"}}},
			{Day: Saturday, IsWorking: false, Shifts: []TimeShift{}},
			{Day: Sunday, IsWorking: false, Shifts: []TimeShift{}},
		},
		Holidays:   []Holiday{},
		Exceptions: []BusinessException{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// Get24x7BusinessHours returns a 24x7 business hours configuration.
func Get24x7BusinessHours() *BusinessHoursConfig {
	shifts := []TimeShift{{StartTime: "00:00", EndTime: "23:59"}}
	return &BusinessHoursConfig{
		Name:        "24x7 Support",
		Description: "Round-the-clock support",
		Timezone:    "UTC",
		IsDefault:   false,
		IsActive:    true,
		WorkingDays: []WorkingDay{
			{Day: Monday, IsWorking: true, Shifts: shifts},
			{Day: Tuesday, IsWorking: true, Shifts: shifts},
			{Day: Wednesday, IsWorking: true, Shifts: shifts},
			{Day: Thursday, IsWorking: true, Shifts: shifts},
			{Day: Friday, IsWorking: true, Shifts: shifts},
			{Day: Saturday, IsWorking: true, Shifts: shifts},
			{Day: Sunday, IsWorking: true, Shifts: shifts},
		},
		Holidays:   []Holiday{},
		Exceptions: []BusinessException{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
