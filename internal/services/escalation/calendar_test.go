package escalation

import (
	"testing"
	"time"

	"github.com/rickar/cal/v2"
)

func TestCalendarAddWorkingTime(t *testing.T) {
	// Create a business calendar with standard hours (Mon-Fri, 8-17)
	c := cal.NewBusinessCalendar()
	c.SetWorkHours(8*time.Hour, 17*time.Hour) // 8:00 - 17:00 (9 hours/day)
	c.SetWorkday(time.Saturday, false)
	c.SetWorkday(time.Sunday, false)

	tests := []struct {
		name     string
		start    time.Time
		minutes  int
		wantDay  int // Expected day of month
		wantHour int // Expected hour
	}{
		{
			name:     "add 60 minutes during work hours",
			start:    time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC), // Monday 10:00
			minutes:  60,
			wantDay:  6,
			wantHour: 11, // 11:00
		},
		{
			name:     "add time that crosses end of day",
			start:    time.Date(2025, 1, 6, 16, 0, 0, 0, time.UTC), // Monday 16:00
			minutes:  120,                                          // 2 hours
			wantDay:  7,                                             // Tuesday
			wantHour: 9,                                             // 09:00 (1 hour left on Monday, 1 hour on Tuesday)
		},
		{
			name:     "add time over weekend",
			start:    time.Date(2025, 1, 10, 16, 0, 0, 0, time.UTC), // Friday 16:00
			minutes:  120,                                            // 2 hours
			wantDay:  13,                                              // Monday
			wantHour: 9,                                               // 09:00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.AddWorkHours(tt.start, time.Duration(tt.minutes)*time.Minute)
			if result.Day() != tt.wantDay {
				t.Errorf("day = %d, want %d", result.Day(), tt.wantDay)
			}
			if result.Hour() != tt.wantHour {
				t.Errorf("hour = %d, want %d", result.Hour(), tt.wantHour)
			}
		})
	}
}

func TestCalendarWorkHoursInRange(t *testing.T) {
	c := cal.NewBusinessCalendar()
	c.SetWorkHours(9*time.Hour, 17*time.Hour) // 9:00 - 17:00 (8 hours/day)
	c.SetWorkday(time.Saturday, false)
	c.SetWorkday(time.Sunday, false)

	tests := []struct {
		name      string
		start     time.Time
		end       time.Time
		wantHours float64
	}{
		{
			name:      "full work day",
			start:     time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC),  // Monday 09:00
			end:       time.Date(2025, 1, 6, 17, 0, 0, 0, time.UTC), // Monday 17:00
			wantHours: 8,
		},
		{
			name:      "partial day",
			start:     time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC),  // Monday 09:00
			end:       time.Date(2025, 1, 6, 12, 0, 0, 0, time.UTC), // Monday 12:00
			wantHours: 3,
		},
		{
			name:      "across weekend",
			start:     time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC),  // Friday 09:00
			end:       time.Date(2025, 1, 13, 17, 0, 0, 0, time.UTC), // Monday 17:00
			wantHours: 16, // 8 hours Friday + 8 hours Monday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.WorkHoursInRange(tt.start, tt.end)
			gotHours := result.Hours()
			if gotHours != tt.wantHours {
				t.Errorf("WorkHoursInRange() = %v hours, want %v", gotHours, tt.wantHours)
			}
		})
	}
}

func TestCalendarIsWorkTime(t *testing.T) {
	c := cal.NewBusinessCalendar()
	c.SetWorkHours(8*time.Hour, 17*time.Hour)
	c.SetWorkday(time.Saturday, false)
	c.SetWorkday(time.Sunday, false)

	tests := []struct {
		name string
		time time.Time
		want bool
	}{
		{
			name: "Monday 10:00 - work time",
			time: time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "Monday 07:00 - before work",
			time: time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "Monday 18:00 - after work",
			time: time.Date(2025, 1, 6, 18, 0, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "Saturday 10:00 - weekend",
			time: time.Date(2025, 1, 11, 10, 0, 0, 0, time.UTC),
			want: false,
		},
		{
			name: "Sunday 10:00 - weekend",
			time: time.Date(2025, 1, 12, 10, 0, 0, 0, time.UTC),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.IsWorkTime(tt.time); got != tt.want {
				t.Errorf("IsWorkTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalendarWithHolidays(t *testing.T) {
	c := cal.NewBusinessCalendar()
	c.SetWorkHours(9*time.Hour, 17*time.Hour)
	c.SetWorkday(time.Saturday, false)
	c.SetWorkday(time.Sunday, false)

	// Add Christmas holiday
	christmas := &cal.Holiday{
		Name:  "Christmas",
		Type:  cal.ObservancePublic,
		Month: time.December,
		Day:   25,
		Func:  cal.CalcDayOfMonth,
	}
	c.AddHoliday(christmas)

	// Test that Christmas is not a workday
	christmasDay := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	if c.IsWorkday(christmasDay) {
		t.Error("Christmas should not be a workday")
	}

	// Test that the day before is a workday (Dec 24, 2025 is Wednesday)
	christmasEve := time.Date(2025, 12, 24, 10, 0, 0, 0, time.UTC)
	if !c.IsWorkday(christmasEve) {
		t.Error("Christmas Eve should be a workday")
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int
	}{
		{"int", 42, 42},
		{"int64", int64(42), 42},
		{"float64", float64(42.9), 42},
		{"string valid", "42", 42},
		{"string invalid", "abc", 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInt(tt.input); got != tt.want {
				t.Errorf("toInt(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
