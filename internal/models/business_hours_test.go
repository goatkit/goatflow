package models

import (
	"testing"
	"time"
)

func TestDayOfWeekConstants(t *testing.T) {
	tests := []struct {
		name     string
		day      DayOfWeek
		expected int
	}{
		{"Sunday", Sunday, 0},
		{"Monday", Monday, 1},
		{"Tuesday", Tuesday, 2},
		{"Wednesday", Wednesday, 3},
		{"Thursday", Thursday, 4},
		{"Friday", Friday, 5},
		{"Saturday", Saturday, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.day) != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.day, tt.expected)
			}
		})
	}
}

func TestGetDefaultBusinessHours(t *testing.T) {
	config := GetDefaultBusinessHours()

	if config == nil {
		t.Fatal("GetDefaultBusinessHours returned nil")
	}

	if config.Name != "Default Business Hours" {
		t.Errorf("Name = %q, want %q", config.Name, "Default Business Hours")
	}

	if config.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", config.Timezone, "America/New_York")
	}

	if !config.IsDefault {
		t.Error("IsDefault should be true")
	}

	if !config.IsActive {
		t.Error("IsActive should be true")
	}

	if len(config.WorkingDays) != 7 {
		t.Errorf("WorkingDays len = %d, want 7", len(config.WorkingDays))
	}

	// Check Monday-Friday are working days
	for _, wd := range config.WorkingDays {
		if wd.Day >= Monday && wd.Day <= Friday {
			if !wd.IsWorking {
				t.Errorf("Day %d should be working", wd.Day)
			}
			if len(wd.Shifts) != 1 {
				t.Errorf("Day %d should have 1 shift, got %d", wd.Day, len(wd.Shifts))
			}
			if wd.Shifts[0].StartTime != "09:00" || wd.Shifts[0].EndTime != "17:00" {
				t.Errorf("Day %d shift should be 09:00-17:00", wd.Day)
			}
		}
	}

	// Check Saturday and Sunday are non-working
	for _, wd := range config.WorkingDays {
		if wd.Day == Saturday || wd.Day == Sunday {
			if wd.IsWorking {
				t.Errorf("Day %d should not be working", wd.Day)
			}
		}
	}
}

func TestGet24x7BusinessHours(t *testing.T) {
	config := Get24x7BusinessHours()

	if config == nil {
		t.Fatal("Get24x7BusinessHours returned nil")
	}

	if config.Name != "24x7 Support" {
		t.Errorf("Name = %q, want %q", config.Name, "24x7 Support")
	}

	if config.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want %q", config.Timezone, "UTC")
	}

	if config.IsDefault {
		t.Error("IsDefault should be false")
	}

	if len(config.WorkingDays) != 7 {
		t.Errorf("WorkingDays len = %d, want 7", len(config.WorkingDays))
	}

	// All days should be working
	for _, wd := range config.WorkingDays {
		if !wd.IsWorking {
			t.Errorf("Day %d should be working in 24x7 config", wd.Day)
		}
		if len(wd.Shifts) != 1 {
			t.Errorf("Day %d should have 1 shift", wd.Day)
		}
		if wd.Shifts[0].StartTime != "00:00" || wd.Shifts[0].EndTime != "23:59" {
			t.Errorf("Day %d should have 00:00-23:59 shift", wd.Day)
		}
	}
}

func TestNewBusinessHoursCalculator(t *testing.T) {
	t.Run("valid timezone", func(t *testing.T) {
		config := GetDefaultBusinessHours()
		calc, err := NewBusinessHoursCalculator(config)

		if err != nil {
			t.Fatalf("NewBusinessHoursCalculator returned error: %v", err)
		}
		if calc == nil {
			t.Fatal("calculator is nil")
		}
		if calc.Config != config {
			t.Error("config not set correctly")
		}
		if calc.locationCache == nil {
			t.Error("locationCache not set")
		}
	})

	t.Run("invalid timezone", func(t *testing.T) {
		config := GetDefaultBusinessHours()
		config.Timezone = "Invalid/Timezone"

		_, err := NewBusinessHoursCalculator(config)
		if err == nil {
			t.Error("expected error for invalid timezone")
		}
	})

	t.Run("caches holidays", func(t *testing.T) {
		config := GetDefaultBusinessHours()
		config.Holidays = []Holiday{
			{Name: "Christmas", Date: time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)},
			{Name: "New Year", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		}

		calc, err := NewBusinessHoursCalculator(config)
		if err != nil {
			t.Fatalf("error creating calculator: %v", err)
		}

		if len(calc.holidayCache) != 2 {
			t.Errorf("holidayCache len = %d, want 2", len(calc.holidayCache))
		}
	})
}

