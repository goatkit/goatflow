package selfservice_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/goatkit/goatflow/internal/api"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/repository"
)

const testBaseURL = "https://helpdesk.example.test"

var (
	router *gin.Engine
	db     *sql.DB
	seq    atomic.Int64
)

// TestMain serves the real route files (routes/selfservice.yaml for the
// self-service pages, routes/auth.yaml for login) with the production handler
// registry, against the test database selected by TEST_DB_*.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Setenv("APP_ENV", "test")                         //nolint:errcheck
	os.Setenv("BASE_URL", testBaseURL+"/")               //nolint:errcheck // trailing slash is trimmed
	os.Setenv("GOATFLOW_FEATURES_REGISTRATION", "true")  //nolint:errcheck
	os.Setenv("GOATFLOW_FEATURES_LOST_PASSWORD", "true") //nolint:errcheck

	if err := config.Load("../../config"); err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if err := database.InitTestDB(); err != nil {
		fmt.Fprintln(os.Stderr, "test database unavailable:", err)
		os.Exit(1)
	}
	var err error
	if db, err = database.GetDB(); err != nil || db == nil {
		fmt.Fprintln(os.Stderr, "test database unavailable (set TEST_DB_*):", err)
		os.Exit(1)
	}
	renderer, err := shared.NewTemplateRenderer("../../templates")
	if err != nil {
		fmt.Fprintln(os.Stderr, "templates:", err)
		os.Exit(1)
	}
	shared.SetGlobalRenderer(renderer)
	// Same wiring as cmd/goats/main.go, so the login routes authenticate for real.
	auth.SetUserRepoFactory(func(db *sql.DB) auth.UserLookup { return repository.NewUserRepository(db) })

	routesDir, err := os.MkdirTemp("", "selfservice-routes")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, f := range []string{"selfservice.yaml", "auth.yaml"} {
		src, _ := filepath.Abs(filepath.Join("../../routes", f))
		if err := os.Symlink(src, filepath.Join(routesDir, f)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	router = gin.New()
	if err := routing.LoadYAMLRoutes(router, routesDir, api.NewRoutingHandlerResolver()); err != nil {
		fmt.Fprintln(os.Stderr, "routes:", err)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(routesDir) //nolint:errcheck
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// fixtures and helpers
// ---------------------------------------------------------------------------

type account struct {
	id       int64
	login    string
	email    string
	password string
}

func uniq(prefix string) string {
	return fmt.Sprintf("%s%d%d", prefix, time.Now().UnixNano()%1e9, seq.Add(1))
}

func hashPW(t *testing.T, pw string) string {
	t.Helper()
	h, err := auth.NewPasswordHasher().HashPassword(pw)
	require.NoError(t, err)
	return h
}

func exec(t *testing.T, q string, args ...any) {
	t.Helper()
	_, err := db.Exec(database.ConvertPlaceholders(q), args...)
	require.NoError(t, err, q)
}

func newCustomer(t *testing.T) account {
	t.Helper()
	a := account{login: uniq("sscust"), password: "Old-Passw0rd!"}
	a.email = a.login + "@example.test"
	now := time.Now().UTC()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, 1, ?, 1) RETURNING id`),
		a.login, a.email, "SSCO", hashPW(t, a.password), "Cee", "Ustomer", now, now)
	require.NoError(t, err)
	a.id = id
	t.Cleanup(func() { cleanup(t, a) })
	return a
}

func newAgent(t *testing.T) account {
	t.Helper()
	a := account{login: uniq("ssagent"), password: "Old-Passw0rd!"}
	a.email = a.login + "@example.test"
	now := time.Now().UTC()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, 1, ?, 1, ?, 1) RETURNING id`),
		a.login, hashPW(t, a.password), "Agnes", "Agent", now, now)
	require.NoError(t, err)
	a.id = id
	exec(t, "INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, 'UserEmail', ?)", id, []byte(a.email))
	t.Cleanup(func() {
		cleanup(t, a)
		cleanupExec(t, "DELETE FROM sessions WHERE session_id IN (SELECT session_id FROM (SELECT session_id FROM sessions WHERE data_key = 'UserLogin' AND data_value = ?) s)", a.login)
		cleanupExec(t, "DELETE FROM user_preferences WHERE user_id = ?", id)
		cleanupExec(t, "DELETE FROM users WHERE id = ?", id)
	})
	return a
}

// cleanupExec runs a cleanup statement and reports (without stopping) a failure.
func cleanupExec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(database.ConvertPlaceholders(q), args...); err != nil {
		t.Errorf("cleanup %q: %v", q, err)
	}
}

