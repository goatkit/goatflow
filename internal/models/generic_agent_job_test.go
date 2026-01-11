package models

import (
	"testing"
	"time"
)

func TestGenericAgentJob_ScheduleDays(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]string
		expected []int
	}{
		{
			name:     "empty config",
			config:   map[string]string{},
			expected: nil,
		},
		{
			name: "OTRS array format",
			config: map[string]string{
				"ScheduleDays[0]": "1",
				"ScheduleDays[1]": "3",
				"ScheduleDays[2]": "5",
			},
			expected: []int{1, 3, 5},
		},
		{
			name: "comma-separated format",
			config: map[string]string{
				"ScheduleDays": "0,2,4,6",
			},
			expected: []int{0, 2, 4, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &GenericAgentJob{Config: tt.config}
			got := job.ScheduleDays()
			if len(got) != len(tt.expected) {
				t.Errorf("ScheduleDays() = %v, want %v", got, tt.expected)
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("ScheduleDays()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestGenericAgentJob_ScheduleHours(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"ScheduleHours[0]": "9",
			"ScheduleHours[1]": "12",
			"ScheduleHours[2]": "17",
		},
	}

	got := job.ScheduleHours()
	expected := []int{9, 12, 17}

	if len(got) != len(expected) {
		t.Errorf("ScheduleHours() = %v, want %v", got, expected)
		return
	}
	for i, v := range got {
		if v != expected[i] {
			t.Errorf("ScheduleHours()[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

func TestGenericAgentJob_ScheduleMinutes(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"ScheduleMinutes": "0,15,30,45",
		},
	}

	got := job.ScheduleMinutes()
	expected := []int{0, 15, 30, 45}

	if len(got) != len(expected) {
		t.Errorf("ScheduleMinutes() = %v, want %v", got, expected)
		return
	}
	for i, v := range got {
		if v != expected[i] {
			t.Errorf("ScheduleMinutes()[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

func TestGenericAgentMatchCriteria_StateIDs(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"StateIDs[0]": "1",
			"StateIDs[1]": "2",
			"StateIDs[2]": "4",
		},
	}

	criteria := job.MatchCriteria()
	got := criteria.StateIDs()
	expected := []int{1, 2, 4}

	if len(got) != len(expected) {
		t.Errorf("StateIDs() = %v, want %v", got, expected)
		return
	}
	for i, v := range got {
		if v != expected[i] {
			t.Errorf("StateIDs()[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

func TestGenericAgentMatchCriteria_TimeFilters(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"TicketCreateTimeOlderMinutes": "60",
			"TicketChangeTimeNewerMinutes": "30",
		},
	}

	criteria := job.MatchCriteria()

	if got := criteria.TicketCreateTimeOlderMinutes(); got != 60 {
		t.Errorf("TicketCreateTimeOlderMinutes() = %v, want 60", got)
	}
	if got := criteria.TicketChangeTimeNewerMinutes(); got != 30 {
		t.Errorf("TicketChangeTimeNewerMinutes() = %v, want 30", got)
	}
}

func TestGenericAgentMatchCriteria_StringFilters(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"CustomerID":        "customer*",
			"CustomerUserLogin": "john.doe",
			"Title":             "*urgent*",
		},
	}

	criteria := job.MatchCriteria()

	if got := criteria.CustomerID(); got != "customer*" {
		t.Errorf("CustomerID() = %v, want customer*", got)
	}
	if got := criteria.CustomerUserLogin(); got != "john.doe" {
		t.Errorf("CustomerUserLogin() = %v, want john.doe", got)
	}
	if got := criteria.Title(); got != "*urgent*" {
		t.Errorf("Title() = %v, want *urgent*", got)
	}
}

func TestGenericAgentMatchCriteria_HasCriteria(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]string
		expected bool
	}{
		{
			name:     "empty config",
			config:   map[string]string{},
			expected: false,
		},
		{
			name:     "with state IDs",
			config:   map[string]string{"StateIDs[0]": "1"},
			expected: true,
		},
		{
			name:     "with customer ID",
			config:   map[string]string{"CustomerID": "test"},
			expected: true,
		},
		{
			name:     "with time filter",
			config:   map[string]string{"TicketCreateTimeOlderMinutes": "60"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &GenericAgentJob{Config: tt.config}
			criteria := job.MatchCriteria()
			if got := criteria.HasCriteria(); got != tt.expected {
				t.Errorf("HasCriteria() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenericAgentActions_NewStateID(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"NewStateID": "3",
		},
	}

	actions := job.Actions()
	got := actions.NewStateID()

	if got == nil {
		t.Error("NewStateID() = nil, want 3")
		return
	}
	if *got != 3 {
		t.Errorf("NewStateID() = %v, want 3", *got)
	}
}

func TestGenericAgentActions_NewQueueID(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"NewQueueID": "5",
		},
	}

	actions := job.Actions()
	got := actions.NewQueueID()

	if got == nil {
		t.Error("NewQueueID() = nil, want 5")
		return
	}
	if *got != 5 {
		t.Errorf("NewQueueID() = %v, want 5", *got)
	}
}

func TestGenericAgentActions_NoteBody(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"NewNoteBody":    "This is a test note",
			"NewNoteSubject": "Test Subject",
		},
	}

	actions := job.Actions()

	if got := actions.NoteBody(); got != "This is a test note" {
		t.Errorf("NoteBody() = %v, want 'This is a test note'", got)
	}
	if got := actions.NoteSubject(); got != "Test Subject" {
		t.Errorf("NoteSubject() = %v, want 'Test Subject'", got)
	}
}

func TestGenericAgentActions_NewPendingTime(t *testing.T) {
	expected := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	job := &GenericAgentJob{
		Config: map[string]string{
			"NewPendingTime": expected.Format(time.RFC3339),
		},
	}

	actions := job.Actions()
	got := actions.NewPendingTime()

	if got == nil {
		t.Error("NewPendingTime() = nil, want time")
		return
	}
	if !got.Equal(expected) {
		t.Errorf("NewPendingTime() = %v, want %v", got, expected)
	}
}

func TestGenericAgentActions_NewPendingTimeDiff(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"NewPendingTimeDiff": "120",
		},
	}

	actions := job.Actions()

	if got := actions.NewPendingTimeDiff(); got != 120 {
		t.Errorf("NewPendingTimeDiff() = %v, want 120", got)
	}
}

func TestGenericAgentActions_Delete(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]string
		expected bool
	}{
		{
			name:     "not set",
			config:   map[string]string{},
			expected: false,
		},
		{
			name:     "set to 1",
			config:   map[string]string{"NewDelete": "1"},
			expected: true,
		},
		{
			name:     "set to 0",
			config:   map[string]string{"NewDelete": "0"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &GenericAgentJob{Config: tt.config}
			actions := job.Actions()
			if got := actions.Delete(); got != tt.expected {
				t.Errorf("Delete() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenericAgentActions_HasActions(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]string
		expected bool
	}{
		{
			name:     "empty config",
			config:   map[string]string{},
			expected: false,
		},
		{
			name:     "with new state",
			config:   map[string]string{"NewStateID": "3"},
			expected: true,
		},
		{
			name:     "with note",
			config:   map[string]string{"NewNoteBody": "test"},
			expected: true,
		},
		{
			name:     "with delete",
			config:   map[string]string{"NewDelete": "1"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &GenericAgentJob{Config: tt.config}
			actions := job.Actions()
			if got := actions.HasActions(); got != tt.expected {
				t.Errorf("HasActions() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenericAgentActions_DynamicFieldValues(t *testing.T) {
	job := &GenericAgentJob{
		Config: map[string]string{
			"DynamicField_Category":   "Support",
			"DynamicField_Priority":   "High",
			"DynamicField_Resolution": "Fixed",
			"NewStateID":              "3", // Not a dynamic field
		},
	}

	actions := job.Actions()
	got := actions.DynamicFieldValues()

	expected := map[string]string{
		"Category":   "Support",
		"Priority":   "High",
		"Resolution": "Fixed",
	}

	if len(got) != len(expected) {
		t.Errorf("DynamicFieldValues() = %v, want %v", got, expected)
		return
	}
	for k, v := range expected {
		if got[k] != v {
			t.Errorf("DynamicFieldValues()[%s] = %v, want %v", k, got[k], v)
		}
	}
}

func TestGenericAgentJob_ScheduleLastRun(t *testing.T) {
	expected := time.Date(2025, 1, 10, 14, 30, 0, 0, time.UTC)

	t.Run("from LastRunAt field", func(t *testing.T) {
		job := &GenericAgentJob{
			LastRunAt: &expected,
			Config:    map[string]string{},
		}
		got := job.ScheduleLastRun()
		if got == nil || !got.Equal(expected) {
			t.Errorf("ScheduleLastRun() = %v, want %v", got, expected)
		}
	})

	t.Run("from config", func(t *testing.T) {
		job := &GenericAgentJob{
			Config: map[string]string{
				"ScheduleLastRun": expected.Format(time.RFC3339),
			},
		}
		got := job.ScheduleLastRun()
		if got == nil || !got.Equal(expected) {
			t.Errorf("ScheduleLastRun() = %v, want %v", got, expected)
		}
	})

	t.Run("not set", func(t *testing.T) {
		job := &GenericAgentJob{Config: map[string]string{}}
		got := job.ScheduleLastRun()
		if got != nil {
			t.Errorf("ScheduleLastRun() = %v, want nil", got)
		}
	})
}
