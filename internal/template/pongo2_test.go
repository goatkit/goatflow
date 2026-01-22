package template

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TemplateTestHelper provides utilities for testing pongo2 templates.
type TemplateTestHelper struct {
	TemplateDir string
	TemplateSet *pongo2.TemplateSet
}

// NewTemplateTestHelper creates a helper for template testing.
func NewTemplateTestHelper(t *testing.T) *TemplateTestHelper {
	// Find template directory relative to project root
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Walk up to find templates directory
	templateDir := findTemplateDir(cwd)
	require.NotEmpty(t, templateDir, "Could not find templates directory")

	// Create a template set with the proper base directory
	loader := pongo2.MustNewLocalFileSystemLoader(templateDir)
	templateSet := pongo2.NewSet("test", loader)

	return &TemplateTestHelper{
		TemplateDir: templateDir,
		TemplateSet: templateSet,
	}
}

func findTemplateDir(startDir string) string {
	dir := startDir
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "templates")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// RenderTemplate renders a template with the given context.
func (h *TemplateTestHelper) RenderTemplate(templatePath string, ctx pongo2.Context) (string, error) {
	tmpl, err := h.TemplateSet.FromFile(templatePath)
	if err != nil {
		return "", err
	}
	return tmpl.Execute(ctx)
}

// RenderAndValidate renders a template and validates the HTML structure.
// This ensures all HTML tags are properly balanced and nested.
func (h *TemplateTestHelper) RenderAndValidate(t *testing.T, templatePath string, ctx pongo2.Context) string {
	t.Helper()

	// Render the template
	html, err := h.RenderTemplate(templatePath, ctx)
	require.NoError(t, err, "template render failed for %s", templatePath)
	require.NotEmpty(t, html, "template produced empty output for %s", templatePath)

	// Validate HTML structure
	if err := ValidateHTML(html); err != nil {
		t.Errorf("HTML structure validation failed for %s: %v", templatePath, err)
	}

	return html
}

// HTMLAsserter provides assertions on rendered HTML.
type HTMLAsserter struct {
	t    *testing.T
	html string
}

// NewHTMLAsserter creates an asserter for HTML content.
func NewHTMLAsserter(t *testing.T, html string) *HTMLAsserter {
	return &HTMLAsserter{t: t, html: html}
}

// HasAttribute checks if an element with selector has the given attribute value.
func (a *HTMLAsserter) HasAttribute(elementPattern, attrName, expectedValue string) {
	// Build regex to find element and extract attribute
	// This is a simple implementation - for complex cases use goquery
	pattern := regexp.MustCompile(elementPattern + `[^>]*` + attrName + `="([^"]*)"`)
	matches := pattern.FindStringSubmatch(a.html)
	if len(matches) < 2 {
		a.t.Errorf("Element matching '%s' with attribute '%s' not found", elementPattern, attrName)
		return
	}
	assert.Equal(a.t, expectedValue, matches[1], "Attribute %s value mismatch", attrName)
}

// ContainsAttribute checks if any element has the attribute with value.
func (a *HTMLAsserter) ContainsAttribute(attrName, expectedValue string) bool {
	pattern := regexp.MustCompile(attrName + `="` + regexp.QuoteMeta(expectedValue) + `"`)
	return pattern.MatchString(a.html)
}

// HasHTMXPost checks for hx-post attribute with given URL.
func (a *HTMLAsserter) HasHTMXPost(url string) {
	if !a.ContainsAttribute("hx-post", url) {
		a.t.Errorf("Expected hx-post=\"%s\" not found in HTML", url)
	}
}

// HasHTMXPut checks for hx-put attribute with given URL.
func (a *HTMLAsserter) HasHTMXPut(url string) {
	if !a.ContainsAttribute("hx-put", url) {
		a.t.Errorf("Expected hx-put=\"%s\" not found in HTML", url)
	}
}

// HasNoHTMXPost checks that hx-post is NOT present.
func (a *HTMLAsserter) HasNoHTMXPost() {
	if strings.Contains(a.html, "hx-post=") {
		a.t.Error("Expected no hx-post attribute but found one")
	}
}

