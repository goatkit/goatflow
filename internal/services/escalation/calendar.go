// Package escalation provides SLA escalation calculation and management.
package escalation

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
	"github.com/rickar/cal/v2"
	"gopkg.in/yaml.v3"
)

// CalendarService manages business calendars, wrapping rickar/cal with OTRS sysconfig.
type CalendarService struct {
	db        *sql.DB
	calendars map[string]*cal.BusinessCalendar // "" = default, "1"-"9" = named calendars
}

// NewCalendarService creates a new calendar service.
func NewCalendarService(db *sql.DB) *CalendarService {
	return &CalendarService{
		db:        db,
		calendars: make(map[string]*cal.BusinessCalendar),
	}
}

// LoadCalendars loads all calendars from sysconfig.
func (s *CalendarService) LoadCalendars(ctx context.Context) error {
	// Load default calendar
	defaultCal, err := s.loadCalendar(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to load default calendar: %w", err)
	}
	s.calendars[""] = defaultCal

	// Load calendars 1-9
	for i := 1; i <= 9; i++ {
		suffix := strconv.Itoa(i)
		c, err := s.loadCalendar(ctx, suffix)
		if err != nil {
			// Use default if specific calendar not configured
			s.calendars[suffix] = defaultCal
		} else {
			s.calendars[suffix] = c
		}
	}

	return nil
}

// loadCalendar loads a single calendar configuration from sysconfig.
func (s *CalendarService) loadCalendar(ctx context.Context, suffix string) (*cal.BusinessCalendar, error) {
	c := cal.NewBusinessCalendar()

	// Build config key names
	workingHoursKey := "TimeWorkingHours"
	vacationDaysKey := "TimeVacationDays"
	vacationDaysOneTimeKey := "TimeVacationDaysOneTime"
	if suffix != "" {
		workingHoursKey += "::Calendar" + suffix
		vacationDaysKey += "::Calendar" + suffix
		vacationDaysOneTimeKey += "::Calendar" + suffix
	}

	// Load and apply working hours
	workingHoursYAML, err := s.getSysconfigValue(ctx, workingHoursKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", workingHoursKey, err)
	}
	if err := s.applyWorkingHours(workingHoursYAML, c); err != nil {
		return nil, fmt.Errorf("failed to apply %s: %w", workingHoursKey, err)
	}

	// Load vacation days (optional) - add as holidays
	vacationDaysYAML, err := s.getSysconfigValue(ctx, vacationDaysKey)
	if err == nil && vacationDaysYAML != "" {
		s.applyVacationDays(vacationDaysYAML, c)
	}

	// Load one-time vacation days (optional)
	vacationDaysOneTimeYAML, err := s.getSysconfigValue(ctx, vacationDaysOneTimeKey)
	if err == nil && vacationDaysOneTimeYAML != "" {
		s.applyVacationDaysOneTime(vacationDaysOneTimeYAML, c)
	}

	return c, nil
}

// getSysconfigValue retrieves a sysconfig value, checking modified first, then default.
func (s *CalendarService) getSysconfigValue(ctx context.Context, name string) (string, error) {
	// First check sysconfig_modified for overrides
	query := database.ConvertPlaceholders(`
		SELECT effective_value FROM sysconfig_modified
		WHERE name = ? AND is_valid = 1
		ORDER BY id DESC LIMIT 1
	`)
	var value string
	err := s.db.QueryRowContext(ctx, query, name).Scan(&value)
	if err == nil {
		return value, nil
	}

	// Fall back to sysconfig_default
	query = database.ConvertPlaceholders(`
		SELECT effective_value FROM sysconfig_default
		WHERE name = ?
	`)
	err = s.db.QueryRowContext(ctx, query, name).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// applyWorkingHours parses OTRS YAML working hours and configures the calendar.
// OTRS format: { Mon: [8,9,10,...], Tue: [...], ... }
func (s *CalendarService) applyWorkingHours(yamlStr string, c *cal.BusinessCalendar) error {
	var hours map[string][]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &hours); err != nil {
		return err
	}

	// Map day names to time.Weekday
	dayMap := map[string]time.Weekday{
		"Mon": time.Monday,
		"Tue": time.Tuesday,
		"Wed": time.Wednesday,
		"Thu": time.Thursday,
		"Fri": time.Friday,
		"Sat": time.Saturday,
		"Sun": time.Sunday,
	}

	// Determine work hours range and which days are workdays
	var minHour, maxHour int = 24, 0

	for dayName, hourList := range hours {
		weekday, ok := dayMap[dayName]
		if !ok {
			continue
		}

		// If no hours, it's not a workday
		if len(hourList) == 0 {
			c.SetWorkday(weekday, false)
			continue
		}

		c.SetWorkday(weekday, true)

		// Find min/max hours for work hours range
		for _, h := range hourList {
			hour := toInt(h)
			if hour < minHour {
				minHour = hour
			}
			if hour > maxHour {
				maxHour = hour
			}
		}
	}

	// Set work hours (rickar/cal uses contiguous range)
	// OTRS default is 8-20, so end time is maxHour + 1 (end of that hour)
	if minHour < 24 && maxHour >= 0 {
		startTime := time.Duration(minHour) * time.Hour
		endTime := time.Duration(maxHour+1) * time.Hour // End of the last working hour
		c.SetWorkHours(startTime, endTime)
	}

	return nil
}

