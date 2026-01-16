package api

import (
	"testing"
)

func TestQueueWithTemplateCountStruct(t *testing.T) {
	q := QueueWithTemplateCount{
		ID:            1,
		Name:          "Test Queue",
		TemplateCount: 5,
	}

	if q.ID != 1 {
		t.Errorf("Expected ID 1, got %d", q.ID)
	}
	if q.Name != "Test Queue" {
		t.Errorf("Expected Name 'Test Queue', got %s", q.Name)
	}
	if q.TemplateCount != 5 {
		t.Errorf("Expected TemplateCount 5, got %d", q.TemplateCount)
	}
}

func TestTemplateWithQueueCountStruct(t *testing.T) {
	tmpl := TemplateWithQueueCount{
		ID:           1,
		Name:         "Test Template",
		TemplateType: "Answer,Note",
		QueueCount:   3,
	}

	if tmpl.ID != 1 {
		t.Errorf("Expected ID 1, got %d", tmpl.ID)
	}
	if tmpl.Name != "Test Template" {
		t.Errorf("Expected Name 'Test Template', got %s", tmpl.Name)
	}
	if tmpl.TemplateType != "Answer,Note" {
		t.Errorf("Expected TemplateType 'Answer,Note', got %s", tmpl.TemplateType)
	}
	if tmpl.QueueCount != 3 {
		t.Errorf("Expected QueueCount 3, got %d", tmpl.QueueCount)
	}
}

func TestQueueInfoStruct(t *testing.T) {
	q := QueueInfo{
		ID:   42,
		Name: "Support Queue",
	}

	if q.ID != 42 {
		t.Errorf("Expected ID 42, got %d", q.ID)
	}
	if q.Name != "Support Queue" {
		t.Errorf("Expected Name 'Support Queue', got %s", q.Name)
	}
}

func TestTemplateOptionStruct(t *testing.T) {
	tests := []struct {
		name     string
		opt      TemplateOption
		expected TemplateOption
	}{
		{
			name: "unselected template",
			opt: TemplateOption{
				ID:           1,
				Name:         "Welcome Email",
				TemplateType: "Answer",
				Selected:     false,
			},
			expected: TemplateOption{
				ID:           1,
				Name:         "Welcome Email",
				TemplateType: "Answer",
				Selected:     false,
			},
		},
		{
			name: "selected template",
			opt: TemplateOption{
				ID:           2,
				Name:         "Ticket Closed",
				TemplateType: "Note",
				Selected:     true,
			},
			expected: TemplateOption{
				ID:           2,
				Name:         "Ticket Closed",
				TemplateType: "Note",
				Selected:     true,
			},
		},
		{
			name: "multi-type template",
			opt: TemplateOption{
				ID:           3,
				Name:         "Multi-Purpose",
				TemplateType: "Answer,Note,Snippet",
				Selected:     true,
			},
			expected: TemplateOption{
				ID:           3,
				Name:         "Multi-Purpose",
				TemplateType: "Answer,Note,Snippet",
				Selected:     true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.opt.ID != tc.expected.ID {
				t.Errorf("Expected ID %d, got %d", tc.expected.ID, tc.opt.ID)
			}
			if tc.opt.Name != tc.expected.Name {
				t.Errorf("Expected Name %s, got %s", tc.expected.Name, tc.opt.Name)
			}
			if tc.opt.TemplateType != tc.expected.TemplateType {
				t.Errorf("Expected TemplateType %s, got %s", tc.expected.TemplateType, tc.opt.TemplateType)
			}
			if tc.opt.Selected != tc.expected.Selected {
				t.Errorf("Expected Selected %v, got %v", tc.expected.Selected, tc.opt.Selected)
			}
		})
	}
}

func TestQueueWithTemplateCountZeroCount(t *testing.T) {
	q := QueueWithTemplateCount{
		ID:            1,
		Name:          "Empty Queue",
		TemplateCount: 0,
	}

	if q.TemplateCount != 0 {
		t.Errorf("Expected TemplateCount 0, got %d", q.TemplateCount)
	}
}

func TestTemplateWithQueueCountZeroCount(t *testing.T) {
	tmpl := TemplateWithQueueCount{
		ID:           1,
		Name:         "Orphan Template",
		TemplateType: "Answer",
		QueueCount:   0,
	}

	if tmpl.QueueCount != 0 {
		t.Errorf("Expected QueueCount 0, got %d", tmpl.QueueCount)
	}
}
