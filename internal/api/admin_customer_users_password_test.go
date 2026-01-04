package api

import (
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

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// TestCustomerUserPasswordHashing verifies that customer passwords are hashed, not stored as plain text.
func TestCustomerUserPasswordHashing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	defer db.Close()

	hasher := auth.NewPasswordHasher()
	testLogin := "pwtest_" + time.Now().Format("20060102150405")
	testPassword := "TestPassword123!"
	testEmail := testLogin + "@example.com"
	testCustomerID := "TESTCUST"

	// Ensure test customer company exists
	createTestCustomerCompany(t, db, testCustomerID)

	// Cleanup after test
	defer func() {
		db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = $1"), testLogin)
	}()

	router := NewSimpleRouterWithDB(db)

	t.Run("create customer user hashes password", func(t *testing.T) {
		form := url.Values{}
		form.Set("login", testLogin)
		form.Set("email", testEmail)
		form.Set("customer_id", testCustomerID)
		form.Set("password", testPassword)
		form.Set("first_name", "Test")
		form.Set("last_name", "User")
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/customer-users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code, "Expected 201 Created, got %d: %s", w.Code, w.Body.String())

		// Verify password was hashed (not stored as plain text)
		var storedPassword string
		err := db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM customer_user WHERE login = $1"), testLogin).Scan(&storedPassword)
		require.NoError(t, err, "Failed to query stored password")

		// Password should NOT be stored as plain text
		assert.NotEqual(t, testPassword, storedPassword, "Password was stored as plain text!")

		// Password should be verifiable with the hasher
		assert.True(t, hasher.VerifyPassword(testPassword, storedPassword), "Stored hash should verify against original password")
	})

	t.Run("update customer user hashes new password", func(t *testing.T) {
		// Get the customer user ID
		var customerUserID int
		err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM customer_user WHERE login = $1"), testLogin).Scan(&customerUserID)
		require.NoError(t, err, "Failed to get customer user ID")

		newPassword := "NewPassword456!"

		form := url.Values{}
		form.Set("login", testLogin)
		form.Set("email", testEmail)
		form.Set("customer_id", testCustomerID)
		form.Set("password", newPassword)
		form.Set("first_name", "Test")
		form.Set("last_name", "User")
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPut, "/admin/customer-users/"+strconv.Itoa(customerUserID), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "Expected 200 OK, got %d: %s", w.Code, w.Body.String())

		// Verify new password was hashed
		var storedPassword string
		err = db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM customer_user WHERE login = $1"), testLogin).Scan(&storedPassword)
		require.NoError(t, err, "Failed to query stored password")

		// Password should NOT be stored as plain text
		assert.NotEqual(t, newPassword, storedPassword, "New password was stored as plain text!")

		// New password should be verifiable
		assert.True(t, hasher.VerifyPassword(newPassword, storedPassword), "Stored hash should verify against new password")

		// Old password should NOT verify
		assert.False(t, hasher.VerifyPassword(testPassword, storedPassword), "Old password should not verify against new hash")
	})

	t.Run("update without password does not clear existing password", func(t *testing.T) {
		// Get current password hash
		var originalHash string
		err := db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM customer_user WHERE login = $1"), testLogin).Scan(&originalHash)
		require.NoError(t, err)

		var customerUserID int
		err = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM customer_user WHERE login = $1"), testLogin).Scan(&customerUserID)
		require.NoError(t, err)

		// Update WITHOUT providing password
		form := url.Values{}
		form.Set("login", testLogin)
		form.Set("email", testEmail)
		form.Set("customer_id", testCustomerID)
		// No password field!
		form.Set("first_name", "Updated")
		form.Set("last_name", "Name")
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPut, "/admin/customer-users/"+strconv.Itoa(customerUserID), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		// Password should remain unchanged
		var storedPassword string
		err = db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM customer_user WHERE login = $1"), testLogin).Scan(&storedPassword)
		require.NoError(t, err)

		assert.Equal(t, originalHash, storedPassword, "Password hash should not change when password field is empty")
	})
}
