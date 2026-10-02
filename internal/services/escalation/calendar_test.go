package escalation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const officeHours = `{Mon: [8,9,10,11,12,13,14,15,16,17], Tue: [8,9,10,11,12,13,14,15,16,17],
	Wed: [8,9,10,11,12,13,14,15,16,17], Thu: [8,9,10,11,12,13,14,15,16,17],
	Fri: [8,9,10,11,12,13,14,15,16,17], Sat: [], Sun: []}`

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func mustCalendar(t *testing.T, loc *time.Location, hours, vacation, oneTime string) *Calendar {
	t.Helper()
	c, err := ParseCalendar(loc, hours, vacation, oneTime)
	require.NoError(t, err)
	return c
}

func add(t *testing.T, c *Calendar, start time.Time, d time.Duration) time.Time {
	t.Helper()
	got, err := c.AddWorkingTime(start, int64(d/time.Second))
	require.NoError(t, err)
	return got
}

func TestAddWorkingTime_OfficeHours(t *testing.T) {
	c := mustCalendar(t, time.UTC, officeHours, "", "")
	// 2026-01-09 is a Friday.
	cases := []struct {
		name  string
		start time.Time
		d     time.Duration
		want  time.Time
	}{
		{"inside one day", utc(2026, 1, 5, 9, 15), 2 * time.Hour, utc(2026, 1, 5, 11, 15)},
		{"before opening starts at opening", utc(2026, 1, 5, 6, 0), 30 * time.Minute, utc(2026, 1, 5, 8, 30)},
		{"after closing rolls over the weekend", utc(2026, 1, 9, 17, 30), time.Hour, utc(2026, 1, 12, 8, 30)},
		{"whole working days", utc(2026, 1, 5, 8, 0), 25 * time.Hour, utc(2026, 1, 7, 13, 0)},
		{"ends exactly at closing", utc(2026, 1, 5, 17, 0), time.Hour, utc(2026, 1, 5, 18, 0)},
		{"started on a Sunday", utc(2026, 1, 11, 12, 0), 10 * time.Hour, utc(2026, 1, 12, 18, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, add(t, c, tc.start, tc.d))
		})
	}
}

// OTRS working hours are per day and need not be contiguous; a day missing
// from the setting is not a working day.
func TestAddWorkingTime_PerDayHours(t *testing.T) {
	// The OTRS 6 setting format: a YAML document with string hours.
	c := mustCalendar(t, time.UTC, "---\nSat:\n- '10'\n- '11'\nWed:\n- '8'\n- '14'\n", "", "")

	// 2026-01-10 is a Saturday: 10:00-12:00 count, then the next working
	// hour is Wednesday 08:00, then Wednesday 14:00.
	assert.Equal(t, utc(2026, 1, 10, 11, 30), add(t, c, utc(2026, 1, 10, 9, 0), 90*time.Minute))
	assert.Equal(t, utc(2026, 1, 14, 8, 30), add(t, c, utc(2026, 1, 10, 11, 0), 90*time.Minute))
	assert.Equal(t, utc(2026, 1, 14, 14, 30), add(t, c, utc(2026, 1, 14, 8, 30), time.Hour))
	// Monday, Tuesday, Thursday, Friday and Sunday are not listed: no working time.
	wt, err := c.WorkingTime(utc(2026, 1, 11, 0, 0), utc(2026, 1, 14, 0, 0))
	require.NoError(t, err)
	assert.Zero(t, wt)
}

func TestAddWorkingTime_VacationDays(t *testing.T) {
	c := mustCalendar(t, time.UTC, officeHours,
		"---\n'12':\n  '25': First Christmas Day\n  '26': Second Christmas Day\n",
		"---\n'2026':\n  '12':\n    '28': Bridge day\n")
	// Thu 2026-12-24 16:00 + 4h: 2h on Thursday, Fri 25 is a recurring
	// vacation day, Mon 28 a one-time vacation day, so Tue 29 08:00-10:00.
	assert.Equal(t, utc(2026, 12, 29, 10, 0), add(t, c, utc(2026, 12, 24, 16, 0), 4*time.Hour))
	// The one-time day is not a holiday in other years: Mon 2027-12-27 has
	// no vacation, but Sat 25/Sun 26 are weekend anyway.
	assert.Equal(t, utc(2027, 12, 27, 10, 0), add(t, c, utc(2027, 12, 24, 16, 0), 4*time.Hour))
}

