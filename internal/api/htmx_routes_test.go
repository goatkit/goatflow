package api

import (
	"os"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/stretchr/testify/assert"
)

func TestTemplateLoading(t *testing.T) {
	// Skip template loading tests if templates directory doesn't exist
	if _, err := os.Stat("../../templates"); os.IsNotExist(err) {
		t.Skip("Templates directory not found, skipping template loading tests")
	}

	// Test pongo2 template loading (the actual templating system used)
	tests := []struct {
		name        string
		file        string
		expectError bool
	}{
		{
			name:        "Load single template",
			file:        "layouts/base.pongo2",
			expectError: false,
		},
		{
			name:        "Load multiple templates",
			file:        "pages/dashboard.pongo2",
			expectError: false,
		},
		{
			name:        "Load non-existent template",
			file:        "nonexistent.pongo2",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)
			os.Chdir("../..")

			loader := pongo2.MustNewLocalFileSystemLoader("templates")
			set := pongo2.NewSet("test", loader)
			tmpl, err := set.FromFile(tt.file)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, tmpl)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tmpl)
			}
		})
	}
}
