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

func TestCannedResponseCategory_Fields(t *testing.T) {
	parentID := uint(1)
	cat := CannedResponseCategory{
		ID:          2,
		Name:        "Technical",
		Description: "Technical responses",
		Icon:        "wrench",
		Order:       1,
		ParentID:    &parentID,
		Active:      true,
	}

	if cat.ID != 2 {
		t.Errorf("ID = %d, want 2", cat.ID)
	}
	if cat.Name != "Technical" {
		t.Errorf("Name = %q, want %q", cat.Name, "Technical")
	}
	if cat.Icon != "wrench" {
		t.Errorf("Icon = %q, want %q", cat.Icon, "wrench")
	}
	if cat.ParentID == nil || *cat.ParentID != 1 {
		t.Error("ParentID should be 1")
	}
	if !cat.Active {
		t.Error("Active should be true")
	}
}

func TestCannedResponseCategory_NoParent(t *testing.T) {
	cat := CannedResponseCategory{
		ID:       1,
		Name:     "Root",
		ParentID: nil,
		Active:   true,
	}

	if cat.ParentID != nil {
		t.Error("ParentID should be nil for root category")
	}
}

func TestCannedResponseUsage_Fields(t *testing.T) {
	usage := CannedResponseUsage{
		ID:             1,
		ResponseID:     10,
		TicketID:       100,
		UserID:         5,
		UsedAt:         time.Now(),
		ModifiedBefore: true,
	}

	if usage.ResponseID != 10 {
		t.Errorf("ResponseID = %d, want 10", usage.ResponseID)
	}
	if usage.TicketID != 100 {
		t.Errorf("TicketID = %d, want 100", usage.TicketID)
	}
	if !usage.ModifiedBefore {
		t.Error("ModifiedBefore should be true")
	}
}

func TestCannedResponseFilter_Fields(t *testing.T) {
	filter := CannedResponseFilter{
		Query:      "greeting",
		Category:   "General",
		Tags:       []string{"welcome"},
		QueueID:    1,
		OnlyPublic: true,
		OnlyOwned:  false,
		UserID:     5,
		Limit:      10,
		Offset:     0,
	}

	if filter.Query != "greeting" {
		t.Errorf("Query = %q, want %q", filter.Query, "greeting")
	}
	if filter.Category != "General" {
		t.Errorf("Category = %q, want %q", filter.Category, "General")
	}
	if !filter.OnlyPublic {
		t.Error("OnlyPublic should be true")
	}
	if filter.OnlyOwned {
		t.Error("OnlyOwned should be false")
	}
	if filter.Limit != 10 {
		t.Errorf("Limit = %d, want 10", filter.Limit)
	}
}

func TestCannedResponseApplication_Fields(t *testing.T) {
	app := CannedResponseApplication{
		ResponseID: 1,
		TicketID:   100,
		Variables: map[string]string{
			"agent_name": "John",
			"ticket_id":  "12345",
		},
		AsInternal: false,
	}

	if app.ResponseID != 1 {
		t.Errorf("ResponseID = %d, want 1", app.ResponseID)
	}
	if app.TicketID != 100 {
		t.Errorf("TicketID = %d, want 100", app.TicketID)
	}
	if len(app.Variables) != 2 {
		t.Errorf("Variables len = %d, want 2", len(app.Variables))
	}
	if app.Variables["agent_name"] != "John" {
		t.Errorf("Variables[agent_name] = %q, want %q", app.Variables["agent_name"], "John")
	}
	if app.AsInternal {
		t.Error("AsInternal should be false")
	}
}

func TestAppliedResponse_Fields(t *testing.T) {
	ar := AppliedResponse{
		Subject:     "Re: Your ticket",
		Content:     "Hello John, thank you for contacting us.",
		ContentType: "text/html",
		Attachments: []string{"file1.pdf", "file2.pdf"},
		AsInternal:  false,
	}

	if ar.Subject != "Re: Your ticket" {
		t.Errorf("Subject = %q, want %q", ar.Subject, "Re: Your ticket")
	}
	if ar.ContentType != "text/html" {
		t.Errorf("ContentType = %q, want %q", ar.ContentType, "text/html")
	}
	if len(ar.Attachments) != 2 {
		t.Errorf("Attachments len = %d, want 2", len(ar.Attachments))
	}
	if ar.AsInternal {
		t.Error("AsInternal should be false")
	}
}
