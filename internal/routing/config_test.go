package routing

import (
	"testing"
)

func TestRouteConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  RouteConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: RouteConfig{
				APIVersion: "v1",
				Kind:       "RouteGroup",
				Metadata: RouteMetadata{
					Name: "test-routes",
				},
			},
			wantErr: false,
		},
		{
			name: "missing apiVersion",
			config: RouteConfig{
				Kind: "RouteGroup",
				Metadata: RouteMetadata{
					Name: "test-routes",
				},
			},
			wantErr: true,
			errMsg:  "apiVersion is required",
		},
		{
			name: "missing kind",
			config: RouteConfig{
				APIVersion: "v1",
				Metadata: RouteMetadata{
					Name: "test-routes",
				},
			},
			wantErr: true,
			errMsg:  "kind is required",
		},
		{
			name: "missing metadata.name",
			config: RouteConfig{
				APIVersion: "v1",
				Kind:       "RouteGroup",
				Metadata:   RouteMetadata{},
			},
			wantErr: true,
			errMsg:  "metadata.name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRouteDefinition_GetMethods(t *testing.T) {
	tests := []struct {
		name     string
		method   interface{}
		expected []string
	}{
		{
			name:     "single method string",
			method:   "GET",
			expected: []string{"GET"},
		},
		{
			name:     "string slice",
			method:   []string{"GET", "POST"},
			expected: []string{"GET", "POST"},
		},
		{
			name:     "interface slice",
			method:   []interface{}{"PUT", "PATCH", "DELETE"},
			expected: []string{"PUT", "PATCH", "DELETE"},
		},
		{
			name:     "nil method defaults to GET",
			method:   nil,
			expected: []string{"GET"},
		},
		{
			name:     "unknown type defaults to GET",
			method:   123,
			expected: []string{"GET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := RouteDefinition{Method: tt.method}
			got := rd.GetMethods()

			if len(got) != len(tt.expected) {
				t.Errorf("expected %d methods, got %d", len(tt.expected), len(got))
				return
			}

			for i, m := range got {
				if m != tt.expected[i] {
					t.Errorf("method[%d]: expected %q, got %q", i, tt.expected[i], m)
				}
			}
		})
	}
}

func TestRouteMetadata_Fields(t *testing.T) {
	metadata := RouteMetadata{
		Name:        "api-routes",
		Description: "API endpoints for the application",
		Namespace:   "default",
		Enabled:     true,
		Version:     "1.0.0",
		Labels: map[string]string{
			"app":     "goatflow",
			"version": "1.0",
		},
		Tenants: []string{"tenant1", "tenant2"},
	}

	if metadata.Name != "api-routes" {
		t.Errorf("expected Name api-routes, got %s", metadata.Name)
	}
	if metadata.Description != "API endpoints for the application" {
		t.Errorf("unexpected Description: %s", metadata.Description)
	}
	if metadata.Namespace != "default" {
		t.Errorf("expected Namespace default, got %s", metadata.Namespace)
	}
	if !metadata.Enabled {
		t.Error("expected Enabled to be true")
	}
	if metadata.Version != "1.0.0" {
		t.Errorf("expected Version 1.0.0, got %s", metadata.Version)
	}
	if len(metadata.Labels) != 2 {
		t.Errorf("expected 2 Labels, got %d", len(metadata.Labels))
	}
	if len(metadata.Tenants) != 2 {
		t.Errorf("expected 2 Tenants, got %d", len(metadata.Tenants))
	}
}

func TestRouteSpec_Fields(t *testing.T) {
	spec := RouteSpec{
		Prefix:     "/api/v1",
		Middleware: []string{"auth", "cors", "ratelimit"},
		Routes: []RouteDefinition{
			{Path: "/users", Method: "GET", Handler: "ListUsers"},
			{Path: "/users", Method: "POST", Handler: "CreateUser"},
		},
		RateLimit: &RateLimitConfig{
			Requests: 100,
			Period:   60,
			Key:      "ip",
		},
	}

	if spec.Prefix != "/api/v1" {
		t.Errorf("expected Prefix /api/v1, got %s", spec.Prefix)
	}
	if len(spec.Middleware) != 3 {
		t.Errorf("expected 3 Middleware, got %d", len(spec.Middleware))
	}
	if len(spec.Routes) != 2 {
		t.Errorf("expected 2 Routes, got %d", len(spec.Routes))
	}
	if spec.RateLimit == nil {
		t.Error("expected RateLimit to be set")
	}
}