func TestBusinessHoursCalculator_IsBusinessDay(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC" // Use UTC for simpler testing
	config.Holidays = []Holiday{
		{Name: "Christmas", Date: time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)},
	}
	config.Exceptions = []BusinessException{
		{Name: "Special Saturday", Date: time.Date(2025, 12, 20, 0, 0, 0, 0, time.UTC), IsWorking: true},
	}

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	tests := []struct {
		name string
		date time.Time
		want bool
	}{
		{
			name: "Monday is business day",
			date: time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC), // Monday
			want: true,
		},
		{
			name: "Tuesday is business day",
			date: time.Date(2025, 12, 23, 10, 0, 0, 0, time.UTC), // Tuesday
			want: true,
		},
		{
			name: "Wednesday is business day",
			date: time.Date(2025, 12, 24, 10, 0, 0, 0, time.UTC), // Wednesday
			want: true,
		},
		{
			name: "Saturday is not business day",
			date: time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC), // Saturday
			want: false,
		},
		{
			name: "Sunday is not business day",
			date: time.Date(2025, 12, 28, 10, 0, 0, 0, time.UTC), // Sunday
			want: false,
		},
		{
			name: "Christmas holiday",
			date: time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC), // Thursday - Christmas
			want: false,
		},
		{
			name: "Special working Saturday exception",
			date: time.Date(2025, 12, 20, 10, 0, 0, 0, time.UTC), // Saturday with exception
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.IsBusinessDay(tt.date)
			if got != tt.want {
				t.Errorf("IsBusinessDay(%v) = %v, want %v", tt.date, got, tt.want)
			}
		})
	}
}

func TestBusinessHoursCalculator_IsWithinBusinessHours(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	tests := []struct {
		name string
		time time.Time
		want bool
	}{
		{
			name: "Monday 10:00 is within hours",
			time: time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC), // Monday 10 AM
			want: true,
		},
		{
			name: "Monday 09:00 is within hours",
			time: time.Date(2025, 12, 22, 9, 0, 0, 0, time.UTC), // Monday 9 AM
			want: true,
		},
		{
			name: "Monday 08:00 is before hours",
			time: time.Date(2025, 12, 22, 8, 0, 0, 0, time.UTC), // Monday 8 AM
			want: false,
		},
		{
			name: "Monday 17:00 is after hours",
			time: time.Date(2025, 12, 22, 17, 0, 0, 0, time.UTC), // Monday 5 PM
			want: false,
		},
		{
			name: "Monday 16:59 is within hours",
			time: time.Date(2025, 12, 22, 16, 59, 0, 0, time.UTC), // Monday 4:59 PM
			want: true,
		},
		{
			name: "Saturday is never within hours",
			time: time.Date(2025, 12, 27, 12, 0, 0, 0, time.UTC), // Saturday noon
			want: false,
		},
		{
			name: "Sunday is never within hours",
			time: time.Date(2025, 12, 28, 12, 0, 0, 0, time.UTC), // Sunday noon
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.IsWithinBusinessHours(tt.time)
			if got != tt.want {
				t.Errorf("IsWithinBusinessHours(%v) = %v, want %v", tt.time, got, tt.want)
			}
		})
	}
}

func TestBusinessHoursCalculator_GetNextBusinessDay(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	tests := []struct {
		name     string
		from     time.Time
		wantDay  time.Weekday
		wantDate int
	}{
		{
			name:     "Monday -> Tuesday",
			from:     time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC), // Monday
			wantDay:  time.Tuesday,
			wantDate: 23,
		},
		{
			name:     "Friday -> Monday",
			from:     time.Date(2025, 12, 26, 10, 0, 0, 0, time.UTC), // Friday
			wantDay:  time.Monday,
			wantDate: 29,
		},
		{
			name:     "Saturday -> Monday",
			from:     time.Date(2025, 12, 27, 10, 0, 0, 0, time.UTC), // Saturday
			wantDay:  time.Monday,
			wantDate: 29,
		},
		{
			name:     "Sunday -> Monday",
			from:     time.Date(2025, 12, 28, 10, 0, 0, 0, time.UTC), // Sunday
			wantDay:  time.Monday,
			wantDate: 29,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.GetNextBusinessDay(tt.from)
			if got.Weekday() != tt.wantDay {
				t.Errorf("GetNextBusinessDay() weekday = %v, want %v", got.Weekday(), tt.wantDay)
			}
			if got.Day() != tt.wantDate {
				t.Errorf("GetNextBusinessDay() day = %d, want %d", got.Day(), tt.wantDate)
			}
		})
	}
}

