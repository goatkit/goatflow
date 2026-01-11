package genericagent

import (
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

func TestService_shouldRun(t *testing.T) {
	// Fixed time: Wednesday, Jan 15, 2025, 10:30:00 UTC
	fixedNow := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	svc := &Service{
		now: func() time.Time { return fixedNow },
	}

	tests := []struct {
		name     string
		job      *models.GenericAgentJob
		expected bool
	}{
		{
			name: "no schedule defined",
			job: &models.GenericAgentJob{
				Name:   "test",
				Config: map[string]string{},
			},
			expected: false,
		},
		{
			name: "matching day, hour, minute",
			job: &models.GenericAgentJob{
				Name: "test",
				Config: map[string]string{
					"ScheduleDays[0]":    "3", // Wednesday
					"ScheduleHours[0]":   "10",
					"ScheduleMinutes[0]": "30",
				},
			},
			expected: true,
		},
		{
			name: "wrong day",
			job: &models.GenericAgentJob{
				Name: "test",
				Config: map[string]string{
					"ScheduleDays[0]":    "1", // Monday
					"ScheduleHours[0]":   "10",
					"ScheduleMinutes[0]": "30",
				},
			},
			expected: false,
		},
		{
			name: "wrong hour",
			job: &models.GenericAgentJob{
				Name: "test",
				Config: map[string]string{
					"ScheduleDays[0]":    "3",
					"ScheduleHours[0]":   "9", // Wrong hour
					"ScheduleMinutes[0]": "30",
				},
			},
			expected: false,
		},
		{
			name: "wrong minute",
			job: &models.GenericAgentJob{
				Name: "test",
				Config: map[string]string{
					"ScheduleDays[0]":    "3",
					"ScheduleHours[0]":   "10",
					"ScheduleMinutes[0]": "0", // Wrong minute
				},
			},
			expected: false,
		},
		{
			name: "multiple days include current",
			job: &models.GenericAgentJob{
				Name: "test",
				Config: map[string]string{
					"ScheduleDays[0]":    "1",
					"ScheduleDays[1]":    "3", // Wednesday - matches
					"ScheduleDays[2]":    "5",
					"ScheduleHours[0]":   "10",
					"ScheduleMinutes[0]": "30",
				},
			},
			expected: true,
		},
		{
			name: "no day restriction",
			job: &models.GenericAgentJob{
				Name: "test",
				Config: map[string]string{
					"ScheduleHours[0]":   "10",
					"ScheduleMinutes[0]": "30",
				},
			},
			expected: true,
		},
		{
			name: "already run this minute",
			job: &models.GenericAgentJob{
				Name:      "test",
				LastRunAt: &fixedNow,
				Config: map[string]string{
					"ScheduleDays[0]":    "3",
					"ScheduleHours[0]":   "10",
					"ScheduleMinutes[0]": "30",
				},
			},
			expected: false,
		},
		{
			name: "last run was previous minute",
			job: &models.GenericAgentJob{
				Name:      "test",
				LastRunAt: func() *time.Time { t := fixedNow.Add(-1 * time.Minute); return &t }(),
				Config: map[string]string{
					"ScheduleDays[0]":    "3",
					"ScheduleHours[0]":   "10",
					"ScheduleMinutes[0]": "30",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.shouldRun(tt.job, fixedNow)
			if got != tt.expected {
				t.Errorf("shouldRun() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestContainsInt(t *testing.T) {
	tests := []struct {
		slice    []int
		val      int
		expected bool
	}{
		{[]int{1, 2, 3}, 2, true},
		{[]int{1, 2, 3}, 4, false},
		{[]int{}, 1, false},
		{nil, 1, false},
		{[]int{0}, 0, true},
	}

	for _, tt := range tests {
		got := containsInt(tt.slice, tt.val)
		if got != tt.expected {
			t.Errorf("containsInt(%v, %d) = %v, want %v", tt.slice, tt.val, got, tt.expected)
		}
	}
}

func TestMakePlaceholders(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{0, ""},
		{1, "?"},
		{3, "?, ?, ?"},
		{5, "?, ?, ?, ?, ?"},
	}

	for _, tt := range tests {
		idx := 0
		got := makePlaceholders(tt.count, &idx)
		if got != tt.expected {
			t.Errorf("makePlaceholders(%d) = %q, want %q", tt.count, got, tt.expected)
		}
	}
}
