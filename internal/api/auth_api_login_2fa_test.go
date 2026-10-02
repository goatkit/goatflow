package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/service"
)

// POST /api/v1/auth/login has no second step, so an account that enrolled a
// second factor must not get tokens from its password alone — the web login
// sends the same account to /login/2fa.
func TestAPILoginRefusesAccountsWithSecondFactor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	router := NewSimpleRouter()

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	password := "Api-2fa-" + sfx
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)
	agentLogin, customerLogin := "api2fa_agent_"+sfx, "api2fa_cust_"+sfx
	agentID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'Api', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`), agentLogin, hash)
	require.NoError(t, err)
	customerID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'api2fa-co', ?, 'Api', 'Customer', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`),
		customerLogin, customerLogin+"@example.test", hash)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM user_preferences WHERE user_id = ?`), agentID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_preferences WHERE user_id = ?`), customerLogin)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), agentID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE id = ?`), customerID)
	})

	login := func(user string) (int, map[string]any) {
		t.Helper()
		w := searchRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", gin.H{"login": user, "password": password})
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body
	}

	code, body := login(agentLogin)
	require.Equal(t, http.StatusOK, code, "agent without a second factor logs in: %v", body)
	require.NotEmpty(t, body["access_token"])
	code, body = login(customerLogin)
	require.Equal(t, http.StatusOK, code, "customer without a second factor logs in: %v", body)

	require.NoError(t, service.NewUserPreferencesBackend(db, int(agentID)).Set("UserTOTPEnabled", "1"))
	require.NoError(t, service.NewCustomerPreferencesBackend(db, customerLogin).Set("UserTOTPEnabled", "1"))

	for _, user := range []string{agentLogin, customerLogin} {
		code, body := login(user)
		require.Equal(t, http.StatusForbidden, code, "%s has TOTP enabled: %v", user, body)
		require.Equal(t, "mfa_required", body["code"], "%v", body)
		require.Nil(t, body["access_token"], "no token may be issued: %v", body)
		require.Nil(t, body["refresh_token"], "no refresh token may be issued: %v", body)
	}
}
