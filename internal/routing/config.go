package routing

import (
	"fmt"
	"time"
)

// RouteConfig represents a complete route configuration file.
type RouteConfig struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   RouteMetadata `yaml:"metadata"`
	Spec       RouteSpec     `yaml:"spec"`
}

// RouteMetadata contains metadata about the route group.
type RouteMetadata struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Namespace   string            `yaml:"namespace"`
	Enabled     bool              `yaml:"enabled"`
	Version     string            `yaml:"version"`
	Labels      map[string]string `yaml:"labels"`
	Tenants     []string          `yaml:"tenants"`
}

// RouteSpec defines the actual routes and their configuration.
type RouteSpec struct {
	Prefix     string            `yaml:"prefix"`
	Middleware []string          `yaml:"middleware"`
	Routes     []RouteDefinition `yaml:"routes"`
	RateLimit  *RateLimitConfig  `yaml:"rateLimit"`
}

// RouteDefinition represents a single route.
type RouteDefinition struct {
	Path        string                 `yaml:"path"`
	Method      interface{}            `yaml:"method"` // Can be string or []string
	Handler     string                 `yaml:"handler"`
	Handlers    map[string]string      `yaml:"handlers"` // For multiple methods
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Permissions []string               `yaml:"permissions"`
	Features    []string               `yaml:"features"`
	Middleware  []string               `yaml:"middleware"`
	RateLimit   *RateLimitConfig       `yaml:"rateLimit"`
	Condition   string                 `yaml:"condition"`
	OpenAPI     *OpenAPISpec           `yaml:"openapi"`
	TestCases   []RouteTestCase        `yaml:"testCases"`
	Params      map[string]ParamConfig `yaml:"params"`
}

// RateLimitConfig defines rate limiting settings.
type RateLimitConfig struct {
	Requests int           `yaml:"requests"`
	Period   time.Duration `yaml:"period"`
	Key      string        `yaml:"key"` // ip, user, api_key
}

// OpenAPISpec contains OpenAPI documentation for the route.
type OpenAPISpec struct {
	Summary     string                 `yaml:"summary"`
	Description string                 `yaml:"description"`
	Tags        []string               `yaml:"tags"`
	Parameters  []OpenAPIParameter     `yaml:"parameters"`
	RequestBody map[string]interface{} `yaml:"requestBody"`
	Responses   map[int]string         `yaml:"responses"`
	Security    []map[string][]string  `yaml:"security"`
}

// OpenAPIParameter defines an OpenAPI parameter.
type OpenAPIParameter struct {
	Name        string                 `yaml:"name"`
	In          string                 `yaml:"in"` // path, query, header, cookie
	Description string                 `yaml:"description"`
	Required    bool                   `yaml:"required"`
	Schema      map[string]interface{} `yaml:"schema"`
}

// RouteTestCase defines a test case for a route.
type RouteTestCase struct {
	Name        string                 `yaml:"name"`
	Input       map[string]interface{} `yaml:"input"`
	Headers     map[string]string      `yaml:"headers"`
	Expect      map[string]interface{} `yaml:"expect"`
	StatusCode  int                    `yaml:"statusCode"`
	Description string                 `yaml:"description"`
}

// ParamConfig defines parameter validation and transformation.
type ParamConfig struct {
	Type        string   `yaml:"type"` // int, string, uuid, email
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Pattern     string   `yaml:"pattern"`
	Min         int      `yaml:"min"`
	Max         int      `yaml:"max"`
	Enum        []string `yaml:"enum"`
	Transform   string   `yaml:"transform"` // lowercase, uppercase, trim
	Description string   `yaml:"description"`
}

// Validate checks if the route configuration is valid.
func (rc *RouteConfig) Validate() error {
	if rc.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if rc.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if rc.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	return nil
}

// GetMethods returns the HTTP methods for a route definition as a slice.
func (rd *RouteDefinition) GetMethods() []string {
	switch v := rd.Method.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []interface{}:
		methods := make([]string, len(v))
		for i, method := range v {
			if s, ok := method.(string); ok {
				methods[i] = s
			}
		}
		return methods
	default:
		return []string{"GET"} // default
	}
}

// MiddlewareConfig defines middleware configuration.
type MiddlewareConfig struct {
	Name    string                 `yaml:"name"`
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:"config"`
}

// FeatureFlag represents a feature flag configuration.
type FeatureFlag struct {
	Name        string    `yaml:"name"`
	Enabled     bool      `yaml:"enabled"`
	Description string    `yaml:"description"`
	EnabledFor  []string  `yaml:"enabledFor"` // Specific tenants or users
	Percentage  int       `yaml:"percentage"` // For gradual rollout
	StartDate   time.Time `yaml:"startDate"`
	EndDate     time.Time `yaml:"endDate"`
}

// RouteGroup represents a logical grouping of routes.
type RouteGroup struct {
	Name       string   `yaml:"name"`
	Prefix     string   `yaml:"prefix"`
	Middleware []string `yaml:"middleware"`
	Files      []string `yaml:"files"` // YAML files to include
	Enabled    bool     `yaml:"enabled"`
}