func TestAddWorkingTime_CalendarTimeZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	c := mustCalendar(t, berlin, "{Mon: [9,10,11,12,13,14,15,16]}", "", "")
	// Mon 2026-01-05 07:30 UTC is 08:30 in Berlin (CET): work starts at 09:00
	// Berlin = 08:00 UTC, one hour later is 09:00 UTC.
	got := add(t, c, utc(2026, 1, 5, 7, 30), time.Hour)
	assert.Equal(t, utc(2026, 1, 5, 9, 0), got.UTC())
	// In summer (CEST, UTC+2) the same Berlin hours are an hour earlier in UTC.
	got = add(t, c, utc(2026, 7, 6, 6, 30), time.Hour)
	assert.Equal(t, utc(2026, 7, 6, 8, 0), got.UTC())
}

// Working time is counted in real seconds, so across the daylight saving
// switch a 24/7 calendar adds exactly the requested duration.
func TestAddWorkingTime_DaylightSavingSwitch(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	all := "{Mon: [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23], " +
		"Tue: [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23], " +
		"Wed: [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23], " +
		"Thu: [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23], " +
		"Fri: [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23], " +
		"Sat: [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23], " +
		"Sun: [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23]}"
	c := mustCalendar(t, berlin, all, "", "")
	start := time.Date(2026, 3, 28, 12, 0, 0, 0, berlin) // the night of 28/29 March has 23 hours
	for _, d := range []time.Duration{24 * time.Hour, 72 * time.Hour, 90 * time.Minute} {
		assert.True(t, start.Add(d).Equal(add(t, c, start, d)), "add %s", d)
	}
}

// Without any working hour OTRS does no working-time arithmetic: the
// destination is the start and no working time passes.
func TestCalendarWithoutWorkingHours(t *testing.T) {
	c := mustCalendar(t, time.UTC, "{Mon: [], Tue: []}", "", "")
	start := utc(2026, 1, 5, 9, 0)
	assert.Equal(t, start, add(t, c, start, 8*time.Hour))
	wt, err := c.WorkingTime(start, start.Add(48*time.Hour))
	require.NoError(t, err)
	assert.Zero(t, wt)
}

func TestWorkingTimeIsTheInverseOfAddWorkingTime(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	cals := map[string]*Calendar{
		"office":   mustCalendar(t, time.UTC, officeHours, "{'12': {'25': x}}", ""),
		"weekends": mustCalendar(t, berlin, "{Sat: ['10', '11'], Sun: ['0', '23']}", "", ""),
	}
	starts := []time.Time{utc(2026, 1, 5, 9, 17), utc(2026, 3, 27, 23, 59), utc(2026, 12, 24, 17, 59), utc(2026, 10, 24, 22, 0)}
	durations := []time.Duration{time.Minute, 59 * time.Minute, 3 * time.Hour, 40 * time.Hour, 500 * time.Hour}
	for name, c := range cals {
		for _, s := range starts {
			for _, d := range durations {
				end := add(t, c, s, d)
				wt, err := c.WorkingTime(s, end)
				require.NoError(t, err)
				assert.Equal(t, int64(d/time.Second), wt, "%s: %s + %s = %s", name, s, d, end)
			}
		}
	}
	wt, err := cals["office"].WorkingTime(utc(2026, 1, 5, 12, 0), utc(2026, 1, 5, 11, 0))
	require.NoError(t, err)
	assert.Zero(t, wt, "stop before start")
}

func TestParseCalendarRejectsInvalidSettings(t *testing.T) {
	for _, tc := range []struct{ hours, vacation, oneTime string }{
		{"{Mon: [24]}", "", ""},
		{"{Monday: [8]}", "", ""},
		{"{Mon: [eight]}", "", ""},
		{"[8, 9]", "", ""},
		{"", "{'12': {'x': y}}", ""},
		{"", "", "{'2026': {'1': {'z': y}}}"},
	} {
		_, err := ParseCalendar(time.UTC, tc.hours, tc.vacation, tc.oneTime)
		assert.Error(t, err, "%+v", tc)
	}
}

func TestCalendarsGetFallsBackToDefault(t *testing.T) {
	def := mustCalendar(t, time.UTC, officeHours, "", "")
	one := mustCalendar(t, time.UTC, "{Sat: [10]}", "", "")
	cs := &Calendars{byName: map[string]*Calendar{"": def, "1": one}}
	assert.Same(t, one, cs.Get("1"))
	assert.Same(t, one, cs.Get("Calendar1"))
	assert.Same(t, def, cs.Get(""))
	assert.Same(t, def, cs.Get("2"))
}