// HasNoHTMXPut checks that hx-put is NOT present.
func (a *HTMLAsserter) HasNoHTMXPut() {
	if strings.Contains(a.html, "hx-put=") {
		a.t.Error("Expected no hx-put attribute but found one")
	}
}

// HasFormAction checks form action attribute.
func (a *HTMLAsserter) HasFormAction(url string) {
	if !a.ContainsAttribute("action", url) {
		a.t.Errorf("Expected form action=\"%s\" not found", url)
	}
}

// Contains checks if HTML contains a string.
func (a *HTMLAsserter) Contains(expected string) {
	assert.Contains(a.t, a.html, expected)
}

// ContainsAny checks if HTML contains at least one of the given strings.
func (a *HTMLAsserter) ContainsAny(options ...string) {
	for _, opt := range options {
		if strings.Contains(a.html, opt) {
			return
		}
	}
	assert.Fail(a.t, "HTML should contain at least one of the expected strings", "Expected one of: %v", options)
}

// NotContains checks if HTML does not contain a string.
func (a *HTMLAsserter) NotContains(unexpected string) {
	assert.NotContains(a.t, a.html, unexpected)
}

// TestCustomerInitialsInBaseTemplate tests that customer initials render correctly
// in the base template navigation. This test ensures that when a Customer object
// is provided with proper initials, those initials are used rather than deriving
// them from an incomplete User object.
func TestCustomerInitialsInBaseTemplate(t *testing.T) {
	helper := NewTemplateTestHelper(t)

	t.Run("customer initials display ES for Emma Scott", func(t *testing.T) {
		// Simulate a customer context with properly set Customer.initials
		// The key is that we DON'T set User, so the template falls through to Customer
		ctx := pongo2.Context{
			"Customer": map[string]string{
				"first_name": "Emma",
				"last_name":  "Scott",
				"email":      "emma.scott@example.com",
				"initials":   "ES",
			},
			"Lang":       "en",
			"Direction":  "ltr",
			"IsRTL":      false,
			"profileHref": "/customer/profile",
			"logoutHref":  "/customer/logout",
		}

		html, err := helper.RenderTemplate("layouts/base.pongo2", ctx)
		require.NoError(t, err)

		// The initials should be "ES", not "E" or "?"
		// Check for the initials span with two letters
		assert.Contains(t, html, ">ES</span>", "Customer initials should be 'ES' (two letters)")
		assert.NotContains(t, html, ">E</span>", "Should not show single letter 'E'")
	})

	t.Run("customer initials NOT overridden by incomplete User object", func(t *testing.T) {
		// This simulates the bug: User is set with only FirstName (from email parsing)
		// but Customer has the correct initials. Template should prefer Customer.initials
		// when in a customer context.
		ctx := pongo2.Context{
			// User object that might be auto-injected with incomplete data
			// (FirstName from email, no LastName)
			"User": map[string]interface{}{
				"FirstName": "emma.scott@example.com", // Wrong: email used as name
				"LastName":  "",                        // Empty because email has no space
			},
			"Customer": map[string]string{
				"first_name": "Emma",
				"last_name":  "Scott",
				"email":      "emma.scott@example.com",
				"initials":   "ES",
			},
			"Lang":       "en",
			"Direction":  "ltr",
			"IsRTL":      false,
			"profileHref": "/customer/profile",
			"logoutHref":  "/customer/logout",
		}

		html, err := helper.RenderTemplate("layouts/base.pongo2", ctx)
		require.NoError(t, err)

		// BUG: Currently this will show "E" because User takes precedence
		// EXPECTED: Should show "ES" from Customer.initials
		// This test documents the expected behavior after the fix

		// For now, we check what the current (buggy) behavior produces
		// After fixing, this assertion should pass:
		assert.Contains(t, html, ">ES</span>", "Should show Customer initials 'ES', not User-derived single letter")
	})
}