func TestBusinessHoursCalculator_GetNextBusinessHour(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	tests := []struct {
		name     string
		from     time.Time
		wantHour int
		wantDay  int
	}{
		{
			name:     "during business hours returns same time",
			from:     time.Date(2025, 12, 22, 10, 30, 0, 0, time.UTC), // Monday 10:30
			wantHour: 10,
			wantDay:  22,
		},
		{
			name:     "before business hours returns 9 AM same day",
			from:     time.Date(2025, 12, 22, 7, 0, 0, 0, time.UTC), // Monday 7 AM
			wantHour: 9,
			wantDay:  22,
		},
		{
			name:     "after business hours returns 9 AM next day",
			from:     time.Date(2025, 12, 22, 18, 0, 0, 0, time.UTC), // Monday 6 PM
			wantHour: 9,
			wantDay:  23,
		},
		{
			name:     "Saturday returns 9 AM Monday",
			from:     time.Date(2025, 12, 27, 12, 0, 0, 0, time.UTC), // Saturday noon
			wantHour: 9,
			wantDay:  29,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.GetNextBusinessHour(tt.from)
			if got.Hour() != tt.wantHour {
				t.Errorf("GetNextBusinessHour() hour = %d, want %d", got.Hour(), tt.wantHour)
			}
			if got.Day() != tt.wantDay {
				t.Errorf("GetNextBusinessHour() day = %d, want %d", got.Day(), tt.wantDay)
			}
		})
	}
}

func TestBusinessHoursCalculator_AddBusinessHours(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	tests := []struct {
		name      string
		from      time.Time
		hours     float64
		wantHour  int
		wantDay   int
		wantMonth time.Month
	}{
		{
			name:      "add 0 hours returns same time",
			from:      time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC),
			hours:     0,
			wantHour:  10,
			wantDay:   22,
			wantMonth: time.December,
		},
		{
			name:      "add 1 hour within day",
			from:      time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC), // Monday 10 AM
			hours:     1,
			wantHour:  11,
			wantDay:   22,
			wantMonth: time.December,
		},
		{
			name:      "add 4 hours within day",
			from:      time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC), // Monday 10 AM
			hours:     4,
			wantHour:  14,
			wantDay:   22,
			wantMonth: time.December,
		},
		{
			name:      "add hours spanning to next day",
			from:      time.Date(2025, 12, 22, 15, 0, 0, 0, time.UTC), // Monday 3 PM
			hours:     4,
			wantHour:  11, // 2 hours left today + 2 more = 11 AM next day
			wantDay:   23,
			wantMonth: time.December,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.AddBusinessHours(tt.from, tt.hours)
			if got.Hour() != tt.wantHour {
				t.Errorf("AddBusinessHours() hour = %d, want %d", got.Hour(), tt.wantHour)
			}
			if got.Day() != tt.wantDay {
				t.Errorf("AddBusinessHours() day = %d, want %d", got.Day(), tt.wantDay)
			}
			if got.Month() != tt.wantMonth {
				t.Errorf("AddBusinessHours() month = %v, want %v", got.Month(), tt.wantMonth)
			}
		})
	}
}

func TestBusinessHoursCalculator_GetBusinessHoursBetween(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  float64
	}{
		{
			name:  "end before start returns 0",
			start: time.Date(2025, 12, 22, 12, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC),
			want:  0,
		},
		{
			name:  "same time returns 0",
			start: time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC),
			want:  0,
		},
		{
			name:  "1 hour within business hours",
			start: time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 12, 22, 11, 0, 0, 0, time.UTC),
			want:  1,
		},
		{
			name:  "full business day (8 hours)",
			start: time.Date(2025, 12, 22, 9, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 12, 22, 17, 0, 0, 0, time.UTC),
			want:  8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.GetBusinessHoursBetween(tt.start, tt.end)
			if got != tt.want {
				t.Errorf("GetBusinessHoursBetween() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBusinessHoursCalculator_isTimeInRange(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	baseDate := time.Date(2025, 12, 22, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		testTime  time.Time
		startStr  string
		endStr    string
		wantInRange bool
	}{
		{
			name:      "within range",
			testTime:  baseDate.Add(10 * time.Hour), // 10:00
			startStr:  "09:00",
			endStr:    "17:00",
			wantInRange: true,
		},
		{
			name:      "at start",
			testTime:  baseDate.Add(9 * time.Hour), // 09:00
			startStr:  "09:00",
			endStr:    "17:00",
			wantInRange: true,
		},
		{
			name:      "before start",
			testTime:  baseDate.Add(8 * time.Hour), // 08:00
			startStr:  "09:00",
			endStr:    "17:00",
			wantInRange: false,
		},
		{
			name:      "at end (exclusive)",
			testTime:  baseDate.Add(17 * time.Hour), // 17:00
			startStr:  "09:00",
			endStr:    "17:00",
			wantInRange: false,
		},
		{
			name:      "after end",
			testTime:  baseDate.Add(18 * time.Hour), // 18:00
			startStr:  "09:00",
			endStr:    "17:00",
			wantInRange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.isTimeInRange(tt.testTime, tt.startStr, tt.endStr)
			if got != tt.wantInRange {
				t.Errorf("isTimeInRange(%v, %s, %s) = %v, want %v", 
					tt.testTime.Format("15:04"), tt.startStr, tt.endStr, got, tt.wantInRange)
			}
		})
	}
}