// cleanup removes everything a test created for the account, dependents first.
func cleanup(t *testing.T, a account) {
	t.Helper()
	cleanupExec(t, "DELETE FROM mail_queue WHERE recipient = ?", a.email)
	cleanupExec(t, "DELETE FROM gk_auth_token WHERE email = ?", a.email)
	cleanupExec(t, "DELETE FROM gk_registration_request WHERE email = ?", a.email)
	cleanupExec(t, "DELETE FROM customer_user WHERE login = ? OR email = ?", a.login, a.email)
}

var ipSeq atomic.Int64

// newIP gives each test its own client address so the per-IP budget of one
// test does not leak into another.
func newIP() string {
	n := ipSeq.Add(1)
	return fmt.Sprintf("198.51.%d.%d", 100+n/250, n%250+1)
}

func do(t *testing.T, ip, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.RemoteAddr = ip + ":40000"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// mails returns the queued raw messages for recipient.
func mails(t *testing.T, recipient string) []string {
	t.Helper()
	rows, err := db.Query(database.ConvertPlaceholders("SELECT raw_message FROM mail_queue WHERE recipient = ? ORDER BY id"), recipient)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var raw []byte
		require.NoError(t, rows.Scan(&raw))
		out = append(out, string(raw))
	}
	require.NoError(t, rows.Err())
	return out
}

// waitForMails waits until n messages are queued for recipient (mail is
// queued after the response so its timing reveals nothing).
func waitForMails(t *testing.T, recipient string, n int) []string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		got := mails(t, recipient)
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d queued mail(s) for %s, got %d", n, recipient, len(got))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func tokenFromMail(t *testing.T, raw, path string) string {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(testBaseURL+path+"?token=") + `([0-9a-f]{64})`)
	m := re.FindStringSubmatch(raw)
	require.NotNil(t, m, "mail has no %s link:\n%s", path, raw)
	return m[1]
}

func login(t *testing.T, path, user, password string) int {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"login": user, "password": password})
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = newIP() + ":40000"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

func customerLogin(t *testing.T, user, password string) int {
	return login(t, "/api/auth/customer/login", user, password)
}

func agentLogin(t *testing.T, user, password string) int {
	return login(t, "/api/auth/login", user, password)
}

func countRows(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(q), args...).Scan(&n))
	return n
}

func requestReset(t *testing.T, ip, forgotPath string, a account) string {
	t.Helper()
	before := len(mails(t, a.email))
	w := do(t, ip, http.MethodPost, forgotPath, url.Values{"identifier": {a.login}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := waitForMails(t, a.email, before+1)
	resetPath := strings.Replace(forgotPath, "forgot-password", "reset-password", 1)
	return tokenFromMail(t, got[len(got)-1], resetPath)
}

func reset(t *testing.T, ip, resetPath, token, pw, confirm string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, ip, http.MethodPost, resetPath, url.Values{"token": {token}, "password": {pw}, "confirm_password": {confirm}})
}

// ---------------------------------------------------------------------------
// password reset
// ---------------------------------------------------------------------------

func TestForgotPassword_SameAnswerForExistingAndUnknownAccounts(t *testing.T) {
	for _, tc := range []struct {
		name, forgot string
		mk           func(*testing.T) account
	}{
		{"customer", "/customer/forgot-password", newCustomer},
		{"agent", "/forgot-password", newAgent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ip := newIP()
			a := tc.mk(t)
			unknown := uniq("nobody") + "@example.test"

			known := do(t, ip, http.MethodPost, tc.forgot, url.Values{"identifier": {a.email}})
			missing := do(t, ip, http.MethodPost, tc.forgot, url.Values{"identifier": {unknown}})

			require.Equal(t, http.StatusOK, known.Code)
			require.Equal(t, http.StatusOK, missing.Code)
			assert.Equal(t, known.Body.String(), missing.Body.String(), "response must not reveal whether the account exists")
			assert.Contains(t, known.Body.String(), "If an account exists, a reset link has been sent.")

			raw := waitForMails(t, a.email, 1)[0]
			assert.Contains(t, raw, "To: "+a.email)
			resetPath := strings.Replace(tc.forgot, "forgot-password", "reset-password", 1)
			token := tokenFromMail(t, raw, resetPath)
			assert.Equal(t, 1, countRows(t, "SELECT COUNT(*) FROM gk_auth_token WHERE email = ? AND used_at IS NULL", a.email))
			assert.Zero(t, countRows(t, "SELECT COUNT(*) FROM gk_auth_token WHERE token = ?", token), "raw token must not be stored")
			assert.Zero(t, countRows(t, "SELECT COUNT(*) FROM gk_auth_token WHERE email = ?", unknown))
			assert.Empty(t, mails(t, unknown))
		})
	}
}

