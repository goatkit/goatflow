package routing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Import api package to trigger init() registrations.
	_ "github.com/goatkit/goatflow/internal/api"
	"github.com/goatkit/goatflow/internal/routing"

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

// TestAllYAMLHandlersAreRegistered ensures every handler referenced in YAML routes
// is registered in the global handler registry via init().
//
// When adding new handlers:
//  1. Create the handler function in internal/api/
//  2. Create an init() function that calls routing.RegisterHandler("handlerName", HandlerFunc)
//     (see internal/api/admin_sla_init.go for example)
func TestAllYAMLHandlersAreRegistered(t *testing.T) {
	// Find routes directory
	routesDir := findRoutesDir(t)
	if routesDir == "" {
		t.Skip("Could not find routes directory")
	}

	// Collect all handler names from YAML files
	yamlHandlers := collectYAMLHandlers(t, routesDir)
	require.NotEmpty(t, yamlHandlers, "Should find handlers in YAML files")

	// Get all registered handlers from GlobalHandlerMap (populated via init())
	registeredHandlers := routing.GlobalHandlerMap

	t.Logf("Found %d handlers in YAML, %d registered via init()", len(yamlHandlers), len(registeredHandlers))

	// Check each YAML handler is registered
	var missing []string
	for handler := range yamlHandlers {
		if _, ok := registeredHandlers[handler]; !ok {
			missing = append(missing, handler)
		}
	}

	if len(missing) > 0 {
		t.Errorf("The following handlers are referenced in YAML but not registered via init():\n  %s\n\n"+
			"To fix: Add routing.RegisterHandler(%q, YourHandler) in an init() function\n"+
			"See internal/api/admin_sla_init.go for example",
			strings.Join(missing, "\n  "), missing[0])
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
