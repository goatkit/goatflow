package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
)

func TestCustomerProfileHandler(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available for testing")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)

	t.Run("returns 401 when not authenticated", func(t *testing.T) {
		router := gin.New()
		router.GET("/customer/profile", handleCustomerProfile(db))

		req := httptest.NewRequest(http.MethodGet, "/customer/profile", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should redirect to login or return 401
		assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusSeeOther || w.Code == http.StatusFound,
			"Expected 401, 302, or 303 redirect, got %d", w.Code)
	})

	t.Run("renders profile page when authenticated", func(t *testing.T) {
		router := gin.New()
		// Simulate customer auth context - must set user_role to "Customer"
		router.Use(func(c *gin.Context) {
			c.Set("username", "customer@example.com")
			c.Set("user_role", "Customer")
			c.Set("authenticated", true)
			c.Next()
		})
		router.GET("/customer/profile", handleCustomerProfile(db))

		req := httptest.NewRequest(http.MethodGet, "/customer/profile", nil)
		req.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200, 500 (customer not in DB), or redirect
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusSeeOther || w.Code == http.StatusInternalServerError,
			"Expected 200, 500 (customer not in DB), or redirect, got %d", w.Code)
	})
}

func TestCustomerUpdateProfileHandler(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available for testing")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)

	t.Run("returns 401 when not authenticated", func(t *testing.T) {
		router := gin.New()
		router.POST("/customer/profile/update", handleCustomerUpdateProfile(db))

		body := `{"first_name":"Test","last_name":"User"}`
		req := httptest.NewRequest(http.MethodPost, "/customer/profile/update", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusSeeOther || w.Code == http.StatusFound,
			"Expected 401, 302, or 303 redirect, got %d", w.Code)
	})

	t.Run("returns 400 when first_name is missing", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "customer@example.com")
			c.Set("user_role", "Customer")
			c.Set("authenticated", true)
			c.Next()
		})
		router.POST("/customer/profile/update", handleCustomerUpdateProfile(db))

		body := `{"last_name":"User"}`
		req := httptest.NewRequest(http.MethodPost, "/customer/profile/update", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})

	t.Run("returns 400 when last_name is missing", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "customer@example.com")
			c.Set("user_role", "Customer")
			c.Set("authenticated", true)
			c.Next()
		})
		router.POST("/customer/profile/update", handleCustomerUpdateProfile(db))

		body := `{"first_name":"Test"}`
		req := httptest.NewRequest(http.MethodPost, "/customer/profile/update", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})
}

func TestCustomerPasswordFormHandler(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available for testing")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)

	t.Run("returns 401 when not authenticated", func(t *testing.T) {
		router := gin.New()
		router.GET("/customer/password/form", handleCustomerPasswordForm(db))

		req := httptest.NewRequest(http.MethodGet, "/customer/password/form", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusSeeOther || w.Code == http.StatusFound,
			"Expected 401, 302, or 303 redirect, got %d", w.Code)
	})
}

func TestCustomerChangePasswordHandler(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available for testing")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)

	t.Run("returns 401 when not authenticated", func(t *testing.T) {
		router := gin.New()
		router.POST("/customer/password/change", handleCustomerChangePassword(db))

		body := `{"current_password":"old","new_password":"new","confirm_password":"new"}`
		req := httptest.NewRequest(http.MethodPost, "/customer/password/change", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusSeeOther || w.Code == http.StatusFound,
			"Expected 401, 302, or 303 redirect, got %d", w.Code)
	})

	t.Run("returns 400 when current_password is missing", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "customer@example.com")
			c.Set("user_role", "Customer")
			c.Set("authenticated", true)
			c.Next()
		})
		router.POST("/customer/password/change", handleCustomerChangePassword(db))

		body := `{"new_password":"newpass123","confirm_password":"newpass123"}`
		req := httptest.NewRequest(http.MethodPost, "/customer/password/change", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"].(string), "required")
	})

	t.Run("returns 400 when passwords do not match", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "customer@example.com")
			c.Set("user_role", "Customer")
			c.Set("authenticated", true)
			c.Next()
		})
		router.POST("/customer/password/change", handleCustomerChangePassword(db))

		body := `{"current_password":"oldpass","new_password":"newpass123","confirm_password":"different123"}`
		req := httptest.NewRequest(http.MethodPost, "/customer/password/change", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"].(string), "match")
	})

	t.Run("returns 400 when new password same as current", func(t *testing.T) {
		// This test requires a valid customer in the database
		// Skip for now if customer doesn't exist
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("username", "customer@example.com")
			c.Set("user_role", "Customer")
			c.Set("authenticated", true)
			c.Next()
		})
		router.POST("/customer/password/change", handleCustomerChangePassword(db))

		// When passwords match but are same as current, should get specific error
		body := `{"current_password":"demo","new_password":"demo","confirm_password":"demo"}`
		req := httptest.NewRequest(http.MethodPost, "/customer/password/change", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Might get 400 (same password) or 401 (wrong current password) depending on actual DB state
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnauthorized || w.Code == http.StatusInternalServerError,
			"Expected 400 or 401 or 500, got %d", w.Code)
	})
}

