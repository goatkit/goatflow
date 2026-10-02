package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// The agent web login (/api/auth/login) is throttled like every other login:
// after 5 failures for one IP + login name the 6th try gets 429, even with
// the right password.
func TestAgentWebLoginIsRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NotNil(t, GetAuthService(), "auth service needs the test database")

	r := gin.New()
	r.POST("/api/auth/login", HandleAuthLogin)
	login := fmt.Sprintf("throttle-%d", time.Now().UnixNano())
	post := func() *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"login":%q,"password":"wrong-password"}`, login)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.7:4000"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	for i := range 5 {
		w := post()
		require.Equal(t, http.StatusUnauthorized, w.Code, "attempt %d: %s", i+1, w.Body.String())
	}
	w := post()
	require.Equal(t, http.StatusTooManyRequests, w.Code, w.Body.String())
	require.NotEmpty(t, w.Header().Get("Retry-After"))
	require.Contains(t, w.Body.String(), "too many failed attempts")
}

func TestCustomerOnlyGuardAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CustomerOnlyGuard(true))
	r.NoRoute(func(c *gin.Context) { c.Status(http.StatusOK) })

	cases := map[string]int{
		"/":                             http.StatusOK,
		"/health":                       http.StatusOK,
		"/healthz":                      http.StatusOK,
		"/login":                        http.StatusOK,
		"/customer":                     http.StatusOK,
		"/customer/tickets":             http.StatusOK,
		"/auth/customer":                http.StatusOK,
		"/api/auth/customer/login":      http.StatusOK,
		"/api/auth/customer/2fa/verify": http.StatusOK,
		"/api/languages":                http.StatusOK,
		"/static/css/output.css":        http.StatusOK,
		"/health/detailed":              http.StatusNotFound,
		"/api/auth/login":               http.StatusNotFound,
		"/api/auth/2fa/verify":          http.StatusNotFound,
		"/api/auth/passkey/begin":       http.StatusNotFound,
		"/login/2fa":                    http.StatusNotFound,
		"/customers/search/x":           http.StatusNotFound,
		"/admin":                        http.StatusNotFound,
		"/api/v1/tickets":               http.StatusNotFound,
		"/staticfoo":                    http.StatusNotFound,
	}
	for path, want := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, want, w.Code, path)
	}
}

// wrongTOTPCode returns a six-digit code the authenticator will reject now
// (it differs from every code inside the validation skew).
func wrongTOTPCode(t *testing.T, secret string) string {
	t.Helper()
	valid := map[string]bool{}
	for _, d := range []time.Duration{-60 * time.Second, -30 * time.Second, 0, 30 * time.Second, 60 * time.Second} {
		code, err := totp.GenerateCode(secret, time.Now().Add(d))
		require.NoError(t, err)
		valid[code] = true
	}
	for n := 0; ; n++ {
		if code := fmt.Sprintf("%06d", n); !valid[code] {
			return code
		}
	}
}

// Second-factor guesses are counted per login across pending sessions: a
// fresh login with the right password (which clears the password counter and
// starts a new pending 2FA session) must not hand out fresh code guesses.
// 4 wrong codes, log in again, 1 wrong code: the next try is refused with 429,
// even with the correct code.
func TestSecondFactorThrottleSurvivesFreshPasswordLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, GetAuthService(), "auth service needs the test database")
	jwtManager := shared.GetJWTManager()

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	password := "Throttle-2FA-" + sfx
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)
	totpSvc := service.NewTOTPService(db, "GoatFlow")

	cases := []struct {
		name      string
		ip        string
		loginPath string
		login     gin.HandlerFunc
		cookie    string
		verify    gin.HandlerFunc
		setup     func(t *testing.T) (login, secret string)
	}{
		{
			name:      "agent",
			ip:        "203.0.113.21",
			loginPath: "/api/auth/login",
			login:     HandleAuthLogin,
			cookie:    "2fa_pending",
			verify:    handle2FAVerify(jwtManager),
			setup: func(t *testing.T) (string, string) {
				login := "throttle2fa_agent_" + sfx
				id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
					INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
					VALUES (?, ?, 'Throttle', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`), login, hash)
				require.NoError(t, err)
				userID := int(id)
				t.Cleanup(func() {
					_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM user_preferences WHERE user_id = ?"), userID)
					_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), userID)
				})
				setup, err := totpSvc.GenerateSetup(userID, login+"@example.test")
				require.NoError(t, err)
				code, err := totp.GenerateCode(setup.Secret, time.Now())
				require.NoError(t, err)
				require.NoError(t, totpSvc.ConfirmSetup(userID, code))
				return login, setup.Secret
			},
		},
		{
			name:      "customer",
			ip:        "203.0.113.22",
			loginPath: "/api/auth/customer/login",
			login:     handleCustomerLogin(jwtManager),
			cookie:    "customer_2fa_pending",
			verify:    handleCustomer2FAVerify,
			setup: func(t *testing.T) (string, string) {
				login := "throttle2fa_cust_" + sfx
				_, err := db.Exec(database.ConvertPlaceholders(`
					INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
					VALUES (?, ?, 'throttle-co', ?, 'Throttle', 'Customer', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`),
					login, login+"@example.test", hash)
				require.NoError(t, err)
				t.Cleanup(func() {
					_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_preferences WHERE user_id = ?"), login)
					_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = ?"), login)
				})
				setup, err := totpSvc.GenerateSetupForCustomer(login)
				require.NoError(t, err)
				code, err := totp.GenerateCode(setup.Secret, time.Now())
				require.NoError(t, err)
				require.NoError(t, totpSvc.ConfirmSetupForCustomer(login, code))
				return login, setup.Secret
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			login, secret := tc.setup(t)
			r := gin.New()
			r.POST(tc.loginPath, tc.login)
			r.POST("/verify", tc.verify)
			send := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "throttle-test")
				req.RemoteAddr = tc.ip + ":4000"
				if cookie != nil {
					req.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				return w
			}
			// Correct password: the real login handler opens a pending 2FA session.
			passwordLogin := func() *http.Cookie {
				t.Helper()
				w := send(tc.loginPath, fmt.Sprintf(`{"login":%q,"password":%q}`, login, password), nil)
				require.Contains(t, []int{http.StatusOK, http.StatusFound}, w.Code, w.Body.String())
				for _, ck := range w.Result().Cookies() {
					if ck.Name == tc.cookie && ck.Value != "" {
						return ck
					}
				}
				t.Fatalf("login set no %s cookie (status %d): %s", tc.cookie, w.Code, w.Body.String())
				return nil
			}
			guess := func(cookie *http.Cookie, code string) *httptest.ResponseRecorder {
				return send("/verify", fmt.Sprintf(`{"code":%q}`, code), cookie)
			}

			first := passwordLogin()
			for i := range 4 {
				w := guess(first, wrongTOTPCode(t, secret))
				require.Equal(t, http.StatusUnauthorized, w.Code, "guess %d: %s", i+1, w.Body.String())
			}

			second := passwordLogin()
			w := guess(second, wrongTOTPCode(t, secret))
			require.Equal(t, http.StatusUnauthorized, w.Code, "guess 5: %s", w.Body.String())

			code, err := totp.GenerateCode(secret, time.Now())
			require.NoError(t, err)
			w = guess(second, code)
			require.Equal(t, http.StatusTooManyRequests, w.Code, "6th code check must be throttled: %s", w.Body.String())
			require.NotEmpty(t, w.Header().Get("Retry-After"))
			require.Contains(t, w.Body.String(), "too many failed attempts")
		})
	}
}