func TestRouteDefinition_Fields(t *testing.T) {
	rd := RouteDefinition{
		Path:        "/tickets/:id",
		Method:      "GET",
		Handler:     "GetTicket",
		Name:        "get-ticket",
		Description: "Get a single ticket by ID",
		Permissions: []string{"tickets.read"},
		Features:    []string{"ticket-view"},
		Middleware:  []string{"auth"},
		Condition:   "feature.enabled",
		Params: map[string]ParamConfig{
			"id": {
				Type:     "uuid",
				Required: true,
			},
		},
	}

	if rd.Path != "/tickets/:id" {
		t.Errorf("expected Path /tickets/:id, got %s", rd.Path)
	}
	if rd.Handler != "GetTicket" {
		t.Errorf("expected Handler GetTicket, got %s", rd.Handler)
	}
	if rd.Name != "get-ticket" {
		t.Errorf("expected Name get-ticket, got %s", rd.Name)
	}
	if len(rd.Permissions) != 1 || rd.Permissions[0] != "tickets.read" {
		t.Errorf("unexpected Permissions: %v", rd.Permissions)
	}
	if len(rd.Features) != 1 || rd.Features[0] != "ticket-view" {
		t.Errorf("unexpected Features: %v", rd.Features)
	}
	if rd.Condition != "feature.enabled" {
		t.Errorf("expected Condition feature.enabled, got %s", rd.Condition)
	}
}

func TestRateLimitConfig_Fields(t *testing.T) {
	rl := RateLimitConfig{
		Requests: 1000,
		Period:   3600,
		Key:      "api_key",
	}

	if rl.Requests != 1000 {
		t.Errorf("expected Requests 1000, got %d", rl.Requests)
	}
	if rl.Period != 3600 {
		t.Errorf("expected Period 3600, got %d", rl.Period)
	}
	if rl.Key != "api_key" {
		t.Errorf("expected Key api_key, got %s", rl.Key)
	}
}

func TestOpenAPISpec_Fields(t *testing.T) {
	spec := OpenAPISpec{
		Summary:     "List all tickets",
		Description: "Returns a paginated list of tickets",
		Tags:        []string{"tickets", "api"},
		Parameters: []OpenAPIParameter{
			{
				Name:        "page",
				In:          "query",
				Description: "Page number",
				Required:    false,
			},
		},
		Responses: map[int]string{
			200: "Success",
			401: "Unauthorized",
		},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}

	if spec.Summary != "List all tickets" {
		t.Errorf("expected Summary 'List all tickets', got %s", spec.Summary)
	}
	if len(spec.Tags) != 2 {
		t.Errorf("expected 2 Tags, got %d", len(spec.Tags))
	}
	if len(spec.Parameters) != 1 {
		t.Errorf("expected 1 Parameter, got %d", len(spec.Parameters))
	}
	if len(spec.Responses) != 2 {
		t.Errorf("expected 2 Responses, got %d", len(spec.Responses))
	}
}

func TestOpenAPIParameter_Fields(t *testing.T) {
	param := OpenAPIParameter{
		Name:        "ticket_id",
		In:          "path",
		Description: "The ticket ID",
		Required:    true,
		Schema: map[string]interface{}{
			"type":   "string",
			"format": "uuid",
		},
	}

	if param.Name != "ticket_id" {
		t.Errorf("expected Name ticket_id, got %s", param.Name)
	}
	if param.In != "path" {
		t.Errorf("expected In path, got %s", param.In)
	}
	if !param.Required {
		t.Error("expected Required to be true")
	}
	if len(param.Schema) != 2 {
		t.Errorf("expected 2 Schema fields, got %d", len(param.Schema))
	}
}

