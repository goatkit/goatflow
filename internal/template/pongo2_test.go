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

// TemplateTestHelper provides utilities for testing pongo2 templates
type TemplateTestHelper struct {
	TemplateDir string
	TemplateSet *pongo2.TemplateSet
}

// NewTemplateTestHelper creates a helper for template testing
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

// RenderTemplate renders a template with the given context
func (h *TemplateTestHelper) RenderTemplate(templatePath string, ctx pongo2.Context) (string, error) {
	tmpl, err := h.TemplateSet.FromFile(templatePath)
	if err != nil {
		return "", err
	}
	return tmpl.Execute(ctx)
}

// HTMLAsserter provides assertions on rendered HTML
type HTMLAsserter struct {
	t    *testing.T
	html string
}

// NewHTMLAsserter creates an asserter for HTML content
func NewHTMLAsserter(t *testing.T, html string) *HTMLAsserter {
	return &HTMLAsserter{t: t, html: html}
}

// HasAttribute checks if an element with selector has the given attribute value
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

// ContainsAttribute checks if any element has the attribute with value
func (a *HTMLAsserter) ContainsAttribute(attrName, expectedValue string) bool {
	pattern := regexp.MustCompile(attrName + `="` + regexp.QuoteMeta(expectedValue) + `"`)
	return pattern.MatchString(a.html)
}

// HasHTMXPost checks for hx-post attribute with given URL
func (a *HTMLAsserter) HasHTMXPost(url string) {
	if !a.ContainsAttribute("hx-post", url) {
		a.t.Errorf("Expected hx-post=\"%s\" not found in HTML", url)
	}
}

// HasHTMXPut checks for hx-put attribute with given URL
func (a *HTMLAsserter) HasHTMXPut(url string) {
	if !a.ContainsAttribute("hx-put", url) {
		a.t.Errorf("Expected hx-put=\"%s\" not found in HTML", url)
	}
}

// HasNoHTMXPost checks that hx-post is NOT present
func (a *HTMLAsserter) HasNoHTMXPost() {
	if strings.Contains(a.html, "hx-post=") {
		a.t.Error("Expected no hx-post attribute but found one")
	}
}

// HasNoHTMXPut checks that hx-put is NOT present  
func (a *HTMLAsserter) HasNoHTMXPut() {
	if strings.Contains(a.html, "hx-put=") {
		a.t.Error("Expected no hx-put attribute but found one")
	}
}

// HasFormAction checks form action attribute
func (a *HTMLAsserter) HasFormAction(url string) {
	if !a.ContainsAttribute("action", url) {
		a.t.Errorf("Expected form action=\"%s\" not found", url)
	}
}

// Contains checks if HTML contains a string
func (a *HTMLAsserter) Contains(expected string) {
	assert.Contains(a.t, a.html, expected)
}

// NotContains checks if HTML does not contain a string
func (a *HTMLAsserter) NotContains(unexpected string) {
	assert.NotContains(a.t, a.html, unexpected)
}
