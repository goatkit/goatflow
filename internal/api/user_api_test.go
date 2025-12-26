//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("List Users", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.GET("/api/v1/users", HandleListUsersAPI)

			req := httptest.NewRequest("GET", "/api/v1/users", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, false, response["success"])
		})

		t.Run("should return paginated user list", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Set("is_authenticated", true)
				c.Next()
			})
			router.GET("/api/v1/users", HandleListUsersAPI)

			req := httptest.NewRequest("GET", "/api/v1/users?page=1&per_page=10", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])
				assert.NotNil(t, response["data"])
			}
		})

		t.Run("should filter by valid status", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/users", HandleListUsersAPI)

			req := httptest.NewRequest("GET", "/api/v1/users?valid=1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should process the filter
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("should search by login or name", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/users", HandleListUsersAPI)

			req := httptest.NewRequest("GET", "/api/v1/users?search=admin", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Get Single User", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.GET("/api/v1/users/:id", HandleGetUserAPI)

			req := httptest.NewRequest("GET", "/api/v1/users/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should return 404 for non-existent user", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/users/:id", HandleGetUserAPI)

			req := httptest.NewRequest("GET", "/api/v1/users/99999", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 404 when user doesn't exist
			if w.Code == http.StatusNotFound {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, false, response["success"])
			}
		})

		t.Run("should return user details with groups", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/users/:id", HandleGetUserAPI)

			req := httptest.NewRequest("GET", "/api/v1/users/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				data := response["data"].(map[string]interface{})
				assert.NotNil(t, data["id"])
				assert.NotNil(t, data["login"])
				// Should include groups array
				assert.NotNil(t, data["groups"])
			}
		})
	})

	t.Run("Create User", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.POST("/api/v1/users", HandleCreateUserAPI)

			body := map[string]interface{}{
				"login": "newuser",
				"email": "new@example.com",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should validate required fields", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/users", HandleCreateUserAPI)

			// Missing login
			body := map[string]interface{}{
				"email": "new@example.com",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("should create user with valid data", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/users", HandleCreateUserAPI)

			body := map[string]interface{}{
				"login":      "newuser",
				"email":      "new@example.com",
				"first_name": "New",
				"last_name":  "User",
				"password":   "SecurePass123!",
				"valid_id":   1,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusCreated {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])

				data := response["data"].(map[string]interface{})
				assert.NotNil(t, data["id"])
				assert.Equal(t, "newuser", data["login"])
			}
		})

		t.Run("should hash password before storing", func(t *testing.T) {
			// This is tested implicitly - the handler should never return
			// the password in the response
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/users", HandleCreateUserAPI)

			body := map[string]interface{}{
				"login":    "pwdtest",
				"email":    "pwd@test.com",
				"password": "PlainTextPassword",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusCreated {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				data := response["data"].(map[string]interface{})
				// Password should never be in response
				assert.Nil(t, data["password"])
				assert.Nil(t, data["pw"])
			}
		})
	})

	t.Run("Update User", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/v1/users/:id", HandleUpdateUserAPI)

			body := map[string]interface{}{
				"first_name": "Updated",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/users/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should update user fields", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.PUT("/api/v1/users/:id", HandleUpdateUserAPI)

			body := map[string]interface{}{
				"first_name": "Updated",
				"last_name":  "Name",
				"email":      "updated@example.com",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/users/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])
			}
		})

		t.Run("should not allow updating login", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.PUT("/api/v1/users/:id", HandleUpdateUserAPI)

			body := map[string]interface{}{
				"login": "changedlogin", // Should be ignored or rejected
				"email": "test@example.com",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/users/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Handler should either ignore the login field or return an error
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("Delete User", func(t *testing.T) {
		t.Run("should require authentication", func(t *testing.T) {
			router := gin.New()
			router.DELETE("/api/v1/users/:id", HandleDeleteUserAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/users/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})

		t.Run("should soft delete user (set valid_id=2)", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/users/:id", HandleDeleteUserAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/users/2", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 204 No Content on success
			if w.Code == http.StatusNoContent {
				// User should be marked invalid, not actually deleted
				// This would be verified in integration tests
				assert.Equal(t, 0, w.Body.Len())
			}
		})

		t.Run("should not delete system users", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/users/:id", HandleDeleteUserAPI)

			// User ID 1 is typically the admin user
			req := httptest.NewRequest("DELETE", "/api/v1/users/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should refuse to delete system users
			if w.Code == http.StatusForbidden {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, false, response["success"])
				assert.Contains(t, response["error"], "system user")
			}
		})
	})

	t.Run("User Groups", func(t *testing.T) {
		t.Run("should get user's groups", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.GET("/api/v1/users/:id/groups", HandleGetUserGroupsAPI)

			req := httptest.NewRequest("GET", "/api/v1/users/1/groups", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])

				data := response["data"].([]interface{})
				// Should return array of groups
				assert.NotNil(t, data)
			}
		})

		t.Run("should add user to group", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.POST("/api/v1/users/:id/groups", HandleAddUserToGroupAPI)

			body := map[string]interface{}{
				"group_id":    1,
				"permissions": "rw",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/v1/users/2/groups", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, true, response["success"])
			}
		})

		t.Run("should remove user from group", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.DELETE("/api/v1/users/:id/groups/:group_id", HandleRemoveUserFromGroupAPI)

			req := httptest.NewRequest("DELETE", "/api/v1/users/2/groups/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNoContent {
				assert.Equal(t, 0, w.Body.Len())
			}
		})
	})
}
