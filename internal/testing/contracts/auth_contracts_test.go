package contracts

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestAuthenticationContracts tests all authentication endpoint contracts.
func TestAuthenticationContracts(t *testing.T) {
	// Set up router
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Use mock handlers for testing
	mocks := &MockHandlers{}

	// Register API v1 routes with mock handlers
	v1 := r.Group("/api/v1")
	{
		// Auth endpoints
		v1.POST("/auth/login", mocks.HandleLogin)
		v1.POST("/auth/refresh", mocks.HandleRefresh)
		v1.POST("/auth/logout", mocks.HandleLogout)
	}

	// Create contract tester
	ct := NewContractTest(t, r)

	// Contract: Successful Login
	ct.AddContract(Contract{
		Name:        "POST /api/v1/auth/login - Success",
		Description: "Valid credentials should return JWT tokens",
		Method:      "POST",
		Path:        "/api/v1/auth/login",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"login":    "testuser",
			"password": "testpass123",
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success": BooleanSchema{Required: true},
					"access_token": StringSchema{
						Required:  true,
						MinLength: 20,
					},
					"refresh_token": StringSchema{
						Required:  true,
						MinLength: 20,
					},
					"token_type": StringSchema{
						Required: true,
					},
					"expires_in": NumberSchema{
						Required: true,
					},
					"user": ObjectSchema{
						Required: true,
						Properties: map[string]Schema{
							"id":         NumberSchema{Required: true},
							"login":      StringSchema{Required: true},
							"email":      StringSchema{},
							"first_name": StringSchema{},
							"last_name":  StringSchema{},
							"role":       StringSchema{},
						},
					},
				},
			},
			Validations: []Validation{
				IsSuccessResponse(),
				HasFields("access_token", "refresh_token", "user"),
			},
		},
	})

	// Contract: Invalid Credentials
	ct.AddContract(Contract{
		Name:        "POST /api/v1/auth/login - Invalid Credentials",
		Description: "Invalid credentials should return 401",
		Method:      "POST",
		Path:        "/api/v1/auth/login",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"login":    "baduser",
			"password": "wrongpass",
		},
		Expected: Response{
			Status: 401,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Missing Required Fields
	ct.AddContract(Contract{
		Name:        "POST /api/v1/auth/login - Missing Password",
		Description: "Missing password should return 400",
		Method:      "POST",
		Path:        "/api/v1/auth/login",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"login": "testuser",
		},
		Expected: Response{
			Status: 400,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Refresh Token
	ct.AddContract(Contract{
		Name:        "POST /api/v1/auth/refresh - Valid Token",
		Description: "Valid refresh token should return new access token",
		Method:      "POST",
		Path:        "/api/v1/auth/refresh",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"refresh_token": "valid_refresh_token_here",
		},
		Expected: Response{
			Status: 200,
			BodySchema: ObjectSchema{
				Required: true,
				Properties: map[string]Schema{
					"success":      BooleanSchema{Required: true},
					"access_token": StringSchema{Required: true, MinLength: 20},
					"token_type":   StringSchema{Required: true},
					"expires_in":   NumberSchema{Required: true},
				},
			},
		},
	})

	// Contract: Invalid Refresh Token
	ct.AddContract(Contract{
		Name:        "POST /api/v1/auth/refresh - Invalid Token",
		Description: "Invalid refresh token should return 401",
		Method:      "POST",
		Path:        "/api/v1/auth/refresh",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"refresh_token": "invalid_token",
		},
		Expected: Response{
			Status: 401,
			Validations: []Validation{
				IsErrorResponse(),
			},
		},
	})

	// Contract: Logout
	ct.AddContract(Contract{
		Name:        "POST /api/v1/auth/logout - Success",
		Description: "Logout should always succeed",
		Method:      "POST",
		Path:        "/api/v1/auth/logout",
		Headers: map[string]string{
			"Authorization": "Bearer valid_token",
		},
		Expected: Response{
			Status: 200,
			Validations: []Validation{
				IsSuccessResponse(),
				HasFields("message"),
			},
		},
	})

	// Run all contracts
	ct.Run()
}
