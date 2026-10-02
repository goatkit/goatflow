package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Regression: customer_user.pw is nullable (an account created without a
// password, signed in through SSO). The change-password handler scanned it into
// a string and answered 500 "Failed to verify current password" instead of
// treating the current password as wrong.
func TestCustomerChangePassword_NullStoredPassword(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)

	login := fmt.Sprintf("nullpw_%d@example.test", time.Now().UnixNano())
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, title, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'nullpw-co', NULL, NULL, 'Null', 'Password', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`),
		login, login)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = ?"), login)
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", login)
		c.Set("user_role", "Customer")
		c.Set("authenticated", true)
		c.Next()
	})
	router.POST("/customer/password/change", handleCustomerChangePassword(db))

	body := `{"current_password":"anything","new_password":"Newpass123","confirm_password":"Newpass123"}`
	req := httptest.NewRequest(http.MethodPost, "/customer/password/change", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Current password is incorrect", response["error"])

	var pw *string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM customer_user WHERE login = ?"), login).Scan(&pw))
	assert.Nil(t, pw, "stored password must stay unset")
}
