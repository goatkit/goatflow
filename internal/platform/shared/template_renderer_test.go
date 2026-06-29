package shared

import (
	"path/filepath"
	"testing"

	"github.com/flosch/pongo2/v6"
)

func TestCountrySelectTemplateExecutesWithFallbackCountries(t *testing.T) {
	templateDir := filepath.Join("..", "..", "..", "templates")
	renderer, err := NewTemplateRenderer(templateDir)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	tmpl, err := renderer.templateSet.FromFile("partials/forms/country_select.pongo2")
	if err != nil {
		t.Fatalf("failed to load country select template: %v", err)
	}

	ctx := pongo2.Context{
		"Countries":        []string{"Canada", "United States"},
		"selected_country": "Canada",
	}

	if _, err := tmpl.Execute(ctx); err != nil {
		t.Fatalf("country select template failed to render: %v", err)
	}
}
