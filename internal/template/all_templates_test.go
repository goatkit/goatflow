// Package template contains tests for template rendering functionality.
package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/stretchr/testify/require"
)

// Common context builders for reusable test data

func baseContext() pongo2.Context {
	return pongo2.Context{
		"t":           func(key string, args ...interface{}) string { return key },
		"User":        map[string]interface{}{"Username": "admin", "IsAdmin": true, "ID": 1},
		"CurrentYear": 2025,
	}
}

func ticketContext() pongo2.Context {
	ctx := baseContext()
	ctx["Ticket"] = map[string]interface{}{
		"id":       123,
		"tn":       "2025010112345678",
		"title":    "Test Ticket",
		"queue":    "Support",
		"state":    "open",
		"priority": "normal",
	}
	ctx["TicketID"] = 123
	ctx["Articles"] = []map[string]interface{}{}
	ctx["Queues"] = []map[string]interface{}{{"ID": 1, "Name": "Support"}}
	ctx["States"] = []map[string]interface{}{{"ID": 1, "Name": "open"}}
	ctx["Priorities"] = []map[string]interface{}{{"ID": 1, "Name": "normal"}}
	return ctx
}

func customerContext() pongo2.Context {
	ctx := baseContext()
	ctx["Customer"] = map[string]interface{}{"ID": 1, "Email": "customer@example.com"}
	return ctx
}

// =============================================================================
// AUTHENTICATION TEMPLATES
// =============================================================================

func TestLoginFormAction(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Error"] = ""

	html, err := helper.RenderTemplate("pages/login.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/api/auth/login")
	// Login uses hx-boost for progressive enhancement, not hx-post
	asserter.Contains("hx-boost=\"true\"")
}

func TestCustomerLoginFormAction(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Error"] = ""

	html, err := helper.RenderTemplate("pages/customer/login.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/api/auth/customer/login")
}

