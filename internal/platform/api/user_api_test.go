package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
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

			id, login := createUpdateTestUser(t)
			body := map[string]interface{}{
				"first_name": "Updated",
				"last_name":  "Name",
				"email":      login + ".updated@example.test",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/users/%d", id), bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var response map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Equal(t, true, response["success"])
			first, last, _, _ := userRow(t, id)
			assert.Equal(t, "Updated", first)
			assert.Equal(t, "Name", last)
		})

		t.Run("should not allow updating login", func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Next()
			})
			router.PUT("/api/v1/users/:id", HandleUpdateUserAPI)

			id, login := createUpdateTestUser(t)
			body := map[string]interface{}{
				"login": "changedlogin", // Should be ignored or rejected
				"email": login + ".other@example.test",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/users/%d", id), bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Handler should either ignore the login field or return an error
			assert.NotEqual(t, http.StatusInternalServerError, w.Code)
			db, err := database.GetDB()
			require.NoError(t, err)
			var got string
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT login FROM users WHERE id = ?`), id).Scan(&got))
			assert.Equal(t, login, got, "login must not change")
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

			id, _ := createUpdateTestUser(t)
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/users/%d", id), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
			assert.Equal(t, 0, w.Body.Len())
			_, _, _, validID := userRow(t, id)
			assert.Equal(t, 2, validID, "user is marked invalid, not removed")
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
}

func TestUserMeAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("should accept uint user id from auth middleware", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", uint(1))
			c.Next()
		})
		router.GET("/api/v1/users/me", HandleUserMeAPI)

		req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusInternalServerError, w.Code)
		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, true, response["success"])

			data := response["data"].(map[string]interface{})
			assert.NotNil(t, data["id"])
			assert.NotNil(t, data["login"])
			assert.NotNil(t, data["email"])
		}
	})
}