// applyVacationDays adds recurring holidays from OTRS VacationDays config.
// OTRS format: { 1: { 1: "New Year" }, 12: { 25: "Christmas" } }
func (s *CalendarService) applyVacationDays(yamlStr string, c *cal.BusinessCalendar) {
	var days map[interface{}]map[interface{}]string
	if err := yaml.Unmarshal([]byte(yamlStr), &days); err != nil {
		return
	}

	for monthKey, dayMap := range days {
		month := time.Month(toInt(monthKey))
		if month < 1 || month > 12 {
			continue
		}
		for dayKey, name := range dayMap {
			day := toInt(dayKey)
			if day < 1 || day > 31 {
				continue
			}
			// Create a recurring holiday using rickar/cal
			holiday := &cal.Holiday{
				Name:  name,
				Type:  cal.ObservancePublic,
				Month: month,
				Day:   day,
				Func:  cal.CalcDayOfMonth,
			}
			c.AddHoliday(holiday)
		}
	}
}

// applyVacationDaysOneTime adds one-time holidays from OTRS config.
// OTRS format: { 2025: { 1: { 1: "Special Day" } } }
func (s *CalendarService) applyVacationDaysOneTime(yamlStr string, c *cal.BusinessCalendar) {
	var days map[interface{}]map[interface{}]map[interface{}]string
	if err := yaml.Unmarshal([]byte(yamlStr), &days); err != nil {
		return
	}

	for yearKey, monthMap := range days {
		year := toInt(yearKey)
		if year == 0 {
			continue
		}
		for monthKey, dayMap := range monthMap {
			month := time.Month(toInt(monthKey))
			if month < 1 || month > 12 {
				continue
			}
			for dayKey, name := range dayMap {
				day := toInt(dayKey)
				if day < 1 || day > 31 {
					continue
				}
				// Create a one-time holiday with year range
				holiday := &cal.Holiday{
					Name:      name,
					Type:      cal.ObservancePublic,
					Month:     month,
					Day:       day,
					Func:      cal.CalcDayOfMonth,
					StartYear: year,
					EndYear:   year,
				}
				c.AddHoliday(holiday)
			}
		}
	}
}

// GetCalendar returns a calendar by name.
// Name can be empty (default), "1"-"9", or "Calendar1"-"Calendar9".
func (s *CalendarService) GetCalendar(name string) *cal.BusinessCalendar {
	// Normalize calendar name - strip "Calendar" prefix if present
	name = strings.TrimPrefix(name, "Calendar")

	if c, ok := s.calendars[name]; ok {
		return c
	}
	// Return default calendar if not found
	return s.calendars[""]
}

// AddWorkingTime adds working time (in minutes) to a start time and returns the destination time.
// This wraps rickar/cal's AddWorkHours for OTRS compatibility (minutes instead of Duration).
func (s *CalendarService) AddWorkingTime(calendarName string, start time.Time, minutes int) time.Time {
	c := s.GetCalendar(calendarName)
	if c == nil {
		// No calendar - just add absolute time
		return start.Add(time.Duration(minutes) * time.Minute)
	}
	return c.AddWorkHours(start, time.Duration(minutes)*time.Minute)
}

// WorkingTimeBetween calculates working time in seconds between two times.
func (s *CalendarService) WorkingTimeBetween(calendarName string, start, end time.Time) int64 {
	c := s.GetCalendar(calendarName)
	if c == nil {
		return int64(end.Sub(start).Seconds())
	}
	hours := c.WorkHoursInRange(start, end)
	return int64(hours.Seconds())
}

// IsWorkingTime checks if a given time is within working hours.
func (s *CalendarService) IsWorkingTime(calendarName string, t time.Time) bool {
	c := s.GetCalendar(calendarName)
	if c == nil {
		return true // No calendar means always working
	}
	return c.IsWorkTime(t)
}

// toInt converts various types to int.
func toInt(v interface{}) int {
	return shared.ToInt(v, 0)
}