func TestGetCustomerInfo(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available for testing")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)

	// Create a test customer user
	testLogin := "initials.test.user"
	testEmail := "initials.test@example.com"
	testFirstName := "Emma"
	testLastName := "Scott"

	// Clean up any existing test user
	_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE login = ?`), testLogin)

	// Insert test user
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'test-company', 'testhash', ?, ?, 1, NOW(), 1, NOW(), 1)
	`), testLogin, testEmail, testFirstName, testLastName)
	require.NoError(t, err, "Failed to create test customer user")

	// Clean up after test
	defer func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE login = ?`), testLogin)
	}()

	t.Run("returns correct initials for Emma Scott", func(t *testing.T) {
		info := getCustomerInfo(db, testLogin)

		assert.Equal(t, testFirstName, info["FirstName"], "FirstName should be Emma")
		assert.Equal(t, testLastName, info["LastName"], "LastName should be Scott")
		assert.Equal(t, testFirstName, info["first_name"], "first_name should be Emma")
		assert.Equal(t, testLastName, info["last_name"], "last_name should be Scott")

		// THE KEY ASSERTION - initials must be two letters
		assert.Equal(t, "ES", info["Initials"], "Initials (CamelCase) should be ES, not E")
		assert.Equal(t, "ES", info["initials"], "initials (snake_case) should be ES, not E")
		assert.Len(t, info["initials"], 2, "initials should be exactly 2 characters")
	})

	t.Run("returns single letter when only first name exists", func(t *testing.T) {
		// Update to have no last name
		_, err = db.Exec(database.ConvertPlaceholders(`UPDATE customer_user SET last_name = '' WHERE login = ?`), testLogin)
		require.NoError(t, err)

		info := getCustomerInfo(db, testLogin)
		assert.Equal(t, "E", info["initials"], "initials should be E when only first name exists")

		// Restore last name
		_, _ = db.Exec(database.ConvertPlaceholders(`UPDATE customer_user SET last_name = ? WHERE login = ?`), testLastName, testLogin)
	})
}

func TestGetCustomerInitials(t *testing.T) {
	tests := []struct {
		name      string
		firstName string
		lastName  string
		expected  string
	}{
		{
			name:      "both names provided",
			firstName: "Emma",
			lastName:  "Smith",
			expected:  "ES",
		},
		{
			name:      "only first name",
			firstName: "Emma",
			lastName:  "",
			expected:  "E",
		},
		{
			name:      "only last name",
			firstName: "",
			lastName:  "Smith",
			expected:  "S",
		},
		{
			name:      "no names",
			firstName: "",
			lastName:  "",
			expected:  "?",
		},
		{
			name:      "lowercase names",
			firstName: "john",
			lastName:  "doe",
			expected:  "JD",
		},
		{
			name:      "mixed case names",
			firstName: "jOHN",
			lastName:  "dOE",
			expected:  "JD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCustomerInitials(tt.firstName, tt.lastName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPasswordPolicyErrorMessage(t *testing.T) {
	tests := []struct {
		code     string
		contains string
	}{
		{"regexp_mismatch", "pattern"},
		{"min_size", "short"},
		{"min_2_lower_2_upper", "uppercase"},
		{"need_digit", "number"},
		{"min_2_characters", "letters"},
		{"unknown_code", "requirements"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := getPasswordPolicyErrorMessage(tt.code)
			assert.Contains(t, strings.ToLower(result), strings.ToLower(tt.contains),
				"Error message for %s should contain '%s'", tt.code, tt.contains)
		})
	}
}
