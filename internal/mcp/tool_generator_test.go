package mcp

import (
	"sync"
	"testing"
)

func TestToolNameFromRoute(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"GET", "/tickets", "list_tickets"},
		{"POST", "/tickets", "create_ticket"},
		{"GET", "/tickets/:id", "get_ticket"},
		{"PUT", "/tickets/:id", "update_ticket"},
		{"DELETE", "/tickets/:id", "delete_ticket"},
		{"GET", "/tickets/:id/articles", "list_ticket_articles"},
		{"POST", "/tickets/:id/articles", "create_ticket_article"},
		{"GET", "/tickets/:id/articles/:article_id", "get_ticket_article"},
		{"GET", "/queues", "list_queues"},
		{"GET", "/queues/:id", "get_queue"},
		{"GET", "/queues/:id/stats", "list_queue_stats"},
		{"GET", "/queues/:id/agents", "list_queue_agents"},
		{"POST", "/admin/sql", "create_admin_sql"},
		{"GET", "/users/me", "get_user_me"},
		{"GET", "/priorities", "list_priorities"},
		{"GET", "/priorities/:id", "get_priority"},
		{"GET", "/tickets/:id/internal-notes", "list_ticket_internal_notes"},
		{"POST", "/tickets/:id/internal-notes", "create_ticket_internal_note"},
		{"POST", "/search", "create_search"},
		{"GET", "/states", "list_states"},
		{"GET", "/types", "list_types"},
		{"POST", "/tickets/:id/reopen", "create_ticket_reopen"},
		{"POST", "/tickets/:id/time", "create_ticket_time"},
		{"GET", "/organisations", "list_organisations"},
		{"POST", "/organisations", "create_organisation"},  // POST singularizes
		{"GET", "/custom-fields/definitions", "list_custom_field_definitions"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			got := toolNameFromRoute(tt.method, tt.path)
			if got != tt.want {
				t.Errorf("toolNameFromRoute(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractPathParams(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/api/v1/tickets", nil},
		{"/api/v1/tickets/:id", []string{"id"}},
		{"/api/v1/tickets/:id/articles/:article_id", []string{"id", "article_id"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractPathParams(tt.path)
			if len(got) != len(tt.want) {
				t.Errorf("extractPathParams(%q) = %v, want %v", tt.path, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractPathParams(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"tickets", "ticket"},
		{"articles", "article"},
		{"queues", "queue"},
		{"priorities", "priority"},
		{"users", "user"},
		{"notes", "note"},
		{"status", "status"}, // ends in us — don't strip
		{"addresses", "address"},
		{"types", "type"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := singularize(tt.input)
			if got != tt.want {
				t.Errorf("singularize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGinPathToOpenAPI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/api/v1/tickets", "/api/v1/tickets"},
		{"/api/v1/tickets/:id", "/api/v1/tickets/{id}"},
		{"/api/v1/tickets/:id/articles/:article_id", "/api/v1/tickets/{id}/articles/{article_id}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ginPathToOpenAPI(tt.input)
			if got != tt.want {
				t.Errorf("ginPathToOpenAPI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInitDynamicTools(t *testing.T) {
	routes := []RouteInput{
		{
			GroupName:       "api-v1-protected",
			Prefix:          "/api/v1",
			GroupMiddleware: []string{"unified_auth"},
			Path:            "/tickets",
			Method:          "GET",
			HandlerName:     "HandleListTicketsAPI",
			Middleware:      []string{"queue_ro"},
			Description:     "List tickets",
		},
		{
			GroupName:       "api-v1-protected",
			Prefix:          "/api/v1",
			GroupMiddleware: []string{"unified_auth"},
			Path:            "/tickets/:id",
			Method:          "GET",
			HandlerName:     "HandleGetTicketAPI",
			Middleware:      []string{"ticket_access_ro"},
			Description:     "Get ticket by ID",
		},
		{
			GroupName:       "api-v1-protected",
			Prefix:          "/api/v1",
			GroupMiddleware: []string{"unified_auth"},
			Path:            "/queues/:id/stats",
			Method:          "GET",
			HandlerName:     "HandleGetQueueStatsAPI",
			Description:     "Queue ticket statistics",
		},
		{
			// MCP opt-out
			GroupName:   "api-v1-protected",
			Prefix:      "/api/v1",
			Path:        "/auth/login",
			Method:      "POST",
			HandlerName: "HandleLoginAPI",
			Description: "Login",
		},
		{
			// Non-API group — should be skipped
			GroupName:   "admin",
			Prefix:      "/admin",
			Path:        "/dashboard",
			Method:      "GET",
			HandlerName: "HandleDashboard",
			Description: "Admin dashboard",
		},
	}

	// Reset the once for testing
	dynamicInitOnce = syncOnceForTest()

	err := InitDynamicTools(routes, "")
	if err != nil {
		t.Fatalf("InitDynamicTools failed: %v", err)
	}

	tools := GetDynamicTools()
	toolsMap := GetDynamicToolsMap()

	// Should have 3 tools: list_tickets, get_ticket, get_queue_stats
	// auth/login is filtered out, admin/dashboard is not api-v1
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
		for _, tool := range tools {
			t.Logf("  tool: %s", tool.Name)
		}
	}

	// Verify specific tools
	if _, ok := toolsMap["list_tickets"]; !ok {
		t.Error("Missing list_tickets tool")
	}
	if _, ok := toolsMap["get_ticket"]; !ok {
		t.Error("Missing get_ticket tool")
	}
	if _, ok := toolsMap["list_queue_stats"]; !ok {
		t.Error("Missing list_queue_stats tool")
	}

	// Verify middleware is combined correctly
	if tool, ok := toolsMap["list_tickets"]; ok {
		expectedMW := []string{"unified_auth", "queue_ro"}
		if len(tool.Middleware) != len(expectedMW) {
			t.Errorf("list_tickets middleware = %v, want %v", tool.Middleware, expectedMW)
		}
	}
}

func TestGeneratePluginTools(t *testing.T) {
	plugins := []PluginRegistration{
		{
			Name: "goatfictus",
			Routes: []PluginRouteInput{
				{
					Method:      "POST",
					Path:        "/admin/generate",
					Handler:     "GenerateStory",
					Middleware:   []string{"auth", "group:fictus-users"},
					Description: "Generate a story",
				},
				{
					Method:      "GET",
					Path:        "/admin/stats",
					Handler:     "GetStats",
					Description: "Get plugin statistics",
				},
			},
			MCPTools: []PluginMCPToolInput{
				{
					Name:        "generate_story",
					Description: "Generate a fictional story with configurable length and genre",
					Handler:     "GenerateStory",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"genre": map[string]any{
								"type":        "string",
								"description": "Story genre",
							},
							"length": map[string]any{
								"type":        "integer",
								"description": "Target word count",
							},
						},
						"required": []any{"genre"},
					},
				},
			},
		},
	}

	tools := GeneratePluginTools(plugins)

	// Should have 3 tools:
	// - MCPTool "goatfictus_generate_story" (declared, with rich schema)
	// - Route "goatfictus_generatestory" (auto-generated from handler name)
	// - Route "goatfictus_getstats" (auto-generated from handler name)
	// The MCPTool overrides by *name*, not handler — so both exist since names differ.
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
		for _, tool := range tools {
			t.Logf("  tool: %s (plugin: %v)", tool.Name, tool.IsPlugin)
		}
	}

	// Check MCPTool override
	foundMCPTool := false
	foundRouteTool := false
	for _, tool := range tools {
		if tool.Name == "goatfictus_generate_story" {
			foundMCPTool = true
			if tool.Description != "Generate a fictional story with configurable length and genre" {
				t.Errorf("MCPTool description mismatch: %q", tool.Description)
			}
			if _, ok := tool.InputSchema.Properties["genre"]; !ok {
				t.Error("MCPTool missing 'genre' property in schema")
			}
			if !tool.IsPlugin {
				t.Error("MCPTool should have IsPlugin=true")
			}
			if tool.PluginName != "goatfictus" {
				t.Errorf("MCPTool plugin name = %q, want %q", tool.PluginName, "goatfictus")
			}
		}
		if tool.Name == "goatfictus_getstats" {
			foundRouteTool = true
			if !tool.IsPlugin {
				t.Error("Route tool should have IsPlugin=true")
			}
		}
	}
	if !foundMCPTool {
		t.Error("Missing goatfictus_generate_story tool")
	}
	if !foundRouteTool {
		t.Error("Missing goatfictus_getstats tool")
	}
}

func TestAddPluginTools(t *testing.T) {
	// Reset
	dynamicTools = []Tool{
		{Name: "list_tickets", Description: "List tickets"},
	}
	dynamicToolsMap = map[string]*GeneratedTool{
		"list_tickets": {
			Tool: dynamicTools[0],
		},
	}

	pluginTools := []*GeneratedTool{
		{
			Tool: Tool{
				Name:        "goatfictus_generate",
				Description: "Generate content",
			},
			IsPlugin:   true,
			PluginName: "goatfictus",
		},
	}

	AddPluginTools(pluginTools)

	// Should now have 2 tools: core + plugin
	if len(dynamicTools) != 2 {
		t.Errorf("Expected 2 tools after AddPluginTools, got %d", len(dynamicTools))
	}
	if _, ok := dynamicToolsMap["list_tickets"]; !ok {
		t.Error("Core tool list_tickets should still exist")
	}
	if _, ok := dynamicToolsMap["goatfictus_generate"]; !ok {
		t.Error("Plugin tool goatfictus_generate should exist")
	}

	// Calling AddPluginTools again should replace (not duplicate) plugin tools
	AddPluginTools(pluginTools)
	if len(dynamicTools) != 2 {
		t.Errorf("Expected 2 tools after re-add, got %d", len(dynamicTools))
	}
}

func TestFilterRBACMiddleware(t *testing.T) {
	input := []string{"unified_auth", "api_token", "auth", "ticket_access_ro", "admin", "queue_ro"}
	got := filterRBACMiddleware(input)
	want := []string{"ticket_access_ro", "admin", "queue_ro"}

	if len(got) != len(want) {
		t.Errorf("filterRBACMiddleware = %v, want %v", got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("filterRBACMiddleware[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// syncOnceForTest resets sync.Once for testing — replaces the package-level var.
func syncOnceForTest() syncOnce {
	dynamicTools = nil
	dynamicToolsMap = nil
	return syncOnce{}
}

// syncOnce wraps sync.Once so tests can reset it.
type syncOnce = sync.Once