func TestBusinessHoursCalculator_isTimeInShift(t *testing.T) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"

	calc, err := NewBusinessHoursCalculator(config)
	if err != nil {
		t.Fatalf("error creating calculator: %v", err)
	}

	baseDate := time.Date(2025, 12, 22, 0, 0, 0, 0, time.UTC)

	t.Run("simple shift without break", func(t *testing.T) {
		shift := TimeShift{StartTime: "09:00", EndTime: "17:00"}

		if !calc.isTimeInShift(baseDate.Add(10*time.Hour), shift) {
			t.Error("10:00 should be in shift")
		}
		if calc.isTimeInShift(baseDate.Add(8*time.Hour), shift) {
			t.Error("08:00 should not be in shift")
		}
	})

	t.Run("shift with break", func(t *testing.T) {
		shift := TimeShift{
			StartTime:  "09:00",
			EndTime:    "17:00",
			BreakStart: "12:00",
			BreakEnd:   "13:00",
		}

		if !calc.isTimeInShift(baseDate.Add(10*time.Hour), shift) {
			t.Error("10:00 should be in shift (before break)")
		}
		if calc.isTimeInShift(baseDate.Add(12*time.Hour+30*time.Minute), shift) {
			t.Error("12:30 should not be in shift (during break)")
		}
		if !calc.isTimeInShift(baseDate.Add(14*time.Hour), shift) {
			t.Error("14:00 should be in shift (after break)")
		}
	})
}

func TestTimeShift_Fields(t *testing.T) {
	shift := TimeShift{
		StartTime:  "09:00",
		EndTime:    "17:00",
		BreakStart: "12:00",
		BreakEnd:   "13:00",
	}

	if shift.StartTime != "09:00" {
		t.Errorf("StartTime = %q, want %q", shift.StartTime, "09:00")
	}
	if shift.EndTime != "17:00" {
		t.Errorf("EndTime = %q, want %q", shift.EndTime, "17:00")
	}
	if shift.BreakStart != "12:00" {
		t.Errorf("BreakStart = %q, want %q", shift.BreakStart, "12:00")
	}
	if shift.BreakEnd != "13:00" {
		t.Errorf("BreakEnd = %q, want %q", shift.BreakEnd, "13:00")
	}
}

func TestWorkingDay_Fields(t *testing.T) {
	wd := WorkingDay{
		Day:       Monday,
		IsWorking: true,
		Shifts:    []TimeShift{{StartTime: "09:00", EndTime: "17:00"}},
	}

	if wd.Day != Monday {
		t.Errorf("Day = %v, want %v", wd.Day, Monday)
	}
	if !wd.IsWorking {
		t.Error("IsWorking should be true")
	}
	if len(wd.Shifts) != 1 {
		t.Errorf("Shifts len = %d, want 1", len(wd.Shifts))
	}
}

func TestHoliday_Fields(t *testing.T) {
	h := Holiday{
		ID:           1,
		ConfigID:     1,
		Name:         "Christmas",
		Date:         time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC),
		IsRecurring:  true,
		IsFloating:   false,
		FloatingRule: "",
	}

	if h.Name != "Christmas" {
		t.Errorf("Name = %q, want %q", h.Name, "Christmas")
	}
	if !h.IsRecurring {
		t.Error("IsRecurring should be true")
	}
	if h.IsFloating {
		t.Error("IsFloating should be false")
	}
}

func TestBusinessException_Fields(t *testing.T) {
	ex := BusinessException{
		ID:        1,
		ConfigID:  1,
		Name:      "Special Saturday",
		Date:      time.Date(2025, 12, 20, 0, 0, 0, 0, time.UTC),
		IsWorking: true,
		StartTime: "10:00",
		EndTime:   "14:00",
		Reason:    "Year-end rush",
	}

	if ex.Name != "Special Saturday" {
		t.Errorf("Name = %q, want %q", ex.Name, "Special Saturday")
	}
	if !ex.IsWorking {
		t.Error("IsWorking should be true")
	}
	if ex.Reason != "Year-end rush" {
		t.Errorf("Reason = %q, want %q", ex.Reason, "Year-end rush")
	}
}

func BenchmarkIsBusinessDay(b *testing.B) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"
	calc, _ := NewBusinessHoursCalculator(config)
	testTime := time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.IsBusinessDay(testTime)
	}
}

func BenchmarkIsWithinBusinessHours(b *testing.B) {
	config := GetDefaultBusinessHours()
	config.Timezone = "UTC"
	calc, _ := NewBusinessHoursCalculator(config)
	testTime := time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.IsWithinBusinessHours(testTime)
	}
}