func TestRouteTestCase_Fields(t *testing.T) {
	tc := RouteTestCase{
		Name:        "valid ticket creation",
		Description: "Test creating a valid ticket",
		StatusCode:  201,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer test-token",
		},
		Input: map[string]interface{}{
			"title":   "Test Ticket",
			"content": "This is a test",
		},
		Expect: map[string]interface{}{
			"success": true,
		},
	}

	if tc.Name != "valid ticket creation" {
		t.Errorf("expected Name 'valid ticket creation', got %s", tc.Name)
	}
	if tc.StatusCode != 201 {
		t.Errorf("expected StatusCode 201, got %d", tc.StatusCode)
	}
	if len(tc.Headers) != 2 {
		t.Errorf("expected 2 Headers, got %d", len(tc.Headers))
	}
	if len(tc.Input) != 2 {
		t.Errorf("expected 2 Input fields, got %d", len(tc.Input))
	}
}

func TestParamConfig_Fields(t *testing.T) {
	pc := ParamConfig{
		Type:        "string",
		Required:    true,
		Default:     "",
		Pattern:     "^[a-z]+$",
		Min:         1,
		Max:         100,
		Enum:        []string{"open", "closed", "pending"},
		Transform:   "lowercase",
		Description: "Ticket status filter",
	}

	if pc.Type != "string" {
		t.Errorf("expected Type string, got %s", pc.Type)
	}
	if !pc.Required {
		t.Error("expected Required to be true")
	}
	if pc.Pattern != "^[a-z]+$" {
		t.Errorf("expected Pattern ^[a-z]+$, got %s", pc.Pattern)
	}
	if pc.Min != 1 {
		t.Errorf("expected Min 1, got %d", pc.Min)
	}
	if pc.Max != 100 {
		t.Errorf("expected Max 100, got %d", pc.Max)
	}
	if len(pc.Enum) != 3 {
		t.Errorf("expected 3 Enum values, got %d", len(pc.Enum))
	}
	if pc.Transform != "lowercase" {
		t.Errorf("expected Transform lowercase, got %s", pc.Transform)
	}
}

func TestMiddlewareConfig_Fields(t *testing.T) {
	mc := MiddlewareConfig{
		Name:    "cors",
		Enabled: true,
		Config: map[string]interface{}{
			"allowed_origins": []string{"*"},
			"max_age":         3600,
		},
	}

	if mc.Name != "cors" {
		t.Errorf("expected Name cors, got %s", mc.Name)
	}
	if !mc.Enabled {
		t.Error("expected Enabled to be true")
	}
	if len(mc.Config) != 2 {
		t.Errorf("expected 2 Config fields, got %d", len(mc.Config))
	}
}

func TestFeatureFlag_Fields(t *testing.T) {
	ff := FeatureFlag{
		Name:        "new-ticket-form",
		Enabled:     true,
		Description: "Enable the new ticket form UI",
		EnabledFor:  []string{"tenant1", "beta-users"},
		Percentage:  50,
	}

	if ff.Name != "new-ticket-form" {
		t.Errorf("expected Name new-ticket-form, got %s", ff.Name)
	}
	if !ff.Enabled {
		t.Error("expected Enabled to be true")
	}
	if len(ff.EnabledFor) != 2 {
		t.Errorf("expected 2 EnabledFor entries, got %d", len(ff.EnabledFor))
	}
	if ff.Percentage != 50 {
		t.Errorf("expected Percentage 50, got %d", ff.Percentage)
	}
}

func TestRouteGroup_Fields(t *testing.T) {
	rg := RouteGroup{
		Name:       "api-v2",
		Prefix:     "/api/v2",
		Middleware: []string{"auth", "audit"},
		Files:      []string{"routes/tickets.yaml", "routes/users.yaml"},
		Enabled:    true,
	}

	if rg.Name != "api-v2" {
		t.Errorf("expected Name api-v2, got %s", rg.Name)
	}
	if rg.Prefix != "/api/v2" {
		t.Errorf("expected Prefix /api/v2, got %s", rg.Prefix)
	}
	if len(rg.Middleware) != 2 {
		t.Errorf("expected 2 Middleware, got %d", len(rg.Middleware))
	}
	if len(rg.Files) != 2 {
		t.Errorf("expected 2 Files, got %d", len(rg.Files))
	}
	if !rg.Enabled {
		t.Error("expected Enabled to be true")
	}
}