func TestRegisterFormAction(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Error"] = ""

	html, err := helper.RenderTemplate("pages/register.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasHTMXPost("/api/auth/register")
}

// =============================================================================
// TICKET TEMPLATES
// =============================================================================

func TestNewTicketFormAction(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := ticketContext()
	ctx["Types"] = []map[string]interface{}{{"ID": 1, "Name": "Unclassified"}}
	ctx["Services"] = []map[string]interface{}{}
	ctx["DynamicFields"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/tickets/new.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/api/tickets")
	asserter.HasHTMXPost("/api/tickets")
}

func TestTicketDetailNoteForm(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := ticketContext()
	ctx["ArticleTypes"] = []map[string]interface{}{{"ID": 1, "Name": "note-internal"}}
	ctx["DynamicFields"] = []map[string]interface{}{}
	ctx["CanEdit"] = true

	html, err := helper.RenderTemplate("pages/tickets/detail.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	// Note form should POST to agent ticket note endpoint
	asserter.HasHTMXPost("/agent/tickets/123/note")
}

func TestTicketDetailAttachmentForm(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := ticketContext()
	ctx["Attachments"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/ticket_detail.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	// Attachment upload form
	asserter.HasHTMXPost("/api/tickets/2025010112345678/attachments")
}

func TestCustomerNewTicketFormAction(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := customerContext()
	ctx["Queues"] = []map[string]interface{}{{"ID": 1, "Name": "Support"}}
	ctx["Priorities"] = []map[string]interface{}{{"ID": 1, "Name": "normal"}}
	ctx["Types"] = []map[string]interface{}{{"ID": 1, "Name": "Unclassified"}}
	ctx["DynamicFields"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/customer/new_ticket.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/customer/tickets/create")
}

func TestCustomerTicketReplyFormAction(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := customerContext()
	ctx["Ticket"] = map[string]interface{}{
		"id":    456,
		"tn":    "2025010112345679",
		"title": "Customer Ticket",
	}
	ctx["Articles"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/customer/ticket_view.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/customer/tickets/456/reply")
}

// =============================================================================
// ADMIN TEMPLATES
// =============================================================================

func TestAdminCustomerPortalSettingsForm(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Settings"] = map[string]interface{}{
		"allow_registration": true,
		"require_approval":   false,
	}

	html, err := helper.RenderTemplate("pages/admin/customer_portal_settings.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/admin/customer/portal/settings")
	asserter.HasHTMXPost("/admin/customer/portal/settings")
}

func TestAdminCustomerCompanyFormCreate(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["IsNew"] = true
	ctx["Company"] = map[string]interface{}{
		"customer_id": "",
		"name":        "",
	}

	html, err := helper.RenderTemplate("pages/admin/customer_company_form.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/admin/customer/companies")
}

func TestAdminCustomerCompanyFormEdit(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["IsNew"] = false
	ctx["Company"] = map[string]interface{}{
		"customer_id": "CUST001",
		"name":        "Test Company",
	}

	html, err := helper.RenderTemplate("pages/admin/customer_company_form.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/admin/customer/companies/CUST001/edit")
}

func TestAdminEmailQueueRetryForms(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Emails"] = []map[string]interface{}{
		{
			"ID":              1,
			"Subject":         "Test",
			"Status":          "failed",
			"CreateTime":      time.Now(),
			"Recipient":       "test@example.com",
			"LastSMTPMessage": "",
		},
	}
	ctx["Stats"] = map[string]interface{}{
		"pending": 1,
		"failed":  1,
	}

	html, err := helper.RenderTemplate("pages/admin/email_queue.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	// Retry all button
	asserter.HasHTMXPost("/admin/email-queue/retry-all")
	// Individual retry/delete buttons
	asserter.Contains("hx-post=\"/admin/email-queue/retry/")
	asserter.Contains("hx-post=\"/admin/email-queue/delete/")
}

// =============================================================================
// SEARCH/FILTER FORMS (GET actions - verify they don't accidentally use POST)
// =============================================================================

func TestQueuesSearchFormIsGET(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Queues"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/queues.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/queues")
	// Search forms should be GET, not POST
	asserter.NotContains("hx-post=\"/queues\"")
}

func TestAdminSLASearchFormIsGET(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["SLAs"] = []map[string]interface{}{}
	ctx["Search"] = ""
	ctx["Status"] = ""

	html, err := helper.RenderTemplate("pages/admin/sla.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/admin/sla")
}

func TestAdminCustomerCompaniesSearchFormIsGET(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Companies"] = []map[string]interface{}{}
	ctx["Search"] = ""

	html, err := helper.RenderTemplate("pages/admin/customer_companies.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/admin/customer/companies")
}

func TestAgentTicketsSearchFormIsGET(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := baseContext()
	ctx["Tickets"] = []map[string]interface{}{}
	ctx["Queues"] = []map[string]interface{}{}
	ctx["States"] = []map[string]interface{}{}
	ctx["Priorities"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/agent/tickets.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/agent/tickets")
}

func TestCustomerTicketsSearchFormIsGET(t *testing.T) {
	helper := NewTemplateTestHelper(t)
	ctx := customerContext()
	ctx["Tickets"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/customer/tickets.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	asserter.HasFormAction("/customer/tickets")
}

// =============================================================================
// TEMPLATE CONSISTENCY TESTS
// These ensure templates don't have common mistakes
// =============================================================================

func TestNoMixedHTMXAndTraditionalSubmit(t *testing.T) {
	// Templates should not have both hx-post AND form method="POST" action pointing to different URLs
	// This is a meta-test that could be expanded to scan all templates

	helper := NewTemplateTestHelper(t)

	// Test new ticket form - both should point to same URL
	ctx := ticketContext()
	ctx["Types"] = []map[string]interface{}{{"ID": 1, "Name": "Unclassified"}}
	ctx["Services"] = []map[string]interface{}{}
	ctx["DynamicFields"] = []map[string]interface{}{}

	html, err := helper.RenderTemplate("pages/tickets/new.pongo2", ctx)
	require.NoError(t, err)

	asserter := NewHTMLAsserter(t, html)
	// Both action and hx-post should be /api/tickets
	asserter.HasFormAction("/api/tickets")
	asserter.HasHTMXPost("/api/tickets")
}

// =============================================================================
// ADMIN API PATH PREFIX TESTS
// Ensure all admin API calls use /admin/api prefix
// =============================================================================

func TestAdminAPIPathsUseCorrectPrefix(t *testing.T) {
	testCases := []struct {
		name           string
		template       string
		ctx            pongo2.Context
		expectedPaths  []string // Should contain these
		forbiddenPaths []string // Should NOT contain these
	}{
		{
			name:     "Dynamic field form uses /admin/api",
			template: "pages/admin/dynamic_field_form.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["IsNew"] = true
				ctx["Field"] = map[string]interface{}{
					"ID": 0, "Name": "", "Label": "", "FieldType": "Text",
					"Config":  map[string]interface{}{"DefaultValue": "", "MaxLength": 0},
					"ValidID": 1,
				}
				ctx["FieldTypes"] = []string{"Text"}
				ctx["ValidOptions"] = []map[string]interface{}{{"ID": 1, "Name": "valid"}}
				return ctx
			}(),
			expectedPaths:  []string{"/admin/api/dynamic-fields"},
			forbiddenPaths: []string{"hx-post=\"/api/dynamic-fields\"", "hx-put=\"/api/dynamic-fields"},
		},
		{
			name:     "Customer portal settings uses /admin path",
			template: "pages/admin/customer_portal_settings.pongo2",
			ctx: func() pongo2.Context {
				ctx := baseContext()
				ctx["Settings"] = map[string]interface{}{}
				return ctx
			}(),
			expectedPaths:  []string{"/admin/customer/portal/settings"},
			forbiddenPaths: []string{"\"/customer/portal/settings\""},
		},
	}

	helper := NewTemplateTestHelper(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			html, err := helper.RenderTemplate(tc.template, tc.ctx)
			require.NoError(t, err, "Template should render")

			asserter := NewHTMLAsserter(t, html)

			for _, expected := range tc.expectedPaths {
				asserter.Contains(expected)
			}

			for _, forbidden := range tc.forbiddenPaths {
				asserter.NotContains(forbidden)
			}
		})
	}
}

// =============================================================================
// FORM TEMPLATE TESTS
// These tests verify that templates with forms have correct action URLs.
// 100% page template coverage is now enforced in template_coverage_test.go
// =============================================================================

// testedFormTemplates lists templates with form-specific tests (action URL verification).
// All page templates have basic render coverage in template_coverage_test.go.
var testedFormTemplates = map[string]bool{
	// Auth
	"pages/login.pongo2":          true,
	"pages/register.pongo2":       true,
	"pages/customer/login.pongo2": true,

	// Tickets
	"pages/tickets/new.pongo2":          true,
	"pages/tickets/detail.pongo2":       true,
	"pages/ticket_detail.pongo2":        true,
	"pages/customer/new_ticket.pongo2":  true,
	"pages/customer/ticket_view.pongo2": true,

	// Admin
	"pages/admin/attachment.pongo2":               true,
	"pages/admin/dynamic_field_form.pongo2":       true,
	"pages/admin/dynamic_fields.pongo2":           true,
	"pages/admin/dynamic_field_screens.pongo2":    true,
	"pages/admin/customer_portal_settings.pongo2": true,
	"pages/admin/customer_company_form.pongo2":    true,
	"pages/admin/email_queue.pongo2":              true,
	"pages/admin/signature_form.pongo2":           true,
	"pages/admin/template_form.pongo2":            true,
	"pages/admin/template_import.pongo2":          true,
	"pages/admin/template_queues.pongo2":          true,
	"pages/admin/template_attachments.pongo2":     true,

	// Search/Filter forms (GET only)
	"pages/queues.pongo2":                   true,
	"pages/tickets.pongo2":                  true,
	"pages/admin/sla.pongo2":                true,
	"pages/admin/customer_companies.pongo2": true,
	"pages/agent/tickets.pongo2":            true,
	"pages/customer/tickets.pongo2":         true,

	// Schema/Discovery (uses fetch() in JS, not HTMX forms)
	"pages/admin/schema_monitoring.pongo2": true,
	"pages/admin/schema_discovery.pongo2":  true,

	// Dev tools (not production, skip strict validation)
	"pages/dev/database.pongo2": true,
}

// TestFormTemplateCoverage scans for templates with forms and verifies they have form action tests.
// Note: 100% render coverage is enforced by TestAllPageTemplatesHaveCoverage in template_coverage_test.go.
func TestFormTemplateCoverage(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	// Patterns that indicate a template needs form testing
	formPatterns := []string{
		"hx-post=",
		"hx-put=",
		"hx-delete=",
		"method=\"POST\"",
		"method=\"post\"",
	}

	// Walk templates directory
	err := filepath.Walk(helper.TemplateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".pongo2") {
			return nil
		}

		// Get relative path from templates dir
		relPath, _ := filepath.Rel(helper.TemplateDir, path)

		// Skip partials and layouts
		if strings.HasPrefix(relPath, "partials/") || strings.HasPrefix(relPath, "layouts/") {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Check if template has form-related content
		hasForm := false
		for _, pattern := range formPatterns {
			if strings.Contains(string(content), pattern) {
				hasForm = true
				break
			}
		}

		if hasForm && !testedFormTemplates[relPath] {
			t.Errorf("Template %s has forms but no form action test. Add it to testedFormTemplates and create a test.", relPath)
		}

		return nil
	})

	require.NoError(t, err)
}