func TestForgotPassword_RequiresIdentifier(t *testing.T) {
	w := do(t, newIP(), http.MethodPost, "/customer/forgot-password", url.Values{"identifier": {"  "}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Enter your username or email address.")
}

func TestForgotPassword_PerIPBudget(t *testing.T) {
	ip := newIP()
	for i := range 10 {
		w := do(t, ip, http.MethodPost, "/customer/forgot-password", url.Values{"identifier": {uniq("x")}})
		require.Equal(t, http.StatusOK, w.Code, "request %d", i+1)
	}
	w := do(t, ip, http.MethodPost, "/customer/forgot-password", url.Values{"identifier": {uniq("x")}})
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestCustomerPasswordReset(t *testing.T) {
	ip := newIP()
	a := newCustomer(t)
	require.Equal(t, http.StatusOK, customerLogin(t, a.login, a.password))

	// An open session of the account ends when the password is reset.
	sid := uniq("sess")
	for k, v := range map[string]string{"UserLogin": a.login, "UserType": "Customer"} {
		exec(t, "INSERT INTO sessions (session_id, data_key, data_value, serialized) VALUES (?, ?, ?, 0)", sid, k, v)
	}
	t.Cleanup(func() { cleanupExec(t, "DELETE FROM sessions WHERE session_id = ?", sid) })

	token := requestReset(t, ip, "/customer/forgot-password", a)

	page := do(t, ip, http.MethodGet, "/customer/reset-password?token="+token, nil)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), `action="/customer/reset-password"`)
	assert.Contains(t, page.Body.String(), `value="`+token+`"`)
	assert.Equal(t, "no-referrer", page.Header().Get("Referrer-Policy"))

	// A mismatch is rejected without using up the token.
	w := reset(t, ip, "/customer/reset-password", token, "New-Passw0rd!", "different")
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Passwords do not match")

	w = reset(t, ip, "/customer/reset-password", token, "New-Passw0rd!", "New-Passw0rd!")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Password reset successfully.")

	assert.Equal(t, http.StatusOK, customerLogin(t, a.login, "New-Passw0rd!"))
	assert.Equal(t, http.StatusUnauthorized, customerLogin(t, a.login, a.password))
	assert.Zero(t, countRows(t, "SELECT COUNT(*) FROM sessions WHERE session_id = ?", sid))

	// Single use: the same link no longer works.
	again := reset(t, ip, "/customer/reset-password", token, "Third-Passw0rd!", "Third-Passw0rd!")
	assert.Equal(t, http.StatusBadRequest, again.Code)
	assert.Contains(t, again.Body.String(), "This link is invalid or has expired.")
	assert.Equal(t, http.StatusBadRequest, do(t, ip, http.MethodGet, "/customer/reset-password?token="+token, nil).Code)
	assert.Equal(t, http.StatusOK, customerLogin(t, a.login, "New-Passw0rd!"))
}

func TestAgentPasswordReset(t *testing.T) {
	ip := newIP()
	a := newAgent(t)
	token := requestReset(t, ip, "/forgot-password", a)

	w := reset(t, ip, "/reset-password", token, "New-Passw0rd!", "New-Passw0rd!")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, http.StatusOK, agentLogin(t, a.login, "New-Passw0rd!"))
	assert.Equal(t, http.StatusUnauthorized, agentLogin(t, a.login, a.password))
	assert.Equal(t, http.StatusBadRequest, reset(t, ip, "/reset-password", token, "Other-Passw0rd!", "Other-Passw0rd!").Code)
}

func TestPasswordReset_ExpiredTokenRejected(t *testing.T) {
	ip := newIP()
	a := newCustomer(t)
	token := requestReset(t, ip, "/customer/forgot-password", a)
	exec(t, "UPDATE gk_auth_token SET expires_at = ? WHERE email = ?", time.Now().UTC().Add(-time.Minute), a.email)

	w := reset(t, ip, "/customer/reset-password", token, "New-Passw0rd!", "New-Passw0rd!")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "This link is invalid or has expired.")
	assert.Equal(t, http.StatusOK, customerLogin(t, a.login, a.password), "password must be unchanged")
}

