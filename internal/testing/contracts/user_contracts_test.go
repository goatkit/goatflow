package contracts

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestUserContracts tests all user endpoint contracts.
func TestUserContracts(t *testing.T) {
	// Set up router
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Use mock handlers for testing
	mocks := &MockHandlers{}

	// Register API v1 routes with mock handlers
	v1 := r.Group("/api/v1")
	{
		// User endpoints
		v1.GET("/users", mocks.HandleListUsers)
		v1.GET("/users/:id", mocks.HandleGetUser)
		v1.POST("/users", mocks.HandleCreateUser)
		v1.PUT("/users/:id", mocks.HandleUpdateUser)
		v1.DELETE("/users/:id", mocks.HandleDeleteUser)

		// User group endpoints
		v1.GET("/users/:id/groups", mocks.HandleGetUserGroups)
		v1.POST("/users/:id/groups", mocks.HandleAddUserToGroup)
		v1.DELETE("/users/:id/groups/:group_id", mocks.HandleRemoveUserFromGroup)
	}

	// Create contract tester
	ct := NewContractTest(t, r)

	// Get a valid JWT token for authenticated requests
	validToken := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"

	// Contract: List Users
	ct.AddContract(Contract{
		Name:        "GET /api/v1/users - List Users",
		Description: "Should return paginated list of users",
		Method:      "GET",
		Path:        "/api/v1/users",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"data": ArraySchema{
						ItemsSchema: ObjectSchema{
							Properties: map[string]Schema{
								"id":         NumberSchema{Required: true},
								"login":      StringSchema{Required: true},
								"email":      StringSchema{},
								"first_name": StringSchema{},
								"last_name":  StringSchema{},
								"valid_id":   NumberSchema{},
								"groups":     ArraySchema{},
							},
						},
					},
					"pagination": ObjectSchema{
						Properties: map[string]Schema{
							"page":        NumberSchema{Required: true},
							"per_page":    NumberSchema{Required: true},
							"total":       NumberSchema{Required: true},
							"total_pages": NumberSchema{Required: true},
							"has_next":    BooleanSchema{},
							"has_prev":    BooleanSchema{},
						},
					},
				},
			},
		},
	})

	// Contract: Get Single User
	ct.AddContract(Contract{
		Name:        "GET /api/v1/users/:id - Get User",
		Description: "Should return user details with groups",
		Method:      "GET",
		Path:        "/api/v1/users/1",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"data": ObjectSchema{
						Properties: map[string]Schema{
							"id":         NumberSchema{Required: true},
							"login":      StringSchema{Required: true},
							"email":      StringSchema{},
							"first_name": StringSchema{},
							"last_name":  StringSchema{},
							"valid_id":   NumberSchema{},
							"groups": ArraySchema{
								ItemsSchema: ObjectSchema{
									Properties: map[string]Schema{
										"id":   NumberSchema{Required: true},
										"name": StringSchema{Required: true},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	// Contract: Create User
	ct.AddContract(Contract{
		Name:        "POST /api/v1/users - Create User",
		Description: "Should create new user",
		Method:      "POST",
		Path:        "/api/v1/users",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"login":      "testuser",
			"email":      "test@example.com",
			"password":   "SecurePass123!",
			"first_name": "Test",
			"last_name":  "User",
		},
		Expected: Response{
			Status: 201,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"message": StringSchema{Required: true},
					"data": ObjectSchema{
						Properties: map[string]Schema{
							"id":    NumberSchema{Required: true},
							"login": StringSchema{Required: true},
							"email": StringSchema{Required: true},
						},
					},
				},
			},
			Validations: []Validation{
				IsSuccessResponse(),
			},
		},
	})

	// Contract: Update User
	ct.AddContract(Contract{
		Name:        "PUT /api/v1/users/:id - Update User",
		Description: "Should update user fields",
		Method:      "PUT",
		Path:        "/api/v1/users/1",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"first_name": "Updated",
			"last_name":  "Name",
		},
		Expected: Response{
			Status: 200,
			Validations: []Validation{
				IsSuccessResponse(),
			},
		},
	})

	// Contract: Delete User
	ct.AddContract(Contract{
		Name:        "DELETE /api/v1/users/:id - Delete User",
		Description: "Should soft delete user",
		Method:      "DELETE",
		Path:        "/api/v1/users/2",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 204,
		},
	})

	// Contract: Invalid User ID
	ct.AddContract(Contract{
		Name:        "GET /api/v1/users/:id - Invalid ID",
		Description: "Non-existent user should return 404",
		Method:      "GET",
		Path:        "/api/v1/users/99999",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 404,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Unauthorized Access
	ct.AddContract(Contract{
		Name:        "GET /api/v1/users - No Auth",
		Description: "Missing auth token should return 401",
		Method:      "GET",
		Path:        "/api/v1/users",
		Expected: Response{
			Status: 401,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Duplicate User
	ct.AddContract(Contract{
		Name:        "POST /api/v1/users - Duplicate Login",
		Description: "Duplicate login should return 409",
		Method:      "POST",
		Path:        "/api/v1/users",
		Headers: map[string]string{
			"Authorization": validToken,
			"Content-Type":  "application/json",
		},
		Body: map[string]interface{}{
			"login":    "admin", // Assuming admin exists
			"email":    "new@example.com",
			"password": "Password123!",
		},
		Expected: Response{
			Status: 409,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Delete System User
	ct.AddContract(Contract{
		Name:        "DELETE /api/v1/users/:id - System User",
		Description: "Cannot delete system user (ID 1)",
		Method:      "DELETE",
		Path:        "/api/v1/users/1",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 403,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Get User Groups
	ct.AddContract(Contract{
		Name:        "GET /api/v1/users/:id/groups - Get User Groups",
		Description: "Should return user's group memberships",
		Method:      "GET",
		Path:        "/api/v1/users/1/groups",
		Headers: map[string]string{
			"Authorization": validToken,
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"data": ArraySchema{
						ItemsSchema: ObjectSchema{
							Properties: map[string]Schema{
								"id":               NumberSchema{Required: true},
								"name":             StringSchema{Required: true},
								"permission_key":   StringSchema{},
								"permission_value": NumberSchema{},
							},
						},
					},
				},
			},
		},
	})

	// Run all contracts
	ct.Run()
}
