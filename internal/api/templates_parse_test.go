//go:build db

package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"
)

// TestPongo2TemplatesParse walks the templates directory and ensures all .pongo2 files parse.
func TestPongo2TemplatesParse(t *testing.T) {
	// Try to locate the templates directory relative to this package
	candidates := []string{"../../templates", "../templates", "templates"}
	var templatesDir string
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			templatesDir = c
			break
		}
	}
	if templatesDir == "" {
		t.Fatal("templates directory not found from internal/api; tried ../../templates, ../templates, templates")
	}

	loader, lerr := pongo2.NewLocalFileSystemLoader(templatesDir)
	if lerr != nil {
		t.Fatalf("failed to create template loader for %s: %v", templatesDir, lerr)
	}
	set := pongo2.NewSet("test-templates", loader)

	var failures []string
	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".pongo2") {
			return nil
		}

		rel, rerr := filepath.Rel(templatesDir, path)
		if rerr != nil {
			failures = append(failures, path+": relpath error: "+rerr.Error())
			return nil
		}
		if _, perr := set.FromFile(rel); perr != nil {
			failures = append(failures, rel+": "+perr.Error())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("error walking templates: %v", err)
	}
	if len(failures) > 0 {
		for _, f := range failures {
			t.Errorf("template parse error: %s", f)
		}
		t.Fatalf("%d template(s) failed to parse", len(failures))
	}
}

// TestCustomerCompanyFormTemplateRendering specifically tests the customer company form template
// with realistic data to ensure it renders correctly and all default filters work properly.
func TestCustomerCompanyFormTemplateRendering(t *testing.T) {
	// Try to locate the templates directory relative to this package
	candidates := []string{"../../templates", "../templates", "templates"}
	var templatesDir string
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			templatesDir = c
			break
		}
	}
	if templatesDir == "" {
		t.Fatal("templates directory not found from internal/api; tried ../../templates, ../templates, templates")
	}

	loader, lerr := pongo2.NewLocalFileSystemLoader(templatesDir)
	if lerr != nil {
		t.Fatalf("failed to create template loader for %s: %v", templatesDir, lerr)
	}
	set := pongo2.NewSet("test-templates", loader)

	// Test data that matches what the handler would provide
	testData := pongo2.Context{
		"Title":           "Customer Companies",
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-companies",
		"IsNew":           true,
		"Company": pongo2.Context{
			"customer_id": "TEST001",
			"name":        "Test Company Ltd",
			"street":      "123 Test Street",
			"zip":         "12345",
			"city":        "Test City",
			"country":     "Test Country",
			"url":         "https://testcompany.com",
			"comments":    "Test comments",
		},
		"PortalConfig": pongo2.Context{
			"logo_url":        "",
			"custom_domain":   "",
			"primary_color":   "#1e40af",
			"secondary_color": "#64748b",
			"header_bg":       "#ffffff",
			"welcome_message": "",
			"footer_text":     "",
			"custom_css":      "",
		},
	}

	// Test rendering the customer company form template
	tmpl, err := set.FromFile("pages/admin/customer_company_form.pongo2")
	if err != nil {
		t.Fatalf("failed to load customer_company_form.pongo2: %v", err)
	}

	// Render the template with test data
	var result strings.Builder
	err = tmpl.ExecuteWriter(testData, &result)
	if err != nil {
		t.Fatalf("failed to render customer_company_form.pongo2: %v", err)
	}

	rendered := result.String()

	// Verify key elements are present in the rendered output
	if !strings.Contains(rendered, "Customer Companies") {
		t.Error("rendered template missing 'Customer Companies' title")
	}
	if !strings.Contains(rendered, "Create New Customer Company") {
		t.Error("rendered template missing 'Create New Customer Company' heading")
	}
	if !strings.Contains(rendered, "Customer ID") {
		t.Error("rendered template missing 'Customer ID' label")
	}
	if !strings.Contains(rendered, "Company Name") {
		t.Error("rendered template missing 'Company Name' label")
	}

	// Test with empty/nil values to ensure default filters work
	emptyData := pongo2.Context{
		"Title":           "Customer Companies",
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-companies",
		"IsNew":           true,
		"Company": pongo2.Context{
			"customer_id": "",
			"name":        "",
			"street":      "",
			"zip":         "",
			"city":        "",
			"country":     "",
			"url":         "",
			"comments":    "",
		},
		"PortalConfig": pongo2.Context{
			"logo_url":        "",
			"custom_domain":   "",
			"primary_color":   "",
			"secondary_color": "",
			"header_bg":       "",
			"welcome_message": "",
			"footer_text":     "",
			"custom_css":      "",
		},
	}

	var emptyResult strings.Builder
	err = tmpl.ExecuteWriter(emptyData, &emptyResult)
	if err != nil {
		t.Fatalf("failed to render customer_company_form.pongo2 with empty values: %v", err)
	}

	emptyRendered := emptyResult.String()

	// Should still render without errors even with empty values
	if emptyRendered == "" {
		t.Error("template rendered empty result with empty data")
	}
}
