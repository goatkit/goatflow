package models

import (
	"testing"
	"time"
)

func TestLookupItem_Fields(t *testing.T) {
	item := LookupItem{
		ID:     1,
		Value:  "high",
		Label:  "High Priority",
		Order:  1,
		Active: true,
	}

	if item.ID != 1 {
		t.Errorf("ID = %d, want 1", item.ID)
	}
	if item.Value != "high" {
		t.Errorf("Value = %q, want %q", item.Value, "high")
	}
	if item.Label != "High Priority" {
		t.Errorf("Label = %q, want %q", item.Label, "High Priority")
	}
	if item.Order != 1 {
		t.Errorf("Order = %d, want 1", item.Order)
	}
	if !item.Active {
		t.Error("Active should be true")
	}
}

func TestQueueInfo_Fields(t *testing.T) {
	queue := QueueInfo{
		ID:          1,
		Name:        "Support",
		Description: "General support queue",
		Active:      true,
	}

	if queue.ID != 1 {
		t.Errorf("ID = %d, want 1", queue.ID)
	}
	if queue.Name != "Support" {
		t.Errorf("Name = %q, want %q", queue.Name, "Support")
	}
	if queue.Description != "General support queue" {
		t.Errorf("Description = %q, want %q", queue.Description, "General support queue")
	}
	if !queue.Active {
		t.Error("Active should be true")
	}
}

func TestTicketFormData_Fields(t *testing.T) {
	formData := TicketFormData{
		Queues: []QueueInfo{
			{ID: 1, Name: "Support", Active: true},
			{ID: 2, Name: "Sales", Active: true},
		},
		Priorities: []LookupItem{
			{ID: 1, Value: "low", Label: "Low"},
			{ID: 2, Value: "high", Label: "High"},
		},
		Types: []LookupItem{
			{ID: 1, Value: "incident", Label: "Incident"},
		},
		Statuses: []LookupItem{
			{ID: 1, Value: "open", Label: "Open"},
			{ID: 2, Value: "closed", Label: "Closed"},
		},
	}

	if len(formData.Queues) != 2 {
		t.Errorf("Queues len = %d, want 2", len(formData.Queues))
	}
	if len(formData.Priorities) != 2 {
		t.Errorf("Priorities len = %d, want 2", len(formData.Priorities))
	}
	if len(formData.Types) != 1 {
		t.Errorf("Types len = %d, want 1", len(formData.Types))
	}
	if len(formData.Statuses) != 2 {
		t.Errorf("Statuses len = %d, want 2", len(formData.Statuses))
	}
}

func TestCannedResponse_Fields(t *testing.T) {
	now := time.Now()
	cr := CannedResponse{
		ID:          1,
		Name:        "Greeting",
		Shortcut:    "/greet",
		Category:    "General",
		Subject:     "Welcome",
		Content:     "Hello, how can I help?",
		ContentType: "text/plain",
		Tags:        []string{"welcome", "greeting"},
		IsPublic:    true,
		IsActive:    true,
		UsageCount:  42,
		CreatedBy:   1,
		UpdatedBy:   2,
		CreatedAt:   now,
		UpdatedAt:   now,
		OwnerID:     1,
		SharedWith:  []uint{2, 3},
		QueueIDs:    []uint{1},
	}

	if cr.ID != 1 {
		t.Errorf("ID = %d, want 1", cr.ID)
	}
	if cr.Name != "Greeting" {
		t.Errorf("Name = %q, want %q", cr.Name, "Greeting")
	}
	if cr.Shortcut != "/greet" {
		t.Errorf("Shortcut = %q, want %q", cr.Shortcut, "/greet")
	}
	if cr.Category != "General" {
		t.Errorf("Category = %q, want %q", cr.Category, "General")
	}
	if cr.Content != "Hello, how can I help?" {
		t.Errorf("Content = %q, want %q", cr.Content, "Hello, how can I help?")
	}
	if !cr.IsPublic {
		t.Error("IsPublic should be true")
	}
	if !cr.IsActive {
		t.Error("IsActive should be true")
	}
	if cr.UsageCount != 42 {
		t.Errorf("UsageCount = %d, want 42", cr.UsageCount)
	}
	if len(cr.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(cr.Tags))
	}
	if len(cr.SharedWith) != 2 {
		t.Errorf("SharedWith len = %d, want 2", len(cr.SharedWith))
	}
}

func TestResponseVariable_Fields(t *testing.T) {
	rv := ResponseVariable{
		Name:         "{{agent_name}}",
		Description:  "Name of the current agent",
		Type:         "text",
		Options:      []string{},
		DefaultValue: "",
		AutoFill:     "agent_name",
	}

	if rv.Name != "{{agent_name}}" {
		t.Errorf("Name = %q, want %q", rv.Name, "{{agent_name}}")
	}
	if rv.Type != "text" {
		t.Errorf("Type = %q, want %q", rv.Type, "text")
	}
	if rv.AutoFill != "agent_name" {
		t.Errorf("AutoFill = %q, want %q", rv.AutoFill, "agent_name")
	}
}

func TestResponseVariable_SelectType(t *testing.T) {
	rv := ResponseVariable{
		Name:        "{{status}}",
		Description: "Ticket status",
		Type:        "select",
		Options:     []string{"Open", "Pending", "Closed"},
	}

	if rv.Type != "select" {
		t.Errorf("Type = %q, want %q", rv.Type, "select")
	}
	if len(rv.Options) != 3 {
		t.Errorf("Options len = %d, want 3", len(rv.Options))
	}
}
