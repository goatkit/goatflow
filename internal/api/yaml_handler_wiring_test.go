package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type yamlRouteFile struct {
	Spec struct {
		Routes []struct {
			Path     string            `yaml:"path"`
			Method   string            `yaml:"method"`
			Handler  string            `yaml:"handler"`
			Handlers map[string]string `yaml:"handlers"`
		} `yaml:"routes"`
	} `yaml:"spec"`
}

func TestAllYAMLHandlersResolveThroughAPIRegistry(t *testing.T) {
	routesDir := findAPIRoutesDir(t)
	if routesDir == "" {
		t.Skip("could not find routes directory")
	}

	yamlHandlers := collectAPIYAMLHandlers(t, routesDir)
	require.NotEmpty(t, yamlHandlers, "should find handlers in YAML files")

	resolver := NewRoutingHandlerResolver()

	var missing []string
	for handler := range yamlHandlers {
		if !resolver.HandlerExists(handler) {
			missing = append(missing, handler)
		}
	}

	require.Emptyf(t, missing, "YAML handlers missing from API registry:\n  %s", strings.Join(missing, "\n  "))
}

func findAPIRoutesDir(t *testing.T) string {
	t.Helper()

	candidates := []string{
		"../../routes",
		"../../../routes",
		"routes",
		"/app/routes",
	}
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

func collectAPIYAMLHandlers(t *testing.T, routesDir string) map[string]bool {
	t.Helper()

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
			t.Logf("warning: could not read %s: %v", path, err)
			return nil
		}

		var routeFile yamlRouteFile
		if err := yaml.Unmarshal(data, &routeFile); err != nil {
			t.Logf("warning: could not parse %s: %v", path, err)
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
	require.NoError(t, err)

	return handlers
}
