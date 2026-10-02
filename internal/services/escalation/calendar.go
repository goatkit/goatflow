// Package escalation keeps the OTRS ticket escalation index (ticket.escalation_*
// columns) up to date and raises the OTRS escalation events.
package escalation

import (
	"context"
	"database/sql"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// maxCalendarSteps bounds the hour-by-hour walks below. A configured calendar
// has at least one working hour a week, so even a year-long SLA finishes in
// far fewer steps; the bound only stops a pathological configuration (every
// working day a one-time vacation day for years) from spinning forever, like
// OTRS's five-second loop protection.
const maxCalendarSteps = 2_000_000

// Calendar is one OTRS business calendar: the working hours of each weekday,
// the recurring and one-time vacation days, and the time zone in which they
// are read. It mirrors Kernel::System::DateTime's AsWorkingTime/ForWorkingTime
// arithmetic.
type Calendar struct {
	location *time.Location
	hours    [7]uint32 // indexed by time.Weekday; bit h set = clock hour h is working time
	vacation map[[2]int]bool
	oneTime  map[[3]int]bool
}

// ParseCalendar builds a calendar from the OTRS setting values: TimeWorkingHours
// ({Mon: ['8', '9', ...], ...}), TimeVacationDays ({'12': {'25': 'Christmas'}})
// and TimeVacationDaysOneTime ({'2026': {'1': {'2': 'Bridge day'}}}). Empty
// strings mean "not set". A weekday missing from the working hours is not a
// working day.
func ParseCalendar(loc *time.Location, workingHours, vacationDays, vacationDaysOneTime string) (*Calendar, error) {
	if loc == nil {
		loc = time.UTC
	}
	c := &Calendar{location: loc, vacation: map[[2]int]bool{}, oneTime: map[[3]int]bool{}}

	if strings.TrimSpace(workingHours) != "" {
		var days map[string][]interface{}
		if err := yaml.Unmarshal([]byte(workingHours), &days); err != nil {
			return nil, fmt.Errorf("working hours: %w", err)
		}
		for name, hours := range days {
			wd, ok := weekdays[name]
			if !ok {
				return nil, fmt.Errorf("working hours: unknown day %q", name)
			}
			for _, h := range hours {
				hour, err := settingInt(h)
				if err != nil || hour < 0 || hour > 23 {
					return nil, fmt.Errorf("working hours: %s: invalid hour %v", name, h)
				}
				c.hours[wd] |= 1 << uint(hour)
			}
		}
	}

	if strings.TrimSpace(vacationDays) != "" {
		var months map[interface{}]map[interface{}]interface{}
		if err := yaml.Unmarshal([]byte(vacationDays), &months); err != nil {
			return nil, fmt.Errorf("vacation days: %w", err)
		}
		for m, days := range months {
			month, err := settingInt(m)
			if err != nil {
				return nil, fmt.Errorf("vacation days: invalid month %v", m)
			}
			for d := range days {
				day, err := settingInt(d)
				if err != nil {
					return nil, fmt.Errorf("vacation days: invalid day %v", d)
				}
				c.vacation[[2]int{month, day}] = true
			}
		}
	}

	if strings.TrimSpace(vacationDaysOneTime) != "" {
		var years map[interface{}]map[interface{}]map[interface{}]interface{}
		if err := yaml.Unmarshal([]byte(vacationDaysOneTime), &years); err != nil {
			return nil, fmt.Errorf("one-time vacation days: %w", err)
		}
		for y, months := range years {
			year, err := settingInt(y)
			if err != nil {
				return nil, fmt.Errorf("one-time vacation days: invalid year %v", y)
			}
			for m, days := range months {
				month, err := settingInt(m)
				if err != nil {
					return nil, fmt.Errorf("one-time vacation days: invalid month %v", m)
				}
				for d := range days {
					day, err := settingInt(d)
					if err != nil {
						return nil, fmt.Errorf("one-time vacation days: invalid day %v", d)
					}
					c.oneTime[[3]int{year, month, day}] = true
				}
			}
		}
	}
	return c, nil
}

var weekdays = map[string]time.Weekday{
	"Mon": time.Monday, "Tue": time.Tuesday, "Wed": time.Wednesday, "Thu": time.Thursday,
	"Fri": time.Friday, "Sat": time.Saturday, "Sun": time.Sunday,
}

func settingInt(v interface{}) (int, error) {
	return strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
}

// configured reports whether any working hour is set. OTRS does no working
// time arithmetic at all without one.
func (c *Calendar) configured() bool {
	for _, h := range c.hours {
		if h != 0 {
			return true
		}
	}
	return false
}

// workingDayHours returns the working-hour bitmap of the day t falls on, or 0
// for vacation days.
func (c *Calendar) workingDayHours(t time.Time) uint32 {
	y, m, d := t.Date()
	if c.vacation[[2]int{int(m), d}] || c.oneTime[[3]int{y, int(m), d}] {
		return 0
	}
	return c.hours[t.Weekday()]
}

// fullDay returns the start of the next day when t is a midnight that starts
// a 24-hour day (OTRS skips such days in one step).
func (c *Calendar) fullDay(t time.Time) (time.Time, bool) {
	if t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0 {
		return time.Time{}, false
	}
	next := time.Unix(t.Unix()+86400, 0).In(c.location)
	if next.Hour() != 0 || next.Minute() != 0 || next.Second() != 0 || next.Day() == t.Day() {
		return time.Time{}, false
	}
	return next, true
}

// AddWorkingTime returns the moment at which seconds of working time have
// passed after start (OTRS DateTime->Add(AsWorkingTime => 1)). Without any
// configured working hour, or for seconds <= 0, start is returned unchanged,
// as in OTRS.
func (c *Calendar) AddWorkingTime(start time.Time, seconds int64) (time.Time, error) {
	if seconds <= 0 || !c.configured() {
		return start, nil
	}
	t := time.Unix(start.Unix(), 0).In(c.location)
	remaining := seconds
	for step := 0; remaining > 0; step++ {
		if step > maxCalendarSteps {
			return start, fmt.Errorf("adding %ds of working time to %s found no working time", seconds, start.Format(time.RFC3339))
		}
		hours := c.workingDayHours(t)
		if next, ok := c.fullDay(t); ok {
			whole := true
			if hours != 0 {
				dayWorking := int64(bits.OnesCount32(hours)) * 3600
				if remaining > dayWorking {
					remaining -= dayWorking
				} else {
					whole = false
				}
			}
			if whole {
				t = next
				continue
			}
		}
		add := int64(3600 - t.Minute()*60 - t.Second())
		if hours&(1<<uint(t.Hour())) != 0 {
			if add > remaining {
				add = remaining
			}
			remaining -= add
		}
		t = time.Unix(t.Unix()+add, 0).In(c.location)
	}
	return t.In(start.Location()), nil
}

// WorkingTime returns the seconds of working time between start and stop
// (OTRS DateTime->Delta(ForWorkingTime => 1)); 0 when stop is not after start
// or no working hour is configured.
func (c *Calendar) WorkingTime(start, stop time.Time) (int64, error) {
	if !c.configured() {
		return 0, nil
	}
	t := time.Unix(start.Unix(), 0).In(c.location)
	end := stop.Unix()
	var working int64
	for step := 0; t.Unix() < end; step++ {
		if step > maxCalendarSteps {
			return 0, fmt.Errorf("working time between %s and %s: too many steps", start.Format(time.RFC3339), stop.Format(time.RFC3339))
		}
		remaining := end - t.Unix()
		hours := c.workingDayHours(t)
		if next, ok := c.fullDay(t); ok && remaining > 86400 {
			whole := true
			if hours != 0 {
				dayWorking := int64(bits.OnesCount32(hours)) * 3600
				if remaining > dayWorking {
					working += dayWorking
				} else {
					whole = false
				}
			}
			if whole {
				t = next
				continue
			}
		}
		add := int64(3600 - t.Minute()*60 - t.Second())
		if hours&(1<<uint(t.Hour())) != 0 {
			if add > remaining {
				add = remaining
			}
			working += add
		}
		t = time.Unix(t.Unix()+add, 0).In(c.location)
	}
	return working, nil
}

// Calendars holds the default calendar and the named calendars "1".."9".
type Calendars struct {
	byName map[string]*Calendar
}

// Get returns the calendar an SLA or queue calendar_name refers to ("" or
// "1".."9", optionally "Calendar1"); unknown or inactive names get the
// default calendar, as in OTRS.
func (cs *Calendars) Get(name string) *Calendar {
	name = strings.TrimPrefix(strings.TrimSpace(name), "Calendar")
	if c, ok := cs.byName[name]; ok {
		return c
	}
	return cs.byName[""]
}

// LoadCalendars reads the OTRS calendar settings: TimeWorkingHours,
// TimeVacationDays and TimeVacationDaysOneTime for the default calendar, and
// their ::CalendarN variants with TimeZone::CalendarN for calendars 1-9. A
// numbered calendar is used when its working hours are set and its
// TimeZone::CalendarNName is not blanked out; otherwise its tickets use the
// default calendar. loc is the time zone of the default calendar and of
// numbered calendars without their own TimeZone::CalendarN.
func LoadCalendars(ctx context.Context, db *sql.DB, loc *time.Location) (*Calendars, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if loc == nil {
		loc = time.UTC
	}
	setting := func(name string) (string, bool) { return sysconfig.Value(db, name) }

	def, err := loadCalendar(setting, "", loc)
	if err != nil {
		return nil, err
	}
	cs := &Calendars{byName: map[string]*Calendar{"": def}}
	for i := 1; i <= 9; i++ {
		suffix := "::Calendar" + strconv.Itoa(i)
		if name, ok := setting("TimeZone::Calendar" + strconv.Itoa(i) + "Name"); ok && scalarSetting(name) == "" {
			continue
		}
		if _, ok := setting("TimeWorkingHours" + suffix); !ok {
			continue
		}
		calLoc := loc
		if tz, ok := setting("TimeZone::Calendar" + strconv.Itoa(i)); ok && scalarSetting(tz) != "" {
			l, err := time.LoadLocation(scalarSetting(tz))
			if err != nil {
				return nil, fmt.Errorf("TimeZone::Calendar%d: %w", i, err)
			}
			calLoc = l
		}
		c, err := loadCalendar(setting, suffix, calLoc)
		if err != nil {
			return nil, err
		}
		cs.byName[strconv.Itoa(i)] = c
	}
	return cs, nil
}

func loadCalendar(setting func(string) (string, bool), suffix string, loc *time.Location) (*Calendar, error) {
	hours, _ := setting("TimeWorkingHours" + suffix)
	vacation, _ := setting("TimeVacationDays" + suffix)
	oneTime, _ := setting("TimeVacationDaysOneTime" + suffix)
	c, err := ParseCalendar(loc, hours, vacation, oneTime)
	if err != nil {
		return nil, fmt.Errorf("calendar settings%s: %w", suffix, err)
	}
	return c, nil
}

// scalarSetting decodes a scalar sysconfig value, stored either plain or as a
// YAML document ("--- Europe/Berlin\n").
func scalarSetting(v string) string {
	var s string
	if err := yaml.Unmarshal([]byte(v), &s); err == nil {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(v)
}
