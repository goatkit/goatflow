package api

import (
	"testing"
)

func TestSubstituteTemplateVariables_OTRS(t *testing.T) {
	text := "Hello <OTRS_CUSTOMER_UserFirstname> <OTRS_CUSTOMER_UserLastname>!"
	vars := map[string]string{
		"CUSTOMER_UserFirstname": "John",
		"CUSTOMER_UserLastname":  "Doe",
	}

	result := SubstituteTemplateVariables(text, vars)

	expected := "Hello John Doe!"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteTemplateVariables_GoatFlow(t *testing.T) {
	text := "Ticket: <GOATFLOW_TICKET_TicketNumber> - <GOATFLOW_TICKET_Title>"
	vars := map[string]string{
		"TICKET_TicketNumber": "2025010112345678",
		"TICKET_Title":        "Test Issue",
	}

	result := SubstituteTemplateVariables(text, vars)

	expected := "Ticket: 2025010112345678 - Test Issue"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteTemplateVariables_Mixed(t *testing.T) {
	text := "Dear <OTRS_CUSTOMER_UserFullname>, Your ticket <GOATFLOW_TICKET_TicketNumber> has been updated."
	vars := map[string]string{
		"CUSTOMER_UserFullname": "Jane Smith",
		"TICKET_TicketNumber":   "2025010187654321",
	}

	result := SubstituteTemplateVariables(text, vars)

	expected := "Dear Jane Smith, Your ticket 2025010187654321 has been updated."
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteTemplateVariables_NoVariables(t *testing.T) {
	text := "This is plain text without any variables."
	vars := map[string]string{
		"CUSTOMER_UserFirstname": "John",
	}

	result := SubstituteTemplateVariables(text, vars)

	if result != text {
		t.Errorf("Expected %q, got %q", text, result)
	}
}

func TestSubstituteTemplateVariables_EmptyVars(t *testing.T) {
	text := "Hello <OTRS_CUSTOMER_UserFirstname>!"
	vars := map[string]string{}

	result := SubstituteTemplateVariables(text, vars)

	// OTRS behavior: unmatched variables are replaced with "-"
	expected := "Hello -!"
	if result != expected {
		t.Errorf("Expected %q (OTRS replaces unmatched vars with '-'), got %q", expected, result)
	}
}

func TestSubstituteTemplateVariables_EmptyText(t *testing.T) {
	text := ""
	vars := map[string]string{
		"CUSTOMER_UserFirstname": "John",
	}

	result := SubstituteTemplateVariables(text, vars)

	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestSubstituteTemplateVariables_UnknownVariable(t *testing.T) {
	text := "Hello <OTRS_UNKNOWN_VAR>!"
	vars := map[string]string{
		"CUSTOMER_UserFirstname": "John",
	}

	result := SubstituteTemplateVariables(text, vars)

	// OTRS behavior: unmatched variables are replaced with "-"
	expected := "Hello -!"
	if result != expected {
		t.Errorf("Unknown variables should be replaced with '-' (OTRS behavior), expected %q, got %q", expected, result)
	}
}

func TestSubstituteTemplateVariables_MultipleOccurrences(t *testing.T) {
	text := "<OTRS_CUSTOMER_UserFirstname> said: Hello <OTRS_CUSTOMER_UserFirstname>!"
	vars := map[string]string{
		"CUSTOMER_UserFirstname": "John",
	}

	result := SubstituteTemplateVariables(text, vars)

	expected := "John said: Hello John!"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteTemplateVariables_EmptyValue(t *testing.T) {
	text := "Name: <OTRS_CUSTOMER_UserFirstname> <OTRS_CUSTOMER_UserLastname>"
	vars := map[string]string{
		"CUSTOMER_UserFirstname": "",
		"CUSTOMER_UserLastname":  "Doe",
	}

	result := SubstituteTemplateVariables(text, vars)

	expected := "Name:  Doe"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestTemplateForAgentStruct(t *testing.T) {
	template := TemplateForAgent{
		ID:           1,
		Name:         "Test Template",
		Text:         "Hello World",
		ContentType:  "text/html",
		TemplateType: "Answer",
	}

	if template.ID != 1 {
		t.Errorf("Expected ID 1, got %d", template.ID)
	}
	if template.Name != "Test Template" {
		t.Errorf("Expected Name 'Test Template', got %s", template.Name)
	}
	if template.ContentType != "text/html" {
		t.Errorf("Expected ContentType 'text/html', got %s", template.ContentType)
	}
}

func TestAttachmentInfoStruct(t *testing.T) {
	attachment := AttachmentInfo{
		ID:          42,
		Name:        "Test Attachment",
		Filename:    "test.pdf",
		ContentType: "application/pdf",
		ContentSize: 1024,
	}

	if attachment.ID != 42 {
		t.Errorf("Expected ID 42, got %d", attachment.ID)
	}
	if attachment.Filename != "test.pdf" {
		t.Errorf("Expected Filename 'test.pdf', got %s", attachment.Filename)
	}
	if attachment.ContentSize != 1024 {
		t.Errorf("Expected ContentSize 1024, got %d", attachment.ContentSize)
	}
}

func TestGetAttachmentsByIDs_Empty(t *testing.T) {
	result := GetAttachmentsByIDs([]int{})

	if result == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %d items", len(result))
	}
}