func TestPasswordReset_TokenBoundToItsAccount(t *testing.T) {
	ip := newIP()
	cust := newCustomer(t)
	agent := newAgent(t)
	custToken := requestReset(t, ip, "/customer/forgot-password", cust)
	agentToken := requestReset(t, ip, "/forgot-password", agent)

	// A token only works on the side (agent/customer) it was issued for.
	assert.Equal(t, http.StatusBadRequest, reset(t, ip, "/reset-password", custToken, "New-Passw0rd!", "New-Passw0rd!").Code)
	assert.Equal(t, http.StatusBadRequest, reset(t, ip, "/customer/reset-password", agentToken, "New-Passw0rd!", "New-Passw0rd!").Code)
	assert.Equal(t, http.StatusOK, customerLogin(t, cust.login, cust.password))
	assert.Equal(t, http.StatusOK, agentLogin(t, agent.login, agent.password))

	// A newer request revokes the older link.
	newer := requestReset(t, ip, "/customer/forgot-password", cust)
	assert.Equal(t, http.StatusBadRequest, reset(t, ip, "/customer/reset-password", custToken, "New-Passw0rd!", "New-Passw0rd!").Code)

	// The link dies when the account's email address changes after it was sent.
	exec(t, "UPDATE customer_user SET email = ? WHERE login = ?", "moved-"+cust.email, cust.login)
	assert.Equal(t, http.StatusBadRequest, reset(t, ip, "/customer/reset-password", newer, "New-Passw0rd!", "New-Passw0rd!").Code)
	assert.Equal(t, http.StatusOK, customerLogin(t, cust.login, cust.password))
}

// ---------------------------------------------------------------------------
// customer self-registration
// ---------------------------------------------------------------------------

func TestCustomerRegistration(t *testing.T) {
	ip := newIP()
	a := account{email: uniq("ssreg") + "@example.test", password: "Fresh-Passw0rd!"}
	t.Cleanup(func() { cleanup(t, a) })

	w := do(t, ip, http.MethodPost, "/customer/register", url.Values{
		"first_name": {"Reggie"}, "last_name": {"Strant"}, "email": {strings.ToUpper(a.email)},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Registration submitted. Check your email.")
	assert.Zero(t, countRows(t, "SELECT COUNT(*) FROM customer_user WHERE login = ?", a.email), "no account before confirmation")

	token := tokenFromMail(t, waitForMails(t, a.email, 1)[0], "/customer/register/complete")
	page := do(t, ip, http.MethodGet, "/customer/register/complete?token="+token, nil)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), `value="`+a.email+`"`)

	w = do(t, ip, http.MethodPost, "/customer/register/complete", url.Values{
		"token": {token}, "password": {a.password}, "confirm_password": {a.password},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Your account has been created. You can now sign in.")

	var first, last, customerID string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT first_name, last_name, customer_id FROM customer_user WHERE login = ? AND email = ? AND valid_id = 1"), a.email, a.email,
	).Scan(&first, &last, &customerID))
	assert.Equal(t, []string{"Reggie", "Strant", a.email}, []string{first, last, customerID})
	assert.Equal(t, http.StatusOK, customerLogin(t, a.email, a.password))

	reuse := do(t, ip, http.MethodPost, "/customer/register/complete", url.Values{
		"token": {token}, "password": {"x"}, "confirm_password": {"x"},
	})
	assert.Equal(t, http.StatusBadRequest, reuse.Code)
}

func TestCustomerRegistration_ExistingAddressGetsSameAnswer(t *testing.T) {
	ip := newIP()
	existing := newCustomer(t)
	fresh := account{email: uniq("ssreg") + "@example.test"}
	t.Cleanup(func() { cleanup(t, fresh) })

	form := func(email string) url.Values {
		return url.Values{"first_name": {"Ann"}, "last_name": {"Other"}, "email": {email}}
	}
	known := do(t, ip, http.MethodPost, "/customer/register", form(existing.email))
	unknown := do(t, ip, http.MethodPost, "/customer/register", form(fresh.email))
	require.Equal(t, http.StatusOK, known.Code)
	assert.Equal(t, unknown.Body.String(), known.Body.String())

	raw := waitForMails(t, existing.email, 1)[0]
	assert.Contains(t, raw, testBaseURL+"/customer/forgot-password")
	assert.NotContains(t, raw, "/customer/register/complete")
	assert.Zero(t, countRows(t, "SELECT COUNT(*) FROM gk_registration_request WHERE email = ?", existing.email))
	assert.Equal(t, http.StatusOK, customerLogin(t, existing.login, existing.password))
}

func TestCustomerRegistration_RejectsInvalidInput(t *testing.T) {
	ip := newIP()
	w := do(t, ip, http.MethodPost, "/customer/register", url.Values{"first_name": {"A"}, "last_name": {"B"}, "email": {"not-an-address"}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = do(t, ip, http.MethodPost, "/customer/register", url.Values{"email": {"a@example.test"}})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
