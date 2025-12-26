package api

import (
	"github.com/gin-gonic/gin"
)

// AdminRouteDefinition defines a single admin route for both production and testing
type AdminRouteDefinition struct {
	Method  string
	Path    string
	Handler gin.HandlerFunc
}

// EndpointContract documents the expected request/response contract for an endpoint.
// This MUST be kept in sync with both the handler logic AND the JavaScript client code.
// When writing tests, use these contracts to ensure test requests mirror real client requests.
type EndpointContract struct {
	Method          string            // HTTP method
	Path            string            // Route path pattern
	RequiredHeaders map[string]string // Headers the client MUST send for JSON response
	ContentType     string            // Expected Content-Type for request body (if any)
	ResponseType    string            // Expected response type: "json" or "html"
	JSFunction      string            // Name of JavaScript function that calls this endpoint
}

// GetAdminRolesContracts returns the endpoint contracts for admin roles.
// CRITICAL: When handlers check headers to decide JSON vs HTML response,
// document the required headers here AND ensure JS client sends them.
func GetAdminRolesContracts() []EndpointContract {
	return []EndpointContract{
		{
			Method:       "GET",
			Path:         "/admin/roles",
			ResponseType: "html", // Page render, no JS fetch
		},
		{
			Method:          "POST",
			Path:            "/admin/roles",
			RequiredHeaders: map[string]string{"Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest"},
			ContentType:     "application/json",
			ResponseType:    "json",
			JSFunction:      "saveRole",
		},
		{
			Method:          "GET",
			Path:            "/admin/roles/:id",
			RequiredHeaders: map[string]string{"Accept": "application/json", "X-Requested-With": "XMLHttpRequest"},
			ResponseType:    "json",
			JSFunction:      "editRole",
		},
		{
			Method:          "PUT",
			Path:            "/admin/roles/:id",
			RequiredHeaders: map[string]string{"Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest"},
			ContentType:     "application/json",
			ResponseType:    "json",
			JSFunction:      "saveRole, toggleRoleStatus",
		},
		{
			Method:       "DELETE",
			Path:         "/admin/roles/:id",
			ResponseType: "json",
		},
		{
			Method:          "GET",
			Path:            "/admin/roles/:id/users",
			RequiredHeaders: map[string]string{"Accept": "application/json", "X-Requested-With": "XMLHttpRequest"},
			ResponseType:    "json",
			JSFunction:      "viewRoleUsers", // THIS WAS MISSING Accept HEADER - caused the bug!
		},
		{
			Method:          "POST",
			Path:            "/admin/roles/:id/users",
			RequiredHeaders: map[string]string{"Content-Type": "application/json"},
			ContentType:     "application/json",
			ResponseType:    "json",
			JSFunction:      "addUserToRole",
		},
		{
			Method:          "DELETE",
			Path:            "/admin/roles/:id/users/:userId",
			RequiredHeaders: map[string]string{"X-Requested-With": "XMLHttpRequest"},
			ResponseType:    "json",
			JSFunction:      "removeUserFromRole",
		},
		{
			Method:       "GET",
			Path:         "/admin/roles/:id/permissions",
			ResponseType: "html", // Page render
		},
		{
			Method:          "POST",
			Path:            "/admin/roles/:id/permissions",
			RequiredHeaders: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
			ContentType:     "application/x-www-form-urlencoded",
			ResponseType:    "json",
			JSFunction:      "savePermissions",
		},
	}
}

// GetAdminRolesRoutes returns the canonical route definitions for admin roles.
// This MUST be used by both htmx_routes.go and test files to prevent route divergence.
func GetAdminRolesRoutes() []AdminRouteDefinition {
	return []AdminRouteDefinition{
		{"GET", "/roles", handleAdminRoles},
		{"POST", "/roles", handleAdminRoleCreate},
		{"GET", "/roles/:id", handleAdminRoleGet},
		{"PUT", "/roles/:id", handleAdminRoleUpdate},
		{"DELETE", "/roles/:id", handleAdminRoleDelete},
		{"GET", "/roles/:id/users", handleAdminRoleUsers},
		{"POST", "/roles/:id/users", handleAdminRoleUserAdd},
		{"DELETE", "/roles/:id/users/:userId", handleAdminRoleUserRemove},
		{"GET", "/roles/:id/permissions", handleAdminRolePermissions},
		{"POST", "/roles/:id/permissions", handleAdminRolePermissionsUpdate},
	}
}

// GetAdminDynamicFieldsRoutes returns the canonical route definitions for dynamic fields admin.
// This MUST be used by both htmx_routes.go and test files to prevent route divergence.
func GetAdminDynamicFieldsRoutes() []AdminRouteDefinition {
	return []AdminRouteDefinition{
		{"GET", "/dynamic-fields", handleAdminDynamicFields},
		{"GET", "/dynamic-fields/new", handleAdminDynamicFieldNew},
		{"GET", "/dynamic-fields/:id", handleAdminDynamicFieldEdit},
		{"GET", "/dynamic-fields/screens", handleAdminDynamicFieldScreenConfig},
		{"POST", "/dynamic-fields", handleCreateDynamicField},
		{"PUT", "/dynamic-fields/:id", handleUpdateDynamicField},
		{"DELETE", "/dynamic-fields/:id", handleDeleteDynamicField},
		{"PUT", "/dynamic-fields/:id/screens", handleAdminDynamicFieldScreenConfigSave},
		{"POST", "/dynamic-fields/:id/screen", handleAdminDynamicFieldScreenConfigSingle},
	}
}

// RegisterAdminRoutes registers routes from definitions onto a router group
func RegisterAdminRoutes(group *gin.RouterGroup, routes []AdminRouteDefinition) {
	for _, r := range routes {
		switch r.Method {
		case "GET":
			group.GET(r.Path, r.Handler)
		case "POST":
			group.POST(r.Path, r.Handler)
		case "PUT":
			group.PUT(r.Path, r.Handler)
		case "DELETE":
			group.DELETE(r.Path, r.Handler)
		case "PATCH":
			group.PATCH(r.Path, r.Handler)
		}
	}
}

// SetupTestRouterWithRoutes creates a test router using canonical route definitions.
// Tests MUST use this instead of manually registering routes.
func SetupTestRouterWithRoutes(routes []AdminRouteDefinition) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add standard test middleware (mock auth)
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", "Admin")
		c.Next()
	})

	// Register routes under /admin prefix (matching production)
	adminGroup := router.Group("/admin")
	RegisterAdminRoutes(adminGroup, routes)

	return router
}

// ContractForPath returns the endpoint contract for a given method and path.
// Use this in tests to ensure requests include the required headers.
func ContractForPath(contracts []EndpointContract, method, path string) *EndpointContract {
	for _, c := range contracts {
		if c.Method == method && c.Path == path {
			return &c
		}
	}
	return nil
}

// ValidateContractHeaders checks that a request includes all required headers
// per the endpoint contract. Returns a list of missing headers.
func ValidateContractHeaders(contract *EndpointContract, headers map[string]string) []string {
	if contract == nil || contract.RequiredHeaders == nil {
		return nil
	}

	var missing []string
	for key, expectedValue := range contract.RequiredHeaders {
		actual, ok := headers[key]
		if !ok || (expectedValue != "" && actual != expectedValue) {
			missing = append(missing, key)
		}
	}
	return missing
}
