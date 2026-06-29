package routing_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/platform/routing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// RouteFile represents the structure of a YAML route file.
type RouteFile struct {
	Spec struct {
		Routes []struct {
			Path     string            `yaml:"path"`
			Method   string            `yaml:"method"`
			Handler  string            `yaml:"handler"`
			Handlers map[string]string `yaml:"handlers"`
		} `yaml:"routes"`
	} `yaml:"spec"`
}

// TestAllYAMLHandlersResolveWithMockResolver ensures every handler referenced in YAML routes
// can be represented by routing's HandlerResolver without importing product API code.
func TestAllYAMLHandlersResolveWithMockResolver(t *testing.T) {
	// Find routes directory
	routesDir := findRoutesDir(t)
	if routesDir == "" {
		t.Skip("Could not find routes directory")
	}

	// Collect all handler names from YAML files
	yamlHandlers := collectYAMLHandlers(t, routesDir)
	require.NotEmpty(t, yamlHandlers, "Should find handlers in YAML files")

	registry := routing.NewHandlerRegistry()
	for handler := range yamlHandlers {
		h := func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		}
		require.NoError(t, registry.Register(handler, h))
	}

	t.Logf("Found %d handlers in YAML and registered them in mock resolver", len(yamlHandlers))

	for handler := range yamlHandlers {
		require.True(t, registry.HandlerExists(handler), "handler %q should resolve through mock resolver", handler)
	}
}

func findRoutesDir(t *testing.T) string {
	// Try relative paths from test location
	candidates := []string{
		"../../routes",
		"../../../routes",
		"routes",
		"/app/routes",
	}

	// Also check ROUTES_DIR env var
	if dir := os.Getenv("ROUTES_DIR"); dir != "" {
		candidates = append([]string{dir}, candidates...)
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	return ""
}

func collectYAMLHandlers(t *testing.T, routesDir string) map[string]bool {
	handlers := make(map[string]bool)

	err := filepath.Walk(routesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Logf("Warning: Could not read %s: %v", path, err)
			return nil
		}

		var routeFile RouteFile
		if err := yaml.Unmarshal(data, &routeFile); err != nil {
			t.Logf("Warning: Could not parse %s: %v", path, err)
			return nil
		}

		for _, route := range routeFile.Spec.Routes {
			if route.Handler != "" {
				handlers[route.Handler] = true
			}
			for _, h := range route.Handlers {
				handlers[h] = true
			}
		}

		return nil
	})

	if err != nil {
		t.Logf("Warning: Error walking routes dir: %v", err)
	}

	return handlers
}
