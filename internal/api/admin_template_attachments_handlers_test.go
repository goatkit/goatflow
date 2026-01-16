package api

import (
	"testing"
)

func TestTemplateWithAttachmentCountStruct(t *testing.T) {
	tmpl := TemplateWithAttachmentCount{
		ID:              1,
		Name:            "Test Template",
		TemplateType:    "Answer",
		AttachmentCount: 3,
	}

	if tmpl.ID != 1 {
		t.Errorf("Expected ID 1, got %d", tmpl.ID)
	}
	if tmpl.Name != "Test Template" {
		t.Errorf("Expected Name 'Test Template', got %s", tmpl.Name)
	}
	if tmpl.TemplateType != "Answer" {
		t.Errorf("Expected TemplateType 'Answer', got %s", tmpl.TemplateType)
	}
	if tmpl.AttachmentCount != 3 {
		t.Errorf("Expected AttachmentCount 3, got %d", tmpl.AttachmentCount)
	}
}

func TestAttachmentWithTemplateCountStruct(t *testing.T) {
	att := AttachmentWithTemplateCount{
		ID:            1,
		Name:          "Test Attachment",
		Filename:      "test.pdf",
		TemplateCount: 5,
	}

	if att.ID != 1 {
		t.Errorf("Expected ID 1, got %d", att.ID)
	}
	if att.Name != "Test Attachment" {
		t.Errorf("Expected Name 'Test Attachment', got %s", att.Name)
	}
	if att.Filename != "test.pdf" {
		t.Errorf("Expected Filename 'test.pdf', got %s", att.Filename)
	}
	if att.TemplateCount != 5 {
		t.Errorf("Expected TemplateCount 5, got %d", att.TemplateCount)
	}
}

func TestAttachmentBasicInfoStruct(t *testing.T) {
	att := AttachmentBasicInfo{
		ID:       42,
		Name:     "Company Logo",
		Filename: "logo.png",
	}

	if att.ID != 42 {
		t.Errorf("Expected ID 42, got %d", att.ID)
	}
	if att.Name != "Company Logo" {
		t.Errorf("Expected Name 'Company Logo', got %s", att.Name)
	}
	if att.Filename != "logo.png" {
		t.Errorf("Expected Filename 'logo.png', got %s", att.Filename)
	}
}

func TestTemplateWithAttachmentCountMultiType(t *testing.T) {
	tmpl := TemplateWithAttachmentCount{
		ID:              1,
		Name:            "Multi-Type Template",
		TemplateType:    "Answer,Note,Snippet",
		AttachmentCount: 2,
	}

	if tmpl.TemplateType != "Answer,Note,Snippet" {
		t.Errorf("Expected TemplateType 'Answer,Note,Snippet', got %s", tmpl.TemplateType)
	}
}

func TestAttachmentWithTemplateCountZeroCount(t *testing.T) {
	att := AttachmentWithTemplateCount{
		ID:            1,
		Name:          "Orphan Attachment",
		Filename:      "orphan.txt",
		TemplateCount: 0,
	}

	if att.TemplateCount != 0 {
		t.Errorf("Expected TemplateCount 0, got %d", att.TemplateCount)
	}
}

func TestTemplateWithAttachmentCountZeroCount(t *testing.T) {
	tmpl := TemplateWithAttachmentCount{
		ID:              1,
		Name:            "No Attachments",
		TemplateType:    "Answer",
		AttachmentCount: 0,
	}

	if tmpl.AttachmentCount != 0 {
		t.Errorf("Expected AttachmentCount 0, got %d", tmpl.AttachmentCount)
	}
}

func TestAttachmentBasicInfoVariousFileTypes(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"PDF file", "document.pdf"},
		{"Image file", "image.png"},
		{"Word doc", "report.docx"},
		{"Excel file", "data.xlsx"},
		{"Text file", "notes.txt"},
		{"Archive", "backup.zip"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			att := AttachmentBasicInfo{
				ID:       1,
				Name:     tc.name,
				Filename: tc.filename,
			}
			if att.Filename != tc.filename {
				t.Errorf("Expected Filename %s, got %s", tc.filename, att.Filename)
			}
		})
	}
}
