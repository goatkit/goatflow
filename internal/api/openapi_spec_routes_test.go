package api

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var (
	openAPIParamPattern = regexp.MustCompile(`\{([^}]+)\}`)
	ginParamSegment     = regexp.MustCompile(`[:*][^/]+`)
	openAPIMethods      = []string{"get", "put", "post", "delete", "patch", "head", "options"}
)

type specDoc struct {
	BasePath string                          `yaml:"basePath"` // Swagger 2.0 (swag)
	Security []map[string][]string           `yaml:"security"`
	Paths    map[string]map[string]yaml.Node `yaml:"paths"` // path item: operations plus parameters/summary/...
}

type specOperation struct {
	Security *[]map[string][]string `yaml:"security"`
}

func loadSpec(t *testing.T, file string) specDoc {
	t.Helper()
	data, err := os.ReadFile(file) //nolint:gosec // test fixture path
	require.NoError(t, err)
	var doc specDoc
	require.NoError(t, yaml.Unmarshal(data, &doc))
	require.NotEmpty(t, doc.Paths, "%s has no paths", file)
	return doc
}

// productionRouteSet returns "METHOD path" for every route the production
// router serves, with gin parameter syntax.
func productionRouteSet(t *testing.T) map[string]bool {
	t.Helper()
	set := map[string]bool{}
	for _, rt := range allRoutes(newProductionRouter(t)) {
		set[rt.Method+" "+rt.Path] = true
	}
	return set
}

// TestOpenAPISpecMatchesRoutes fails when api/openapi.yaml documents an
// operation the server does not route (path, parameter name or method), or
// when its auth requirement disagrees with the route's middleware: public
// routes must say `security: []`, every other route must require
// credentials.
func TestOpenAPISpecMatchesRoutes(t *testing.T) {
	routes := productionRouteSet(t)
	spec := loadSpec(t, "../../api/openapi.yaml")
	var problems []string
	for path, item := range spec.Paths {
		ginPath := openAPIParamPattern.ReplaceAllString(path, ":$1")
		for _, method := range openAPIMethods {
			node, ok := item[method]
			if !ok {
				continue
			}
			var op specOperation
			require.NoError(t, node.Decode(&op), "%s %s", method, path)
			key := strings.ToUpper(method) + " " + ginPath
			if !routes[key] {
				problems = append(problems, key+": documented but not routed")
				continue
			}
			public := classifyRoute(strings.ToUpper(method), ginPath) == authzPublic
			// An operation without `security` inherits the root requirement.
			documentedPublic := len(spec.Security) == 0
			if op.Security != nil {
				documentedPublic = len(*op.Security) == 0
			}
			if public != documentedPublic {
				problems = append(problems, key+": route is public="+boolWord(public)+
					" but the spec says public="+boolWord(documentedPublic))
			}
		}
	}
	sort.Strings(problems)
	require.Empty(t, problems, "api/openapi.yaml disagrees with the router")
}

// TestSwaggerAnnotationsMatchRoutes fails when a swag @Router annotation
// (docs/api/swagger.yaml, served at /swagger/) names a path or method the
// server does not route. Parameter names are not compared: swag takes them
// from @Param, not from the route.
func TestSwaggerAnnotationsMatchRoutes(t *testing.T) {
	routes := map[string]bool{}
	for key := range productionRouteSet(t) {
		routes[ginParamSegment.ReplaceAllString(key, ":")] = true
	}
	spec := loadSpec(t, "../../docs/api/swagger.yaml")

	var problems []string
	for path, item := range spec.Paths {
		full := strings.TrimSuffix(spec.BasePath, "/") + openAPIParamPattern.ReplaceAllString(path, ":")
		for _, method := range openAPIMethods {
			if _, ok := item[method]; !ok {
				continue
			}
			key := strings.ToUpper(method) + " " + full
			if !routes[key] {
				problems = append(problems, key+" (@Router "+path+")")
			}
		}
	}
	sort.Strings(problems)
	require.Empty(t, problems, "swag @Router annotations for unrouted paths; fix the annotation and run make openapi-generate")
}

func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
