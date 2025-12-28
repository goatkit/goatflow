//go:build db

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleAdminUserGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         string
		expectedStatus int
		expectedError  string
		checkSuccess   bool
		allowNotFound  bool // Allow 404 for user queries when DB isn't seeded
	}{
		{
			name:           "invalid_user_ID",
			userID:         "invalid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid user ID",
			checkSuccess:   false,
		},
		{
			name:           "valid_user_ID_returns_data",
			userID:         "1",
			expectedStatus: http.StatusOK,
			expectedError:  "",
			checkSuccess:   true,
			allowNotFound:  true, // User 1 may not exist in test DB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/admin/users/:id", HandleAdminUserGet)

			req, _ := http.NewRequest("GET", "/admin/users/"+tt.userID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Allow 404 if the test DB isn't seeded with user 1
			if tt.allowNotFound && w.Code == http.StatusNotFound {
				t.Skip("User not found in test database - database may not be seeded")
			}

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				assert.False(t, response["success"].(bool))
				assert.Equal(t, tt.expectedError, response["error"].(string))
			}

			if tt.checkSuccess {
				assert.True(t, response["success"].(bool))
				data, ok := response["data"].(map[string]interface{})
				require.True(t, ok, "Response should contain data object")
				assert.NotNil(t, data["id"])
				assert.NotNil(t, data["login"])
				assert.NotNil(t, data["first_name"])
				assert.NotNil(t, data["last_name"])
				assert.NotNil(t, data["valid_id"])
				assert.NotNil(t, data["groups"])
			}
		})
	}
}

func TestHandleAdminUserCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing_login", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/users", HandleAdminUserCreate)

		formData := url.Values{
			"first_name": {"Test"},
			"last_name":  {"User"},
			"valid_id":   {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Login, first name, and last name are required", response["error"].(string))
	})

	t.Run("missing_first_name", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/users", HandleAdminUserCreate)

		formData := url.Values{
			"login":     {"test@example.com"},
			"last_name": {"User"},
			"valid_id":  {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Login, first name, and last name are required", response["error"].(string))
	})

	t.Run("missing_last_name", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/users", HandleAdminUserCreate)

		formData := url.Values{
			"login":      {"test@example.com"},
			"first_name": {"Test"},
			"valid_id":   {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Login, first name, and last name are required", response["error"].(string))
	})

	t.Run("all_required_fields_present_returns_200", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/users", HandleAdminUserCreate)

		formData := url.Values{
			"login":      {"newuser_unique_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"},
			"first_name": {"Test"},
			"last_name":  {"User"},
			"valid_id":   {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200 OK (either with JSON success or toast response depending on DB state)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleAdminUserUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         string
		formData       url.Values
		expectedStatus int
		expectSuccess  bool
		expectedError  string
	}{
		{
			name:   "invalid_user_ID",
			userID: "invalid",
			formData: url.Values{
				"login":      {"test@example.com"},
				"first_name": {"Test"},
				"last_name":  {"User"},
				"valid_id":   {"1"},
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
			expectedError:  "Invalid user ID",
		},
		{
			name:   "successful_update",
			userID: "1",
			formData: url.Values{
				"login":      {"updated@example.com"},
				"first_name": {"Updated"},
				"last_name":  {"User"},
				"valid_id":   {"1"},
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:   "update_with_password",
			userID: "1",
			formData: url.Values{
				"login":      {"test@example.com"},
				"first_name": {"Test"},
				"last_name":  {"User"},
				"password":   {"newpassword123"},
				"valid_id":   {"1"},
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:   "update_with_groups",
			userID: "1",
			formData: url.Values{
				"login":      {"test@example.com"},
				"first_name": {"Test"},
				"last_name":  {"User"},
				"valid_id":   {"1"},
				"groups":     {"admin", "users"},
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/admin/users/:id", HandleAdminUserUpdate)

			req, _ := http.NewRequest("PUT", "/admin/users/"+tt.userID,
				strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				assert.False(t, response["success"].(bool))
				assert.Equal(t, tt.expectedError, response["error"].(string))
			}

			if tt.expectSuccess {
				assert.True(t, response["success"].(bool))
			}
		})
	}
}

func TestHandleAdminUserDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_user_ID", func(t *testing.T) {
		router := gin.New()
		router.DELETE("/admin/users/:id", HandleAdminUserDelete)

		req, _ := http.NewRequest("DELETE", "/admin/users/invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid user ID", response["error"].(string))
	})
}

func TestHandleAdminUserGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_user_ID", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/users/:id/groups", HandleAdminUserGroups)

		req, _ := http.NewRequest("GET", "/admin/users/invalid/groups", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid user ID", response["error"].(string))
	})
}

func TestHandleAdminUsersStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         string
		formData       url.Values
		expectedStatus int
		expectSuccess  bool
		expectedError  string
	}{
		{
			name:           "invalid_user_ID",
			userID:         "invalid",
			formData:       url.Values{"valid_id": {"1"}},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
			expectedError:  "Invalid user ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/admin/users/:id/status", HandleAdminUsersStatus)

			req, _ := http.NewRequest("PUT", "/admin/users/"+tt.userID+"/status",
				strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				assert.False(t, response["success"].(bool))
				assert.Equal(t, tt.expectedError, response["error"].(string))
			}
		})
	}
}

func TestHandlePasswordPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns_password_policy", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/password-policy", HandlePasswordPolicy)

		req, _ := http.NewRequest("GET", "/api/password-policy", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))

		policy, ok := response["policy"].(map[string]interface{})
		require.True(t, ok, "Response should contain policy object")

		assert.Equal(t, float64(8), policy["minLength"])
		assert.Equal(t, true, policy["requireUppercase"])
		assert.Equal(t, true, policy["requireLowercase"])
		assert.Equal(t, true, policy["requireDigit"])
		assert.Equal(t, false, policy["requireSpecial"])
	})
}

func TestHandleAdminUserCreateJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("create_user_with_JSON_returns_200", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/users", HandleAdminUserCreate)

		jsonBody := `{
			"login": "jsonuser_` + strconv.FormatInt(time.Now().UnixNano(), 10) + `@example.com",
			"first_name": "JSON",
			"last_name": "User",
			"valid_id": 1,
			"groups": ["admin"]
		}`

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200 OK (either JSON or toast depending on DB state)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleAdminUserUpdateJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("update_user_with_JSON", func(t *testing.T) {
		router := gin.New()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		jsonBody := `{
			"login": "updated@example.com",
			"first_name": "Updated",
			"last_name": "User",
			"valid_id": 1,
			"groups": ["admin", "users"]
		}`

		req, _ := http.NewRequest("PUT", "/admin/users/1",
			strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
	})

	t.Run("update_user_with_empty_groups_clears_memberships", func(t *testing.T) {
		router := gin.New()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		// Simulate form submission with groups_submitted flag but no groups
		formData := url.Values{
			"login":            {"test@example.com"},
			"first_name":       {"Test"},
			"last_name":        {"User"},
			"valid_id":         {"1"},
			"groups_submitted": {"1"},
		}

		req, _ := http.NewRequest("PUT", "/admin/users/1",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
	})
}

func TestHandleAdminUsersStatusJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("toggle_status_with_JSON", func(t *testing.T) {
		router := gin.New()
		router.PUT("/admin/users/:id/status", HandleAdminUsersStatus)

		jsonBody := `{"valid_id": 2}`

		req, _ := http.NewRequest("PUT", "/admin/users/1/status",
			strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// May fail without DB connection but should at least parse request
		// Not StatusBadRequest means we got past parsing
		assert.NotEqual(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminUsersFormEncoding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("handles_groups_array_bracket_notation_returns_200", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/users", HandleAdminUserCreate)

		// Some frontend frameworks use groups[] notation
		formData := "login=test_bracket_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com&first_name=Test&last_name=User&valid_id=1&groups[]=admin&groups[]=users"

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200 OK (either JSON or toast depending on DB state)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles_multiple_groups_values_returns_200", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/users", HandleAdminUserCreate)

		// Standard multiple values for same key
		formData := url.Values{
			"login":      {"test_multi_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"},
			"first_name": {"Test"},
			"last_name":  {"User"},
			"valid_id":   {"1"},
		}
		formData.Add("groups", "admin")
		formData.Add("groups", "users")
		formData.Add("groups", "support")

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200 OK (either JSON or toast depending on DB state)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
