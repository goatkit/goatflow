package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// Every endpoint that re-checks a signed-in account's password counts failed
// checks per account and client IP: after 5 wrong passwords the next try is
// refused with 429, even with the right password, so a hijacked session
// cannot be used to guess the password.
func TestPasswordRecheckEndpointsAreRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err)

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	password := "Recheck-Pass-" + sfx
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)

	agentLogin := "recheck_agent_" + sfx
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'Recheck', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`), agentLogin, hash)
	require.NoError(t, err)
	agentID := int(id)
	customerLogin := "recheck_cust_" + sfx
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'recheck-co', ?, 'Recheck', 'Customer', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`),
		customerLogin, customerLogin+"@example.test", hash)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM user_preferences WHERE user_id = ?"), agentID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), agentID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_preferences WHERE user_id = ?"), customerLogin)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = ?"), customerLogin)
	})

	asAgent := func(c *gin.Context) {
		c.Set("user_id", agentID)
		c.Set("user_login", agentLogin)
		c.Next()
	}
	asCustomer := func(c *gin.Context) {
		c.Set("is_customer", true)
		c.Set("username", customerLogin)
		c.Set("user_role", "Customer")
		c.Next()
	}
	passwordBody := func(pw string) string { return fmt.Sprintf(`{"password":%q}`, pw) }
	codeBody := func(pw string) string { return fmt.Sprintf(`{"code":"000000","password":%q}`, pw) }
	changeBody := func(pw string) string {
		return fmt.Sprintf(`{"current_password":%q,"new_password":"Changed-Pass-1x","confirm_password":"Changed-Pass-1x"}`, pw)
	}

	cases := []struct {
		name    string
		method  string
		route   string
		path    string
		as      gin.HandlerFunc
		handler gin.HandlerFunc
		body    func(string) string
	}{
		{"agent password change", http.MethodPost, "/x", "/x", asAgent, HandleAgentChangePassword, changeBody},
		{"agent 2fa setup", http.MethodPost, "/x", "/x", asAgent, handleTOTPSetup, passwordBody},
		{"agent 2fa confirm", http.MethodPost, "/x", "/x", asAgent, handleTOTPConfirm, codeBody},
		{"agent 2fa disable", http.MethodPost, "/x", "/x", asAgent, handleTOTPDisable, codeBody},
		{"agent recovery codes", http.MethodPost, "/x", "/x", asAgent, handleRecoveryCodesRegenerate, passwordBody},
		{"agent passkey register", http.MethodPost, "/x", "/x", asAgent, handleWebAuthnRegisterBegin, passwordBody},
		{"agent passkey remove", http.MethodDelete, "/x/:id", "/x/999999", asAgent, handleWebAuthnCredentialDelete, passwordBody},
		{"customer password change", http.MethodPost, "/x", "/x", asCustomer, handleCustomerChangePassword(db), changeBody},
		{"customer 2fa setup", http.MethodPost, "/x", "/x", asCustomer, handleCustomerTOTPSetup, passwordBody},
		{"customer 2fa confirm", http.MethodPost, "/x", "/x", asCustomer, handleCustomerTOTPConfirm, codeBody},
		{"customer 2fa disable", http.MethodPost, "/x", "/x", asCustomer, handleCustomerTOTPDisable, codeBody},
		{"customer recovery codes", http.MethodPost, "/x", "/x", asCustomer, handleCustomerRecoveryCodesRegenerate, passwordBody},
		{"customer passkey register", http.MethodPost, "/x", "/x", asCustomer, handleCustomerWebAuthnRegisterBegin, passwordBody},
		{"customer passkey remove", http.MethodDelete, "/x/:id", "/x/999999", asCustomer, handleCustomerWebAuthnCredentialDelete, passwordBody},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// One client address per endpoint keeps the per-IP counters apart.
			ip := fmt.Sprintf("203.0.113.%d", 100+i)
			r := gin.New()
			r.Handle(tc.method, tc.route, tc.as, tc.handler)
			send := func(pw string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body(pw)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json")
				req.RemoteAddr = ip + ":4000"
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				return w
			}
			for n := range 5 {
				w := send("wrong-password")
				require.Equal(t, http.StatusUnauthorized, w.Code, "wrong password %d: %s", n+1, w.Body.String())
			}
			w := send(password)
			require.Equal(t, http.StatusTooManyRequests, w.Code, "check after 5 failures must be throttled: %s", w.Body.String())
			require.NotEmpty(t, w.Header().Get("Retry-After"))
			require.Contains(t, w.Body.String(), "too many failed attempts")
		})
	}
}
